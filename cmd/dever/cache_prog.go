package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	cacheProgAccessUpdateInterval = 10 * time.Minute
	cacheProgMetadataSuffix       = ".json"
	cacheProgObjectSuffix         = ".data"
	cacheProgTempPrefix           = ".tmp-"
)

type cacheProgCommand string

const (
	cacheProgGet   cacheProgCommand = "get"
	cacheProgPut   cacheProgCommand = "put"
	cacheProgClose cacheProgCommand = "close"
)

type cacheProgRequest struct {
	ID       int64
	Command  cacheProgCommand
	ActionID []byte `json:",omitempty"`
	OutputID []byte `json:",omitempty"`
	BodySize int64  `json:",omitempty"`
}

type cacheProgResponse struct {
	ID            int64
	Err           string             `json:",omitempty"`
	KnownCommands []cacheProgCommand `json:",omitempty"`
	Miss          bool               `json:",omitempty"`
	OutputID      []byte             `json:",omitempty"`
	Size          int64              `json:",omitempty"`
	Time          *time.Time         `json:",omitempty"`
	DiskPath      string             `json:",omitempty"`
}

type boundedCacheMetadata struct {
	OutputID string `json:"output_id"`
	Size     int64  `json:"size"`
}

type boundedCacheEntry struct {
	actionID   string
	outputID   string
	size       int64
	lastUsed   time.Time
	metaPath   string
	objectPath string
}

type boundedCacheProgram struct {
	actionsDir  string
	objectsDir  string
	maxBytes    int64
	targetBytes int64
	totalBytes  int64

	// Action 索引可以共享同一个内容寻址对象；返回给 Go 的对象在 close 前保持固定。
	entries     map[string]*boundedCacheEntry
	objectRefs  map[string]int
	objectSizes map[string]int64
	pinned      map[string]struct{}
}

func runCacheProg(args []string) {
	fs := flag.NewFlagSet("cache-prog", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dir := fs.String("dir", "", "缓存目录")
	maxBytes := fs.Int64("max-bytes", 0, "缓存高水位字节数")
	targetBytes := fs.Int64("target-bytes", 0, "缓存淘汰目标字节数")
	if err := fs.Parse(args); err != nil {
		logCacheProgFatal(err)
	}

	program, err := newBoundedCacheProgram(*dir, *maxBytes, *targetBytes)
	if err != nil {
		logCacheProgFatal(err)
	}
	if err := program.serve(os.Stdin, os.Stdout); err != nil {
		logCacheProgFatal(err)
	}
}

func logCacheProgFatal(err error) {
	fmt.Fprintf(os.Stderr, "dever cache-prog: %v\n", err)
	os.Exit(1)
}

func newBoundedCacheProgram(rootDir string, maxBytes, targetBytes int64) (*boundedCacheProgram, error) {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		return nil, errors.New("缓存目录不能为空")
	}
	absoluteRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("解析缓存目录失败: %w", err)
	}
	if maxBytes <= 0 {
		return nil, errors.New("缓存高水位必须大于 0")
	}
	if targetBytes <= 0 || targetBytes > maxBytes {
		return nil, errors.New("缓存淘汰目标必须大于 0 且不能超过高水位")
	}

	program := &boundedCacheProgram{
		actionsDir:  filepath.Join(absoluteRoot, "actions"),
		objectsDir:  filepath.Join(absoluteRoot, "objects"),
		maxBytes:    maxBytes,
		targetBytes: targetBytes,
		entries:     make(map[string]*boundedCacheEntry),
		objectRefs:  make(map[string]int),
		objectSizes: make(map[string]int64),
		pinned:      make(map[string]struct{}),
	}
	for _, dir := range []string{absoluteRoot, program.actionsDir, program.objectsDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("创建缓存目录失败 %s: %w", dir, err)
		}
	}
	return program, nil
}

func (p *boundedCacheProgram) serve(input io.Reader, output io.Writer) error {
	writer := bufio.NewWriter(output)
	encoder := json.NewEncoder(writer)
	if err := writeCacheProgResponse(encoder, writer, cacheProgResponse{
		ID:            0,
		KnownCommands: []cacheProgCommand{cacheProgGet, cacheProgPut, cacheProgClose},
	}); err != nil {
		return fmt.Errorf("发送缓存协议能力失败: %w", err)
	}

	if err := p.load(); err != nil {
		return err
	}
	if err := p.evict(false); err != nil {
		return err
	}

	decoder := json.NewDecoder(bufio.NewReader(input))
	for {
		var request cacheProgRequest
		if err := decoder.Decode(&request); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("读取缓存协议请求失败: %w", err)
		}

		response, closeProgram, err := p.handleRequest(decoder, request)
		response.ID = request.ID
		if err != nil {
			response.Err = err.Error()
		}
		if writeErr := writeCacheProgResponse(encoder, writer, response); writeErr != nil {
			return fmt.Errorf("发送缓存协议响应失败: %w", writeErr)
		}
		if closeProgram {
			return nil
		}
	}
}

func writeCacheProgResponse(encoder *json.Encoder, writer *bufio.Writer, response cacheProgResponse) error {
	if err := encoder.Encode(response); err != nil {
		return err
	}
	return writer.Flush()
}

func (p *boundedCacheProgram) handleRequest(decoder *json.Decoder, request cacheProgRequest) (cacheProgResponse, bool, error) {
	switch request.Command {
	case cacheProgGet:
		response, err := p.get(request.ActionID)
		return response, false, err
	case cacheProgPut:
		body, err := readCacheProgBody(decoder, request.BodySize)
		if err != nil {
			return cacheProgResponse{}, false, err
		}
		response, err := p.put(request.ActionID, request.OutputID, body)
		return response, false, err
	case cacheProgClose:
		p.pinned = make(map[string]struct{})
		if err := p.removeOrphanObjects(); err != nil {
			return cacheProgResponse{}, true, err
		}
		return cacheProgResponse{}, true, p.evict(true)
	default:
		return cacheProgResponse{}, false, fmt.Errorf("不支持的缓存协议命令 %q", request.Command)
	}
}

func readCacheProgBody(decoder *json.Decoder, expectedSize int64) ([]byte, error) {
	if expectedSize < 0 {
		return nil, errors.New("缓存对象大小不能为负数")
	}
	if expectedSize == 0 {
		return nil, nil
	}
	var body []byte
	if err := decoder.Decode(&body); err != nil {
		return nil, fmt.Errorf("读取缓存对象内容失败: %w", err)
	}
	if int64(len(body)) != expectedSize {
		return nil, fmt.Errorf("缓存对象大小不匹配: 实际 %d，期望 %d", len(body), expectedSize)
	}
	return body, nil
}

func (p *boundedCacheProgram) get(actionID []byte) (cacheProgResponse, error) {
	actionKey, err := cacheProgID(actionID)
	if err != nil {
		return cacheProgResponse{}, err
	}
	entry := p.entries[actionKey]
	if entry == nil {
		return cacheProgResponse{Miss: true}, nil
	}
	info, err := os.Stat(entry.objectPath)
	if err != nil || info.Size() != entry.size {
		if removeErr := p.removeEntry(entry); removeErr != nil {
			return cacheProgResponse{}, removeErr
		}
		return cacheProgResponse{Miss: true}, nil
	}

	now := time.Now()
	if entry.lastUsed.After(now) || now.Sub(entry.lastUsed) >= cacheProgAccessUpdateInterval {
		if err := os.Chtimes(entry.metaPath, now, now); err != nil {
			return cacheProgResponse{}, fmt.Errorf("更新缓存访问时间失败: %w", err)
		}
		entry.lastUsed = now
	}
	p.pinned[entry.objectPath] = struct{}{}
	outputID, err := hex.DecodeString(entry.outputID)
	if err != nil {
		return cacheProgResponse{}, fmt.Errorf("解析缓存输出 ID 失败: %w", err)
	}
	accessedAt := entry.lastUsed
	return cacheProgResponse{
		OutputID: outputID,
		Size:     entry.size,
		Time:     &accessedAt,
		DiskPath: entry.objectPath,
	}, nil
}

func (p *boundedCacheProgram) put(actionID, outputID, body []byte) (cacheProgResponse, error) {
	actionKey, err := cacheProgID(actionID)
	if err != nil {
		return cacheProgResponse{}, err
	}
	outputKey, err := cacheProgID(outputID)
	if err != nil {
		return cacheProgResponse{}, err
	}
	digest := sha256.Sum256(body)
	if !bytes.Equal(digest[:], outputID) {
		return cacheProgResponse{}, errors.New("缓存对象校验值与输出 ID 不一致")
	}
	if existing := p.entries[actionKey]; existing != nil {
		if existing.outputID == outputKey {
			if info, statErr := os.Stat(existing.objectPath); statErr == nil && info.Size() == int64(len(body)) {
				now := time.Now()
				if err := os.Chtimes(existing.metaPath, now, now); err != nil {
					return cacheProgResponse{}, fmt.Errorf("更新缓存访问时间失败: %w", err)
				}
				existing.lastUsed = now
				p.pinned[existing.objectPath] = struct{}{}
				return cacheProgResponse{DiskPath: existing.objectPath}, nil
			}
		}
		if err := p.removeEntry(existing); err != nil {
			return cacheProgResponse{}, err
		}
	}

	objectPath := p.objectPath(outputKey)
	if err := os.MkdirAll(filepath.Dir(objectPath), 0o700); err != nil {
		return cacheProgResponse{}, fmt.Errorf("创建缓存对象目录失败: %w", err)
	}
	if info, statErr := os.Stat(objectPath); statErr != nil || info.Size() != int64(len(body)) {
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return cacheProgResponse{}, fmt.Errorf("读取缓存对象失败: %w", statErr)
		}
		if statErr == nil {
			if err := os.Remove(objectPath); err != nil {
				return cacheProgResponse{}, fmt.Errorf("删除损坏的缓存对象失败: %w", err)
			}
		}
		if err := atomicWriteCacheFile(objectPath, body); err != nil {
			return cacheProgResponse{}, fmt.Errorf("写入缓存对象失败: %w", err)
		}
	}

	metaPath := p.metadataPath(actionKey)
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o700); err != nil {
		return cacheProgResponse{}, fmt.Errorf("创建缓存索引目录失败: %w", err)
	}
	metadata, err := json.Marshal(boundedCacheMetadata{OutputID: outputKey, Size: int64(len(body))})
	if err != nil {
		return cacheProgResponse{}, fmt.Errorf("编码缓存索引失败: %w", err)
	}
	if err := atomicWriteCacheFile(metaPath, metadata); err != nil {
		return cacheProgResponse{}, fmt.Errorf("写入缓存索引失败: %w", err)
	}

	now := time.Now()
	entry := &boundedCacheEntry{
		actionID:   actionKey,
		outputID:   outputKey,
		size:       int64(len(body)),
		lastUsed:   now,
		metaPath:   metaPath,
		objectPath: objectPath,
	}
	p.entries[actionKey] = entry
	p.attachObject(entry)
	p.pinned[objectPath] = struct{}{}
	if err := p.evict(false); err != nil {
		return cacheProgResponse{}, err
	}
	return cacheProgResponse{DiskPath: objectPath}, nil
}

func cacheProgID(value []byte) (string, error) {
	if len(value) != sha256.Size {
		return "", fmt.Errorf("缓存 ID 长度错误: %d", len(value))
	}
	return hex.EncodeToString(value), nil
}

func (p *boundedCacheProgram) load() error {
	for _, dir := range []string{p.actionsDir, p.objectsDir} {
		if err := cleanupCacheTempFiles(dir); err != nil {
			return err
		}
	}
	if err := filepath.WalkDir(p.actionsDir, func(path string, item os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if item.IsDir() || !strings.HasSuffix(item.Name(), cacheProgMetadataSuffix) {
			return nil
		}
		actionKey := strings.TrimSuffix(item.Name(), cacheProgMetadataSuffix)
		if !validCacheProgHexID(actionKey) {
			return os.Remove(path)
		}
		entry, err := p.loadEntry(actionKey, path)
		if err != nil {
			if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				return removeErr
			}
			return nil
		}
		p.entries[actionKey] = entry
		p.attachObject(entry)
		return nil
	}); err != nil {
		return fmt.Errorf("扫描缓存索引失败: %w", err)
	}
	return p.removeOrphanObjects()
}

func (p *boundedCacheProgram) loadEntry(actionKey, metaPath string) (*boundedCacheEntry, error) {
	content, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}
	var metadata boundedCacheMetadata
	if err := json.Unmarshal(content, &metadata); err != nil {
		return nil, err
	}
	if !validCacheProgHexID(metadata.OutputID) || metadata.Size < 0 {
		return nil, errors.New("缓存索引内容无效")
	}
	objectPath := p.objectPath(metadata.OutputID)
	objectInfo, err := os.Stat(objectPath)
	if err != nil || objectInfo.Size() != metadata.Size {
		return nil, errors.New("缓存索引引用的对象不存在或大小不匹配")
	}
	metaInfo, err := os.Stat(metaPath)
	if err != nil {
		return nil, err
	}
	return &boundedCacheEntry{
		actionID:   actionKey,
		outputID:   metadata.OutputID,
		size:       metadata.Size,
		lastUsed:   metaInfo.ModTime(),
		metaPath:   metaPath,
		objectPath: objectPath,
	}, nil
}

func (p *boundedCacheProgram) attachObject(entry *boundedCacheEntry) {
	if p.objectRefs[entry.outputID] == 0 {
		if _, tracked := p.objectSizes[entry.outputID]; !tracked {
			p.objectSizes[entry.outputID] = entry.size
			p.totalBytes += entry.size
		}
	}
	p.objectRefs[entry.outputID]++
}

func (p *boundedCacheProgram) removeEntry(entry *boundedCacheEntry) error {
	if entry == nil {
		return nil
	}
	delete(p.entries, entry.actionID)
	if err := os.Remove(entry.metaPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("删除缓存索引失败: %w", err)
	}
	refs := p.objectRefs[entry.outputID] - 1
	if refs > 0 {
		p.objectRefs[entry.outputID] = refs
		return nil
	}
	delete(p.objectRefs, entry.outputID)
	size := p.objectSizes[entry.outputID]
	if _, pinned := p.pinned[entry.objectPath]; pinned {
		p.objectRefs[entry.outputID] = 0
		return nil
	}
	delete(p.objectSizes, entry.outputID)
	if err := os.Remove(entry.objectPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("删除缓存对象失败: %w", err)
	}
	p.totalBytes -= size
	if p.totalBytes < 0 {
		p.totalBytes = 0
	}
	return nil
}

func (p *boundedCacheProgram) evict(strict bool) error {
	if p.totalBytes <= p.maxBytes {
		return nil
	}
	entries := make([]*boundedCacheEntry, 0, len(p.entries))
	for _, entry := range p.entries {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].lastUsed.Equal(entries[j].lastUsed) {
			return entries[i].actionID < entries[j].actionID
		}
		return entries[i].lastUsed.Before(entries[j].lastUsed)
	})
	for _, entry := range entries {
		if p.totalBytes <= p.targetBytes {
			break
		}
		if _, pinned := p.pinned[entry.objectPath]; pinned {
			continue
		}
		if err := p.removeEntry(entry); err != nil {
			return err
		}
	}
	if strict && p.totalBytes > p.maxBytes {
		return fmt.Errorf("缓存有效工作集 %s 超过上限 %s", formatPublishSize(p.totalBytes), formatPublishSize(p.maxBytes))
	}
	return nil
}

func (p *boundedCacheProgram) removeOrphanObjects() error {
	if err := filepath.WalkDir(p.objectsDir, func(path string, item os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if item.IsDir() || !strings.HasSuffix(item.Name(), cacheProgObjectSuffix) {
			return nil
		}
		outputKey := strings.TrimSuffix(item.Name(), cacheProgObjectSuffix)
		if p.objectRefs[outputKey] > 0 {
			return nil
		}
		return p.removeOrphanObject(outputKey, path)
	}); err != nil {
		return err
	}

	for outputKey := range p.objectSizes {
		if p.objectRefs[outputKey] > 0 {
			continue
		}
		if err := p.removeOrphanObject(outputKey, p.objectPath(outputKey)); err != nil {
			return err
		}
	}
	return nil
}

func (p *boundedCacheProgram) removeOrphanObject(outputKey, path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	trackedSize, tracked := p.objectSizes[outputKey]
	delete(p.objectRefs, outputKey)
	delete(p.objectSizes, outputKey)
	if tracked {
		p.totalBytes -= trackedSize
		if p.totalBytes < 0 {
			p.totalBytes = 0
		}
	}
	return nil
}

func cleanupCacheTempFiles(root string) error {
	return filepath.WalkDir(root, func(path string, item os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if item.IsDir() || !strings.HasPrefix(item.Name(), cacheProgTempPrefix) {
			return nil
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	})
}

func atomicWriteCacheFile(path string, content []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), cacheProgTempPrefix)
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func validCacheProgHexID(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func (p *boundedCacheProgram) metadataPath(actionKey string) string {
	return filepath.Join(p.actionsDir, actionKey[:2], actionKey+cacheProgMetadataSuffix)
}

func (p *boundedCacheProgram) objectPath(outputKey string) string {
	return filepath.Join(p.objectsDir, outputKey[:2], outputKey+cacheProgObjectSuffix)
}

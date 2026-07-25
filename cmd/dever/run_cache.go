package main

import (
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	defaultRunCacheMaxBytes = int64(4 << 30)
	sharedBuildLockTimeout  = 30 * time.Minute
)

type runCacheOptions struct {
	dir         string
	maxBytes    int64
	targetBytes int64
}

type sharedBuildLock struct {
	file *os.File
}

func resolveRunCacheOptions(cacheDir, cacheMax string) (runCacheOptions, error) {
	maxBytes, err := parseByteSize(cacheMax)
	if err != nil {
		return runCacheOptions{}, fmt.Errorf("解析缓存上限失败: %w", err)
	}
	if maxBytes == 0 {
		return runCacheOptions{}, nil
	}
	cacheDir = strings.TrimSpace(cacheDir)
	if cacheDir == "" {
		userCacheDir, err := os.UserCacheDir()
		if err != nil {
			return runCacheOptions{}, fmt.Errorf("读取用户缓存目录失败，请使用 --cache-dir 指定: %w", err)
		}
		cacheDir = filepath.Join(userCacheDir, "dever", "go-build")
	} else if !filepath.IsAbs(cacheDir) {
		if callerDir := strings.TrimSpace(os.Getenv(callerDirEnv)); callerDir != "" {
			cacheDir = filepath.Join(callerDir, cacheDir)
		}
	}
	absoluteDir, err := filepath.Abs(cacheDir)
	if err != nil {
		return runCacheOptions{}, fmt.Errorf("解析缓存目录失败: %w", err)
	}
	return runCacheOptions{
		dir:         absoluteDir,
		maxBytes:    maxBytes,
		targetBytes: maxBytes - maxBytes/4,
	}, nil
}

func (o runCacheOptions) enabled() bool {
	return strings.TrimSpace(o.dir) != "" && o.maxBytes > 0
}

func (o runCacheOptions) buildEnvironment() (map[string]string, error) {
	if !o.enabled() {
		return nil, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("读取 dever 可执行文件失败: %w", err)
	}
	command, err := joinGoCacheProgArguments([]string{
		executable,
		"cache-prog",
		"--dir=" + o.dir,
		"--max-bytes=" + strconv.FormatInt(o.maxBytes, 10),
		"--target-bytes=" + strconv.FormatInt(o.targetBytes, 10),
	})
	if err != nil {
		return nil, err
	}
	nativeCacheDir := filepath.Join(o.dir, "native")
	if err := os.MkdirAll(nativeCacheDir, 0o700); err != nil {
		return nil, fmt.Errorf("创建 Go 原生缓存目录失败: %w", err)
	}
	return map[string]string{
		"GOCACHE":     nativeCacheDir,
		"GOCACHEPROG": command,
	}, nil
}

func acquireSharedBuildLock(cacheDir string, timeout time.Duration) (*sharedBuildLock, error) {
	lockPath := filepath.Join(cacheDir, ".build.lock")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return nil, fmt.Errorf("创建全局构建锁目录失败: %w", err)
	}
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("打开全局构建锁失败: %w", err)
	}
	locked := false
	defer func() {
		if !locked {
			_ = file.Close()
		}
	}()

	deadline := time.Now().Add(timeout)
	waitLogged := false
	for {
		err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			if err := writeBuildLockOwner(file); err != nil {
				_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
				return nil, err
			}
			locked = true
			return &sharedBuildLock{file: file}, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			return nil, fmt.Errorf("获取全局构建锁失败: %w", err)
		}

		ownerPID := readPIDFile(lockPath)
		if !waitLogged {
			if ownerPID > 0 {
				log.Printf("另一个 dever run 正在构建（pid=%d），等待全局构建锁", ownerPID)
			} else {
				log.Printf("另一个 dever run 正在构建，等待全局构建锁")
			}
			waitLogged = true
		}
		if time.Now().After(deadline) {
			if ownerPID > 0 {
				return nil, fmt.Errorf("等待全局构建锁超时，当前持有者 pid=%d", ownerPID)
			}
			return nil, errors.New("等待全局构建锁超时")
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func writeBuildLockOwner(file *os.File) error {
	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("清空全局构建锁失败: %w", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("定位全局构建锁失败: %w", err)
	}
	if _, err := fmt.Fprintf(file, "%d", os.Getpid()); err != nil {
		return fmt.Errorf("写入全局构建锁失败: %w", err)
	}
	return nil
}

func (l *sharedBuildLock) release() {
	if l == nil || l.file == nil {
		return
	}
	_ = syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	_ = l.file.Close()
	l.file = nil
}

func parseByteSize(value string) (int64, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return defaultRunCacheMaxBytes, nil
	}
	if normalized == "0" || normalized == "OFF" {
		return 0, nil
	}

	multipliers := []struct {
		suffix     string
		multiplier float64
	}{
		{suffix: "TIB", multiplier: 1 << 40},
		{suffix: "TB", multiplier: 1e12},
		{suffix: "GIB", multiplier: 1 << 30},
		{suffix: "GB", multiplier: 1e9},
		{suffix: "MIB", multiplier: 1 << 20},
		{suffix: "MB", multiplier: 1e6},
		{suffix: "KIB", multiplier: 1 << 10},
		{suffix: "KB", multiplier: 1e3},
		{suffix: "B", multiplier: 1},
	}
	for _, unit := range multipliers {
		if !strings.HasSuffix(normalized, unit.suffix) {
			continue
		}
		number := strings.TrimSpace(strings.TrimSuffix(normalized, unit.suffix))
		parsed, err := strconv.ParseFloat(number, 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < 0 || parsed > float64(math.MaxInt64)/unit.multiplier {
			return 0, fmt.Errorf("无效容量 %q", value)
		}
		return int64(parsed * unit.multiplier), nil
	}
	parsed, err := strconv.ParseInt(normalized, 10, 64)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("无效容量 %q", value)
	}
	return parsed, nil
}

func joinGoCacheProgArguments(arguments []string) (string, error) {
	quoted := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		value, err := quoteGoCacheProgArgument(argument)
		if err != nil {
			return "", err
		}
		quoted = append(quoted, value)
	}
	return strings.Join(quoted, " "), nil
}

func quoteGoCacheProgArgument(argument string) (string, error) {
	if !strings.ContainsAny(argument, " \t\n\r'\"") {
		return argument, nil
	}
	if !strings.Contains(argument, "'") {
		return "'" + argument + "'", nil
	}
	if !strings.Contains(argument, "\"") {
		return "\"" + argument + "\"", nil
	}
	return "", fmt.Errorf("GOCACHEPROG 参数同时包含单双引号，无法编码: %q", argument)
}

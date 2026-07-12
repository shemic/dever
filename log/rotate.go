package dlog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	bytesPerMegabyte = int64(1024 * 1024)
	backupTimeFormat = "20060102T150405.000000000Z"
)

type fileRotationConfig struct {
	maxSizeBytes int64
	maxBackups   int
}

type rotatingFileWriter struct {
	mu         sync.Mutex
	path       string
	maxSize    int64
	maxBackups int
	file       *os.File
	size       int64
}

func newRotatingFileWriter(path string, rotation fileRotationConfig) (*rotatingFileWriter, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("日志文件路径不能为空")
	}
	if rotation.maxSizeBytes <= 0 {
		return nil, fmt.Errorf("日志文件大小上限必须大于 0")
	}
	if rotation.maxBackups <= 0 {
		return nil, fmt.Errorf("日志备份数量必须大于 0")
	}

	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("创建日志目录 %q 失败: %w", filepath.Dir(path), err)
	}

	writer := &rotatingFileWriter{
		path:       path,
		maxSize:    rotation.maxSizeBytes,
		maxBackups: rotation.maxBackups,
	}
	if err := writer.open(); err != nil {
		return nil, err
	}
	return writer, nil
}

func (w *rotatingFileWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	if w.shouldRotate(len(data)) {
		if err := w.rotate(); err != nil {
			if w.file == nil {
				return 0, err
			}
			fmt.Fprintf(os.Stderr, "log: 滚动日志 %q 失败，继续写入当前文件: %v\n", w.path, err)
		}
	}

	written, err := w.file.Write(data)
	w.size += int64(written)
	return written, err
}

func (w *rotatingFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	w.size = 0
	return err
}

func (w *rotatingFileWriter) open() error {
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("打开日志文件 %q 失败: %w", w.path, err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return fmt.Errorf("读取日志文件 %q 状态失败: %w", w.path, err)
	}
	w.file = file
	w.size = info.Size()
	return nil
}

func (w *rotatingFileWriter) shouldRotate(nextSize int) bool {
	if nextSize <= 0 || w.size <= 0 {
		return false
	}
	if w.size >= w.maxSize {
		return true
	}
	return int64(nextSize) > w.maxSize-w.size
}

func (w *rotatingFileWriter) rotate() error {
	if err := w.file.Close(); err != nil {
		w.file = nil
		return w.reopenAfterFailure(fmt.Errorf("关闭日志文件 %q 失败: %w", w.path, err))
	}
	w.file = nil

	backup := logBackupPath(w.path, time.Now())
	if err := os.Rename(w.path, backup); err != nil {
		return w.reopenAfterFailure(fmt.Errorf("备份日志文件 %q 失败: %w", w.path, err))
	}
	if err := w.open(); err != nil {
		return w.restoreBackup(backup, err)
	}
	if err := w.cleanupBackups(); err != nil {
		fmt.Fprintf(os.Stderr, "log: 清理日志 %q 的旧备份失败: %v\n", w.path, err)
	}
	return nil
}

func (w *rotatingFileWriter) reopenAfterFailure(operationErr error) error {
	if err := w.open(); err != nil {
		return errors.Join(operationErr, fmt.Errorf("重新打开日志文件 %q 失败: %w", w.path, err))
	}
	return operationErr
}

func (w *rotatingFileWriter) restoreBackup(backup string, operationErr error) error {
	if err := os.Rename(backup, w.path); err != nil {
		return errors.Join(operationErr, fmt.Errorf("恢复日志备份 %q 失败: %w", backup, err))
	}
	if err := w.open(); err != nil {
		return errors.Join(operationErr, fmt.Errorf("重新打开恢复后的日志文件 %q 失败: %w", w.path, err))
	}
	return operationErr
}

func (w *rotatingFileWriter) cleanupBackups() error {
	directory := filepath.Dir(w.path)
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("读取日志目录 %q 失败: %w", directory, err)
	}

	prefix, extension := logBackupNameParts(w.path)
	backups := make([]string, 0)
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !isLogBackup(entry.Name(), prefix, extension) {
			continue
		}
		backups = append(backups, entry.Name())
	}
	if len(backups) <= w.maxBackups {
		return nil
	}

	sort.Strings(backups)
	var cleanupErr error
	for _, name := range backups[:len(backups)-w.maxBackups] {
		path := filepath.Join(directory, name)
		if err := os.Remove(path); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("删除旧日志备份 %q 失败: %w", path, err))
		}
	}
	return cleanupErr
}

func logBackupPath(path string, now time.Time) string {
	prefix, extension := logBackupNameParts(path)
	name := prefix + now.UTC().Format(backupTimeFormat) + extension
	return filepath.Join(filepath.Dir(path), name)
}

func logBackupNameParts(path string) (string, string) {
	extension := filepath.Ext(path)
	base := strings.TrimSuffix(filepath.Base(path), extension)
	return base + "-", extension
}

func isLogBackup(name, prefix, extension string) bool {
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, extension) {
		return false
	}
	timestamp := strings.TrimPrefix(name, prefix)
	timestamp = strings.TrimSuffix(timestamp, extension)
	_, err := time.Parse(backupTimeFormat, timestamp)
	return err == nil
}

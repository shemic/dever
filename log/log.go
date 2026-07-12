package dlog

import (
	"fmt"
	"io"
	stdlog "log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/shemic/dever/config"
)

var currentLevel atomic.Value

var (
	writerMu     sync.Mutex
	accessCloser io.Closer
	errorCloser  io.Closer
	accessOut    io.Writer = io.Discard
	errorOut     io.Writer = io.Discard

	errorLogger = stdlog.New(io.Discard, "", stdlog.LstdFlags)
)

func init() {
	currentLevel.Store("info")
}

// Configure 根据配置调整标准日志行为。
func Configure(cfg config.Log) {
	level := strings.ToLower(strings.TrimSpace(cfg.Level))
	if level == "" {
		level = "info"
	}
	currentLevel.Store(level)

	flags := stdlog.LstdFlags
	if cfg.Development {
		flags |= stdlog.Lshortfile
	}
	stdlog.SetFlags(flags)
	errorLogger.SetFlags(flags)

	enabled := true
	if cfg.Enabled != nil && !*cfg.Enabled {
		enabled = false
	}

	baseOutput := strings.ToLower(strings.TrimSpace(cfg.Output))
	if baseOutput == "" {
		baseOutput = "stdout"
	}

	var accessOverride *config.LogTarget
	if cfg.Access != nil {
		accessOverride = cfg.Access
	} else if baseOutput == "file" {
		if path := strings.TrimSpace(cfg.SuccessFile); path != "" {
			accessOverride = &config.LogTarget{
				Output:   "file",
				FilePath: path,
			}
		}
	}
	var errorOverride *config.LogTarget
	if cfg.Error != nil {
		errorOverride = cfg.Error
	} else if baseOutput == "file" {
		if path := strings.TrimSpace(cfg.ErrorFile); path != "" {
			errorOverride = &config.LogTarget{
				Output:   "file",
				FilePath: path,
			}
		}
	}

	accessFallback := strings.TrimSpace(cfg.SuccessFile)
	if accessFallback == "" {
		accessFallback = filepath.Join("log", "access.log")
	}
	errorFallback := strings.TrimSpace(cfg.ErrorFile)
	if errorFallback == "" {
		errorFallback = filepath.Join("log", "error.log")
	}

	accessTarget := selectTarget(baseOutput, cfg.FilePath, accessOverride, accessFallback)
	errorTarget := selectTarget(baseOutput, cfg.FilePath, errorOverride, errorFallback)
	rotation := rotationConfig(cfg)

	accessWriter, newAccessCloser, accessErr := resolveWriter(enabled, accessTarget, rotation)
	if accessErr != nil {
		fmt.Fprintf(os.Stderr, "log: 使用访问日志输出目标 %q 失败，已回退到标准输出: %v\n", accessTarget.output, accessErr)
		accessWriter = os.Stdout
		newAccessCloser = nil
	}
	errorWriter, newErrorCloser, errorErr := resolveWriter(enabled, errorTarget, rotation)
	if errorErr != nil {
		fmt.Fprintf(os.Stderr, "log: 使用错误日志输出目标 %q 失败，已回退到标准错误: %v\n", errorTarget.output, errorErr)
		errorWriter = os.Stderr
		newErrorCloser = nil
	}

	writerMu.Lock()
	defer writerMu.Unlock()

	if accessCloser != nil {
		_ = accessCloser.Close()
	}
	if errorCloser != nil && errorCloser != accessCloser {
		_ = errorCloser.Close()
	}

	accessCloser = newAccessCloser
	errorCloser = newErrorCloser
	accessOut = accessWriter
	errorOut = errorWriter

	stdlog.SetOutput(accessWriter)
	errorLogger.SetOutput(errorWriter)
}

// Level 返回当前日志级别。
func Level() string {
	if v := currentLevel.Load(); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return "info"
}

// Access 返回访问日志记录器。
func Access() *stdlog.Logger {
	return stdlog.Default()
}

// Error 返回错误日志记录器。
func Error() *stdlog.Logger {
	return errorLogger
}

type logTarget struct {
	output   string
	filePath string
}

func selectTarget(defaultOutput, defaultFile string, override *config.LogTarget, fallbackFile string) logTarget {
	output := strings.ToLower(strings.TrimSpace(defaultOutput))
	filePath := strings.TrimSpace(defaultFile)

	if override != nil {
		if v := strings.ToLower(strings.TrimSpace(override.Output)); v != "" {
			output = v
		}
		if v := strings.TrimSpace(override.FilePath); v != "" {
			filePath = v
		}
	}

	if output == "" {
		output = "stdout"
	}
	if output == "file" && filePath == "" {
		filePath = fallbackFile
	}
	return logTarget{
		output:   output,
		filePath: filePath,
	}
}

func resolveWriter(enabled bool, target logTarget, rotation fileRotationConfig) (io.Writer, io.Closer, error) {
	if !enabled {
		return io.Discard, nil, nil
	}

	switch target.output {
	case "", "stdout", "screen":
		return os.Stdout, nil, nil
	case "stderr":
		return os.Stderr, nil, nil
	case "file":
		path := target.filePath
		if path == "" {
			path = filepath.Join("log", "access.log")
		}
		writer, err := newRotatingFileWriter(path, rotation)
		if err != nil {
			return nil, nil, err
		}
		return writer, writer, nil
	case "off", "none", "disable", "disabled":
		return io.Discard, nil, nil
	default:
		return nil, nil, fmt.Errorf("未知的日志输出类型: %s", target.output)
	}
}

func rotationConfig(cfg config.Log) fileRotationConfig {
	maxSizeMB := cfg.MaxSizeMB
	if maxSizeMB <= 0 {
		maxSizeMB = config.DefaultLogMaxSizeMB
	}
	maxBackups := cfg.MaxBackups
	if maxBackups <= 0 {
		maxBackups = config.DefaultLogMaxBackups
	}
	return fileRotationConfig{
		maxSizeBytes: int64(maxSizeMB) * bytesPerMegabyte,
		maxBackups:   maxBackups,
	}
}

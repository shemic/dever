package dlog

import (
    "strings"
    "sync/atomic"

    stdlog "log"

    "github.com/shemic/dever/config"
)

var currentLevel atomic.Value

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

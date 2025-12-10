package orm

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/shemic/dever/config"
)

var (
	dbInitOnce sync.Once
	dbInitErr  error
)

// ensureDatabaseInitialized 在首次模型加载时初始化数据库连接与迁移配置。
func ensureDatabaseInitialized() error {
	dbInitOnce.Do(func() {
		cfg, err := config.Load("")
		if err != nil {
			dbInitErr = fmt.Errorf("读取数据库配置失败: %w", err)
			return
		}

		SetDefaultDatabase(cfg.Database.Default)
		EnableAutoMigrate(cfg.Database.Create)
		EnableSchemaPersistence(cfg.Database.Persist)
		EnableMigrationLog(cfg.Database.MigrationLog)

		if len(cfg.Database.Connections) == 0 {
			return
		}

		names := make([]string, 0, len(cfg.Database.Connections))
		for name := range cfg.Database.Connections {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, rawName := range names {
			conn := cfg.Database.Connections[rawName]
			target := strings.TrimSpace(rawName)
			if target == "" {
				target = cfg.Database.Default
			}
			if _, err := Init(target, ConfigFromDBConf(conn)); err != nil {
				dbInitErr = fmt.Errorf("初始化数据库 %q 失败: %w", target, err)
				return
			}
		}
	})
	return dbInitErr
}

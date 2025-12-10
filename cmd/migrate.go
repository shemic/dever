package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/shemic/dever/config"
	"github.com/shemic/dever/orm"
)

// RunMigrations 读取 data/table 下记录的表结构，应用到指定数据库。
func RunMigrations(projectRoot, target string) error {
	cfgPath := filepath.Join(projectRoot, config.DefaultPath)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("读取配置失败: %w", err)
	}

	orm.SetDefaultDatabase(cfg.Database.Default)
	orm.EnableAutoMigrate(false)

	connCfg, ok := cfg.Database.Connections[target]
	if !ok {
		defaultCfg, ok := cfg.Database.Connections[cfg.Database.Default]
		if !ok {
			return fmt.Errorf("未找到数据库配置 %q，且默认配置缺失", target)
		}
		connCfg = defaultCfg
		if strings.TrimSpace(connCfg.DBName) == "" {
			connCfg.DBName = target
		} else {
			connCfg.DBName = strings.TrimSpace(connCfg.DBName)
		}
	}
	if strings.TrimSpace(connCfg.DBName) == "" {
		connCfg.DBName = target
	}

	cfgMap := orm.ConfigFromDBConf(connCfg)
	if _, err := orm.Init(target, cfgMap); err != nil {
		return fmt.Errorf("初始化数据库连接失败: %w", err)
	}
	defer func() {
		_ = orm.Close(target)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	statements, err := orm.ApplyRecordedSchemas(ctx, target)
	if err != nil {
		return fmt.Errorf("应用表结构失败: %w", err)
	}

	if len(statements) == 0 {
		fmt.Printf("数据库 %s 已经是最新结构\n", target)
		return nil
	}

	fmt.Printf("数据库 %s 已应用 %d 条结构变更：\n", target, len(statements))
	for _, stmt := range statements {
		fmt.Printf("  - %s\n", stmt)
	}
	return nil
}

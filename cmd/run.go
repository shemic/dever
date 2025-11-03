package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/mattn/go-runewidth"

	dever "github.com/shemic/dever"
	"github.com/shemic/dever/config"
	dlog "github.com/shemic/dever/log"
	"github.com/shemic/dever/orm"
	"github.com/shemic/dever/server"
	serverhttp "github.com/shemic/dever/server/http"
)

// Run 启动 HTTP 服务，加载配置并注册项目路由。
func Run(register func(server.Server)) error {
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("读取配置失败: %w", err)
	}

	orm.SetDefaultDatabase(cfg.Database.Default)
	orm.EnableAutoMigrate(cfg.Database.Create)
	orm.EnableSchemaPersistence(cfg.Database.Persist)
	orm.EnableMigrationLog(cfg.Database.MigrationLog)

	dbInitialized := false
	if len(cfg.Database.Connections) > 0 {
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
			if _, err := orm.Init(target, makeORMConfig(conn)); err != nil {
				return fmt.Errorf("初始化数据库 %q 失败: %w", target, err)
			}
			dbInitialized = true
		}
	}
	if dbInitialized {
		defer func() {
			_ = orm.CloseAll()
		}()
	}

	dlog.Configure(cfg.Log)

	fiberCfg := makeFiberConfig(cfg.HTTP)
	app := serverhttp.NewWithConfig(fiberCfg)
	if register != nil && (!fiberCfg.Prefork || fiber.IsChild()) {
		register(app)
	}

	addr := cfg.HTTP.Addr()
	if !fiber.IsChild() {
		printStartupBanner(cfg.HTTP.AppName, addr)
	}

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- app.Run(addr)
	}()

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErrCh:
		if err != nil {
			return fmt.Errorf("HTTP 服务启动失败: %w", err)
		}
		return nil
	case <-sigCtx.Done():
		shutdownTimeout := cfg.HTTP.ShutdownTimeout.Duration()
		shutdownCtx := context.Background()
		var cancel context.CancelFunc
		if shutdownTimeout > 0 {
			shutdownCtx, cancel = context.WithTimeout(context.Background(), shutdownTimeout)
		}
		if cancel != nil {
			defer cancel()
		}

		if err := app.Shutdown(shutdownCtx); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return fmt.Errorf("HTTP 服务优雅停机超时: %w", err)
			}
			if !errors.Is(err, context.Canceled) {
				return fmt.Errorf("HTTP 服务优雅停机失败: %w", err)
			}
		}

		if err := <-serverErrCh; err != nil {
			return fmt.Errorf("HTTP 服务退出异常: %w", err)
		}
		return nil
	}
}

func makeFiberConfig(httpCfg config.HTTP) fiber.Config {
	conf := fiber.Config{
		DisableStartupMessage: true, // 统一关闭 Fiber 自带的启动输出
	}
	if !httpCfg.EnableTuning {
		return conf
	}

	conf.AppName = httpCfg.AppName
	conf.Prefork = httpCfg.Prefork
	conf.StrictRouting = httpCfg.StrictRouting
	conf.CaseSensitive = httpCfg.CaseSensitive
	conf.ReduceMemoryUsage = httpCfg.ReduceMemoryUsage
	conf.ServerHeader = httpCfg.ServerHeader
	conf.ReadTimeout = httpCfg.ReadTimeout.Duration()
	conf.WriteTimeout = httpCfg.WriteTimeout.Duration()
	conf.IdleTimeout = httpCfg.IdleTimeout.Duration()
	if httpCfg.BodyLimit > 0 {
		conf.BodyLimit = httpCfg.BodyLimit
	}
	if httpCfg.Concurrency > 0 {
		conf.Concurrency = httpCfg.Concurrency
	}
	return conf
}

func printStartupBanner(appName, addr string) {
	name := strings.TrimSpace(appName)
	if name == "" {
		name = dever.Name
	}
	header := fmt.Sprintf("应用：%s", name)
	subheader := fmt.Sprintf("框架：基于 %s 构建", dever.FullName)
	pid := os.Getpid()
	hostname, _ := os.Hostname()

	lines := []string{
		header,
		subheader,
		fmt.Sprintf("Listening: http://%s", addr),
		fmt.Sprintf("Hostname : %s", hostname),
		fmt.Sprintf("PID      : %d", pid),
	}

	width := 0
	for _, line := range lines {
		if w := runewidth.StringWidth(line); w > width {
			width = w
		}
	}

	border := strings.Repeat("─", width+2)
	fmt.Printf("┌%s┐\n", border)
	for _, line := range lines {
		padding := width - runewidth.StringWidth(line)
		if padding < 0 {
			padding = 0
		}
		fmt.Printf("│ %s%s │\n", line, strings.Repeat(" ", padding))
	}
	fmt.Printf("└%s┘\n", border)
}

func makeORMConfig(dbCfg config.DBConf) orm.Config {
	params := map[string]string{}
	for k, v := range dbCfg.Params {
		params[k] = v
	}
	return orm.Config{
		Driver:            dbCfg.Driver,
		Host:              dbCfg.Host,
		User:              dbCfg.User,
		Password:          dbCfg.Pwd,
		DBName:            dbCfg.DBName,
		Path:              dbCfg.Path,
		Params:            params,
		DSN:               "",
		MaxOpenConns:      dbCfg.MaxOpenConns,
		MaxIdleConns:      dbCfg.MaxIdleConns,
		ConnMaxLifetime:   dbCfg.ConnMaxLifetime.Duration(),
		ConnMaxIdleTime:   dbCfg.ConnMaxIdleTime.Duration(),
		HealthCheckPeriod: dbCfg.HealthCheckPeriod.Duration(),
	}
}

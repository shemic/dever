package orm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
)

var (
	mu              sync.RWMutex
	databases       = map[string]*sqlx.DB{}
	defaultDatabase = "default"
)

// Init 根据配置初始化命名数据库连接。
func Init(name string, cfg Config) (*sqlx.DB, error) {
	if strings.TrimSpace(name) == "" {
		name = currentDefaultDatabase()
	}
	driver := cfg.driverName()
	if driver != "sqlite3" && strings.TrimSpace(cfg.DBName) == "" && strings.TrimSpace(cfg.DSN) == "" {
		return nil, fmt.Errorf("orm: database %q requires dbname", name)
	}
	if err := ensureDatabaseExists(driver, cfg); err != nil {
		return nil, fmt.Errorf("orm: ensure database %q failed: %w", name, err)
	}

	dsn, err := cfg.buildDSN()
	if err != nil {
		return nil, fmt.Errorf("orm: build dsn for %q failed: %w", name, err)
	}

	db, err := sqlx.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("orm: open %q failed: %w", name, err)
	}

	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeoutOrDefault(cfg.HealthCheckPeriod))
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("orm: ping %q failed: %w", name, err)
	}

	mu.Lock()
	defer mu.Unlock()
	if existing, ok := databases[name]; ok {
		_ = existing.Close()
	}
	databases[name] = db
	return db, nil
}

// InitDefault 初始化默认数据库连接。
func InitDefault(cfg Config) (*sqlx.DB, error) {
	return Init(currentDefaultDatabase(), cfg)
}

// Get 返回命名数据库连接；若未指定名称则返回默认连接。
func Get(name ...string) (*sqlx.DB, error) {
	target := currentDefaultDatabase()
	if len(name) > 0 && name[0] != "" {
		target = name[0]
	}
	mu.RLock()
	db, ok := databases[target]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("orm: database %q not initialized", target)
	}
	return db, nil
}

// Close 关闭指定数据库连接。
func Close(name string) error {
	if strings.TrimSpace(name) == "" {
		name = currentDefaultDatabase()
	}
	mu.Lock()
	db, ok := databases[name]
	if ok {
		delete(databases, name)
	}
	mu.Unlock()
	if !ok {
		return nil
	}
	return db.Close()
}

// CloseAll 关闭所有数据库连接。
func CloseAll() error {
	mu.Lock()
	defer mu.Unlock()
	var errs []error
	for name, db := range databases {
		if err := db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
		delete(databases, name)
	}
	return errors.Join(errs...)
}

// Ping 检查数据库连接是否可用。
func Ping(ctx context.Context, name ...string) error {
	db, err := Get(name...)
	if err != nil {
		return err
	}
	ctx, cancel := withOptionalTimeout(ctx, 5*time.Second)
	defer cancel()
	return db.PingContext(ctx)
}

func timeoutOrDefault(d time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return 5 * time.Second
}

// SetDefaultDatabase 更新默认数据库名称，在初始化连接前调用。
func SetDefaultDatabase(name string) {
	value := strings.TrimSpace(name)
	if value == "" {
		value = "default"
	}
	mu.Lock()
	defaultDatabase = value
	mu.Unlock()
}

func currentDefaultDatabase() string {
	mu.RLock()
	name := defaultDatabase
	mu.RUnlock()
	if strings.TrimSpace(name) == "" {
		return "default"
	}
	return name
}

func withOptionalTimeout(ctx context.Context, fallback time.Duration) (context.Context, context.CancelFunc) {
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) > 0 {
		return context.WithCancel(ctx)
	}
	if fallback <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, fallback)
}

func ensureDatabaseExists(driver string, cfg Config) error {
	switch driver {
	case "mysql":
		return ensureMySQLDatabase(cfg)
	case "pgx":
		return ensurePostgresDatabase(cfg)
	case "sqlite3":
		return ensureSQLiteDatabase(cfg)
	default:
		if strings.TrimSpace(cfg.DSN) == "" {
			return fmt.Errorf("orm: driver %s requires dsn", driver)
		}
		return nil
	}
}

func ensureMySQLDatabase(cfg Config) error {
	if strings.TrimSpace(cfg.DBName) == "" {
		return fmt.Errorf("orm: mysql dbname required")
	}
	adminCfg := cfg
	adminCfg.DBName = ""
	adminCfg.DSN = ""
	adminDSN, err := adminCfg.buildDSNWithoutDatabase()
	if err != nil {
		return err
	}
	db, err := sqlx.Open(adminCfg.driverName(), adminDSN)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeoutOrDefault(cfg.HealthCheckPeriod))
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return err
	}

	stmt := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", quoteMySQLIdentifier(cfg.DBName))
	_, err = db.ExecContext(ctx, stmt)
	return err
}

func ensurePostgresDatabase(cfg Config) error {
	if strings.TrimSpace(cfg.DBName) == "" {
		return fmt.Errorf("orm: postgres dbname required")
	}
	adminCfg := cfg
	adminCfg.DBName = ""
	adminCfg.DSN = ""
	adminDSN, err := adminCfg.buildDSNWithoutDatabase()
	if err != nil {
		return err
	}
	db, err := sqlx.Open(adminCfg.driverName(), adminDSN)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeoutOrDefault(cfg.HealthCheckPeriod))
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return err
	}

	var exists bool
	query := "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)"
	if err := db.GetContext(ctx, &exists, query, cfg.DBName); err != nil {
		return err
	}
	if exists {
		return nil
	}
	stmt := fmt.Sprintf("CREATE DATABASE %s", quotePostgresIdentifier(cfg.DBName))
	_, err = db.ExecContext(ctx, stmt)
	return err
}

func ensureSQLiteDatabase(cfg Config) error {
	dsn, err := cfg.buildDSN()
	if err != nil {
		return err
	}
	path := dsn
	if strings.HasPrefix(path, "file:") {
		if idx := strings.Index(path, "?"); idx >= 0 {
			path = path[len("file:"):idx]
		} else {
			path = path[len("file:"):]
		}
	}
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("orm: sqlite path required")
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func quoteMySQLIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func quotePostgresIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

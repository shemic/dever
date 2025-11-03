package orm

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Config 描述数据库连接的通用配置。
type Config struct {
	Driver            string            `mapstructure:"driver"`
	Host              string            `mapstructure:"host"`
	User              string            `mapstructure:"user"`
	Password          string            `mapstructure:"password"`
	DBName            string            `mapstructure:"dbname"`
	Path              string            `mapstructure:"path"`
	Params            map[string]string `mapstructure:"params"`
	DSN               string            `mapstructure:"dsn"`
	MaxOpenConns      int               `mapstructure:"maxOpenConns"`
	MaxIdleConns      int               `mapstructure:"maxIdleConns"`
	ConnMaxLifetime   time.Duration     `mapstructure:"connMaxLifetime"`
	ConnMaxIdleTime   time.Duration     `mapstructure:"connMaxIdleTime"`
	HealthCheckPeriod time.Duration     `mapstructure:"healthCheckPeriod"`
}

func (c Config) driverName() string {
	switch strings.ToLower(strings.TrimSpace(c.Driver)) {
	case "", "mysql", "mariadb":
		return "mysql"
	case "postgres", "postgresql", "pgsql":
		return "pgx"
	case "sqlite", "sqlite3":
		return "sqlite3"
	default:
		return c.Driver
	}
}

func (c Config) buildDSN() (string, error) {
	return c.buildDSNWithDatabase(true)
}

func (c Config) buildDSNWithoutDatabase() (string, error) {
	return c.buildDSNWithDatabase(false)
}

func (c Config) buildDSNWithDatabase(includeDB bool) (string, error) {
	if strings.TrimSpace(c.DSN) != "" {
		return c.DSN, nil
	}

	driver := c.driverName()
	switch driver {
	case "mysql":
		if c.Host == "" {
			return "", fmt.Errorf("orm: mysql host required")
		}
		if c.User == "" {
			return "", fmt.Errorf("orm: mysql user required")
		}
		params := defaultsForParams(c.Params, map[string]string{
			"charset":   "utf8mb4",
			"parseTime": "true",
			"loc":       "Local",
		})
		dbName := ""
		if includeDB && c.DBName != "" {
			dbName = "/" + c.DBName
		} else {
			dbName = "/"
		}
		paramStr := encodeParams(params)
		if paramStr != "" {
			return fmt.Sprintf("%s:%s@tcp(%s)%s?%s",
				c.User,
				c.Password,
				c.Host,
				dbName,
				paramStr,
			), nil
		}
		return fmt.Sprintf("%s:%s@tcp(%s)%s",
			c.User,
			c.Password,
			c.Host,
			dbName,
		), nil
	case "pgx":
		if c.Host == "" {
			return "", fmt.Errorf("orm: postgres host required")
		}
		if c.User == "" {
			return "", fmt.Errorf("orm: postgres user required")
		}
		dbName := c.DBName
		if !includeDB || dbName == "" {
			dbName = "postgres"
		}
		params := defaultsForParams(c.Params, nil)
		u := url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(c.User, c.Password),
			Host:   c.Host,
			Path:   "/" + dbName,
		}
		q := url.Values{}
		for k, v := range params {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
		return u.String(), nil
	case "sqlite3":
		path := strings.TrimSpace(c.Path)
		if path == "" {
			path = strings.TrimSpace(c.Host)
		}
		if path == "" {
			path = c.DBName
		}
		if path == "" {
			return "", fmt.Errorf("orm: sqlite path required")
		}
		params := defaultsForParams(c.Params, map[string]string{
			"_busy_timeout": "5000",
		})
		if len(params) == 0 {
			return path, nil
		}
		return fmt.Sprintf("file:%s?%s", path, encodeParams(params)), nil
	default:
		if c.DSN == "" {
			return "", fmt.Errorf("orm: driver %q requires dsn", driver)
		}
		return c.DSN, nil
	}
}

func defaultsForParams(params, defaults map[string]string) map[string]string {
	if defaults == nil {
		defaults = map[string]string{}
	}
	result := make(map[string]string, len(defaults))
	for k, v := range defaults {
		result[k] = v
	}
	for k, v := range params {
		result[strings.TrimSpace(k)] = v
	}
	return result
}

func encodeParams(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, fmt.Sprintf("%s=%s", url.QueryEscape(key), url.QueryEscape(params[key])))
	}
	return strings.Join(values, "&")
}

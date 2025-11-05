package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultPath 默认配置文件位置。
	DefaultPath = "config/setting.json"
)

// Duration 支持从字符串（如 "10s"）或数值（表示纳秒）解析到 time.Duration。
type Duration time.Duration

// UnmarshalJSON 将 JSON 中的字符串或数字转换为 Duration。
func (d *Duration) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		*d = 0
		return nil
	}
	if b[0] == '"' {
		str, err := strconv.Unquote(string(b))
		if err != nil {
			return fmt.Errorf("解析 Duration 字符串失败: %w", err)
		}
		dur, err := time.ParseDuration(str)
		if err != nil {
			return fmt.Errorf("解析 Duration 失败: %w", err)
		}
		*d = Duration(dur)
		return nil
	}

	var nanos int64
	if err := json.Unmarshal(b, &nanos); err != nil {
		return fmt.Errorf("解析 Duration 数值失败: %w", err)
	}
	*d = Duration(time.Duration(nanos))
	return nil
}

// Duration 返回 time.Duration 形式的值。
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// App 定义应用配置结构。
type App struct {
	Log      Log      `json:"log"`
	HTTP     HTTP     `json:"http"`
	Database Database `json:"database"`
	Redis    Redis    `json:"redis"`
}

// Log 表示日志相关配置。
type Log struct {
	Level       string `json:"level"`
	Encoding    string `json:"encoding"`
	Development bool   `json:"development"`
	Enabled     *bool  `json:"enabled,omitempty"`
	Output      string `json:"output"`
	FilePath    string `json:"filePath"`
	SuccessFile string `json:"successFile"`
	ErrorFile   string `json:"errorFile"`
	Access      *LogTarget `json:"access,omitempty"`
	Error       *LogTarget `json:"error,omitempty"`
}

// LogTarget 表示访问或错误日志的输出目标。
type LogTarget struct {
	Output   string `json:"output"`
	FilePath string `json:"filePath"`
}

// HTTP 表示 HTTP 服务配置。
type HTTP struct {
	Host              string   `json:"host"`
	Port              int      `json:"port"`
	ShutdownTimeout   Duration `json:"shutdownTimeout"`
	AppName           string   `json:"appName"`
	EnableTuning      bool     `json:"enableTuning"`
	Prefork           bool     `json:"prefork"`
	StrictRouting     bool     `json:"strictRouting"`
	CaseSensitive     bool     `json:"caseSensitive"`
	ReduceMemoryUsage bool     `json:"reduceMemoryUsage"`
	ServerHeader      string   `json:"serverHeader"`
	BodyLimit         int      `json:"bodyLimit"`
	Concurrency       int      `json:"concurrency"`
	ReadTimeout       Duration `json:"readTimeout"`
	WriteTimeout      Duration `json:"writeTimeout"`
	IdleTimeout       Duration `json:"idleTimeout"`
}

// Addr 构造监听地址。
func (h HTTP) Addr() string {
	host := h.Host
	if host == "" {
		host = "0.0.0.0"
	}
	if h.Port <= 0 {
		return host
	}
	return net.JoinHostPort(host, strconv.Itoa(h.Port))
}

// Database 表示数据库集合配置。
type Database struct {
	Create      bool              `json:"create"`
	Default     string            `json:"default"`
	Connections map[string]DBConf `json:"connections"`
	Persist     bool              `json:"persist"`
	MigrationLog bool             `json:"migrationLog"`
}

func (d *Database) UnmarshalJSON(data []byte) error {
	type rawEntry = json.RawMessage
	var raw map[string]rawEntry
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if d.Connections == nil {
		d.Connections = make(map[string]DBConf)
	}
	d.Persist = true
	d.MigrationLog = false
	for key, value := range raw {
		switch key {
		case "create":
			if err := json.Unmarshal(value, &d.Create); err != nil {
				return err
			}
		case "default":
			if len(value) > 0 && value[0] == '{' {
				var conf DBConf
				if err := json.Unmarshal(value, &conf); err != nil {
					return err
				}
				d.Connections["default"] = conf
				d.Default = "default"
			} else if len(value) > 0 {
				if err := json.Unmarshal(value, &d.Default); err != nil {
					return err
				}
			}
		case "persist":
			if err := json.Unmarshal(value, &d.Persist); err != nil {
				return err
			}
		case "migrationLog":
			if err := json.Unmarshal(value, &d.MigrationLog); err != nil {
				return err
			}
		default:
			var conf DBConf
			if err := json.Unmarshal(value, &conf); err != nil {
				return err
			}
			d.Connections[key] = conf
		}
	}
	if strings.TrimSpace(d.Default) == "" {
		if _, ok := d.Connections["default"]; ok {
			d.Default = "default"
		} else if len(d.Connections) == 1 {
			for name := range d.Connections {
				d.Default = name
			}
		} else if len(d.Connections) == 0 {
			d.Default = "default"
		}
	}
	return nil
}

// DBConf 表示数据库配置。
type DBConf struct {
	Driver            string            `json:"driver"`
	Host              string            `json:"host"`
	User              string            `json:"user"`
	Pwd               string            `json:"pwd"`
	DBName            string            `json:"dbname"`
	Path              string            `json:"path"`
	Params            map[string]string `json:"params"`
	MaxOpenConns      int               `json:"maxOpenConns"`
	MaxIdleConns      int               `json:"maxIdleConns"`
	ConnMaxLifetime   Duration          `json:"connMaxLifetime"`
	ConnMaxIdleTime   Duration          `json:"connMaxIdleTime"`
	HealthCheckPeriod Duration          `json:"healthCheckPeriod"`
}

// Redis 表示 Redis 相关配置。
type Redis struct {
	Enable       bool     `json:"enable"`
	Addr         string   `json:"addr"`
	Username     string   `json:"username"`
	Password     string   `json:"password"`
	DB           int      `json:"db"`
	Prefix       string   `json:"prefix"`
	PoolSize     int      `json:"poolSize"`
	MinIdleConns int      `json:"minIdleConns"`
	MaxRetries   int      `json:"maxRetries"`
	DialTimeout  Duration `json:"dialTimeout"`
	ReadTimeout  Duration `json:"readTimeout"`
	WriteTimeout Duration `json:"writeTimeout"`
	PingTimeout  Duration `json:"pingTimeout"`
	UseTLS       bool     `json:"useTLS"`
}

// Load 从指定路径读取配置。
func Load(path string) (*App, error) {
	if path == "" {
		path = DefaultPath
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("解析配置路径失败: %w", err)
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("读取配置失败 (%s): %w", abs, err)
	}

	var cfg App
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	cfg.applyDefaults()
	return &cfg, nil
}

func (c *App) applyDefaults() {
	if strings.TrimSpace(c.Log.Output) == "" {
		c.Log.Output = "stdout"
	}
	if strings.TrimSpace(c.Log.SuccessFile) == "" {
		c.Log.SuccessFile = filepath.Join("log", "access.log")
	}
	if strings.TrimSpace(c.Log.ErrorFile) == "" {
		c.Log.ErrorFile = filepath.Join("log", "error.log")
	}
	if strings.TrimSpace(c.Log.FilePath) == "" {
		c.Log.FilePath = c.Log.SuccessFile
	}
	if c.HTTP.Port == 0 {
		c.HTTP.Port = 8080
	}
	if c.HTTP.Host == "" {
		c.HTTP.Host = "0.0.0.0"
	}
	if c.HTTP.ShutdownTimeout <= 0 {
		c.HTTP.ShutdownTimeout = Duration(10 * time.Second)
	}
	if strings.TrimSpace(c.HTTP.AppName) == "" {
		c.HTTP.AppName = "Dever"
	}
	if c.HTTP.BodyLimit < 0 {
		c.HTTP.BodyLimit = 0
	}
	if c.HTTP.Concurrency < 0 {
		c.HTTP.Concurrency = 0
	}
	if c.HTTP.ReadTimeout < 0 {
		c.HTTP.ReadTimeout = 0
	}
	if c.HTTP.WriteTimeout < 0 {
		c.HTTP.WriteTimeout = 0
	}
	if c.HTTP.IdleTimeout < 0 {
		c.HTTP.IdleTimeout = 0
	}
	if strings.TrimSpace(c.Database.Default) == "" {
		c.Database.Default = "default"
	}
	if c.Database.Connections == nil {
		c.Database.Connections = make(map[string]DBConf)
	}
	for name, conf := range c.Database.Connections {
		if conf.Params == nil {
			conf.Params = map[string]string{}
		}
		if conf.MaxOpenConns < 0 {
			conf.MaxOpenConns = 0
		}
		if conf.MaxIdleConns < 0 {
			conf.MaxIdleConns = 0
		}
		if conf.ConnMaxLifetime < 0 {
			conf.ConnMaxLifetime = 0
		}
		if conf.ConnMaxIdleTime < 0 {
			conf.ConnMaxIdleTime = 0
		}
		if conf.HealthCheckPeriod < 0 {
			conf.HealthCheckPeriod = 0
		}
		if strings.TrimSpace(conf.Driver) == "" {
			conf.Driver = "mysql"
		}
		if strings.TrimSpace(conf.DBName) == "" {
			conf.DBName = name
		}
		if strings.TrimSpace(conf.Host) == "" && !isSQLite(conf.Driver) {
			conf.Host = "127.0.0.1:3306"
		}
		c.Database.Connections[name] = conf
	}
}

func isSQLite(driver string) bool {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "sqlite", "sqlite3":
		return true
	default:
		return false
	}
}

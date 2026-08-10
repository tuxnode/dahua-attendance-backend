package config

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const (
	defaultHTTPAddr              = ":8080"
	defaultHTTPMaxBodyBytes      = 10 << 20
	defaultHTTPReadHeaderTimeout = 5 * time.Second
	defaultHTTPReadTimeout       = 15 * time.Second
	defaultHTTPWriteTimeout      = 5 * time.Second
	defaultHTTPIdleTimeout       = 60 * time.Second
	defaultHTTPShutdownTimeout   = 10 * time.Second
	defaultDatabaseDriver        = "mysql"
	defaultDatabaseMaxOpenConns  = 10
	defaultDatabaseMaxIdleConns  = 5
	defaultDatabaseConnLifetime  = 30 * time.Minute
	defaultDatabaseConnectTime   = 5 * time.Second
	defaultLogLevel              = "info"
)

var (
	globalMu     sync.RWMutex
	globalConfig *Config
)

type Config struct {
	App      AppConfig      `toml:"app"`
	HTTP     HTTPConfig     `toml:"http"`
	Database DatabaseConfig `toml:"database"`
	Log      LogConfig      `toml:"log"`
}

type AppConfig struct {
	Name string `toml:"name"`
	Env  string `toml:"env"`
}

type HTTPConfig struct {
	Addr              string   `toml:"addr"`
	MaxBodyBytes      int64    `toml:"max_body_bytes"`
	ReadHeaderTimeout Duration `toml:"read_header_timeout"`
	ReadTimeout       Duration `toml:"read_timeout"`
	WriteTimeout      Duration `toml:"write_timeout"`
	IdleTimeout       Duration `toml:"idle_timeout"`
	ShutdownTimeout   Duration `toml:"shutdown_timeout"`
}

type DatabaseConfig struct {
	Driver          string   `toml:"driver"`
	DSN             string   `toml:"dsn"`
	MaxOpenConns    int      `toml:"max_open_conns"`
	MaxIdleConns    int      `toml:"max_idle_conns"`
	ConnMaxLifetime Duration `toml:"conn_max_lifetime"`
	ConnectTimeout  Duration `toml:"connect_timeout"`
}

type LogConfig struct {
	Level    string `toml:"level"`
	FilePath string `toml:"file_path"`
}

type Duration time.Duration

func Load(filePath string) (*Config, error) {
	if strings.TrimSpace(filePath) == "" {
		return nil, fmt.Errorf("config file path is required")
	}

	cfg := defaultConfig()
	if err := cfg.loadFromFile(filePath); err != nil {
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func Init(filePath string) (*Config, error) {
	cfg, err := Load(filePath)
	if err != nil {
		return nil, err
	}

	globalMu.Lock()
	globalConfig = cfg
	globalMu.Unlock()

	return cfg, nil
}

func Get() *Config {
	globalMu.RLock()
	defer globalMu.RUnlock()

	if globalConfig == nil {
		panic("config is not initialized, call config.Init() first")
	}

	return globalConfig
}

func (d Duration) Std() time.Duration {
	return time.Duration(d)
}

func (d Duration) String() string {
	return time.Duration(d).String()
}

func (d *Duration) UnmarshalText(text []byte) error {
	value := strings.TrimSpace(string(text))
	if value == "" {
		*d = 0
		return nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", value, err)
	}

	*d = Duration(duration)
	return nil
}

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

func defaultConfig() *Config {
	return &Config{
		HTTP: HTTPConfig{
			Addr:              defaultHTTPAddr,
			MaxBodyBytes:      defaultHTTPMaxBodyBytes,
			ReadHeaderTimeout: Duration(defaultHTTPReadHeaderTimeout),
			ReadTimeout:       Duration(defaultHTTPReadTimeout),
			WriteTimeout:      Duration(defaultHTTPWriteTimeout),
			IdleTimeout:       Duration(defaultHTTPIdleTimeout),
			ShutdownTimeout:   Duration(defaultHTTPShutdownTimeout),
		},
		Database: DatabaseConfig{
			Driver:          defaultDatabaseDriver,
			MaxOpenConns:    defaultDatabaseMaxOpenConns,
			MaxIdleConns:    defaultDatabaseMaxIdleConns,
			ConnMaxLifetime: Duration(defaultDatabaseConnLifetime),
			ConnectTimeout:  Duration(defaultDatabaseConnectTime),
		},
		Log: LogConfig{
			Level: defaultLogLevel,
		},
	}
}

func (c *Config) loadFromFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read config file %q: %w", filePath, err)
	}

	if err := toml.Unmarshal(data, c); err != nil {
		return fmt.Errorf("unmarshal toml config %q: %w", filePath, err)
	}

	return nil
}

func (c *Config) validate() error {
	if strings.TrimSpace(c.App.Name) == "" {
		return fmt.Errorf("config error: app.name cannot be empty")
	}
	if strings.TrimSpace(c.App.Env) == "" {
		c.App.Env = "dev"
	}

	if strings.TrimSpace(c.HTTP.Addr) == "" {
		c.HTTP.Addr = defaultHTTPAddr
	}
	if c.HTTP.MaxBodyBytes <= 0 {
		c.HTTP.MaxBodyBytes = defaultHTTPMaxBodyBytes
	}
	if c.HTTP.ReadHeaderTimeout <= 0 {
		c.HTTP.ReadHeaderTimeout = Duration(defaultHTTPReadHeaderTimeout)
	}
	if c.HTTP.ReadTimeout <= 0 {
		c.HTTP.ReadTimeout = Duration(defaultHTTPReadTimeout)
	}
	if c.HTTP.WriteTimeout <= 0 {
		c.HTTP.WriteTimeout = Duration(defaultHTTPWriteTimeout)
	}
	if c.HTTP.IdleTimeout <= 0 {
		c.HTTP.IdleTimeout = Duration(defaultHTTPIdleTimeout)
	}
	if c.HTTP.ShutdownTimeout <= 0 {
		c.HTTP.ShutdownTimeout = Duration(defaultHTTPShutdownTimeout)
	}

	if strings.TrimSpace(c.Database.Driver) == "" {
		c.Database.Driver = defaultDatabaseDriver
	}
	if strings.TrimSpace(c.Database.DSN) == "" {
		return fmt.Errorf("config error: database.dsn cannot be empty")
	}
	if c.Database.MaxOpenConns <= 0 {
		c.Database.MaxOpenConns = defaultDatabaseMaxOpenConns
	}
	if c.Database.MaxIdleConns <= 0 {
		c.Database.MaxIdleConns = defaultDatabaseMaxIdleConns
	}
	if c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		c.Database.MaxIdleConns = c.Database.MaxOpenConns
	}
	if c.Database.ConnMaxLifetime <= 0 {
		c.Database.ConnMaxLifetime = Duration(defaultDatabaseConnLifetime)
	}
	if c.Database.ConnectTimeout <= 0 {
		c.Database.ConnectTimeout = Duration(defaultDatabaseConnectTime)
	}

	c.Log.Level = strings.ToLower(strings.TrimSpace(c.Log.Level))
	if c.Log.Level == "" {
		c.Log.Level = defaultLogLevel
	}
	if !validLogLevel(c.Log.Level) {
		return fmt.Errorf("config error: unsupported log.level %q", c.Log.Level)
	}

	return nil
}

func validLogLevel(level string) bool {
	switch level {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}

func resetForTest() {
	globalMu.Lock()
	defer globalMu.Unlock()

	globalConfig = nil
}

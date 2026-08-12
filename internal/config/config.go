package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
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
	defaultNacosAddress          = "127.0.0.1:8848"
	defaultNacosNamespace        = "public"
	defaultNacosGroup            = "DEFAULT_GROUP"
	defaultNacosClusterName      = "DEFAULT"
	defaultNacosWeight           = 1.0
	defaultNacosTimeoutMs        = 5000
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
	Nacos    NacosConfig    `toml:"nacos"`
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

type NacosConfig struct {
	Enabled     bool    `toml:"enabled"`
	Address     string  `toml:"address"`
	Namespace   string  `toml:"namespace"`
	Group       string  `toml:"group"`
	Username    string  `toml:"username"`
	Password    string  `toml:"password"`
	ServiceName string  `toml:"service_name"`
	IP          string  `toml:"ip"`
	Port        int     `toml:"port"`
	ClusterName string  `toml:"cluster_name"`
	Weight      float64 `toml:"weight"`
	Ephemeral   bool    `toml:"ephemeral"`
	TimeoutMs   uint64  `toml:"timeout_ms"`
	LogDir      string  `toml:"log_dir"`
	CacheDir    string  `toml:"cache_dir"`
	LogLevel    string  `toml:"log_level"`
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
		Nacos: NacosConfig{
			Address:     defaultNacosAddress,
			Namespace:   defaultNacosNamespace,
			Group:       defaultNacosGroup,
			ClusterName: defaultNacosClusterName,
			Weight:      defaultNacosWeight,
			Ephemeral:   true,
			TimeoutMs:   defaultNacosTimeoutMs,
			LogLevel:    defaultLogLevel,
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

	c.Nacos.Address = strings.TrimSpace(c.Nacos.Address)
	if c.Nacos.Address == "" {
		c.Nacos.Address = defaultNacosAddress
	}
	c.Nacos.Namespace = strings.TrimSpace(c.Nacos.Namespace)
	if c.Nacos.Namespace == "" {
		c.Nacos.Namespace = defaultNacosNamespace
	}
	c.Nacos.Group = strings.TrimSpace(c.Nacos.Group)
	if c.Nacos.Group == "" {
		c.Nacos.Group = defaultNacosGroup
	}
	c.Nacos.Username = strings.TrimSpace(c.Nacos.Username)
	c.Nacos.Password = strings.TrimSpace(c.Nacos.Password)
	c.Nacos.ServiceName = strings.TrimSpace(c.Nacos.ServiceName)
	if c.Nacos.ServiceName == "" {
		c.Nacos.ServiceName = c.App.Name
	}
	c.Nacos.IP = strings.TrimSpace(c.Nacos.IP)
	if c.Nacos.Port <= 0 {
		port, err := portFromAddr(c.HTTP.Addr)
		if err == nil {
			c.Nacos.Port = port
		}
	}
	c.Nacos.ClusterName = strings.TrimSpace(c.Nacos.ClusterName)
	if c.Nacos.ClusterName == "" {
		c.Nacos.ClusterName = defaultNacosClusterName
	}
	if c.Nacos.Weight <= 0 {
		c.Nacos.Weight = defaultNacosWeight
	}
	if c.Nacos.TimeoutMs == 0 {
		c.Nacos.TimeoutMs = defaultNacosTimeoutMs
	}
	c.Nacos.LogDir = strings.TrimSpace(c.Nacos.LogDir)
	c.Nacos.CacheDir = strings.TrimSpace(c.Nacos.CacheDir)
	c.Nacos.LogLevel = strings.ToLower(strings.TrimSpace(c.Nacos.LogLevel))
	if c.Nacos.LogLevel == "" {
		c.Nacos.LogLevel = defaultLogLevel
	}
	if !validLogLevel(c.Nacos.LogLevel) {
		return fmt.Errorf("config error: unsupported nacos.log_level %q", c.Nacos.LogLevel)
	}
	if c.Nacos.Enabled && c.Nacos.Address == "" {
		return fmt.Errorf("config error: nacos.address cannot be empty when nacos.enabled is true")
	}
	if c.Nacos.Enabled && c.Nacos.IP == "" {
		return fmt.Errorf("config error: nacos.ip cannot be empty when nacos.enabled is true")
	}
	if c.Nacos.Enabled && c.Nacos.Port <= 0 {
		return fmt.Errorf("config error: nacos.port must be positive when nacos.enabled is true")
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

func portFromAddr(addr string) (int, error) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		if strings.HasPrefix(addr, ":") {
			port = strings.TrimPrefix(addr, ":")
		} else {
			return 0, err
		}
	}

	parsed, err := strconv.Atoi(port)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("invalid address port %q", port)
	}

	return parsed, nil
}

func resetForTest() {
	globalMu.Lock()
	defer globalMu.Unlock()

	globalConfig = nil
}

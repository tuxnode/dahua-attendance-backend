package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadRequiresConfigPath(t *testing.T) {
	_, err := Load("")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadParsesConfigFile(t *testing.T) {
	path := writeConfigFile(t, `
[app]
name = "attendance"
env = "test"

[http]
addr = ":18080"
max_body_bytes = 2048
read_header_timeout = "2s"
read_timeout = "3s"
write_timeout = "4s"
idle_timeout = "5s"
shutdown_timeout = "6s"

[database]
driver = "mysql"
dsn = "user:password@tcp(127.0.0.1:3306)/attendance?parseTime=true"
max_open_conns = 20
max_idle_conns = 8
conn_max_lifetime = "10m"
connect_timeout = "7s"

[nacos]
enabled = true
address = "127.0.0.1:8848"
namespace = "public"
group = "ATTENDANCE_GROUP"
service_name = "attendance-api"
ip = "192.168.120.10"
port = 18080
cluster_name = "cluster-a"
weight = 2.5
ephemeral = true
timeout_ms = 3000
log_dir = "/tmp/nacos/log"
cache_dir = "/tmp/nacos/cache"
log_level = "warn"

[log]
level = "debug"
file_path = "/tmp/attendance.log"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.App.Name != "attendance" {
		t.Fatalf("unexpected app name: %s", cfg.App.Name)
	}
	if cfg.HTTP.Addr != ":18080" {
		t.Fatalf("unexpected addr: %s", cfg.HTTP.Addr)
	}
	if cfg.HTTP.MaxBodyBytes != 2048 {
		t.Fatalf("unexpected max body bytes: %d", cfg.HTTP.MaxBodyBytes)
	}
	if cfg.HTTP.ReadHeaderTimeout.Std() != 2*time.Second {
		t.Fatalf("unexpected read header timeout: %s", cfg.HTTP.ReadHeaderTimeout)
	}
	if cfg.Database.DSN == "" {
		t.Fatal("expected database dsn")
	}
	if cfg.Database.MaxOpenConns != 20 {
		t.Fatalf("unexpected max open conns: %d", cfg.Database.MaxOpenConns)
	}
	if cfg.Database.MaxIdleConns != 8 {
		t.Fatalf("unexpected max idle conns: %d", cfg.Database.MaxIdleConns)
	}
	if cfg.Database.ConnMaxLifetime.Std() != 10*time.Minute {
		t.Fatalf("unexpected conn max lifetime: %s", cfg.Database.ConnMaxLifetime)
	}
	if cfg.Database.ConnectTimeout.Std() != 7*time.Second {
		t.Fatalf("unexpected connect timeout: %s", cfg.Database.ConnectTimeout)
	}
	if !cfg.Nacos.Enabled {
		t.Fatal("expected nacos to be enabled")
	}
	if cfg.Nacos.ServiceName != "attendance-api" {
		t.Fatalf("unexpected nacos service name: %s", cfg.Nacos.ServiceName)
	}
	if cfg.Nacos.Group != "ATTENDANCE_GROUP" {
		t.Fatalf("unexpected nacos group: %s", cfg.Nacos.Group)
	}
	if cfg.Nacos.IP != "192.168.120.10" {
		t.Fatalf("unexpected nacos ip: %s", cfg.Nacos.IP)
	}
	if cfg.Nacos.Port != 18080 {
		t.Fatalf("unexpected nacos port: %d", cfg.Nacos.Port)
	}
	if cfg.Nacos.ClusterName != "cluster-a" {
		t.Fatalf("unexpected nacos cluster: %s", cfg.Nacos.ClusterName)
	}
	if cfg.Nacos.Weight != 2.5 {
		t.Fatalf("unexpected nacos weight: %f", cfg.Nacos.Weight)
	}
	if cfg.Nacos.TimeoutMs != 3000 {
		t.Fatalf("unexpected nacos timeout: %d", cfg.Nacos.TimeoutMs)
	}
	if cfg.Nacos.LogLevel != "warn" {
		t.Fatalf("unexpected nacos log level: %s", cfg.Nacos.LogLevel)
	}
	if cfg.Log.Level != "debug" {
		t.Fatalf("unexpected log level: %s", cfg.Log.Level)
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	path := writeConfigFile(t, `
[app]
name = "attendance"

[database]
dsn = "user:password@tcp(127.0.0.1:3306)/attendance?parseTime=true"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.App.Env != "dev" {
		t.Fatalf("unexpected app env: %s", cfg.App.Env)
	}
	if cfg.HTTP.Addr != ":8080" {
		t.Fatalf("unexpected http addr: %s", cfg.HTTP.Addr)
	}
	if cfg.HTTP.MaxBodyBytes != 10<<20 {
		t.Fatalf("unexpected max body bytes: %d", cfg.HTTP.MaxBodyBytes)
	}
	if cfg.Database.Driver != "mysql" {
		t.Fatalf("unexpected database driver: %s", cfg.Database.Driver)
	}
	if cfg.Database.ConnectTimeout.Std() != 5*time.Second {
		t.Fatalf("unexpected connect timeout: %s", cfg.Database.ConnectTimeout)
	}
	if cfg.Log.Level != "info" {
		t.Fatalf("unexpected log level: %s", cfg.Log.Level)
	}
	if cfg.Nacos.Enabled {
		t.Fatal("expected nacos to be disabled by default")
	}
	if cfg.Nacos.ServiceName != "attendance" {
		t.Fatalf("unexpected nacos service name: %s", cfg.Nacos.ServiceName)
	}
	if cfg.Nacos.Port != 8080 {
		t.Fatalf("unexpected nacos port: %d", cfg.Nacos.Port)
	}
	if cfg.Nacos.ClusterName != "DEFAULT" {
		t.Fatalf("unexpected nacos cluster: %s", cfg.Nacos.ClusterName)
	}
	if cfg.Nacos.Weight != 1 {
		t.Fatalf("unexpected nacos weight: %f", cfg.Nacos.Weight)
	}
	if !cfg.Nacos.Ephemeral {
		t.Fatal("expected nacos ephemeral default")
	}
}

func TestLoadRejectsMissingDatabaseDSN(t *testing.T) {
	path := writeConfigFile(t, `
[app]
name = "attendance"
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadRejectsInvalidLogLevel(t *testing.T) {
	path := writeConfigFile(t, `
[app]
name = "attendance"

[database]
dsn = "user:password@tcp(127.0.0.1:3306)/attendance?parseTime=true"

[log]
level = "trace"
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadRejectsEnabledNacosWithoutIP(t *testing.T) {
	path := writeConfigFile(t, `
[app]
name = "attendance"

[database]
dsn = "user:password@tcp(127.0.0.1:3306)/attendance?parseTime=true"

[nacos]
enabled = true
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInitCanRetryAfterFailure(t *testing.T) {
	resetForTest()
	t.Cleanup(resetForTest)

	_, err := Init(filepath.Join(t.TempDir(), "missing.toml"))
	if err == nil {
		t.Fatal("expected missing file error")
	}

	path := writeConfigFile(t, `
[app]
name = "attendance"

[database]
dsn = "user:password@tcp(127.0.0.1:3306)/attendance?parseTime=true"
`)

	cfg, err := Init(path)
	if err != nil {
		t.Fatalf("init config: %v", err)
	}
	if cfg != Get() {
		t.Fatal("expected Get to return initialized config")
	}
}

func TestGetPanicsBeforeInit(t *testing.T) {
	resetForTest()
	t.Cleanup(resetForTest)

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	_ = Get()
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	return path
}

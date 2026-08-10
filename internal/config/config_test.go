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

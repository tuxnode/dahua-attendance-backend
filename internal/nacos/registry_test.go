package nacos

import (
	"testing"

	"github.com/tuxnode/dahua-attendance-backend/internal/config"
)

func TestNewRegistryReturnsNilWhenDisabled(t *testing.T) {
	registry, err := NewRegistry(config.NacosConfig{}, nil)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	if registry != nil {
		t.Fatal("expected nil registry")
	}
}

func TestSplitAddress(t *testing.T) {
	host, port, err := splitAddress("127.0.0.1:8848")
	if err != nil {
		t.Fatalf("split address: %v", err)
	}
	if host != "127.0.0.1" {
		t.Fatalf("unexpected host: %s", host)
	}
	if port != 8848 {
		t.Fatalf("unexpected port: %d", port)
	}
}

func TestRegisterParam(t *testing.T) {
	param := registerParam(config.NacosConfig{
		ServiceName: "attendance",
		Group:       "DEFAULT_GROUP",
		ClusterName: "DEFAULT",
		IP:          "192.168.120.10",
		Port:        8080,
		Weight:      1,
		Ephemeral:   true,
	})

	if param.ServiceName != "attendance" {
		t.Fatalf("unexpected service name: %s", param.ServiceName)
	}
	if param.Ip != "192.168.120.10" {
		t.Fatalf("unexpected ip: %s", param.Ip)
	}
	if param.Port != 8080 {
		t.Fatalf("unexpected port: %d", param.Port)
	}
	if !param.Enable || !param.Healthy || !param.Ephemeral {
		t.Fatalf("unexpected flags: %+v", param)
	}
}

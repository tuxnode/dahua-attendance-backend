package dubbo_test

import (
	"strings"
	"testing"

	"github.com/tuxnode/dahua-attendance-backend/internal/config"
	"github.com/tuxnode/dahua-attendance-backend/internal/transport/dubbo"
)

func TestNewServerReturnsNilWhenDubboDisabled(t *testing.T) {
	server, err := dubbo.NewServer(config.Config{}, nil, nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	if server != nil {
		t.Fatal("expected nil server")
	}
}

func TestNewServerRejectsNilProviderWhenDubboEnabled(t *testing.T) {
	_, err := dubbo.NewServer(config.Config{
		Dubbo: config.DubboConfig{Enabled: true},
	}, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "attendance provider is nil") {
		t.Fatalf("unexpected error: %v", err)
	}
}

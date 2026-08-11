package nacos

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/tuxnode/dahua-attendance-backend/internal/config"
)

type Registry struct {
	cfg    config.NacosConfig
	client naming_client.INamingClient
	logger *slog.Logger

	mu         sync.Mutex
	registered bool
}

func NewRegistry(cfg config.NacosConfig, logger *slog.Logger) (*Registry, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if logger == nil {
		logger = slog.Default()
	}

	host, port, err := splitAddress(cfg.Address)
	if err != nil {
		return nil, err
	}

	serverConfigs := []constant.ServerConfig{
		*constant.NewServerConfig(host, port, constant.WithContextPath("/nacos")),
	}

	clientOptions := []constant.ClientOption{
		constant.WithNamespaceId(cfg.Namespace),
		constant.WithTimeoutMs(cfg.TimeoutMs),
		constant.WithNotLoadCacheAtStart(true),
		constant.WithLogLevel(cfg.LogLevel),
	}
	if cfg.Username != "" {
		clientOptions = append(clientOptions, constant.WithUsername(cfg.Username))
	}
	if cfg.Password != "" {
		clientOptions = append(clientOptions, constant.WithPassword(cfg.Password))
	}
	if cfg.LogDir != "" {
		clientOptions = append(clientOptions, constant.WithLogDir(cfg.LogDir))
	}
	if cfg.CacheDir != "" {
		clientOptions = append(clientOptions, constant.WithCacheDir(cfg.CacheDir))
	}

	clientConfig := *constant.NewClientConfig(clientOptions...)
	client, err := clients.NewNamingClient(vo.NacosClientParam{
		ClientConfig:  &clientConfig,
		ServerConfigs: serverConfigs,
	})
	if err != nil {
		return nil, fmt.Errorf("nacos: create naming client: %w", err)
	}

	return &Registry{
		cfg:    cfg,
		client: client,
		logger: logger,
	}, nil
}

func (r *Registry) Register(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if err := ctxErr(ctx); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.registered {
		return nil
	}

	success, err := r.client.RegisterInstance(registerParam(r.cfg))
	if err != nil {
		return fmt.Errorf("nacos: register instance: %w", err)
	}
	if !success {
		return fmt.Errorf("nacos: register instance failed")
	}

	r.registered = true
	r.logger.Info(
		"nacos instance registered",
		"service_name", r.cfg.ServiceName,
		"group", r.cfg.Group,
		"cluster", r.cfg.ClusterName,
		"ip", r.cfg.IP,
		"port", r.cfg.Port,
	)

	return nil
}

func (r *Registry) Deregister(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if err := ctxErr(ctx); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.registered {
		return nil
	}

	success, err := r.client.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          r.cfg.IP,
		Port:        uint64(r.cfg.Port),
		ServiceName: r.cfg.ServiceName,
		GroupName:   r.cfg.Group,
		Cluster:     r.cfg.ClusterName,
		Ephemeral:   r.cfg.Ephemeral,
	})
	if err != nil {
		return fmt.Errorf("nacos: deregister instance: %w", err)
	}
	if !success {
		return fmt.Errorf("nacos: deregister instance failed")
	}

	r.registered = false
	r.logger.Info(
		"nacos instance deregistered",
		"service_name", r.cfg.ServiceName,
		"group", r.cfg.Group,
		"cluster", r.cfg.ClusterName,
		"ip", r.cfg.IP,
		"port", r.cfg.Port,
	)

	return nil
}

func registerParam(cfg config.NacosConfig) vo.RegisterInstanceParam {
	return vo.RegisterInstanceParam{
		Ip:          cfg.IP,
		Port:        uint64(cfg.Port),
		ServiceName: cfg.ServiceName,
		GroupName:   cfg.Group,
		ClusterName: cfg.ClusterName,
		Weight:      cfg.Weight,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   cfg.Ephemeral,
		Metadata: map[string]string{
			"protocol": "http",
		},
	}
}

func splitAddress(address string) (string, uint64, error) {
	address = strings.TrimSpace(address)
	address = strings.TrimPrefix(address, "http://")
	address = strings.TrimPrefix(address, "https://")

	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, fmt.Errorf("nacos: invalid address %q: %w", address, err)
	}

	port, err := strconv.ParseUint(portText, 10, 64)
	if err != nil || port == 0 {
		return "", 0, fmt.Errorf("nacos: invalid address port %q", portText)
	}

	return host, port, nil
}

func ctxErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}

	return ctx.Err()
}

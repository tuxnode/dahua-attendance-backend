package dubbo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	dubbogo "dubbo.apache.org/dubbo-go/v3"
	"dubbo.apache.org/dubbo-go/v3/protocol"
	"dubbo.apache.org/dubbo-go/v3/registry"
	dubboserver "dubbo.apache.org/dubbo-go/v3/server"
	"github.com/tuxnode/dahua-attendance-backend/internal/config"
)

type Server struct {
	cfg    config.Config
	logger *slog.Logger
	server *dubboserver.Server

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func NewServer(cfg config.Config, provider *AttendanceProvider, logger *slog.Logger) (*Server, error) {
	if !cfg.Dubbo.Enabled {
		return nil, nil
	}
	if provider == nil {
		return nil, fmt.Errorf("dubbo: attendance provider is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}

	registryOpts := []registry.Option{
		registry.WithNacos(),
		registry.WithAddress(cfg.Nacos.Address),
		registry.WithGroup(cfg.Nacos.Group),
		registry.WithNamespace(cfg.Nacos.Namespace),
	}
	if cfg.Nacos.Username != "" {
		registryOpts = append(registryOpts, registry.WithUsername(cfg.Nacos.Username))
	}
	if cfg.Nacos.Password != "" {
		registryOpts = append(registryOpts, registry.WithPassword(cfg.Nacos.Password))
	}

	instance, err := dubbogo.NewInstance(
		dubbogo.WithName(cfg.App.Name),
		dubbogo.WithEnvironment(cfg.App.Env),
		dubbogo.WithRegistry(registryOpts...),
		dubbogo.WithProtocol(
			protocol.WithTriple(),
			protocol.WithPort(cfg.Dubbo.Port),
			protocol.WithIp(cfg.Dubbo.AdvertiseIP),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("dubbo: create instance: %w", err)
	}

	server, err := instance.NewServer()
	if err != nil {
		return nil, fmt.Errorf("dubbo: create server: %w", err)
	}

	if err := server.RegisterService(
		provider,
		dubboserver.WithInterface(cfg.Dubbo.Interface),
		dubboserver.WithGroup(cfg.Dubbo.Group),
		dubboserver.WithVersion(cfg.Dubbo.Version),
	); err != nil {
		return nil, fmt.Errorf("dubbo: register attendance provider: %w", err)
	}

	return &Server{
		cfg:    cfg,
		logger: logger,
		server: server,
	}, nil
}

func (s *Server) Start(ctx context.Context) error {
	if s == nil || s.server == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	startCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		cancel()
		return errors.New("dubbo: server is already started")
	}
	s.cancel = cancel
	s.done = done
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.cancel = nil
		s.done = nil
		close(done)
		s.mu.Unlock()
		cancel()
	}()

	s.logger.Info(
		"dubbo server started",
		"interface", s.cfg.Dubbo.Interface,
		"group", s.cfg.Dubbo.Group,
		"version", s.cfg.Dubbo.Version,
		"protocol", s.cfg.Dubbo.Protocol,
		"port", s.cfg.Dubbo.Port,
		"advertise_ip", s.cfg.Dubbo.AdvertiseIP,
		"nacos_address", s.cfg.Nacos.Address,
	)

	if err := s.server.ServeContext(startCtx); err != nil {
		return fmt.Errorf("dubbo: serve: %w", err)
	}

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	s.mu.Unlock()

	if cancel == nil {
		return nil
	}

	cancel()
	if done == nil {
		return nil
	}

	select {
	case <-done:
		s.logger.Info("dubbo server stopped")
	case <-ctx.Done():
		return fmt.Errorf("dubbo: stop: %w", ctx.Err())
	}

	return nil
}

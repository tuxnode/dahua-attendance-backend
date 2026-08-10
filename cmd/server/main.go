package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/repository"
	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/service"
	transporthttp "github.com/tuxnode/dahua-attendance-backend/internal/transport/http"

	_ "github.com/go-sql-driver/mysql"
)

const (
	defaultHTTPAddr          = ":8080"
	defaultReadHeaderTimeout = 5 * time.Second
	defaultReadTimeout       = 15 * time.Second
	defaultWriteTimeout      = 5 * time.Second
	defaultIdleTimeout       = 60 * time.Second
	defaultShutdownTimeout   = 10 * time.Second
	defaultDBDriver          = "mysql"
	defaultDBConnectTimeout  = 5 * time.Second
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	addr := envString("HTTP_ADDR", defaultHTTPAddr)
	shutdownTimeout := envDuration("HTTP_SHUTDOWN_TIMEOUT", defaultShutdownTimeout, logger)
	dbConnectTimeout := envDuration("DB_CONNECT_TIMEOUT", defaultDBConnectTimeout, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealthz)

	consumer, cleanup := buildConsumer(logger, dbConnectTimeout)
	defer cleanup()

	handler := transporthttp.NewHandler(
		consumer,
		transporthttp.WithLogger(logger),
		transporthttp.WithMaxBodyBytes(envInt64("HTTP_MAX_BODY_BYTES", transporthttp.DefaultMaxBodyBytes, logger)),
	)
	handler.RegisterRoutes(mux)

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: envDuration("HTTP_READ_HEADER_TIMEOUT", defaultReadHeaderTimeout, logger),
		ReadTimeout:       envDuration("HTTP_READ_TIMEOUT", defaultReadTimeout, logger),
		WriteTimeout:      envDuration("HTTP_WRITE_TIMEOUT", defaultWriteTimeout, logger),
		IdleTimeout:       envDuration("HTTP_IDLE_TIMEOUT", defaultIdleTimeout, logger),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info(
			"device HTTP server started",
			"addr", addr,
			"paths", []string{transporthttp.DefaultPath, transporthttp.DeviceEventsPath},
			"health_path", "/healthz",
		)
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("device HTTP server failed", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("device HTTP server shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("device HTTP server stopped")
}

func buildConsumer(logger *slog.Logger, connectTimeout time.Duration) (transporthttp.EventConsumer, func()) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		logger.Warn("database is not configured, fallback to logging consumer")
		return transporthttp.NewLoggingConsumer(logger), func() {}
	}

	driver := envString("DB_DRIVER", defaultDBDriver)
	db, err := sql.Open(driver, dsn)
	if err != nil {
		logger.Error("open database failed", "driver", driver, "error", err)
		os.Exit(1)
	}

	db.SetMaxOpenConns(int(envInt64("DB_MAX_OPEN_CONNS", 10, logger)))
	db.SetMaxIdleConns(int(envInt64("DB_MAX_IDLE_CONNS", 5, logger)))
	db.SetConnMaxLifetime(envDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute, logger))

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		logger.Error("ping database failed", "driver", driver, "error", err)
		os.Exit(1)
	}

	store, err := repository.NewSQLRepository(db)
	if err != nil {
		_ = db.Close()
		logger.Error("create sql repository failed", "error", err)
		os.Exit(1)
	}

	logger.Info("database repository enabled", "driver", driver)

	return service.New(store, service.WithLogger(logger)), func() {
		if err := db.Close(); err != nil {
			logger.Warn("close database failed", "error", err)
		}
	}
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func envString(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envDuration(key string, fallback time.Duration, logger *slog.Logger) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		logger.Warn("invalid duration env, fallback is used", "key", key, "value", value, "fallback", fallback.String(), "error", err)
		return fallback
	}

	return duration
}

func envInt64(key string, fallback int64, logger *slog.Logger) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		logger.Warn("invalid int env, fallback is used", "key", key, "value", value, "fallback", fallback, "error", err)
		return fallback
	}

	return parsed
}

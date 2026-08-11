package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/repository"
	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/service"
	"github.com/tuxnode/dahua-attendance-backend/internal/config"
	nacosregistry "github.com/tuxnode/dahua-attendance-backend/internal/nacos"
	transporthttp "github.com/tuxnode/dahua-attendance-backend/internal/transport/http"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	configPath := flag.String("config", "", "path to TOML config file")
	flag.Parse()

	cfg, err := config.Init(*configPath)
	if err != nil {
		bootstrapLogger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
		bootstrapLogger.Error("load config failed", "error", err)
		os.Exit(1)
	}

	logger, closeLogger := buildLogger(cfg.Log)
	defer closeLogger()
	slog.SetDefault(logger)

	attendanceService, cleanup := buildAttendanceService(cfg.Database, logger)
	defer cleanup()

	router := transporthttp.NewRouter(
		attendanceService,
		transporthttp.WithLogger(logger),
		transporthttp.WithMaxBodyBytes(cfg.HTTP.MaxBodyBytes),
	)

	server := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           router,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout.Std(),
		ReadTimeout:       cfg.HTTP.ReadTimeout.Std(),
		WriteTimeout:      cfg.HTTP.WriteTimeout.Std(),
		IdleTimeout:       cfg.HTTP.IdleTimeout.Std(),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	registry, err := nacosregistry.NewRegistry(cfg.Nacos, logger)
	if err != nil {
		logger.Error("create nacos registry failed", "error", err)
		os.Exit(1)
	}
	if err := registry.Register(context.Background()); err != nil {
		logger.Error("register nacos instance failed", "error", err)
		os.Exit(1)
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info(
			"gin HTTP server started",
			"app", cfg.App.Name,
			"env", cfg.App.Env,
			"addr", cfg.HTTP.Addr,
			"paths", []string{
				transporthttp.DefaultPath,
				transporthttp.DeviceEventsPath,
				transporthttp.AttendanceRecordsPath,
				transporthttp.DailyAttendancePath,
				transporthttp.MonthlyAttendancePath,
				transporthttp.AttendanceSummaryPath,
				transporthttp.AttendanceExceptionsPath,
			},
			"health_path", "/healthz",
		)
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		if err != nil {
			logger.Error("gin HTTP server failed", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout.Std())
	defer cancel()

	if err := registry.Deregister(shutdownCtx); err != nil {
		logger.Warn("deregister nacos instance failed", "error", err)
	}

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("gin HTTP server shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("gin HTTP server stopped")
}

func buildAttendanceService(cfg config.DatabaseConfig, logger *slog.Logger) (*service.Service, func()) {
	db, err := sql.Open(cfg.Driver, cfg.DSN)
	if err != nil {
		logger.Error("open database failed", "driver", cfg.Driver, "error", err)
		os.Exit(1)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime.Std())

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout.Std())
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		logger.Error("ping database failed", "driver", cfg.Driver, "error", err)
		os.Exit(1)
	}

	store, err := repository.NewSQLRepository(db)
	if err != nil {
		_ = db.Close()
		logger.Error("create sql repository failed", "error", err)
		os.Exit(1)
	}

	logger.Info("database repository enabled", "driver", cfg.Driver)

	return service.New(store, service.WithLogger(logger)), func() {
		if err := db.Close(); err != nil {
			logger.Warn("close database failed", "error", err)
		}
	}
}

func buildLogger(cfg config.LogConfig) (*slog.Logger, func()) {
	level := slog.LevelInfo
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	output := io.Writer(os.Stdout)
	cleanup := func() {}
	if cfg.FilePath != "" {
		file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			bootstrapLogger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
			bootstrapLogger.Error("open log file failed", "path", cfg.FilePath, "error", err)
			os.Exit(1)
		}
		output = file
		cleanup = func() {
			if err := file.Close(); err != nil {
				_, _ = os.Stderr.WriteString("close log file failed: " + err.Error() + "\n")
			}
		}
	}

	return slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{Level: level})), cleanup
}

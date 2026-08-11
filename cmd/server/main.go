package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/domain"
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

	rules, err := buildAttendanceRules(cfg.Attendance)
	if err != nil {
		logger.Error("build attendance rules failed", "error", err)
		os.Exit(1)
	}

	attendanceService, cleanup := buildAttendanceService(cfg.Database, rules, logger)
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

	var registry *nacosregistry.Registry
	if cfg.Nacos.Enabled {
		registry, err = nacosregistry.NewRegistry(cfg.Nacos, logger)
		if err != nil {
			logger.Error("create nacos registry failed", "error", err)
			os.Exit(1)
		}
		if err := registry.Register(context.Background()); err != nil {
			logger.Error("register nacos instance failed", "error", err)
			os.Exit(1)
		}
	} else {
		logger.Info("nacos registry disabled")
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

	if registry != nil {
		if err := registry.Deregister(shutdownCtx); err != nil {
			logger.Warn("deregister nacos instance failed", "error", err)
		}
	}

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("gin HTTP server shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("gin HTTP server stopped")
}

func buildAttendanceService(cfg config.DatabaseConfig, rules domain.AttendanceRules, logger *slog.Logger) (*service.Service, func()) {
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

	return service.New(store, service.WithLogger(logger), service.WithAttendanceRules(rules)), func() {
		if err := db.Close(); err != nil {
			logger.Warn("close database failed", "error", err)
		}
	}
}

func buildAttendanceRules(cfg config.AttendanceConfig) (domain.AttendanceRules, error) {
	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return domain.AttendanceRules{}, fmt.Errorf("load attendance timezone %q: %w", cfg.Timezone, err)
	}

	rules := domain.AttendanceRules{
		Location:        location,
		DefaultShiftID:  cfg.DefaultShiftID,
		WeekendDays:     make(map[time.Weekday]bool),
		Holidays:        make(map[string]string),
		Workdays:        make(map[string]bool),
		Shifts:          make(map[string]domain.AttendanceShift),
		Schedules:       make([]domain.AttendanceSchedule, 0, len(cfg.Schedules)),
		WeeklySchedules: make([]domain.AttendanceWeeklySchedule, 0, len(cfg.WeeklySchedules)),
	}

	for _, value := range cfg.WeekendDays {
		weekday, err := parseWeekday(value)
		if err != nil {
			return domain.AttendanceRules{}, err
		}
		rules.WeekendDays[weekday] = true
	}
	for _, value := range cfg.Workdays {
		date, err := parseConfigDate(value, location)
		if err != nil {
			return domain.AttendanceRules{}, fmt.Errorf("parse attendance workday: %w", err)
		}
		rules.Workdays[date.Format("2006-01-02")] = true
	}
	for _, holiday := range cfg.Holidays {
		date, err := parseConfigDate(holiday.Date, location)
		if err != nil {
			return domain.AttendanceRules{}, fmt.Errorf("parse attendance holiday: %w", err)
		}
		rules.Holidays[date.Format("2006-01-02")] = holiday.Name
	}
	for _, shift := range cfg.Shifts {
		attendanceShift, err := buildAttendanceShift(shift)
		if err != nil {
			return domain.AttendanceRules{}, err
		}
		rules.Shifts[attendanceShift.ID] = attendanceShift
	}
	for _, schedule := range cfg.Schedules {
		date, err := parseConfigDate(schedule.Date, location)
		if err != nil {
			return domain.AttendanceRules{}, fmt.Errorf("parse attendance schedule date: %w", err)
		}
		rules.Schedules = append(rules.Schedules, domain.AttendanceSchedule{
			UserID:   schedule.UserID,
			DeviceSN: schedule.DeviceSN,
			Date:     date,
			ShiftID:  schedule.ShiftID,
			Rest:     schedule.Rest,
			Reason:   schedule.Reason,
		})
	}
	for _, schedule := range cfg.WeeklySchedules {
		weekday, err := parseWeekday(schedule.Weekday)
		if err != nil {
			return domain.AttendanceRules{}, err
		}
		rules.WeeklySchedules = append(rules.WeeklySchedules, domain.AttendanceWeeklySchedule{
			UserID:   schedule.UserID,
			DeviceSN: schedule.DeviceSN,
			Weekday:  weekday,
			ShiftID:  schedule.ShiftID,
			Rest:     schedule.Rest,
			Reason:   schedule.Reason,
		})
	}

	return rules, nil
}

func buildAttendanceShift(cfg config.AttendanceShiftConfig) (domain.AttendanceShift, error) {
	start, err := parseClockTime(cfg.StartTime)
	if err != nil {
		return domain.AttendanceShift{}, fmt.Errorf("parse attendance shift %q start_time: %w", cfg.ID, err)
	}
	end, err := parseClockTime(cfg.EndTime)
	if err != nil {
		return domain.AttendanceShift{}, fmt.Errorf("parse attendance shift %q end_time: %w", cfg.ID, err)
	}

	name := cfg.Name
	if strings.TrimSpace(name) == "" {
		name = cfg.ID
	}

	return domain.AttendanceShift{
		ID:              cfg.ID,
		Name:            name,
		Start:           start,
		End:             end,
		LateGrace:       time.Duration(cfg.LateGraceMinutes) * time.Minute,
		EarlyLeaveGrace: time.Duration(cfg.EarlyLeaveGraceMinutes) * time.Minute,
		Flexible:        time.Duration(cfg.FlexibleMinutes) * time.Minute,
		Enabled:         cfg.Enabled == nil || *cfg.Enabled,
	}, nil
}

func parseConfigDate(value string, location *time.Location) (time.Time, error) {
	date, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(value), location)
	if err != nil {
		return time.Time{}, fmt.Errorf("%q must use YYYY-MM-DD format", value)
	}

	return date, nil
}

func parseClockTime(value string) (domain.ClockTime, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return domain.ClockTime{}, fmt.Errorf("%q must use HH:MM format", value)
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return domain.ClockTime{}, fmt.Errorf("invalid hour %q", parts[0])
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return domain.ClockTime{}, fmt.Errorf("invalid minute %q", parts[1])
	}

	return domain.ClockTime{Hour: hour, Minute: minute}, nil
}

func parseWeekday(value string) (time.Weekday, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "0", "sunday", "sun":
		return time.Sunday, nil
	case "1", "monday", "mon":
		return time.Monday, nil
	case "2", "tuesday", "tue":
		return time.Tuesday, nil
	case "3", "wednesday", "wed":
		return time.Wednesday, nil
	case "4", "thursday", "thu":
		return time.Thursday, nil
	case "5", "friday", "fri":
		return time.Friday, nil
	case "6", "saturday", "sat":
		return time.Saturday, nil
	default:
		return 0, fmt.Errorf("unsupported weekday %q", value)
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

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/domain"
)

func (s *Service) SaveAttendanceCorrection(ctx context.Context, correction domain.AttendanceCorrection) (domain.AttendanceCorrection, error) {
	if s.repository == nil {
		return domain.AttendanceCorrection{}, errors.New("service: repository is nil")
	}

	normalized, err := normalizeAttendanceCorrection(correction)
	if err != nil {
		return domain.AttendanceCorrection{}, err
	}
	if normalized.Status == "" {
		normalized.Status = domain.AttendanceCorrectionStatusApplied
	}

	saved, err := s.repository.SaveAttendanceCorrection(ctx, normalized)
	if err != nil {
		return domain.AttendanceCorrection{}, fmt.Errorf("service: save attendance correction: %w", err)
	}
	if saved.ID == 0 {
		saved.ID = normalized.ID
	}

	result, err := s.buildCorrectedMonthlyAttendanceResult(ctx, saved)
	if err != nil {
		return domain.AttendanceCorrection{}, err
	}
	if _, err := s.repository.SaveMonthlyAttendanceResult(ctx, result); err != nil {
		return domain.AttendanceCorrection{}, fmt.Errorf("service: save corrected monthly attendance result: %w", err)
	}

	return saved, nil
}

func normalizeAttendanceCorrection(correction domain.AttendanceCorrection) (domain.AttendanceCorrection, error) {
	correction.UserID = strings.TrimSpace(correction.UserID)
	correction.DeviceSN = strings.TrimSpace(correction.DeviceSN)
	correction.Date = startOfDay(correction.Date)
	correction.Reason = strings.TrimSpace(correction.Reason)
	if correction.UserID == "" {
		return domain.AttendanceCorrection{}, errors.New("service: attendance correction user_id cannot be empty")
	}
	if correction.Date.IsZero() {
		return domain.AttendanceCorrection{}, errors.New("service: attendance correction date cannot be empty")
	}
	if correction.CorrectedAt.IsZero() {
		return domain.AttendanceCorrection{}, errors.New("service: attendance correction corrected_at cannot be empty")
	}
	if !correction.Type.IsValid() {
		return domain.AttendanceCorrection{}, fmt.Errorf("service: unsupported attendance correction type %q", correction.Type)
	}

	return correction, nil
}

func (s *Service) buildCorrectedMonthlyAttendanceResult(ctx context.Context, correction domain.AttendanceCorrection) (domain.MonthlyAttendanceDailyResult, error) {
	dailies, err := s.listDailyAttendance(ctx, domain.DailyAttendanceQuery{
		AttendancePersonFilter: domain.AttendancePersonFilter{
			UserID:   correction.UserID,
			DeviceSN: correction.DeviceSN,
		},
		DateRangeFilter: domain.DateRangeFilter{
			StartDate: correction.Date,
			EndDate:   correction.Date,
		},
		Pagination: domain.Pagination{Limit: maxQueryLimit},
	})
	if err != nil {
		return domain.MonthlyAttendanceDailyResult{}, fmt.Errorf("service: recalculate attendance day: %w", err)
	}

	var daily domain.DailyAttendance
	if len(dailies) > 0 {
		daily = dailies[0]
	} else {
		daily = domain.DailyAttendance{
			Date:     correction.Date,
			UserID:   correction.UserID,
			DeviceSN: correction.DeviceSN,
			Status:   domain.DailyAttendanceStatusNormal,
		}
	}

	applyCorrectionToDailyAttendance(&daily, correction)
	settings, err := s.loadAttendanceSettings(ctx)
	if err != nil {
		return domain.MonthlyAttendanceDailyResult{}, err
	}

	return monthlyAttendanceResultFromDaily(daily, s.now(), settings.SettlementDay), nil
}

func applyCorrectionToDailyAttendance(daily *domain.DailyAttendance, correction domain.AttendanceCorrection) {
	if daily == nil {
		return
	}

	normal := false
	daily.IsAbnormalOverride = &normal
	daily.Corrected = true
	daily.CorrectionStatus = correction.Status
	daily.CorrectionType = correction.Type
	daily.CorrectionReason = correction.Reason
	daily.CorrectedAt = correction.CorrectedAt
	daily.Status = domain.DailyAttendanceStatusCorrected
	daily.Exceptions = nil
	daily.LateDuration = 0
	daily.EarlyLeaveDuration = 0
	if daily.FirstEntryAt.IsZero() && correction.Type == domain.AttendanceCorrectionTypeCheckIn {
		daily.FirstEntryAt = correction.CorrectedAt
	}
	if daily.LastExitAt.IsZero() && correction.Type == domain.AttendanceCorrectionTypeCheckOut {
		daily.LastExitAt = correction.CorrectedAt
	}
}

func monthlyAttendanceResultFromDaily(daily domain.DailyAttendance, calculatedAt time.Time, settlementDay int) domain.MonthlyAttendanceDailyResult {
	return domain.MonthlyAttendanceDailyResult{
		Month:              settlementMonthForDate(daily.Date, settlementDay),
		Date:               startOfDay(daily.Date),
		UserID:             daily.UserID,
		UserName:           daily.UserName,
		DeviceSN:           daily.DeviceSN,
		ShiftID:            daily.ShiftID,
		ShiftName:          daily.ShiftName,
		IsWorkday:          daily.IsWorkday,
		NonWorkdayReason:   daily.NonWorkdayReason,
		Status:             daily.Status,
		Exceptions:         daily.Exceptions,
		IsAbnormal:         daily.IsAbnormal(),
		Corrected:          daily.Corrected,
		CorrectionStatus:   daily.CorrectionStatus,
		CorrectionType:     daily.CorrectionType,
		CorrectionReason:   daily.CorrectionReason,
		CorrectedAt:        daily.CorrectedAt,
		WorkStartAt:        daily.WorkStartAt,
		WorkEndAt:          daily.WorkEndAt,
		FirstEntryAt:       daily.FirstEntryAt,
		LastExitAt:         daily.LastExitAt,
		LateDuration:       daily.LateDuration,
		EarlyLeaveDuration: daily.EarlyLeaveDuration,
		RecordCount:        daily.RecordCount,
		SnapshotCount:      daily.SnapshotCount,
		CalculatedAt:       calculatedAt,
	}
}

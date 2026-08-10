package dubbo

import (
	"context"
	"errors"
	"fmt"
	"time"

	attendancev1 "github.com/tuxnode/dahua-attendance-backend/api/attendance/v1"
	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/domain"
	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/service"
)

type AttendanceProvider struct {
	service *service.Service
}

var _ attendancev1.AttendanceService = (*AttendanceProvider)(nil)

func NewAttendanceProvider(service *service.Service) *AttendanceProvider {
	if service == nil {
		panic("attendance service is nil")
	}

	return &AttendanceProvider{
		service: service,
	}
}

func (p *AttendanceProvider) ListAttendanceRecords(
	ctx context.Context,
	req *attendancev1.ListAttendanceRecordsRequest,
) (*attendancev1.ListAttendanceRecordsResponse, error) {
	if p == nil || p.service == nil {
		return nil, errors.New("dubbo: attendance service is nil")
	}
	if req == nil {
		return nil, errors.New("dubbo: list attendance records request is nil")
	}

	records, err := p.service.ListAttendanceRecords(ctx, domain.AttendanceRecordQuery{
		UserID:    req.UserID,
		DeviceSN:  req.DeviceSN,
		StartTime: unixSeconds(req.StartTime),
		EndTime:   unixSeconds(req.EndTime),
		Limit:     req.Limit,
		Offset:    req.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("dubbo: list attendance records: %w", err)
	}

	return &attendancev1.ListAttendanceRecordsResponse{
		Records: toAttendanceRecordDTOs(records),
	}, nil
}

func unixSeconds(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}

	return time.Unix(value, 0)
}

func toAttendanceRecordDTOs(records []domain.AttendanceRecord) []attendancev1.AttendanceRecordDTO {
	dtos := make([]attendancev1.AttendanceRecordDTO, 0, len(records))
	for _, record := range records {
		dtos = append(dtos, toAttendanceRecordDTO(record))
	}

	return dtos
}

func toAttendanceRecordDTO(record domain.AttendanceRecord) attendancev1.AttendanceRecordDTO {
	return attendancev1.AttendanceRecordDTO{
		UserID:        record.UserID,
		UserName:      record.CardName,
		DeviceSN:      record.DeviceSN,
		Direction:     string(record.Direction),
		Method:        int32(record.Method),
		MethodName:    record.Method.String(),
		Status:        record.Status,
		EventTime:     timeToUnixSeconds(record.EventTime),
		ReceivedAt:    timeToUnixSeconds(record.ReceivedAt),
		HasSnapshot:   record.ImageCount > 0,
		SnapshotCount: record.ImageCount,
	}
}

func timeToUnixSeconds(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}

	return value.Unix()
}

package v1

import "context"

type AttendanceService interface {
	ListAttendanceRecords(ctx context.Context, req *ListAttendanceRecordsRequest) (*ListAttendanceRecordsResponse, error)
}

type ListAttendanceRecordsRequest struct {
	UserID    string `json:"user_id"`
	DeviceSN  string `json:"device_sn"`
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
}

type ListAttendanceRecordsResponse struct {
	Records []AttendanceRecordDTO `json:"records"`
}

type AttendanceRecordDTO struct {
	UserID        string `json:"user_id"`
	UserName      string `json:"user_name"`
	DeviceSN      string `json:"device_sn"`
	Direction     string `json:"direction"`
	Method        int32  `json:"method"`
	MethodName    string `json:"method_name"`
	Status        int32  `json:"status"`
	EventTime     int64  `json:"event_time"`
	ReceivedAt    int64  `json:"received_at"`
	HasSnapshot   bool   `json:"has_snapshot"`
	SnapshotCount int    `json:"snapshot_count"`
}

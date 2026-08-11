package v1

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

type ListDailyAttendanceRequest struct {
	UserID    string `json:"user_id"`
	DeviceSN  string `json:"device_sn"`
	Date      string `json:"date"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
}

type ListDailyAttendanceResponse struct {
	Records []DailyAttendanceDTO `json:"records"`
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

type DailyAttendanceDTO struct {
	Date              string   `json:"date"`
	UserID            string   `json:"user_id"`
	UserName          string   `json:"user_name"`
	DeviceSN          string   `json:"device_sn"`
	Status            string   `json:"status"`
	Exceptions        []string `json:"exceptions"`
	IsAbnormal        bool     `json:"is_abnormal"`
	WorkStartAt       int64    `json:"work_start_at"`
	WorkEndAt         int64    `json:"work_end_at"`
	FirstEntryAt      int64    `json:"first_entry_at"`
	LastExitAt        int64    `json:"last_exit_at"`
	LateSeconds       int64    `json:"late_seconds"`
	EarlyLeaveSeconds int64    `json:"early_leave_seconds"`
	RecordCount       int      `json:"record_count"`
	SnapshotCount     int      `json:"snapshot_count"`
}

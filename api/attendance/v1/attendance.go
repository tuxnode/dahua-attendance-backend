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

type ListMonthlyAttendanceRequest struct {
	UserID   string `json:"user_id"`
	DeviceSN string `json:"device_sn"`
	Month    string `json:"month"`
	Limit    int    `json:"limit"`
	Offset   int    `json:"offset"`
}

type ListMonthlyAttendanceResponse struct {
	Records []MonthlyAttendanceDTO `json:"records"`
}

type GetAttendanceSummaryRequest struct {
	UserID    string `json:"user_id"`
	DeviceSN  string `json:"device_sn"`
	Date      string `json:"date"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type GetAttendanceSummaryResponse struct {
	Summary AttendanceSummaryDTO `json:"summary"`
}

type ListAttendanceExceptionsRequest struct {
	UserID    string `json:"user_id"`
	DeviceSN  string `json:"device_sn"`
	Date      string `json:"date"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
}

type ListAttendanceExceptionsResponse struct {
	Records []DailyAttendanceDTO `json:"records"`
}

type CreateAttendanceCorrectionRequest struct {
	UserID      string `json:"user_id"`
	DeviceSN    string `json:"device_sn"`
	Date        string `json:"date"`
	Type        string `json:"type"`
	CorrectedAt int64  `json:"corrected_at"`
	Reason      string `json:"reason"`
}

type CreateAttendanceCorrectionResponse struct {
	Correction AttendanceCorrectionDTO `json:"correction"`
}

type AttendanceCorrectionDTO struct {
	ID          int64  `json:"id"`
	UserID      string `json:"user_id"`
	DeviceSN    string `json:"device_sn"`
	Date        string `json:"date"`
	Type        string `json:"type"`
	CorrectedAt int64  `json:"corrected_at"`
	Reason      string `json:"reason"`
	Status      string `json:"status"`
}

type AttendanceSettingsDTO struct {
	Timezone       string   `json:"timezone"`
	DefaultShiftID string   `json:"default_shift_id"`
	WeekendDays    []string `json:"weekend_days"`
}

type GetAttendanceSettingsResponse struct {
	Settings AttendanceSettingsDTO `json:"settings"`
}

type SaveAttendanceSettingsRequest struct {
	Timezone       string   `json:"timezone"`
	DefaultShiftID string   `json:"default_shift_id"`
	WeekendDays    []string `json:"weekend_days"`
}

type SaveAttendanceSettingsResponse struct {
	Settings AttendanceSettingsDTO `json:"settings"`
}

type AttendanceShiftDTO struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	StartTime              string `json:"start_time"`
	EndTime                string `json:"end_time"`
	LateGraceMinutes       int    `json:"late_grace_minutes"`
	EarlyLeaveGraceMinutes int    `json:"early_leave_grace_minutes"`
	FlexibleMinutes        int    `json:"flexible_minutes"`
	Enabled                bool   `json:"enabled"`
}

type ListAttendanceShiftsResponse struct {
	Records []AttendanceShiftDTO `json:"records"`
}

type SaveAttendanceShiftRequest struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	StartTime              string `json:"start_time"`
	EndTime                string `json:"end_time"`
	LateGraceMinutes       int    `json:"late_grace_minutes"`
	EarlyLeaveGraceMinutes int    `json:"early_leave_grace_minutes"`
	FlexibleMinutes        int    `json:"flexible_minutes"`
	Enabled                *bool  `json:"enabled"`
}

type SaveAttendanceShiftResponse struct {
	Record AttendanceShiftDTO `json:"record"`
}

type AttendanceCalendarDayDTO struct {
	Date    string `json:"date"`
	DayType string `json:"day_type"`
	Name    string `json:"name"`
}

type ListAttendanceCalendarDaysResponse struct {
	Records []AttendanceCalendarDayDTO `json:"records"`
}

type SaveAttendanceCalendarDayRequest struct {
	Date    string `json:"date"`
	DayType string `json:"day_type"`
	Name    string `json:"name"`
}

type SaveAttendanceCalendarDayResponse struct {
	Record AttendanceCalendarDayDTO `json:"record"`
}

type AttendanceScheduleDTO struct {
	ID       int64  `json:"id"`
	UserID   string `json:"user_id"`
	DeviceSN string `json:"device_sn"`
	Date     string `json:"date"`
	ShiftID  string `json:"shift_id"`
	Rest     bool   `json:"rest"`
	Reason   string `json:"reason"`
	Enabled  bool   `json:"enabled"`
}

type ListAttendanceSchedulesResponse struct {
	Records []AttendanceScheduleDTO `json:"records"`
}

type SaveAttendanceScheduleRequest struct {
	ID       int64  `json:"id"`
	UserID   string `json:"user_id"`
	DeviceSN string `json:"device_sn"`
	Date     string `json:"date"`
	ShiftID  string `json:"shift_id"`
	Rest     bool   `json:"rest"`
	Reason   string `json:"reason"`
	Enabled  *bool  `json:"enabled"`
}

type SaveAttendanceScheduleResponse struct {
	Record AttendanceScheduleDTO `json:"record"`
}

type AttendanceWeeklyScheduleDTO struct {
	ID       int64  `json:"id"`
	UserID   string `json:"user_id"`
	DeviceSN string `json:"device_sn"`
	Weekday  string `json:"weekday"`
	ShiftID  string `json:"shift_id"`
	Rest     bool   `json:"rest"`
	Reason   string `json:"reason"`
	Enabled  bool   `json:"enabled"`
}

type ListAttendanceWeeklySchedulesResponse struct {
	Records []AttendanceWeeklyScheduleDTO `json:"records"`
}

type SaveAttendanceWeeklyScheduleRequest struct {
	ID       int64  `json:"id"`
	UserID   string `json:"user_id"`
	DeviceSN string `json:"device_sn"`
	Weekday  string `json:"weekday"`
	ShiftID  string `json:"shift_id"`
	Rest     bool   `json:"rest"`
	Reason   string `json:"reason"`
	Enabled  *bool  `json:"enabled"`
}

type SaveAttendanceWeeklyScheduleResponse struct {
	Record AttendanceWeeklyScheduleDTO `json:"record"`
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
	ShiftID           string   `json:"shift_id"`
	ShiftName         string   `json:"shift_name"`
	IsWorkday         bool     `json:"is_workday"`
	NonWorkdayReason  string   `json:"non_workday_reason"`
	Status            string   `json:"status"`
	Exceptions        []string `json:"exceptions"`
	IsAbnormal        bool     `json:"is_abnormal"`
	Corrected         bool     `json:"corrected"`
	CorrectionStatus  string   `json:"correction_status"`
	CorrectionReason  string   `json:"correction_reason"`
	CorrectedAt       int64    `json:"corrected_at"`
	WorkStartAt       int64    `json:"work_start_at"`
	WorkEndAt         int64    `json:"work_end_at"`
	FirstEntryAt      int64    `json:"first_entry_at"`
	LastExitAt        int64    `json:"last_exit_at"`
	LateSeconds       int64    `json:"late_seconds"`
	EarlyLeaveSeconds int64    `json:"early_leave_seconds"`
	RecordCount       int      `json:"record_count"`
	SnapshotCount     int      `json:"snapshot_count"`
}

type MonthlyAttendanceDTO struct {
	Month    string               `json:"month"`
	UserID   string               `json:"user_id"`
	UserName string               `json:"user_name"`
	DeviceSN string               `json:"device_sn"`
	Days     []DailyAttendanceDTO `json:"days"`
	Stats    AttendanceStatsDTO   `json:"stats"`
}

type AttendanceSummaryDTO struct {
	StartDate string             `json:"start_date"`
	EndDate   string             `json:"end_date"`
	UserCount int                `json:"user_count"`
	Stats     AttendanceStatsDTO `json:"stats"`
}

type AttendanceStatsDTO struct {
	TotalDays              int   `json:"total_days"`
	WorkDays               int   `json:"work_days"`
	RestDays               int   `json:"rest_days"`
	NormalDays             int   `json:"normal_days"`
	AbnormalDays           int   `json:"abnormal_days"`
	LateDays               int   `json:"late_days"`
	EarlyLeaveDays         int   `json:"early_leave_days"`
	LateAndEarlyLeaveDays  int   `json:"late_and_early_leave_days"`
	MissingCheckInDays     int   `json:"missing_check_in_days"`
	MissingCheckOutDays    int   `json:"missing_check_out_days"`
	AbsentDays             int   `json:"absent_days"`
	RecordCount            int   `json:"record_count"`
	SnapshotCount          int   `json:"snapshot_count"`
	TotalLateSeconds       int64 `json:"total_late_seconds"`
	TotalEarlyLeaveSeconds int64 `json:"total_early_leave_seconds"`
}

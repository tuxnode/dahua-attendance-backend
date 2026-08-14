ALTER TABLE monthly_attendance_results
    ADD COLUMN correction_type VARCHAR(32) NOT NULL DEFAULT '' AFTER correction_status;

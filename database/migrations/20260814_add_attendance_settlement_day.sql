ALTER TABLE attendance_settings
    ADD COLUMN settlement_day TINYINT UNSIGNED NOT NULL DEFAULT 1 AFTER weekend_days;

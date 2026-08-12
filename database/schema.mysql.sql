CREATE TABLE IF NOT EXISTS attendance_records (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    device_sn VARCHAR(64) NOT NULL,
    user_id VARCHAR(64) NOT NULL DEFAULT '',
    card_name VARCHAR(128) NOT NULL DEFAULT '',
    card_no VARCHAR(64) NOT NULL DEFAULT '',
    method INT NOT NULL DEFAULT 0,
    direction VARCHAR(16) NOT NULL DEFAULT '',
    status INT NOT NULL DEFAULT 0,
    event_time DATETIME NOT NULL,
    create_time BIGINT NOT NULL DEFAULT 0,
    utc BIGINT NOT NULL DEFAULT 0,
    real_utc BIGINT NOT NULL DEFAULT 0,
    data_source VARCHAR(32) NOT NULL DEFAULT '',
    channel_index INT NOT NULL DEFAULT 0,
    door INT NOT NULL DEFAULT 0,
    reader_id VARCHAR(32) NOT NULL DEFAULT '',
    card_type INT NOT NULL DEFAULT 0,
    user_type INT NOT NULL DEFAULT 0,
    error_code INT NOT NULL DEFAULT 0,
    block_id BIGINT NOT NULL DEFAULT 0,
    dedup_block_id BIGINT NULL,
    image_count INT NOT NULL DEFAULT 0,
    raw_event JSON NULL,
    received_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_attendance_event_time (event_time),
    KEY idx_attendance_user_event_time (user_id, event_time),
    KEY idx_attendance_device_event_time (device_sn, event_time),
    KEY idx_attendance_block_id (block_id),
    UNIQUE KEY uniq_attendance_device_block (device_sn, dedup_block_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS door_status_records (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    device_sn VARCHAR(64) NOT NULL,
    status VARCHAR(16) NOT NULL,
    event_time DATETIME NOT NULL,
    utc BIGINT NOT NULL DEFAULT 0,
    real_utc BIGINT NOT NULL DEFAULT 0,
    data_source VARCHAR(32) NOT NULL DEFAULT '',
    channel_index INT NOT NULL DEFAULT 0,
    raw_event JSON NULL,
    received_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_door_status_event_time (event_time),
    KEY idx_door_status_device_event_time (device_sn, event_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS attendance_settings (
    id TINYINT UNSIGNED NOT NULL PRIMARY KEY,
    timezone VARCHAR(64) NOT NULL DEFAULT 'Asia/Shanghai',
    default_shift_id VARCHAR(64) NOT NULL DEFAULT 'day',
    weekend_days VARCHAR(64) NOT NULL DEFAULT 'saturday,sunday',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO attendance_settings (id, timezone, default_shift_id, weekend_days)
VALUES (1, 'Asia/Shanghai', 'day', 'saturday,sunday')
ON DUPLICATE KEY UPDATE id = id;

CREATE TABLE IF NOT EXISTS attendance_shifts (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    name VARCHAR(128) NOT NULL DEFAULT '',
    start_time CHAR(5) NOT NULL DEFAULT '09:00',
    end_time CHAR(5) NOT NULL DEFAULT '18:00',
    late_grace_minutes INT NOT NULL DEFAULT 0,
    early_leave_grace_minutes INT NOT NULL DEFAULT 0,
    flexible_minutes INT NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_attendance_shifts_enabled (enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO attendance_shifts (
    id,
    name,
    start_time,
    end_time,
    late_grace_minutes,
    early_leave_grace_minutes,
    flexible_minutes,
    enabled
) VALUES (
    'day',
    'Day Shift',
    '09:00',
    '18:00',
    0,
    0,
    0,
    TRUE
) ON DUPLICATE KEY UPDATE id = id;

CREATE TABLE IF NOT EXISTS attendance_calendar_days (
    calendar_date DATE NOT NULL PRIMARY KEY,
    day_type VARCHAR(16) NOT NULL,
    name VARCHAR(128) NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_attendance_calendar_day_type (day_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS attendance_schedules (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL DEFAULT '',
    device_sn VARCHAR(64) NOT NULL DEFAULT '',
    schedule_date DATE NOT NULL,
    shift_id VARCHAR(64) NOT NULL DEFAULT '',
    rest BOOLEAN NOT NULL DEFAULT FALSE,
    reason VARCHAR(128) NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uniq_attendance_schedule_scope (user_id, device_sn, schedule_date),
    KEY idx_attendance_schedule_date (schedule_date),
    KEY idx_attendance_schedule_user_date (user_id, schedule_date),
    KEY idx_attendance_schedule_device_date (device_sn, schedule_date),
    KEY idx_attendance_schedule_enabled (enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS attendance_weekly_schedules (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL DEFAULT '',
    device_sn VARCHAR(64) NOT NULL DEFAULT '',
    weekday TINYINT NOT NULL,
    shift_id VARCHAR(64) NOT NULL DEFAULT '',
    rest BOOLEAN NOT NULL DEFAULT FALSE,
    reason VARCHAR(128) NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uniq_attendance_weekly_schedule_scope (user_id, device_sn, weekday),
    KEY idx_attendance_weekly_schedule_weekday (weekday),
    KEY idx_attendance_weekly_schedule_user_weekday (user_id, weekday),
    KEY idx_attendance_weekly_schedule_device_weekday (device_sn, weekday),
    KEY idx_attendance_weekly_schedule_enabled (enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

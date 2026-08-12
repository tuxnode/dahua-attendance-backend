# API 说明

本文档描述当前 Gin HTTP 服务对设备和前端暴露的接口。默认服务地址为 `http://127.0.0.1:8080`，以实际配置文件中的 `http.addr` 为准。

## 通用约定

- 请求和响应编码使用 UTF-8。
- 前端查询接口响应格式为 JSON。
- 时间戳字段使用 Unix 秒。
- 日期字段使用 `YYYY-MM-DD`，月份字段使用 `YYYY-MM`。
- `limit` 默认 `100`，最大 `500`；`offset` 小于 `0` 时按 `0` 处理。
- 日报、汇总、异常列表的日期范围最大为 `31` 天。

错误响应格式：

```json
{
  "code": "invalid_request",
  "message": "date must use YYYY-MM-DD format"
}
```

常见状态码：

| HTTP 状态码 | 说明 |
| --- | --- |
| `200` | 请求成功。 |
| `400` | 查询参数格式错误。 |
| `500` | 服务未配置、数据库查询失败或业务处理失败。 |

## 健康检查

### `GET /healthz`

用于探活。

响应：

```text
ok
```

## 设备上报

### `POST /`

设备默认上报入口。

### `POST /api/v1/device/events`

设备上报入口，和 `POST /` 使用同一套处理逻辑。

请求头：

| Header | 必填 | 说明 |
| --- | --- | --- |
| `Content-Type` | 是 | 支持 `application/json`、`multipart/x-mixed-replace`、`multipart/form-data`。 |
| `Content-Encoding` | 否 | 支持空值和 `deflate`。 |

请求体由设备上报，支持：

- `AccessControl`：通行记录，写入 `attendance_records`。
- `DoorStatus`：门状态记录，写入 `door_status_records`。
- multipart 抓拍报文：解析文本事件，并保存事件中的图片数量统计。

成功响应：

```json
{
  "code": 0,
  "message": "success"
}
```

说明：设备报文解析失败时，服务仍会返回成功响应，避免设备持续重试；业务写库失败时返回 `500`。

## 查询考勤原始记录

### `GET /api/v1/attendance/records`

查询已入库的通行记录。

查询参数：

| 参数 | 必填 | 格式 | 说明 |
| --- | --- | --- | --- |
| `user_id` | 否 | string | 用户 ID。 |
| `device_sn` | 否 | string | 设备 SN。 |
| `start_time` | 否 | Unix 秒 | 起始事件时间。 |
| `end_time` | 否 | Unix 秒 | 结束事件时间，不能早于 `start_time`。 |
| `limit` | 否 | int | 分页大小。 |
| `offset` | 否 | int | 分页偏移。 |

示例：

```bash
curl "http://127.0.0.1:8080/api/v1/attendance/records?user_id=REDACTED_USER_ID&start_time=1786333800&end_time=1786420200"
```

响应：

```json
{
  "records": [
    {
      "user_id": "REDACTED_USER_ID",
      "user_name": "REDACTED_NAME",
      "device_sn": "REDACTED_DEVICE_SN",
      "direction": "Entry",
      "method": 15,
      "method_name": "face_open",
      "status": 1,
      "event_time": 1786333807,
      "received_at": 1786333925,
      "has_snapshot": true,
      "snapshot_count": 1
    }
  ]
}
```

字段说明：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `user_id` | string | 用户 ID。 |
| `user_name` | string | 用户姓名。 |
| `device_sn` | string | 设备 SN。 |
| `direction` | string | 通行方向，例如 `Entry`、`Exit`。 |
| `method` | int | 设备上报的开门方式编码。 |
| `method_name` | string | 开门方式名称，例如 `card`、`password`、`face`、`button`、`face_open`、`unknown`。 |
| `status` | int | 设备上报的通行状态，`1` 表示成功。 |
| `event_time` | int | 事件发生时间，Unix 秒。 |
| `received_at` | int | 服务接收时间，Unix 秒。 |
| `has_snapshot` | bool | 是否包含抓拍图片。 |
| `snapshot_count` | int | 抓拍图片数量。 |

## 查询考勤日报

### `GET /api/v1/attendance/daily`

按考勤规则生成日报。日报会基于数据库中的时区、班次、周末、节假日、工作日覆盖和排班计算。

查询参数：

| 参数 | 必填 | 格式 | 说明 |
| --- | --- | --- | --- |
| `user_id` | 否 | string | 用户 ID。传入后，即使当天无记录也会返回缺勤或休息日结果。 |
| `device_sn` | 否 | string | 设备 SN。 |
| `date` | 否 | `YYYY-MM-DD` | 查询单日；不能和 `start_date`、`end_date` 同时使用。 |
| `start_date` | 否 | `YYYY-MM-DD` | 起始日期。 |
| `end_date` | 否 | `YYYY-MM-DD` | 结束日期，不能早于 `start_date`。 |
| `limit` | 否 | int | 分页大小。 |
| `offset` | 否 | int | 分页偏移。 |

示例：

```bash
curl "http://127.0.0.1:8080/api/v1/attendance/daily?user_id=REDACTED_USER_ID&date=2026-08-10"
```

响应：

```json
{
  "records": [
    {
      "date": "2026-08-10",
      "user_id": "REDACTED_USER_ID",
      "user_name": "REDACTED_NAME",
      "device_sn": "REDACTED_DEVICE_SN",
      "shift_id": "day",
      "shift_name": "Day Shift",
      "is_workday": true,
      "non_workday_reason": "",
      "status": "late",
      "exceptions": ["late"],
      "is_abnormal": true,
      "corrected": false,
      "correction_status": "",
      "correction_reason": "",
      "corrected_at": 0,
      "work_start_at": 1786323600,
      "work_end_at": 1786356000,
      "first_entry_at": 1786324500,
      "last_exit_at": 1786356600,
      "late_seconds": 900,
      "early_leave_seconds": 0,
      "record_count": 2,
      "snapshot_count": 1
    }
  ]
}
```

日报字段说明：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `date` | string | 考勤业务日期。 |
| `user_id` | string | 用户 ID。 |
| `user_name` | string | 用户姓名。 |
| `device_sn` | string | 设备 SN。 |
| `shift_id` | string | 命中的班次 ID。 |
| `shift_name` | string | 命中的班次名称。 |
| `is_workday` | bool | 是否为工作日。 |
| `non_workday_reason` | string | 非工作日原因，例如 `weekend`、`holiday`、`scheduled_rest`。 |
| `status` | string | 日报状态。 |
| `exceptions` | string[] | 异常类型列表。 |
| `is_abnormal` | bool | 是否异常；休息日不算异常。 |
| `corrected` | bool | 是否已通过补卡修正。 |
| `correction_status` | string | 补卡状态，例如 `applied`；未补卡时为空。 |
| `correction_reason` | string | 补卡原因。 |
| `corrected_at` | int | 补卡时间，Unix 秒；未补卡时为 `0`。 |
| `work_start_at` | int | 应上班时间，Unix 秒。 |
| `work_end_at` | int | 应下班时间，Unix 秒。 |
| `first_entry_at` | int | 当日首次入场时间，Unix 秒；无记录时为 `0`。 |
| `last_exit_at` | int | 当日最后出场时间，Unix 秒；无记录时为 `0`。 |
| `late_seconds` | int | 迟到秒数。 |
| `early_leave_seconds` | int | 早退秒数。 |
| `record_count` | int | 当日匹配到的通行记录数量。 |
| `snapshot_count` | int | 当日抓拍图片数量。 |

日报状态：

| 状态 | 说明 |
| --- | --- |
| `normal` | 正常。 |
| `corrected` | 已补卡修正，按非异常处理。 |
| `late` | 迟到。 |
| `early_leave` | 早退。 |
| `late_and_early_leave` | 迟到且早退。 |
| `missing_check_in` | 缺少上班打卡。 |
| `missing_check_out` | 缺少下班打卡。 |
| `absent` | 缺勤。 |
| `rest_day` | 休息日。 |

异常类型：

| 异常 | 说明 |
| --- | --- |
| `late` | 迟到。 |
| `early_leave` | 早退。 |
| `missing_check_in` | 缺少上班打卡。 |
| `missing_check_out` | 缺少下班打卡。 |
| `absent` | 缺勤。 |

## 查询考勤月报

### `GET /api/v1/attendance/monthly`

按用户聚合指定月份的考勤统计。

查询参数：

| 参数 | 必填 | 格式 | 说明 |
| --- | --- | --- | --- |
| `user_id` | 否 | string | 用户 ID。 |
| `device_sn` | 否 | string | 设备 SN。 |
| `month` | 否 | `YYYY-MM` | 查询月份；为空时使用当前月份。 |
| `limit` | 否 | int | 分页大小。 |
| `offset` | 否 | int | 分页偏移。 |

示例：

```bash
curl "http://127.0.0.1:8080/api/v1/attendance/monthly?month=2026-08"
```

响应：

```json
{
  "records": [
    {
      "month": "2026-08",
      "user_id": "REDACTED_USER_ID",
      "user_name": "REDACTED_NAME",
      "device_sn": "REDACTED_DEVICE_SN",
      "days": [
        {
          "date": "2026-08-10",
          "user_id": "REDACTED_USER_ID",
          "user_name": "REDACTED_NAME",
          "device_sn": "REDACTED_DEVICE_SN",
          "shift_id": "day",
          "shift_name": "Day Shift",
          "is_workday": true,
          "non_workday_reason": "",
          "status": "corrected",
          "exceptions": [],
          "is_abnormal": false,
          "corrected": true,
          "correction_status": "applied",
          "correction_reason": "manual correction",
          "corrected_at": 1786356000,
          "work_start_at": 1786323600,
          "work_end_at": 1786356000,
          "first_entry_at": 1786323600,
          "last_exit_at": 1786356000,
          "late_seconds": 0,
          "early_leave_seconds": 0,
          "record_count": 1,
          "snapshot_count": 0
        }
      ],
      "stats": {
        "total_days": 31,
        "work_days": 21,
        "rest_days": 10,
        "normal_days": 18,
        "abnormal_days": 3,
        "late_days": 1,
        "early_leave_days": 1,
        "late_and_early_leave_days": 0,
        "missing_check_in_days": 0,
        "missing_check_out_days": 1,
        "absent_days": 0,
        "record_count": 42,
        "snapshot_count": 20,
        "total_late_seconds": 300,
        "total_early_leave_seconds": 600
      }
    }
  ]
}
```

说明：`days` 为该用户当月按天的考勤明细，字段和日报接口一致，用于前端按天展示当天是否异常、是否已补卡。月报统计会基于这些按天结果聚合。

## 查询考勤汇总

### `GET /api/v1/attendance/summary`

聚合指定日期范围内的整体考勤统计。

查询参数：

| 参数 | 必填 | 格式 | 说明 |
| --- | --- | --- | --- |
| `user_id` | 否 | string | 用户 ID。 |
| `device_sn` | 否 | string | 设备 SN。 |
| `date` | 否 | `YYYY-MM-DD` | 查询单日；不能和 `start_date`、`end_date` 同时使用。 |
| `start_date` | 否 | `YYYY-MM-DD` | 起始日期。 |
| `end_date` | 否 | `YYYY-MM-DD` | 结束日期。 |

示例：

```bash
curl "http://127.0.0.1:8080/api/v1/attendance/summary?start_date=2026-08-01&end_date=2026-08-31"
```

响应：

```json
{
  "summary": {
    "start_date": "2026-08-01",
    "end_date": "2026-08-31",
    "user_count": 12,
    "stats": {
      "total_days": 372,
      "work_days": 252,
      "rest_days": 120,
      "normal_days": 230,
      "abnormal_days": 22,
      "late_days": 8,
      "early_leave_days": 4,
      "late_and_early_leave_days": 1,
      "missing_check_in_days": 3,
      "missing_check_out_days": 6,
      "absent_days": 2,
      "record_count": 504,
      "snapshot_count": 240,
      "total_late_seconds": 3600,
      "total_early_leave_seconds": 2400
    }
  }
}
```

统计字段说明：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `total_days` | int | 聚合的日报条数。 |
| `work_days` | int | 工作日天数。 |
| `rest_days` | int | 休息日天数。 |
| `normal_days` | int | 正常天数。 |
| `abnormal_days` | int | 异常天数，不包含休息日。 |
| `late_days` | int | 迟到天数。 |
| `early_leave_days` | int | 早退天数。 |
| `late_and_early_leave_days` | int | 迟到且早退天数。 |
| `missing_check_in_days` | int | 缺少上班打卡天数。 |
| `missing_check_out_days` | int | 缺少下班打卡天数。 |
| `absent_days` | int | 缺勤天数。 |
| `record_count` | int | 通行记录数量。 |
| `snapshot_count` | int | 抓拍图片数量。 |
| `total_late_seconds` | int | 总迟到秒数。 |
| `total_early_leave_seconds` | int | 总早退秒数。 |

## 查询考勤异常列表

### `GET /api/v1/attendance/exceptions`

返回指定日期范围内的异常日报，响应结构和日报接口一致。

查询参数：

| 参数 | 必填 | 格式 | 说明 |
| --- | --- | --- | --- |
| `user_id` | 否 | string | 用户 ID。 |
| `device_sn` | 否 | string | 设备 SN。 |
| `date` | 否 | `YYYY-MM-DD` | 查询单日；不能和 `start_date`、`end_date` 同时使用。 |
| `start_date` | 否 | `YYYY-MM-DD` | 起始日期。 |
| `end_date` | 否 | `YYYY-MM-DD` | 结束日期。 |
| `limit` | 否 | int | 分页大小。 |
| `offset` | 否 | int | 分页偏移。 |

示例：

```bash
curl "http://127.0.0.1:8080/api/v1/attendance/exceptions?start_date=2026-08-01&end_date=2026-08-31&limit=20"
```

响应：

```json
{
  "records": [
    {
      "date": "2026-08-10",
      "user_id": "REDACTED_USER_ID",
      "user_name": "REDACTED_NAME",
      "device_sn": "REDACTED_DEVICE_SN",
      "shift_id": "day",
      "shift_name": "Day Shift",
      "is_workday": true,
      "non_workday_reason": "",
      "status": "missing_check_out",
      "exceptions": ["missing_check_out"],
      "is_abnormal": true,
      "corrected": false,
      "correction_status": "",
      "correction_reason": "",
      "corrected_at": 0,
      "work_start_at": 1786323600,
      "work_end_at": 1786356000,
      "first_entry_at": 1786323300,
      "last_exit_at": 0,
      "late_seconds": 0,
      "early_leave_seconds": 0,
      "record_count": 1,
      "snapshot_count": 1
    }
  ]
}
```

## 补卡

### `POST /api/v1/attendance/corrections`

接收补卡请求。服务会保留原始设备打卡记录不变，新增补卡记录，并把对应用户、对应日期的月报按天结果写入 `monthly_attendance_results`，标记为已补卡且非异常。后续日报、月报、异常列表查询会读取该修正状态。

请求参数：

| 字段 | 必填 | 格式 | 说明 |
| --- | --- | --- | --- |
| `user_id` | 是 | string | 用户 ID。 |
| `device_sn` | 否 | string | 设备 SN；为空时表示不限定设备。 |
| `date` | 是 | `YYYY-MM-DD` | 需要补卡的考勤业务日期。 |
| `type` | 是 | string | 补卡类型，支持 `check_in`、`check_out`。 |
| `corrected_at` | 是 | Unix 秒 | 补卡时间。 |
| `reason` | 否 | string | 补卡原因。 |

请求示例：

```bash
curl -X POST "http://127.0.0.1:8080/api/v1/attendance/corrections" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "REDACTED_USER_ID",
    "device_sn": "REDACTED_DEVICE_SN",
    "date": "2026-08-10",
    "type": "check_out",
    "corrected_at": 1786356000,
    "reason": "manual correction"
  }'
```

响应：

```json
{
  "correction": {
    "id": 1,
    "user_id": "REDACTED_USER_ID",
    "device_sn": "REDACTED_DEVICE_SN",
    "date": "2026-08-10",
    "type": "check_out",
    "corrected_at": 1786356000,
    "reason": "manual correction",
    "status": "applied"
  }
}
```

补卡后的日报示例：

```json
{
  "records": [
    {
      "date": "2026-08-10",
      "user_id": "REDACTED_USER_ID",
      "status": "corrected",
      "exceptions": [],
      "is_abnormal": false,
      "corrected": true,
      "correction_status": "applied",
      "correction_reason": "manual correction",
      "corrected_at": 1786356000
    }
  ]
}
```

## 考勤规则管理

当前不使用鉴权。规则写入数据库后，日报、月报、汇总、异常列表会在查询时加载当前启用规则。

规则匹配优先级：

1. 按日期排班优先于按星期排班。
2. 指定 `user_id` 和 `device_sn` 的规则优先于只指定 `user_id` 或只指定 `device_sn` 的规则。
3. `user_id`、`device_sn` 均为空表示全局规则。
4. 未命中排班时，按日历覆盖、周末设置和默认班次计算。

### 设置

#### `GET /api/v1/attendance/settings`

响应：

```json
{
  "settings": {
    "timezone": "Asia/Shanghai",
    "default_shift_id": "day",
    "weekend_days": ["saturday", "sunday"]
  }
}
```

#### `PUT /api/v1/attendance/settings`

请求：

```json
{
  "timezone": "Asia/Shanghai",
  "default_shift_id": "day",
  "weekend_days": ["saturday", "sunday"]
}
```

### 班次

#### `GET /api/v1/attendance/shifts`

查询参数：

| 参数 | 必填 | 格式 | 说明 |
| --- | --- | --- | --- |
| `include_disabled` | 否 | bool | 是否包含已禁用班次。 |
| `limit` | 否 | int | 分页大小。 |
| `offset` | 否 | int | 分页偏移。 |

#### `POST /api/v1/attendance/shifts`

新增或更新班次。

#### `PUT /api/v1/attendance/shifts/{id}`

按路径 ID 新增或更新班次。请求体中的 `id` 为空时使用路径 ID；同时提供时必须一致。

请求：

```json
{
  "id": "night",
  "name": "Night Shift",
  "start_time": "21:00",
  "end_time": "06:00",
  "late_grace_minutes": 5,
  "early_leave_grace_minutes": 5,
  "flexible_minutes": 10,
  "enabled": true
}
```

响应：

```json
{
  "record": {
    "id": "night",
    "name": "Night Shift",
    "start_time": "21:00",
    "end_time": "06:00",
    "late_grace_minutes": 5,
    "early_leave_grace_minutes": 5,
    "flexible_minutes": 10,
    "enabled": true
  }
}
```

#### `DELETE /api/v1/attendance/shifts/{id}`

删除班次。成功返回 `204`。

迟到判断阈值为 `start_time + late_grace_minutes + flexible_minutes`，早退判断阈值为 `end_time - early_leave_grace_minutes`。`end_time` 小于等于 `start_time` 时按跨日班次处理。

### 日历覆盖

日历覆盖用于配置节假日、调休工作日和额外休息日。

#### `GET /api/v1/attendance/calendar-days`

查询参数：

| 参数 | 必填 | 格式 | 说明 |
| --- | --- | --- | --- |
| `date` | 否 | `YYYY-MM-DD` | 查询单日，不能和 `start_date`、`end_date` 同时使用。 |
| `start_date` | 否 | `YYYY-MM-DD` | 起始日期。 |
| `end_date` | 否 | `YYYY-MM-DD` | 结束日期。 |
| `day_type` | 否 | string | `holiday`、`workday`、`rest_day`。 |
| `limit` | 否 | int | 分页大小。 |
| `offset` | 否 | int | 分页偏移。 |

#### `POST /api/v1/attendance/calendar-days`

新增或更新日历覆盖。

#### `PUT /api/v1/attendance/calendar-days/{date}`

按路径日期新增或更新日历覆盖。

请求：

```json
{
  "date": "2026-10-01",
  "day_type": "holiday",
  "name": "national_day"
}
```

`day_type` 说明：

| 值 | 说明 |
| --- | --- |
| `holiday` | 节假日，非工作日。 |
| `rest_day` | 额外休息日，非工作日。 |
| `workday` | 调休工作日，覆盖周末或节假日。 |

#### `DELETE /api/v1/attendance/calendar-days/{date}`

删除日历覆盖。成功返回 `204`。

### 按日期排班

#### `GET /api/v1/attendance/schedules`

查询参数：

| 参数 | 必填 | 格式 | 说明 |
| --- | --- | --- | --- |
| `user_id` | 否 | string | 用户 ID。 |
| `device_sn` | 否 | string | 设备 SN。 |
| `date` | 否 | `YYYY-MM-DD` | 查询单日。 |
| `start_date` | 否 | `YYYY-MM-DD` | 起始日期。 |
| `end_date` | 否 | `YYYY-MM-DD` | 结束日期。 |
| `include_disabled` | 否 | bool | 是否包含已禁用排班。 |
| `limit` | 否 | int | 分页大小。 |
| `offset` | 否 | int | 分页偏移。 |

#### `POST /api/v1/attendance/schedules`

新增或更新按日期排班。`user_id` 和 `device_sn` 可同时为空，表示全局日期排班。

#### `PUT /api/v1/attendance/schedules/{id}`

按 ID 更新排班。

请求：

```json
{
  "user_id": "REDACTED_USER_ID",
  "device_sn": "REDACTED_DEVICE_SN",
  "date": "2026-08-10",
  "shift_id": "night",
  "rest": false,
  "reason": "",
  "enabled": true
}
```

排休示例：

```json
{
  "user_id": "REDACTED_USER_ID",
  "date": "2026-08-11",
  "shift_id": "",
  "rest": true,
  "reason": "scheduled_rest",
  "enabled": true
}
```

#### `DELETE /api/v1/attendance/schedules/{id}`

删除按日期排班。成功返回 `204`。

### 按星期排班

#### `GET /api/v1/attendance/weekly-schedules`

查询参数：

| 参数 | 必填 | 格式 | 说明 |
| --- | --- | --- | --- |
| `user_id` | 否 | string | 用户 ID。 |
| `device_sn` | 否 | string | 设备 SN。 |
| `weekday` | 否 | string | `sunday` 到 `saturday`，也支持 `sun` 到 `sat`、`0` 到 `6`。 |
| `include_disabled` | 否 | bool | 是否包含已禁用周排班。 |
| `limit` | 否 | int | 分页大小。 |
| `offset` | 否 | int | 分页偏移。 |

#### `POST /api/v1/attendance/weekly-schedules`

新增或更新按星期排班。`user_id` 和 `device_sn` 可同时为空，表示全局周排班。

#### `PUT /api/v1/attendance/weekly-schedules/{id}`

按 ID 更新周排班。

请求：

```json
{
  "user_id": "",
  "device_sn": "",
  "weekday": "monday",
  "shift_id": "day",
  "rest": false,
  "reason": "",
  "enabled": true
}
```

#### `DELETE /api/v1/attendance/weekly-schedules/{id}`

删除按星期排班。成功返回 `204`。

## 参数校验规则

- `start_time`、`end_time` 必须为正整数 Unix 秒。
- `end_time` 不能早于 `start_time`。
- `date` 不能和 `start_date`、`end_date` 同时传入。
- `date`、`start_date`、`end_date` 必须使用 `YYYY-MM-DD`。
- `end_date` 不能早于 `start_date`。
- 日报、汇总、异常列表日期范围不能超过 `31` 天。
- `month` 必须使用 `YYYY-MM`。
- `limit`、`offset` 必须为整数。
- `corrected_at` 必须为正整数 Unix 秒。
- 补卡 `type` 只支持 `check_in`、`check_out`。
- `start_time`、`end_time` 班次时间必须使用 `HH:MM`。
- `day_type` 只支持 `holiday`、`workday`、`rest_day`。
- `weekday` 支持英文全称、三字母缩写或 `0` 到 `6`。

## 敏感信息

接口字段中包含 `user_id`、`user_name`、`device_sn`，前端展示和日志采集需要按实际生产要求脱敏。服务内部日志不应输出真实姓名、用户 ID、设备 SN。

# Dahua Attendance Backend

适配大华 `DH-ASI41KH-M` 门禁设备的考勤事件接收服务。

支持集成至Nacos统一注册管理

服务接收门禁设备主动上报的通行记录、门状态和抓拍事件，解析后写入 MySQL，并通过 Gin HTTP API 为前端提供考勤记录、日报、月报、汇总、异常列表、补卡接收以及考勤规则管理。查询接口支持按 `user_id`、`user_name`、`device_sn` 过滤。Nacos 注册为可选能力。

## 功能

- 设备 HTTP 上报接入：支持 JSON、`multipart/x-mixed-replace`、`multipart/form-data` 和 `deflate`。
- 事件入库：`AccessControl` 写入 `attendance_records`，`DoorStatus` 写入 `door_status_records`。
- 考勤统计：支持日报、月报、汇总、异常列表，月报支持配置结算日并返回按天明细，也支持按姓名查询。
- 补卡修正：支持请假、出差和上下班打卡修正，保留原始设备记录不变，将补卡后的日期标记为已修正且非异常。
- 考勤规则：支持业务时区、上下班时间、周末/节假日、工作日覆盖、弹性打卡、多班次和排班，规则通过 Web 接口写入数据库。
- 服务注册：可选接入 Nacos Go SDK。

## 技术栈

- Go `1.26.5`
- Gin
- MySQL
- TOML
- Nacos Go SDK

## 快速开始

初始化数据库：

```bash
mysql -h 127.0.0.1 -u root -p dahua_attendance < database/schema.mysql.sql
```

已有数据库需要按顺序执行迁移：

```bash
mysql -h 127.0.0.1 -u root -p dahua_attendance < database/migrations/20260814_add_monthly_attendance_result_correction_type.sql
mysql -h 127.0.0.1 -u root -p dahua_attendance < database/migrations/20260814_add_attendance_settlement_day.sql
```

启动服务：

```bash
go run ./cmd/server -config configs/config.example.toml
```

配置文件必须显式指定，且 `database.dsn` 必须指向可访问的 MySQL。

检查工程：

```bash
go test ./...
```

## Docker

使用 docker-compose 启动应用与 MySQL：

```bash
docker compose up -d
```

默认监听 `http://127.0.0.1:8080`，健康检查为：

```bash
curl http://127.0.0.1:8080/healthz
```

停止服务：

```bash
docker compose down
```

Docker 默认配置文件为 [configs/config.docker.toml](configs/config.docker.toml)。

## 文档

- [配置文件说明](docs/CONFIG.md)
- [API 说明](docs/API.md)
- [设备协议反向说明](REVERSE.md)
- [数据库脚本](database/schema.mysql.sql)

## HTTP 接口

- `GET /healthz`：健康检查
- `POST /`：设备默认上报入口
- `POST /api/v1/device/events`：设备上报入口
- `GET /api/v1/attendance/records`：查询考勤原始记录
- `GET /api/v1/attendance/daily`：查询考勤日报
- `GET /api/v1/attendance/monthly`：查询考勤月报
- `GET /api/v1/attendance/summary`：查询考勤汇总
- `GET /api/v1/attendance/exceptions`：查询考勤异常列表
- `POST /api/v1/attendance/corrections`：接收补卡并修正当天考勤状态
- `GET/PUT /api/v1/attendance/settings`：查询和更新考勤设置
- `GET/POST/PUT/DELETE /api/v1/attendance/shifts`：管理班次
- `GET/POST/PUT/DELETE /api/v1/attendance/calendar-days`：管理节假日和调休
- `GET/POST/PUT/DELETE /api/v1/attendance/schedules`：管理按日期排班
- `GET/POST/PUT/DELETE /api/v1/attendance/weekly-schedules`：管理按星期排班

详细请求参数和响应字段见 [API 说明](docs/API.md)。日报、月报、汇总和异常列表都支持按 `user_name` 精确过滤。

## 数据库表

- `attendance_records`：设备上报的原始通行记录。
- `door_status_records`：设备上报的门状态记录。
- `attendance_corrections`：补卡记录。
- `monthly_attendance_results`：按月归档、按天存储的考勤状态，用于记录 `is_abnormal`、`corrected`、`correction_type` 等修正结果。
- `attendance_settings`、`attendance_shifts`、`attendance_calendar_days`、`attendance_schedules`、`attendance_weekly_schedules`：考勤规则配置。

## 目录结构

```text
.
├── api/          # 对外 DTO
├── cmd/          # 应用入口
├── configs/      # 配置示例
├── database/     # 数据库初始化脚本
├── docs/         # 项目文档
├── internal/     # 内部业务实现
└── REVERSE.md    # 设备上报协议说明
```

## 开发约定

- 业务逻辑放在 `internal/attendance/`。
- HTTP 入口放在 `internal/transport/http/`。
- 配置加载放在 `internal/config/`。
- 不要在示例、日志或提交内容中保留真实姓名、用户 ID、设备 SN 等生产敏感信息。

## License

本项目遵循 [MIT License](LICENSE)。

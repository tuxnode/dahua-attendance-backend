# Dahua Attendance Backend

适配大华 `DH-ASI41KH-M` 门禁设备的考勤事件接收服务。

项目面向门禁设备主动上报和前端查询场景，负责接收通行记录、门状态变更等事件，解析后写入 MySQL，并通过 HTTP API 为前端提供考勤记录查询能力。

## 项目状态

当前已完成设备 HTTP 上报接入、JSON/multipart 解析、考勤记录与门状态记录入库、基础去重、配置文件启动、Gin HTTP API、考勤日报/月报/汇总/异常列表查询、考勤班次与规则配置，以及 Nacos SDK 服务注册入口。

## 技术栈

- Go `1.26.5`
- MySQL
- TOML 配置文件
- Gin
- Nacos Go SDK
- HTTP POST
- JSON；包含抓拍图片时支持 `multipart/x-mixed-replace`、`multipart/form-data`

## 目录结构

```text
.
├── api/          # 对外服务契约和 DTO
├── cmd/          # 应用程序入口
├── configs/      # 配置文件示例
├── database/     # 数据库初始化脚本
├── internal/     # 业务实现及内部模块
├── REVERSE.md    # 门禁设备 HTTP 主动上传协议说明
├── go.mod        # Go 模块及依赖定义
└── go.sum        # 依赖校验信息
```

## 快速开始

### 环境要求

- Go `1.26.5` 或兼容版本
- MySQL
- 可访问 Nacos 的运行环境（启用服务注册/发现时）

### 初始化依赖

```bash
go mod download
```

### 初始化数据库

```bash
mysql -h 127.0.0.1 -u root -p dahua_attendance < database/schema.mysql.sql
```

### 启动服务

服务启动必须显式指定配置文件：

```bash
go run ./cmd/server -config configs/config.example.toml
```

配置文件中的 `database.dsn` 必须指向可访问的 MySQL；启动时会连接数据库并启用持久化写入。

### Docker 启动

使用 [docker-compose.yml](docker-compose.yml) 启动应用与 MySQL，数据库会自动初始化。应用镜像从 GHCR 拉取 `ghcr.io/tuxnode/dahua-attendance-backend:latest`：

```bash
docker compose up -d
```

服务启动后监听 `http://127.0.0.1:8080`，健康检查为 `GET /healthz`。停止并清理：

```bash
docker compose down
```

如需连入已存在的 MySQL，可修改 [configs/config.docker.toml](configs/config.docker.toml) 中的 `database.dsn`，并在 docker-compose.yml 中移除 `mysql` 服务后单独启动 `app`。

Docker 镜像默认使用 `configs/config.docker.toml`，该配置不包含 `[nacos]` 段，因此默认不会连接 Nacos。如需启用服务注册，在 `config.docker.toml` 中补充 `[nacos]` 配置并设置 `enabled = true`。

### 配置文件

参考 [`configs/config.example.toml`](configs/config.example.toml)：

```toml
[app]
name = "dahua-attendance-backend"
env = "dev"

[http]
addr = ":8080"
max_body_bytes = 10485760
read_header_timeout = "5s"
read_timeout = "15s"
write_timeout = "5s"
idle_timeout = "60s"
shutdown_timeout = "10s"

[database]
driver = "mysql"
dsn = "user:password@tcp(127.0.0.1:3306)/dahua_attendance?parseTime=true&charset=utf8mb4&loc=Local"
max_open_conns = 10
max_idle_conns = 5
conn_max_lifetime = "30m"
connect_timeout = "5s"

# Nacos is optional. Remove this section or keep enabled = false to run without Nacos.
[nacos]
enabled = false
address = "127.0.0.1:8848"
namespace = "public"
group = "DEFAULT_GROUP"
username = ""
password = ""
service_name = "dahua-attendance-backend"
ip = ""
port = 8080
cluster_name = "DEFAULT"
weight = 1.0
ephemeral = true
timeout_ms = 5000
log_dir = ""
cache_dir = ""
log_level = "info"

[log]
level = "info"
file_path = ""
```

`log.file_path` 为空时日志输出到标准输出。配置文件缺失、`app.name` 为空、`database.dsn` 为空或日志级别非法时，服务会启动失败。

`[nacos]` 为可选配置段。未配置 `[nacos]` 或 `nacos.enabled = false` 时，服务不会创建 Nacos SDK client，也不会连接或注册到 Nacos。启用 Nacos 注册时，将 `nacos.enabled` 设置为 `true`，并填写前端或网关可访问的 `nacos.ip` 和 `nacos.port`。

`[attendance]` 用于配置考勤规则：

```toml
[attendance]
timezone = "Asia/Shanghai"
default_shift_id = "day"
weekend_days = ["saturday", "sunday"]
workdays = ["2026-09-26"]

[[attendance.holidays]]
date = "2026-10-01"
name = "national_day"

[[attendance.shifts]]
id = "day"
name = "Day Shift"
start_time = "09:00"
end_time = "18:00"
late_grace_minutes = 5
early_leave_grace_minutes = 5
flexible_minutes = 10
enabled = true

[[attendance.shifts]]
id = "night"
name = "Night Shift"
start_time = "21:00"
end_time = "06:00"
late_grace_minutes = 5
early_leave_grace_minutes = 5
flexible_minutes = 0
enabled = true

[[attendance.schedules]]
user_id = "REDACTED_USER_ID"
date = "2026-08-10"
shift_id = "night"

[[attendance.weekly_schedules]]
weekday = "monday"
shift_id = "day"
```

`timezone` 决定日报、月报、汇总和异常统计使用的业务时区。`weekend_days`、`workdays`、`holidays` 用于覆盖默认周末规则。`shifts` 定义班次上下班时间、迟到/早退宽限和弹性打卡。`schedules` 与 `weekly_schedules` 用于指定某人、某设备或某周几对应的班次，支持多班次/排班。

### 检查工程

```bash
go test ./...
```

## HTTP 接口

- `GET /healthz`：健康检查
- `POST /`：设备默认上报入口
- `POST /api/v1/device/events`：设备上报入口
- `GET /api/v1/attendance/records`：前端查询考勤记录
- `GET /api/v1/attendance/daily`：前端查询考勤日报
- `GET /api/v1/attendance/monthly`：前端查询考勤月报
- `GET /api/v1/attendance/summary`：前端查询考勤汇总
- `GET /api/v1/attendance/exceptions`：前端查询考勤异常列表

设备上报请求会被解析为 `AccessControl` 或 `DoorStatus` 事件，并分别写入 `attendance_records` 和 `door_status_records`。

考勤记录查询 `GET /api/v1/attendance/records` 支持以下查询参数：

```text
user_id
device_sn
start_time
end_time
limit
offset
```

考勤日报查询 `GET /api/v1/attendance/daily` 支持以下查询参数：

```text
user_id
device_sn
date
start_date
end_date
limit
offset
```

`date`、`start_date` 和 `end_date` 使用 `YYYY-MM-DD` 格式。传入 `date` 时查询单日；传入 `start_date` 和 `end_date` 时查询日期范围。

考勤月报查询 `GET /api/v1/attendance/monthly` 支持以下查询参数：

```text
user_id
device_sn
month
limit
offset
```

`month` 使用 `YYYY-MM` 格式，例如 `2026-08`。

考勤汇总查询 `GET /api/v1/attendance/summary` 支持以下查询参数：

```text
user_id
device_sn
date
start_date
end_date
```

考勤异常列表 `GET /api/v1/attendance/exceptions` 支持以下查询参数：

```text
user_id
device_sn
date
start_date
end_date
limit
offset
```

## 设备协议

门禁设备通过 HTTP POST 主动推送事件，后端需要在 3 秒内返回 HTTP `200 OK`，并返回如下 JSON：

```json
{
  "code": 0,
  "message": "success"
}
```

完整的字段定义、事件类型和报文示例请参阅 [`REVERSE.md`](REVERSE.md)。

## 服务注册

启用 `nacos.enabled` 后，服务会通过 Nacos Go SDK 注册当前 Gin HTTP 服务实例。未启用或未配置 `[nacos]` 时会跳过 Nacos 连接。Nacos 仅用于服务注册与发现。

## 开发说明

- 新增应用入口放置于 `cmd/`。
- 业务逻辑和内部依赖放置于 `internal/`。
- 不要在示例报文中提交真实姓名、用户编号、设备序列号或其他生产数据。

## License

本项目遵循 [MIT License](LICENSE)。

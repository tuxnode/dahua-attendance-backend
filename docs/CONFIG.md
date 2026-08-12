# 配置文件说明

服务启动必须显式指定 TOML 配置文件：

```bash
go run ./cmd/server -config configs/config.example.toml
```

示例配置见 [configs/config.example.toml](../configs/config.example.toml)，Docker 默认配置见 [configs/config.docker.toml](../configs/config.docker.toml)。

## 必填项

最小可用配置需要包含 `app.name` 和 `database.dsn`：

```toml
[app]
name = "dahua-attendance-backend"

[database]
dsn = "user:password@tcp(127.0.0.1:3306)/dahua_attendance?parseTime=true&charset=utf8mb4&loc=Local"
```

未配置的可选字段会使用默认值。`database.dsn` 为空、配置文件路径为空、日志级别非法、启用 Nacos 但缺少注册地址信息时，服务会启动失败。

## app

```toml
[app]
name = "dahua-attendance-backend"
env = "dev"
```

| 字段 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `name` | 是 | 无 | 应用名称；Nacos `service_name` 未配置时会使用该值。 |
| `env` | 否 | `dev` | 运行环境标识，例如 `dev`、`test`、`prod`、`docker`。 |

## http

```toml
[http]
addr = ":8080"
max_body_bytes = 10485760
read_header_timeout = "5s"
read_timeout = "15s"
write_timeout = "5s"
idle_timeout = "60s"
shutdown_timeout = "10s"
```

| 字段 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `addr` | 否 | `:8080` | Gin HTTP 服务监听地址。 |
| `max_body_bytes` | 否 | `10485760` | 单个设备上报请求最大 Body 字节数。 |
| `read_header_timeout` | 否 | `5s` | 读取 HTTP Header 超时时间。 |
| `read_timeout` | 否 | `15s` | 读取完整请求超时时间。 |
| `write_timeout` | 否 | `5s` | 写响应超时时间。 |
| `idle_timeout` | 否 | `60s` | Keep-Alive 空闲连接超时时间。 |
| `shutdown_timeout` | 否 | `10s` | 收到退出信号后的优雅关闭超时时间。 |

时间字段使用 Go duration 格式，例如 `500ms`、`5s`、`1m`。

## database

```toml
[database]
driver = "mysql"
dsn = "user:password@tcp(127.0.0.1:3306)/dahua_attendance?parseTime=true&charset=utf8mb4&loc=Local"
max_open_conns = 10
max_idle_conns = 5
conn_max_lifetime = "30m"
connect_timeout = "5s"
```

| 字段 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `driver` | 否 | `mysql` | 数据库驱动，目前使用 MySQL。 |
| `dsn` | 是 | 无 | MySQL 连接串。建议包含 `parseTime=true`。 |
| `max_open_conns` | 否 | `10` | 最大打开连接数。 |
| `max_idle_conns` | 否 | `5` | 最大空闲连接数；大于 `max_open_conns` 时会被修正。 |
| `conn_max_lifetime` | 否 | `30m` | 连接最大生命周期。 |
| `connect_timeout` | 否 | `5s` | 启动时数据库连通性检查超时时间。 |

## 考勤规则

考勤规则不再通过配置文件维护。服务启动配置只包含 `app`、`http`、`database`、`nacos`、`log` 等基础项；业务时区、默认班次、周末、节假日、工作日覆盖、班次、个人/设备排班通过 HTTP API 写入数据库。

初始化数据库脚本会创建规则表，并写入默认设置：

- `attendance_settings`：默认 `Asia/Shanghai`、默认班次 `day`、周六周日休息。
- `attendance_shifts`：默认 `day` 班次，`09:00-18:00`。
- `attendance_calendar_days`：节假日、调休工作日、额外休息日。
- `attendance_schedules`：按日期排班，支持 `user_id`、`device_sn` 作用范围。
- `attendance_weekly_schedules`：按星期排班，支持 `user_id`、`device_sn` 作用范围。

规则管理接口见 [API 说明](API.md#考勤规则管理)。

## nacos

`nacos` 为可选配置段。未配置或 `enabled = false` 时服务不会连接 Nacos。

```toml
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
```

| 字段 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `enabled` | 否 | `false` | 是否启用 Nacos 注册。 |
| `address` | 否 | `127.0.0.1:8848` | Nacos 地址。 |
| `namespace` | 否 | `public` | Nacos 命名空间。 |
| `group` | 否 | `DEFAULT_GROUP` | Nacos 分组。 |
| `username` | 否 | 空 | Nacos 用户名。 |
| `password` | 否 | 空 | Nacos 密码。 |
| `service_name` | 否 | `app.name` | 注册到 Nacos 的服务名。 |
| `ip` | 启用时必填 | 空 | 注册实例 IP，应填写前端或网关可访问地址。 |
| `port` | 启用时必填 | `http.addr` 端口 | 注册实例端口。 |
| `cluster_name` | 否 | `DEFAULT` | Nacos 集群名。 |
| `weight` | 否 | `1.0` | 实例权重。 |
| `ephemeral` | 否 | `true` | 是否注册为临时实例。 |
| `timeout_ms` | 否 | `5000` | Nacos SDK 请求超时时间，单位毫秒。 |
| `log_dir` | 否 | 空 | Nacos SDK 日志目录。 |
| `cache_dir` | 否 | 空 | Nacos SDK 缓存目录。 |
| `log_level` | 否 | `info` | Nacos SDK 日志级别，支持 `debug`、`info`、`warn`、`error`。 |

## log

```toml
[log]
level = "info"
file_path = ""
```

| 字段 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `level` | 否 | `info` | 服务日志级别，支持 `debug`、`info`、`warn`、`error`。 |
| `file_path` | 否 | 空 | 日志文件路径。为空时输出到标准输出。 |

日志中不应输出真实姓名、用户 ID、设备 SN 等敏感字段。

## Docker 配置

[configs/config.docker.toml](../configs/config.docker.toml) 默认连接 docker-compose 中的 MySQL：

```toml
[database]
dsn = "attendance:attendance_password@tcp(mysql:3306)/dahua_attendance?parseTime=true&charset=utf8mb4&loc=Local"
```

该配置默认不包含 `[nacos]` 段，因此容器启动时不会连接 Nacos。需要服务注册时，在 Docker 配置中补充 `[nacos]` 并设置 `enabled = true`。

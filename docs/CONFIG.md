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

## attendance

`attendance` 控制日报、月报、汇总、异常列表的业务日期和考勤规则。

```toml
[attendance]
timezone = "Asia/Shanghai"
default_shift_id = "day"
weekend_days = ["saturday", "sunday"]
workdays = ["2026-09-26"]
```

| 字段 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `timezone` | 否 | `Asia/Shanghai` | 考勤业务时区；设备事件会按该时区归属业务日期。 |
| `default_shift_id` | 否 | `day` | 默认班次 ID。 |
| `weekend_days` | 否 | `["saturday", "sunday"]` | 默认休息日星期列表。支持英文全称、三字母缩写或 `0`-`6`。 |
| `workdays` | 否 | 空 | 工作日覆盖列表。用于把周末或节假日改为上班日。 |

### 节假日

```toml
[[attendance.holidays]]
date = "2026-10-01"
name = "national_day"
```

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `date` | 是 | 日期，格式 `YYYY-MM-DD`。 |
| `name` | 否 | 休息原因；为空时统计结果使用 `holiday`。 |

规则优先级中，`workdays` 高于 `holidays` 和 `weekend_days`。

### 班次

```toml
[[attendance.shifts]]
id = "day"
name = "Day Shift"
start_time = "09:00"
end_time = "18:00"
late_grace_minutes = 5
early_leave_grace_minutes = 5
flexible_minutes = 10
enabled = true
```

| 字段 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `id` | 是 | 无 | 班次 ID，需要和 `default_shift_id`、排班中的 `shift_id` 对应。 |
| `name` | 否 | `id` | 班次名称，会返回给前端。 |
| `start_time` | 否 | `09:00` | 上班时间，格式 `HH:MM`。 |
| `end_time` | 否 | `18:00` | 下班时间，格式 `HH:MM`；小于等于上班时间时按跨日班次处理。 |
| `late_grace_minutes` | 否 | `0` | 迟到宽限分钟数。 |
| `early_leave_grace_minutes` | 否 | `0` | 早退宽限分钟数。 |
| `flexible_minutes` | 否 | `0` | 弹性打卡分钟数，会叠加到迟到判断阈值。 |
| `enabled` | 否 | `true` | 是否启用该班次。显式写 `false` 时该班次不会参与规则匹配。 |

迟到判断阈值为 `start_time + late_grace_minutes + flexible_minutes`。早退判断阈值为 `end_time - early_leave_grace_minutes`。

跨日班次示例：

```toml
[[attendance.shifts]]
id = "night"
name = "Night Shift"
start_time = "21:00"
end_time = "06:00"
late_grace_minutes = 5
early_leave_grace_minutes = 5
enabled = true
```

## 排班

排班用于覆盖默认工作日和默认班次，支持按日期和按星期配置。匹配优先级为：指定用户和设备 > 指定用户 > 指定设备 > 全局规则。

### 按日期排班

```toml
[[attendance.schedules]]
user_id = "REDACTED_USER_ID"
device_sn = ""
date = "2026-08-10"
shift_id = "night"
rest = false
reason = ""
```

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `user_id` | 否 | 用户 ID；为空表示不限用户。 |
| `device_sn` | 否 | 设备 SN；为空表示不限设备。 |
| `date` | 是 | 生效日期，格式 `YYYY-MM-DD`。 |
| `shift_id` | 否 | 上班日使用的班次 ID；为空时使用默认班次。 |
| `rest` | 否 | 是否排休。为 `true` 时当天状态为 `rest_day`。 |
| `reason` | 否 | 排休原因；为空时使用 `scheduled_rest`。 |

### 按星期排班

```toml
[[attendance.weekly_schedules]]
user_id = ""
device_sn = ""
weekday = "monday"
shift_id = "day"
rest = false
reason = ""
```

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `user_id` | 否 | 用户 ID；为空表示不限用户。 |
| `device_sn` | 否 | 设备 SN；为空表示不限设备。 |
| `weekday` | 是 | 星期。支持 `sunday` 到 `saturday`、`sun` 到 `sat`、`0` 到 `6`。 |
| `shift_id` | 否 | 上班日使用的班次 ID。 |
| `rest` | 否 | 是否排休。 |
| `reason` | 否 | 排休原因。 |

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

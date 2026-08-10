# Dahua Attendance Backend

适配大华 `DH-ASI41KH-M` 门禁设备的考勤事件接收服务。

项目面向门禁设备主动上报场景，负责接收通行记录、门状态变更等事件，解析后写入 MySQL，并为后续通过 Dubbo-Go/Nacos 向 Web 提供考勤记录查询服务打基础。

## 项目状态

当前已完成设备 HTTP 上报接入、JSON/multipart 解析、考勤记录与门状态记录入库、基础去重、配置文件启动，以及 Web 查询服务的 Provider 适配层。

Dubbo-Go 与 Nacos 的真实服务注册/发现尚未接入。

## 技术栈

- Go `1.26.5`
- MySQL
- TOML 配置文件
- Dubbo-Go / Nacos（后续接入服务注册与发现）
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

[log]
level = "info"
file_path = ""
```

`log.file_path` 为空时日志输出到标准输出。配置文件缺失、`app.name` 为空、`database.dsn` 为空或日志级别非法时，服务会启动失败。

### 检查工程

```bash
go test ./...
```

## HTTP 接口

- `GET /healthz`：健康检查
- `POST /`：设备默认上报入口
- `POST /api/v1/device/events`：设备上报入口

设备上报请求会被解析为 `AccessControl` 或 `DoorStatus` 事件，并分别写入 `attendance_records` 和 `door_status_records`。

## 设备协议

门禁设备通过 HTTP POST 主动推送事件，后端需要在 3 秒内返回 HTTP `200 OK`，并返回如下 JSON：

```json
{
  "code": 0,
  "message": "success"
}
```

完整的字段定义、事件类型和报文示例请参阅 [`REVERSE.md`](REVERSE.md)。

## 服务契约

`api/attendance/v1` 定义 Web 查询侧使用的请求、响应和 DTO。

`internal/transport/dubbo` 已实现 `AttendanceProvider` 适配层，负责把外部查询请求转换为内部 `attendance/service` 查询。当前尚未启动真实 Dubbo-Go Provider，也尚未注册到 Nacos。

## 开发说明

- 新增应用入口放置于 `cmd/`。
- 业务逻辑和内部依赖放置于 `internal/`。
- 不要在示例报文中提交真实姓名、用户编号、设备序列号或其他生产数据。

## License

本项目遵循 [MIT License](LICENSE)。

# Dahua Attendance Backend

适配大华 `DH-ASI41KH-M` 门禁设备的考勤事件接收服务。

项目面向门禁设备主动上报场景，负责接收通行记录、门状态变更等事件，为后续的考勤处理、数据存储和业务集成提供基础。

## 项目状态

当前仓库处于基础工程阶段，已完成 Go 模块初始化和设备上报协议整理，业务处理能力将按实际接入需求逐步完善。

## 技术栈

- Go `1.26.5`
- Nacos Go SDK `v2.3.5`
- HTTP POST
- JSON；包含抓拍图片时支持 `multipart/form-data`

## 目录结构

```text
.
├── cmd/          # 应用程序入口
├── internal/     # 业务实现及内部模块
├── REVERSE.md    # 门禁设备 HTTP 主动上传协议说明
├── go.mod        # Go 模块及依赖定义
└── go.sum        # 依赖校验信息
```

## 快速开始

### 环境要求

- Go `1.26.5` 或兼容版本
- 可访问 Nacos 的运行环境（启用服务注册或配置管理时）

### 初始化依赖

```bash
go mod download
```

### 检查工程

当前仓库尚未包含可测试的 Go package。业务代码添加后，可使用以下命令运行全部测试：

```bash
go test ./...
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

## 开发说明

- 新增应用入口放置于 `cmd/`。
- 业务逻辑和内部依赖放置于 `internal/`。
- 不要在示例报文中提交真实姓名、用户编号、设备序列号或其他生产数据。

## License

本项目遵循 [MIT License](LICENSE)。

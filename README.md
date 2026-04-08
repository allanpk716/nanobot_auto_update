# Nanobot Auto Updater

[![Go Version](https://img.shields.io/badge/Go-1.24.11-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

> **这是一个给 Nanobot AI 助手使用的自动化更新工具** - 配置一次，永久自动运行，无需人工干预

## What It Does

Nanobot Auto Updater 是专为 [Nanobot](https://github.com/nicepkg/nanobot) AI 助手设计的自动化更新管理器：

- **双源更新** - 优先 GitHub（最新功能），失败自动回退 PyPI（稳定版本）
- **生命周期管理** - 安全停止、更新、重启 Nanobot 服务
- **HTTP API + 监控服务** - REST API 实时触发 + 自动网络恢复检测
- **实时日志查看** - Web UI 和 SSE 流式传输查看实例日志
- **Pushover 通知** - 可选推送更新状态到设备

## Quick Start

**1. Configure** -- 编辑 `config.yaml`（首次运行自动创建）：

```yaml
api:
  port: 8080
  bearer_token: "your-secret-token-at-least-32-characters-long"
monitor:
  interval: 15m
instances:
  - name: "nanobot-instance-1"
    port: 18790
    start_command: "nanobot gateway"
```

**2. Start** -- 启动服务：

```bash
./nanobot-auto-updater.exe
```

**3. Trigger update** -- 通过 API 触发更新：

```bash
curl -X POST http://localhost:8080/api/v1/trigger-update \
  -H "Authorization: Bearer your-secret-token-at-least-32-characters-long"

# 成功: {"success":true,"version":"1.2.3","source":"github"}
# 冲突: {"error":"Update already in progress","status":429}
```

## API Reference

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/trigger-update` | Bearer Token | 触发 Nanobot 更新（双源：GitHub -> PyPI） |
| GET | `/logs/{instance_name}` | - | Web UI 实时日志查看器 |
| GET | `/api/v1/logs/{instance_name}/stream` | - | SSE 实时日志流（stdout/stderr 事件） |
| GET | `/api/v1/update/progress` | Bearer Token | 查询当前更新进度百分比 |

## Configuration Overview

| Section | Required | Description |
|---------|----------|-------------|
| `api` | Required | HTTP API 服务：`port`、`bearer_token`（>=32 字符）、`timeout` |
| `monitor` | Required | 监控服务：`interval`（Google 连通性检查）、`timeout` |
| `instances` | Required | Nanobot 实例列表：`name`、`port`、`start_command`、`startup_timeout` |
| `pushover` | Optional | Pushover 通知：`api_token`、`user_key` |

Full configuration details: [docs/configuration.md](docs/configuration.md)

## CLI Reference

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `./config.yaml` | 配置文件路径 |
| `--api-port` | 8080 | 覆盖 API 端口 |
| `--skip-monitor` | false | 禁用监控服务（仅 API 模式） |
| `--version` | false | 显示版本信息 |
| `-h, --help` | - | 显示帮助信息 |

Usage scenarios and Pushover setup: [docs/usage-guide.md](docs/usage-guide.md)

## Requirements

- **OS**: Windows 10/11
- **Go**: 1.24+（仅构建时需要）
- **uv**: Python 包管理器（[安装指南](https://github.com/astral-sh/uv)）
- **Nanobot**: 已安装的 Nanobot 实例

## Documentation

| Document | Description |
|----------|-------------|
| [Architecture](docs/architecture.md) | v0.3 架构组件：HTTP API、Monitor、Shared Lock |
| [Real-time Log Viewer](docs/logs-viewer.md) | Web UI、SSE API、EventSource API、技术细节 |
| [Usage Guide](docs/usage-guide.md) | CLI 场景、Pushover 配置、安装方式 |
| [Update Flow](docs/update-flow.md) | Mermaid 流程图、详细步骤、设计决策 |
| [Configuration Reference](docs/configuration.md) | 完整配置示例和说明 |
| [Development Guide](docs/development.md) | 项目结构、构建命令、日志系统 |
| [Troubleshooting](docs/troubleshooting.md) | 9 个常见问题的排查和解决方案 |
| [Nanobot Usage](docs/nanobot-usage.md) | Nanobot AI 助手专用使用说明 |

## Contributing

1. Fork 本仓库
2. 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

遵循 Go 标准代码格式（`gofmt`），确保测试通过 (`make test`)。

## Changelog

查看 [CHANGELOG.md](CHANGELOG.md) 了解版本历史和变更记录。

## License

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。

## Acknowledgments

- [Nanobot](https://github.com/nicepkg/nanobot) - AI 助手项目
- [uv](https://github.com/astral-sh/uv) - Python 包管理器
- [robfig/cron](https://github.com/robfig/cron) - Cron 调度库

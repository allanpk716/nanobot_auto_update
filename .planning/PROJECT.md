# Nanobot Auto Updater

## What This Is

一个 Windows 后台程序，使用 Golang 开发，用于自动更新 nanobot 工具。程序通过 cron 定时任务检查并更新 nanobot，支持失败通知和回退机制。**v0.2 新增多实例支持**，可以同时管理多个 nanobot 实例的升级和启动。无界面运行，通过配置文件和命令行参数控制行为。

## Core Value

自动保持 nanobot 处于最新版本，无需用户手动干预。

## Requirements

### Validated

**v0.2 多实例支持** — 2026-03-16:
- ✓ 多实例配置 (YAML) — v0.2
- ✓ 实例名称和端口唯一性验证 — v0.2
- ✓ 停止/启动所有实例的生命周期管理 — v0.2
- ✓ 优雅降级 (某实例失败不影响其他实例) — v0.2
- ✓ 多实例失败通知 (包含详细错误信息) — v0.2
- ✓ 错误聚合和结构化报告 — v0.2

**v1.0 单实例自动更新** — 2026-02-18:
- ✓ 检测系统是否安装 uv 包管理器 — v1.0
- ✓ 按 cron 表达式定时执行更新任务 — v1.0
- ✓ 使用 uv 安装 nanobot GitHub 最新代码 — v1.0
- ✓ 更新失败时回退到 uv tool install nanobot-ai 稳定版 — v1.0
- ✓ 更新失败时通过 Pushover 通知用户 — v1.0
- ✓ 支持配置文件 (YAML) 配置运行参数 — v1.0
- ✓ 支持命令行参数覆盖配置 — v1.0
- ✓ 后台运行，隐藏控制台窗口 — v1.0
- ✓ 记录日志到文件，支持日志轮转 — v1.0
- ✓ --update-now 立即更新模式 (JSON 输出) — v1.0

### Active

(无 — 规划下一个里程碑时添加)

### Out of Scope

- GUI 界面 — 命令行工具，无需图形界面
- 更新历史记录 — 保持简单，不存储历史
- 开机自启动 — 用户手动启动
- 跨平台支持 — 仅支持 Windows

## Context

**nanobot**: 一个 AI Agent 工具，托管在 GitHub (https://github.com/HKUDS/nanobot)，支持通过 uv 包管理器安装。

**uv**: Python 包管理器，用于安装和管理 Python 工具。

**Pushover**: 推送通知服务，用于在更新失败时通知用户。

**v0.2 Shipped:** 2026-03-16 — 多实例支持里程碑完成，5 个阶段，7 个计划，8 个任务，~5000 LOC Go 代码。测试覆盖: 单元测试 + 集成测试 + E2E 测试。

**Tech Stack:** Golang, viper (YAML 配置), logrus (日志), cron (调度), Pushover API (通知)

**Key Patterns:**
- 上下文感知日志 (logger.With 预注入)
- 结构化错误包装 (InstanceError + 错误链)
- 优雅降级 (失败不中断整体流程)
- TDD 开发模式 (RED-GREEN-REFACTOR)
- 配置向后兼容 (legacy 模式自动检测)

## Constraints

- **平台**: Windows 操作系统 — 目标用户在 Windows 环境使用
- **语言**: Golang — 用户指定
- **日志库**: github.com/WQGroup/logger — 用户指定，基于 logrus
- **日志格式**: `2024-01-01 12:00:00.123 - [INFO]: message` — 需要自定义 formatter
- **配置格式**: YAML — 可读性好，支持注释

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| 隐藏窗口运行 | 后台服务，无需用户交互 | ✓ Good |
| YAML 配置文件 | 可读性好，支持注释，Go 生态支持完善 | ✓ Good |
| Cron 定时触发 | 灵活的时间配置，用户熟悉 | ✓ Good |
| Pushover 失败通知 | 简单可靠的通知方式 | ✓ Good |
| GitHub main 分支优先 | 获取最新功能和修复 | ✓ Good |
| 稳定版回退机制 | 保证更新失败时仍可用 | ✓ Good |
| 多实例模式检测 (len(Instances) > 0) | 清晰的模式切换,向后兼容 | ✓ Good — Phase 10 |
| InstanceError 错误链支持 | 中文消息 + errors.Is/As 遍历 | ✓ Good — Phase 7 |
| 优雅降级 (继续处理其他实例) | 部分失败不影响整体流程 | ✓ Good — Phase 8 |
| 条件通知模式 (仅失败时发送) | 避免不必要的通知打扰 | ✓ Good — Phase 9 |
| mapstructure 标签 (而非 yaml) | viper 使用 mapstructure 解析 | ✓ Good — Phase 6 |
| O(n) map-based 唯一性验证 | 避免嵌套循环,性能更好 | ✓ Good — Phase 6 |
| errors.Join 聚合所有验证错误 | 用户一次性看到所有配置问题 | ✓ Good — Phase 6 |
| 上下文感知日志 (logger.With 预注入) | 所有日志自动包含实例名和组件 | ✓ Good — Phase 7 |
| cmd /c 执行启动命令 | 支持管道和重定向等复杂命令 | ✓ Good — Phase 7 |
| 端口监听验证替代进程名 | 更精确的启动验证 | ✓ Good — Phase 7 |
| Double error checking (UV + Instance) | 分类错误并路由到正确通知 | ✓ Good — Phase 10 |
| Context timeout (--update-now only) | 定时模式无超时,立即更新可配置超时 | ✓ Good — Phase 10 |
| 1-based 实例序号 | 符合用户直觉,避免 0-based 困惑 | ✓ Good — Phase 10-02 |

## Configuration

**配置文件**: `./config.yaml`

**v1.0 Legacy 配置项** (向后兼容):
- cron: "0 3 * * *" (每天凌晨 3 点)
- pushover_token: "" (Pushover App Token)
- pushover_user: "" (Pushover User Key)
- nanobot.port: 18790 (nanobot 端口)
- nanobot.start_command: "启动命令" (nanobot 启动命令)
- nanobot.startup_timeout: 30 (启动超时秒数)

**v0.2 多实例配置** (新):
```yaml
instances:
  - name: "gateway"
    port: 18790
    start_command: "python -m nanobot.gateway"
    startup_timeout: 30
  - name: "worker"
    port: 18791
    start_command: "python -m nanobot.worker --port 18791"
    startup_timeout: 45
```

**命令行参数**:
- `-cron`: 覆盖配置文件中的 cron 表达式
- `-config`: 指定配置文件路径
- `--update-now`: 立即执行一次更新并退出 (JSON 输出)
- `--timeout`: 配置立即更新超时时间 (默认 5 分钟)
- `-version`: 显示版本信息
- `help`: 显示帮助信息

**注意**: 旧的 `-run-once` 参数已在 v1.0 中移除,替换为 `--update-now`

---
*Last updated: 2026-03-16 after v0.2 milestone completion*

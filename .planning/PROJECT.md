# Nanobot Auto Updater

## What This Is

一个 Windows 后台程序，使用 Golang 开发，用于自动更新 nanobot 工具。程序通过 cron 定时任务检查并更新 nanobot，支持失败通知和回退机制。无界面运行，通过配置文件和命令行参数控制行为。

## Core Value

自动保持 nanobot 处于最新版本，无需用户手动干预。

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] 检测系统是否安装 uv 包管理器
- [ ] 按 cron 表达式定时执行更新任务
- [ ] 使用 uv 安装 nanobot GitHub 最新代码
- [ ] 更新失败时回退到 uv tool install nanobot-ai 稳定版
- [ ] 更新失败时通过 Pushover 通知用户
- [ ] 支持配置文件 (YAML) 配置运行参数
- [ ] 支持命令行参数覆盖配置
- [ ] 后台运行，隐藏控制台窗口
- [ ] 记录日志到文件，支持日志轮转

### Out of Scope

- GUI 界面 — 命令行工具，无需图形界面
- 更新历史记录 — 保持简单，不存储历史
- 开机自启动 — 用户手动启动
- 跨平台支持 — 仅支持 Windows

## Context

**nanobot**: 一个 AI Agent 工具，托管在 GitHub (https://github.com/HKUDS/nanobot)，支持通过 uv 包管理器安装。

**uv**: Python 包管理器，用于安装和管理 Python 工具。

**Pushover**: 推送通知服务，用于在更新失败时通知用户。

## Constraints

- **平台**: Windows 操作系统 — 目标用户在 Windows 环境使用
- **语言**: Golang — 用户指定
- **日志库**: github.com/WQGroup/logger — 用户指定，基于 logrus
- **日志格式**: `2024-01-01 12:00:00.123 - [INFO]: message` — 需要自定义 formatter
- **配置格式**: YAML — 可读性好，支持注释

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| 隐藏窗口运行 | 后台服务，无需用户交互 | — Pending |
| YAML 配置文件 | 可读性好，支持注释，Go 生态支持完善 | — Pending |
| Cron 定时触发 | 灵活的时间配置，用户熟悉 | — Pending |
| Pushover 失败通知 | 简单可靠的通知方式 | — Pending |
| GitHub main 分支优先 | 获取最新功能和修复 | — Pending |
| 稳定版回退机制 | 保证更新失败时仍可用 | — Pending |

## Configuration

**配置文件**: `./config.yaml`

**默认配置项**:
- cron: "0 3 * * *" (每天凌晨 3 点)
- pushover_token: "" (Pushover App Token)
- pushover_user: "" (Pushover User Key)

**命令行参数**:
- `-cron`: 覆盖配置文件中的 cron 表达式
- `-config`: 指定配置文件路径
- `-run-once`: 立即执行一次更新后退出
- `-version`: 显示版本信息
- `help`: 显示帮助信息

---
*Last updated: 2025-02-18 after initialization*

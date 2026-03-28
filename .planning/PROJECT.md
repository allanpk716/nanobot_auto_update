# Nanobot Auto Updater

## What This Is

一个 Windows 后台监控服务，使用 Golang 开发，用于监控网络连通性并通过 HTTP API 触发 nanobot 工具的更新。**v0.3 重大架构变更**：从定时更新工具转变为监控服务 + HTTP API 触发更新模式。**v0.4 实时日志查看**：通过 SSE 流式传输和嵌入式 Web UI 实时查看 nanobot 实例的 stdout/stderr 输出。**v0.5 核心监控和自动化**：启动时自动启动实例、实例健康监控、Google 连通性监控、HTTP API 触发更新。多实例管理保持不变。通过配置文件和 HTTP API 控制行为。

## Core Value

自动保持 nanobot 处于最新版本，无需用户手动干预。

## Requirements

### Validated

### Active

- LOG-01: 更新日志数据结构 — Phase 30 ✓
- LOG-02: 更新触发时记录日志 — Phase 30 ✓
- LOG-03: 非阻塞日志记录 — Phase 30 ✓
- LOG-04: 更新 ID 返回给客户端 — Phase 30 ✓
- STORE-01: JSONL 持久化 — Phase 31 ✓
- STORE-02: 7天自动清理 — Phase 31 ✓
- QUERY-01: 查询 API — Phase 32
- QUERY-02: 分页参数 — Phase 32
- QUERY-03: 认证保护 — Phase 32

### Out of Scope

**v0.4 实时日志查看** — 2026-03-20:
- ✓ 环形缓冲区（5000行容量）和并发读写 — v0.4
- ✓ Stdout/stderr 捕获和并发管道读取 — v0.4
- ✓ 实例生命周期集成（独立缓冲） — v0.4
- ✓ SSE 流式传输 API 和历史日志 — v0.4
- ✓ 嵌入式 Web UI 和实例选择器 — v0.4
- ✓ 优雅降级错误处理 — v0.4

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

### Validated

**v0.6 Update Log Recording** — 2026-03-28:
- ✓ UpdateLog 数据模型 (UUID, 时间戳, 触发方式, 实例详情) — Phase 30
- ✓ UpdateLogger 组件 (线程安全 Record/GetAll) — Phase 30
- ✓ TriggerHandler 集成日志记录 (UUID v4, 非阻塞) — Phase 30
- ✓ JSONL 文件持久化 (atomic write + fsync) — Phase 31
- ✓ 7天自动清理 (bufio.Scanner + atomic rename) — Phase 31
- ✓ UpdateLogger 生命周期集成 (main.go + cron) — Phase 31

**v0.5 核心监控和自动化** — 2026-03-24:
- ✓ 启动时自动启动所有配置的实例 — Phase 24
- ✓ 实例健康监控（检测实例是否正常运行） — Phase 25
- ✓ Google 连通性监控服务（定时测试 google.com） — Phase 26
- ✓ 网络连通性变化通知（Pushover + 1分钟冷却） — Phase 27
- ✓ HTTP API 触发更新端点 (/api/v1/trigger-update) — Phase 28
- ✓ HTTP help 接口 (/api/v1/help) — Phase 29

- GUI 界面 — 命令行工具，无需图形界面
- 更新历史记录 — 保持简单，不存储历史
- 开机自启动 — 用户手动启动
- 跨平台支持 — 仅支持 Windows
- 日志文本搜索和正则表达式过滤 — v2 功能
- 日志导出功能 — v2 功能
- 暗色主题 UI — v2 功能
- 可配置缓冲区大小 — v2 功能

## Context

**nanobot**: 一个 AI Agent 工具，托管在 GitHub (https://github.com/HKUDS/nanobot)，支持通过 uv 包管理器安装。

**uv**: Python 包管理器，用于安装和管理 Python 工具。

**Pushover**: 推送通知服务，用于在更新失败时通知用户。

**v0.5 Shipped:** 2026-03-24 — 核心监控和自动化里程碑完成，6 个阶段 (24-29)，16 个计划，22 个任务，~2,400 行新增代码。Phase 24: 启动时自动启动实例。Phase 25: 实例健康监控。Phase 26: Google 连通性监控。Phase 27: Pushover 通知（1 分钟冷却）。Phase 28: HTTP API 触发更新 (Bearer Token + 并发控制)。Phase 29: HTTP help 接口。审计结果: 20/20 需求满足，6/6 阶段通过，15/15 集成点连接，5/5 E2E 流程完整，2 技术债（非阻塞文档缺口）。

**v0.4 Shipped:** 2026-03-20 — 实时日志查看里程碑完成，5 个阶段，11 个计划，33 个需求，~12,000 行代码增加。测试覆盖: 单元测试 + 集成测试 + E2E 测试。审计结果: 33/33 需求满足，5/5 阶段通过，17/17 集成点连接，4/4 E2E 流程完整，0 缺口，0 技术债。

**v0.2 Shipped:** 2026-03-16 — 多实例支持里程碑完成，5 个阶段，7 个计划，8 个任务，~5000 LOC Go 代码。测试覆盖: 单元测试 + 集成测试 + E2E 测试。

**Tech Stack:** Golang, viper (YAML 配置), logrus (日志), cron (调度), Pushover API (通知), SSE (Server-Sent Events), embed.FS (静态资源嵌入)

**Key Patterns:**
- 上下文感知日志 (logger.With 预注入)
- 结构化错误包装 (InstanceError + 错误链)
- 优雅降级 (失败不中断整体流程)
- TDD 开发模式 (RED-GREEN-REFACTOR)
- 配置向后兼容 (legacy 模式自动检测)
- 环形缓冲区 (固定容量 5000 行)
- 非阻塞订阅者模式 (慢订阅者丢弃日志)

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
| 自实现环形缓冲区 ([5000]LogEntry) | 避免外部依赖和序列化开销 | ✓ Good — Phase 19 |
| sync.RWMutex 线程安全 | 读多写少场景,允许并发读取 | ✓ Good — Phase 19 |
| Channel 订阅模式 (容量 100) | 匹配 Go 并发习惯,集成 Phase 22 SSE | ✓ Good — Phase 19 |
| 非阻塞订阅者发送 (select+default) | 防止 Write 阻塞,确保系统稳定性 | ✓ Good — Phase 19 |
| bufio.Scanner 行读取 | 自动处理行边界,API 更简洁 | ✓ Good — Phase 20 |
| os.Pipe() 替代 cmd.StdoutPipe() | 避免 race condition | ✓ Good — Phase 20 |
| Context 取消 + select 非阻塞扫描 | 及时退出 goroutine | ✓ Good — Phase 20 |
| Clear() 清空数组 + 重置指针 | 线程安全状态重置 | ✓ Good — Phase 21 |
| 启动前 Clear, 停止后保留 | 调试友好,重启清空历史 | ✓ Good — Phase 21 |
| WriteTimeout=0 支持 SSE 长连接 | SSE 协议要求 | ✓ Good — Phase 22 |
| Graceful shutdown (10秒超时) | 优雅关闭服务器 | ✓ Good — Phase 22 |
| embed.FS 嵌入静态文件 | 单文件部署 | ✓ Good — Phase 23 |
| 原生 HTML/CSS/JS (无框架) | 简单日志查看器约 300 行 | ✓ Good — Phase 23 |
| 智能自动滚动 (50px 容差) | 检测手动滚动 | ✓ Good — Phase 23 |
| 高对比度 stderr 颜色 (#dc2626) | 确保可见性 | ✓ Good — Phase 23 |
| 实例名称按配置顺序返回 | 可预测的选择器顺序 | ✓ Good — Phase 23-02 |
| ERROR 级别记录管道读取错误 | 意外错误,服务继续 | ✓ Good — Phase 23-03 |
| WARN 级别记录 SSE 连接错误 | 预期错误,服务继续 | ✓ Good — Phase 23-03 |
| WARN 级别丢弃慢订阅者日志 | 非阻塞,不卡主流程 | ✓ Good — Phase 23-03 |
| 异步启动实例 (goroutine + panic recovery) | 非阻塞启动,应用稳定 | ✓ Good — Phase 24-03 |
| Context 5分钟超时控制 (auto-start) | 防止无限等待 | ✓ Good — Phase 24-03 |
| Nil-safe 组件管理 (healthMonitor 检查) | 零实例配置兼容 | ✓ Good — Phase 25-02 |
| 反向关闭顺序 (后启动先停止) | 确保依赖关系正确 | ✓ Good — Phase 25-02 |
| 网络监控独立启动 (不依赖实例数) | 监控 Google 无需实例 | ✓ Good — Phase 26-02 |
| 状态变化冷却确认 (1分钟 timer) | 避免频繁通知 | ✓ Good — Phase 27-01 |
| Pushover 异步发送 (goroutine) | 不阻塞监控主循环 | ✓ Good — Phase 27-01 |
| Bearer Token 认证 (RFC 6750) | 标准化认证机制 | ✓ Good — Phase 28-01 |
| Constant time string comparison | 防止时序攻击 | ✓ Good — Phase 28-01 |
| Atomic.Bool 并发更新控制 | 防止重复更新 | ✓ Good — Phase 28-02 |
| JSON 错误响应 (RFC 7807) | 标准化错误格式 | ✓ Good — Phase 28-03 |
| APIInstanceError 序列化适配 | JSON 兼容错误结构 | ✓ Good — Phase 28-03 |
| HTTP 状态码策略 (200/401/409/504) | 清晰的错误分类 | ✓ Good — Phase 28-03 |
| Help 接口无认证 (公开访问) | 第三方程序智能查询 | ✓ Good — Phase 29-02 |
| Version 注入 (main → server → handler) | 未来扩展性 | ✓ Good — Phase 29-02 |

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

## Current Milestone: v0.6 Update Log Recording and Query System

**Goal:** 记录每次 HTTP API 触发的更新操作,并提供查询接口获取更新历史日志

**Target features:**
- 更新日志记录:每次 trigger-update 调用时自动记录详细日志
- 日志持久化:使用 JSON Lines 格式保存到文件(保留7天)
- 日志查询 API:通过 /api/v1/update-logs 查询最近N次更新
- 分页支持:支持 limit/offset 参数
- 认证保护:与 trigger-update 使用相同的 Bearer Token 认证

**Log content:**
- 更新ID(唯一标识符)
- 时间戳(开始/结束时间)
- 触发来源
- 实例更新结果(每个实例的成功/失败状态和详细消息)
- 完整输出日志(每个实例的 stdout/stderr)

**Key context:**
- 文件格式: JSON Lines (每行一个JSON对象,便于追加)
- 存储位置: 文件持久化
- 清理策略: 保留最近7天,自动删除旧日志
- 认证方式: Bearer Token (与现有 API 一致)

**Last Shipped: v0.5 Core Monitoring and Automation** — Completed 2026-03-24

---

*Last updated: 2026-03-28 Phase 31 complete*

# Nanobot Auto Updater

## What This Is

一个 Windows 后台监控服务，使用 Golang 开发，用于监控网络连通性并通过 HTTP API 触发 nanobot 工具的更新。**v0.11 Windows 服务自启动**：支持通过 config.yaml `auto_start: true` 开启 Windows 服务模式，系统启动即运行，无需用户登录桌面。包括 svc.Handler 服务生命周期管理、ServiceManager (SCM 注册/卸载/恢复策略)、双模式兼容适配（服务模式跳过守护进程、SCM 重启策略）、配置热重载（viper.WatchConfig + 500ms debounce + 6 组件重建回调）。**v0.10 管理界面自更新功能**：嵌入式 Web UI 新增自更新管理区域（版本显示、检测更新、一键更新、下载进度百分比），后端进度追踪（atomic.Value + io.TeeReader）和 Web Token API。**v0.9 启动通知与 Telegram 监控**：实例启动结果聚合通知、Telegram 连接状态监控（日志模式检测 + 30s 超时状态机）和 Pushover 通知。**v0.8 自更新**：通过 GitHub Releases 自动检测、下载并替换更新 nanobot-auto-updater 自身，包括 CI/CD 自动构建、HTTP API 触发自更新、安全恢复机制。**v0.7 更新生命周期通知**：HTTP API 触发更新时发送 Pushover 通知，包括更新开始/完成通知、非阻塞发送、优雅降级。**v0.6 更新日志记录和查询**：持久化记录每次更新操作的详细日志，提供分页查询 API 和 7 天自动清理。**v0.5 核心监控和自动化**：启动时自动启动实例、实例健康监控、Google 连通性监控、HTTP API 触发更新和 help 接口。**v0.4 实时日志查看**：通过 SSE 流式传输和嵌入式 Web UI 实时查看 nanobot 实例的 stdout/stderr 输出。多实例管理保持不变。通过配置文件和 HTTP API 控制行为。

## Core Value

自动保持 nanobot 处于最新版本，无需用户手动干预。

## Requirements

### Validated

**v0.8 Safety & Recovery** — 2026-03-30 (Phase 40):
- ✓ SAFE-01: 自更新后 self-spawn 重启 (daemon.go 标志) + 端口重试 (500ms × 5)
- ✓ SAFE-02: Pushover 通知 (开始/成功/失败) + Notifier 注入
- ✓ SAFE-03: .update-success 标记 + 启动时 .old 清理
- ✓ SAFE-04: .exe.old 异常检测 + 自动恢复旧版本

**v0.8 Self-Update HTTP API** — 2026-03-30 (Phase 39):
- ✓ API-01: POST /api/v1/self-update Bearer Token 认证 (401 Unauthorized)
- ✓ API-02: 自更新与 trigger-update 互斥锁 (409 Conflict)
- ✓ API-03: GET /api/v1/self-update/check 版本检查 + 状态查询
- ✓ API-04: Help 接口包含 self_update_check 和 self_update 端点说明

**v0.8 Self-Update Core** — 2026-03-30 (Phase 38):
- ✓ UPDATE-01: CheckLatest() 获取 GitHub 最新 Release
- ✓ UPDATE-02: NeedUpdate() semver 版本比较 (dev 版本始终更新)
- ✓ UPDATE-03: SHA256 校验下载 ZIP 完整性
- ✓ UPDATE-04: minio/selfupdate Apply 运行中 exe 替换
- ✓ UPDATE-05: .old 备份 + RollbackError 检查
- ✓ UPDATE-06: 1小时 Release 缓存
- ✓ UPDATE-07: self_update 配置段 (默认 HQGroup/nanobot-auto-updater)

**v0.8 CI/CD Pipeline** — 2026-03-29 (Phase 37):
- ✓ CICD-01: v* tag 触发 GitHub Actions 自动构建
- ✓ CICD-02: GoReleaser 编译 Windows amd64 + checksums 发布到 GitHub Releases
- ✓ CICD-03: ldflags 注入版本号 (-version 输出正确)

**v0.8 PoC Validation** — 2026-03-29 (Phase 36):
- ✓ VALID-01: minio/selfupdate v0.6.0 成功替换运行中 Windows exe
- ✓ VALID-02: 旧版本保存为 .old 备份文件
- ✓ VALID-03: 自更新后独立重启 (self-spawn via cmd.Start)

**v0.7 Update Lifecycle Notifications** — 2026-03-29:
- ✓ UNOTIF-01: 更新开始通知 (触发来源 + 实例数量)
- ✓ UNOTIF-02: 更新完成通知 (三态状态 + 耗时 + 实例详情)
- ✓ UNOTIF-03: 非阻塞通知 (goroutine + panic recovery)
- ✓ UNOTIF-04: 优雅降级 (Pushover 未配置时跳过)

**v0.6 Update Log Recording and Query System** — 2026-03-29:
- ✓ LOG-01: UpdateLog 数据模型 (UUID, 时间戳, 触发方式, 实例详情)
- ✓ LOG-02: 更新触发时记录日志 (TriggerHandler 集成)
- ✓ LOG-03: 非阻塞日志记录 (文件写入失败不影响更新)
- ✓ LOG-04: 更新 ID (UUID v4) 返回给客户端
- ✓ STORE-01: JSONL 文件持久化 (atomic write + fsync)
- ✓ STORE-02: 7天自动清理 (bufio.Scanner + atomic rename)
- ✓ QUERY-01: 查询 API (GET /api/v1/update-logs)
- ✓ QUERY-02: 分页参数 (limit/offset, 默认20/最大100)
- ✓ QUERY-03: 认证保护 (Bearer Token, 复用 Phase 28)

**v0.5 核心监控和自动化** — 2026-03-24:
- ✓ 启动时自动启动所有配置的实例 — Phase 24
- ✓ 实例健康监控（检测实例是否正常运行） — Phase 25
- ✓ Google 连通性监控服务（定时测试 google.com） — Phase 26
- ✓ 网络连通性变化通知（Pushover + 1分钟冷却） — Phase 27
- ✓ HTTP API 触发更新端点 (/api/v1/trigger-update) — Phase 28
- ✓ HTTP help 接口 (/api/v1/help) — Phase 29

**v0.4 实时日志查看** — 2026-03-20:

**v0.9 Startup Notification** — 2026-04-06 (Phase 41):
- ✓ STRT-01: 实例启动后通过 Pushover 通知启动结果（成功或失败）
- ✓ STRT-02: 通知在 auto-start goroutine 内执行（非阻塞）
- ✓ STRT-03: 未配置 Pushover 时优雅降级（返回 nil）

**v0.9 Telegram Monitor Core** — 2026-04-06 (Phase 42):
- ✓ TELE-01: "Starting Telegram bot" 触发监控窗口
- ✓ TELE-02: "Telegram bot commands registered" 检测连接成功
- ✓ TELE-03: "httpx.ConnectError" 检测连接失败
- ✓ TELE-04: 30 秒超时通知
- ✓ TELE-05: 成功 Pushover 通知
- ✓ TELE-06: 失败 Pushover 通知
- ✓ TELE-08: 历史日志条目过滤 (startTime 前忽略)

**v0.9 Telegram Monitor Integration** — 2026-04-06 (Phase 43):
- ✓ TELE-07: 未产生 trigger 日志的实例无监控开销 (TestMonitor_NoTriggerNoNotifications)
- ✓ TELE-09: 实例停止时取消监控,不发送虚假通知 (TestMonitor_StopCancelsMonitor)

**v0.12 Instance Config CRUD API** — 2026-04-11 (Phase 50):
- ✓ IC-01: POST 创建实例 (name, port, start_command, startup_timeout, auto_start) 自动持久化到 config.yaml
- ✓ IC-02: PUT 更新实例配置，500ms 内反映在 config.yaml
- ✓ IC-03: DELETE 删除实例 (先停止运行中实例再删除)
- ✓ IC-04: POST 复制实例 (克隆 auto-updater + nanobot 配置)
- ✓ IC-05: 配置变更自动持久化并触发热重载 (复用 500ms debounce)
- ✓ IC-06: 配置验证 — 唯一名称、唯一端口、必填字段、端口范围 1-65535

**v0.12 Lifecycle Control API** — 2026-04-12 (Phase 51):
- ✓ LC-01: POST /api/v1/instances/{name}/start 启动已停止的实例 (409 已运行)
- ✓ LC-02: POST /api/v1/instances/{name}/stop 停止运行中的实例 (409 已停止)
- ✓ LC-03: 生命周期端点 Bearer Token 认证 (401 未授权/错误 token)

**v0.11 Windows 服务自启动** — 2026-04-11 (Phases 46-49):
- ✓ SVC-01: svc.IsWindowsService() 检测运行模式，自动选择服务/控制台模式
- ✓ SVC-02: svc.Handler Execute 方法处理服务启动/停止/关机请求
- ✓ SVC-03: 服务模式优雅关闭，响应 Stop/Shutdown 控制码
- ✓ MGR-01: config.yaml auto_start: true/false 配置项
- ✓ MGR-02: auto_start=true 时管理员权限自动注册 Windows 服务
- ✓ MGR-03: auto_start=false 时检测已注册服务自动卸载
- ✓ MGR-04: SCM 恢复策略 (3x restart, 60s interval)
- ✓ ADPT-01: 服务模式跳过守护进程模式
- ✓ ADPT-02: restartFn 服务模式使用 SCM 重启
- ✓ ADPT-03: 服务模式工作目录自动设置 (exe 所在目录)
- ✓ ADPT-04: 配置文件变更自动重载 (500ms debounce)

**v0.10 管理界面自更新功能** — 2026-04-08 (Phases 44-45):
- ✓ UI-01: 自更新管理区域布局 (home.html header 与 main 之间)
- ✓ UI-02: 当前版本显示 (蓝色标签样式)
- ✓ UI-03: 检测更新 (版本号+发布日期+release notes 截断展开)
- ✓ UI-04: 触发更新与进度显示 (checking→downloading%→installing→complete/failed)
- ✓ UI-05: 下载进度百分比 (500ms 轮询 + 进度条 + 百分比文字)
- ✓ API-01: 更新进度状态追踪 (ProgressState + io.TeeReader + atomic.Value)
- ✓ API-02: Web UI Token API (localhost-only GET /api/v1/web-config)

**v0.12 Nanobot Config Management API** — 2026-04-12 (Phase 52):
- ✓ NC-01: 创建实例时自动创建 nanobot 配置目录和默认 config.json
- ✓ NC-02: GET 读取任意实例的 nanobot config.json
- ✓ NC-03: PUT 更新任意实例的 nanobot config.json
- ✓ NC-04: 复制实例时克隆 nanobot config.json 到新目录

**v0.12 Instance Management UI** — 2026-04-13 (Phase 53):
- ✓ UI-01: 卡片式实例列表 (名称/端口/命令/状态/操作按钮)
- ✓ UI-02: 创建实例对话框 (全配置字段 + nanobot 配置编辑)
- ✓ UI-03: 编辑实例对话框 (修改自动更新器配置)
- ✓ UI-04: 复制实例对话框 (克隆配置 + nanobot 配置)
- ✓ UI-05: 删除实例确认对话框 (运行中实例警告)
- ✓ UI-06: Nanobot 配置混合编辑器 (结构化表单 + JSON 文本)

## Current Milestone: v0.18.0 实例管理增强

**Goal:** 增强 Web UI 实例管理的安全性、易用性和配置编辑体验

**Target features:**
- 删除按钮状态保护（运行中禁用）
- 创建实例集成配置编辑
- 删除操作二次确认
- 自定义配置目录（自动创建/读取已有）
- JSON 配置编辑器增强（语法高亮 + 实时校验）

### Active

- **DEL-01**: 实例运行中时删除按钮禁用，仅停止状态可删除
- **DEL-02**: 删除操作需二次确认对话框
- **CFG-01**: 创建实例对话框集成 nanobot 配置编辑区域
- **CFG-02**: 创建实例时允许用户填写 config 保存目录
- **CFG-03**: 启动时自动创建不存在的配置目录，读取已有目录中的 config 文件
- **EDT-01**: JSON 配置编辑器支持语法高亮显示
- **EDT-02**: JSON 配置编辑器实时格式校验，语法错误即时提示用户

### Out of Scope

<!-- Explicit boundaries. Includes reasoning to prevent re-adding. -->

(None for this milestone)

## Context

**nanobot**: 一个 AI Agent 工具，托管在 GitHub (https://github.com/HKUDS/nanobot)，支持通过 uv 包管理器安装。

**uv**: Python 包管理器，用于安装和管理 Python 工具。

**Pushover**: 推送通知服务，用于在更新失败时通知用户。

**v0.12 Shipped:** 2026-04-13 — 实例管理与配置编辑里程碑完成，4 个阶段 (50-53)，9 个计划。Phase 50: CRUD API (UpdateConfig atomic + deep copy + viper ReadInConfig 修复)。Phase 51: Lifecycle API (HandleStart/HandleStop + TryLockUpdate + 12 测试)。Phase 52: Nanobot Config API (ConfigManager + callback injection + 38 测试)。Phase 53: 完整管理 UI (卡片列表 + CRUD 对话框 + 混合配置编辑器 + XSS 安全)。19/19 需求满足。

**v0.11 Windows 服务自启动 Shipped:** 2026-04-11 — Windows 服务自启动里程碑完成，4 个阶段 (46-49)，8 个计划，15 个任务。Phase 46: ServiceConfig + svc.IsWindowsService() 检测。Phase 47: svc.Handler 生命周期管理 + 优雅关闭。Phase 48: ServiceManager (SCM 注册/卸载/恢复策略)。Phase 49: 双模式适配 + 配置热重载 (500ms debounce + 6 组件回调)。11/11 需求满足。

**v0.10 管理界面自更新功能 Shipped:** 2026-04-08 — 管理界面自更新功能里程碑完成，2 个阶段 (44-45)，4 个计划。Phase 44: 下载进度追踪 (ProgressState + io.TeeReader + atomic.Value) + Web Token API (localhost-only)。Phase 45: 自更新管理 UI (版本标签 + 检测更新 + 触发更新 + 500ms 进度轮询 + 进度条)。7/7 需求满足。

**v0.9 Shipped:** 2026-04-06 — 启动通知与 Telegram 监控里程碑完成，3 个阶段 (41-43)，6 个计划。Phase 41: NotifyStartupResult 聚合通知 + auto-start 集成。Phase 42: TelegramMonitor 状态机 (模式检测 + AfterFunc 超时 + duck-typing 接口) + 8 并发压力测试。Phase 43: InstanceLifecycle 集成 (constructor chain + context 取消 + goroutine 生命周期)。12/12 需求满足。

**v0.7 Shipped:** 2026-03-29 — 更新生命周期通知里程碑完成，2 个阶段 (34-35)，2 个计划，4 个任务。Phase 34: Notifier 注入 TriggerHandler + 异步通知 (开始/完成) + panic recovery。Phase 35: Notifier 接口重构 + recordingNotifier mock + 4 个 E2E 通知测试。审计结果: 4/4 需求满足，2/2 阶段通过，5/5 集成点连接，5/5 E2E 流程完整，77/77 测试通过。

**v0.6 Shipped:** 2026-03-29 — 更新日志记录和查询系统里程碑完成，4 个阶段 (30-33)，8 个计划，16 个任务。Phase 30: UpdateLog 数据模型 + UpdateLogger 组件。Phase 31: JSONL 文件持久化 + 7天自动清理。Phase 32: 查询 API + 分页 + Bearer Token 认证。Phase 33: E2E 集成测试 + 性能基准 (867ns-87us)。验证: 5/5 成功标准通过，50+ 测试全部通过。

**v0.5 Shipped:** 2026-03-24 — 核心监控和自动化里程碑完成，6 个阶段 (24-29)，16 个计划，22 个任务，~2,400 行新增代码。Phase 24: 启动时自动启动实例。Phase 25: 实例健康监控。Phase 26: Google 连通性监控。Phase 27: Pushover 通知（1 分钟冷却）。Phase 28: HTTP API 触发更新 (Bearer Token + 并发控制)。Phase 29: HTTP help 接口。审计结果: 20/20 需求满足，6/6 阶段通过，15/15 集成点连接，5/5 E2E 流程完整，2 技术债（非阻塞文档缺口）。

**v0.4 Shipped:** 2026-03-20 — 实时日志查看里程碑完成，5 个阶段，11 个计划，33 个需求，~12,000 行代码增加。测试覆盖: 单元测试 + 集成测试 + E2E 测试。审计结果: 33/33 需求满足，5/5 阶段通过，17/17 集成点连接，4/4 E2E 流程完整，0 缺口，0 技术债。

**v0.2 Shipped:** 2026-03-16 — 多实例支持里程碑完成，5 个阶段，7 个计划，8 个任务，~5000 LOC Go 代码。测试覆盖: 单元测试 + 集成测试 + E2E 测试。

**Tech Stack:** Golang, viper (YAML 配置), logrus (日志), cron (调度), Pushover API (通知), SSE (Server-Sent Events), embed.FS (静态资源嵌入), minio/selfupdate (exe 替换), GoReleaser (CI/CD), golang.org/x/mod/semver (版本比较)

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
| JSON Lines 日志格式 | 简单追加,无需全文件解析 | ✓ Good — Phase 30 |
| 三态分类 (success/partial_success/failed) | 精确表达多实例更新结果 | ✓ Good — Phase 30 |
| TriggerUpdater 接口 | mock 友好测试,解耦具体依赖 | ✓ Good — Phase 30 |
| Nil-safe UpdateLogger | handler 检查 nil,非阻塞错误日志 | ✓ Good — Phase 30 |
| UUID v4 入口生成 | 完整生命周期追踪 | ✓ Good — Phase 30 |
| Separate fileMu mutex | 文件 I/O 不阻塞 GetAll() | ✓ Good — Phase 31 |
| Lazy file open | 首次 Record() 时打开,支持纯内存模式 | ✓ Good — Phase 31 |
| Atomic cleanup (temp + rename) | Windows 安全的文件清理 | ✓ Good — Phase 31 |
| Non-blocking file write failure | 静默降级到纯内存模式 | ✓ Good — Phase 31 |
| UpdateLogger in main.go | 应用级生命周期控制 | ✓ Good — Phase 31 |
| bufio.Scanner 流式分页 | 避免 1000+ 记录内存问题 | ✓ Good — Phase 32 |
| Notifier 注入 TriggerHandler | 更新生命周期通知,与 UpdateLogger 同模式 | ✓ Good — Phase 34 |
| 异步通知 goroutine + panic recovery | 通知失败不影响更新流程 | ✓ Good — Phase 34 |
| instanceCount 构造时注入 | 通知内容含实例数,无需运行时查询 | ✓ Good — Phase 34 |
| statusToTitle/formatCompletionMessage | 三态通知标题和实例详情格式化 | ✓ Good — Phase 34 |
| Notifier 接口 (trigger.go 本地定义) | 最小范围,单方法接口,duck typing | ✓ Good — Phase 35 |
| TryLockUpdate/UnlockUpdate 共享互斥锁 | 自更新与 trigger-update 复用 atomic.Bool | ✓ Good — Phase 39 |
| SelfUpdateHandler (Check + Update 端点) | HandleCheck 版本信息 + HandleUpdate 异步更新 | ✓ Good — Phase 39 |
| atomic.Value 状态追踪 (idle/updating/updated/failed) | 无锁并发读取更新状态 | ✓ Good — Phase 39 |
| 202 Accepted 异步更新 (goroutine + panic recovery) | 非阻塞触发 + 容错 | ✓ Good — Phase 39 |
| selfupdate.Updater 注入 NewServer (第8参数) | 零配置自更新集成 | ✓ Good — Phase 39 |
| Help 端点自更新条目 (self_update_check + self_update) | API 可发现性 | ✓ Good — Phase 39 |
| minio/selfupdate v0.6.0 选择 | 战斗验证的 Windows exe rename trick | ✓ Good — Phase 36 |
| PoC-first 验证策略 | 消除 Windows exe 替换技术不确定性 | ✓ Good — Phase 36 |
| GoReleaser ZIP 格式 | Phase 38 下载 ZIP 并提取 exe | ✓ Good — Phase 37 |
| golang.org/x/mod/semver | 标准库扩展，成熟的 semver 比较 | ✓ Good — Phase 38 |
| struct-based 缓存 (cachedRelease + cacheTime) | 简洁且可测试 (操纵 cacheTime 测试过期) | ✓ Good — Phase 38 |
| In-memory ZIP 提取 (bytes.Reader) | 无临时文件，避免 Windows 文件锁问题 | ✓ Good — Phase 38 |
| GoReleaser checksums.txt 两空格分隔 | 匹配 GoReleaser 标准输出格式 | ✓ Good — Phase 38 |
| SelfUpdateChecker/UpdateMutex 接口 (duck typing) | 最小范围，单方法接口，与 TriggerUpdater 同模式 | ✓ Good — Phase 39 |
| restartFn 注入 SelfUpdateHandler | 生产用 defaultRestartFn，测试覆写 no-op，避免子进程循环 | ✓ Good — Phase 40 |
| Sync 完成通知 (os.Exit 前) | 避免 goroutine 被 os.Exit 杀死 (Pitfall 1) | ✓ Good — Phase 40 |
| CheckUpdateState 内外函数分离 | checkUpdateStateInternal 返回决策字符串，CheckUpdateStateForPath 执行，可测试无 os.Exit | ✓ Good — Phase 40 |
| net.Listen + http.Serve 替代 ListenAndServe | 启用端口绑定重试能力 | ✓ Good — Phase 40 |
| NotifyStartupResult 方法 + formatStartupMessage | 聚合多实例启动结果为单条通知 | ✓ Good — Phase 41 |
| TelegramMonitor 独立包 (internal/telegram) | 日志模式检测 + 30s AfterFunc 超时状态机 | ✓ Good — Phase 42 |
| Duck-typed LogSubscriber/Notifier 接口 | 最小范围,单方法接口,避免循环导入 | ✓ Good — Phase 42 |
| Notifier constructor chain (main→manager→lifecycle) | 注入式通知,与现有模式一致 | ✓ Good — Phase 43 |
| TelegramMonitor lifecycle tied to InstanceLifecycle | 启动后创建,停止前取消,goroutine 安全 | ✓ Good — Phase 43 |
| atomic.Value ProgressState + io.TeeReader | 无锁并发读取进度 + 流式下载字节计数 | ✓ Good — Phase 44 |
| localhost-only web-config endpoint | Token 仅限本地获取，防止远程滥用 | ✓ Good — Phase 44 |
| textContent 渲染所有 API 数据 | 防止 GitHub release notes XSS | ✓ Good — Phase 45 |
| 500ms setInterval 轮询 + 60s 超时 | 实时进度反馈 + 防止无限轮询 | ✓ Good — Phase 45 |
| 按钮状态互锁 (更新中禁用所有按钮) | 防止重复触发更新 | ✓ Good — Phase 45 |
| golang.org/x/sys/windows/svc | Windows 标准库服务接口 | ✓ Good — Phase 46 |
| build-tag 双实现 (service_windows.go / service.go) | 跨平台编译兼容 | ✓ Good — Phase 46 |
| svc.IsWindowsService() 检测 | 自动模式选择，无需命令行参数 | ✓ Good — Phase 46 |
| AppComponents/AppStartup/AppShutdown 提取 | 服务模式与控制台模式共享启动/关闭逻辑 | ✓ Good — Phase 47 |
| onReady callback 模式 | 服务 Running 状态后执行初始化（热重载等） | ✓ Good — Phase 47 |
| ServiceManager 便利包装函数 | main.go 简洁调用 RegisterService/UnregisterService/IsAdmin | ✓ Good — Phase 48 |
| SetRecoveryActionsOnNonCrashFailures 非关键 | 日志警告但不阻断，SCM 基础恢复策略已足够 | ✓ Good — Phase 48 |
| IsServiceMode guard pattern | 服务模式下跳过守护进程逻辑 (MakeDaemon/MakeDaemonSimple) | ✓ Good — Phase 49 |
| 双重启策略 (service=os.Exit(1), console=self-spawn) | 服务模式触发 SCM recovery，控制台模式自重启 | ✓ Good — Phase 49 |
| 500ms debounce timer (time.AfterFunc) | Windows fsnotify 快速事件合并 | ✓ Good — Phase 49 |
| sync.Mutex 序列化组件重建 | 防止并发 config change 导致组件状态混乱 | ✓ Good — Phase 49 |
| 全量替换策略 (StopAll→recreate→StartAll) | 实例配置变更，避免复杂 diff | ✓ Good — Phase 49 |
| 动态 Bearer Token getter (func() string) | 热重载 Token 无需重启 API 服务器 | ✓ Good — Phase 49 |
| SelfUpdater 仅日志不重建 | 避免破坏 SelfUpdateHandler 的引用 | ✓ Good — Phase 49 |
| UpdateConfig atomic read-modify-write (updateMu + deepCopyConfig) | 防止并发 API 竞态条件，隔离深拷贝避免 backing array 污染 | ✓ Good — Phase 50 |
| ReadInConfig-before-WriteConfig pattern | 修复 viper 状态丢失 bug，确保非 v.Set() 键不丢失 | ✓ Good — Phase 50 |
| skipReload flag on hotReloadState | UpdateConfig 写入时抑制 WatchConfig 重载，防止状态损坏 | ✓ Good — Phase 50 |
| Injected config reader (getConfig closure) | Handler 可测试，无需 NewServer 签名变更 | ✓ Good — Phase 50 |
| validationError/notFoundError custom types | errors.As 路由，422 字段级错误详情 | ✓ Good — Phase 50 |
| context.Background() for lifecycle ops | 防止客户端断连导致孤立进程 (start 60s, stop 30s) | ✓ Good — Phase 51 |
| TryLockUpdate guard on lifecycle handlers | 生命周期操作与 TriggerUpdate/SelfUpdate 序列化 | ✓ Good — Phase 51 |
| 409 Conflict for wrong-state operations | 清晰的状态错误反馈 (已运行→start, 已停止→stop) | ✓ Good — Phase 51 |
| SetPIDForTest cross-package helper | api 包测试可注入运行状态，production-code 非 _test.go | ✓ Good — Phase 51 |
| NanobotConfigManager with ParseConfigPath | 从 start_command --config 提取路径，os.UserHomeDir() Windows 兼容 | ✓ Good — Phase 52 |
| Lazy-creation fallback on GET | 缺失配置文件自动创建默认 config.json | ✓ Good — Phase 52 |
| Callback injection (onCreate/onCopy/onDelete) | 非阻塞回调，失败仅 warn 不阻断主操作 | ✓ Good — Phase 52 |
| setter methods for optional callbacks | nil by default，现有测试无需修改 | ✓ Good — Phase 52 |
| Promise.allSettled dual-API fetch | 认证失败时优雅降级为 status-only 卡片 | ✓ Good — Phase 53 |
| Modal system (Escape/overlay/X close) | 统一对话框管理，DOM API 防 XSS | ✓ Good — Phase 53 |
| Toast prepend + 3s auto-dismiss | 最新通知置顶，CSS fade-out 动画 | ✓ Good — Phase 53 |
| Shared form builder (buildInstanceFormHtml) | Create/Edit/Copy 复用表单，避免代码重复 | ✓ Good — Phase 53 |
| syncGuard bidirectional sync | 防止 form↔JSON 无限循环，JSON 为保存数据源 | ✓ Good — Phase 53 |
| textContent for all user data | XSS 安全渲染，innerHTML 仅用于静态模板 | ✓ Good — Phase 53 |
| API key password field + show/hide toggle | 敏感信息默认隐藏 (T-53-11 缓解) | ✓ Good — Phase 53 |

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

**v0.8 自更新配置** (新):
```yaml
self_update:
  github_owner: "HQGroup"       # 默认值
  github_repo: "nanobot-auto-updater"  # 默认值
```

**v0.11 服务模式配置** (新):
```yaml
auto_start: false  # true: 注册为 Windows 服务, false: 控制台模式
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

*Last updated: 2026-04-13 after v0.18.0 milestone started*

## Current State

**Shipped:** v0.12 实例管理与配置编辑 (2026-04-13)
**Active:** v0.18.0 实例管理增强
**Total:** 12 milestones shipped, 53 phases, ~23,400 LOC Go

*Last updated: 2026-04-13 after v0.18.0 milestone started*

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd:transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

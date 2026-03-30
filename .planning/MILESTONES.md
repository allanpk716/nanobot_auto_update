# Milestones

## v0.8 Self-Update (Shipped: 2026-03-30)

**Phases completed:** 5 phases, 8 plans, 12 tasks

**Key accomplishments:**

- Validated minio/selfupdate v0.6.0 can replace a running Windows exe, save .old backup, and self-spawn new version in under 3 seconds
- SelfUpdateHandler with HandleCheck/HandleUpdate endpoints, shared mutex with trigger-update, async 202 Accepted pattern, and 8 unit tests
- Self-update endpoint descriptions added to Help API response with self_update_check (GET) and self_update (POST) entries plus verification test
- Notifier injection with start/complete/failure Pushover notifications, .update-success status file, and testable self-spawn restart in SelfUpdateHandler
- 1. [Rule 3 - Blocking] Pre-existing capture_test.go compilation error

---

## v0.7 Update Lifecycle Notifications (Shipped: 2026-03-29)

**Phases completed:** 2 phases, 2 plans, 4 tasks

**Key accomplishments:**

- Notifier refactored to interface with recordingNotifier mock, 4 E2E tests validating full notification lifecycle (start, completion, non-blocking, graceful degradation)

---

## v0.6 Update Log Recording and Query System (Shipped: 2026-03-29)

**Phases completed:** 4 phases, 8 plans, 16 tasks

**Key accomplishments:**

- UpdateLog/InstanceUpdateDetail data structures with three-state status classification and thread-safe UpdateLogger using sync.RWMutex
- UUID v4 generation, timing metadata, and UpdateLogger integration in TriggerHandler with mock-based test coverage
- JSONL file persistence with sync.Mutex-protected atomic append, 7-day auto-cleanup via temp file + atomic rename, and non-blocking GetAll() using separate locks
- UpdateLogger wired into main.go with startup cleanup, daily cron at 3 AM, and graceful Close() in shutdown; NewServer() accepts external injection
- E2E integration tests covering trigger->JSONL persistence->query retrieval, update ID consistency, non-blocking file failure, and startup recovery
- 4 Go benchmarks confirming sub-millisecond query performance (867ns-87us), all 50+ tests passing, and all 5 Phase 33 success criteria verified

---

## v0.5 Core Monitoring and Automation (Shipped: 2026-03-24)

**Phases completed:** 6 phases (24-29), 16 plans, 22 tasks

**Key accomplishments:**

- ✅ Auto-Start: 应用启动时异步启动所有实例，带 panic 恢复和 5 分钟超时控制
- ✅ Health Monitoring: 定期检查实例运行状态，记录状态变化（运行→停止 ERROR 日志，停止→运行 INFO 日志）
- ✅ Network Monitoring: 定期测试 Google 连通性，记录请求成功/失败状态，可配置监控间隔和超时
- ✅ Pushover Notifications: 网络连通性状态变化时发送通知，带 1 分钟冷却确认机制避免频繁通知
- ✅ HTTP API Trigger: 通过 Bearer Token 认证的 POST /api/v1/trigger-update 端点触发更新，带并发控制和超时处理
- ✅ HTTP Help Endpoint: 提供 GET /api/v1/help 接口供第三方程序智能查询程序使用说明，避免 CLI 命令冲突

**Tech additions:**

- Context-based timeout control
- Atomic.Bool for concurrent update control
- Bearer Token authentication (RFC 6750)
- Server-Sent Events (SSE) for health state monitoring
- 1-minute cooldown timer for notification deduplication

---

## v0.4 实时日志查看 (Shipped: 2026-03-20)

**Phases completed:** 10 phases, 24 plans, 5 tasks

**Key accomplishments:**

- 实现线程安全的环形缓冲区（5000行容量），支持并发读写和 FIFO 自动覆盖
- 捕获 nanobot 进程的 stdout/stderr 输出，使用 os.Pipe() 和并发 goroutine 避免死锁
- 将 LogBuffer 集成到实例生命周期管理，每个实例独立缓冲，支持清空和保留策略
- 实现 SSE 流式传输 API，实时推送日志，支持历史日志回放和 30 秒心跳
- 提供嵌入式 Web UI，单文件部署，实例选择器，自动滚动控制和优雅降级错误处理

**Tech additions:**

- Server-Sent Events (SSE) for real-time streaming
- Go embed.FS for static file serving
- Ring buffer with subscriber pattern
- Non-blocking error handling with graceful degradation

---

## v0.2 多实例支持 (Shipped: 2026-03-16)

**Phases completed:** 10 phases, 18 plans, 8 tasks

**Key accomplishments:**

- (none recorded)

---

## v1.0 - Single Instance Auto-Update

**Completed:** 2026-02-18

**Features shipped:**

- 基础日志系统配置
- YAML 配置文件加载
- 单个 nanobot 实例的停止和启动
- UV 包管理器检测
- GitHub main 分支更新（带回退到 PyPI 稳定版）
- Pushover 失败通知
- Cron 定时调度
- Makefile 和 build.ps1 构建脚本

**Phases completed:** 4 phases (Phase 01-04)

- Phase 01: 基础配置和日志
- Phase 01.1: Nanobot 生命周期管理
- Phase 02: UV 检测和更新逻辑
- Phase 03: 调度和通知
- Phase 04: 运行时集成

**Lessons learned:**

- Windows 特定实现需要 `go:build windows` 约束
- 使用 `taskkill` 命令终止 Windows 进程
- 使用 `CREATE_NO_WINDOW` 标志隐藏控制台窗口
- Cron 调度需要 `SkipIfStillRunning` 防止重叠执行

---

## v0.2 - Multi-Instance Support

**Started:** 2026-03-09

**Goal:** 支持同时管理多个 nanobot 实例的升级和启动

**Status:** In Progress - Defining requirements

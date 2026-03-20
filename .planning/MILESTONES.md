# Milestones

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

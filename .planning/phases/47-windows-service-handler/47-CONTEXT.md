# Phase 47: Windows Service Handler - Context

**Gathered:** 2026-04-10
**Status:** Ready for planning

<domain>
## Phase Boundary

实现 `svc.Handler` 接口的 `Execute` 方法，使 Windows SCM 能通过标准服务接口启动和停止程序。处理服务生命周期（Start/Stop/Shutdown 控制码），30 秒内完成资源清理。服务模式下 SCM 状态正确显示 Running/Stopped。

不包括：服务注册/卸载（Phase 48）、守护进程/重启/工作目录适配（Phase 49）。

</domain>

<decisions>
## Implementation Decisions

### Handler 接口设计
- **D-01:** ServiceHandler 结构体放在 `internal/lifecycle/service_windows.go`，与现有 `IsServiceMode()` 同包，生命周期相关代码内聚
- **D-02:** 使用标准状态机：StartPending → Running → StopPending → Stopped，通过 `s chan<- svc.Status` 向 SCM 报告状态
- **D-03:** Execute 方法接收 `svc.ChangeRequest`，处理 `svc.Interrogate`、`svc.Stop`、`svc.Shutdown` 控制码
- **D-04:** Stop 和 Shutdown 执行相同的优雅关闭流程（均为 30 秒超时）

### 关闭流程编排
- **D-05:** 抽取共用关闭函数 `AppShutdown(ctx context.Context, components)` 从 main.go，服务模式和控制台模式复用同一关闭逻辑
- **D-06:** 统一超时策略：调用方传入 context 超时（控制台模式 10 秒，服务模式 30 秒）
- **D-07:** 关闭顺序与现有 main.go 一致：通知管理器 → 网络监控 → 健康监控 → cron → UpdateLogger → API 服务器
- **D-08:** 需要一个组件容器结构体（如 `AppComponents`）持有所有需要关闭的组件引用，传递给 AppShutdown

### 服务启动入口
- **D-09:** main.go 分支调用：服务模式下初始化日志、配置后，构造 ServiceHandler 并调用 `svc.Run(serviceName, handler)`
- **D-10:** Execute 方法内部启动所有业务组件（实例管理、HTTP API、定时任务等），与控制台模式相同的启动逻辑
- **D-11:** 控制台模式走现有 main.go 流程不变（signal.Notify → AppShutdown）
- **D-12:** 服务模式下 Execute 返回后程序退出，不需要额外 os.Exit

### Claude's Discretion
- ServiceHandler 结构体的具体字段设计
- AppComponents 容器结构体的字段组织方式
- Execute 内部 goroutine 的启动模式
- 非服务控制码（如 ParamChange、SessionChange）的忽略方式
- 测试策略（单元测试模拟 ChangeRequest channel）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 服务模式检测（Phase 46 产出）
- `internal/lifecycle/servicedetect_windows.go` — IsServiceMode() 实现，svc.IsWindowsService() 封装
- `internal/lifecycle/servicedetect.go` — 非 Windows 平台的 stub 实现

### 配置系统
- `internal/config/service.go` — ServiceConfig 结构体（service_name, display_name, auto_start）
- `internal/config/config.go` — Config 根结构体、Load() 函数

### 入口点
- `cmd/nanobot-auto-updater/main.go` — 程序入口，现有关闭逻辑在第 277-319 行

### 现有关闭逻辑参考
- `internal/api/server.go` — Server.Shutdown(ctx) 方法
- `internal/health/monitor.go` — HealthMonitor.Stop() 方法
- `internal/network/monitor.go` — NetworkMonitor.Stop() 方法
- `internal/notification/manager.go` — NotificationManager.Stop() 方法
- `internal/updatelog/logger.go` — UpdateLogger.Close() 方法

### Go 标准库
- `golang.org/x/sys/windows/svc` — Handler 接口、ChangeRequest、Status 类型（已在 go.mod 中）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/lifecycle/servicedetect_windows.go` — 已有 svc 包导入和 IsServiceMode()，可直接扩展
- `golang.org/x/sys v0.41.0` — windows/svc 子包已可用，无需新增依赖
- main.go 第 277-319 行 — 现有关闭逻辑可直接抽取为 AppShutdown 函数

### Established Patterns
- build tags: `//go:build windows` 用于 Windows 特定代码，配套 `//go:build !windows` stub
- nil-safe 组件管理：关闭前检查组件是否为 nil（如 healthMonitor、apiServer）
- 非阻塞关闭：组件 Stop() 方法不应阻塞，使用 context 超时控制
- goroutine + panic recovery：异步启动使用 defer recover

### Integration Points
- `cmd/nanobot-auto-updater/main.go:70-75` — 服务模式检测后的分支点，需改为调用 svc.Run()
- main.go 所有组件初始化（第 148-273 行）— 需抽取为可复用的启动函数或移入 Execute
- main.go 关闭逻辑（第 277-319 行）— 需抽取为 AppShutdown 函数

</code_context>

<specifics>
## Specific Ideas

- 控制台模式行为完全不变，服务模式是新代码路径
- AppComponents 容器避免 main.go 长参数列表，提高可测试性
- Execute 内部的业务组件启动逻辑应与控制台模式一致（复用相同的初始化代码）

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---
*Phase: 47-windows-service-handler*
*Context gathered: 2026-04-10*

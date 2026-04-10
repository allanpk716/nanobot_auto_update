# Phase 48: Service Manager - Context

**Gathered:** 2026-04-11
**Status:** Ready for planning

<domain>
## Phase Boundary

用户设置 `auto_start: true` 后，程序以管理员权限运行时自动完成 Windows 服务注册（SCM CreateService）和恢复策略配置，然后以退出码 2 退出。`auto_start: false` 时，检测到已注册服务则自动停止并卸载（DeleteService），然后继续以控制台模式运行。非 Windows 平台编译时 auto_start 配置被忽略。

不包括：svc.Handler 接口实现（Phase 47 已完成）、守护进程/重启/工作目录适配（Phase 49）。

</domain>

<decisions>
## Implementation Decisions

### 服务注册细节
- **D-01:** 服务账户使用 LocalSystem（权限最高，无需密码，适合需要访问文件系统和网络的场景）
- **D-02:** 启动类型为自动启动（Windows 启动时自动启动，无需用户登录）— 匹配 ROADMAP "系统启动即运行" 核心诉求
- **D-03:** 服务描述硬编码中文 "自动保持 nanobot 处于最新版本"，在 Windows 服务管理器中可见
- **D-04:** 服务已注册时跳过注册，输出日志提示服务已存在（幂等操作，安全简单）

### 服务卸载策略
- **D-05:** 卸载流程：先调用 `Control(svc.Stop)` 停止服务，等待停止完成后再 `DeleteService`
- **D-06:** 卸载后程序继续以控制台模式运行（不退出），适合"关掉服务但保留手动运行"场景

### SCM 恢复策略
- **D-07:** 无限重启策略：第一次/第二次/后续失败均重启服务，间隔 60 秒，24 小时重置失败计数器。简单有效，确保后台监控服务持续运行

### 权限与跨平台
- **D-08:** 管理员权限检测使用 `OpenProcessToken` + `TokenElevation`，非管理员时输出错误提示并以退出码 1 退出
- **D-09:** 跨平台遵循已有 build tag 模式（`//go:build windows` + `//go:build !windows` stub），非 Windows 平台 ServiceManager 操作为 no-op

### Claude's Discretion
- ServiceManager 文件组织（放在 `internal/lifecycle/` 与现有服务代码内聚）
- 错误处理细节（SCM API 错误包装、日志格式）
- 测试策略（SCM 操作的 mock 设计）
- 具体的 Go API 调用方式和参数
- 恢复策略的实现方式（svc/mgr API vs 调用 sc.exe）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 配置系统（Phase 46 产出）
- `internal/config/service.go` — ServiceConfig 结构体（service_name, display_name, auto_start）+ Validate()
- `internal/config/config.go` — Config 根结构体、Load() 函数

### 服务模式入口（Phase 47 产出）
- `cmd/nanobot-auto-updater/main.go` — 第 116-123 行（auto_start: false 占位）、第 125-136 行（auto_start: true 占位）
- `internal/lifecycle/service_windows.go` — ServiceHandler.Execute 已完成，本阶段不修改
- `internal/lifecycle/servicedetect_windows.go` — IsServiceMode() 实现

### 现有代码模式
- `internal/lifecycle/app.go` — AppComponents/AppStartup/AppShutdown 模式
- `internal/lifecycle/daemon.go` — 现有守护进程代码（build tag: windows）

### Go 标准库
- `golang.org/x/sys/windows/svc/mgr` — 服务管理 API（Connect, CreateService, OpenService, DeleteService）
- `golang.org/x/sys/windows/svc` — 服务控制 API（Status, Stop, 等控制码）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `golang.org/x/sys v0.41.0` — 已在 go.mod 中，`svc/mgr` 子包可直接导入，无需新增依赖
- `internal/lifecycle/servicedetect_windows.go` — 已有 svc 包导入和 IsServiceMode()，同包内可直接使用
- `internal/config/service.go` — ServiceConfig 提供注册所需的所有参数（service_name, display_name）
- main.go 占位代码 — 第 116-123 行和 125-136 行已预留调用位置，Phase 48 填充实际逻辑

### Established Patterns
- build tags: `//go:build windows` 用于 Windows 特定代码，配套 `//go:build !windows` stub
- 子段配置模式：每个子配置类型独立文件
- 上下文感知日志 (logger.With 预注入)
- 优雅降级 (失败不中断整体流程)
- 退出码策略：0=正常退出，1=错误退出，2=注册服务后退出（Phase 46 D-09）

### Integration Points
- `cmd/nanobot-auto-updater/main.go:125-136` — auto_start: true 注册入口（替换占位代码）
- `cmd/nanobot-auto-updater/main.go:116-123` — auto_start: false 卸载入口（替换占位代码）
- `internal/config/service.go` — 读取 cfg.Service.ServiceName 和 cfg.Service.DisplayName 传给 CreateService
- `internal/lifecycle/` — ServiceManager 新代码放置位置

</code_context>

<specifics>
## Specific Ideas

- 注册和卸载逻辑放在 `internal/lifecycle/` 包内，与 ServiceHandler、IsServiceMode 同包内聚
- ServiceManager 提供两个公开方法：`RegisterService()` 和 `UnregisterService()`
- main.go 占位代码处直接调用这两个方法
- 恢复策略在注册服务后立即配置（同一个函数调用内完成）
- 非管理员运行时清晰的错误提示，引导用户以管理员身份运行

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---
*Phase: 48-service-manager*
*Context gathered: 2026-04-11*

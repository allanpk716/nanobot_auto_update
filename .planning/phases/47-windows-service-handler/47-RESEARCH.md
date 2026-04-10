# Phase 47: Windows Service Handler - Research

**Researched:** 2026-04-10
**Domain:** golang.org/x/sys/windows/svc service lifecycle management
**Confidence:** HIGH

## Summary

Phase 47 implements the `svc.Handler` interface's `Execute` method so Windows SCM can start and stop the program through standard service interfaces. The core work involves (1) creating a `ServiceHandler` struct implementing `svc.Handler`, (2) extracting the current inline startup/shutdown logic from `main.go` into reusable functions (`AppStartup` / `AppShutdown`), and (3) wiring the service mode path in `main.go` to call `svc.Run()`.

The `golang.org/x/sys/windows/svc` package (already in `go.mod` at v0.41.0) provides a straightforward Handler interface: `Execute(args []string, r <-chan ChangeRequest, s chan<- Status) (svcSpecificEC bool, exitCode uint32)`. The Execute method runs in a goroutine spawned by `svc.Run`, communicating with SCM via channels. Status transitions follow: StartPending -> Running -> StopPending -> Stopped.

The existing codebase has clean shutdown patterns already -- all components (NotificationManager, NetworkMonitor, HealthMonitor, cron, UpdateLogger, APIServer) have Stop()/Shutdown()/Close() methods. The main.go shutdown sequence at lines 289-317 provides the exact order to preserve. Extracting this into a shared `AppShutdown` function with a configurable context timeout (10s console, 30s service) is the primary refactoring task.

**Primary recommendation:** Extract startup/shutdown from main.go into `AppComponents` container + `AppStartup`/`AppShutdown` functions in `internal/lifecycle/`, then implement `ServiceHandler.Execute` to call those functions while managing SCM state transitions.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** ServiceHandler 结构体放在 `internal/lifecycle/service_windows.go`，与现有 `IsServiceMode()` 同包，生命周期相关代码内聚
- **D-02:** 使用标准状态机：StartPending -> Running -> StopPending -> Stopped，通过 `s chan<- svc.Status` 向 SCM 报告状态
- **D-03:** Execute 方法接收 `svc.ChangeRequest`，处理 `svc.Interrogate`、`svc.Stop`、`svc.Shutdown` 控制码
- **D-04:** Stop 和 Shutdown 执行相同的优雅关闭流程（均为 30 秒超时）
- **D-05:** 抽取共用关闭函数 `AppShutdown(ctx context.Context, components)` 从 main.go，服务模式和控制台模式复用同一关闭逻辑
- **D-06:** 统一超时策略：调用方传入 context 超时（控制台模式 10 秒，服务模式 30 秒）
- **D-07:** 关闭顺序与现有 main.go 一致：通知管理器 -> 网络监控 -> 健康监控 -> cron -> UpdateLogger -> API 服务器
- **D-08:** 需要一个组件容器结构体（如 `AppComponents`）持有所有需要关闭的组件引用，传递给 AppShutdown
- **D-09:** main.go 分支调用：服务模式下初始化日志、配置后，构造 ServiceHandler 并调用 `svc.Run(serviceName, handler)`
- **D-10:** Execute 方法内部启动所有业务组件（实例管理、HTTP API、定时任务等），与控制台模式相同的启动逻辑
- **D-11:** 控制台模式走现有 main.go 流程不变（signal.Notify -> AppShutdown）
- **D-12:** 服务模式下 Execute 返回后程序退出，不需要额外 os.Exit

### Claude's Discretion
- ServiceHandler 结构体的具体字段设计
- AppComponents 容器结构体的字段组织方式
- Execute 内部 goroutine 的启动模式
- 非服务控制码（如 ParamChange、SessionChange）的忽略方式
- 测试策略（单元测试模拟 ChangeRequest channel）

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SVC-02 | 实现 svc.Handler 接口的 Execute 方法，处理服务启动/停止/关机请求 | `svc.Handler` interface documented; Execute signature: `Execute(args []string, r <-chan ChangeRequest, s chan<- Status) (svcSpecificEC bool, exitCode uint32)`. Handle `svc.Stop`, `svc.Shutdown`, `svc.Interrogate` cmds from `r` channel. Report state via `s` channel. |
| SVC-03 | 服务模式优雅关闭，响应 Stop 和 Shutdown 控制码，确保资源清理 | Existing shutdown sequence in main.go lines 289-317 covers: NotificationManager.Stop() -> NetworkMonitor.Stop() -> HealthMonitor.Stop() -> cron.Stop() -> UpdateLogger.Close() -> APIServer.Shutdown(ctx). Extract to shared `AppShutdown` function with 30s context timeout for service mode. |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| golang.org/x/sys/windows/svc | v0.41.0 (in go.mod) | Windows Service Control Manager integration | Official Go x/sys package -- the only standard way to implement Windows services in Go [VERIFIED: go.mod] |
| golang.org/x/sys/windows/svc/debug | v0.41.0 (in go.mod) | Debug mode for running service handler on console | Official testing utility -- `debug.Run()` executes Handler in console mode with Ctrl+C as Stop [VERIFIED: pkg.go.dev] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| robfig/cron/v3 | v3.0.1 (in go.mod) | Scheduled task cleanup | Already used in main.go for log cleanup; needs Stop() in shutdown |
| testify | v1.11.1 (in go.mod) | Testing assertions | For unit tests simulating ChangeRequest channel |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| golang.org/x/sys/windows/svc | kardianos/service | kardianos/service wraps svc with cross-platform abstraction but adds an unnecessary dependency and abstraction layer. The project is Windows-only and svc is already in go.mod. Direct svc usage gives full control over state transitions. |

**Installation:**
```bash
# No new dependencies needed -- golang.org/x/sys v0.41.0 already in go.mod
```

**Version verification:** `golang.org/x/sys v0.41.0` confirmed in go.mod. The package has since updated to v0.43.0 on pkg.go.dev (published 2026-03-27), but v0.41.0 is sufficient for svc.Handler. No upgrade required.

## Architecture Patterns

### Recommended Project Structure
```
internal/lifecycle/
    servicedetect_windows.go   # Existing: IsServiceMode() -- Phase 46
    servicedetect.go           # Existing: non-windows stub -- Phase 46
    service_windows.go         # NEW: ServiceHandler, Execute, AppComponents, AppStartup, AppShutdown
    service.go                 # NEW: non-windows stub for AppShutdown/AppStartup (or build-tag gated)
cmd/nanobot-auto-updater/
    main.go                    # MODIFIED: service mode branch calls svc.Run()
```

### Pattern 1: Handler.Execute State Machine
**What:** The Execute method implements a select loop reading ChangeRequests from `r` and writing Status to `s`.
**When to use:** This is the only pattern for svc.Handler -- every Windows service in Go follows this.
**Example:**
```go
// Source: https://pkg.go.dev/golang.org/x/sys/windows/svc
// Simplified from official example at:
// https://raw.githubusercontent.com/golang/sys/master/windows/svc/example/service.go

func (h *ServiceHandler) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
    const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

    // Report starting
    s <- svc.Status{State: svc.StartPending}

    // Start all business components
    components, err := AppStartup(h.cfg, h.logger)
    if err != nil {
        s <- svc.Status{State: svc.Stopped}
        return true, 1 // service-specific error code
    }
    h.components = components

    // Report running
    s <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

loop:
    for {
        select {
        case c := <-r:
            switch c.Cmd {
            case svc.Interrogate:
                s <- c.CurrentStatus
            case svc.Stop, svc.Shutdown:
                break loop
            default:
                // Ignore ParamChange, SessionChange, etc.
            }
        }
    }

    // Report stopping
    s <- svc.Status{State: svc.StopPending}

    // Graceful shutdown with 30s timeout
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    AppShutdown(shutdownCtx, components, h.logger)

    s <- svc.Status{State: svc.Stopped}
    return false, 0
}
```

### Pattern 2: AppComponents Container
**What:** A struct holding references to all initialized components, enabling clean shutdown ordering.
**When to use:** Both service mode and console mode need the same component references for shutdown.
**Example:**
```go
type AppComponents struct {
    NotificationManager *notification.NotificationManager
    NetworkMonitor      *network.NetworkMonitor
    HealthMonitor       *health.HealthMonitor
    CleanupCron         *cron.Cron
    UpdateLogger        *updatelog.UpdateLogger
    APIServer           *api.Server
}
```

### Pattern 3: Shared AppShutdown Function
**What:** A single function that performs ordered component shutdown, called from both service mode and console mode.
**When to use:** Any mode that needs to clean up resources before exit.
**Example:**
```go
func AppShutdown(ctx context.Context, c *AppComponents, logger *slog.Logger) {
    // D-07: Order matches existing main.go lines 289-317
    if c.NotificationManager != nil {
        c.NotificationManager.Stop()
    }
    if c.NetworkMonitor != nil {
        c.NetworkMonitor.Stop()
    }
    if c.HealthMonitor != nil {
        c.HealthMonitor.Stop()
    }
    if c.CleanupCron != nil {
        c.CleanupCron.Stop()
        logger.Info("Update log cleanup scheduler stopped")
    }
    if c.UpdateLogger != nil {
        if err := c.UpdateLogger.Close(); err != nil {
            logger.Error("Failed to close update logger", "error", err)
        }
    }
    if c.APIServer != nil {
        if err := c.APIServer.Shutdown(ctx); err != nil {
            logger.Error("API server shutdown error", "error", err)
        }
    }
    logger.Info("Shutdown completed")
}
```

### Pattern 4: main.go Service Mode Branch
**What:** After IsServiceMode() and config loading, branch into svc.Run() instead of inline startup.
**When to use:** At the existing service mode check point (main.go line 70).
**Example:**
```go
if inService {
    // D-09: Service mode -- construct handler and call svc.Run
    handler := lifecycle.NewServiceHandler(cfg, logger, Version)
    if err := svc.Run(cfg.Service.ServiceName, handler); err != nil {
        slog.Error("Service execution failed", "error", err)
        os.Exit(1)
    }
    return // D-12: Execute returns -> program exits
}
```

### Anti-Patterns to Avoid
- **Blocking in Execute without select on `r`:** The `r` channel MUST be continuously read. If Execute blocks on component startup without reading `r`, SCM cannot send Stop commands and will mark the service as non-responsive after timeout. Solution: start components in a goroutine and use a done channel to signal completion, while keeping the select loop active.
- **Forgetting to report StartPending before slow init:** SCM expects status updates during startup. If init takes more than 30 seconds without CheckPoint updates, SCM kills the process. For this app, init is fast (sub-second), so StartPending -> Running transition is sufficient.
- **Calling os.Exit inside Execute:** The svc.Run function handles process lifecycle. Execute should simply return, and the svc package calls the appropriate Windows API to report Stopped. os.Exit inside Execute skips SCM state reporting.
- **Sharing global state between Handler and main:** The Handler is only used in service mode; main's inline path is only used in console mode. They never run simultaneously. Keep them separate.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Windows SCM integration | Custom Win32 API calls via syscall | `golang.org/x/sys/windows/svc` | svc package wraps RegisterServiceCtrlHandlerEx, StartServiceCtrlDispatcher, SetServiceStatus -- complex Win32 API with strict calling conventions |
| Service mode detection | Process token or parent process checks | `svc.IsWindowsService()` (already used in Phase 46) | Properly handles Session 0 isolation, service host detection |
| Debug/test without SCM | Running as actual service during development | `svc/debug.Run()` | Executes Handler in console mode, Ctrl+C sends Stop command |

**Key insight:** The svc package is essentially a thin wrapper around three Win32 APIs (StartServiceCtrlDispatcher, RegisterServiceCtrlHandlerEx, SetServiceStatus). Building this from scratch would require CGo or manual syscall wrangling with callback registration -- exactly the kind of low-level Windows API work that the x/sys team has already solved.

## Common Pitfalls

### Pitfall 1: Execute Blocks Without Reading ChangeRequest Channel
**What goes wrong:** If Execute starts components synchronously and doesn't enter the select loop, SCM cannot send Stop. After ~30 seconds, SCM marks the service as unresponsive and may kill it.
**Why it happens:** Developers put all startup logic inline before the select loop, not realizing init could be slow or SCM might send Stop early.
**How to avoid:** Start components in goroutines. Enter the select loop immediately after reporting Running. Use a done channel or WaitGroup for startup completion.
**Warning signs:** Service appears to start but cannot be stopped from services.msc; SCM shows "stopping" indefinitely.

### Pitfall 2: Missing StopPending Status Before Slow Shutdown
**What goes wrong:** If shutdown takes time (HTTP server draining, goroutines stopping), SCM doesn't know the service is shutting down and may send additional control codes or kill the process.
**Why it happens:** Developers skip StopPending and go straight from Running to Stopped.
**How to avoid:** Always report StopPending before beginning shutdown, then Stopped after completion. For very long shutdowns, use CheckPoint/WaitHint to report progress.
**Warning signs:** Service shows "Running" during shutdown, then suddenly "Stopped" without intermediate state.

### Pitfall 3: svc.Run Called from Non-Service Context
**What goes wrong:** Calling svc.Run() from a console process will fail immediately because StartServiceCtrlDispatcher requires the process to have been launched by SCM.
**Why it happens:** Confusion about when to call svc.Run -- it only works when the process IS a Windows service.
**How to avoid:** Only call svc.Run inside the `inService` branch, which is guarded by `IsServiceMode()`. For testing, use `debug.Run()`.
**Warning signs:** svc.Run returns error "The service process could not connect to the service controller."

### Pitfall 4: Component Start Order Causes Nil Pointer in Shutdown
**What goes wrong:** If startup fails partway through (e.g., API server creation fails), some components are nil. Shutdown code must handle nil checks.
**Why it happens:** Existing code already handles this (nil checks in main.go lines 289-314), but refactoring to AppComponents must preserve this pattern.
**How to avoid:** AppComponents fields start as nil. AppStartup should return partial components on error. AppShutdown must check each field for nil before calling Stop/Close.
**Warning signs:** Nil pointer dereference during shutdown after a startup failure.

### Pitfall 5: Context Timeout vs Component Stop Behavior
**What goes wrong:** Some components' Stop() methods use internal context cancellation (NetworkMonitor, HealthMonitor, NotificationManager) and return immediately. Others like APIServer.Shutdown(ctx) actually wait for the context. The overall shutdown timeout must account for all components.
**Why it happens:** Mixing sync and async stop patterns.
**How to avoid:** Pass the timeout context to APIServer.Shutdown (the only component that respects it). Other Stop() calls are near-instant. Total shutdown should complete well within 30 seconds.
**Warning signs:** Shutdown takes longer than expected; service marked as unresponsive.

## Code Examples

### Complete ServiceHandler Implementation Pattern
```go
// Source: Based on official example at https://pkg.go.dev/golang.org/x/sys/windows/svc/example
// and verified against service.go source at https://raw.githubusercontent.com/golang/sys/master/windows/svc/service.go

//go:build windows

package lifecycle

import (
    "context"
    "log/slog"
    "time"

    "golang.org/x/sys/windows/svc"
    "github.com/HQGroup/nanobot-auto-updater/internal/config"
)

// ServiceHandler implements svc.Handler for Windows service lifecycle.
type ServiceHandler struct {
    cfg        *config.Config
    logger     *slog.Logger
    version    string
    components *AppComponents
}

// NewServiceHandler creates a new service handler.
func NewServiceHandler(cfg *config.Config, logger *slog.Logger, version string) *ServiceHandler {
    return &ServiceHandler{
        cfg:     cfg,
        logger:  logger,
        version: version,
    }
}

// Execute is called by svc.Run to run the service.
// It must read from r for control requests and write to s for status updates.
func (h *ServiceHandler) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
    const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

    // Report starting state
    s <- svc.Status{State: svc.StartPending}

    // Initialize all business components
    components, err := AppStartup(h.cfg, h.logger, h.version)
    if err != nil {
        h.logger.Error("Service startup failed", "error", err)
        s <- svc.Status{State: svc.Stopped}
        return true, 1
    }
    h.components = components

    // Report running state
    s <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
    h.logger.Info("Service is running")

    // Main event loop -- wait for Stop/Shutdown
loop:
    for {
        select {
        case c := <-r:
            switch c.Cmd {
            case svc.Interrogate:
                s <- c.CurrentStatus
            case svc.Stop, svc.Shutdown:
                h.logger.Info("Service stop requested", "cmd", c.Cmd)
                break loop
            default:
                // Ignore other control codes
            }
        }
    }

    // Report stopping
    s <- svc.Status{State: svc.StopPending}

    // Graceful shutdown with 30-second timeout (D-04, D-06)
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    AppShutdown(shutdownCtx, components, h.logger)

    // Report stopped -- svc package will handle process exit
    s <- svc.Status{State: svc.Stopped}
    return false, 0
}
```

### Non-Windows Stub for ServiceHandler
```go
// Source: Pattern established by servicedetect.go (Phase 46)
//go:build !windows

package lifecycle

// NewServiceHandler is not available on non-Windows platforms.
// This stub ensures the package compiles on all platforms.
// On Windows, use the build-tagged service_windows.go implementation.
```

### AppStartup Function Signature
```go
// AppStartup initializes all application components and returns a container.
// This function encapsulates the component initialization from main.go lines 148-273.
// It should be called from both service mode (in Execute) and console mode (in main.go).
func AppStartup(cfg *config.Config, logger *slog.Logger, version string) (*AppComponents, error) {
    // ... component initialization matching main.go lines 148-273 ...
    // Returns partial components even on error (for clean shutdown of what was started)
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `svc.IsAnInteractiveSession()` | `svc.IsWindowsService()` | golang.org/x/sys v0.19.0+ (2022) | IsAnInteractiveSession was deprecated; IsWindowsService is more reliable and already used in Phase 46 |
| Manual Win32 service API via syscall | `svc.Run()` + `Handler` interface | Since Go 1.x | No need for direct Win32 API calls -- the svc package wraps everything |

**Deprecated/outdated:**
- `svc.IsAnInteractiveSession()`: Deprecated in favor of `svc.IsWindowsService()`. Already handled in Phase 46.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | All components' Stop() methods are non-blocking and complete quickly (sub-second) except APIServer.Shutdown(ctx) which waits for context | Architecture Patterns | If any Stop() blocks for seconds, total shutdown may exceed 30s timeout |
| A2 | Component startup in main.go lines 148-273 completes in under 30 seconds (SCM default wait hint) | Architecture Patterns | If startup is slow, SCM may kill the process before Running is reported |
| A3 | The AppStartup function can be extracted cleanly without breaking the existing console mode path | Architecture Patterns | Refactoring bugs could break existing functionality |
| A4 | cron.Cron.Stop() is safe to call and waits for running jobs to complete | Common Pitfalls | If Stop() hangs, shutdown sequence stalls |

**Note:** A1 and A2 are LOW risk -- the codebase uses goroutines for all component starts, and there are no blocking operations in init. A3 is the primary risk and should be verified by keeping console mode behavior identical after refactoring.

## Open Questions

1. **Should AppStartup and AppShutdown live in lifecycle package or main package?**
   - What we know: CONTEXT.md D-01 says ServiceHandler goes in `internal/lifecycle/`. D-05 says extract AppShutdown from main.go.
   - What's unclear: Whether AppStartup/AppShutdown should be in `internal/lifecycle/` (sharing the package with ServiceHandler) or remain in `main` package (where they're used).
   - Recommendation: Put them in `internal/lifecycle/` -- the CONTEXT.md decision D-01 establishes lifecycle as the package for service-related code, and AppShutdown is shared between both modes. This also improves testability since internal packages can be imported by tests.

2. **Should the non-windows stub provide no-op AppStartup/AppShutdown or panic?**
   - What we know: The project is Windows-only but uses build tags for cross-platform compilation.
   - What's unclear: Whether AppStartup/AppShutdown need non-windows stubs.
   - Recommendation: Since the console mode path also uses AppShutdown, and main.go imports it, these functions should be platform-independent (no build tags needed). Only ServiceHandler needs Windows build tags.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Build & test | Yes | go1.24.11 (go.mod) | -- |
| golang.org/x/sys/windows/svc | Service handler | Yes | v0.41.0 (go.mod) | -- |
| Windows SCM | Runtime service registration | N/A (Phase 48) | -- | svc/debug.Run for testing |

**Missing dependencies with no fallback:**
- None -- all dependencies are already in go.mod.

**Missing dependencies with fallback:**
- None.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | testing + testify v1.11.1 |
| Config file | none (Go convention) |
| Quick run command | `go test ./internal/lifecycle/ -short -count=1` |
| Full suite command | `go test ./internal/lifecycle/ -count=1 -v` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SVC-02 | Execute handles StartPending -> Running state transition | unit | `go test ./internal/lifecycle/ -run TestServiceHandler -short` | No -- Wave 0 |
| SVC-02 | Execute handles Interrogate control code | unit | `go test ./internal/lifecycle/ -run TestServiceHandler_Interrogate -short` | No -- Wave 0 |
| SVC-02 | Execute handles Stop control code | unit | `go test ./internal/lifecycle/ -run TestServiceHandler_Stop -short` | No -- Wave 0 |
| SVC-02 | Execute handles Shutdown control code | unit | `go test ./internal/lifecycle/ -run TestServiceHandler_Shutdown -short` | No -- Wave 0 |
| SVC-03 | AppShutdown cleans up all components in order | unit | `go test ./internal/lifecycle/ -run TestAppShutdown -short` | No -- Wave 0 |
| SVC-03 | Shutdown completes within 30 second timeout | unit | `go test ./internal/lifecycle/ -run TestAppShutdown_Timeout -short` | No -- Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/lifecycle/ -short -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/lifecycle/service_test.go` -- unit tests for ServiceHandler.Execute with mocked ChangeRequest channel
- [ ] `internal/lifecycle/shutdown_test.go` -- unit tests for AppShutdown with mock components
- [ ] Existing test infrastructure (capture_test.go, update_state_test.go) provides patterns to follow

### Testing Strategy for svc.Handler
The `svc.Handler` interface uses unexported channel types, making it impossible to mock directly. The recommended approach:
1. Create a test helper that simulates the `r <-chan svc.ChangeRequest` and `s chan<- svc.Status` channels
2. Send ChangeRequests (Stop, Shutdown, Interrogate) via the `r` channel
3. Read and assert Status values from the `s` channel
4. Use `svc/debug.Run()` for integration testing on Windows (runs handler in console mode, Ctrl+C = Stop)

```go
// Test pattern for Execute
func TestServiceHandler_Stop(t *testing.T) {
    // Create channels matching Handler.Execute signature
    crChan := make(chan svc.ChangeRequest)
    statusChan := make(chan svc.Status)

    handler := NewServiceHandler(testCfg, testLogger, "test")

    // Run Execute in goroutine
    done := make(chan struct{})
    go func() {
        handler.Execute([]string{}, crChan, statusChan)
        close(done)
    }()

    // Read and assert status transitions
    assertStatus(t, statusChan, svc.StartPending)
    assertStatus(t, statusChan, svc.Running)

    // Send Stop command
    crChan <- svc.ChangeRequest{Cmd: svc.Stop}

    assertStatus(t, statusChan, svc.StopPending)
    assertStatus(t, statusChan, svc.Stopped)

    // Verify Execute returned
    <-done
}
```

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | Service runs under configured Windows account (Phase 48 configures this) |
| V3 Session Management | No | N/A for service lifecycle |
| V4 Access Control | Yes | Service runs with whatever privileges the configured Windows account has |
| V5 Input Validation | Yes | Service name validated by config.ServiceConfig.Validate() (alphanumeric only) |
| V6 Cryptography | No | N/A for service lifecycle |

### Known Threat Patterns for Windows Service

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Service executable path tampering | Tampering | Phase 48 sets service binary path; verify path integrity at startup |
| Service running as SYSTEM (excessive privileges) | Elevation of Privilege | Configure service to run as specific user account (Phase 48) |
| Uncontrolled service crash/restart loop | Denial of Service | SCM recovery policy configured in Phase 48 (MGR-04) |

## Sources

### Primary (HIGH confidence)
- pkg.go.dev/golang.org/x/sys/windows/svc -- Handler interface, Execute signature, Status/ChangeRequest types [VERIFIED: fetched 2026-04-10]
- raw.githubusercontent.com/golang/sys/master/windows/svc/service.go -- svc.Run implementation showing StartServiceCtrlDispatcher wrapping, Execute called in goroutine [VERIFIED: fetched 2026-04-10]
- raw.githubusercontent.com/golang/sys/master/windows/svc/example/service.go -- Official example showing state machine pattern [VERIFIED: fetched 2026-04-10]
- Project go.mod -- golang.org/x/sys v0.41.0 confirmed [VERIFIED: read from file]

### Secondary (MEDIUM confidence)
- pkg.go.dev/golang.org/x/sys/windows/svc/debug -- debug.Run() for console-mode testing [CITED: pkg.go.dev]
- Codebase analysis -- main.go shutdown sequence, component Stop/Close methods, config.ServiceConfig [VERIFIED: read from files]

### Tertiary (LOW confidence)
- None -- all findings verified from primary or secondary sources.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- svc package already in go.mod, API verified from official docs
- Architecture: HIGH -- state machine pattern is well-established, existing codebase has clear shutdown sequence
- Pitfalls: HIGH -- based on official documentation and common Windows service development knowledge

**Research date:** 2026-04-10
**Valid until:** 2026-05-10 (stable -- svc package API unchanged for years)

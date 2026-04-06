# Phase 43: Telegram Monitor Integration - Research

**Researched:** 2026-04-06
**Domain:** Go goroutine lifecycle management / per-instance monitor wiring
**Confidence:** HIGH

## Summary

Phase 43 wires the `TelegramMonitor` built in Phase 42 into `InstanceLifecycle`. Each instance owns its own monitor, created after successful `StartAfterUpdate()` and stopped before `StopForUpdate()`. The core technical challenge is managing the monitor goroutine's lifecycle correctly: it must receive its own `context.Context` (not the instance lifecycle's context) so `Stop()` can cancel it independently, and the goroutine must not leak on any code path.

The implementation touches four files: `lifecycle.go` (add notifier parameter + monitor field + start/stop wiring), `manager.go` (pass notifier to `NewInstanceLifecycle`), `main.go` (pass `notif` to `NewInstanceManager`), and a new test file `lifecycle_monitor_test.go`. All duck-typed interfaces (`LogSubscriber`, `Notifier`) are already satisfied by existing concrete types, so no adapter code is needed.

**Primary recommendation:** Add a `telegramMonitor *telegram.TelegramMonitor` field to `InstanceLifecycle`, inject `Notifier` via constructor, and wire Start/Stop into the existing lifecycle methods. Write unit tests using the same mock patterns from Phase 42 (`mockLogSubscriber`, `mockNotifier`).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** TelegramMonitor managed internally by InstanceLifecycle -- add `telegramMonitor` field, create+goroutine start in `StartAfterUpdate()` after success, call `monitor.Stop()` before stopping process in `StopForUpdate()`
- **D-02:** Consistent with existing patterns -- each InstanceLifecycle owns its own LogBuffer, logger, TelegramMonitor at the same level
- **D-03:** Constructor injection -- modify `NewInstanceLifecycle(cfg, baseLogger, notifier)` signature to add Notifier parameter
- **D-04:** Notifier is immutable after construction -- matches project DI pattern
- **D-05:** Must update all `NewInstanceLifecycle` call sites (in `manager.go` via `NewInstanceManager`)
- **D-06:** Unit tests as primary strategy -- mock LogSubscriber + Notifier, verify InstanceLifecycle monitor create/start/stop logic
- **D-07:** Test style consistent with Phase 42 -- fast execution, race detector verified

### Claude's Discretion
- Monitor goroutine panic recovery implementation details
- Log message specific wording
- Test case boundary condition selection

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TELE-07 | Instances that never produce "Starting Telegram bot" log line run normally without any monitor overhead or spurious notifications | Guaranteed by TelegramMonitor design: `Start()` blocks on channel read, state machine stays `stateIdle` unless trigger pattern seen. No timer created, no goroutines spawned beyond the blocking read. |
| TELE-09 | When instance is stopped (for update or shutdown), any in-progress Telegram monitor for that instance is immediately cancelled and no timeout or failure notification is sent | `TelegramMonitor.Stop()` calls `m.cancel()` which cancels the internal context. The `Start()` goroutine exits via `ctx.Done()` select case. Timer is also stopped via `m.timer.Stop()`. Verified in Phase 42 `TestMonitor_StopCancelsTimer`. |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| stdlib `context` | Go 1.24 | Monitor goroutine cancellation | Project already uses context throughout lifecycle |
| stdlib `sync` | Go 1.24 | Mutex for monitor field access | Same pattern as TelegramMonitor itself |
| `github.com/HQGroup/nanobot-auto-updater/internal/telegram` | local | TelegramMonitor, patterns, interfaces | Phase 42 output, canonical |
| `github.com/HQGroup/nanobot-auto-updater/internal/logbuffer` | local | LogBuffer (satisfies LogSubscriber) | Existing per-instance buffer |
| `github.com/HQGroup/nanobot-auto-updater/internal/notifier` | local | Notifier (satisfies telegram.Notifier) | Existing Pushover notifier |
| `github.com/stretchr/testify` | v1.11.1 | Test assertions | Project-wide testing standard |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `log/slog` | Go 1.24 | Structured logging | All monitor operations |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Field on InstanceLifecycle | Separate MonitorManager | Extra complexity, no benefit for 1:1 relationship |
| Constructor injection | Method injection (SetNotifier) | D-04 locked: immutable after construction |
| Duck-typed Notifier interface | Explicit interface in instance package | Duck typing avoids import cycles, matches Phase 42 |

**Installation:**
No new dependencies required -- all packages are stdlib or existing internal packages.

**Version verification:**
```
stretchr/testify v1.11.1 (verified in go.mod)
Go 1.24.11 (verified via `go version`)
```

## Architecture Patterns

### Recommended Project Structure (changes only)
```
internal/instance/
  lifecycle.go               # MODIFY: add notifier param + monitor field + wiring
  lifecycle_monitor_test.go  # NEW: monitor integration tests
  manager.go                 # MODIFY: pass notifier through to NewInstanceLifecycle

cmd/nanobot-auto-updater/
  main.go                    # MODIFY: pass notif to NewInstanceManager
```

### Pattern 1: Per-Instance Monitor Lifecycle
**What:** Each `InstanceLifecycle` creates, starts, and stops its own `TelegramMonitor`.
**When to use:** Always -- the 1:1 relationship between instance and monitor is inherent.
**Example:**
```go
// In InstanceLifecycle struct
type InstanceLifecycle struct {
    config          config.InstanceConfig
    logger          *slog.Logger
    logBuffer       *logbuffer.LogBuffer
    pid             int32
    notifier        Notifier   // NEW: injected via constructor
    telegramMonitor *telegram.TelegramMonitor  // NEW: per-instance monitor
    monitorCancel   context.CancelFunc  // NEW: to cancel monitor goroutine
}

// Constructor change
func NewInstanceLifecycle(cfg config.InstanceConfig, baseLogger *slog.Logger, notifier Notifier) *InstanceLifecycle {
    // ... existing code ...
    return &InstanceLifecycle{
        config:    cfg,
        logger:    instanceLogger,
        logBuffer: logBuffer,
        notifier:  notifier,
    }
}
```
[VERIFIED: codebase grep of InstanceLifecycle struct in lifecycle.go]

### Pattern 2: Start-after-success, Stop-before-process
**What:** Monitor is created and started in `StartAfterUpdate()` after successful process start. Monitor is stopped in `StopForUpdate()` before stopping the process.
**When to use:** The monitor needs to watch log output from the running process, so it can only start after the process is confirmed running and must stop before the process is killed.
**Example:**
```go
// In StartAfterUpdate(), after successful start:
monitor := telegram.NewTelegramMonitor(
    il.logBuffer, il.notifier, il.config.Name,
    telegram.DefaultTimeout, il.logger,
)
monitorCtx, monitorCancel := context.WithCancel(context.Background())
il.telegramMonitor = monitor
il.monitorCancel = monitorCancel
go monitor.Start(monitorCtx)

// In StopForUpdate(), before stopping process:
if il.telegramMonitor != nil {
    il.telegramMonitor.Stop()
    il.monitorCancel()
    il.telegramMonitor = nil
    il.monitorCancel = nil
}
```
[VERIFIED: CONTEXT.md D-01 specifies StartAfterUpdate/StopForUpdate wiring]

### Pattern 3: Notifier Interface (Duck Typing)
**What:** `InstanceLifecycle` stores a local interface matching `notifier.Notifier`'s method set, avoiding import of the notifier package from the instance package.
**When to use:** Same duck-typing pattern used by `telegram.Notifier` in Phase 42.
**Example:**
```go
// In lifecycle.go -- local interface, same methods as notifier.Notifier
type Notifier interface {
    IsEnabled() bool
    Notify(title, message string) error
}

// *notifier.Notifier satisfies this via duck typing -- no import needed
```
[VERIFIED: Pattern established in Phase 35 (trigger.go local interface) and Phase 42 (telegram package)]

### Anti-Patterns to Avoid
- **Sharing context between instance lifecycle and monitor:** If monitor uses the instance lifecycle's context, cancelling the lifecycle context would cascade. Monitor must have its own `context.WithCancel(context.Background())` so `Stop()` controls it independently. [VERIFIED: CONTEXT.md specifics]
- **Stopping monitor after process kill:** If process is killed first, log channel closes, but timer may still fire. Must stop monitor BEFORE killing process. [VERIFIED: CONTEXT.md D-01]
- **Nil-notifier panic:** If Pushover is not configured, `notifier.NewWithConfig()` returns a disabled notifier (non-nil, `IsEnabled()=false`). The `Notifier` parameter to `NewInstanceLifecycle` must never be nil -- guaranteed by main.go construction order (notif created before InstanceManager). [VERIFIED: main.go lines 131-141]
- **Forgetting monitorCancel():** `TelegramMonitor.Stop()` cancels the internal context AND stops the timer, but the caller's context (passed to `Start()`) also needs cancellation to unblock the channel read. Both `monitor.Stop()` and `monitorCancel()` must be called. [VERIFIED: monitor.go Start() reads from ctx.Done()]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Monitor state machine | Custom watcher in instance package | `telegram.TelegramMonitor` | Phase 42 already implements full state machine with timer, pattern matching, notification |
| Log subscription | Direct channel access | `logbuffer.LogBuffer.Subscribe()` | Handles history replay, goroutine management, non-blocking sends |
| Pattern detection | String matching in lifecycle | `telegram.IsTrigger/IsSuccess/IsFailure` | Centralized patterns, testable independently |
| Notification delivery | Direct Pushover call | `telegram.Notifier` (duck typed) | Handles disabled state, async sending, panic recovery |

**Key insight:** This phase is pure wiring -- connecting existing, tested components. No new business logic needed.

## Common Pitfalls

### Pitfall 1: Monitor goroutine leak on failed start
**What goes wrong:** If `StartAfterUpdate()` succeeds (process starts) but something fails after creating the monitor, the goroutine runs forever.
**Why it happens:** Monitor is created and started in a goroutine before all post-start work completes.
**How to avoid:** Create and start monitor as the LAST thing in `StartAfterUpdate()`, after all error checks pass. If anything fails before that point, no monitor exists to leak.
**Warning signs:** Goroutine count grows over time in `runtime.NumGoroutine()`.

### Pitfall 2: Double-stop panic
**What goes wrong:** Calling `Stop()` on a monitor that was already stopped, or calling `monitorCancel()` on a nil `monitorCancel`.
**Why it happens:** Multiple stop paths (StopForUpdate + Clear + restart) without nil checks.
**How to avoid:** Always nil-check before calling Stop/cancel. Set fields to nil after stopping. The existing `StopForUpdate()` already handles `pid == 0` gracefully -- extend this pattern.
**Warning signs:** Nil pointer dereference in StopForUpdate.

### Pitfall 3: Race between monitor goroutine and Stop()
**What goes wrong:** Stop is called while `Start()` is still in its initial setup (setting `startTime`).
**Why it happens:** `Start()` sets `startTime = time.Now()` after subscribing but before entering the main loop. If Stop cancels context before this completes, the unsubscribe path runs concurrently.
**How to avoid:** TelegramMonitor already handles this -- `Start()` uses `defer m.logBuffer.Unsubscribe(ch)` and checks `ctx.Done()` in the select. The Phase 42 test `TestMonitor_ContextCancelledBeforeTrigger` verifies this exact scenario.
**Warning signs:** Flaky tests with race detector enabled.

### Pitfall 4: Import cycle with telegram package
**What goes wrong:** `instance` package imports `telegram` package, and `telegram` imports `logbuffer` -- this is fine. But if `telegram` tried to import `instance`, it would create a cycle.
**Why it happens:** Adding types from the wrong direction.
**How to avoid:** `instance` imports `telegram` (to use `TelegramMonitor` and `NewTelegramMonitor`). `telegram` never imports `instance`. Duck-typed interfaces in `telegram` (`LogSubscriber`, `Notifier`) are satisfied by `logbuffer.LogBuffer` and `notifier.Notifier` without direct imports. [VERIFIED: go.mod has no circular dependencies]

### Pitfall 5: Notifier nil when Pushover not configured
**What goes wrong:** Passing nil as Notifier parameter, causing nil pointer dereference when monitor tries to check `IsEnabled()`.
**Why it happens:** Assuming notif could be nil when Pushover env vars not set.
**How to avoid:** `notifier.NewWithConfig()` always returns a non-nil `*Notifier` (with `enabled=false` when not configured). Verify main.go passes this directly -- it does (line 135-141).
**Warning signs:** Panic in `sendNotification()` with nil receiver.

## Code Examples

### InstanceLifecycle with Monitor Wiring
```go
// Source: Derived from CONTEXT.md D-01, D-02, D-03, existing lifecycle.go
package instance

import (
    "context"
    "time"

    "github.com/HQGroup/nanobot-auto-updater/internal/config"
    "github.com/HQGroup/nanobot-auto-updater/internal/telegram"
)

// Notifier interface for dependency injection (duck typing)
type Notifier interface {
    IsEnabled() bool
    Notify(title, message string) error
}

type InstanceLifecycle struct {
    config          config.InstanceConfig
    logger          *slog.Logger
    logBuffer       *logbuffer.LogBuffer
    pid             int32
    notifier        Notifier                      // NEW (D-03)
    telegramMonitor *telegram.TelegramMonitor      // NEW (D-01)
    monitorCancel   context.CancelFunc             // NEW: cancel monitor's context
}

func NewInstanceLifecycle(cfg config.InstanceConfig, baseLogger *slog.Logger, notifier Notifier) *InstanceLifecycle {
    instanceLogger := baseLogger.With("instance", cfg.Name).With("component", "instance-lifecycle")
    logBuffer := logbuffer.NewLogBuffer(instanceLogger)
    return &InstanceLifecycle{
        config:    cfg,
        logger:    instanceLogger,
        logBuffer: logBuffer,
        notifier:  notifier,
    }
}
```

### StartAfterUpdate with Monitor Creation
```go
// Source: CONTEXT.md D-01, existing lifecycle.go StartAfterUpdate
func (il *InstanceLifecycle) StartAfterUpdate(ctx context.Context) error {
    il.logger.Info("Starting instance after update")
    il.logBuffer.Clear()

    startupTimeout := il.config.StartupTimeout
    if startupTimeout == 0 {
        startupTimeout = 30 * time.Second
    }

    pid, err := lifecycle.StartNanobotWithCapture(ctx, il.config.StartCommand, il.config.Port, startupTimeout, il.logger, il.logBuffer)
    if err != nil {
        // ... error handling (unchanged)
        return &InstanceError{...}
    }

    il.pid = int32(pid)
    il.logger.Info("Instance started successfully with log capture", "pid", pid)

    // NEW (D-01): Start Telegram monitor after successful process start
    il.startTelegramMonitor()

    return nil
}

func (il *InstanceLifecycle) startTelegramMonitor() {
    monitor := telegram.NewTelegramMonitor(
        il.logBuffer,
        il.notifier,
        il.config.Name,
        telegram.DefaultTimeout,
        il.logger,
    )
    monitorCtx, cancel := context.WithCancel(context.Background())
    il.telegramMonitor = monitor
    il.monitorCancel = cancel

    go func() {
        defer func() {
            if r := recover(); r != nil {
                il.logger.Error("telegram monitor goroutine panic",
                    "panic", r)
            }
        }()
        monitor.Start(monitorCtx)
    }()

    il.logger.Info("Telegram monitor started for instance")
}
```

### StopForUpdate with Monitor Cancellation
```go
// Source: CONTEXT.md D-01, existing lifecycle.go StopForUpdate
func (il *InstanceLifecycle) StopForUpdate(ctx context.Context) error {
    il.logger.Info("Starting stop-before-update process")

    // NEW (D-01): Stop monitor before stopping process
    il.stopTelegramMonitor()

    if il.pid == 0 {
        il.logger.Info("Instance never started, nothing to stop")
        return nil
    }

    // ... existing stop logic unchanged
}

func (il *InstanceLifecycle) stopTelegramMonitor() {
    if il.telegramMonitor != nil {
        il.telegramMonitor.Stop()  // Cancels internal timer + context
        il.monitorCancel()         // Cancel caller's context (unblocks Start())
        il.telegramMonitor = nil
        il.monitorCancel = nil
        il.logger.Info("Telegram monitor stopped")
    }
}
```

### Manager Changes
```go
// Source: CONTEXT.md D-05, existing manager.go
func NewInstanceManager(cfg *config.Config, baseLogger *slog.Logger, notifier Notifier) *InstanceManager {
    logger := baseLogger.With("component", "instance-manager")
    instances := make([]*InstanceLifecycle, 0, len(cfg.Instances))
    for _, instCfg := range cfg.Instances {
        lifecycle := NewInstanceLifecycle(instCfg, baseLogger, notifier)  // NEW: pass notifier
        instances = append(instances, lifecycle)
    }
    return &InstanceManager{instances: instances, logger: logger}
}
```

### Main.go Changes
```go
// Source: CONTEXT.md, existing main.go
// Line 131: BEFORE notifier creation was after InstanceManager
// AFTER: Create InstanceManager AFTER notifier, pass notif to it
notif := notifier.NewWithConfig(notifier.Config{...}, logger)
instanceManager := instance.NewInstanceManager(cfg, logger, notif)  // NEW: pass notif
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Notifier created after InstanceManager | Notifier created before InstanceManager | Phase 43 | main.go creation order swap |
| InstanceLifecycle 2-param constructor | 3-param constructor (cfg, logger, notifier) | Phase 43 | All callers updated |
| InstanceManager 2-param constructor | 3-param constructor (cfg, logger, notifier) | Phase 43 | main.go call updated |

**Deprecated/outdated:**
- None -- this is a pure addition phase, no existing patterns deprecated.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `notifier.NewWithConfig()` always returns non-nil `*Notifier` | Anti-Patterns / Pitfall 5 | Nil pointer dereference at runtime |
| A2 | `monitorCancel()` after `monitor.Stop()` is safe (Stop cancels internal context, monitorCancel cancels external context passed to Start) | Code Examples | Goroutine leak if external context not cancelled |
| A3 | No need to wait for monitor goroutine to exit before returning from StopForUpdate | Code Examples | Race condition if process is killed while monitor is still reading from channel |

**Risk mitigation:** A1 is verified in code (notifier.go returns `&Notifier{}` even when disabled). A2 is by design -- two contexts need independent cancellation. A3 is safe because `monitor.Stop()` stops timer and `monitorCancel()` unblocks `Start()` channel read; the goroutine exits asynchronously and that is fine.

## Open Questions

1. **Monitor cleanup on Clear() -- is it needed?**
   - What we know: `StartAfterUpdate()` calls `logBuffer.Clear()` before starting. If a monitor exists from a previous start (which should not happen since StopForUpdate clears it), it would subscribe to a cleared buffer.
   - What's unclear: Whether `Clear()` should also stop any existing monitor as a safety measure.
   - Recommendation: Not needed -- the lifecycle guarantees Stop-before-Start. But `startTelegramMonitor()` could defensively call `stopTelegramMonitor()` first for safety (Claude's discretion).

2. **Should monitor goroutine exit be logged?**
   - What we know: The goroutine runs `monitor.Start(ctx)` which returns silently when context is cancelled.
   - What's unclear: Whether we should log "Telegram monitor goroutine exited" for debugging.
   - Recommendation: Log at Debug level on goroutine exit (Claude's discretion).

## Environment Availability

Step 2.6: SKIPPED (no external dependencies identified -- all changes are internal Go code with existing packages).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | none -- standard `go test` |
| Quick run command | `go test ./internal/instance/... -run TestMonitor -count=1 -v` |
| Full suite command | `go test ./internal/instance/... -count=1 -v` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TELE-07 | Instance without trigger log runs with zero monitor overhead | unit | `go test ./internal/instance/... -run TestMonitor_NoTriggerNoOverhead -count=1 -v` | No -- Wave 0 |
| TELE-09 | StopForUpdate cancels monitor, no timeout notification | unit | `go test ./internal/instance/... -run TestMonitor_StopCancelsMonitor -count=1 -v` | No -- Wave 0 |
| TELE-07 | InstanceLifecycle creates monitor after successful start | unit | `go test ./internal/instance/... -run TestMonitor_CreatedAfterStart -count=1 -v` | No -- Wave 0 |
| TELE-09 | StopForUpdate with no monitor (never started) returns nil | unit | `go test ./internal/instance/... -run TestMonitor_StopNoMonitor -count=1 -v` | No -- Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/instance/... -run TestMonitor -count=1 -v`
- **Per wave merge:** `go test ./internal/instance/... -count=1 -v`
- **Phase gate:** `go test ./... -count=1 -v` (full project suite)

### Wave 0 Gaps
- [ ] `internal/instance/lifecycle_monitor_test.go` -- covers TELE-07, TELE-09 integration tests
- [ ] Mock types may need to be defined (or reuse from telegram package -- but they are unexported, so local copies needed)

**Note:** The `mockLogSubscriber` and `mockNotifier` types in `telegram/monitor_test.go` are unexported. The instance package tests will need their own local mock implementations, or we define exported test helpers. Given project convention of per-package mocks, local copies are recommended.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | N/A |
| V3 Session Management | no | N/A |
| V4 Access Control | no | N/A |
| V5 Input Validation | no | Log content from own process, no external input |
| V6 Cryptography | no | N/A |

### Known Threat Patterns for Go Instance Lifecycle

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Goroutine leak | Denial of Service | Context cancellation + defer cleanup |
| Nil pointer dereference | Denial of Service | Nil checks before field access |
| Race condition | Tampering | Mutex protection + race detector tests |

## Sources

### Primary (HIGH confidence)
- Codebase: `internal/telegram/monitor.go` -- TelegramMonitor implementation, Start/Stop/processEntry
- Codebase: `internal/telegram/patterns.go` -- TriggerPattern, SuccessPattern, FailurePattern constants
- Codebase: `internal/instance/lifecycle.go` -- InstanceLifecycle struct, StartAfterUpdate, StopForUpdate
- Codebase: `internal/instance/manager.go` -- InstanceManager, NewInstanceManager, StartAllInstances
- Codebase: `internal/notifier/notifier.go` -- Notifier struct, duck-type compatible with telegram.Notifier
- Codebase: `cmd/nanobot-auto-updater/main.go` -- Main wiring, creation order
- Codebase: `go.mod` -- Dependency versions verified

### Secondary (MEDIUM confidence)
- CONTEXT.md D-01 through D-07 -- Locked decisions from discuss phase

### Tertiary (LOW confidence)
- None -- all findings verified against source code

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all packages are existing internal code, no new dependencies
- Architecture: HIGH -- wiring pattern follows established project conventions, CONTEXT.md locked decisions
- Pitfalls: HIGH -- all pitfalls derived from actual code analysis and Phase 42 test coverage

**Research date:** 2026-04-06
**Valid until:** 2026-05-06 (stable codebase, no external dependencies)

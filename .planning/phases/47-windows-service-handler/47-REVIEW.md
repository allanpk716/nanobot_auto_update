---
phase: 47-windows-service-handler
reviewed: 2026-04-10T12:00:00Z
depth: standard
files_reviewed: 7
files_reviewed_list:
  - cmd/nanobot-auto-updater/main.go
  - internal/lifecycle/app.go
  - internal/lifecycle/app_test.go
  - internal/lifecycle/service.go
  - internal/lifecycle/service_handler_test.go
  - internal/lifecycle/service_windows.go
  - internal/lifecycle/starter.go
findings:
  critical: 0
  warning: 2
  info: 4
  total: 6
status: issues_found
---

# Phase 47: Code Review Report

**Reviewed:** 2026-04-10T12:00:00Z
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

Reviewed the Phase 47 Windows Service Handler implementation across 7 files. The implementation cleanly extracts AppComponents/AppStartup/AppShutdown from main.go into the lifecycle package (Plan 01), adds a ServiceHandler implementing svc.Handler (Plan 02), and wires service mode entry point in main.go. The factory callback pattern is well-designed to avoid circular imports.

The SCM state machine in service_windows.go correctly handles StartPending -> Running -> StopPending -> Stopped transitions, with proper support for Interrogate, Stop, and Shutdown commands. Build tags are correctly used for platform-specific files (service_windows.go vs service.go).

Two warnings found -- one is a latent design defect in AutoStartDone channel handling, and one is a potential nil pointer risk when NotifySender is nil. Neither is an active bug in current code paths.

## Warnings

### WR-01: AutoStartDone channel never closed when startInstances is nil

**File:** `internal/lifecycle/app.go:180,266-287`
**Issue:** The `AutoStartDone` channel is unconditionally created at line 180 (`make(chan struct{})`), but it is only closed inside the `if startInstances != nil` block at line 268. When `startInstances` is nil, the channel remains open forever. Any future caller that waits on `AutoStartDone` (e.g., `select { case <-components.AutoStartDone: }`) will block indefinitely. The field's doc comment states it "is closed when the auto-start goroutine completes," which is misleading when no goroutine is launched.

Currently no caller waits on this channel, so this is a latent defect rather than an active bug. However, the 47-01-SUMMARY.md notes "AutoStartDone channel available for ServiceHandler to wait on during shutdown," indicating planned usage that would trigger the hang.

**Fix:**
```go
// In AppStartup, after the auto-start block (after line 287):
if startInstances == nil {
    close(c.AutoStartDone)
}
```

Or restructure to close outside the goroutine when the goroutine is not launched.

### WR-02: Nil NotifySender passed to NotificationManager without guard

**File:** `internal/lifecycle/app.go:254-261`
**Issue:** When `notif` (the NotifySender parameter) is nil, it is passed directly to `notification.NewNotificationManager` at line 256. This occurs in the test path where `testServiceHandler` passes nil for all callbacks. Whether this causes a nil pointer dereference depends on the `NotificationManager` implementation (outside review scope). The `startInstances` callback at line 260 is nil-checked (`if startInstances != nil`), but `notif` has no similar guard before being used as a dependency.

**Fix:**
```go
// Add a nil guard before creating NotificationManager:
if notif != nil {
    c.NotificationManager = notification.NewNotificationManager(
        c.NetworkMonitor,
        notif,
        logger,
    )
    go c.NotificationManager.Start(cfg.Monitor.Interval)
    logger.Info("Notification manager started", "check_interval", cfg.Monitor.Interval)
}
```

## Info

### IN-01: Non-Windows NewServiceHandler silently discards all parameters

**File:** `internal/lifecycle/service.go:19-29`
**Issue:** The non-Windows stub `NewServiceHandler` accepts cfg, logger, version, and five other parameters but returns an empty `&ServiceHandler{}`. This is intentional (the stub should never be called in practice since `IsServiceMode()` returns false on non-Windows), but the function signature creates a false API contract. Consider adding a log warning or panic if this function is actually invoked, since it indicates a programming error.

**Fix:** No action required, but consider adding a log warning if the function is called:
```go
func NewServiceHandler(...) *ServiceHandler {
    logger.Warn("NewServiceHandler called on non-Windows platform -- this should not happen")
    return &ServiceHandler{}
}
```

### IN-02: Service mode detection log precedes config loading

**File:** `cmd/nanobot-auto-updater/main.go:66-71`
**Issue:** When `inService` is true, a message is logged to stderr (line 70) and then execution falls through to config loading (line 74), logger initialization (line 98), and finally the service mode branch at line 219. The early log is correct (slog is not yet initialized), but the flow is slightly confusing because service mode detection, config loading, and the actual service-mode branch are spread across 150+ lines. A reader might expect the service mode to branch immediately after detection. This is a readability concern, not a bug.

**Fix:** No action required. The existing code is correct -- config is needed for `RunService` parameters. Adding a brief comment at the service branch (line 219) referencing the detection at line 66 could help readability.

### IN-03: App tests are windows-only

**File:** `internal/lifecycle/app_test.go:1`
**Issue:** The test file has `//go:build windows` tag, which means the AppShutdown tests are never run on non-Windows development machines. While `AppShutdown` is platform-independent logic, the test file needs the tag because it imports lifecycle_test which requires windows-only build-tagged files in the same package. This is a minor test coverage gap for developers working on non-Windows platforms.

**Fix:** Consider splitting tests into platform-independent tests (AppShutdown tests) in a separate file without build tags, and windows-only tests in a tagged file.

### IN-04: Magic number for auto-start timeout

**File:** `internal/lifecycle/app.go:277`
**Issue:** The auto-start timeout `5 * time.Minute` and the cleanup context timeout `30 * time.Second` (line 187) are hardcoded magic numbers. These values should ideally be configurable or at minimum defined as named constants.

**Fix:**
```go
const (
    autoStartTimeout    = 5 * time.Minute
    rollbackTimeout     = 30 * time.Second
)
```

---

_Reviewed: 2026-04-10T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_

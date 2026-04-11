---
phase: 48-service-manager
reviewed: 2026-04-11T12:00:00Z
depth: standard
files_reviewed: 4
files_reviewed_list:
  - internal/lifecycle/servicemgr_windows.go
  - internal/lifecycle/servicemgr.go
  - internal/lifecycle/servicemgr_test.go
  - cmd/nanobot-auto-updater/main.go
findings:
  critical: 1
  warning: 4
  info: 3
  total: 8
status: issues_found
---

# Phase 48: Code Review Report

**Reviewed:** 2026-04-11T12:00:00Z
**Depth:** standard
**Files Reviewed:** 4
**Status:** issues_found

## Summary

Reviewed the Windows service manager implementation (registration, unregistration, admin detection) and its integration into the main entrypoint. The code is generally well-structured with proper build tags, idempotent operations, context-aware cancellation, and defensive validation. However, one security-relevant issue was found: the hardcoded service description in Chinese leaks into the compiled binary on non-Chinese locales and is not configurable, violating the project rule that development scripts must not contain Chinese characters. A privilege escalation risk exists in the unconditional auto-uninstall path. Several warning-level issues around type safety and error handling were also identified.

## Critical Issues

### CR-01: Unconditional Auto-Uninstall on Console Start Allows Privilege Abuse

**File:** `cmd/nanobot-auto-updater/main.go:152-165`
**Issue:** Case 3 unconditionally attempts `UnregisterService` whenever the app starts in console mode with `auto_start: false` (or nil). If a non-admin user runs the application, `UnregisterService` will fail at SCM connect -- but if a user has admin privileges (perhaps from another context or inherited elevation), the service gets silently uninstalled with no user confirmation. This means any user who can run the binary as admin will have the service removed just by having `auto_start: false` in config. There is no safeguard preventing accidental uninstall of a production service.

Furthermore, the `UnregisterService` function on non-Windows platforms (`servicemgr.go:34`) is a no-op that silently returns nil. On non-Windows, the code at line 160 will log "Service uninstalled, switched to console mode" even though nothing was actually uninstalled -- a misleading log message.

**Fix:**
```go
// main.go Case 3: Add explicit confirmation or only attempt uninstall
// when there is evidence the service was previously registered.
if !inService && (cfg.Service.AutoStart == nil || !*cfg.Service.AutoStart) {
    // Only attempt unregister on Windows where services exist
    if runtime.GOOS == "windows" {
        if err := lifecycle.UnregisterService(context.Background(), cfg, logger); err != nil {
            slog.Warn("Failed to unregister service", "error", err)
        } else {
            slog.Info("Service uninstalled, switched to console mode",
                "service_name", cfg.Service.ServiceName)
        }
    }
    // Continue to normal console mode execution
}
```

For the misleading log on non-Windows, the no-op `UnregisterService` in `servicemgr.go` should return a sentinel or the caller should skip the "uninstalled" message when the platform does not support services.

## Warnings

### WR-01: Hardcoded Chinese String in Service Description

**File:** `internal/lifecycle/servicemgr_windows.go:83`
**Issue:** The service description is hardcoded as a Chinese string `"自动保持 nanobot 处于最新版本"`. Per project rules in `CLAUDE.md`: "开发脚本禁止中文 -- 开发、编辑 BAT 脚本时不包含任何中文字符". While this is Go, not a BAT script, the principle applies to all development scripts. More importantly, this string should come from the config (`cfg.Service.Description`) rather than being hardcoded, which would allow localization and make the field configurable per deployment.

**Fix:**
```go
// Add Description field to ServiceConfig and use it:
svcHandle, err := scm.CreateService(
    m.cfg.Service.ServiceName,
    exePath,
    mgr.Config{
        StartType:        mgr.StartAutomatic,
        ErrorControl:     mgr.ErrorNormal,
        ServiceStartName: "LocalSystem",
        DisplayName:      m.cfg.Service.DisplayName,
        Description:      m.cfg.Service.Description,
    },
)
```

### WR-02: `any` Type Erasure in createComponents and startInstances Closures

**File:** `cmd/nanobot-auto-updater/main.go:187-243`
**Issue:** The `createComponents` and `startInstances` closures accept and return parameters typed as `any`, then immediately cast them back to concrete types (`*notifier.Notifier`, `*updatelog.UpdateLogger`, `*instance.InstanceManager`). This is done to break circular imports, but it sacrifices compile-time type safety. If a refactor changes one of these types, the cast will panic at runtime rather than failing at compile time. The `notif.(*notifier.Notifier)` cast at line 196 and 238 will panic if nil is passed.

**Fix:** This is an architectural trade-off for circular import avoidance. At minimum, add nil checks before casting:
```go
if notif == nil {
    return nil, nil, nil, fmt.Errorf("notif parameter must not be nil")
}
concreteNotif, ok := notif.(*notifier.Notifier)
if !ok {
    return nil, nil, nil, fmt.Errorf("notif parameter has wrong type: %T", notif)
}
```

### WR-03: Missing Nil Check on cfg.Service Before AutoStart Access

**File:** `cmd/nanobot-auto-updater/main.go:121,129,153`
**Issue:** The code accesses `cfg.Service.AutoStart` directly with nil-pointer checks (`cfg.Service.AutoStart == nil || !*cfg.Service.AutoStart`). This is correct for the `*bool` nil case, but if the `Config` struct's `Service` field were ever a pointer (currently it is a value type `ServiceConfig`), this would panic. Currently safe because `Service` is `ServiceConfig` (value type), but the defensive pattern is fragile. No action needed now, but worth noting for future refactoring.

### WR-04: UnregisterService Does Not Check for Empty ServiceName

**File:** `internal/lifecycle/servicemgr_windows.go:115`
**Issue:** `RegisterService` (line 46) validates that `ServiceName` is not empty, but `UnregisterService` has no such check. If called with an empty service name, it will attempt to open a service with an empty name from SCM, which may produce an unclear error from the Windows API rather than a clear validation error.

**Fix:**
```go
func (m *ServiceManager) UnregisterService(ctx context.Context) error {
    if m.cfg.Service.ServiceName == "" {
        return nil // Nothing to unregister if no service name configured
    }
    // ... rest of function
}
```

## Info

### IN-01: Exit Code 2 for Service Registration Is Undocumented

**File:** `cmd/nanobot-auto-updater/main.go:149`
**Issue:** `os.Exit(2)` is used when service registration succeeds, with a comment saying "signals to calling scripts". This exit code convention is not documented in `--help` output (lines 43-51) or in any visible user-facing documentation. Callers may not know what exit code 2 means.

**Fix:** Consider adding a note in `--help` output:
```go
fmt.Println("  Exit codes: 0=normal, 1=error, 2=service registered")
```

### IN-02: Non-Windows ServiceManager Stub Has Different Struct Shape

**File:** `internal/lifecycle/servicemgr.go:13`
**Issue:** The non-Windows `ServiceManager` struct is `struct{}` (empty), while the Windows version has `cfg` and `logger` fields. This is fine because build tags ensure only one is compiled, but `NewServiceManager` on non-Windows (`servicemgr.go:16`) accepts `cfg` and `logger` parameters and silently discards them. This is intentional (no-op stub) but could confuse future developers who expect the parameters to be used.

**Fix:** Consider adding a doc comment:
```go
// NewServiceManager returns a no-op ServiceManager on non-Windows platforms.
// The cfg and logger parameters are accepted for API compatibility but are not used.
```

### IN-03: Test File Uses Different Logger in One Test

**File:** `internal/lifecycle/servicemgr_test.go:68`
**Issue:** `TestRegisterService_NonAdminOrNonWindows` uses `slog.Default()` instead of the `testServiceMgrLogger()` helper used by other tests. This means if `slog.Default()` produces output, it will appear in test output rather than being discarded. Minor inconsistency.

**Fix:**
```go
err := lifecycle.RegisterService(cfg, testServiceMgrLogger())
```

---

_Reviewed: 2026-04-11T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_

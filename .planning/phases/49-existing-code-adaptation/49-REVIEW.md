---
phase: 49-existing-code-adaptation
reviewed: 2026-04-11T18:20:00Z
depth: standard
files_reviewed: 15
files_reviewed_list:
  - cmd/nanobot-auto-updater/main.go
  - internal/api/auth.go
  - internal/api/auth_test.go
  - internal/api/query_test.go
  - internal/api/selfupdate_handler.go
  - internal/api/selfupdate_handler_test.go
  - internal/api/server.go
  - internal/api/server_test.go
  - internal/api/trigger_test.go
  - internal/config/config.go
  - internal/config/hotreload.go
  - internal/lifecycle/daemon.go
  - internal/lifecycle/service.go
  - internal/lifecycle/service_handler_test.go
  - internal/lifecycle/service_windows.go
findings:
  critical: 0
  warning: 6
  info: 4
  total: 10
status: issues_found
---

# Phase 49: Code Review Report

**Reviewed:** 2026-04-11T18:20:00Z
**Depth:** standard
**Files Reviewed:** 15
**Status:** issues_found

## Summary

Reviewed 15 source files from the nanobot-auto-updater project at standard depth, focusing on concurrency safety, error handling, resource leaks, and Windows-specific issues as requested by the phase context.

The codebase demonstrates solid engineering practices: constant-time token comparison in auth, atomic status tracking in selfupdate_handler, mutex-serialized config reload with debounce, proper panic recovery in goroutines, and clean SCM state transitions in the Windows service handler.

Key concerns found: a data race on `currentBearerToken` (read by HTTP goroutines, written by hot-reload callback), a `defer` in a long-lived closure that will never fire until process exit, an ignored `os.Chdir` error in service mode, and a goroutine-leak risk if the SCM `Execute` function never reaches the event loop.

## Warnings

### WR-01: Data race on `currentBearerToken` variable

**File:** `cmd/nanobot-auto-updater/main.go:189,224,366`
**Issue:** The variable `currentBearerToken` (line 189) is read by the API server's `getToken` closure on every HTTP request (line 224: `func() string { return currentBearerToken }`) and written by the `OnBearerTokenChange` callback (line 366: `currentBearerToken = newCfg.API.BearerToken`) which runs in the hot-reload timer goroutine via `doReload`. There is no synchronization between these accesses. This is a data race detectable by `go test -race`.

The hot-reload `doReload` method holds `state.mu` but the HTTP handler closure does not acquire any lock when reading `currentBearerToken`.
**Fix:**
```go
// Use atomic.Value or sync.Mutex to protect currentBearerToken
var tokenMu sync.RWMutex
currentBearerToken := cfg.API.BearerToken

// In getToken closure:
func() string {
    tokenMu.RLock()
    defer tokenMu.RUnlock()
    return currentBearerToken
}

// In OnBearerTokenChange:
tokenMu.Lock()
currentBearerToken = newCfg.API.BearerToken
tokenMu.Unlock()
```
Alternative: use `atomic.Value` storing a string for lock-free reads.

### WR-02: `defer startCancel()` in long-lived closure never fires timely

**File:** `cmd/nanobot-auto-updater/main.go:381-382`
**Issue:** Inside the `OnInstancesChange` callback (a closure stored in `HotReloadCallbacks`), `defer startCancel()` is used. Since this closure is a long-lived function value stored in the callback struct, the `defer` will not execute until the closure function returns -- which only happens when the closure is garbage collected or the process exits. The `context.WithTimeout` already provides a 5-minute deadline, so the context will expire on its own. However, the `defer` is misleading and the `cancel` function (and its associated internal timer goroutine) will leak until the context expires naturally rather than being released immediately after `StartAllInstances` returns.
**Fix:**
```go
OnInstancesChange: func(newCfg *Config) {
    // ... stop old instances ...
    newIM := instance.NewInstanceManager(newCfg, logger, notif)
    hotReloadComponents.InstanceManager = newIM
    startCtx, startCancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer startCancel() // this is fine, but misleading in a closure
    newIM.StartAllInstances(startCtx)
    // Better: call startCancel() explicitly after StartAllInstances returns
},
```
Or remove `defer` and call `startCancel()` directly after `StartAllInstances`.

### WR-03: `os.Chdir` error silently ignored in service mode

**File:** `cmd/nanobot-auto-updater/main.go:80-84`
**Issue:** In service mode, `os.Chdir(exeDir)` is called to fix the working directory (ADPT-03), but the error return is silently discarded. If `Chdir` fails (e.g., directory deleted, permission denied), the process continues with `C:\Windows\System32` as the working directory. This will cause all subsequent relative path operations (`./config.yaml`, `./logs/`) to resolve against the wrong directory, likely causing cascading failures that are hard to diagnose.
**Fix:**
```go
if exePath, err := os.Executable(); err == nil {
    if exeDir := filepath.Dir(exePath); exeDir != "" {
        if chdirErr := os.Chdir(exeDir); chdirErr != nil {
            fmt.Fprintf(os.Stderr, "Warning: failed to change working directory to %s: %v\n", exeDir, chdirErr)
        }
    }
}
```
At minimum, log the error so it is diagnosable.

### WR-04: `restartAsDaemon` leaks log file handle

**File:** `internal/lifecycle/daemon.go:82-98`
**Issue:** In `restartAsDaemon`, the log file is opened at line 82 but never explicitly closed. The code calls `cmd.Start()` (line 91), then `cmd.Process.Release()` (line 96), and finally `os.Exit(0)` (line 101). While `os.Exit(0)` terminates the process and the OS will reclaim the file descriptor, the sequence is fragile: if `cmd.Process.Release()` fails (line 96-98), the function returns an error *without closing the log file*. This leaks a file handle in the error path. In practice this is a minor issue because the caller would then exit, but it is still good hygiene.
**Fix:**
```go
logFile, err := os.OpenFile("./logs/daemon.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
if err != nil {
    return false, fmt.Errorf("failed to create daemon log file: %w", err)
}
defer logFile.Close() // ensure cleanup on error paths
```
Note: `os.Exit(0)` at line 101 means the deferred close will NOT run on the happy path (os.Exit bypasses defers), but it will run if the function returns on error paths.

### WR-05: Hot-reload timer callback can deadlock if reload takes longer than debounce

**File:** `internal/config/hotreload.go:98-100`
**Issue:** The `time.AfterFunc` callback calls `state.doReload()` which acquires `state.mu` (line 110). The `OnConfigChange` callback (line 81-101) also acquires `state.mu`. If a config file change event arrives while `doReload` is running (holding the lock), the `OnConfigChange` callback will block waiting for the mutex. Since `OnConfigChange` is called from viper's fsnotify goroutine, this could delay or stack up file system events. This is not a true deadlock (the lock will eventually be released), but it means rapid config file changes could cause the fsnotify callback to block, potentially missing events or causing memory pressure if events queue up.

More importantly: the `time.AfterFunc` callback runs in its own goroutine, and `doReload` acquires the mutex. This is safe because the timer goroutine is separate from the fsnotify goroutine. The actual concern is that if a new config change arrives while `doReload` holds the lock, the debounce timer reset (line 96-97) is blocked until `doReload` completes. This means the debounce window is effectively extended.
**Fix:** This is an inherent trade-off of the mutex-serialized design. Consider documenting this behavior. If it becomes a problem, the `OnConfigChange` callback could use `TryLock` and skip the event if a reload is already in progress.

### WR-06: `viperInstance` global is not safe for concurrent access

**File:** `internal/config/config.go:150`
**Issue:** `viperInstance` is a package-level `*viper.Viper` variable that is written by `Load()` (line 180) and read by `GetViper()` (line 154) and `ReloadConfig()` (line 161). While in the current code flow `Load()` is called once during startup before any concurrent access, there is no synchronization protecting this variable. If `ReloadConfig` is called concurrently with a config reload (which happens in the hot-reload timer goroutine), both `ReloadConfig` calls operate on the same `viperInstance` without synchronization. Specifically, `viperInstance.ReadInConfig()` (line 164) and `viperInstance.Unmarshal()` (line 168) are called by `ReloadConfig` which runs inside `doReload`'s mutex, so this is serialized. However, the `WatchConfig` / `OnConfigChange` mechanism in viper also accesses the same viper instance internally from its own goroutine. This is a known limitation of viper's concurrent safety.
**Fix:** This is a known viper limitation. The current mutex in `hotReloadState` mitigates the most obvious race. Document this as a known constraint.

## Info

### IN-01: `restartAsDaemon` unreachable return after `os.Exit(0)`

**File:** `internal/lifecycle/daemon.go:100-103`
**Issue:** After `os.Exit(0)` on line 101, the return statement on line 102-103 (`return true, nil`) is unreachable code. This is a Go compiler-accepted pattern but is dead code.
**Fix:** Consider adding a comment `// unreachable` or restructuring to avoid the dead code.

### IN-02: `globalHotReload` package-global variable limits testability

**File:** `internal/config/hotreload.go:52`
**Issue:** `globalHotReload` is a package-level singleton. This means `WatchConfig` can only be called once per process, and `StopWatch()` sets it to nil with no way to restart. This limits testability -- tests cannot run multiple watch scenarios in the same process.
**Fix:** Consider making `hotReloadState` a struct that is created and managed by the caller, rather than using a package global. This is a low-priority design improvement.

### IN-03: Duplicate `IsServiceMode()` calls in daemon.go

**File:** `internal/lifecycle/daemon.go:19,47`
**Issue:** Both `MakeDaemon()` and `MakeDaemonSimple()` call `IsServiceMode()` independently. If these functions are called in sequence (which does not happen in the current code), the service mode check would be performed twice. The current code only calls one of these, so this is not a runtime issue.
**Fix:** No action needed. Just noting for awareness.

### IN-04: `onReady` callback runs inline in SCM Execute -- long callback blocks service

**File:** `internal/lifecycle/service_windows.go:88-89`
**Issue:** The `onReady` callback is called inline within the `Execute` method, before the event loop starts. If the callback performs slow operations (e.g., the current implementation starts goroutines and configures hot-reload which is fine), it would block the SCM state machine from processing Stop/Shutdown commands until it completes. The current callback starts goroutines and returns quickly, so this is not a problem in practice.
**Fix:** No action needed for the current implementation. Worth documenting the constraint that `onReady` must return quickly.

---

_Reviewed: 2026-04-11T18:20:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_

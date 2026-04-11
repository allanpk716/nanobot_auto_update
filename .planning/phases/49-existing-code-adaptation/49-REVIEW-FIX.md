---
phase: 49-existing-code-adaptation
fixed_at: 2026-04-11T11:26:46Z
review_path: .planning/phases/49-existing-code-adaptation/49-REVIEW.md
iteration: 1
findings_in_scope: 6
fixed: 4
skipped: 2
status: partial
---

# Phase 49: Code Review Fix Report

**Fixed at:** 2026-04-11T11:26:46Z
**Source review:** .planning/phases/49-existing-code-adaptation/49-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 6
- Fixed: 4
- Skipped: 2

## Fixed Issues

### WR-01: Data race on `currentBearerToken` variable

**Files modified:** `cmd/nanobot-auto-updater/main.go`
**Commit:** 23c4088
**Applied fix:** Added `sync.RWMutex` (`tokenMu`) to protect `currentBearerToken`. The `getToken` closure now acquires `tokenMu.RLock()`/`RUnlock()` when reading. The `OnBearerTokenChange` callback acquires `tokenMu.Lock()`/`Unlock()` when writing. Imported `sync` package.

### WR-02: `defer startCancel()` in long-lived closure never fires timely

**Files modified:** `cmd/nanobot-auto-updater/main.go`
**Commit:** 9becd4f
**Applied fix:** Replaced `defer startCancel()` with an explicit `startCancel()` call after `newIM.StartAllInstances(startCtx)` returns. This ensures the context cancel function and its internal timer goroutine are released immediately, rather than leaking until the long-lived `OnInstancesChange` closure is garbage collected.

### WR-03: `os.Chdir` error silently ignored in service mode

**Files modified:** `cmd/nanobot-auto-updater/main.go`
**Commit:** 23b0282
**Applied fix:** Changed bare `os.Chdir(exeDir)` to check the error return. If `Chdir` fails, logs a warning to stderr with the directory path and error message, so the issue is diagnosable.

### WR-04: `restartAsDaemon` leaks log file handle

**Files modified:** `internal/lifecycle/daemon.go`
**Commit:** 3aeb110
**Applied fix:** Added `defer logFile.Close()` immediately after the log file is successfully opened. While `os.Exit(0)` at line 102 bypasses defers on the happy path, this protects the error paths where `cmd.Start()` or `cmd.Process.Release()` fail and the function returns an error.

## Skipped Issues

### WR-05: Hot-reload timer callback can deadlock if reload takes longer than debounce

**File:** `internal/config/hotreload.go:98-100`
**Reason:** Documentation-only finding. Reviewer recommends documenting the behavior rather than changing code. The mutex-serialized design is an inherent trade-off and the current implementation works correctly for expected usage patterns.
**Original issue:** The debounce timer reset is blocked while `doReload` holds the mutex, extending the debounce window during rapid config changes.

### WR-06: `viperInstance` global is not safe for concurrent access

**File:** `internal/config/config.go:150`
**Reason:** Documentation-only finding. This is a known viper limitation. The current `hotReloadState` mutex already mitigates the most obvious race. Reviewer recommends documenting as a known constraint rather than changing code.
**Original issue:** `viperInstance` package-level variable lacks synchronization, but `WatchConfig`/`OnConfigChange` is a known viper concurrent safety limitation.

---

_Fixed: 2026-04-11T11:26:46Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_

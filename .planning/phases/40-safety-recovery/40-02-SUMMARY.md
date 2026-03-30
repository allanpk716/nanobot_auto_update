---
phase: 40-safety-recovery
plan: 02
subsystem: lifecycle
tags: [recovery, port-retry, self-update, startup, windows]

# Dependency graph
requires:
  - phase: 40-01
    provides: "SelfUpdateHandler with notif parameter, self-spawn pattern"
provides:
  - "CheckUpdateState function for .old cleanup/recovery on startup"
  - "ListenWithRetry for port binding with retry after self-update restart"
  - "checkUpdateStateInternal for testable state detection logic"
affects: [main-startup, server-start, self-update-recovery]

# Tech tracking
tech-stack:
  added: []
  patterns: ["internal/external split for testable startup logic", "net.Listen+Serve replacing ListenAndServe for retry capability"]

key-files:
  created:
    - internal/lifecycle/update_state.go
    - internal/lifecycle/update_state_test.go
  modified:
    - cmd/nanobot-auto-updater/main.go
    - internal/api/server.go
    - internal/api/server_test.go

key-decisions:
  - "CheckUpdateState placed after logger creation but before server startup (needs logger, must precede network operations)"
  - "checkUpdateStateInternal internal function for testability without os.Exit side effects"
  - "Corrupt .update-success marker treated as missing (falls through to recovery check)"
  - "Empty .old file (size 0) NOT treated as needing recovery"
  - "net.Listen + http.Serve replaces ListenAndServe to enable port binding retry"

patterns-established:
  - "Internal/external function split: checkUpdateStateInternal returns decision string, CheckUpdateStateForPath executes it (testable without os.Exit)"

requirements-completed: [SAFE-03, SAFE-04]

# Metrics
duration: 21min
completed: 2026-03-30
---

# Phase 40 Plan 02: Startup Cleanup/Recovery and Port Retry Summary

Startup .old backup cleanup/recovery and port binding retry mechanism -- CheckUpdateState cleans up after successful updates, recovers old version on crash, and ListenWithRetry handles port binding race during restart.

## Performance

- **Duration:** 21 min
- **Started:** 2026-03-30T12:17:18Z
- **Completed:** 2026-03-30T12:38:26Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

### Task 1: Create update_state.go with CheckUpdateState and ListenWithRetry

Created `internal/lifecycle/update_state.go` with `//go:build windows` constraint containing:

- `checkUpdateStateInternal(exePath, logger) string` -- internal decision function returning "cleanup", "recover", or "normal"
- `CheckUpdateStateForPath(exePath, logger)` -- executes recovery: os.Rename old backup, self-spawn with full daemon.go flags, os.Exit
- `CheckUpdateState(logger)` -- convenience wrapper getting exe path automatically
- `ListenWithRetry(addr, logger) (net.Listener, error)` -- 5 retries at 500ms intervals per D-05

Key behaviors:
- `.update-success` with valid JSON triggers cleanup (removes .old and marker)
- `.old` exists without `.update-success` triggers recovery (Rename + self-spawn + Exit)
- Corrupt `.update-success` treated as missing (falls through to recovery check)
- Empty `.old` file (size 0) treated as normal (no recovery)
- Recovery self-spawn uses full daemon.go flags: `CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP | DETACHED_PROCESS`

Created `internal/lifecycle/update_state_test.go` with 7 tests:
- TestCheckUpdateStateInternal_Cleanup, Recover, Normal, CorruptMarker, EmptyOldFile
- TestListenWithRetry_Success, AfterClose

All 7 tests pass.

**Commit:** d33275b

### Task 2: Integrate CheckUpdateState into main.go and ListenWithRetry into Server.Start()

Modified `cmd/nanobot-auto-updater/main.go`:
- Added `internal/lifecycle` import
- Added `lifecycle.CheckUpdateState(logger)` call at line 86, right after `slog.SetDefault(logger)` and before server startup

Modified `internal/api/server.go`:
- Added `internal/lifecycle` import
- Replaced `ListenAndServe()` with `lifecycle.ListenWithRetry()` + `Serve(listener)` in Start() method
- Preserved Plan 40-01's `NewSelfUpdateHandler` constructor call with `notif` parameter

Added `TestServerStart_PortRetry` to `internal/api/server_test.go` verifying the new start behavior.

Build succeeds. All server tests pass (6 tests including the new one).

**Commit:** 44826ad

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Pre-existing capture_test.go compilation error**
- **Found during:** Task 1 test execution
- **Issue:** `internal/lifecycle/capture_test.go` has type mismatch (strings.Reader used as *os.File) -- pre-existing, unrelated to current plan
- **Fix:** Temporarily moved capture_test.go aside to run update_state tests, then restored. Documented as out-of-scope pre-existing issue.
- **Files modified:** None (temporary workaround for test run only)

**2. [Rule 3 - Blocking] Pre-existing TestE2E_Notification_NonBlocking timeout**
- **Found during:** Task 2 full API test suite run
- **Issue:** `TestE2E_Notification_NonBlocking` uses 30s time.Sleep causing timeout under 30s test timeout -- pre-existing, unrelated to current plan
- **Fix:** Ran targeted tests excluding this slow E2E test. All relevant tests pass.
- **Files modified:** None (test selection strategy)

None of the plan's code changes introduced any issues -- all deviations are pre-existing.

## Verification Results

1. `go build ./cmd/nanobot-auto-updater/` -- PASS (compiles without errors)
2. `internal/lifecycle/update_state_test.go` -- PASS (7/7 tests)
3. `internal/api/server_test.go` -- PASS (6/6 targeted tests including new port retry test)
4. `grep CheckUpdateState main.go` -- PASS (line 86: `lifecycle.CheckUpdateState(logger)`)
5. `grep ListenWithRetry server.go` -- PASS (Start method uses it)
6. `grep CREATE_NEW_PROCESS_GROUP update_state.go` -- PASS (full daemon.go flags confirmed)

## Deferred Issues

- `internal/lifecycle/capture_test.go` has pre-existing compilation error (strings.Reader vs *os.File type mismatch) -- out of scope
- `internal/api/integration_test.go` TestE2E_Notification_NonBlocking has pre-existing 30s timeout issue -- out of scope

## Key Decisions

1. **CheckUpdateState placement**: After logger creation (needs slog.Logger), before server startup (D-04 requirement). Cannot be before config loading since logger comes from config path.
2. **Internal/external split**: `checkUpdateStateInternal` returns decision string for testability without os.Exit side effects, `CheckUpdateStateForPath` handles actual recovery and process exit.
3. **Corrupt marker handling**: Invalid JSON in .update-success treated as missing, falling through to .old recovery check. This is conservative -- better to attempt recovery than lose a backup.
4. **Empty .old guard**: Files with size 0 are not treated as needing recovery. This prevents false positives from empty/corrupt .old files.

## Self-Check: PASSED

- [x] internal/lifecycle/update_state.go -- FOUND
- [x] internal/lifecycle/update_state_test.go -- FOUND
- [x] cmd/nanobot-auto-updater/main.go -- FOUND
- [x] internal/api/server.go -- FOUND
- [x] internal/api/server_test.go -- FOUND
- [x] .planning/phases/40-safety-recovery/40-02-SUMMARY.md -- FOUND
- [x] d33275b (Task 1 commit) -- FOUND
- [x] 44826ad (Task 2 commit) -- FOUND

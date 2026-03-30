---
phase: 40-safety-recovery
plan: 01
subsystem: api
tags: [self-update, pushover, notifications, self-spawn, restart, status-file]

# Dependency graph
requires:
  - phase: 39
    provides: "SelfUpdateHandler, SelfUpdateChecker interface, UpdateMutex interface, HandleCheck/HandleUpdate methods"
  - phase: 34
    provides: "Notifier interface (duck typing, Notify(title, message) pattern)"
  - phase: 38
    provides: "selfupdate.Updater with NeedUpdate/Update, ReleaseInfo struct"
provides:
  - "SelfUpdateHandler with Notifier injection for self-update lifecycle notifications"
  - "Start/complete/failure notification sends in SelfUpdateHandler"
  - ".update-success status file writing after successful Apply"
  - "Self-spawn restart via restartFn (defaultRestartFn with daemon.go flags)"
  - "mockNotifier for testing notification behavior"
affects: [40-02]

# Tech tracking
tech-stack:
  added: [golang.org/x/sys/windows]
  patterns: [restartFn-injection for testable os.Exit paths]

key-files:
  created: []
  modified:
    - internal/api/selfupdate_handler.go
    - internal/api/selfupdate_handler_test.go
    - internal/api/server.go

key-decisions:
  - "restartFn field on SelfUpdateHandler for testable self-spawn (defaultRestartFn in production, no-op in tests)"
  - "Completion notification synchronous before os.Exit to avoid Pitfall 1 (goroutine killed by os.Exit)"
  - "Start and failure notifications async goroutine (non-blocking, panic recovery)"

patterns-established:
  - "restartFn injection: override os.Exit+exec.Command in tests via struct field, production default in constructor"
  - "Three notification points: start (async), completion (sync before exit), failure (async)"

requirements-completed: [SAFE-01, SAFE-02]

# Metrics
duration: 23min
completed: 2026-03-30
---

# Phase 40 Plan 01: Self-Update Notifications and Restart Summary

**Notifier injection with start/complete/failure Pushover notifications, .update-success status file, and testable self-spawn restart in SelfUpdateHandler**

## Performance

- **Duration:** 23 min
- **Started:** 2026-03-30T11:14:20Z
- **Completed:** 2026-03-30T11:37:12Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- SelfUpdateHandler sends async start notification before update, sync completion notification after success (avoids Pitfall 1), and async failure notification on error or panic
- Writes .update-success status file (JSON with timestamp, old_version, new_version) after Apply succeeds for startup cleanup logic in Plan 02
- Self-spawn restart via restartFn injection pattern -- defaultRestartFn uses exec.Command + os.Exit(0) with full daemon.go flags (CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP | DETACHED_PROCESS)
- All 44 API tests pass including 3 new notification tests (StartNotification, FailureNotification, NilNotifier)

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Notifier injection and notification + self-spawn + status file to SelfUpdateHandler** - `e7067de` (feat)
2. **Task 2: Update existing tests and add notification/restart tests** - `94f627b` (test)

## Files Created/Modified
- `internal/api/selfupdate_handler.go` - Added Notifier field, restartFn field, notifications (start/complete/failure), status file writing, self-spawn via defaultRestartFn
- `internal/api/selfupdate_handler_test.go` - Added mockNotifier, updated all tests for new Notifier parameter, added StartNotification/FailureNotification/NilNotifier tests
- `internal/api/server.go` - Updated NewSelfUpdateHandler call to pass notif parameter

## Decisions Made
- **restartFn injection**: Added `restartFn func(exePath string)` field to SelfUpdateHandler. Production uses `defaultRestartFn` (exec.Command + os.Exit(0)). Tests override with no-op. This prevents test process termination and spawned child processes that cause infinite test loops. The self-spawn pattern itself was validated by Phase 36 PoC.
- **Sync vs Async notifications**: Completion notification is synchronous (before os.Exit) because goroutines are killed by os.Exit before they run (Pitfall 1 from RESEARCH). Start and failure notifications are async goroutines (non-blocking, with panic recovery).
- **Status file location**: `exePath + ".update-success"` -- same directory as executable, easy to find during startup cleanup.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Extracted restartFn for testability**
- **Found during:** Task 2 (running tests)
- **Issue:** Tests with `updateErr: nil` triggered real `exec.Command(exePath)` + `os.Exit(0)`, spawning child test processes that re-ran tests in an infinite loop. Go test runner catches os.Exit as panic but the child process was already started.
- **Fix:** Extracted self-spawn logic into `restartFn func(exePath string)` field on SelfUpdateHandler. Constructor sets `defaultRestartFn` (production). Tests override with no-op function. All tests pass without spawning child processes.
- **Files modified:** internal/api/selfupdate_handler.go, internal/api/selfupdate_handler_test.go
- **Verification:** All 44 API tests pass (go test ./internal/api/ -v -count=1 -timeout 60s)
- **Committed in:** 94f627b (Task 2 commit)

**2. [Rule 3 - Blocking] Updated two missed test call sites**
- **Found during:** Task 2 (first test run)
- **Issue:** Two test functions (TestSelfUpdateUpdate_PanicRecovery, TestSelfUpdateCheck_StatusDuringUpdate) used panicSelfUpdateChecker and slowSelfUpdateChecker which were not updated by the `replace_all` for newTestSelfUpdateHandler calls.
- **Fix:** Added nil Notifier parameter to both remaining call sites.
- **Files modified:** internal/api/selfupdate_handler_test.go
- **Verification:** Compilation succeeds, all tests pass
- **Committed in:** 94f627b (part of Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 blocking)
**Impact on plan:** restartFn injection is a necessary testability improvement that makes the self-spawn path testable without modifying production behavior. No scope creep.

## Issues Encountered
- None beyond the deviations documented above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- SelfUpdateHandler fully integrated with Notifier, notifications, status file, and restart logic
- Plan 02 can now implement startup .old cleanup/recovery and port binding retry using the .update-success marker
- SAFE-01 (restart) and SAFE-02 (notifications) requirements are satisfied
- SAFE-03 and SAFE-04 remain for Plan 02

---
*Phase: 40-safety-recovery*
*Completed: 2026-03-30*

## Self-Check: PASSED

- All 3 modified files exist in working tree
- Both task commits (e7067de, 94f627b) found in git log
- All 44 API tests pass
- Build compiles without errors

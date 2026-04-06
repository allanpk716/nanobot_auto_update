---
phase: 43-telegram-monitor-integration
plan: 01
subsystem: instance-lifecycle
tags: [telegram, monitor, notifier, goroutine, context-cancellation]

# Dependency graph
requires:
  - phase: 42-telegram-monitor-core
    provides: "TelegramMonitor, Notifier interface, DefaultTimeout, pattern detection"
provides:
  - "InstanceLifecycle with Notifier injection and TelegramMonitor lifecycle management"
  - "startTelegramMonitor/stopTelegramMonitor helper methods"
  - "Notifier constructor chain: main.go -> NewInstanceManager -> NewInstanceLifecycle"
affects: [43-02, instance-lifecycle, instance-manager, main-startup]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Duck-typing Notifier interface per-package (instance package local copy)"
    - "Independent context.WithCancel(context.Background()) for monitor goroutine"
    - "Panic recovery in monitor goroutine"
    - "Double cancellation: monitor.Stop() + monitorCancel() for clean goroutine exit"

key-files:
  created: []
  modified:
    - "internal/instance/lifecycle.go"
    - "internal/instance/manager.go"
    - "cmd/nanobot-auto-updater/main.go"
    - "internal/instance/lifecycle_test.go"
    - "internal/instance/manager_test.go"

key-decisions:
  - "Local Notifier interface in instance package (duck typing, avoids cross-package import)"
  - "Independent context.WithCancel(context.Background()) per monitor (not tied to instance ctx)"
  - "Monitor created as last step in StartAfterUpdate to avoid leak on early error"
  - "main.go creation order: notif before instanceManager (D-05)"

patterns-established:
  - "Constructor injection chain for Notifier: main -> manager -> lifecycle"
  - "Monitor lifecycle: start after process, stop before process"
  - "Nil-guard double-stop prevention in stopTelegramMonitor"

requirements-completed: [TELE-07, TELE-09]

# Metrics
duration: 10min
completed: 2026-04-06
---

# Phase 43 Plan 01: Telegram Monitor Integration Summary

**TelegramMonitor wired into InstanceLifecycle with Notifier injection, independent context management, and clean goroutine lifecycle**

## Performance

- **Duration:** 10 min
- **Started:** 2026-04-06T11:55:28Z
- **Completed:** 2026-04-06T12:05:39Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- InstanceLifecycle now creates and starts a TelegramMonitor goroutine after each successful process start
- StopForUpdate cleanly cancels monitor (Stop + CancelFunc + nil guards) before stopping the process
- Notifier injection chain established: main.go creates notif, passes to NewInstanceManager, which passes to each NewInstanceLifecycle

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Notifier parameter and monitor fields to InstanceLifecycle** - `6b83d52` (feat)
2. **Task 2: Update NewInstanceManager and main.go to pass Notifier through** - `b278c84` (feat)

## Files Created/Modified
- `internal/instance/lifecycle.go` - Notifier interface, telegramMonitor/monitorCancel fields, startTelegramMonitor/stopTelegramMonitor methods, constructor updated to 3 params
- `internal/instance/manager.go` - NewInstanceManager accepts Notifier, passes to NewInstanceLifecycle
- `cmd/nanobot-auto-updater/main.go` - Creation order fixed (notif before instanceManager), passes notif to NewInstanceManager
- `internal/instance/lifecycle_test.go` - mockNotifier added, all calls updated to 3-param NewInstanceLifecycle
- `internal/instance/manager_test.go` - All calls updated to 3-param NewInstanceManager

## Decisions Made
- Local Notifier interface in instance package (duck typing pattern, same as telegram package) avoids importing notifier package directly
- Independent context.WithCancel(context.Background()) for each monitor goroutine (not tied to instance context) prevents premature cancellation
- Monitor created as last step in StartAfterUpdate to avoid goroutine leak if process start fails
- main.go creation order: notif before instanceManager ensures notifier is available at construction time

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated existing tests for new 3-param constructors**
- **Found during:** Task 2 (verification step)
- **Issue:** lifecycle_test.go and manager_test.go had compilation errors due to NewInstanceLifecycle and NewInstanceManager signature changes
- **Fix:** Added mockNotifier type to lifecycle_test.go, replaced all 2-param calls with 3-param calls using newTestNotifier()
- **Files modified:** internal/instance/lifecycle_test.go, internal/instance/manager_test.go
- **Verification:** go test ./internal/instance/... passes, go build ./... succeeds
- **Committed in:** b278c84 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Auto-fix was necessary for test compilation after constructor signature change. No scope creep.

## Issues Encountered
None beyond the test update deviation documented above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- TELE-07 (no trigger = zero overhead) delivered: monitor only activates when "Starting Telegram bot" appears in logs
- TELE-09 (stop cancels monitor) delivered: StopForUpdate calls stopTelegramMonitor before process stop
- Phase 43-02 can proceed with end-to-end verification and any remaining integration work

---
*Phase: 43-telegram-monitor-integration*
*Completed: 2026-04-06*

## Self-Check: PASSED
- internal/instance/lifecycle.go: FOUND
- internal/instance/manager.go: FOUND
- cmd/nanobot-auto-updater/main.go: FOUND
- Task 1 commit 6b83d52: FOUND
- Task 2 commit b278c84: FOUND

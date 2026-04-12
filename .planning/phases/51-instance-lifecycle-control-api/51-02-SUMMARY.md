---
phase: 51-instance-lifecycle-control-api
plan: 02
subsystem: testing
tags: [http, lifecycle, auth, concurrency, rest, unit-test, testify]

# Dependency graph
requires:
  - phase: 51-instance-lifecycle-control-api/01
    provides: "InstanceLifecycleHandler with HandleStart/HandleStop, auth middleware, TryLockUpdate guard"
provides:
  - "Comprehensive test coverage for lifecycle handler (12 test functions)"
  - "SetPIDForTest helper in instance package for running-state injection"
  - "lifecycleMockNotifier for instance.Notifier interface in api test package"
affects: ["phase-53-ui"]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "SetPIDForTest helper: inject PID into InstanceLifecycle for testing already-running code path"
    - "cmd /c REM trick: embed --port in comment to prevent auto-append by containsPortFlag"

key-files:
  created:
    - internal/instance/lifecycle_test_helper.go
    - internal/api/instance_lifecycle_handler_test.go
  modified: []

key-decisions:
  - "SetPIDForTest as production-code method (not _test.go) because api package tests need cross-package access"
  - "cmd /c with REM trick to embed --port in comment for success-path tests (prevents StartCommand --port auto-append)"
  - "lifecycleMockNotifier name to avoid collision with existing mockNotifier in selfupdate_handler_test.go"
  - "Direct handler call (bypass ServeMux) for empty-name tests because ServeMux redirects double-slash paths"

patterns-established:
  - "Cross-package test helper: production-code method with ForTest suffix in instance package, called from api test package"
  - "Mock naming collision avoidance: prefix with package context (lifecycleMockNotifier) when same package has multiple mock types"

requirements-completed: [LC-01, LC-02, LC-03]

# Metrics
duration: 12min
completed: 2026-04-12
---

# Phase 51 Plan 02: Lifecycle Handler Tests Summary

**12 comprehensive tests covering lifecycle handler success, error, auth, and concurrency scenarios with SetPIDForTest helper for already-running state injection**

## Performance

- **Duration:** 12 min
- **Started:** 2026-04-12T02:13:07Z
- **Completed:** 2026-04-12T02:25:31Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Created SetPIDForTest helper enabling cross-package test state injection for already-running scenarios (resolves review HIGH-3)
- Wrote 12 test functions covering all lifecycle handler behaviors: success paths, error paths, auth rejection, and update-in-progress concurrency guard
- All tests pass with no regressions in api and instance packages

## Task Commits

Each task was committed atomically:

1. **Task 1: Create SetPIDForTest helper in instance package** - `7fc2d0b` (feat)
2. **Task 2: Write comprehensive lifecycle handler tests** - `f034af0` (test)

## Files Created/Modified
- `internal/instance/lifecycle_test_helper.go` - Test-only method to inject PID into InstanceLifecycle for running-state simulation
- `internal/api/instance_lifecycle_handler_test.go` - 12 test functions covering all handler behaviors including success, error, auth, and concurrency

## Decisions Made
- Used production-code file (not `_test.go`) for SetPIDForTest because `api` package tests need cross-package access to inject running state into `InstanceLifecycle`
- Used `cmd /c "ping -n 30 127.0.0.1 & rem --port 18790"` as StartCommand to satisfy `containsPortFlag()` while keeping the ping process alive for success-path tests
- Named mock `lifecycleMockNotifier` to avoid collision with existing `mockNotifier` in `selfupdate_handler_test.go`
- Called handlers directly (bypassing ServeMux) for empty-name tests because Go ServeMux redirects double-slash paths instead of routing to the handler

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Renamed mockNotifier to lifecycleMockNotifier**
- **Found during:** Task 2 (writing tests)
- **Issue:** `mockNotifier` type already declared in `selfupdate_handler_test.go` within same `api` package
- **Fix:** Renamed to `lifecycleMockNotifier` to avoid compilation error
- **Files modified:** `internal/api/instance_lifecycle_handler_test.go`
- **Verification:** `go test ./internal/api/... -count=1` passes
- **Committed in:** f034af0 (Task 2 commit)

**2. [Rule 3 - Blocking] Used cmd /c REM trick for --port auto-append prevention**
- **Found during:** Task 2 (success-path tests)
- **Issue:** `ping -n 30 127.0.0.1` fails because `StartNanobotWithCapture` auto-appends `--port 18790`, causing `ping` to fail with unrecognized option
- **Fix:** Embedded `--port` in a REM comment: `cmd /c "ping -n 30 127.0.0.1 & rem --port 18790"` so `containsPortFlag()` returns true and no port is appended
- **Files modified:** `internal/api/instance_lifecycle_handler_test.go`
- **Verification:** TestHandleStart_Success and TestHandleStop_Success pass
- **Committed in:** f034af0 (Task 2 commit)

**3. [Rule 3 - Blocking] Direct handler call for empty-name tests**
- **Found during:** Task 2 (empty-name tests)
- **Issue:** Go ServeMux redirects `//` paths with 301 instead of routing to handler, so empty-name test never reaches handler code
- **Fix:** Call `handler.HandleStart(rec, req)` directly instead of via `mux.ServeHTTP(rec, req)`
- **Files modified:** `internal/api/instance_lifecycle_handler_test.go`
- **Verification:** TestHandleStart_EmptyName and TestHandleStop_EmptyName pass with 400 status
- **Committed in:** f034af0 (Task 2 commit)

---

**Total deviations:** 3 auto-fixed (3 blocking)
**Impact on plan:** All auto-fixes were necessary to unblock test execution. No scope creep.

## Issues Encountered
None beyond the blocking issues documented in deviations.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All 12 lifecycle handler tests pass, verifying LC-01, LC-02, LC-03 requirements
- Test infrastructure (SetPIDForTest, lifecycleMockNotifier, setupLifecycleTest) ready for future test expansion
- Phase 53 UI can consume lifecycle endpoints with confidence that all success/error/auth/concurrency paths are verified

---
*Phase: 51-instance-lifecycle-control-api*
*Completed: 2026-04-12*

## Self-Check: PASSED

- [x] `internal/instance/lifecycle_test_helper.go` exists
- [x] `internal/api/instance_lifecycle_handler_test.go` exists
- [x] Commit `7fc2d0b` exists in git log
- [x] Commit `f034af0` exists in git log

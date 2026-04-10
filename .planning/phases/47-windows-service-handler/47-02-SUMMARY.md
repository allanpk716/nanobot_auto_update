---
phase: 47-windows-service-handler
plan: 02
subsystem: infra
tags: [windows-service, svc-handler, scm, build-tags, golang.org/x/sys/windows/svc]

# Dependency graph
requires:
  - phase: 47-01
    provides: AppComponents, AppStartup with factory callbacks, AppShutdown, servicedetect build tags
provides:
  - ServiceHandler struct implementing svc.Handler with Execute method
  - RunService wrapper function (Windows: svc.Run, non-Windows: error stub)
  - main.go service mode entry point calling lifecycle.RunService()
  - Service handler tests (Stop, Shutdown, Interrogate)
affects: [phase-48-service-registration]

# Tech tracking
tech-stack:
  added: [golang.org/x/sys/windows/svc (already in go.mod, first direct use in lifecycle)]
  patterns: [svc.Handler interface, build-tagged platform stubs, channel-based state machine testing]

key-files:
  created:
    - internal/lifecycle/service_windows.go
    - internal/lifecycle/service.go
    - internal/lifecycle/service_handler_test.go
  modified:
    - cmd/nanobot-auto-updater/main.go

key-decisions:
  - "ServiceHandler receives all AppStartup parameters (factory callbacks) to delegate to AppStartup inside Execute"
  - "Nil callbacks in tests skip component creation, allowing isolated state machine testing"
  - "30-second shutdown timeout in service mode vs 10-second in console mode"

patterns-established:
  - "Channel-based SCM testing: hand-rolled reqCh/statusCh manipulation for precise state transition verification"
  - "Build-tagged service implementations: service_windows.go for real, service.go for non-Windows stub"

requirements-completed: [SVC-02, SVC-03]

# Metrics
duration: 8min
completed: 2026-04-10
---

# Phase 47 Plan 02: ServiceHandler Implementation Summary

**ServiceHandler implementing svc.Handler with Execute state machine (StartPending->Running->StopPending->Stopped), RunService wrapper, and main.go service mode wiring using factory callback pattern**

## Performance

- **Duration:** 8 min
- **Started:** 2026-04-10T15:15:32Z
- **Completed:** 2026-04-10T15:23:12Z
- **Tasks:** 1 completed (Task 2 is human-verify checkpoint)
- **Files modified:** 4

## Accomplishments
- ServiceHandler struct with Execute method implementing svc.Handler interface
- Execute manages full SCM state machine: StartPending, Running, StopPending, Stopped
- Execute handles svc.Interrogate (echo status), svc.Stop and svc.Shutdown (graceful 30s shutdown)
- On AppStartup failure: reports Stopped, returns (true, 1) service-specific error
- RunService wrapper: svc.Run on Windows, descriptive error on non-Windows
- main.go service mode branch calls lifecycle.RunService() with all factory callback parameters
- Console mode behavior completely unchanged
- 3 unit tests passing: TestServiceHandler_Stop, TestServiceHandler_Shutdown, TestServiceHandler_Interrogate

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement ServiceHandler with Execute method, RunService wrapper, and main.go service mode branch** - `640882a` (feat)

**Task 2 is a checkpoint:human-verify -- awaiting human verification.**

## Files Created/Modified
- `internal/lifecycle/service_windows.go` - ServiceHandler struct, Execute method, RunService wrapper (Windows implementation)
- `internal/lifecycle/service.go` - Non-Windows stub for ServiceHandler and RunService
- `internal/lifecycle/service_handler_test.go` - Unit tests for ServiceHandler state transitions
- `cmd/nanobot-auto-updater/main.go` - Added service mode branch calling lifecycle.RunService()

## Decisions Made
- **ServiceHandler carries all AppStartup parameters**: The plan originally assumed a simple 3-parameter AppStartup, but Plan 01 used factory callbacks (7 parameters). ServiceHandler stores all parameters and passes them through to AppStartup inside Execute. This keeps the service handler thin and delegates all real work to AppStartup.
- **Nil callbacks for isolated testing**: Tests pass nil for createComponents and startInstances, causing AppStartup to skip those steps. This allows testing the SCM state machine without real component dependencies.
- **30s vs 10s shutdown timeout**: Service mode uses 30-second shutdown timeout (matching SCM expectations), while console mode retains its 10-second timeout.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated NewServiceHandler signature to match actual AppStartup interface**
- **Found during:** Task 1 (test compilation)
- **Issue:** Plan assumed 3-parameter AppStartup(cfg, logger, version), but Plan 01 actually implemented 7-parameter AppStartup with factory callbacks (createComponents, startInstances). Tests failed with "not enough arguments".
- **Fix:** Updated ServiceHandler to store all 7 AppStartup parameters (cfg, logger, version, updateLogger, notif, createComponents, startInstances) and pass them through in Execute. Updated RunService signature similarly. Non-Windows stub matches the same signature.
- **Files modified:** service_windows.go, service.go, service_handler_test.go
- **Verification:** All tests pass, build succeeds
- **Committed in:** 640882a (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking - interface mismatch with Plan 01)
**Impact on plan:** Necessary adaptation. Plan was written before Plan 01 chose factory callback pattern. No scope creep.

## Issues Encountered
- None beyond the documented interface mismatch

## User Setup Required
None - no external service configuration required.

## Self-Check: PASSED

- internal/lifecycle/service_windows.go: FOUND
- internal/lifecycle/service.go: FOUND
- internal/lifecycle/service_handler_test.go: FOUND
- cmd/nanobot-auto-updater/main.go: FOUND
- Commit 640882a: FOUND

## Next Phase Readiness
- Phase 48 (Service Registration) can now implement actual SCM registration using the ServiceHandler and RunService infrastructure
- Console mode is fully functional and unchanged
- Service mode entry point is wired and tested via unit tests
- Actual SCM testing (sc.exe create/start/stop) is Phase 48 scope

---
*Phase: 47-windows-service-handler*
*Completed: 2026-04-10*

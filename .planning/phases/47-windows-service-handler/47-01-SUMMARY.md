---
phase: 47-windows-service-handler
plan: 01
subsystem: lifecycle
tags: [app-lifecycle, shutdown, startup, rollback, components, interfaces]

# Dependency graph
requires:
  - phase: 46-service-configuration-mode-detection
    provides: "IsServiceMode() function, ServiceConfig, main.go service mode branching"
provides:
  - "AppComponents struct holding all 9 component references + AutoStartDone channel"
  - "AppShutdown(ctx, components, logger) ordered shutdown with nil-safety"
  - "AppStartup(cfg, logger, version, ...) component initialization with rollback on error"
  - "Factory callback pattern (CreateComponentsFunc, StartInstancesFunc) for circular import decoupling"
  - "Decoupling interfaces: APIServerControl, HealthMonitorControl, NotifySender, LogScheduler"
affects: [phase-47-plan-02, phase-48, service-handler, console-mode, main.go]

# Tech tracking
tech-stack:
  added: []
  patterns: ["Factory callback pattern for circular import decoupling (CreateComponentsFunc, StartInstancesFunc)", "Duck-typed interfaces for lifecycle component management", "AppStartup rollback helper with 30s context timeout"]

key-files:
  created:
    - internal/lifecycle/app.go
  modified:
    - cmd/nanobot-auto-updater/main.go
    - internal/lifecycle/app_test.go
    - internal/lifecycle/starter.go

key-decisions:
  - "Factory callback pattern (CreateComponentsFunc, StartInstancesFunc) decouples lifecycle from api/instance/health/notifier/updatelog packages that form circular import chains"
  - "AppComponents uses interfaces (APIServerControl, HealthMonitorControl, NotifySender, LogScheduler) instead of concrete types to avoid circular imports"
  - "InstanceManager stored as any type with type assertion in main.go callbacks -- only main.go can import all packages without circularity"
  - "updateLogger and notif created in main.go and passed to AppStartup rather than created inside AppStartup, because updatelog and notifier packages import instance -> lifecycle"
  - "captureLogs signature changed from *os.File to io.Reader with optional io.Closer check to fix pre-existing test compilation failure"

patterns-established:
  - "Factory callback pattern: main.go provides closures that capture concrete types from circular-dependency packages, passed to lifecycle functions as parameters"
  - "Interface-based component references in AppComponents for packages with circular imports"

requirements-completed: [SVC-02, SVC-03]

# Metrics
duration: 32min
completed: 2026-04-10
---

# Phase 47 Plan 01: AppComponents Extraction Summary

**AppComponents + AppStartup (with rollback) + AppShutdown extracted into lifecycle package using factory callback pattern to solve circular import constraints (lifecycle -> api/instance/health/notifier/updatelog -> instance -> lifecycle)**

## Performance

- **Duration:** 32 min
- **Started:** 2026-04-10T14:29:18Z
- **Completed:** 2026-04-10T14:59:30Z
- **Tasks:** 1 (TDD: RED -> GREEN -> REFACTOR)
- **Files modified:** 4

## Accomplishments
- AppComponents struct with 9 component fields + AutoStartDone channel, using interfaces for circular-dependency decoupling
- AppShutdown(ctx, components, logger) with nil-safety and ordered shutdown matching original main.go
- AppStartup with rollback helper (30s context timeout) that cleans up partial components on error
- Auto-start goroutine with panic recovery and AutoStartDone channel signaling
- main.go refactored from 320 lines of inline initialization to ~170 lines using lifecycle.AppStartup/AppShutdown
- 5 unit tests for AppShutdown (nil pointer, all-nil fields, partial components, context timeout, full components)

## Task Commits

Each task was committed atomically (TDD flow):

1. **Task 1 (RED): Add failing tests for AppComponents and AppShutdown** - `b2bd139` (test)
2. **Task 1 (GREEN): Extract AppComponents, AppStartup, AppShutdown into lifecycle package** - `ba5cfce` (feat)

## Files Created/Modified
- `internal/lifecycle/app.go` - AppComponents struct, AppStartup with rollback, AppShutdown, decoupling interfaces (APIServerControl, HealthMonitorControl, NotifySender, LogScheduler)
- `internal/lifecycle/app_test.go` - External test package with 5 AppShutdown unit tests
- `cmd/nanobot-auto-updater/main.go` - Refactored to use lifecycle.AppStartup/AppShutdown with createComponents and startInstances factory closures
- `internal/lifecycle/starter.go` - Fixed captureLogs signature from *os.File to io.Reader with optional Closer check

## Decisions Made
- Factory callback pattern chosen over direct imports: lifecycle cannot import api, instance, health, notifier, or updatelog because they all (directly or indirectly) import instance -> lifecycle. main.go serves as the "bridge" that imports all packages and provides closures
- AppComponents.InstanceManager uses `any` type because instance.InstanceManager is in a circular-dependency package. Type assertion happens in main.go callbacks only
- NotifySender interface includes only IsEnabled() and Notify() -- NotifyStartupResult is handled via StartInstancesFunc callback because its parameter type (*instance.AutoStartResult) is in the instance package
- updateLogger and notif are created in main.go (not inside AppStartup) because updatelog imports instance and notifier imports instance, both forming circular chains
- APIServerControl includes Start() error for type safety, even though Start() is called in main.go's createComponents closure

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Circular import constraints require factory callback pattern instead of direct imports**
- **Found during:** Task 1 (GREEN phase - AppStartup implementation)
- **Issue:** Plan specified AppStartup should directly import and create api.Server, instance.InstanceManager, health.HealthMonitor, notifier.Notifier, updatelog.UpdateLogger. Circular import chain discovered: lifecycle -> api/health/notifier/updatelog -> instance -> lifecycle
- **Fix:** Introduced factory callback pattern (CreateComponentsFunc, StartInstancesFunc) where main.go provides closures that capture concrete types. Changed AppStartup signature to accept LogScheduler and NotifySender interfaces plus factory callbacks. AppComponents fields use interfaces (APIServerControl, HealthMonitorControl, NotifySender, LogScheduler) instead of concrete types
- **Files modified:** internal/lifecycle/app.go, cmd/nanobot-auto-updater/main.go
- **Verification:** go build ./cmd/nanobot-auto-updater/ succeeds, go test ./internal/lifecycle/ -run TestAppShutdown passes
- **Committed in:** ba5cfce (Task 1 GREEN commit)

**2. [Rule 3 - Blocking] Fixed pre-existing captureLogs compilation error blocking tests**
- **Found during:** Task 1 (GREEN phase - running tests)
- **Issue:** captureLogs function in starter.go accepted *os.File but capture_test.go passed *strings.Reader and *errorReader, causing compilation failure that blocked all tests in the package
- **Fix:** Changed captureLogs parameter from *os.File to io.Reader with deferred optional Close via type assertion: `if closer, ok := reader.(io.Closer); ok { closer.Close() }`
- **Files modified:** internal/lifecycle/starter.go
- **Verification:** go test ./internal/lifecycle/ passes (all 5 AppShutdown tests + pre-existing tests)
- **Committed in:** ba5cfce (Task 1 GREEN commit)

---

**Total deviations:** 2 auto-fixed (1 blocking circular import, 1 blocking pre-existing test failure)
**Impact on plan:** Factory callback pattern is the correct architectural solution for Go's circular import constraints. Behavior is identical to original main.go -- same components started in same order, same shutdown order, same timeouts. main.go still imports some packages (api, instance, health, notifier, selfupdate, updatelog) for the factory closures, which differs from plan's expectation that main.go would only import lifecycle.

## Issues Encountered
- Plan assumed AppStartup could directly import and instantiate all component types. Go's strict circular import rules made this impossible for 5 of the 9 packages (api, instance, health, notifier, updatelog). The factory callback pattern is the idiomatic Go solution.
- AppStartup signature differs from plan's design (cfg, logger, version) -> (cfg, logger, version, updateLogger, notif, createComponents, startInstances). This is a necessary adaptation to the circular import constraint.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- AppComponents, AppStartup, AppShutdown ready for Phase 47 Plan 02 (ServiceHandler using svc.Handler interface)
- ServiceHandler.Execute can call AppStartup on svc.Start and AppShutdown on svc.Stop
- AutoStartDone channel available for ServiceHandler to wait on during shutdown
- Factory callback pattern established: Plan 02 should reuse the same createComponents/startInstances closures from main.go

---
*Phase: 47-windows-service-handler*
*Completed: 2026-04-10*

## Self-Check: PASSED

- FOUND: internal/lifecycle/app.go
- FOUND: internal/lifecycle/app_test.go
- FOUND: cmd/nanobot-auto-updater/main.go
- FOUND: internal/lifecycle/starter.go
- FOUND: .planning/phases/47-windows-service-handler/47-01-SUMMARY.md
- FOUND: ba5cfce (Task 1 GREEN feat commit)
- FOUND: b2bd139 (Task 1 RED test commit)

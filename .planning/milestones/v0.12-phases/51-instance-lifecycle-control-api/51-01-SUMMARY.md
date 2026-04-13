---
phase: 51-instance-lifecycle-control-api
plan: 01
subsystem: api
tags: [http, lifecycle, auth, concurrency, rest]

# Dependency graph
requires:
  - phase: 50-instance-config-crud-api
    provides: "InstanceConfigHandler pattern, authMiddleware, writeJSONError helper, server.go registration pattern"
provides:
  - "InstanceLifecycleHandler with HandleStart and HandleStop endpoints"
  - "POST /api/v1/instances/{name}/start and /stop routes with auth middleware"
  - "Update-lock coordination via TryLockUpdate/UnlockUpdate"
affects: [51-02, "phase-53-ui"]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "TryLockUpdate guard pattern for lifecycle operations (mirrors TriggerUpdate/SelfUpdate coordination)"

key-files:
  created:
    - internal/api/instance_lifecycle_handler.go
  modified:
    - internal/api/server.go

key-decisions:
  - "context.Background() with 60s timeout for start (prevents orphaned processes on client disconnect)"
  - "context.Background() with 30s timeout for stop (exceeds inner 5s stopTimeout)"
  - "Success response schema {message, running} differs from restart handler {success} to support Phase 53 UI status indicators"
  - "409 Conflict for wrong-state operations rather than 200 OK"
  - "TryLockUpdate/defer UnlockUpdate in both handlers serializes lifecycle ops with TriggerUpdate/SelfUpdate"

patterns-established:
  - "TryLockUpdate guard: lifecycle handlers acquire update lock, return 409 if already held, defer unlock on success"
  - "Detached context pattern: context.Background() for start/stop to prevent client disconnect from orphaning processes"

requirements-completed: [LC-01, LC-02, LC-03]

# Metrics
duration: 2min
completed: 2026-04-12
---

# Phase 51 Plan 01: Instance Lifecycle Handler Summary

**InstanceLifecycleHandler with HandleStart/HandleStop endpoints, TryLockUpdate coordination with TriggerUpdate/SelfUpdate, and auth-protected route registration**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-12T02:07:37Z
- **Completed:** 2026-04-12T02:10:29Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Created InstanceLifecycleHandler with HandleStart and HandleStop methods including TryLockUpdate update-lock guard
- Registered POST /api/v1/instances/{name}/start and /stop routes wrapped with authMiddleware
- Full project compiles cleanly (go build ./..., go vet ./internal/api/...)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create InstanceLifecycleHandler with HandleStart and HandleStop** - `6b225c9` (feat)
2. **Task 2: Register lifecycle start/stop routes in server.go with auth middleware** - `3d028cc` (feat)

## Files Created/Modified
- `internal/api/instance_lifecycle_handler.go` - InstanceLifecycleHandler with HandleStart/HandleStop, TryLockUpdate guard, detached contexts
- `internal/api/server.go` - Route registration for POST start/stop endpoints with authMiddleware

## Decisions Made
- Used context.Background() for start (60s) and stop (30s) operations to prevent client disconnect from orphaning processes
- Success response uses {message, running} schema instead of restart handler's {success} to support Phase 53 UI status indicators
- TryLockUpdate guard in both handlers prevents races with TriggerUpdate and SelfUpdate via shared atomic.Bool

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Lifecycle start/stop endpoints ready for 51-02 (which extends the API with additional lifecycle features)
- Phase 53 UI can consume the {message, running} response to update status indicators without additional GET requests

---
*Phase: 51-instance-lifecycle-control-api*
*Completed: 2026-04-12*

## Self-Check: PASSED

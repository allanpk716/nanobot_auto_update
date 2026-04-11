---
phase: 50-instance-config-crud-api
plan: 01
subsystem: api
tags: [crud, rest-api, config-management, atomic-persistence, bearer-token-auth]

# Dependency graph
requires:
  - phase: 49-dual-mode-adaptation
    provides: "config.GetCurrentConfig, hot reload with 500ms debounce, dynamic Bearer token getter"
provides:
  - "UpdateConfig(fn func(*Config) error) for atomic read-modify-write config persistence"
  - "InstanceConfigHandler with 6 CRUD endpoints (List, Get, Create, Update, Delete, Copy)"
  - "Route registration for /api/v1/instance-configs with auth middleware"
affects: [50-02, "phase 52 directory management", "phase 53 UI"]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Atomic read-modify-write with deep copy (updateMu + deepCopyConfig)"
    - "Injected config reader (getConfig func() *Config) for testability"
    - "Multi-error validation collecting field + uniqueness errors in one pass"
    - "Custom error types (validationError, notFoundError) for error routing via errors.As"

key-files:
  created:
    - internal/api/instance_config_handler.go
  modified:
    - internal/config/config.go
    - internal/api/server.go

key-decisions:
  - "updateMu separate mutex from globalHotReload.mu to avoid deadlock (updateMu acquired first, then globalHotReload.mu via GetCurrentConfig)"
  - "deepCopyConfig recreates Instances slice and copies AutoStart *bool pointers individually"
  - "Handler accepts getConfig closure for testability; production passes config.GetCurrentConfig"
  - "Copy endpoint uses io.ReadAll + conditional unmarshal for empty body handling"
  - "Delete uses StopAllNanobots (stops all nanobots, hot-reload restarts survivors within 500ms)"

patterns-established:
  - "UpdateConfig(fn func(*Config) error) pattern: atomic mutation with deep copy, rollback on error"
  - "validationError/notFoundError custom error types with errors.As routing"
  - "InstanceConfigRequest/Response with uint32 seconds for startup_timeout (D-06)"

requirements-completed: [IC-01, IC-02, IC-03, IC-04, IC-05, IC-06]

# Metrics
duration: 7min
completed: 2026-04-11
---

# Phase 50 Plan 01: Instance Config CRUD API Summary

**Atomic CRUD API for instance configurations with mutex-protected persistence, deep copy isolation, and injected config reader for testability**

## Performance

- **Duration:** 7 min
- **Started:** 2026-04-11T15:16:32Z
- **Completed:** 2026-04-11T15:24:25Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments
- UpdateConfig function with updateMu mutex serializes full read-modify-write cycle, preventing concurrent API race conditions
- InstanceConfigHandler with 6 endpoints using injected config reader (config.GetCurrentConfig in production, closure in tests)
- All routes registered with Bearer token auth middleware; full project compiles cleanly

## Task Commits

Each task was committed atomically:

1. **Task 1: Add UpdateConfig function to config package** - `de5d510` (feat)
2. **Task 2: Create InstanceConfigHandler with 6 CRUD endpoints** - `5c870f4` (feat)
3. **Task 3: Register instance-config routes in server.go** - `2631bb3` (feat)

## Files Created/Modified
- `internal/config/config.go` - Added UpdateConfig(), deepCopyConfig(), and updateMu mutex for atomic config persistence
- `internal/api/instance_config_handler.go` - New file: InstanceConfigHandler with HandleList/Get/Create/Update/Delete/Copy + validation helpers
- `internal/api/server.go` - Registered 6 new routes under /api/v1/instance-configs with authMiddleware

## Decisions Made
- updateMu is separate from globalHotReload.mu to avoid deadlock (locking order: updateMu first, then globalHotReload.mu internally via GetCurrentConfig)
- deepCopyConfig recreates the Instances slice so append does not corrupt the original backing array; AutoStart *bool copied individually
- Handler constructor accepts getConfig func() *Config for testability; no NewServer signature change needed
- Copy endpoint handles empty body via io.ReadAll + conditional json.Unmarshal
- Delete endpoint calls StopAllNanobots (known limitation: stops all nanobot.exe system-wide; hot-reload restarts survivors within 500ms)
- Validation collects ALL errors (field + uniqueness) in one pass, returns 422 with field-level details

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- CRUD API fully functional and compiled; ready for Phase 50 Plan 02 (unit tests)
- UpdateConfig function available for any future atomic config mutation needs
- InstanceConfigHandler can be tested independently via injected getConfig closure

## Self-Check: PASSED

All 3 created/modified files verified present on disk. All 3 task commits (de5d510, 5c870f4, 2631bb3) verified in git log.

---
*Phase: 50-instance-config-crud-api*
*Completed: 2026-04-11*

---
phase: 52-nanobot-config-management-api
plan: 02
subsystem: api
tags: [nanobot-config, callbacks, lifecycle-integration, tests, windows-paths]

# Dependency graph
requires:
  - phase: 52-nanobot-config-management-api
    plan: 01
    provides: "NanobotConfigManager and NanobotConfigHandler"
provides:
  - "InstanceConfigHandler callback injection (onCreateInstance, onCopyInstance, onDeleteInstance)"
  - "CleanupConfig method for nanobot config directory removal"
  - "Server.go callback wiring to NanobotConfigManager"
  - "Comprehensive test coverage for nanobot config lifecycle"
affects: [53-ui]

# Tech tracking
tech-stack:
  added: []
  patterns: [callback-injection, setter-methods-for-optional-dependencies, non-blocking-callbacks]

key-files:
  created:
    - internal/nanobot/config_manager_test.go
    - internal/api/nanobot_config_handler_test.go
  modified:
    - internal/api/instance_config_handler.go
    - internal/api/instance_config_handler_test.go
    - internal/api/server.go
    - internal/nanobot/config_manager.go

key-decisions:
  - "Callback fields are nil by default, safe to ignore in tests that don't need nanobot config behavior"
  - "Callbacks log warnings on failure, do not block the primary create/copy/delete operation"
  - "sourceStartCommand captured inside UpdateConfig closure to read from source instance before clone"
  - "deletedStartCommand captured before config removal to enable cleanup path resolution"
  - "CleanupConfig uses os.RemoveAll on the parent directory of the resolved config path"

patterns-established:
  - "Setter methods for optional lifecycle callbacks (SetOnCreateInstance, SetOnCopyInstance, SetOnDeleteInstance)"
  - "Non-blocking callback pattern: callback failure is logged as warning, primary operation succeeds"

requirements-completed: [NC-01, NC-04]

# Metrics
duration: 16min
completed: 2026-04-12
---

# Phase 52 Plan 02: Callback Integration and Tests Summary

**Callback injection for nanobot config lifecycle (create/copy/delete) with comprehensive tests including Windows path edge cases**

## Performance

- **Duration:** 16 min
- **Started:** 2026-04-12T06:03:44Z
- **Completed:** 2026-04-12T06:20:02Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Added onCreateInstance, onCopyInstance, onDeleteInstance callback fields to InstanceConfigHandler
- Added setter methods for optional dependency injection
- HandleCreate calls onCreateInstance after config write (non-blocking on failure)
- HandleCopy captures sourceStartCommand and calls onCopyInstance after config write (non-blocking)
- HandleDelete captures deletedStartCommand and calls onDeleteInstance after config write (non-blocking)
- Added CleanupConfig method to ConfigManager for directory removal on instance deletion
- Wired all callbacks in server.go to NanobotConfigManager methods
- Documented callback contract on InstanceConfigHandler struct
- 23 tests in config_manager_test.go covering ParseConfigPath, GenerateDefaultConfig, ReadConfig/WriteConfig, CreateDefaultConfig, CloneConfig, CleanupConfig
- 10 tests in nanobot_config_handler_test.go covering HandleGet and HandlePut
- 5 tests in instance_config_handler_test.go covering callback invocation and non-blocking failure

## Task Commits

Each task was committed atomically:

1. **Task 1: Inject nanobot config callbacks** - `29670d3` (feat)
2. **Task 2: Write comprehensive tests** - `fbae387` (test)

## Files Created/Modified
- `internal/api/instance_config_handler.go` - Added callback fields, setter methods, callback invocations in HandleCreate/HandleCopy/HandleDelete
- `internal/api/server.go` - Added SetOnCreateInstance, SetOnCopyInstance, SetOnDeleteInstance wiring to nanobotConfigManager
- `internal/nanobot/config_manager.go` - Added CleanupConfig method for removing nanobot config directories
- `internal/nanobot/config_manager_test.go` - 23 tests covering all ConfigManager functions including Windows path edge cases
- `internal/api/nanobot_config_handler_test.go` - 10 tests for HandleGet (success, not found, lazy-creation, auth) and HandlePut (success, invalid JSON, not found, auth, hint)
- `internal/api/instance_config_handler_test.go` - Added 5 callback tests (create/copy/delete invocation, non-blocking failure)

## Decisions Made
- Callbacks are nil by default, allowing existing tests to continue working without modification
- sourceStartCommand captured inside UpdateConfig closure to ensure it reads from the current config state
- deletedStartCommand captured before config removal since the instance data is deleted from the slice
- CleanupConfig uses os.RemoveAll which does not error on nonexistent paths (safe for idempotent cleanup)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed test expectations for Windows path normalization**
- **Found during:** Task 2 (config_manager_test.go)
- **Issue:** ParseConfigPath uses filepath.Abs which normalizes forward slashes to backslashes on Windows. Test assertions expected forward slashes.
- **Fix:** Updated test assertions to use filepath.FromSlash() for expected paths on Windows
- **Files modified:** internal/nanobot/config_manager_test.go
- **Verification:** All 23 nanobot tests pass

**2. [Rule 1 - Bug] Fixed JSON number type mismatch in roundtrip tests**
- **Found during:** Task 2 (config_manager_test.go)
- **Issue:** JSON unmarshal converts all numbers to float64, but test used int/uint32 types for comparison
- **Fix:** Updated test assertions to use float64 for numbers read back from JSON
- **Files modified:** internal/nanobot/config_manager_test.go
- **Verification:** All tests pass

---

**Total deviations:** 2 auto-fixed (test expectation bugs)
**Impact on plan:** None -- both were test-side fixes, no production code changes needed.

## Issues Encountered
None beyond the test expectation fixes documented above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Instance create/copy/delete flows now manage nanobot config lifecycle automatically
- All callbacks wired and tested
- Ready for UI integration in Phase 53

---
*Phase: 52-nanobot-config-management-api*
*Completed: 2026-04-12*

## Self-Check: PASSED

- FOUND: internal/api/instance_config_handler.go
- FOUND: internal/api/server.go
- FOUND: internal/nanobot/config_manager.go
- FOUND: internal/nanobot/config_manager_test.go
- FOUND: internal/api/nanobot_config_handler_test.go
- FOUND: internal/api/instance_config_handler_test.go
- FOUND: 29670d3 (Task 1 commit)
- FOUND: fbae387 (Task 2 commit)

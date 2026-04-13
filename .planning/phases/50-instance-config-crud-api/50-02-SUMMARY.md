---
phase: 50-instance-config-crud-api
plan: 02
subsystem: testing
tags: [tdd, crud-tests, concurrency-safety, integration-tests, auth-tests]

# Dependency graph
requires:
  - phase: 50-01
    provides: "InstanceConfigHandler with 6 CRUD endpoints, UpdateConfig function, auth middleware"
provides:
  - "Comprehensive test coverage for all 6 CRUD endpoints (19 handler test functions)"
  - "UpdateConfig test coverage including concurrency safety (7 test functions)"
  - "Fix for viper state corruption during rapid WriteConfig calls (ReadInConfig before Set)"
  - "skipReload flag to suppress WatchConfig interference during UpdateConfig writes"
affects: [50-02, "phase 52 directory management", "phase 53 UI"]

# Tech tracking
tech-stack:
  added: []
patterns:
  - "Integration-style mutation tests (real config file + Load + WatchConfig per test)"
  - "Read-only handler tests with injected config closure (no file system dependency)"
  - "ReadInConfig() before v.Set+WriteConfig to prevent viper state corruption"
  - "skipReload flag on hotReloadState to suppress WatchConfig during UpdateConfig writes"

key-files:
  created:
    - internal/api/instance_config_handler_test.go
    - internal/config/update_test.go
  modified:
    - internal/config/config.go
    - internal/config/hotreload.go

key-decisions:
  - "Fix UpdateConfig: call v.ReadInConfig() before v.Set+WriteConfig to prevent viper from losing keys not explicitly set via v.Set()"
  - "Add skipReload flag to hotReloadState to prevent doReload from running during UpdateConfig writes"
  - "Mutation handler tests use integration setup with real config file; read-only tests use injected config closure"

patterns-established:
  - "Integration test pattern: t.TempDir + config.yaml + config.Load + config.WatchConfig + config.GetCurrentConfig + t.Cleanup(config.StopWatch)"
  - "ReadInConfig-before-WriteConfig pattern: ensures viper's internal state is fully synchronized with the file before overwriting it"

requirements-completed: [IC-01, IC-02, IC-03, IC-04, IC-05, IC-06]

# Metrics
duration: 30min
completed: 2026-04-11
---

# Phase 50 Plan 02: Instance Config CRUD API Tests Summary

**Comprehensive test coverage for all 6 CRUD endpoints and UpdateConfig with fix for viper state corruption during rapid writes**

## Performance

- **Duration:** 30 min
- **Started:** 2026-04-11T16:00:55Z
- **Completed:** 2026-04-11T16:30:55Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- 19 handler test functions covering all 6 CRUD endpoints with success, error, validation, and auth cases
- 7 UpdateConfig test functions covering write persistence, error paths, deep copy safety, and concurrent safety (10 goroutines, 11 instances preserved)
- Fixed viper state corruption bug where rapid WriteConfig calls lost keys like api.bearer_token
- All tests deterministic with no time.Sleep, no flakiness on Windows CI

## Task Commits

Each task was committed atomically:

1. **Task 1: Write comprehensive handler tests using injected config reader** - `40edec0` (test)
2. **Task 2: Write UpdateConfig tests including concurrency safety** - `5ca9e29` (test)

## Files Created/Modified
- `internal/api/instance_config_handler_test.go` - 19 test functions covering all CRUD endpoints, auth, validation, and integration tests
- `internal/config/update_test.go` - 7 test functions for UpdateConfig including concurrency safety with 10 goroutines
- `internal/config/config.go` - Fixed UpdateConfig: added ReadInConfig() before Set+WriteConfig to prevent viper state corruption
- `internal/config/hotreload.go` - Added skipReload flag to suppress WatchConfig reloads during UpdateConfig writes

## Decisions Made
- Fixed UpdateConfig to call v.ReadInConfig() before v.Set+WriteConfig. Root cause: viper's internal state mixes v.Set() keys with file-read keys; without re-reading the file, keys not explicitly set via v.Set() (like api.bearer_token) can be lost during rapid writes. The ReadInConfig() call synchronizes viper's state with the current file content before the write.
- Added skipReload flag to hotReloadState. When UpdateConfig is writing, doReload is suppressed to prevent ReadInConfig() from corrupting viper's internal state with stale file data.
- Mutation handler tests use integration-style setup (real config file + Load + WatchConfig) because HandleCreate/Update/Delete/Copy call config.UpdateConfig internally. Read-only tests (List, Get, Auth) use injected config closure for pure unit testing without file system.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed viper state corruption during rapid WriteConfig calls**
- **Found during:** Task 2 (UpdateConfig tests)
- **Issue:** Concurrent UpdateConfig calls lost api.bearer_token and api.port reverted to default values. Viper's internal state mixes v.Set() keys with file-read keys; without re-reading the file before each write, keys not explicitly set via v.Set() could be lost.
- **Fix:** Added v.ReadInConfig() call before v.Set("instances") + v.WriteConfig() in UpdateConfig to synchronize viper's internal state with the file. Added skipReload flag to hotReloadState to suppress WatchConfig's doReload during writes.
- **Files modified:** internal/config/config.go, internal/config/hotreload.go
- **Verification:** TestUpdateConfig_ConcurrentMutationsNoDataLoss passes with 10 concurrent goroutines, all 11 instances preserved, bearer_token intact. TestUpdateConfig_ConcurrentPreservesOtherFields verifies non-instance fields survive writes.
- **Committed in:** 5ca9e29 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Bug fix was essential for correctness. Without it, production API would lose config fields during rapid successive updates. No scope creep.

## Issues Encountered
- The viper library on Windows has a subtle interaction between WriteConfig(), fsnotify file watching, and ReadInConfig() that can cause internal state corruption. The ReadInConfig-before-WriteConfig pattern resolves this.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All CRUD endpoint tests pass, UpdateConfig concurrency safety verified
- Bug fix in UpdateConfig prevents viper state corruption in production
- Ready for Phase 50 verification or next phase

## Self-Check: PASSED

All 4 created/modified files verified present on disk. Both task commits (40edec0, 5ca9e29) verified in git log. All handler tests pass (go test ./internal/api/... -run TestHandle). All UpdateConfig tests pass (go test ./internal/config/... -run TestUpdateConfig).

---
*Phase: 50-instance-config-crud-api*
*Completed: 2026-04-11*

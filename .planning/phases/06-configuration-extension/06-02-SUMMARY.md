---
phase: 06-configuration-extension
plan: 02
subsystem: testing
tags: [go, testing, yaml, viper, configuration, integration-tests, unit-tests, tdd]

# Dependency graph
requires:
  - phase: 06-01
    provides: InstanceConfig structure, Config extensions, validation functions
provides:
  - Complete test suite for multi-instance configuration
  - Integration tests for YAML loading
  - Test data files for all configuration scenarios
  - 92.4% test coverage for config package
affects: [06-future-plans, configuration-validation]

# Tech tracking
tech-stack:
  added: []
  patterns: [table-driven-tests, integration-testing, test-data-fixtures, error-message-validation]

key-files:
  created:
    - testutil/testdata/config/instances_valid.yaml
    - testutil/testdata/config/instances_duplicate_name.yaml
    - testutil/testdata/config/instances_duplicate_port.yaml
    - testutil/testdata/config/legacy_v1.yaml
    - testutil/testdata/config/mixed_mode.yaml
  modified:
    - internal/config/config_test.go
    - internal/config/config.go

key-decisions:
  - "Defaults applied in Validate() not New() to enable proper mode detection"
  - "Test data uses relative paths from internal/config directory"
  - "Integration tests verify both loading and validation in single workflow"

patterns-established:
  - "Test data files in testutil/testdata/config/ for YAML fixtures"
  - "Integration tests verify error messages contain expected Chinese text"
  - "Defaults for legacy mode applied during validation, not initialization"

requirements-completed: [CONF-01, CONF-02, CONF-03]

# Metrics
duration: 15min
completed: 2026-03-10
---

# Phase 06 Plan 02: Multi-Instance Configuration Testing

**Complete test suite for multi-instance configuration with 92.4% coverage, integration tests for YAML loading, and test fixtures for all validation scenarios**

## Performance

- **Duration:** 15 min
- **Started:** 2026-03-10T14:32:57Z
- **Completed:** 2026-03-10T14:47:52Z
- **Tasks:** 2
- **Files modified:** 7 (5 created, 2 modified)

## Accomplishments
- Created 5 YAML test data files covering all configuration scenarios (valid, duplicates, legacy, mixed mode)
- Implemented 5 integration tests for end-to-end YAML loading and validation
- Fixed critical bug: mode detection now works correctly by applying defaults in Validate() not New()
- Achieved 92.4% test coverage (exceeds 80% target)
- Verified backward compatibility with v1.0 configuration files

## Task Commits

Each task was committed atomically:

1. **Task 1: 创建 InstanceConfig 和多实例验证单元测试** - Tests already existed from 06-01
2. **Task 2: 创建测试数据文件和集成测试** - `c8f164e` (test), `e9f14c3` (test + fix)

**Plan metadata:** Will be created after STATE.md update

_Note: Task 1 tests were created during 06-01 TDD implementation. Task 2 added test data files and integration tests, plus a critical bug fix._

## Files Created/Modified
- `testutil/testdata/config/instances_valid.yaml` - Valid multi-instance configuration with 2 instances
- `testutil/testdata/config/instances_duplicate_name.yaml` - Duplicate name validation test case
- `testutil/testdata/config/instances_duplicate_port.yaml` - Duplicate port validation test case
- `testutil/testdata/config/legacy_v1.yaml` - v1.0 single-instance config for backward compatibility
- `testutil/testdata/config/mixed_mode.yaml` - Mixed mode detection test case
- `internal/config/config_test.go` - Added 5 integration tests (TestLoad*)
- `internal/config/config.go` - Fixed mode detection by moving defaults from New() to Validate()

## Decisions Made
- **Defaults in Validate() not New()**: Nanobot.Port and StartupTimeout defaults now applied during validation for legacy mode, enabling proper detection of multi-instance vs legacy mode
- **Test data location**: testutil/testdata/config/ provides organized fixtures for YAML configuration testing
- **Integration test scope**: Tests verify complete Load() workflow including YAML parsing, defaults, and validation

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed mode detection for multi-instance configuration**
- **Found during:** Task 2 (integration test execution)
- **Issue:** TestLoadInstancesYAML failed with "不能同时使用" error - defaults() was setting Nanobot.Port=18790, causing mode detection to think both modes were configured even when YAML only had instances
- **Fix:** Moved Nanobot defaults from defaults() to Validate(), only applied when using legacy mode (len(Instances)==0). Removed viper.SetDefault for nanobot.port/startup_timeout
- **Files modified:** internal/config/config.go (defaults(), Validate(), Load())
- **Verification:** All integration tests pass, TestLoadInstancesYAML loads successfully, legacy config still works
- **Committed in:** e9f14c3 (Task 2 commit)

**2. [Rule 2 - Missing Critical] Updated TestNewConfigDefaults**
- **Found during:** Task 2 (after fixing mode detection)
- **Issue:** Test expected Nanobot.Port/StartupTimeout to have defaults in New(), but these now default to 0 and are set in Validate()
- **Fix:** Updated test to reflect new behavior - defaults now applied in Validate() during validation, not in New() during initialization
- **Files modified:** internal/config/config_test.go
- **Verification:** All tests pass including TestNewConfigDefaults
- **Committed in:** e9f14c3 (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 bug, 1 missing critical test update)
**Impact on plan:** Both fixes essential for correct multi-instance mode detection. Mode detection bug would have broken all multi-instance usage. No scope creep.

## Issues Encountered
- **Relative path discovery**: Integration tests needed `../../testutil/testdata/config/` (not `../testutil/`) because they run from internal/config directory
- **Default value conflict**: viper.SetDefault was causing Nanobot.Port to always be non-zero, triggering mode conflict detection. Solution: only set defaults for fields that don't affect mode detection

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Configuration testing complete with comprehensive coverage
- All validation scenarios tested (unique names, unique ports, mode compatibility)
- Backward compatibility verified for v1.0 configurations
- Ready for Phase 6 remaining plans (instance management implementation)

## Self-Check: PASSED

**Verified:**
- 5/5 test data files created and found
- 2/2 task commits found (c8f164e, e9f14c3)
- All integration tests passing
- Test coverage: 92.4% (exceeds 80% target)

---
*Phase: 06-configuration-extension*
*Completed: 2026-03-10*

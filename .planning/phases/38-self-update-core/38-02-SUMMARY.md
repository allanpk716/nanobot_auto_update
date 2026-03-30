---
phase: 38-self-update-core
plan: 02
subsystem: selfupdate
tags: [sha256, checksum, zip-extract, selfupdate-apply, config, golang, tdd]

# Dependency graph
requires:
  - phase: 38-plan-01
    provides: "Updater struct, CheckLatest, NeedUpdate, ReleaseInfo types"
provides:
  - "Update() method with full download-checksum-verify-extract-apply pipeline"
  - "verifyChecksum (SHA256), parseChecksum (GoReleaser format), extractExeFromZip"
  - "SelfUpdateConfig in config package with defaults and validation"
  - "self_update YAML config section support"

affects: [phase-39-http-api-integration]

# Tech tracking
tech-stack:
  added: [github.com/minio/selfupdate@v0.6.0, archive/zip, crypto/sha256]
  patterns: [tdd-red-green, in-memory-zip-extraction, sha256-checksum-verification, goreleaser-checksums-parsing, selfupdate-apply-with-rollback]

key-files:
  created:
    - internal/config/selfupdate.go
    - internal/config/selfupdate_test.go
  modified:
    - internal/selfupdate/selfupdate.go
    - internal/selfupdate/selfupdate_test.go
    - internal/config/config.go
    - internal/config/multi_instance_test.go

key-decisions:
  - "In-memory ZIP extraction via bytes.Reader (no temp files, per D-01)"
  - "GoReleaser checksums.txt parsing with two-space delimiter"
  - "exeName constant for binary name inside ZIP (nanobot-auto-updater.exe)"
  - "Full Update() pipeline: NeedUpdate -> download -> checksum -> extract -> Apply"
  - "OldSavePath set to exe path + .old for backup"
  - "RollbackError checked for dual-failure detection"
  - "SelfUpdateConfig defaults: HQGroup/nanobot-auto-updater"

requirements-completed: [UPDATE-03, UPDATE-04, UPDATE-05, UPDATE-07]

# Metrics
duration: 13min
completed: 2026-03-30
---

# Phase 38 Plan 02: Update Pipeline and Config Extension Summary

**Update() method with download-checksum-verify-extract-apply pipeline using SHA256, in-memory ZIP extraction, and minio/selfupdate, plus SelfUpdateConfig in config package**

## Performance

- **Duration:** 13 min
- **Started:** 2026-03-30T03:40:38Z
- **Completed:** 2026-03-30T03:53:49Z
- **Tasks:** 2 (TDD: RED + GREEN for each, no refactor needed)
- **Files modified:** 6

## Accomplishments
- Added Update() method implementing full self-update pipeline: check version, download checksums + ZIP, SHA256 verify, in-memory ZIP extract, minio/selfupdate Apply
- Added verifyChecksum (SHA256 comparison), parseChecksum (GoReleaser format), extractExeFromZip (in-memory, no temp files)
- Added SelfUpdateConfig to config package with defaults (HQGroup/nanobot-auto-updater), validation, and viper integration
- All 26 tests pass (22 selfupdate + 4 config), no regressions in existing tests

## Task Commits

Each task was committed atomically with TDD RED-GREEN pattern:

1. **Task 1 (RED): Add failing tests for Update pipeline** - `af31397` (test)
2. **Task 1 (GREEN): Implement Update pipeline** - `d385016` (feat)
3. **Task 2 (RED): Add failing tests for SelfUpdateConfig** - `f81b45c` (test)
4. **Task 2 (GREEN): Implement SelfUpdateConfig in config package** - `11f4ce9` (feat)

## Files Created/Modified
- `internal/selfupdate/selfupdate.go` - Added verifyChecksum, parseChecksum, extractExeFromZip, download, Update() method with full pipeline
- `internal/selfupdate/selfupdate_test.go` - Added 8 new tests (verifyChecksum x2, parseChecksum x2, extractExeFromZip x2, Update_FullFlow, Update_AlreadyUpToDate)
- `internal/config/selfupdate.go` - New file: SelfUpdateConfig struct with Validate() method
- `internal/config/selfupdate_test.go` - New file: 4 tests (Defaults, ViperLoad, EmptyValues, ValidValues)
- `internal/config/config.go` - Added SelfUpdate field, defaults, viper defaults, validation integration
- `internal/config/multi_instance_test.go` - Fixed existing test to include SelfUpdate config

## Decisions Made
- In-memory ZIP extraction using bytes.Reader avoids temp file creation on Windows (per D-01)
- GoReleaser checksums.txt parsed with two-space delimiter, hex decode for SHA256 bytes
- exeName constant "nanobot-auto-updater.exe" matches GoReleaser Windows binary naming
- Update() returns early if already up-to-date (no unnecessary downloads)
- OldSavePath uses exe path + ".old" suffix for backup before Apply
- RollbackError checked to distinguish between "update failed, rolled back" vs "update + rollback both failed"
- SelfUpdateConfig defaults provide zero-config operation for HQGroup/nanobot-auto-updater

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed TestUpdate_FullFlow server URL reference before creation**
- **Found during:** Task 1 GREEN phase
- **Issue:** httptest.NewServer handler referenced `server.URL` before server was created
- **Fix:** Used `serverURL` variable captured by closure, set after httptest.NewServer
- **Files modified:** internal/selfupdate/selfupdate_test.go
- **Commit:** d385016

**2. [Rule 3 - Blocking] Fixed TestSelfUpdateConfig_ViperLoad missing start_command**
- **Found during:** Task 2 GREEN phase
- **Issue:** Test YAML instance config lacked required `start_command` field
- **Fix:** Added `start_command: "echo test"` to test YAML
- **Files modified:** internal/config/selfupdate_test.go
- **Commit:** 11f4ce9

**3. [Rule 3 - Blocking] Fixed TestConfigValidateWithInstances missing SelfUpdate config**
- **Found during:** Task 2 GREEN phase
- **Issue:** Existing test Config struct lacked new SelfUpdate field, triggering validation error
- **Fix:** Added SelfUpdate field with default values to valid_multi-instance_config test case
- **Files modified:** internal/config/multi_instance_test.go
- **Commit:** 11f4ce9

### Deferred Items
- `internal/lifecycle/capture_test.go` has pre-existing build failure (type mismatch strings.Reader vs *os.File) - not caused by this plan's changes

## Next Phase Readiness
- selfupdate package complete with CheckLatest, NeedUpdate, and Update methods
- Config package extended with SelfUpdate section and defaults
- Ready for Phase 39 to wire updater.Update() into HTTP API and CLI
- All types and methods follow the D-03 public API design from CONTEXT.md

## Self-Check: PASSED

- FOUND: internal/selfupdate/selfupdate.go
- FOUND: internal/selfupdate/selfupdate_test.go
- FOUND: internal/config/selfupdate.go
- FOUND: internal/config/selfupdate_test.go
- FOUND: af31397 (test commit)
- FOUND: d385016 (feat commit)
- FOUND: f81b45c (test commit)
- FOUND: 11f4ce9 (feat commit)

---
*Phase: 38-self-update-core*
*Completed: 2026-03-30*

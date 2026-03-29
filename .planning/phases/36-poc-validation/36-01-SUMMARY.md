---
phase: 36-poc-validation
plan: 01
subsystem: selfupdate
tags: [minio/selfupdate, windows-exe-replacement, self-spawn, ldflags, build-tags]

# Dependency graph
requires:
  - phase: none
    provides: "First phase of v0.8 milestone, no prior dependencies"
provides:
  - "Validated minio/selfupdate v0.6.0 Windows exe replacement mechanism"
  - "PoC reference implementation in tmp/poc_selfupdate.go"
  - "Automated test in tmp/poc_selfupdate_test.go validating VALID-01/02/03"
  - "minio/selfupdate v0.6.0 added to go.mod for Phase 38"
affects: [phase-38, phase-40]

# Tech tracking
tech-stack:
  added: [github.com/minio/selfupdate v0.6.0, aead.dev/minisign v0.2.0]
  patterns: [selfupdate.Apply with OldSavePath, self-spawn via cmd.Start + SysProcAttr, //go:build manual test isolation]

key-files:
  created: [tmp/poc_selfupdate.go, tmp/poc_selfupdate_test.go]
  modified: [go.mod, go.sum, tmp/test_config_loader.go, tmp/test_error.go, tmp/test_error2.go, tmp/test_flags.go, tmp/test_nanobot_output.go, tmp/test_nanobot_process.go, tmp/test_nanobot_startup.go, tmp/test_port_detection.go, tmp/test_ports.go, tmp/test_starter_exact.go]

key-decisions:
  - "Added //go:build manual tag to poc_selfupdate.go to isolate from existing tmp/ files"
  - "Added //go:build ignore to 10 old tmp test files to prevent package main conflicts"
  - "Added 1s cleanup pause in test to handle v2 process file locks on Windows"

patterns-established:
  - "selfupdate.Apply(newBin, opts) with explicit OldSavePath for visible .old backup"
  - "Self-spawn restart via exec.Command(exePath).Start() with CREATE_NO_WINDOW"
  - "Version file output pattern (exePath + '.version') for automated verification"
  - "Polling verification: 500ms interval, 30s max timeout for Windows Defender delays"

requirements-completed: [VALID-01, VALID-02, VALID-03]

# Metrics
duration: 5min
completed: 2026-03-29
---

# Phase 36 Plan 01: PoC Self-Update Validation Summary

**Validated minio/selfupdate v0.6.0 can replace a running Windows exe, save .old backup, and self-spawn new version in under 3 seconds**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-29T11:00:46Z
- **Completed:** 2026-03-29T11:05:38Z
- **Tasks:** 2
- **Files modified:** 12

## Accomplishments
- PoC program replaces its own running exe with a new version using minio/selfupdate v0.6.0
- .old backup file created (5.5MB) and visible in filesystem after replacement
- Self-spawn restart works: v2 process starts independently after v1 exits
- Automated test validates all 3 VALID requirements in ~3 seconds with proper cleanup

## Task Commits

Each task was committed atomically:

1. **Task 1: Create PoC main program with selfupdate.Apply and self-spawn restart** - `33ed98c` (feat)
2. **Task 2: Create automated test that validates self-update flow end-to-end** - `1662f04` (feat)

## Files Created/Modified
- `tmp/poc_selfupdate.go` - PoC main program with version injection, selfupdate.Apply(), self-spawn, and version file output
- `tmp/poc_selfupdate_test.go` - Automated test with //go:build manual tag, builds v1/v2, validates VALID-01/02/03
- `go.mod` - Added github.com/minio/selfupdate v0.6.0 and aead.dev/minisign v0.2.0
- `go.sum` - Added checksum entries for minio/selfupdate and dependencies
- `tmp/test_*.go` (10 files) - Added //go:build ignore to prevent package main conflicts with PoC files

## Decisions Made
- Added `//go:build manual` tag to poc_selfupdate.go (not just the test file) so both files compile together only when `-tags manual` is specified, isolating from existing tmp/ test programs
- Added `//go:build ignore` to 10 old tmp test files that were causing "main redeclared" build errors -- these legacy test programs from earlier phases are no longer compiled by `go build ./...`
- Added 1-second cleanup pause before file removal in test defer to handle Windows file locking by the still-running v2 process

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added //go:build manual tag to poc_selfupdate.go and //go:build ignore to old tmp files**
- **Found during:** Task 2 (test execution)
- **Issue:** tmp/ directory contains 10 existing `package main` files from earlier phases with no build tags, causing "main redeclared" compilation errors when building the test
- **Fix:** Added `//go:build manual` to poc_selfupdate.go (matching the test file) and `//go:build ignore` to all 10 old test files
- **Files modified:** tmp/poc_selfupdate.go, tmp/test_config_loader.go, tmp/test_error.go, tmp/test_error2.go, tmp/test_flags.go, tmp/test_nanobot_output.go, tmp/test_nanobot_process.go, tmp/test_nanobot_startup.go, tmp/test_port_detection.go, tmp/test_ports.go, tmp/test_starter_exact.go
- **Verification:** `go test ./tmp/ -run TestSelfUpdate -v -tags manual -timeout 60s` passes
- **Committed in:** 1662f04 (Task 2 commit)

**2. [Rule 1 - Bug] Added cleanup pause and reordered cleanup artifacts**
- **Found during:** Task 2 (test cleanup verification)
- **Issue:** poc_v1.exe left behind after test because v2 process still held file lock
- **Fix:** Added 1-second pause before cleanup, reordered cleanup to remove non-locked files first
- **Files modified:** tmp/poc_selfupdate_test.go
- **Verification:** `ls tmp/poc_v*.exe` after test shows no leftover files
- **Committed in:** 1662f04 (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug)
**Impact on plan:** Both auto-fixes necessary for test isolation and Windows file locking. No scope creep.

## Issues Encountered
None beyond the deviations documented above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- minio/selfupdate v0.6.0 validated on Windows: exe replacement, .old backup, self-spawn all work correctly
- Dependency already in go.mod, ready for Phase 38 internal/selfupdate/ package implementation
- PoC code in tmp/ serves as reference implementation for Phase 38
- No blockers or concerns for subsequent phases

## Self-Check: PASSED

- tmp/poc_selfupdate.go: FOUND
- tmp/poc_selfupdate_test.go: FOUND
- .planning/phases/36-poc-validation/36-01-SUMMARY.md: FOUND
- Commit 33ed98c (Task 1): FOUND
- Commit 1662f04 (Task 2): FOUND

---
*Phase: 36-poc-validation*
*Completed: 2026-03-29*

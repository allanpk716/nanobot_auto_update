---
phase: 38-self-update-core
plan: 01
subsystem: selfupdate
tags: [github-api, semver, cache, httptest, golang]

# Dependency graph
requires:
  - phase: 37-ci-cd-pipeline
    provides: "GoReleaser ZIP naming convention for asset matching"
provides:
  - "Updater struct with CheckLatest() and NeedUpdate() methods"
  - "ReleaseInfo type with Version, DownloadURL, ChecksumURL fields"
  - "SelfUpdateConfig type for GitHub owner/repo configuration"
  - "1-hour cache for GitHub API responses"
  - "semver version comparison with dev version handling"

affects: [phase-39-http-api-integration, phase-40-safety-recovery]

# Tech tracking
tech-stack:
  added: [golang.org/x/mod@v0.17.0]
  patterns: [httptest-mock-server, struct-based-cache-with-TTL, semver-version-comparison]

key-files:
  created:
    - internal/selfupdate/selfupdate.go
    - internal/selfupdate/selfupdate_test.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "golang.org/x/mod/semver for standard semver comparison instead of hand-rolled string comparison"
  - "struct-based cache with cacheTime field for testability (manipulate cacheTime to test expiry)"
  - "baseURL field on Updater for test injection of httptest server URL"
  - "Asset matching by suffix (_windows_amd64.zip, _checksums.txt) for GoReleaser naming convention"

patterns-established:
  - "httptest mock server for GitHub API testing (no real API calls in unit tests)"
  - "Cache pattern: cachedRelease + cacheTime + TTL check before API call"

requirements-completed: [UPDATE-01, UPDATE-02, UPDATE-06]

# Metrics
duration: 9min
completed: 2026-03-30
---

# Phase 38 Plan 01: Selfupdate Package Foundation Summary

**Selfupdate package with GitHub Release checking via httptest-mocked API, semver version comparison via golang.org/x/mod, and 1-hour TTL cache**

## Performance

- **Duration:** 9 min
- **Started:** 2026-03-30T03:23:25Z
- **Completed:** 2026-03-30T03:32:17Z
- **Tasks:** 1 (TDD: RED + GREEN, no refactor needed)
- **Files modified:** 4

## Accomplishments
- Created internal/selfupdate/ package with Updater struct, CheckLatest(), NeedUpdate(), and 1-hour cache
- All 14 unit tests pass using httptest mock servers (no real GitHub API calls)
- semver comparison correctly handles older/same/newer/dev/v-prefixed versions
- Cache prevents redundant API calls within TTL; expiry triggers fresh fetch

## Task Commits

Each task was committed atomically:

1. **Task 1 (RED): Add failing tests** - `7cc5c9d` (test)
2. **Task 1 (GREEN): Implement selfupdate package** - `cc7701b` (feat)

_Note: No REFACTOR commit needed — code was clean on first pass._

## Files Created/Modified
- `internal/selfupdate/selfupdate.go` - Updater struct, CheckLatest, NeedUpdate, cache, public types (SelfUpdateConfig, ReleaseInfo, AssetInfo)
- `internal/selfupdate/selfupdate_test.go` - 14 tests: CheckLatest (success, 500, 404, no zip, no checksums, invalid JSON), NeedUpdate (older, same, newer, dev, v-prefix), cache (hit, expiry), API error propagation
- `go.mod` - Added golang.org/x/mod v0.17.0 as direct dependency
- `go.sum` - Updated checksums

## Decisions Made
- Used golang.org/x/mod/semver for version comparison (standard library extension, well-tested)
- Cache uses struct fields (cachedRelease *ReleaseInfo + cacheTime time.Time) for simplicity and testability
- baseURL field allows httptest server injection without interface abstraction
- Asset matching uses HasSuffix for ZIP and Contains for checksums (per RESEARCH Pitfall 4)

## Deviations from Plan

None - plan executed exactly as written.

## Next Phase Readiness
- selfupdate package foundation ready for Plan 02 (Update method: download, checksum, ZIP extract, Apply)
- SelfUpdateConfig ready for config.Config embedding in Plan 02
- All types and methods follow the D-03 public API design from CONTEXT.md

## Self-Check: PASSED

- FOUND: internal/selfupdate/selfupdate.go
- FOUND: internal/selfupdate/selfupdate_test.go
- FOUND: .planning/phases/38-self-update-core/38-01-SUMMARY.md
- FOUND: cc7701b (feat commit)
- FOUND: 7cc5c9d (test commit)

---
*Phase: 38-self-update-core*
*Completed: 2026-03-30*

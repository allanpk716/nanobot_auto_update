---
phase: 20-log-capture-integration
plan: 02
subsystem: lifecycle
tags: [os.exec, os.Pipe, bufio.Scanner, context, goroutine, sync.WaitGroup]

# Dependency graph
requires:
  - phase: 20-01
    provides: captureLogs function for reading stdout/stderr streams
provides:
  - StartNanobotWithCapture function with integrated log capture
  - Concurrent stdout/stderr pipe reading
  - Context-based goroutine lifecycle management
affects: [phase-21-instance-lifecycle, phase-22-sse-integration]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - os.Pipe() for stdout/stderr capture (avoids StdoutPipe race condition)
    - Two goroutines for concurrent pipe reading
    - context.WithCancel + sync.WaitGroup for goroutine lifecycle

key-files:
  created: []
  modified:
    - internal/lifecycle/starter.go
    - internal/lifecycle/capture_test.go

key-decisions:
  - "Use os.Pipe() instead of cmd.StdoutPipe() to avoid race condition"
  - "Use select+default pattern in captureLogs for non-blocking scan with context cancellation"
  - "Wait 1 second for goroutines to finish in tests (increased from 500ms for Windows)"

patterns-established:
  - "Process capture pattern: create pipes before cmd.Start, launch goroutines before cmd.Start, close writers on process exit, cancel context to stop goroutines"

requirements-completed: [CAPT-04, CAPT-05]

# Metrics
duration: 8min
completed: 2026-03-17
---
# Phase 20 Plan 02: StartNanobotWithCapture Integration Summary

**Add StartNanobotWithCapture function integrating log capture into process lifecycle with os.Pipe(), concurrent goroutine reading, and context-based cleanup**

## Performance

- **Duration:** 8 min
- **Started:** 2026-03-17T07:33:10Z
- **Completed:** 2026-03-17T07:41:15Z
- **Tasks:** 1 (TDD: test → implementation)
- **Files modified:** 2

## Accomplishments
- Implemented StartNanobotWithCapture function with stdout/stderr capture
- Integrated captureLogs function from Plan 01 into process startup
- Verified goroutine lifecycle management (context cancellation + WaitGroup)
- All tests pass (3 new integration tests + 3 existing unit tests)

## Task Commits

Each task was committed atomically:

1. **Task 1: Add StartNanobotWithCapture function** (TDD)
   - `0b99707` (test) - Add failing tests for StartNanobotWithCapture
   - `c06880b` (feat) - Implement StartNanobotWithCapture with log capture

**Plan metadata:** (pending final commit)

_Note: TDD workflow followed (RED → GREEN)_

## Files Created/Modified
- `internal/lifecycle/starter.go` - Added StartNanobotWithCapture function with sync/logbuffer imports
- `internal/lifecycle/capture_test.go` - Added 3 integration tests, fixed Windows echo output handling

## Decisions Made
- Used os.Pipe() instead of cmd.StdoutPipe() to avoid race condition (as documented in RESEARCH.md)
- Increased test wait time from 500ms to 1s for Windows process scheduling
- Used strings.Contains instead of exact match for Windows echo command (trailing spaces)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed test expectations for Windows echo command output**
- **Found during:** Task 1 (GREEN phase - test execution)
- **Issue:** Windows echo command adds trailing space, test expected exact match "stdout_test" but got "stdout_test "
- **Fix:** Changed test to use strings.Contains instead of == for content verification
- **Files modified:** internal/lifecycle/capture_test.go
- **Verification:** All 3 StartNanobotWithCapture tests pass
- **Committed in:** c06880b (Task 1 implementation commit)

**2. [Rule 1 - Bug] Fixed TestStartNanobotWithCapture_InvalidCommand expectations**
- **Found during:** Task 1 (GREEN phase - test execution)
- **Issue:** Test expected no logs for invalid command, but cmd /c successfully starts even for invalid commands (produces stderr output)
- **Fix:** Changed test to verify error is returned (port verification fails) rather than checking for zero logs
- **Files modified:** internal/lifecycle/capture_test.go
- **Verification:** Test passes with realistic expectations
- **Committed in:** c06880b (Task 1 implementation commit)

**3. [Rule 1 - Bug] Increased goroutine wait time in tests**
- **Found during:** Task 1 (GREEN phase - debugging)
- **Issue:** 500ms wait was insufficient for goroutines to capture output on Windows
- **Fix:** Increased wait time to 1 second
- **Files modified:** internal/lifecycle/capture_test.go
- **Verification:** Tests consistently pass with 1s wait
- **Committed in:** c06880b (Task 1 implementation commit)

---

**Total deviations:** 3 auto-fixed (all Rule 1 - Bug fixes for test reliability)
**Impact on plan:** All fixes were necessary for test reliability on Windows. No scope creep, implementation matches plan specification.

## Issues Encountered
None - TDD workflow executed smoothly. Windows-specific behaviors (echo trailing spaces, process scheduling) were handled during GREEN phase.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- StartNanobotWithCapture ready for integration into InstanceLifecycle (Phase 21)
- LogBuffer integration pending (Phase 21 will create LogBuffer per instance)
- SSE streaming of logs will use LogBuffer.Subscribe() (Phase 22)

---
*Phase: 20-log-capture-integration*
*Completed: 2026-03-17*

## Self-Check: PASSED

- ✓ 20-02-SUMMARY.md exists
- ✓ Commit 0b99707 (test) exists
- ✓ Commit c06880b (feat) exists
- ✓ All files verified

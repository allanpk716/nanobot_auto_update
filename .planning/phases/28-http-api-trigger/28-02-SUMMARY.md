---
phase: 28-http-api-trigger
plan: 02
subsystem: instance-manager
tags:
  - api
  - concurrency
  - atomic
  - tdd
requires:
  - API-03
  - API-06
provides:
  - TriggerUpdate(ctx) (*UpdateResult, error)
  - IsUpdating() bool
  - ErrUpdateInProgress error
affects:
  - HTTP API update endpoint
  - Concurrent update handling
tech-stack:
  added:
    - sync/atomic.Bool
  patterns:
    - CompareAndSwap for atomic check-and-set
    - defer for guaranteed cleanup
    - TDD (test-driven development)
key-files:
  created: []
  modified:
    - internal/instance/manager.go
    - internal/instance/manager_test.go
decisions:
  - Use atomic.Bool instead of mutex for simple true/false state
  - Use CompareAndSwap(false, true) for atomic check-and-set
  - Use defer pattern to guarantee isUpdating flag reset
  - Use Chinese log messages to match project standards
metrics:
  duration: 8 minutes
  tasks_completed: 2
  files_modified: 2
  tests_added: 6
  test_pass_rate: 100%
---

# Phase 28 Plan 02: Concurrent Update Control Summary

## One-liner

Implemented thread-safe concurrent update control using atomic.Bool to prevent multiple simultaneous update operations on InstanceManager.

## Implementation Details

### Task 1: Write Tests for Concurrent Update Control (TDD RED)

Added comprehensive test suite covering all concurrent update scenarios:

1. **TestTriggerUpdate_Concurrent**: Verifies that calling TriggerUpdate during an ongoing update returns ErrUpdateInProgress
2. **TestTriggerUpdate_ResetsFlag**: Ensures isUpdating flag is reset after successful completion
3. **TestTriggerUpdate_ResetsFlagOnError**: Confirms flag reset even when errors occur
4. **TestTriggerUpdate_ContextCancellation**: Validates flag reset on context cancellation
5. **TestIsUpdating**: Tests the IsUpdating() method returns correct state
6. **TestTriggerUpdate_CallsUpdateAll**: Verifies TriggerUpdate internally calls UpdateAll

**Key Test Design Decisions:**
- Used empty instance lists to avoid real process management during tests
- Manually set isUpdating flag for concurrent testing instead of complex goroutine orchestration
- Ensured tests are fast and reliable without external dependencies

### Task 2: Implement Concurrent Update Control (TDD GREEN)

Added concurrent update control to InstanceManager:

**Changes to internal/instance/manager.go:**
1. Added `sync/atomic` import
2. Added package-level error variable:
   ```go
   var ErrUpdateInProgress = errors.New("update already in progress")
   ```
3. Modified InstanceManager struct:
   ```go
   type InstanceManager struct {
       instances  []*InstanceLifecycle
       logger     *slog.Logger
       isUpdating atomic.Bool // API-06: 并发控制标志
   }
   ```
4. Implemented TriggerUpdate method:
   - Uses `CompareAndSwap(false, true)` for atomic check-and-set
   - Returns `ErrUpdateInProgress` if update already running
   - Uses `defer m.isUpdating.Store(false)` to guarantee flag reset
   - Calls `UpdateAll(ctx)` to execute update flow
   - Logs update status in Chinese to match project standards
5. Implemented IsUpdating method:
   - Returns `m.isUpdating.Load()` to expose current state

**All tests pass:** `go test ./internal/instance/... -v -run TestTriggerUpdate`

## Verification Results

### Test Execution

```
=== RUN   TestTriggerUpdate_Concurrent
--- PASS: TestTriggerUpdate_Concurrent (0.01s)
=== RUN   TestTriggerUpdate_ResetsFlag
--- PASS: TestTriggerUpdate_ResetsFlag (0.01s)
=== RUN   TestTriggerUpdate_ResetsFlagOnError
--- PASS: TestTriggerUpdate_ResetsFlagOnError (0.01s)
=== RUN   TestTriggerUpdate_ContextCancellation
--- PASS: TestTriggerUpdate_ContextCancellation (0.01s)
=== RUN   TestIsUpdating
--- PASS: TestIsUpdating (0.00s)
=== RUN   TestTriggerUpdate_CallsUpdateAll
--- PASS: TestTriggerUpdate_CallsUpdateAll (1.55s)
PASS
ok      github.com/HQGroup/nanobot-auto-updater/internal/instance    5.529s
```

### All Instance Package Tests

All existing tests continue to pass:
```
PASS
ok      github.com/HQGroup/nanobot-auto-updater/internal/instance    27.930s
```

## Key Implementation Patterns

### Atomic Compare-And-Swap Pattern

```go
func (m *InstanceManager) TriggerUpdate(ctx context.Context) (*UpdateResult, error) {
    // Atomically check if false and set to true
    if !m.isUpdating.CompareAndSwap(false, true) {
        m.logger.Warn("更新请求被拒绝: 更新正在进行中")
        return nil, ErrUpdateInProgress
    }
    // Guaranteed cleanup on any return path
    defer m.isUpdating.Store(false)

    // ... update logic ...
}
```

This pattern ensures:
1. Only one update can run at a time (atomic check-and-set)
2. Flag is always reset (defer pattern)
3. No race conditions (atomic operations)

## Requirements Satisfied

**API-03:** TriggerUpdate method executes stop->update->start flow
- TriggerUpdate calls UpdateAll(ctx) internally
- UpdateAll implements the complete update lifecycle

**API-06:** Concurrent update control prevents simultaneous operations
- atomic.Bool provides thread-safe state management
- CompareAndSwap ensures atomic check-and-set
- ErrUpdateInProgress returned when update already running
- isUpdating flag always reset after completion (defer)

## Deviations from Plan

**Test Design Adjustment:**
- Plan suggested using goroutines with channels to simulate concurrent updates
- Actual implementation used simpler approach: manually setting isUpdating flag
- Rationale: More reliable, faster tests without complex synchronization
- All test objectives achieved with simpler design

**Port Selection:**
- Tests use high port numbers (9994-9999) to avoid conflicts with real instances
- Some tests use empty instance lists to avoid real process management

## Known Stubs

None - all functionality is fully implemented and tested.

## Self-Check: PASSED

All implementation claims verified:
- Files exist: internal/instance/manager.go, internal/instance/manager_test.go
- Commit exists: 4dde0bd
- isUpdating atomic.Bool field added to InstanceManager
- ErrUpdateInProgress error variable defined
- TriggerUpdate(ctx) method implemented
- IsUpdating() method implemented
- All tests passing (6/6 tests, 100% pass rate)

## Next Steps

Plan 28-03 will integrate TriggerUpdate into the HTTP API endpoint, providing:
- POST /api/v1/updates endpoint
- Bearer token authentication
- JSON response with UpdateResult
- Integration with this concurrent update control

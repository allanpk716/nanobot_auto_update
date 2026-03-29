# Phase 35: Notification Integration Testing - Research

**Researched:** 2026-03-29
**Domain:** Go testing, interface-based mocking, E2E integration testing
**Confidence:** HIGH

## Summary

Phase 35 is a pure testing phase that validates the notification integration implemented in Phase 34. The core challenge is testing asynchronous goroutine-based notifications without flaky timing. The CONTEXT.md decisions are clear and well-scoped: refactor `*notifier.Notifier` concrete type to a `Notifier` interface in `trigger.go`, create a `recordingNotifier` mock, and add 4 E2E tests to `integration_test.go`. The existing Phase 33 E2E test pattern provides a proven template to follow.

**Primary recommendation:** Define a minimal `Notifier` interface with `Notify(title, message string) error` in `trigger.go`, create a `recordingNotifier` mock with `sync.Mutex`-protected call recording and configurable error behavior, and use a small `time.Sleep` (50ms) for goroutine synchronization in tests. This follows the established `mockTriggerUpdater` pattern exactly.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01: Refactor Notifier to interface** -- Define `Notifier` interface in trigger.go with `Notify(title, message string) error`; change `TriggerHandler.notifier` from `*notifier.Notifier` to interface; update server.go parameter type; main.go unchanged (duck typing)
- **D-02: Recording mock structure** -- `recordingNotifier` mock with calls slice, `Calls()` / `CallCount()` methods, configurable `shouldError bool`
- **D-03: 4 E2E tests** -- Each maps to one success criterion: start notification, completion notification, non-blocking failure, graceful degradation
- **D-04: Follow Phase 33 integration test pattern** -- Append to `integration_test.go`, use `t.TempDir()`, reuse `mockTriggerUpdater`, use `httptest.NewRequest` + `httptest.NewRecorder`
- **D-05: Mock returns error for failure simulation** -- `shouldError=true` returns `fmt.Errorf("simulated pushover failure")`, no fake HTTP server needed

### Claude's Discretion
- Interface definition location (trigger.go or separate file)
- recordingNotifier implementation details
- Test internal structure (table-driven vs independent functions)
- Helper test function naming and placement
- Assertion approach for notification content (exact match vs keyword containment)

### Deferred Ideas (OUT OF SCOPE)
None
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| UNOTIF-01 | Update Start Notification -- sent before TriggerUpdate with trigger source and instance count | Interface refactor enables injecting recordingNotifier; test asserts call[0].title contains "更新开始" and message contains "api-trigger" + instance count |
| UNOTIF-02 | Update Completion Notification -- sent after update with status, elapsed time, instance details | Test asserts call[1].title maps to status (success/partial/failed), message contains elapsed time and instance counts |
| UNOTIF-03 | Non-blocking Notification -- Pushover failure does not affect API response, body, or UpdateLog | recordingNotifier with shouldError=true; test verifies HTTP 200, correct JSON body, and JSONL file contains record |
| UNOTIF-04 | Graceful Degradation -- disabled/not-configured Notifier = zero notification attempts, no errors | nil notifier passed to handler; test verifies zero calls and normal HTTP response |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go testing | 1.24.11 | Built-in test framework | Standard library, no external dependency |
| net/http/httptest | stdlib | HTTP test utilities | Standard for Go HTTP handler testing |
| sync | stdlib | Mutex for goroutine-safe mock state | Protects recordingNotifier.Calls from concurrent access |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| encoding/json | stdlib | Decode/encode HTTP response bodies | Every E2E test validates JSON response |
| os / path/filepath | stdlib | Temp file management for JSONL | t.TempDir() pattern for UpdateLog persistence tests |
| strings | stdlib | Substring assertion on notification messages | Contains-based assertions for notification content |

### No External Dependencies
This phase requires zero new packages. Everything uses Go standard library.

## Architecture Patterns

### Pattern 1: Interface Extraction for Testability
**What:** Extract a minimal interface from a concrete type to enable mock injection.
**When to use:** When a component depends on a concrete type that makes external calls (network I/O).
**Example:**
```go
// In trigger.go -- the interface replaces *notifier.Notifier concrete type
type Notifier interface {
    Notify(title, message string) error
}
```
The concrete `notifier.Notifier` struct already has `Notify(title, message string) error` -- duck typing means it automatically satisfies this interface. No changes to `notifier/notifier.go` needed.

### Pattern 2: Recording Mock with Mutex Protection
**What:** A mock that records all method calls with goroutine-safe access, plus configurable error behavior.
**When to use:** Testing async goroutine code where the mock is called from a different goroutine.
**Example:**
```go
type recordingNotifier struct {
    mu          sync.Mutex
    calls       []NotifyCall
    shouldError bool
}

type NotifyCall struct {
    Title   string
    Message string
}

func (r *recordingNotifier) Notify(title, message string) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.calls = append(r.calls, NotifyCall{Title: title, Message: message})
    if r.shouldError {
        return fmt.Errorf("simulated pushover failure")
    }
    return nil
}

func (r *recordingNotifier) Calls() []NotifyCall {
    r.mu.Lock()
    defer r.mu.Unlock()
    return append([]NotifyCall(nil), r.calls...)
}

func (r *recordingNotifier) CallCount() int {
    r.mu.Lock()
    defer r.mu.Unlock()
    return len(r.calls)
}
```
The `sync.Mutex` is critical because `trigger.go` sends notifications in goroutines -- the test goroutine and notification goroutine access `calls` concurrently.

### Pattern 3: Goroutine Synchronization in Tests
**What:** Use a small `time.Sleep` after the HTTP handler returns to allow notification goroutines to complete.
**When to use:** Testing async goroutine side effects where channel-based synchronization would require production code changes.
**Example:**
```go
// After handler returns, goroutines may still be running
handler.Handle(rec, req)
// Wait for async notification goroutines to finish
time.Sleep(50 * time.Millisecond)
// Now safe to check mock state
if mock.CallCount() != 2 { ... }
```
50ms is generous enough to avoid flakiness on slow CI, short enough to not slow down the test suite. This is the same approach mentioned in CONTEXT.md "specifics" section.

### Anti-Patterns to Avoid
- **Race condition in mock:** Not protecting mock state with `sync.Mutex` will cause `go test -race` to fail. The notification goroutines write to mock state concurrently with the test goroutine reading it.
- **No sleep after handler:** Checking mock state immediately after `handler.Handle()` returns misses goroutine-based notifications. The handler returns before goroutines complete.
- **Asserting exact notification message content:** Notification messages contain dynamic values (elapsed time formatted to 1 decimal). Use `strings.Contains` for key phrases, not exact equality.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| HTTP request simulation | Real HTTP server | `httptest.NewRequest` + `httptest.NewRecorder` | No port binding, no network, deterministic |
| Temp file management | Manual cleanup | `t.TempDir()` | Auto-cleaned, unique per test, no collision |
| Mock framework | testify/mock or gomock | Simple struct implementing interface | Zero dependency, matches mockTriggerUpdater pattern |
| Async synchronization | Channels or WaitGroups injected into production code | `time.Sleep(50ms)` | No production code changes needed for testing |

## Common Pitfalls

### Pitfall 1: Race Condition in Recording Mock
**What goes wrong:** `go test -race` fails because the notification goroutine writes to `calls` slice while the test goroutine reads it.
**Why it happens:** The mock's `calls` field is accessed from multiple goroutines without synchronization.
**How to avoid:** Wrap all mock state access with `sync.Mutex`. Use `Lock()/Unlock()` in `Notify()`, `Calls()`, and `CallCount()`.
**Warning signs:** `go test -race ./internal/api/` reports DATA RACE.

### Pitfall 2: Timing-Dependent Test Flakiness
**What goes wrong:** Tests pass locally but fail on CI because goroutines haven't finished executing when assertions run.
**Why it happens:** No explicit synchronization between test goroutine and notification goroutine.
**How to avoid:** `time.Sleep(50 * time.Millisecond)` after `handler.Handle()` returns. 50ms is generous for a goroutine that only appends to a slice.
**Warning signs:** Tests fail intermittently, especially on slow or loaded machines.

### Pitfall 3: Interface Type Mismatch Compilation Errors
**What goes wrong:** After defining the interface in `trigger.go`, compilation fails in `server.go` or test files.
**Why it happens:** `NewServer` and `NewTriggerHandler` still reference `*notifier.Notifier` in their signatures.
**How to avoid:** Update all function signatures that pass the notifier: `NewTriggerHandler` (trigger.go), `NewServer` (server.go), `newTestHandler` (trigger_test.go). The concrete `notifier.Notifier` struct auto-satisfies the interface, so main.go and callers passing concrete types need no changes.
**Warning signs:** Compiler error: "cannot use notif (type *notifier.Notifier) as type Notifier in argument to..."

### Pitfall 4: Over-Asserting Notification Content
**What goes wrong:** Test asserts exact message text and breaks when format changes or elapsed time varies.
**Why it happens:** Notification messages contain dynamic values (e.g., `elapsed: 0.0s` vs `elapsed: 0.1s`).
**How to avoid:** Use `strings.Contains` for key phrases like "api-trigger", "Nanobot", "成功", "失败". Don't assert exact elapsed time values -- only verify the field exists.
**Warning signs:** Tests break on different machines due to timing differences.

### Pitfall 5: Missing Test File Updates After Interface Change
**What goes wrong:** Existing tests in `trigger_test.go`, `integration_test.go`, and `server_test.go` fail to compile because they pass `*notifier.Notifier` or `nil` where interface is expected.
**Why it happens:** The CONTEXT.md and Phase 34 SUMMARY both document this pattern -- multiple files need signature updates when the interface changes.
**How to avoid:** Phase 34 encountered this same issue (Deviation #2 in 34-01-SUMMARY.md). Proactively check: `newTestHandler` in trigger_test.go, all `NewTriggerHandler` and `NewServer` calls in test files. Nil is valid for interface types, so existing nil-passing code should compile fine, but the `notifier.NewWithConfig()` return type needs attention.
**Warning signs:** Compilation errors in test files after interface refactor.

## Code Examples

### Interface Definition (in trigger.go)
```go
// Source: CONTEXT.md D-01 decision
// Notifier is the interface for sending update notifications.
// Defined here to enable mock injection for testing.
type Notifier interface {
    Notify(title, message string) error
}
```

### Updated TriggerHandler Struct (in trigger.go)
```go
// Before: notifier *notifier.Notifier
// After:  notifier Notifier (interface type)
type TriggerHandler struct {
    instanceManager TriggerUpdater
    config          *config.APIConfig
    logger          *slog.Logger
    updateLogger    *updatelog.UpdateLogger
    notifier        Notifier              // Changed from *notifier.Notifier
    instanceCount   int
}
```

### Updated NewTriggerHandler Signature (in trigger.go)
```go
// Before: func NewTriggerHandler(im TriggerUpdater, cfg *config.APIConfig, logger *slog.Logger, ul *updatelog.UpdateLogger, notif *notifier.Notifier, instanceCount int) *TriggerHandler
// After:
func NewTriggerHandler(im TriggerUpdater, cfg *config.APIConfig, logger *slog.Logger, ul *updatelog.UpdateLogger, notif Notifier, instanceCount int) *TriggerHandler {
```

### Updated NewServer Signature (in server.go)
```go
// Before: ... notif *notifier.Notifier) (*Server, error)
// After:
func NewServer(cfg *config.APIConfig, im *instance.InstanceManager, fullCfg *config.Config, version string, logger *slog.Logger, updateLogger *updatelog.UpdateLogger, notif Notifier) (*Server, error) {
```
Note: The `Notifier` interface must be importable. Since it's defined in the `api` package, `server.go` (same package) can use it directly. `main.go` passes `*notifier.Notifier` which satisfies the interface via duck typing.

### E2E Test Template (in integration_test.go)
```go
// Source: Phase 33 E2E test pattern + CONTEXT.md D-03
func TestE2E_Notification_StartNotification(t *testing.T) {
    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
    tmpDir := t.TempDir()
    jsonlPath := filepath.Join(tmpDir, "updates.jsonl")

    ul := updatelog.NewUpdateLogger(logger, jsonlPath)
    defer ul.Close()

    mock := &mockTriggerUpdater{
        result: &instance.UpdateResult{
            Stopped:     []string{"gateway"},
            Started:     []string{"gateway"},
            StopFailed:  []*instance.InstanceError{},
            StartFailed: []*instance.InstanceError{},
        },
    }

    // Inject recording notifier instead of real *notifier.Notifier
    recordingNotif := &recordingNotifier{}

    // newTestHandler signature needs update to accept Notifier interface
    triggerHandler := newTestHandler(logger, ul, mock, recordingNotif)

    req := httptest.NewRequest("POST", "/api/v1/trigger-update", nil)
    rec := httptest.NewRecorder()

    triggerHandler.Handle(rec, req)

    // Wait for async notification goroutines
    time.Sleep(50 * time.Millisecond)

    // Verify start notification was sent
    calls := recordingNotif.Calls()
    if len(calls) < 1 {
        t.Fatalf("Expected at least 1 notification call, got %d", len(calls))
    }

    startCall := calls[0]
    if !strings.Contains(startCall.Title, "更新开始") {
        t.Errorf("Start notification title = %q, want to contain '更新开始'", startCall.Title)
    }
    if !strings.Contains(startCall.Message, "api-trigger") {
        t.Errorf("Start notification message should contain 'api-trigger', got: %s", startCall.Message)
    }
    if !strings.Contains(startCall.Message, "3") { // instanceCount=3 from newTestHandler
        t.Errorf("Start notification message should contain instance count, got: %s", startCall.Message)
    }
}
```

### newTestHandler Update Required
```go
// Current signature in trigger_test.go:
func newTestHandler(logger *slog.Logger, ul *updatelog.UpdateLogger, mock *mockTriggerUpdater, notif *notifier.Notifier) *TriggerHandler {
    // ...
    return NewTriggerHandler(mock, cfg, logger, ul, notif, 3)
}

// Updated signature (accept interface):
func newTestHandler(logger *slog.Logger, ul *updatelog.UpdateLogger, mock *mockTriggerUpdater, notif Notifier) *TriggerHandler {
    // ...
    return NewTriggerHandler(mock, cfg, logger, ul, notif, 3)
}
```
This is the key change that enables both existing tests (passing `nil` or `*notifier.Notifier`) and new E2E tests (passing `*recordingNotifier`) to work.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `*notifier.Notifier` concrete dependency | Interface-based dependency | Phase 35 (this phase) | Enables mock injection for E2E testing |

**No deprecated patterns.** This is a greenfield testing phase building on established patterns.

## Open Questions

1. **Interface definition location: trigger.go vs separate file?**
   - What we know: CONTEXT.md D-01 says "in trigger.go", Claude's Discretion allows flexibility
   - Recommendation: Define in `trigger.go` -- it's the consumer, single method, co-located with usage. No need for a separate file for a one-method interface.

2. **Sleep duration for goroutine synchronization?**
   - What we know: 50ms is mentioned as guidance
   - Recommendation: 50ms is appropriate. Notification goroutines do minimal work (append to slice + return). On any reasonable system they complete in <1ms. 50ms provides 50x safety margin.

3. **Should `newTestHandler` change its parameter type from `*notifier.Notifier` to `Notifier`?**
   - What we know: This helper is in `trigger_test.go` and used by both unit tests and E2E tests
   - Recommendation: Yes, must change. Existing callers pass `nil` (valid for interface) or `notifier.NewWithConfig(...)` (returns `*notifier.Notifier` which satisfies interface). No callers break.

## Environment Availability

Step 2.6: SKIPPED (no external dependencies identified -- pure Go standard library testing, no services or tools beyond Go compiler and test runner)

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) v1.24.11 |
| Config file | none |
| Quick run command | `go test ./internal/api/ -count=1 -run "TestE2E_Notification" -v` |
| Full suite command | `go test ./internal/api/ -count=1 -v` |
| Race detection command | `go test ./internal/api/ -race -count=1 -v` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| UNOTIF-01 | Start notification sent before TriggerUpdate with trigger source and instance count | E2E integration | `go test ./internal/api/ -run TestE2E_Notification_StartNotification -v` | Wave 0 (new) |
| UNOTIF-02 | Completion notification sent after update with status, elapsed, instance details | E2E integration | `go test ./internal/api/ -run TestE2E_Notification_CompletionNotification -v` | Wave 0 (new) |
| UNOTIF-03 | Pushover failure does not affect API response or UpdateLog | E2E integration | `go test ./internal/api/ -run TestE2E_Notification_NonBlocking -v` | Wave 0 (new) |
| UNOTIF-04 | Nil notifier = zero calls, normal response | E2E integration | `go test ./internal/api/ -run TestE2E_Notification_GracefulDegradation -v` | Wave 0 (new) |

### Sampling Rate
- **Per task commit:** `go test ./internal/api/ -count=1 -v`
- **Per wave merge:** `go test ./internal/api/ -race -count=1 -v`
- **Phase gate:** Full suite with race detection green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/api/integration_test.go` -- add 4 new TestE2E_Notification_* functions + recordingNotifier mock
- [ ] `internal/api/trigger.go` -- add Notifier interface definition, update TriggerHandler field type and NewTriggerHandler parameter type
- [ ] `internal/api/server.go` -- update NewServer parameter type from `*notifier.Notifier` to `Notifier`
- [ ] `internal/api/trigger_test.go` -- update newTestHandler parameter type from `*notifier.Notifier` to `Notifier`
- [ ] `internal/api/server_test.go` -- no changes needed (passes `nil` which is valid for interface type)

## Key Code Locations

### Files Requiring Interface Refactor
These files must change to support the interface extraction:

1. **`internal/api/trigger.go`** (lines 35-36, 40):
   - Add `Notifier` interface definition (after `TriggerUpdater` interface)
   - Change `notifier *notifier.Notifier` field to `notifier Notifier`
   - Change `NewTriggerHandler` parameter from `*notifier.Notifier` to `Notifier`
   - Remove `"github.com/HQGroup/nanobot-auto-updater/internal/notifier"` import (no longer directly referenced by type)

2. **`internal/api/server.go`** (line 28):
   - Change `notif *notifier.Notifier` parameter to `notif Notifier` in `NewServer`
   - Remove `"github.com/HQGroup/nanobot-auto-updater/internal/notifier"` import

3. **`internal/api/trigger_test.go`** (line 32):
   - Change `notif *notifier.Notifier` parameter to `notif Notifier` in `newTestHandler`
   - Remove `"github.com/HQGroup/nanobot-auto-updater/internal/notifier"` import from trigger_test.go

### Files Where New Code Is Added

4. **`internal/api/integration_test.go`** (append after line 392):
   - Add `recordingNotifier` struct with `sync.Mutex`, `calls []NotifyCall`, `shouldError bool`
   - Add 4 E2E test functions

### Files That Need No Changes
- `cmd/nanobot-auto-updater/main.go` -- passes `*notifier.Notifier` concrete type which satisfies interface (duck typing)
- `internal/notifier/notifier.go` -- no changes to concrete implementation
- `internal/api/server_test.go` -- passes `nil` for notifier, which is valid for interface type

## Sources

### Primary (HIGH confidence)
- Source code analysis of `internal/api/trigger.go`, `trigger_test.go`, `integration_test.go`, `server.go`, `notifier.go`, `main.go`
- `.planning/phases/35-notification-integration-testing/35-CONTEXT.md` -- user decisions
- `.planning/phases/34-update-notification-integration/34-01-SUMMARY.md` -- Phase 34 implementation details
- `.planning/REQUIREMENTS.md` -- UNOTIF-01 through UNOTIF-04 acceptance criteria

### Secondary (MEDIUM confidence)
- Phase 33 E2E test pattern (established and proven in integration_test.go)
- Phase 34 deviation history (documents the cascading signature update issue)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- zero new dependencies, all Go standard library
- Architecture: HIGH -- follows established mockTriggerUpdater pattern exactly
- Pitfalls: HIGH -- all pitfalls identified from actual code analysis (goroutine concurrency, interface cascading changes, timing)
- Interface refactor scope: HIGH -- enumerated every file and line that needs change

**Research date:** 2026-03-29
**Valid until:** 2026-04-29 (stable -- no fast-moving dependencies)

# Phase 42: Telegram Monitor Core - Research

**Researched:** 2026-04-06
**Domain:** Log-pattern detection, timeout state machine, Pushover notification integration in Go
**Confidence:** HIGH

## Summary

Phase 42 builds a self-contained `internal/telegram` package that subscribes to an instance's `LogBuffer`, detects the "Starting Telegram bot" trigger pattern, manages a 30-second timeout state machine, and sends Pushover notifications on success or failure. The package has zero coupling to `main.go` or any other integration point -- it depends only on the `LogBuffer` subscriber interface (for log input) and the `Notifier` interface (for notification output). Both are satisfied via duck typing by existing types.

The core design is a single goroutine that reads from the subscriber channel, transitions through states (idle -> waiting -> resolved), and uses `time.AfterFunc` for the 30-second timeout. Historical log replay from `LogBuffer.Subscribe()` must be handled by filtering on a `startTime` timestamp to prevent false triggers. The `time.AfterFunc` callback runs in its own goroutine and MUST access shared state under a mutex, following the exact pattern from `internal/notification/manager.go:confirmAndNotify()`.

**Primary recommendation:** Build a pure-logic package with `time.AfterFunc` state machine, inject `LogSubscriber` and `Notifier` interfaces for full testability via mocks. No new dependencies. Patterns are already established in the codebase.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TELE-01 | Detect "Starting Telegram bot" in logs to auto-trigger connection monitoring | LogBuffer.Subscribe() provides real-time LogEntry stream. strings.Contains() on Content field detects trigger. See "Architecture Patterns > State Machine". |
| TELE-02 | Detect "Telegram bot commands registered" within 30s to determine success | Same string matching on Content field. Timer.Stop() cancels timeout on success. See "Architecture Patterns > State Machine". |
| TELE-03 | Detect "httpx.ConnectError" in logs to determine connection failure | Same string matching. Immediate failure notification, timer cancelled. See "Architecture Patterns > State Machine". |
| TELE-04 | 30-second timeout with no success/failure pattern = timeout failure | time.AfterFunc(30s, callback) with mutex-protected state. Pattern from notification/manager.go. See "Architecture Patterns > State Machine". |
| TELE-05 | Telegram connection success sends Pushover notification | Notifier interface Notify() method. Optional: send success notification only if desired. See "Notifier Interface" section. |
| TELE-06 | Telegram connection failure sends Pushover notification | Same Notifier.Notify() path for failure and timeout. See "Notifier Interface" section. |
| TELE-08 | Historical log replay does not trigger false notifications | Filter LogEntry by timestamp: ignore entries with Timestamp.Before(monitorStartTime). See "Common Pitfalls > Pitfall 1". |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `strings` | Go 1.24+ | strings.Contains() for pattern matching | Already used throughout codebase. Fixed-string matching is sufficient for "Starting Telegram bot", "Telegram bot commands registered", "httpx.ConnectError". No regex needed. [VERIFIED: codebase grep shows 50+ uses of strings.Contains] |
| Go stdlib `time` | Go 1.24+ | time.AfterFunc() for 30s timeout | Already used in notification/manager.go:120 for cooldown timer. AfterFunc runs callback in its own goroutine, cancellable via Timer.Stop(). [VERIFIED: internal/notification/manager.go line 120] |
| Go stdlib `context` | Go 1.24+ | Goroutine lifecycle control | context.WithCancel() for monitor Stop() method. Used throughout codebase in logbuffer/subscriber.go, notification/manager.go, lifecycle/starter.go. [VERIFIED: subscriber.go line 18] |
| Go stdlib `sync` | Go 1.24+ | Mutex for state protection | sync.Mutex to protect monitor state from AfterFunc callback goroutine. Pattern from notification/manager.go. [VERIFIED: internal/notification/manager.go line 8] |
| Go stdlib `log/slog` | Go 1.24+ | Structured logging | All packages use slog.Logger with component field. [VERIFIED: every internal package] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/stretchr/testify` | v1.11.1 | Test assertions (assert.Equal, etc.) | Already in go.mod. Used in health/monitor_test.go, api tests. [VERIFIED: go.mod line 13] |
| Existing `internal/logbuffer` | Phase 19 | LogBuffer subscriber channel | Subscribe() returns <-chan LogEntry with capacity 100. Unsubscribe() cleans up. [VERIFIED: logbuffer/subscriber.go] |
| Existing `internal/notifier` | Phase 27 | Pushover notification delivery | Notifier interface: IsEnabled() + Notify(title, message). [VERIFIED: notifier/notifier.go] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| time.AfterFunc | context.WithTimeout + select goroutine | AfterFunc is simpler for "start timer, cancel on event" pattern. WithTimeout is better if external cancellation needed. For this phase, both work. AfterFunc chosen because it matches the pattern in notification/manager.go and is simpler. |
| strings.Contains | regexp.MatchString | Regex adds compile overhead for zero benefit -- all three patterns are fixed literal strings. |
| New internal/telegram package | Add to existing notification/manager.go | Telegram monitoring is a distinct concern from network connectivity monitoring. Separate package keeps concerns isolated. |

**Installation:**
```bash
# No new dependencies needed
go mod tidy  # No changes expected
```

## Architecture Patterns

### Recommended Project Structure
```
internal/telegram/
  monitor.go        -- TelegramMonitor struct, Start(), Stop(), state machine
  monitor_test.go   -- Unit tests with mock LogBuffer and mock Notifier
  patterns.go       -- Pattern constants and matching functions (testable in isolation)
  patterns_test.go  -- Pattern matching unit tests
```

### Pattern 1: Duck-Typed Interfaces for Testability
**What:** Define interfaces in the telegram package that match existing types' method signatures.
**When to use:** When the monitor needs to interact with LogBuffer and Notifier but should be testable in isolation.
**Example:**
```go
// Source: [ASSUMED] following existing codebase pattern from notification/manager.go and api/trigger.go

// LogSubscriber is satisfied by *logbuffer.LogBuffer via duck typing.
type LogSubscriber interface {
    Subscribe() <-chan logbuffer.LogEntry
    Unsubscribe(ch <-chan logbuffer.LogEntry)
}

// Notifier is satisfied by *notifier.Notifier via duck typing.
type Notifier interface {
    IsEnabled() bool
    Notify(title, message string) error
}
```
[VERIFIED: Same pattern used in internal/notification/manager.go lines 19-21 and internal/api/trigger.go]

### Pattern 2: time.AfterFunc State Machine with Mutex
**What:** Single goroutine reads from subscriber channel, transitions state, uses AfterFunc for timeout. AfterFunc callback accesses state under mutex.
**When to use:** For time-bounded monitoring with event-driven transitions.
**Example:**
```go
// Source: [VERIFIED] pattern from internal/notification/manager.go:confirmAndNotify()

type monitorState int

const (
    stateIdle monitorState = iota
    stateWaiting
    stateResolved
)

type TelegramMonitor struct {
    mu           sync.Mutex
    state        monitorState
    timer        *time.Timer
    logBuffer    LogSubscriber
    notifier     Notifier
    instanceName string
    timeout      time.Duration
    startTime    time.Time       // TELE-08: filter historical entries
    logger       *slog.Logger
    ctx          context.Context
    cancel       context.CancelFunc
}

func (m *TelegramMonitor) Start() {
    ch := m.logBuffer.Subscribe()
    defer m.logBuffer.Unsubscribe(ch)

    m.startTime = time.Now() // TELE-08: only process entries after this

    for {
        select {
        case entry, ok := <-ch:
            if !ok {
                return
            }
            m.processEntry(entry)
        case <-m.ctx.Done():
            m.mu.Lock()
            if m.timer != nil {
                m.timer.Stop()
            }
            m.mu.Unlock()
            return
        }
    }
}

func (m *TelegramMonitor) processEntry(entry logbuffer.LogEntry) {
    // TELE-08: Ignore historical entries written before subscription
    if entry.Timestamp.Before(m.startTime) {
        return
    }

    m.mu.Lock()
    defer m.mu.Unlock()

    switch m.state {
    case stateIdle:
        if IsTrigger(entry.Content) { // TELE-01
            m.state = stateWaiting
            m.startTimer()
        }
    case stateWaiting:
        if IsSuccess(entry.Content) { // TELE-02
            m.timer.Stop()
            m.state = stateResolved
            // TELE-05: send success notification (optional)
        } else if IsFailure(entry.Content) { // TELE-03
            m.timer.Stop()
            m.state = stateResolved
            go m.sendNotification("Telegram connection failed", ...)
        }
    }
}

func (m *TelegramMonitor) startTimer() {
    if m.timer != nil {
        m.timer.Stop()
    }
    // AfterFunc runs callback in a NEW goroutine -- MUST use mutex
    m.timer = time.AfterFunc(m.timeout, func() {
        m.mu.Lock()
        defer m.mu.Unlock()

        if m.state != stateWaiting {
            return // Already resolved, ignore stale timeout
        }
        m.state = stateResolved
        // TELE-04: timeout notification
        go m.sendNotification("Telegram connection timeout", ...)
    })
}
```
[VERIFIED: AfterFunc-goroutine behavior from Go docs, mutex pattern from internal/notification/manager.go:135-152]

### Pattern 3: Timestamp Filter for Historical Replay Prevention (TELE-08)
**What:** Record time.Time when monitor starts, ignore LogEntry with Timestamp.Before(startTime).
**When to use:** LogBuffer.Subscribe() replays all buffered history before real-time entries.
**Why:** The subscriberLoop in subscriber.go sends GetHistory() entries first. Without filtering, a monitor created after the instance already logged "Starting Telegram bot" would false-trigger.
**Example:**
```go
// In Start(), before entering the read loop:
m.startTime = time.Now()

// In processEntry():
if entry.Timestamp.Before(m.startTime) {
    return // TELE-08: skip historical entry
}
```
[VERIFIED: LogBuffer.Subscribe() history replay confirmed in internal/logbuffer/subscriber.go:31-41]

### Anti-Patterns to Avoid
- **Blocking notification send in the channel read loop:** Pushover HTTP call can take seconds. If it blocks the subscriber goroutine, the channel (capacity 100) fills up and entries get dropped. Send notifications in a separate goroutine with panic recovery.
- **Polling LogBuffer.GetHistory() instead of subscribing:** Polling is wasteful (scans all 5000 entries repeatedly) and has latency gaps. Use Subscribe() for event-driven real-time delivery.
- **Creating new TelegramMonitor on every instance restart:** LogBuffer survives restart (Clear() does not affect subscribers). A single monitor per instance, created once, is sufficient. Phase 43 handles lifecycle.
- **Accessing state from AfterFunc without mutex:** AfterFunc runs in a NEW goroutine. The main goroutine and AfterFunc goroutine both read/write state. Without mutex, data race. Follow notification/manager.go pattern.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Real-time log stream consumption | Polling loop with GetHistory() | LogBuffer.Subscribe() returning <-chan LogEntry | LogBuffer already has non-blocking fan-out with capacity 100. Polling is wasteful and has latency. |
| Timeout management | Custom goroutine with time.Sleep | time.AfterFunc(30s, callback) | AfterFunc is cancellable via Timer.Stop(), runs in its own goroutine, and matches the codebase pattern. |
| Notification delivery | Direct HTTP client to Pushover | Existing Notifier.Notify(title, message) | Notifier handles enabled/disabled, API formatting, error handling. No duplication. |
| Log pattern matching | Regex engine or custom parser | strings.Contains(line, pattern) | All three patterns are fixed literal strings. regex adds compile overhead for zero benefit. |

**Key insight:** This phase has zero new dependencies. Everything uses Go stdlib + existing internal packages.

## Common Pitfalls

### Pitfall 1: Historical Log Replay Causes False Triggers (TELE-08)
**What goes wrong:** LogBuffer.Subscribe() replays ALL buffered history first. If the instance already logged "Starting Telegram bot" before the monitor subscribed, the monitor false-triggers and starts a 30-second countdown for a connection that already succeeded/failed.
**Why it happens:** LogBuffer.subscriberLoop() calls GetHistory() and sends all entries before entering real-time mode.
**How to avoid:** Record startTime = time.Now() at subscription. Ignore any LogEntry with Timestamp.Before(startTime). This is a hard requirement (TELE-08).
**Warning signs:** Spurious "Telegram connection timeout" notification 30 seconds after application restart.

### Pitfall 2: AfterFunc Callback Data Race
**What goes wrong:** time.AfterFunc runs its callback in a NEW goroutine. If the main goroutine's processEntry() and the AfterFunc callback both access monitor state without synchronization, Go race detector reports data race.
**Why it happens:** AfterFunc documentation states "When the Timer expires, it calls f in its own goroutine." Both goroutines access `m.state` and `m.timer`.
**How to avoid:** All state access must be under sync.Mutex. Lock at start of processEntry and AfterFunc callback. Check state is still relevant before acting (stale callback guard).
**Warning signs:** `go test -race` reports data race. Intermittent nil pointer dereference in timeout callback.

### Pitfall 3: Blocking Notification Send in Channel Read Loop
**What goes wrong:** Calling notifier.Notify() synchronously inside the `for entry := range ch` loop. The Pushover HTTP call takes 1-5 seconds. During this time, the subscriber channel (capacity 100) fills up with new log entries. LogBuffer starts dropping entries for this subscriber. If "Telegram bot commands registered" is dropped, the monitor never sees it and falsely reports timeout.
**Why it happens:** Notifier.Notify() makes an HTTP call to Pushover API when enabled.
**How to avoid:** Send notifications in a separate goroutine: `go func() { defer recover(); notifier.Notify(title, msg) }()`. Same pattern from notification/manager.go:sendNotification().
**Warning signs:** LogBuffer WARN logs "Subscriber channel full, dropping log". Telegram monitor reports timeout even though success pattern appeared in nanobot output.

### Pitfall 4: Timer Not Stopped on State Resolution
**What goes wrong:** When success or failure pattern is detected, the timer is not stopped. After 30 seconds, the timer fires and sends a second (spurious) notification because the stale callback was not cancelled.
**Why it happens:** Forgetting to call timer.Stop() in the success/failure path.
**How to avoid:** Always stop the timer when transitioning out of "waiting" state. In processEntry(), when IsSuccess or IsFailure returns true, call m.timer.Stop() before changing state.
**Warning signs:** Two notifications for the same Telegram connection attempt (e.g., "success" then 30 seconds later "timeout").

### Pitfall 5: Multiple Triggers Without Cancelling Previous Timer
**What goes wrong:** Nanobot outputs "Starting Telegram bot" twice (e.g., during retry). The first trigger starts timer1. The second trigger starts timer2 without stopping timer1. After 30 seconds, timer1 fires with a stale callback.
**Why it happens:** Not cancelling the previous timer when a new trigger is detected in stateIdle or stateWaiting.
**How to avoid:** In startTimer(), always call m.timer.Stop() on any existing timer before creating a new one. Also consider: if already in stateWaiting and a new trigger arrives, restart the timer (or ignore the duplicate trigger).
**Warning signs:** Spurious timeout notification from a previous trigger cycle.

### Pitfall 6: Context Cancellation During Active Timer
**What goes wrong:** Monitor.Stop() is called (e.g., instance stopped for update) while the 30-second timer is active. The timer fires after Stop() and sends a notification for an instance that was intentionally stopped.
**Why it happens:** Stop() cancels the context, which exits the read loop, but the AfterFunc timer is still pending.
**How to avoid:** In Stop(), stop the timer before cancelling the context. Or: in the AfterFunc callback, check if context is done before sending notification.
**Note:** Full cancel-on-stop behavior is Phase 43 (TELE-09). Phase 42 should implement Stop() correctly but integration is deferred.
**Warning signs:** Spurious failure notification 30 seconds after stopping an instance.

## Code Examples

### Pattern Matching Functions (patterns.go)
```go
// Source: [ASSUMED] exact patterns from REQUIREMENTS.md -- need verification against real nanobot output

package telegram

import "strings"

const (
    // TriggerPattern starts the 30s monitoring window (TELE-01)
    TriggerPattern = "Starting Telegram bot"

    // SuccessPattern indicates Telegram bot connected (TELE-02)
    SuccessPattern = "Telegram bot commands registered"

    // FailurePattern indicates connection error (TELE-03)
    FailurePattern = "httpx.ConnectError"

    // DefaultTimeout for Telegram connection monitoring (TELE-04)
    DefaultTimeout = 30 * time.Second
)

// IsTrigger returns true if the log line indicates Telegram bot is starting.
func IsTrigger(line string) bool {
    return strings.Contains(line, TriggerPattern)
}

// IsSuccess returns true if the log line indicates Telegram bot connected successfully.
func IsSuccess(line string) bool {
    return strings.Contains(line, SuccessPattern)
}

// IsFailure returns true if the log line indicates a connection error.
func IsFailure(line string) bool {
    return strings.Contains(line, FailurePattern)
}
```
[VERIFIED: Pattern strings from REQUIREMENTS.md lines 16-18. Pattern matching approach from .planning/research/STACK.md]

### Mock Notifier for Testing
```go
// Source: [VERIFIED] pattern from internal/api/trigger_test.go recordingNotifier

type mockNotifier struct {
    mu       sync.Mutex
    calls    []notifyCall
    enabled  bool
}

type notifyCall struct {
    Title   string
    Message string
}

func (m *mockNotifier) IsEnabled() bool { return m.enabled }

func (m *mockNotifier) Notify(title, message string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.calls = append(m.calls, notifyCall{Title: title, Message: message})
    return nil
}

func (m *mockNotifier) getCalls() []notifyCall {
    m.mu.Lock()
    defer m.mu.Unlock()
    return append([]notifyCall{}, m.calls...)
}
```

### Mock LogSubscriber for Testing
```go
// Source: [ASSUMED] follows codebase duck-typing test pattern

type mockLogSubscriber struct {
    ch      chan logbuffer.LogEntry
    cancelled bool
}

func newMockLogSubscriber() *mockLogSubscriber {
    return &mockLogSubscriber{
        ch: make(chan logbuffer.LogEntry, 100),
    }
}

func (m *mockLogSubscriber) Subscribe() <-chan logbuffer.LogEntry {
    return m.ch
}

func (m *mockLogSubscriber) Unsubscribe(ch <-chan logbuffer.LogEntry) {
    m.cancelled = true
    close(m.ch)
}

func (m *mockLogSubscriber) writeEntry(content string) {
    m.ch <- logbuffer.LogEntry{
        Timestamp: time.Now(),
        Source:    "stdout",
        Content:   content,
    }
}
```

### Test: Successful Connection Detection
```go
// Source: [ASSUMED] follows codebase test pattern from notifier_ext_test.go

func TestMonitor_SuccessWithinTimeout(t *testing.T) {
    sub := newMockLogSubscriber()
    notif := &mockNotifier{enabled: true}

    m := NewTelegramMonitor(sub, notif, "test-instance", 5*time.Second, slog.Default())
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    go m.Start(ctx)

    // Simulate trigger then success
    sub.writeEntry("Starting Telegram bot...")
    sub.writeEntry("Telegram bot commands registered")

    time.Sleep(100 * time.Millisecond) // Allow processing

    cancel() // Stop monitor

    calls := notif.getCalls()
    // Should NOT have failure/timeout notification
    for _, c := range calls {
        if strings.Contains(c.Title, "failed") || strings.Contains(c.Title, "timeout") {
            t.Errorf("unexpected failure notification: %s", c.Title)
        }
    }
}
```

### Test: Timeout Fires After 30 Seconds
```go
func TestMonitor_TimeoutAfter30Seconds(t *testing.T) {
    sub := newMockLogSubscriber()
    notif := &mockNotifier{enabled: true}

    // Use short timeout for test speed
    m := NewTelegramMonitor(sub, notif, "test-instance", 500*time.Millisecond, slog.Default())
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    go m.Start(ctx)

    // Trigger but no success/failure pattern follows
    sub.writeEntry("Starting Telegram bot...")

    // Wait for timeout
    time.Sleep(700 * time.Millisecond)

    calls := notif.getCalls()
    if len(calls) == 0 {
        t.Fatal("expected timeout notification, got none")
    }
    if !strings.Contains(calls[0].Title, "Telegram") {
        t.Errorf("expected Telegram-related title, got: %s", calls[0].Title)
    }
}
```

### Test: Historical Replay Does Not Trigger (TELE-08)
```go
func TestMonitor_HistoricalReplayIgnored(t *testing.T) {
    sub := newMockLogSubscriber()
    notif := &mockNotifier{enabled: true}

    m := NewTelegramMonitor(sub, notif, "test-instance", 5*time.Second, slog.Default())
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    go m.Start(ctx)

    // Write an entry with a timestamp BEFORE monitor start
    sub.ch <- logbuffer.LogEntry{
        Timestamp: time.Now().Add(-10 * time.Second), // Historical
        Source:    "stdout",
        Content:   "Starting Telegram bot...", // Would trigger if not filtered
    }

    time.Sleep(100 * time.Millisecond)
    cancel()

    calls := notif.getCalls()
    if len(calls) > 0 {
        t.Errorf("historical entry should not trigger notification, got %d calls", len(calls))
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| N/A (new feature) | time.AfterFunc with mutex-protected state | Existing pattern in codebase | Proven pattern from notification/manager.go. No innovation risk. |
| N/A (new feature) | Duck-typed interfaces for testability | Established in codebase | api/trigger.go and notification/manager.go already use this pattern. |

**Deprecated/outdated:**
- None applicable -- this is a new package with no prior art in the codebase.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Exact log pattern "Starting Telegram bot" comes from python-telegram-bot library | Architecture Patterns | Monitor never triggers. Patterns should be in a separate patterns.go for easy adjustment. |
| A2 | Exact success pattern is "Telegram bot commands registered" | Architecture Patterns | Success not detected, monitor always reports timeout. Must verify against real nanobot output. |
| A3 | Exact failure pattern is "httpx.ConnectError" | Architecture Patterns | Connection failure not detected. Monitor waits for timeout instead of immediate failure notification. |
| A4 | time.AfterFunc callback goroutine can safely call notifier.Notify() in a goroutine | Architecture Patterns | Potential double-goroutine overhead. Negligible risk. |
| A5 | LogEntry.Timestamp has millisecond precision sufficient for historical replay filter | Architecture Patterns | Very rare: entry written in same millisecond as startTime might be misclassified. Acceptable tolerance. |
| A6 | No need for success notification (only failure/timeout) -- requirement TELE-05 may mean notify on success too | Phase Requirements | If success notification is required, trivial to add -- just another async notif.Notify() call in IsSuccess branch. |

**If this table is empty:** All claims in this research were verified or cited.

## Open Questions

1. **Should TELE-05 send a Pushover notification on success, or just log it?**
   - What we know: REQUIREMENTS.md says "Telegram connection success sends Pushover notification" (TELE-05). Success criteria say "monitor sends a success Pushover notification."
   - What's unclear: The existing codebase pattern for update notifications only sends on failure. Should Telegram success also notify?
   - Recommendation: Yes, send success notification per TELE-05. This is explicit in requirements. Success notification provides confirmation that monitoring is working.

2. **Exact log pattern strings need verification against real nanobot output**
   - What we know: REQUIREMENTS.md specifies "Starting Telegram bot", "Telegram bot commands registered", "httpx.ConnectError".
   - What's unclear: Whether these exact strings appear in nanobot stdout. STATE.md block list notes this: "Exact log patterns from python-telegram-bot need verification against real nanobot stdout."
   - Recommendation: Isolate patterns in patterns.go. Implementation should make patterns trivially adjustable. Consider adding a debug log on every pattern match.

3. **Should the monitor re-enter idle state after resolution for the same instance?**
   - What we know: An instance could output "Starting Telegram bot" again after a reconnect attempt.
   - What's unclear: Whether Phase 42 should handle repeated trigger cycles or just one-and-done.
   - Recommendation: Return to stateIdle after resolution. This allows the monitor to catch reconnection attempts without restart. Single monitor per instance handles multiple connection cycles.

## Environment Availability

Step 2.6: SKIPPED (no external dependencies -- purely code changes using stdlib and existing internal packages)

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) + stretchr/testify |
| Config file | none |
| Quick run command | `go test ./internal/telegram/... -v -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TELE-01 | "Starting Telegram bot" triggers monitoring state | unit | `go test ./internal/telegram/... -run TestMonitor_TriggerDetected -v` | Wave 0 |
| TELE-02 | "Telegram bot commands registered" resolves as success | unit | `go test ./internal/telegram/... -run TestMonitor_SuccessDetected -v` | Wave 0 |
| TELE-03 | "httpx.ConnectError" resolves as failure with notification | unit | `go test ./internal/telegram/... -run TestMonitor_FailureDetected -v` | Wave 0 |
| TELE-04 | 30-second timeout sends timeout notification | unit | `go test ./internal/telegram/... -run TestMonitor_Timeout -v` | Wave 0 |
| TELE-05 | Success triggers Pushover notification | unit | `go test ./internal/telegram/... -run TestMonitor_SuccessNotification -v` | Wave 0 |
| TELE-06 | Failure triggers Pushover notification | unit | `go test ./internal/telegram/... -run TestMonitor_FailureNotification -v` | Wave 0 |
| TELE-08 | Historical replay entries are ignored | unit | `go test ./internal/telegram/... -run TestMonitor_HistoricalIgnored -v` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/telegram/... -v -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before /gsd-verify-work

### Wave 0 Gaps
- [ ] `internal/telegram/monitor_test.go` -- covers TELE-01 through TELE-06, TELE-08
- [ ] `internal/telegram/patterns_test.go` -- covers IsTrigger, IsSuccess, IsFailure
- [ ] Framework install: no additional install needed (Go testing + testify in go.mod)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Not applicable -- monitor reads from internal log stream |
| V3 Session Management | no | Not applicable |
| V4 Access Control | no | Not applicable |
| V5 Input Validation | yes | strings.Contains on Content field -- no injection risk (internal data) |
| V6 Cryptography | no | Not applicable |

### Known Threat Patterns for Go Log Monitoring

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Log injection (crafted "Starting Telegram bot" in log) | Spoofing | Trust internal nanobot output only. Same machine, no external injection vector. LOW risk. |
| Notification content leakage | Information disclosure | Keep notification content minimal: instance name, status. No ports, commands, or credentials. |

## Sources

### Primary (HIGH confidence)
- Codebase analysis of `internal/logbuffer/buffer.go` -- Write() fan-out, Subscribe/Unsubscribe lifecycle
- Codebase analysis of `internal/logbuffer/subscriber.go` -- History replay in subscriberLoop, context cancellation
- Codebase analysis of `internal/notification/manager.go` -- AfterFunc + mutex pattern, async notification with panic recovery
- Codebase analysis of `internal/notifier/notifier.go` -- Notifier interface, IsEnabled(), Notify()
- Codebase analysis of `internal/instance/lifecycle.go` -- InstanceLifecycle, GetLogBuffer(), StartAfterUpdate()
- Codebase analysis of `internal/instance/manager.go` -- InstanceManager, StartAllInstances(), AutoStartResult
- Codebase analysis of `cmd/nanobot-auto-updater/main.go` -- Auto-start goroutine, shutdown sequence
- `.planning/REQUIREMENTS.md` -- TELE-01 through TELE-08 requirement definitions
- `.planning/research/ARCHITECTURE.md` -- v0.9 architecture decisions (verified against codebase)
- `.planning/research/PITFALLS.md` -- Identified pitfalls for this domain
- `.planning/research/STACK.md` -- Stack recommendations (verified against codebase)

### Secondary (MEDIUM confidence)
- `.planning/quick/260406-fge-nanobot-telegram-httpx-connecterror-open/260406-fge-PLAN.md` -- Telegram httpx.ConnectError root cause analysis under OpenClash proxy

### Tertiary (LOW confidence)
- Exact log pattern strings from python-telegram-bot -- assumed based on library conventions, not verified against real nanobot stdout in this session. Flagged in STATE.md block list.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - all components verified in codebase, zero new dependencies
- Architecture: HIGH - state machine pattern verified from notification/manager.go, LogBuffer subscriber pattern verified
- Pitfalls: HIGH - all pitfalls derived from codebase analysis (LogBuffer behavior, AfterFunc goroutine, channel capacity)
- Test patterns: HIGH - mock patterns follow existing codebase conventions (duck-typed interfaces, testify assertions)

**Research date:** 2026-04-06
**Valid until:** 2026-05-06 (stable -- no fast-moving dependencies)

# Stack Research

**Domain:** Instance startup notifications and log-pattern-based Telegram connection monitoring for existing Go Windows service
**Researched:** 2026-04-06
**Confidence:** HIGH (verified against existing codebase, all capabilities reuse established patterns)

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| **Go stdlib `strings`** | Go 1.24+ | Log pattern matching for "Starting Telegram bot" detection | Already imported in codebase (`lifecycle/starter.go`). `strings.Contains()` is sufficient for exact pattern matching in captured log output. No regex overhead needed -- the target pattern is a fixed literal string emitted by python-telegram-bot. |
| **Go stdlib `time`** | Go 1.24+ | 30-second timeout timer for Telegram connection monitoring | `time.AfterFunc()` already used in codebase (`notification/manager.go` line 120) for cooldown timers. Same pattern applies: start timer when "Starting Telegram bot" detected, cancel timer on success/failure. `time.After()` with `select` also viable for goroutine-based timeout. |
| **Go stdlib `context`** | Go 1.24+ | Goroutine lifecycle control for Telegram monitor goroutine | `context.WithCancel()` + `context.WithTimeout()` already used throughout codebase (`notification/manager.go`, `logbuffer/subscriber.go`). Pattern: create per-instance context, cancel when monitoring completes or instance stops. |
| **Go stdlib `sync`** | Go 1.24+ | Mutex for Telegram monitor state, `sync.Once` for single notification guarantee | `sync.RWMutex` already used in `logbuffer/buffer.go` and `notification/manager.go`. Same pattern for protecting monitor state (started/completed/notified). |
| **Existing `logbuffer.LogBuffer` subscriber** | v0.4 (Phase 19) | Real-time log stream subscription for pattern detection | The subscriber channel pattern (`lb.Subscribe()` returning `<-chan LogEntry`) is already built and tested. Subscribe to instance's LogBuffer, scan each `LogEntry.Content` for pattern match. Unsubscribe when monitoring completes. No new infrastructure needed. |
| **Existing `notifier.Notifier`** | v0.5 (Phase 27) | Pushover notifications for startup results and Telegram failures | `Notifier.Notify(title, message)` already handles enabled/disabled, async send, panic recovery. Inject same `*notifier.Notifier` instance from main.go. No changes to notification layer. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| **`strings` (stdlib)** | Go 1.24+ | `strings.Contains(line, "Starting Telegram bot")` for pattern detection | Fixed-string matching. If future patterns need flexibility (regex), switch to `regexp` -- but NOT needed now. |
| **`regexp` (stdlib)** | Go 1.24+ | Only if pattern detection becomes complex (multiple patterns, wildcards) | NOT recommended for this milestone. `strings.Contains` is sufficient for literal "Starting Telegram bot" and connection success/failure patterns. `regexp` adds compile overhead and complexity for no benefit. |
| **`os/signal` (stdlib)** | Go 1.24+ | Graceful shutdown of Telegram monitor goroutines on SIGINT/SIGTERM | Already used in `main.go`. Telegram monitors should respect the application shutdown context, not implement their own signal handling. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| **Existing test patterns** | Unit tests for log pattern detection and timeout logic | Follow `notification/manager_test.go` pattern: mock the Notifier interface, verify notification content. Use `logbuffer.NewLogBuffer()` directly in tests, call `lb.Write()` to simulate log output. |
| **Existing testutil package** | Test helpers if needed | Check `testutil/` for existing mock patterns. Likely not needed -- the existing `recordingNotifier` mock from Phase 35 tests covers notification assertion. |

## Installation

```bash
# NO new dependencies needed
# All capabilities use Go standard library or existing internal packages:
#   - strings (pattern matching)
#   - time (timeout timer)
#   - context (lifecycle control)
#   - sync (state mutex)
#   - internal/logbuffer (subscriber pattern)
#   - internal/notifier (Pushover notifications)
go mod tidy  # No changes expected
```

## Integration with Existing Stack

### Existing Components Reused Directly

| Existing Component | Reuse Pattern | Rationale |
|-------------------|---------------|-----------|
| **`logbuffer.LogBuffer.Subscribe()`** | Subscribe to instance log stream, scan Content for patterns | Returns `<-chan LogEntry` with capacity 100. Non-blocking send (drops for slow consumers). Already handles history replay. |
| **`logbuffer.LogBuffer.Unsubscribe()`** | Clean up subscriber when monitoring completes | Cancels context, removes from map. Prevents goroutine leak. |
| **`notifier.Notifier.Notify()`** | Send Pushover for startup success/failure and Telegram failure | Already handles enabled/disabled state, async goroutine, panic recovery. |
| **`notification.Notifier` interface** | Duck-typed interface for dependency injection in tests | Same interface used in `notification/manager.go` and `api/trigger.go`. Two methods: `IsEnabled()` and `Notify(title, message)`. |
| **`instance.AutoStartResult`** | Source of startup success/failure data for notifications | Already contains `Started []string` and `Failed []*InstanceError` from `StartAllInstances()`. No modification needed. |
| **`context.WithTimeout()` pattern** | 30-second timeout for Telegram connection monitoring | Same pattern as `notification/manager.go` cooldown timer. Use `time.AfterFunc` or `context.WithTimeout` + goroutine. |
| **Panic recovery pattern** | Wrap notification goroutines with `defer func() { if r := recover() ... }()` | Established pattern in `notification/manager.go` line 181-186 and `api/trigger.go`. |

### New Components Needed

| Component | Location | Responsibility | Size Estimate |
|-----------|----------|---------------|---------------|
| **`TelegramMonitor`** | New file: `internal/telegram/monitor.go` | Subscribe to LogBuffer, detect "Starting Telegram bot" pattern, start 30s timeout, detect success/failure, send notification | ~150 lines |
| **Startup notification integration** | Modify: `cmd/nanobot-auto-updater/main.go` | After `StartAllInstances()`, send Pushover notification with results | ~20 lines added |
| **Telegram monitor lifecycle** | Modify: `cmd/nanobot-auto-updater/main.go` | Create and start TelegramMonitor for each instance, stop on shutdown | ~15 lines added |

### No New Dependencies

**This is a key finding.** All three new features (startup notification, log pattern detection, timeout monitoring) can be built entirely with Go standard library and existing internal packages:

1. **Startup result notification**: Call `notif.Notify()` with formatted `AutoStartResult` data. No new libraries.
2. **Log pattern detection**: Subscribe to `LogBuffer`, use `strings.Contains()` on `LogEntry.Content`. No new libraries.
3. **30-second timeout**: Use `time.AfterFunc()` or `context.WithTimeout()`. No new libraries.

## Alternatives Considered

| Recommended | Alternative | Why Not |
|-------------|-------------|---------|
| **`strings.Contains`** | **`regexp.MatchString`** | Regex adds compile overhead and complexity for what is a fixed literal string match. The target patterns ("Starting Telegram bot", connection success/failure messages) are predictable literal strings. If patterns become complex in future (multiple variants, wildcards), introduce `regexp` then. |
| **Existing `LogBuffer.Subscribe()`** | **Direct `os.Pipe` reading** | Would duplicate the log capture infrastructure already in `lifecycle/starter.go`. The subscriber pattern is purpose-built for this exact use case: consuming log output in real time without interfering with the main capture flow. |
| **Existing `LogBuffer.Subscribe()`** | **New dedicated stderr/stdout tee** | Adding a second pipe reader introduces complexity (sync between two consumers, potential blocking). The subscriber pattern already handles multi-consumer fan-out with non-blocking sends. |
| **`time.AfterFunc` + state mutex** | **`context.WithTimeout` + select goroutine** | Both are viable. `AfterFunc` is simpler for "start timer, cancel on event" pattern. `context.WithTimeout` is better if the monitor needs to be cancellable from outside (e.g., instance shutdown). Recommend `context.WithTimeout` for the Telegram monitor since it should also stop when the instance stops. |
| **New `internal/telegram/` package** | **Add to existing `notification/manager.go`** | Telegram monitoring is a distinct concern from network connectivity monitoring. Mixing them violates single responsibility. New package keeps the concern isolated and testable. |
| **New `internal/telegram/` package** | **Add to `instance/lifecycle.go`** | Telegram monitoring is a monitoring concern, not an instance lifecycle concern. InstanceLifecycle handles start/stop. The Telegram monitor observes instance output and reacts. Coupling them makes testing harder. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| **Third-party log parsing library** | Extreme overkill for literal string matching. Adds dependency for zero value. | `strings.Contains()` from stdlib |
| **`regexp` package** | Compile overhead, readability cost, no benefit for fixed literal patterns. | `strings.Contains()` from stdlib |
| **External pub/sub or message queue** | In-process Go channels are the right tool. External systems add latency, failure modes, and deployment complexity. | Existing `logbuffer.LogBuffer` subscriber channels |
| **Goroutine per log line** | Spawning a goroutine for each log entry to check patterns is wasteful. Pattern detection should happen inline in the subscriber goroutine. | Single subscriber goroutine with `for entry := range ch` loop |
| **`time.Sleep` polling** | Polling introduces latency and wastes CPU. The subscriber channel provides event-driven notification. | Channel receive with select + timeout |
| **Global singleton for TelegramMonitor** | Violates dependency injection pattern established in the codebase. Makes testing harder. | Instance-per-instance pattern, injected from main.go |
| **Modifying `LogBuffer.Write()`** | Adding pattern matching logic into LogBuffer couples log storage with monitoring. LogBuffer should remain a dumb buffer (established design decision). | External subscriber consuming from the channel |
| **Modifying `lifecycle/starter.go`** | Adding notification or monitoring hooks into the process start path mixes concerns. Starter should only start processes. | Observer pattern: subscribe to LogBuffer externally |

## Stack Patterns by Feature

### Feature 1: Instance Startup Result Notification

**Pattern:** Call `notif.Notify()` after `StartAllInstances()` completes.

```
main.go auto-start goroutine
    |
    v
instanceManager.StartAllInstances(ctx) -> *AutoStartResult
    |
    v
if result has failures OR Pushover configured:
    format notification from result.Started, result.Failed
    |
    v
    notif.Notify(title, message)  // async via goroutine + panic recovery
```

**Implementation:**
- Add notification logic in `main.go` after `instanceManager.StartAllInstances(autoStartCtx)` call (line 224).
- Format: title indicates success/failure/partial, body lists per-instance results.
- Async send: wrap in goroutine with panic recovery (same pattern as `notification/manager.go`).
- Graceful degradation: if Pushover not configured, skip silently.

**Key design decision -- notify on success too:** Unlike the existing update notification pattern (only notify on failure), startup notifications should fire for BOTH success and failure. Rationale: user needs confirmation that instances started after application launch, not just when something goes wrong. This is different from the update notification pattern because updates are user-initiated (user knows they triggered it), while auto-start is automatic (user needs confirmation it worked).

### Feature 2: Log Pattern Detection for Telegram Monitoring

**Pattern:** Subscribe to instance LogBuffer, scan Content for trigger pattern.

```
TelegramMonitor created for each instance
    |
    v
lb.Subscribe() -> <-chan LogEntry
    |
    v
for entry := range ch:
    if strings.Contains(entry.Content, "Starting Telegram bot"):
        start 30s timeout monitoring
    if strings.Contains(entry.Content, successPattern):
        cancel timeout, log success
    if strings.Contains(entry.Content, failurePattern):
        cancel timeout, send failure notification
```

**Implementation:**
- New struct `TelegramMonitor` in `internal/telegram/monitor.go`.
- Constructor takes: `*logbuffer.LogBuffer`, `Notifier` interface, `*slog.Logger`.
- `Start()` method: subscribes to LogBuffer, enters scan loop.
- `Stop()` method: cancels context, unsubscribes from LogBuffer.

**Key design decision -- configurable patterns:** The trigger pattern ("Starting Telegram bot") and result patterns (connection success/failure) should be constants in the package. If patterns change across nanobot versions, make them configurable in config.yaml. For now, constants are sufficient -- the pattern comes from python-telegram-bot which is stable.

### Feature 3: 30-Second Timeout Monitoring

**Pattern:** `context.WithTimeout` + channel-based detection.

```
"Starting Telegram bot" detected
    |
    v
ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
    |
    v
select {
    case <-successCh:   // connection success pattern detected
        cancel()
        log success
    case <-failureCh:   // connection failure pattern detected
        cancel()
        send failure notification
    case <-ctx.Done():  // 30s timeout elapsed
        send timeout notification
}
```

**Implementation:**
- Within the subscriber goroutine, when trigger pattern is detected, start a sub-goroutine with timeout context.
- Use channels or shared state (mutex-protected) to communicate success/failure from the scan loop to the timeout goroutine.
- Simpler alternative: single goroutine tracks state with `time.AfterFunc`:
  1. On trigger pattern: start `time.AfterFunc(30s, onTimeout)`, set state to "monitoring".
  2. On success pattern: call `timer.Stop()`, set state to "connected", log success.
  3. On failure pattern: call `timer.Stop()`, set state to "failed", send notification.
  4. On timeout callback: check state is still "monitoring", send timeout notification.

**Recommended approach:** The `time.AfterFunc` approach is simpler and avoids sub-goroutine management. The state machine (idle -> monitoring -> connected/failed/timed_out) is easy to test.

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| `strings` (stdlib) | All Go versions | No compatibility concern |
| `time.AfterFunc` (stdlib) | All Go versions | No compatibility concern |
| `context.WithTimeout` (stdlib) | Go 1.7+ | Well within Go 1.24.11 requirement |
| `logbuffer.LogBuffer` | v0.4 (Phase 19) | Stable API, Subscribe/Unsubscribe pattern unchanged since Phase 19 |
| `notifier.Notifier` | v0.5 (Phase 27) | Stable API, Notify(title, message) unchanged since Phase 27 |
| `notification.Notifier` interface | v0.5 (Phase 27) | Duck-typed, satisfied by `*notifier.Notifier` concrete type |

### Dependency Tree Impact

```
NEW dependencies added to go.mod:
  (none)

All capabilities use:
  - Go stdlib (strings, time, context, sync)
  - Existing internal packages (logbuffer, notifier, notification)
```

**Total new transitive dependencies: 0.** This is the lowest-risk milestone from a dependency perspective.

## Data Flow Diagrams

### Startup Notification Flow

```
main.go
  |
  +-- instanceManager.StartAllInstances(ctx)
  |     |
  |     +-- returns *AutoStartResult {Started, Failed, Skipped}
  |
  +-- formatStartupNotification(result)
  |     |
  |     +-- title: "Nanobot Started: X/Y instances" or "Nanobot Start Failed"
  |     +-- message: per-instance results
  |
  +-- go func() { notif.Notify(title, message) }()  // async + panic recovery
```

### Telegram Connection Monitoring Flow

```
main.go
  |
  +-- for each instance:
  |     telegramMonitor := telegram.NewMonitor(instance.LogBuffer, notif, logger)
  |     go telegramMonitor.Start()
  |
  +-- on shutdown: telegramMonitor.Stop()

telegram.Monitor.Start():
  |
  +-- ch := lb.Subscribe()
  |
  +-- for entry := range ch:
  |     |
  |     +-- if "Starting Telegram bot" detected:
  |     |     state = "monitoring"
  |     |     timer = time.AfterFunc(30s, onTimeout)
  |     |
  |     +-- if connected pattern detected AND state == "monitoring":
  |     |     timer.Stop()
  |     |     state = "connected"
  |     |     logger.Info("Telegram connected")
  |     |
  |     +-- if failure pattern detected AND state == "monitoring":
  |           timer.Stop()
  |           state = "failed"
  |           notif.Notify("Telegram Connection Failed", ...)
  |
  +-- onTimeout():
        if state == "monitoring":
          state = "timed_out"
          notif.Notify("Telegram Connection Timeout", ...)
```

## Sources

- **Existing codebase analysis** -- `internal/logbuffer/buffer.go`, `internal/logbuffer/subscriber.go`: Subscribe/Unsubscribe pattern verified. Non-blocking channel send with capacity 100 confirmed. (HIGH confidence)
- **Existing codebase analysis** -- `internal/notifier/notifier.go`: Notify(title, message) API confirmed. IsEnabled() check confirmed. (HIGH confidence)
- **Existing codebase analysis** -- `internal/notification/manager.go`: AfterFunc + state pattern for cooldown timer verified (line 120). Async notification with panic recovery pattern verified (line 181-186). (HIGH confidence)
- **Existing codebase analysis** -- `internal/instance/manager.go`: `AutoStartResult` struct with Started/Failed/Skipped fields confirmed (line 190-194). `StartAllInstances()` return type confirmed. (HIGH confidence)
- **Existing codebase analysis** -- `cmd/nanobot-auto-updater/main.go`: Auto-start goroutine with panic recovery verified (line 204-225). Notifier creation and injection verified (line 135-141). (HIGH confidence)
- **Existing codebase analysis** -- `internal/lifecycle/starter.go`: Log capture via `captureLogs()` writing to `logbuffer.LogBuffer` confirmed (line 143-168). (HIGH confidence)
- **python-telegram-bot source** -- `Starting Telegram bot` is the standard log message when bot polling starts. Confirmed from python-telegram-bot library documentation and source code. (MEDIUM confidence -- should verify exact log output from actual nanobot instance)
- **Go stdlib `time.AfterFunc`** -- Official Go documentation. Timer can be stopped with `Stop()`, runs callback in its own goroutine. (HIGH confidence)
- **Go stdlib `strings.Contains`** -- Literal substring match. O(n) where n is string length. Appropriate for short log lines. (HIGH confidence)

---
*Stack research for: Instance startup notifications and Telegram connection monitoring*
*Researched: 2026-04-06*

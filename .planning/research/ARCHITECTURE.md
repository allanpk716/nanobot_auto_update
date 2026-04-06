# Architecture: Instance Startup Notifications and Telegram Connection Monitoring (v0.9)

**Domain:** Go Windows service -- adding Pushover notifications for instance startup results and log-pattern-based Telegram connection monitoring.
**Researched:** 2026-04-06
**Overall confidence:** HIGH

## Executive Summary

This milestone adds two notification features to the existing nanobot-auto-updater: (1) Pushover notifications when instances start (success or failure), and (2) detection of Telegram connection status by monitoring log output for specific patterns, with a 30-second timeout and failure notification.

Both features integrate into the existing architecture without structural changes. The instance startup notification hooks into the existing `InstanceManager.StartAllInstances()` return path in `main.go`, iterating the `AutoStartResult` and sending a summary notification. The Telegram connection monitor is a new `TelegramMonitor` component that subscribes to the existing `LogBuffer` per-instance, watches for the trigger pattern "Starting Telegram bot", starts a 30-second timer, and checks for success/failure patterns. Both features reuse the existing `Notifier` interface for Pushover delivery.

---

## Current Architecture (Integration Points)

### Existing Component Inventory

```
cmd/nanobot-auto-updater/main.go       -- Entry point, wires all components
internal/api/server.go                  -- HTTP mux, handler registration
internal/instance/manager.go           -- InstanceManager: StartAllInstances(), TriggerUpdate()
internal/instance/lifecycle.go         -- InstanceLifecycle: StartAfterUpdate(), GetLogBuffer()
internal/logbuffer/buffer.go           -- Ring buffer: Write(), Subscribe(), GetHistory()
internal/logbuffer/subscriber.go       -- Channel-based subscriber pattern (capacity 100)
internal/notifier/notifier.go          -- Pushover: Notify(), IsEnabled()
internal/notification/manager.go       -- Network connectivity notification manager
internal/lifecycle/starter.go          -- StartNanobotWithCapture() with log capture goroutines
internal/health/monitor.go             -- Periodic health checks for all instances
```

### Existing Data Flow: Instance Startup

```
main.go (goroutine)
  -> instanceManager.StartAllInstances(ctx)
     -> lifecycle.StopAllNanobots()           // Clean slate
     -> for each instance:
           inst.StartAfterUpdate(ctx)
              -> logBuffer.Clear()            // INST-05
              -> lifecycle.StartNanobotWithCapture()
                 -> cmd.Start()               // Start process
                 -> go captureLogs(stdout)    // Goroutine: pipe -> LogBuffer.Write()
                 -> go captureLogs(stderr)    // Goroutine: pipe -> LogBuffer.Write()
     -> return AutoStartResult{Started, Failed, Skipped}
```

### Existing Data Flow: Log Capture to Ring Buffer

```
Nanobot Process (stdout/stderr)
  -> os.Pipe()
  -> captureLogs goroutine (lifecycle/starter.go)
  -> logBuffer.Write(LogEntry{Timestamp, Source, Content})
     -> ring buffer store (fixed array [5000])
     -> non-blocking fan-out to all subscribers
        -> subscriber channels (capacity 100)
           -> SSE handler (existing, for web UI)
           -> [NEW] TelegramMonitor subscriber
```

### Existing Patterns to Reuse

| Pattern | Where Used | How to Apply for v0.9 |
|---------|-----------|----------------------|
| `Notifier` interface (duck typing) | `notification/manager.go`, `api/trigger.go` | Both new features call `notif.Notify()` via the same interface |
| Async notification + panic recovery | `notification/manager.go:sendNotification()`, `api/trigger.go:Handle()` | Same pattern: `go func() { defer recover(); notif.Notify() }()` |
| Channel-based LogBuffer subscriber | `logbuffer/subscriber.go` | `TelegramMonitor` calls `logBuffer.Subscribe()`, reads `<-chan LogEntry` |
| Nil-safe Notifier checks | `notification/manager.go:sendNotification()` checks `nm.notifier.IsEnabled()` | Both features check `notif.IsEnabled()` before sending |
| Context-aware logging | All components: `logger.With("component", "...")` | New `TelegramMonitor` uses same pattern |
| Graceful degradation | InstanceManager: failed instances don't block others | Telegram monitor failure must not affect instance startup |
| TDD with interfaces | `trigger.go` defines local interfaces, tests use mocks | `TelegramMonitor` accepts `LogSubscriber` interface for testing |

---

## Recommended Architecture

### New Components

| Component | Package | Responsibility |
|-----------|---------|---------------|
| `TelegramMonitor` | `internal/telegram/` | Subscribe to LogBuffer, detect "Starting Telegram bot", track 30s timeout, notify on failure |
| `sendStartupNotification` | `main.go` (or helper in `notification/`) | Format and send AutoStartResult summary via Pushover |

### Modified Components

| Component | Change | Why |
|-----------|--------|-----|
| `main.go` | Add startup notification call after `StartAllInstances()`, create and start `TelegramMonitor` per instance | Wire new notification features into the startup flow |
| No other files modified | | Integration is minimal, touches only main.go |

### Component Boundaries

```
                              main.go
                                 |
                    +------------+------------+------------+
                    |            |            |            |
              instanceManager  apiServer  notif (exist)  [NEW]
                    |            |            |         startupNotif()
                    |            |            |         (after auto-start)
                    |     +------+------+
                    |     |      |      |
                    |   SelfUpdate  TriggerHandler (exist)
                    |   Handler     QueryHandler (exist)
                    |     |
            +-------+-------+
            |               |
     selfupdate (exist)  notifier.Notifier
                        (update notifications)

            +--- [NEW: TelegramMonitor] ---+
            |                               |
    logBuffer.Subscribe()          notifier.Notifier
    (<-chan LogEntry)              (failure notifications)
```

---

## Feature 1: Instance Startup Notification

### Design

After `instanceManager.StartAllInstances()` returns `*AutoStartResult` in `main.go`, send a Pushover notification summarizing the results. This is a one-shot notification, not a long-running component.

### Data Flow

```
1. main.go goroutine:
   autoStartResult := instanceManager.StartAllInstances(ctx)

2. [NEW] sendStartupNotification(notif, autoStartResult, logger)
   a. Check notif.IsEnabled() -- skip if Pushover not configured
   b. If autoStartResult.Failed is empty AND len(Started) > 0:
      Send success notification:
        Title: "Nanobot 启动完成"
        Message: "成功: X 个实例\n耗时: Ys"
   c. If autoStartResult.Failed is not empty:
      Send failure notification:
        Title: "Nanobot 启动失败"
        Message: "成功: X\n失败: Y\n失败实例: ..."
   d. Send async (goroutine + panic recovery) -- same pattern as trigger.go
```

### Implementation Location

A standalone function in `main.go` or a helper function extracted to a new file in `notification/` package. Given the simplicity (one function, no state), keeping it in `main.go` is the lightest option. If it grows, extract later.

```go
// In main.go, after autoStartResult := instanceManager.StartAllInstances(ctx)
sendStartupNotification(notif, autoStartResult, logger)
```

### Key Decision: Notify on Success AND Failure

Unlike the update notification pattern (which only notifies on failure), startup notifications should fire on both success and failure. Rationale: the user needs confirmation that instances started correctly, especially after a reboot or service restart. The cost of one extra Pushover message on success is low; the cost of missing a "failed to start" event is high.

However, if Pushover is not configured, skip silently (same graceful degradation as existing patterns).

---

## Feature 2: Telegram Connection Monitor

### Design

A new `TelegramMonitor` struct that subscribes to each instance's `LogBuffer` and watches for Telegram connection patterns. When "Starting Telegram bot" appears, it starts a 30-second timer. If "Telegram bot @xxx connected" appears within 30 seconds, the timer is cancelled (success). If the timer fires without a success pattern, a Pushover notification is sent.

### Data Flow

```
1. main.go:
   For each instance that was started:
     tm := telegram.NewTelegramMonitor(inst.GetLogBuffer(), notif, inst.Name(), logger)
     go tm.Start()
     // Store tm references for shutdown

2. TelegramMonitor.Start():
   a. ch := logBuffer.Subscribe()    // Get <-chan LogEntry
   b. defer logBuffer.Unsubscribe(ch)
   c. for entry := range ch:
        if strings.Contains(entry.Content, "Starting Telegram bot"):
          Start 30-second timer (time.AfterFunc)
          Set state = "waiting"
        if state == "waiting" && matchSuccessPattern(entry.Content):
          Cancel timer
          Set state = "connected"
          Log success
        // Timer fires -> onTimeout()
          Send Pushover notification: "Telegram 连接超时"
          Set state = "timeout"

3. On instance restart (via StartAllInstances or trigger-update):
   - Old TelegramMonitor's logBuffer.Clear() happens before StartAfterUpdate()
   - Old subscriber continues receiving new logs
   - OR: Stop old monitor, create new one after restart
```

### New Internal Package: `internal/telegram/`

```
internal/telegram/
  monitor.go        -- TelegramMonitor struct, Start(), Stop()
  monitor_test.go   -- Unit tests with mock LogBuffer subscriber
  patterns.go       -- Pattern constants and matching functions
```

### Core Types

```go
// telegram/monitor.go

// LogSubscriber is the interface for subscribing to log entries.
// Defined here for testability -- LogBuffer satisfies this via duck typing.
type LogSubscriber interface {
    Subscribe() <-chan logbuffer.LogEntry
    Unsubscribe(ch <-chan logbuffer.LogEntry)
}

// Notifier interface for sending failure notifications.
// Matches the existing notifier.Notifier duck typing pattern.
type Notifier interface {
    IsEnabled() bool
    Notify(title, message string) error
}

// TelegramMonitor watches instance logs for Telegram connection patterns.
// After detecting "Starting Telegram bot", it waits up to 30 seconds
// for a success indication. If none arrives, it sends a Pushover alert.
type TelegramMonitor struct {
    logBuffer   LogSubscriber
    notifer     Notifier
    instanceName string
    timeout     time.Duration    // Default: 30 seconds
    logger      *slog.Logger
    ctx         context.Context
    cancel      context.CancelFunc
}
```

### Pattern Matching Logic

```go
// telegram/patterns.go

const (
    // TriggerPattern is the log line that indicates Telegram bot is starting
    TriggerPattern = "Starting Telegram bot"

    // SuccessPatterns indicate Telegram connected successfully
    // Matched via strings.Contains (case-sensitive, partial match)
    SuccessPattern1 = "Telegram bot @"     // e.g., "Telegram bot @mybot connected"
    SuccessPattern2 = "telegram"           // Fallback: any telegram success indicator

    // DefaultTimeout is the default wait time for connection confirmation
    DefaultTimeout = 30 * time.Second
)

// IsTrigger returns true if the log line indicates Telegram bot is starting.
func IsTrigger(line string) bool {
    return strings.Contains(line, TriggerPattern)
}

// IsSuccess returns true if the log line indicates Telegram connected.
func IsSuccess(line string) bool {
    // Check for known success patterns
    // "Telegram bot @xxx connected" or similar
    return strings.Contains(line, "bot @") &&
           (strings.Contains(line, "connected") || strings.Contains(line, "started"))
}
```

**Note on patterns**: The exact log patterns depend on what nanobot actually outputs. Before implementation, verify the exact strings by checking a running nanobot's stdout. The patterns above are estimates. The implementation should make patterns easily configurable.

### State Machine

```
  [idle] --(trigger pattern)--> [waiting] --(success pattern)--> [connected]
                                   |
                                   +--(30s timeout)--> [timed_out] --> send notification --> [idle]
                                   |
                                   +--(new trigger pattern)--> restart timer --> [waiting]
```

State transitions:
- `idle -> waiting`: Log line matches trigger pattern
- `waiting -> connected`: Log line matches success pattern within timeout
- `waiting -> timed_out`: Timer fires, send notification, return to idle
- `waiting -> waiting`: Another trigger pattern seen (restart timer)
- Any state -> idle: Monitor receives new trigger after previous cycle

### Integration with Instance Lifecycle

The TelegramMonitor lifecycle is tied to instance startup:

1. **On auto-start** (main.go): After `StartAllInstances()` succeeds for an instance, create and start a `TelegramMonitor` for that instance.
2. **On trigger-update** (trigger.go flow): After `UpdateAll()` restarts instances, the monitors from step 1 are still subscribed to the same LogBuffer. The LogBuffer.Clear() in `StartAfterUpdate()` resets the buffer but subscribers keep receiving new entries. The existing monitor continues working.
3. **On manual restart** (web UI restart endpoint): Same as trigger-update -- the LogBuffer is the same object, monitor keeps receiving.

This means we do NOT need to create/destroy TelegramMonitors on every restart. A single monitor per instance, created once at startup, is sufficient. The `LogBuffer.Clear()` does not affect subscribers (confirmed by the existing code comment in `buffer.go` line 105: "Subscribers continue receiving new logs after Clear()").

### Shutdown

```go
// TelegramMonitor.Stop() cancels the context, which:
// 1. Stops the subscriber goroutine (exits range loop)
// 2. Cancels any pending timeout timer
// 3. Unsubscribes from LogBuffer

func (tm *TelegramMonitor) Stop() {
    tm.cancel()
}
```

In `main.go` shutdown sequence, stop all TelegramMonitors alongside other components (health monitor, notification manager, etc.).

---

## Updated main.go Wiring

### After Auto-Start

```go
// Existing auto-start goroutine in main.go
go func() {
    defer func() {
        if r := recover(); r != nil {
            logger.Error("auto-start goroutine panic", ...)
        }
    }()

    autoStartCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    autoStartResult := instanceManager.StartAllInstances(autoStartCtx)

    // [NEW] Send startup notification
    sendStartupNotification(notif, autoStartResult, logger)

    // [NEW] Start Telegram monitors for instances that started successfully
    for _, startedName := range autoStartResult.Started {
        logBuf, _ := instanceManager.GetLogBuffer(startedName)
        tm := telegram.NewTelegramMonitor(logBuf, notif, startedName, logger)
        telegramMonitors = append(telegramMonitors, tm)
        go tm.Start()
    }
}()
```

### Shutdown Sequence Update

```go
// Stop Telegram monitors [NEW]
for _, tm := range telegramMonitors {
    tm.Stop()
}
```

### Complete Component Wiring

```
main.go creates:
  -> logger (slog.Logger)
  -> config (*config.Config) via config.Load()
  -> updateLogger (*updatelog.UpdateLogger)
  -> instanceManager (*instance.InstanceManager)
  -> notif (*notifier.Notifier)           // reused by both new features
  -> selfUpdater (*selfupdate.Updater)
  -> apiServer (*api.Server)
  -> healthMonitor (*health.HealthMonitor)
  -> networkMonitor (*network.NetworkMonitor)
  -> notificationManager (*notification.NotificationManager)
  -> telegramMonitors []*telegram.TelegramMonitor   // [NEW] one per instance
```

---

## Patterns to Follow

### Pattern 1: Subscribe to LogBuffer for Pattern Detection

**What:** Use the existing `LogBuffer.Subscribe()` channel to receive log entries in real-time.
**When:** Any component that needs to react to log content.
**Why:** Non-blocking, already handles slow subscribers (drops entries), clean lifecycle via context cancellation.

```go
ch := logBuffer.Subscribe()
defer logBuffer.Unsubscribe(ch)

for entry := range ch {
    if isInteresting(entry.Content) {
        handlePattern(entry)
    }
}
```

### Pattern 2: Timer-Based Timeout with State Tracking

**What:** Use `time.AfterFunc` for the 30-second Telegram connection timeout.
**When:** When waiting for a specific event within a time window.
**Why:** `AfterFunc` is more efficient than `time.After` for long waits because it does not allocate a channel. It can also be cancelled (`Timer.Stop()`).

```go
var timer *time.Timer
state := "idle"

for entry := range ch {
    if IsTrigger(entry.Content) && state != "waiting" {
        if timer != nil {
            timer.Stop()
        }
        state = "waiting"
        timer = time.AfterFunc(30*time.Second, func() {
            // Handle timeout
            notif.Notify("Telegram connection timeout", ...)
            state = "idle"
        })
    }
    if IsSuccess(entry.Content) && state == "waiting" {
        timer.Stop()
        state = "connected"
    }
}
```

### Pattern 3: One Monitor Per Instance (Not Per Restart)

**What:** Create one `TelegramMonitor` per instance at application startup, not on every restart.
**When:** Monitoring long-lived instances whose LogBuffer persists across restarts.
**Why:** `LogBuffer.Clear()` does not remove subscribers. The same subscriber continues receiving new entries after restart. Creating new monitors on every restart would leak goroutines and subscribers.

### Pattern 4: Interface-Based Testing

**What:** Define `LogSubscriber` and `Notifier` interfaces in the `telegram` package.
**When:** Testing `TelegramMonitor` without real LogBuffer or Pushover.
**Why:** Follows the project's established duck-typing pattern. `LogBuffer` satisfies `LogSubscriber` implicitly. Tests inject mock channel.

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Polling LogBuffer History Instead of Subscribing

**What:** Periodically calling `logBuffer.GetHistory()` to scan for patterns.
**Why bad:** Polling is wasteful (scanning all 5000 entries repeatedly), has latency gaps between polls, and scales poorly with instance count.
**Instead:** Use `Subscribe()` to receive entries in real-time via channel.

### Anti-Pattern 2: Creating New TelegramMonitor on Every Instance Restart

**What:** In `StartAfterUpdate()`, destroying old monitor and creating new one.
**Why bad:** The LogBuffer is the same object; the old subscriber still works. Creating new monitors on restart leaks goroutines and subscriber entries.
**Instead:** Create once at startup. The monitor's subscriber survives `LogBuffer.Clear()` calls.

### Anti-Pattern 3: Blocking Notification Send in Log Processing Loop

**What:** Calling `notif.Notify()` synchronously inside the `for entry := range ch` loop.
**Why bad:** Pushover HTTP call can take seconds. If it blocks, the subscriber channel fills up (capacity 100), entries get dropped, and the monitor misses the success pattern.
**Instead:** Send notifications in a separate goroutine with panic recovery (same pattern as `notification/manager.go:sendNotification()`).

### Anti-Pattern 4: Hard-Coding Log Patterns Without Verification

**What:** Assuming log pattern strings without checking actual nanobot output.
**Why bad:** If the pattern is wrong (case, spacing, wording), the monitor never triggers or never detects success.
**Instead:** Before implementation, capture actual nanobot stdout to verify exact pattern strings. Make patterns configurable constants in a separate file (`patterns.go`).

---

## Suggested Build Order

```
Phase 1: Instance Startup Notification (LOW complexity)
  Files:
    cmd/nanobot-auto-updater/main.go  -- add sendStartupNotification() function
  Test: manual verification (start app, check Pushover notification)
  Dependencies: NONE (uses existing Notifier, AutoStartResult)
  Rationale: Simplest possible change. One function in main.go.
             Tests the notification pipeline end-to-end before
             adding the more complex Telegram monitor.

Phase 2: Telegram Monitor Core Component
  Files:
    internal/telegram/monitor.go      -- TelegramMonitor struct
    internal/telegram/patterns.go     -- Pattern constants and matching
    internal/telegram/monitor_test.go -- Unit tests with mock subscriber
  Test: unit tests with mock channel, verified patterns
  Dependencies: NONE (self-contained package, uses LogBuffer interface)
  Rationale: Pure logic with no integration points. Can be tested
             fully with mocks. Pattern matching is the core value
             and should be validated in isolation first.

Phase 3: Telegram Monitor Integration
  Files:
    cmd/nanobot-auto-updater/main.go  -- create monitors, wire into startup/shutdown
  Test: manual E2E (start app with nanobot, verify monitor triggers)
  Dependencies: Phase 1, Phase 2
  Rationale: Integration is minimal (main.go only). Phase 1 already
             proved the notification pipeline works. Phase 2 proved
             the monitor logic works. This phase just connects them.

Phase 4: E2E Validation
  Test: full integration test
  Dependencies: ALL previous phases
  Rationale: Final validation after all pieces are in place.
```

### Phase Ordering Rationale

1. **Startup notification first** because it is the simplest change (one function, no new packages, no new goroutines beyond the existing async notification pattern). It validates that the Pushover notification path works in the startup context before we build something more complex on top of it.

2. **Telegram monitor core second** because it is a self-contained package with zero coupling to the rest of the application. It depends only on the `LogSubscriber` interface (satisfied by `LogBuffer`). Can be developed and tested in complete isolation.

3. **Integration third** because it is just wiring in `main.go` -- create the monitors, store references, stop on shutdown. Minimal surface area for bugs.

4. **E2E last** for final validation.

### Phase Complexity Assessment

| Phase | New Files | Modified Files | New Goroutines | Risk |
|-------|-----------|---------------|----------------|------|
| 1: Startup Notif | 0 | 1 (main.go) | 1 (async notif) | LOW |
| 2: Telegram Core | 3 | 0 | 0 (tested via mocks) | LOW |
| 3: Integration | 0 | 1 (main.go) | N (one per instance) | LOW-MED |
| 4: E2E | 0 | 0 | 0 | LOW |

---

## Configuration Impact

No new configuration is required. Both features use existing settings:

- **Pushover notifications**: Uses existing `pushover.api_token` and `pushover.user_key` from `config.yaml`. If not configured, features degrade gracefully (same as network connectivity notifications).
- **Telegram monitor timeout**: Hard-coded at 30 seconds. If configurability is needed later, add a `monitor.telegram_timeout` field to `config.yaml`.
- **Log patterns**: Defined as constants in `internal/telegram/patterns.go`. If patterns vary across nanobot versions, make them configurable later.

---

## Scalability Considerations

| Concern | At 1 instance | At 5 instances | At 20 instances |
|---------|--------------|----------------|-----------------|
| Telegram monitors | 1 goroutine + 1 subscriber | 5 goroutines + 5 subscribers | 20 goroutines + 20 subscribers |
| Startup notification | 1 Pushover message | 1 Pushover message | 1 Pushover message |
| Pattern matching per entry | 2 string.Contains calls | 2 per monitor per entry | 2 per monitor per entry |
| Memory per monitor | ~1 KB (state + timer) | ~5 KB total | ~20 KB total |

String matching is O(n) per log entry where n is the length of the pattern. Since patterns are short (<30 chars) and log entries are typically <500 chars, this is negligible even at 20 instances.

The subscriber channel capacity (100 entries) provides adequate buffering. If an instance produces >100 log lines in the time it takes to process one entry, some entries will be dropped (existing LogBuffer behavior). This is acceptable because the Telegram monitor only needs to see the trigger and success patterns once -- missing intermediate log lines does not affect correctness.

---

## Sources

- Code review of `internal/logbuffer/buffer.go` -- Write() fan-out, Subscribe()/Unsubscribe() lifecycle, Clear() does not affect subscribers (HIGH confidence)
- Code review of `internal/lifecycle/starter.go` -- captureLogs() goroutine, StartNanobotWithCapture() flow (HIGH confidence)
- Code review of `internal/instance/manager.go` -- StartAllInstances() return type AutoStartResult, graceful degradation pattern (HIGH confidence)
- Code review of `internal/notification/manager.go` -- Async notification with panic recovery pattern (HIGH confidence)
- Code review of `internal/notifier/notifier.go` -- Notifier interface, IsEnabled(), Notify() (HIGH confidence)
- Code review of `cmd/nanobot-auto-updater/main.go` -- Component wiring, shutdown sequence (HIGH confidence)
- `.planning/quick/260406-fge-PLAN.md` -- Context on nanobot Telegram connectivity issue, OpenClash proxy environment (HIGH confidence -- project documentation)

---

*Architecture research for: v0.9 Instance Startup Notifications and Telegram Connection Monitoring*
*Researched: 2026-04-06*

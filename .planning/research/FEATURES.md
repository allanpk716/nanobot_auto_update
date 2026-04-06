# Feature Landscape

**Domain:** Instance startup notifications and Telegram connection log-pattern monitoring
**Researched:** 2026-04-06
**Scope:** v0.9 milestone ONLY -- Pushover notifications for instance startup results, log-pattern-triggered Telegram connection monitoring with 30-second timeout
**Confidence:** HIGH (based on thorough codebase analysis and existing pattern inventory)

## Table Stakes

Features users expect. Missing = the monitoring service feels incomplete -- why monitor health but not tell me when startup fails?

| Feature | Why Expected | Complexity | Dependencies on Existing | Notes |
|---------|--------------|------------|--------------------------|-------|
| Instance startup result notification (Pushover) | The service already starts instances at boot and monitors health. Not notifying on startup failure means the user has no idea an instance is down until they check manually. This is a gap in the existing notification coverage. | Low | `notifier.Notifier` interface (already injected into `TriggerHandler`), `instance.AutoStartResult` struct (already returned from `StartAllInstances`), async goroutine + panic recovery pattern from `TriggerHandler` and `NotificationManager` | The `StartAllInstances` method already returns `AutoStartResult` with `Started`, `Failed`, `Skipped` fields. The notification code just needs to read this result and call `notif.Notify()`. No new infrastructure needed. |
| Per-instance startup notification | In multi-instance setups, the user needs to know which specific instance(s) failed, not just "something failed." The existing `AutoStartResult.Failed` already contains `InstanceError` with `InstanceName`, `Port`, and `Err`. | Low | Same as above. `AutoStartResult.Failed` is `[]*InstanceError` with all the fields needed. | Format the notification message to list each failed instance by name and error. Follow the same multi-instance formatting pattern as `notifier.Notifier.formatUpdateResultMessage`. |
| Telegram connection monitoring triggered by log pattern | When nanobot starts a Telegram bot, it logs "Starting Telegram bot" (or similar). If the connection then fails (httpx.ConnectError per the documented bug), the process stays running but is non-functional. The monitor should detect this pattern and notify. | Medium | `logbuffer.LogBuffer` subscriber system (`Subscribe()`/`Unsubscribe()`), `notifier.Notifier` interface, existing `captureLogs` goroutine that writes to `LogBuffer` | This is the novel feature. The existing LogBuffer already has a `Subscribe()` method that streams new log entries to a channel. A new "log watcher" component can subscribe to each instance's buffer and look for patterns. |
| 30-second timeout for Telegram connection success/failure | After "Starting Telegram bot" is detected in logs, wait up to 30 seconds for a success or failure indicator. If neither appears, treat as failure. | Low | `context.WithTimeout` (standard Go pattern used throughout codebase), `time.AfterFunc` (used in `NotificationManager`) | The timeout is a simple `time.AfterFunc` or `select` with `time.After(30*time.Second)`. No new infrastructure. |
| Failure notification on Telegram connection timeout | If Telegram connection does not succeed within 30 seconds, send Pushover notification. | Low | `notifier.Notifier` interface | Same notification pattern as all other alerts. Title like "Nanobot Telegram 连接超时" with instance name and port. |

## Differentiators

Features that elevate the monitoring quality. Not expected, but valued.

| Feature | Value Proposition | Complexity | Dependencies on Existing | Notes |
|---------|-------------------|------------|--------------------------|-------|
| Configurable log patterns per instance | Instead of hardcoding "Starting Telegram bot", allow per-instance `watch_patterns` in config.yaml. Each pattern has a trigger string, success string, failure string, and timeout. | Medium | `config.InstanceConfig` struct, `config.yaml` | Makes the log watcher generic rather than Telegram-specific. Future-proof for monitoring any subprocess lifecycle event (database connections, API server readiness, etc.). Example: `watch_patterns: [{trigger: "Starting Telegram bot", success: "Telegram bot connected", failure: "ConnectError", timeout: 30s}]` |
| Success notification for Telegram connection | Send a Pushover notification when Telegram connects successfully within the timeout window. | Low | Same notification infrastructure | Gives the user positive confirmation. Should be optional/configurable to avoid notification fatigue. |
| Log pattern matching with regex support | Support regex patterns instead of just string contains, for more flexible matching. | Low | `regexp` standard library | `strings.Contains` is sufficient for the known patterns. Regex adds flexibility for future patterns. Low cost to add. |

## Anti-Features

Features to explicitly NOT build. These are traps that would overcomplicate a focused monitoring feature.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Telegram API polling / direct health checks | The monitor's job is to watch the subprocess, not to independently verify Telegram connectivity. Polling `api.telegram.org` would duplicate nanobot's own connectivity logic and introduce proxy/authentication complexity (the whole reason nanobot has the httpx.ConnectError in the first place). | Log-pattern monitoring only. The subprocess logs are the source of truth for its own connection state. |
| Retry logic for Telegram connections | If Telegram connection fails, the monitor should notify, not retry. Retrying is nanobot's responsibility (or the user's via config). The monitor is an observer, not an actor. | Single detection + notification. If user wants retries, they configure nanobot's own retry behavior. |
| Log pattern matching via external rules engine / DSL | Over-engineering. The patterns are known and few: "Starting Telegram bot" (trigger), some form of success/failure indicator. | Hardcode the known patterns for v0.9. If needed later, add `watch_patterns` config as a differentiator. |
| WebSocket or SSE for Telegram connection status | The existing SSE infrastructure is for log streaming. Adding Telegram status streaming mixes concerns. | Pushover notification only. The user gets alerted on failure. If they want real-time status, they look at the existing log viewer. |
| Automatic remediation (restart on Telegram failure) | Restarting the instance on Telegram failure is tempting but dangerous: it could create restart loops, mask underlying proxy/network issues, and the user explicitly chose the startup command. The monitor should observe and alert, not act. | Notify only. Let the user decide whether to restart, fix proxy config, or take other action. |
| Per-instance notification routing | Sending different instances' notifications to different Pushover users/groups. | Single Pushover destination (existing pattern). If multi-user notification is needed later, it is a separate milestone. |
| Startup notification for successful auto-start (all instances succeeded) | Sending "all 3 instances started successfully" adds notification noise. The user cares about failures. | Only notify on failure or partial failure (following existing pattern from `notifier.Notifier.NotifyUpdateResult` which skips when no errors). Success notification for Telegram connection is a differentiator because it confirms a specific async lifecycle event, not a routine startup. |

## Feature Dependencies

```
Feature A: Instance startup notifications
  No hard dependencies beyond existing infrastructure.
  Reads AutoStartResult from StartAllInstances (already returns this).
  Uses Notifier.Notify() (already exists).

Feature B: Telegram connection log monitoring
  Depends on: LogBuffer subscriber system (already exists)
  Depends on: captureLogs writing to LogBuffer (already exists)
  Depends on: Log patterns being knowable (nanobot logs "Starting Telegram bot")
  No dependency on Feature A (can be built independently).

Feature B detailed flow:
  captureLogs (existing) --> LogBuffer.Write (existing) --> subscriber channel
                                                              |
  New: TelegramConnectionWatcher.Subscribe to LogBuffer       |
          |                                                   |
          +-- pattern match on "Starting Telegram bot"  <-----+
          |        |
          |        +-- start 30s timer
          |        |       |
          |        |       +-- watch for success pattern (e.g., "bot started")
          |        |       |
          |        |       +-- watch for failure pattern (e.g., "ConnectError", "error")
          |        |       |
          |        |       +-- 30s timeout without success --> notify failure
          |        |
          |        +-- found success within 30s --> (optionally notify success)
          |
          +-- no trigger pattern found --> do nothing (passive watch)
```

## Detailed Behavioral Specification

### Instance Startup Notification

**Trigger:** After `StartAllInstances` completes in `main.go`.

**Behavior:**
1. Read `AutoStartResult` (already returned).
2. If `len(result.Failed) > 0`, send Pushover notification.
3. Message includes: which instances failed (name + port + error), which succeeded.
4. If `len(result.Failed) == 0`, skip notification (success is the expected state).
5. Notification is async (goroutine + panic recovery), non-blocking.
6. If Pushover is not configured (`!notif.IsEnabled()`), log a warning and skip.

**Integration point:** `main.go` line 224, after `instanceManager.StartAllInstances(autoStartCtx)`. Add notification logic here, same pattern as `TriggerHandler`'s async notification.

**Notification format (failure):**
```
Title: Nanobot 实例启动失败
Body:
  启动失败: 2 个实例

  失败的实例:
    x nanobot-me (端口 18790)
      原因: process exited immediately after start (PID 12345)
    x nanobot-work (端口 18792)
      原因: failed to start nanobot: timeout

  成功启动的实例 (1):
    v nanobot-helper (端口 18791)
```

### Telegram Connection Monitoring

**Trigger:** Log line matching "Starting Telegram bot" (case-insensitive substring) in any instance's LogBuffer.

**Behavior:**
1. A new component (`TelegramWatcher` or generic `LogPatternWatcher`) subscribes to each instance's `LogBuffer` via the existing `Subscribe()` method.
2. Each incoming `LogEntry.Content` is checked against the trigger pattern.
3. On trigger match, start a 30-second observation window for that instance.
4. During the observation window:
   - If a line matching a success pattern appears (e.g., "bot started", "polling started"), mark as SUCCESS. Optionally notify.
   - If a line matching a failure pattern appears (e.g., "ConnectError", "ConnectionError", "error while polling"), mark as FAILED. Notify immediately.
   - If 30 seconds elapse with neither success nor failure pattern, mark as TIMEOUT. Notify.
5. Each instance can only have one active observation window at a time (prevent duplicates if "Starting Telegram bot" appears multiple times).

**Known patterns from nanobot (python-telegram-bot library):**
- Trigger: `"Starting Telegram bot"` or `"Starting bot"` (nanobot gateway startup log)
- Success: `"started polling"` or `"bot started"` (python-telegram-bot successful connection)
- Failure: `"ConnectError"`, `"ConnectionError"`, `"error while polling"`, `"NetworkError"` (httpx/telegram error patterns)

**Note on pattern accuracy:** The exact log strings need to be verified against actual nanobot output. The patterns above are based on python-telegram-bot source analysis and the documented httpx.ConnectError. This is a known validation point -- the implementation phase should capture real nanobot logs to confirm patterns.

**Integration point:** After `instanceManager` creation in `main.go`, create and start the watcher for each instance that has a LogBuffer.

**Notification format (failure):**
```
Title: Nanobot Telegram 连接失败
Body:
  实例: nanobot-me (端口 18790)
  原因: 30 秒内未检测到连接成功

  最近日志:
    2026-04-06 10:00:01 - Starting Telegram bot...
    2026-04-06 10:00:02 - httpx.ConnectError: ...
```

## MVP Recommendation

**Build in this order (2 phases):**

1. **Phase 1: Instance startup notifications**
   - Add notification call in `main.go` after `StartAllInstances`
   - Format message from `AutoStartResult`
   - Async with panic recovery
   - Only notify on failure/partial failure
   - Tests: mock Notifier, verify message format, verify no notification on success

2. **Phase 2: Telegram connection monitoring**
   - New `internal/logwatcher/` package
   - `LogPatternWatcher` struct with `Subscribe()` to LogBuffer
   - Pattern matching (strings.Contains for v0.9, not regex)
   - 30-second timeout with `time.AfterFunc`
   - Pushover notification on failure/timeout
   - Tests: pattern matching, timeout behavior, success detection, multi-instance isolation

**Defer:**
- Configurable log patterns (use hardcoded known patterns for v0.9, add config later if needed)
- Success notification for Telegram connection (add as option after core monitoring works)
- Regex pattern support (strings.Contains is sufficient for known patterns)

## Existing Infrastructure Inventory

These components already exist and will be reused. No new packages or libraries needed.

| Component | Location | How It's Used for New Features |
|-----------|----------|-------------------------------|
| `notifier.Notifier` interface | `internal/notifier/notifier.go` | `Notify(title, message)` for all Pushover notifications. Already injected everywhere. |
| `notifier.Notifier.IsEnabled()` | `internal/notifier/notifier.go` | Check before sending. Graceful degradation when Pushover not configured. |
| `logbuffer.LogBuffer` | `internal/logbuffer/buffer.go` | Per-instance circular buffer. `Write()` sends to all subscribers. |
| `logbuffer.Subscribe()` | `internal/logbuffer/subscriber.go` | Returns `<-chan LogEntry`. New watcher subscribes to receive real-time logs. |
| `logbuffer.Unsubscribe()` | `internal/logbuffer/subscriber.go` | Clean up subscriber on shutdown. |
| `LogEntry{Timestamp, Source, Content}` | `internal/logbuffer/buffer.go` | Each captured log line. Content is the raw subprocess output. |
| `instance.AutoStartResult` | `internal/instance/manager.go` | `Started []string`, `Failed []*InstanceError`, `Skipped []string`. Already populated. |
| `instance.InstanceError` | `internal/instance/errors.go` | `InstanceName`, `Operation`, `Port`, `Err`. Rich error info for notifications. |
| `instance.InstanceManager.GetLifecycle()` | `internal/instance/manager.go` | Get specific instance by name, access its LogBuffer. |
| `instance.InstanceLifecycle.GetLogBuffer()` | `internal/instance/lifecycle.go` | Returns the instance's LogBuffer. Used to subscribe for log watching. |
| Async notification pattern | `internal/api/trigger.go`, `internal/notification/manager.go` | Goroutine + `defer recover` + `debug.Stack()`. Copy this pattern exactly. |
| `notification.Notifier` interface (local) | `internal/notification/manager.go` | `IsEnabled()`, `Notify(title, message)`. Same duck-typed interface. |
| Context + cancel pattern | Multiple files | `context.WithCancel(context.Background())` for lifecycle control. |
| `config.Config` + viper | `internal/config/config.go` | Add new config fields if needed (watch patterns, timeouts). |

## Edge Cases to Consider

| Edge Case | Handling |
|-----------|----------|
| Instance starts, but "Starting Telegram bot" never appears in logs | Watcher stays passive. No false positive. Only activates on trigger pattern. |
| Multiple "Starting Telegram bot" lines in quick succession | Only one active observation window per instance. New trigger cancels/replaces the old timer. |
| Instance stops and restarts during observation window | LogBuffer.Clear() is called on restart. Subscriber receives new logs. Old observation should be cancelled on Clear. |
| Pushover is not configured | `notif.IsEnabled()` returns false. Log warning, skip notification. Same graceful degradation as all existing notifications. |
| LogBuffer subscriber channel full (100 entries) | Non-blocking send drops logs (existing behavior). Watcher may miss patterns. Log a warning if pattern is missed during observation window. |
| Very slow log output (nanobot takes 29 seconds to log anything) | 30-second timeout may fire before any log appears. This is correct behavior -- if Telegram doesn't connect in 30s, it is a failure. |
| Nanobot logs in non-English | Known risk. v0.9 hardcodes English patterns from python-telegram-bot. If nanobot localization changes log messages, patterns will break. Flagged for validation. |
| Instance has no Telegram channel configured | "Starting Telegram bot" never appears. Watcher stays passive. No false positive. |

## Sources

- Codebase analysis: `internal/notifier/notifier.go` -- Notifier interface and Pushover integration (HIGH confidence, direct source)
- Codebase analysis: `internal/logbuffer/subscriber.go` -- Subscribe/Unsubscribe channel pattern (HIGH confidence, direct source)
- Codebase analysis: `internal/instance/manager.go` -- `AutoStartResult` and `StartAllInstances` (HIGH confidence, direct source)
- Codebase analysis: `internal/api/trigger.go` -- Async notification pattern with panic recovery (HIGH confidence, direct source)
- Codebase analysis: `internal/notification/manager.go` -- NotificationManager with cooldown timer pattern (HIGH confidence, direct source)
- Codebase analysis: `.planning/quick/260406-fge-*/260406-fge-PLAN.md` -- Telegram httpx.ConnectError root cause and nanobot proxy behavior (HIGH confidence, documented diagnosis)
- python-telegram-bot library: known log patterns for bot startup, polling, and connection errors (MEDIUM confidence, based on library behavior understanding; exact patterns need validation against real nanobot output)

---
*Feature research for: v0.9 Instance Startup Notifications + Telegram Connection Monitoring milestone*
*Researched: 2026-04-06*

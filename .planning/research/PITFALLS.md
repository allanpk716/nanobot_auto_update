# Domain Pitfalls: Instance Startup Notifications and Telegram Connection Monitoring

**Domain:** Adding log-pattern monitoring with timeout and instance lifecycle notifications to an existing Go Windows service (nanobot-auto-updater)
**Researched:** 2026-04-06
**Overall confidence:** HIGH (derived from codebase analysis and verified Go concurrency patterns)

## Critical Pitfalls

Mistakes that cause rewrites, notification spam, or silent monitoring failures.

---

### Pitfall 1: Log Pattern Scanner Race with LogBuffer Write

**What goes wrong:**
A new log-pattern scanner subscribes to LogBuffer to detect "Starting Telegram bot". The scanner goroutine reads LogEntry from the subscriber channel, but the LogBuffer's non-blocking send (`select + default`) can drop entries when the channel is full (capacity 100). If the "Starting Telegram bot" line is dropped, the Telegram connection monitor never activates, and a real connection failure goes undetected.

**Why it happens:**
- LogBuffer.Write() uses non-blocking send: slow subscribers get entries dropped at WARN level
- The existing subscriber pattern was designed for SSE streaming (loss of one log line is acceptable for UI display)
- A log-pattern monitor has different requirements: missing a single trigger line means the entire monitoring feature silently fails
- The scanner goroutine might be slow because it does string matching on every entry, or because it was blocked sending a Pushover notification synchronously

**How to avoid:**
1. The pattern scanner goroutine must NEVER block on anything other than reading from the subscriber channel -- no synchronous notification sends, no file I/O, no network calls
2. When the trigger pattern is detected, hand off to a SEPARATE goroutine for the timeout monitoring and notification logic
3. Consider using a dedicated channel (not the subscriber channel) for pattern detection, or use a callback hook in LogBuffer.Write() instead of the subscriber pattern
4. If using the subscriber channel, ensure the scanner goroutine is always ready to receive (use `select` with only the channel read, no additional cases that could block)

**Warning signs:**
- LogBuffer WARN logs showing "Subscriber channel full, dropping log" during instance startup
- Telegram connection monitor never activates even though nanobot output contains "Starting Telegram bot"
- Tests pass in isolation but fail under load (multiple instances starting simultaneously)

**Phase to address:**
Phase implementing log-pattern scanner -- the scanner must be designed from the start with the non-blocking constraint in mind.

---

### Pitfall 2: time.AfterFunc Callback Accesses Shared State Without Mutex

**What goes wrong:**
The Telegram connection monitor uses `time.AfterFunc(30*time.Second, callback)` to implement the 30-second timeout. The callback function captures a pointer to the monitor state (e.g., `monitor.pendingInstances`). The main goroutine and the AfterFunc callback goroutine both read/write this state without synchronization, causing a data race.

**Why it happens:**
- `time.AfterFunc` runs its callback in a NEW goroutine (documented behavior)
- The callback closure captures variables by reference -- the outer goroutine may have modified them by the time the callback runs
- The existing NotificationManager (network monitor) already has this pattern correctly: `confirmAndNotify` is called from AfterFunc and accesses state protected by `nm.mu.Lock()`
- But a NEW component may forget to follow this pattern

**How to avoid:**
1. Any state accessed by the AfterFunc callback MUST be protected by a mutex (the same pattern as NotificationManager)
2. Lock the mutex at the start of the callback, defer unlock
3. Check if the state is still relevant (the instance may have already been stopped/restarted by the time the 30 seconds elapse)
4. Follow the exact pattern from `internal/notification/manager.go`: `confirmAndNotify` acquires the lock, checks if the state is still current, then proceeds or aborts

**Warning signs:**
- `go test -race` detects data race in timeout callback
- Intermittent nil pointer dereference in AfterFunc callback
- Notification sent for a stale instance (instance was restarted during the 30s window)

**Phase to address:**
Phase implementing Telegram connection timeout monitor -- mutex must be in the initial design.

---

### Pitfall 3: Notification Spam on Service Restart Loop

**What goes wrong:**
The nanobot-auto-updater service restarts (due to self-update or manual restart). Each restart triggers auto-start of all instances. Each instance start sends a Pushover notification. If the service restarts in a loop (e.g., self-update keeps failing), the user receives dozens of identical notifications within minutes. Pushover free tier limits at 7,500 messages/month -- a restart loop could exhaust this quota in hours.

**Why it happens:**
- The new feature adds startup notifications for every instance on every application start
- The existing v0.7 notification system only sends notifications on explicit update triggers (not on every startup)
- There is no cooldown or deduplication for startup notifications
- The self-update mechanism (v0.8) can cause restart loops if the new binary keeps crashing
- Multiple instances = multiple notifications per restart

**How to avoid:**
1. Implement a startup notification cooldown: track the last time a startup notification was sent per instance. If the same instance sent a notification less than N minutes ago, suppress the duplicate
2. The cooldown value should be configurable, with a sensible default (e.g., 5 minutes)
3. Consider aggregating all instance startup results into a SINGLE notification rather than one per instance
4. On startup, send one summary notification ("3 instances started, 0 failed") instead of 3 individual notifications
5. Store the cooldown state in memory (a simple `map[string]time.Time` with mutex) -- no need for persistence since service restart resets the cooldown (which is correct behavior)

**Warning signs:**
- User reports receiving many notifications in short succession
- Pushover API returns rate-limit errors (500ms between calls minimum)
- Log shows notification sent every few seconds during a restart loop

**Phase to address:**
Phase implementing startup notifications -- the cooldown/aggregation must be designed before writing the first notification send call.

---

### Pitfall 4: Telegram Connection Monitor Activates on Historical Logs

**What goes wrong:**
The log-pattern scanner subscribes to LogBuffer, and the Subscribe() method replays ALL historical entries first (see `subscriberLoop` in subscriber.go). If the LogBuffer already contains "Starting Telegram bot" from a previous run, the scanner detects it and starts a 30-second timeout monitor for a Telegram connection that is already established (or already failed). This results in a spurious "Telegram connection failed" notification 30 seconds after the auto-updater restarts.

**Why it happens:**
- LogBuffer.Subscribe() sends history logs before real-time logs (by design, for SSE UI)
- The subscriber loop iterates `GetHistory()` and sends each entry to the channel
- The pattern scanner does not distinguish between historical entries and real-time entries
- On application restart, LogBuffer is cleared (Clear() is called before start), BUT the race window exists: if the instance starts and outputs "Starting Telegram bot" before the scanner subscribes, the entry is already in the buffer and will be replayed

**How to avoid:**
1. Add a `timestamp` or `sequence` filter: only process log entries that were written AFTER the scanner was initialized
2. Store a `startTime time.Time` when the scanner is created, and ignore any LogEntry with `Timestamp.Before(startTime)`
3. Alternatively, add a LogBuffer method that subscribes WITHOUT history replay (a "real-time only" subscribe)
4. This is the most robust approach: the pattern scanner only cares about NEW log entries, never historical ones

**Warning signs:**
- After restarting auto-updater, a spurious "Telegram connection failed" notification arrives 30 seconds later
- The notification mentions an instance that has been running successfully for hours
- The 30-second timeout fires for a Telegram connection that is actually working

**Phase to address:**
Phase implementing log-pattern scanner -- the history-replay filter must be part of the initial scanner design.

---

### Pitfall 5: Instance Stop/Restart During Active Telegram Monitor

**What goes wrong:**
The Telegram connection monitor is active (waiting for "connected" pattern within 30 seconds). During this window, the user triggers an update (via API or cron). The update stops the instance, which kills the nanobot process. The log capture goroutine detects EOF and exits. The "connected" pattern never appears. After 30 seconds, the timeout fires and sends a "Telegram connection failed" notification -- but the failure is EXPECTED because the instance was intentionally stopped.

**Why it happens:**
- The update trigger (cron or API) does not coordinate with the Telegram connection monitor
- The monitor has no awareness of instance lifecycle (stop/update/start)
- The 30-second timeout cannot distinguish between "genuinely failed to connect" and "instance was stopped for update"
- The stop/start cycle takes ~30-60 seconds, which overlaps perfectly with the 30-second monitor window

**How to avoid:**
1. The Telegram connection monitor must be notified when an instance is about to be stopped
2. Implement a cancellation mechanism: when StopForUpdate() is called, cancel any active Telegram monitor for that instance
3. Use `context.Context` cancellation: pass a cancellable context to the monitor, and cancel it when the instance stops
4. In the AfterFunc callback, check if the monitor was cancelled before sending the notification
5. The monitor should also be cancelled when the instance is restarted (a new monitor starts for the new startup)

**Warning signs:**
- Spurious "Telegram connection failed" notifications arriving ~30 seconds after a cron-triggered update
- Notifications arrive during the update window (between stop and start)
- Users confused by failure notifications when the update eventually succeeds

**Phase to address:**
Phase implementing Telegram connection timeout -- the cancellation mechanism must be designed alongside the timeout.

---

### Pitfall 6: Scanner Goroutine Leak on Instance Restart

**What goes wrong:**
Each time an instance starts, a new log-pattern scanner goroutine is created. When the instance is stopped and restarted, a NEW scanner is created but the OLD one is never properly cleaned up. Over multiple update cycles, goroutines accumulate. After 100 update cycles, there are 100 idle scanner goroutines per instance, each holding a subscriber channel in the LogBuffer.

**Why it happens:**
- The existing LogBuffer.Unsubscribe() exists but requires the caller to track the channel and call Unsubscribe
- If the scanner goroutine is blocked on something (e.g., waiting for the 30-second timeout), it cannot respond to the Unsubscribe cancellation
- The captureLogs goroutine already uses `detachedCtx` (context.Background()) to survive parent cancellation -- this means the scanner goroutine attached to it may also outlive expectations
- The existing pattern in InstanceLifecycle.StartAfterUpdate() calls `logBuffer.Clear()` but does NOT unsubscribe existing subscribers

**How to avoid:**
1. Use context.Context for scanner lifecycle management (same pattern as captureLogs)
2. When creating a scanner, pass a cancellable context. Store the cancel function alongside the scanner
3. When the instance stops or restarts, cancel the context. The scanner goroutine should check `ctx.Done()` and exit cleanly
4. In the scanner goroutine, always `defer LogBuffer.Unsubscribe(ch)` to ensure cleanup
5. Track active scanners per instance in InstanceLifecycle: `scannerCancel context.CancelFunc` field

**Warning signs:**
- `runtime.NumGoroutine()` steadily increases after each update cycle
- LogBuffer WARN logs showing "Subscriber channel full" with increasing channel capacity
- Memory usage grows over days/weeks of operation

**Phase to address:**
Phase implementing log-pattern scanner -- context lifecycle must be designed in the initial scanner struct.

---

## Moderate Pitfalls

### Pitfall 7: String Matching Too Narrow or Too Broad

**What goes wrong:**
The scanner looks for the exact string "Starting Telegram bot" but nanobot outputs a slightly different format in a new version (e.g., "Starting telegram bot" with lowercase, or "Telegram bot starting" with different word order). Alternatively, the pattern is too broad (e.g., just "Telegram") and matches log lines that are not the actual startup indicator.

**How to avoid:**
1. Use `strings.Contains()` with a case-insensitive match for robustness: check for "starting telegram" or "telegram bot" as a substring
2. Make the trigger pattern configurable in config.yaml so it can be updated without code changes
3. Log every pattern match at DEBUG level so mismatches can be diagnosed
4. Test against actual nanobot output -- look at real log lines in the LogBuffer to verify the exact format
5. Consider matching on multiple patterns: "Starting Telegram bot" to start monitoring, AND a positive pattern like "Telegram bot connected" or "Bot @xxx started" to detect success

**Warning signs:**
- Telegram monitor never activates even though nanobot starts its Telegram channel
- False positives from unrelated log lines mentioning "Telegram"

**Phase to address:**
Phase implementing log-pattern scanner -- pattern should be verified against real nanobot output.

---

### Pitfall 8: 30-Second Timeout Too Short or Too Long

**What goes wrong:**
The 30-second timeout for Telegram connection is a fixed guess. Under OpenClash proxy (the known environment), the connection may take longer due to proxy hops. If too short, every startup triggers a false failure notification. If too long, real failures are not detected promptly, and the user is unaware for 30+ seconds.

**How to avoid:**
1. Make the timeout configurable in config.yaml (e.g., `telegram.monitor_timeout: 30s`)
2. Start with 30 seconds as default but allow override
3. Measure actual connection times over a week of operation and tune
4. Consider a two-phase approach: warn at 15 seconds, fail at 30 seconds
5. The existing OpenClash proxy environment may cause higher latency -- account for this

**Phase to address:**
Phase implementing Telegram connection monitor.

---

### Pitfall 9: Notification Not Sent on Instance Startup FAILURE

**What goes wrong:**
The startup notification feature is designed to notify on success AND failure. But the failure path is easy to miss: if `StartNanobotWithCapture()` returns an error (process exits immediately, port verification fails), the notification must still be sent. The error handling in `StartAllInstances()` catches the error but the notification hook may not be wired into the error path.

**How to avoid:**
1. The notification must be sent from the same location that handles the startup error (in StartAllInstances or InstanceLifecycle)
2. Do NOT rely on the caller to send the notification -- wire it directly into the start flow
3. Follow the existing pattern from TriggerHandler: the notification is sent whether the operation succeeds or fails
4. Test the failure path explicitly: start an instance with an invalid command and verify the failure notification arrives

**Phase to address:**
Phase implementing startup notifications.

---

### Pitfall 10: Pushover Notification Send Blocks Startup Flow

**What goes wrong:**
The Pushover notification is sent synchronously during instance startup. The HTTP call to Pushover API takes 1-5 seconds. For multiple instances starting sequentially, this adds 1-5 seconds PER INSTANCE to the total startup time. With 3 instances, that is 3-15 seconds of additional delay.

**Why it happens:**
- The existing pattern in TriggerHandler and NotificationManager already uses async goroutines for notification sends
- But the NEW startup notification code may be placed in a synchronous path (e.g., inside StartAllInstances loop)
- The existing pattern has the correct solution: `go func() { defer recover(); notifier.Notify() }()`

**How to avoid:**
1. ALWAYS send Pushover notifications asynchronously using a goroutine with panic recovery
2. Follow the exact pattern from `internal/notification/manager.go` sendNotification():
   ```go
   go func() {
       defer func() {
           if r := recover(); r != nil {
               logger.Error("notification goroutine panic", "panic", r, "stack", string(debug.Stack()))
           }
       }()
       if err := notifier.Notify(title, message); err != nil {
           logger.Error("notification failed", "error", err)
       }
   }()
   ```
3. This is a non-negotiable pattern in this codebase -- violation breaks the "non-blocking" principle

**Phase to address:**
Phase implementing startup notifications -- must be async from the first line of implementation.

---

### Pitfall 11: Multiple Simultaneous Telegram Monitors for Same Instance

**What goes wrong:**
Instance starts, scanner detects "Starting Telegram bot", starts a 30-second Telegram connection monitor. Before the 30 seconds expire, the instance crashes and restarts. A NEW "Starting Telegram bot" line is detected. A SECOND monitor is started for the same instance. After 30 seconds, BOTH monitors fire -- the first one sends a "failed" notification (because the original connection attempt died with the crash), and the second one may send another notification.

**How to avoid:**
1. Track the active monitor per instance using a map: `map[string]context.CancelFunc` (instance name -> cancel function)
2. When a new "Starting Telegram bot" is detected, cancel the previous monitor for that instance BEFORE starting a new one
3. The AfterFunc callback should check if it is still the "current" monitor before sending notification
4. Use a generation counter or unique ID per monitor to detect stale callbacks

**Phase to address:**
Phase implementing Telegram connection monitor.

---

## Minor Pitfalls

### Pitfall 12: LogBuffer.Clear() Destroys Scanner Context

**What goes wrong:**
InstanceLifecycle.StartAfterUpdate() calls `logBuffer.Clear()` before starting the instance. This resets the buffer but does NOT affect subscribers (subscribers map unchanged per the code comment). However, if the scanner relied on the buffer being non-empty for some initialization logic, Clear() could break assumptions.

**Prevention:**
- Do not rely on LogBuffer contents for scanner initialization
- The scanner should use the real-time stream exclusively (or with the timestamp filter described in Pitfall 4)
- Clear() is fine -- it clears history but subscribers keep receiving new entries

---

### Pitfall 13: Missing Notifier Nil Check

**What goes wrong:**
The startup notification code calls `notifier.Notify()` but the notifier may be nil (if Pushover is not configured). The existing notifier handles this internally (`IsEnabled()` check in Notify), but if a DIFFERENT notification interface is used without the nil-check, a nil pointer dereference occurs.

**Prevention:**
- Follow the existing pattern: check `if notifier != nil` before calling, AND use the Notifier interface that handles IsEnabled() internally
- See `internal/api/trigger.go` line 77: `if h.notifier != nil { ... }`

---

### Pitfall 14: Test-only Notifier Mock Missing New Methods

**What goes wrong:**
The existing test mock for Notifier (recordingNotifier in trigger_test.go) implements `Notify(title, message) error`. If new notification methods are added to the Notifier interface (e.g., a specialized startup notification method), existing tests break because the mock no longer satisfies the interface.

**Prevention:**
- Keep the Notifier interface minimal: single method `Notify(title, message) error`
- Build notification content (title, message formatting) at the caller level, not in the Notifier
- Do NOT add specialized methods like `NotifyStartup()` or `NotifyTelegramFailure()` to the interface
- The duck-typing pattern in this codebase explicitly favors minimal single-method interfaces

---

## Technical Debt Patterns

Shortcuts that seem reasonable but create long-term problems.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Fixed 30s timeout hardcoded | Faster to implement | Cannot tune for proxy environments, false alarms | MVP only, then make configurable |
| Per-instance notification (no aggregation) | Simpler code | 3 instances = 3 notifications, spammy | Never -- always aggregate into single notification |
| Scanner pattern hardcoded in Go | No config parsing needed | Pattern change requires code rebuild | MVP only, then make configurable |
| No notification deduplication | Simpler code | Restart loop floods user | Never -- at minimum, track last-sent timestamps |
| Skip cancel on instance stop | Less wiring | Spurious failure notifications during updates | Never -- must cancel monitors on stop |

## Integration Gotchas

Common mistakes when connecting to existing components.

| Integration Point | Common Mistake | Correct Approach |
|-------------------|----------------|------------------|
| LogBuffer + scanner | Use Subscribe() with history replay | Add timestamp filter or real-time-only subscribe |
| NotificationManager + startup | Send notification synchronously in start loop | Async goroutine with panic recovery (existing pattern) |
| InstanceLifecycle + Telegram monitor | No coordination between stop and monitor | Cancel active monitor on StopForUpdate() |
| Notifier + startup notification | Add new methods to Notifier interface | Keep single Notify(title, message) interface, format at caller |
| AutoStart + notification | Send one notification per instance | Aggregate all results into single summary notification |
| time.AfterFunc + state | Access monitor state without mutex | Lock mutex in callback, same as NotificationManager pattern |
| Config + new features | Add required fields without defaults | All new config fields must have sensible defaults for backward compat |

## Performance Traps

Patterns that work at small scale but fail as usage grows.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Per-line string scan on every log entry | CPU spike during verbose nanobot output | Only scan lines written AFTER "Starting Telegram bot" detected -- stop scanning once monitor is active | 10+ instances with verbose logging |
| Subscriber channel (capacity 100) for pattern scanner | Dropped trigger lines under burst logging | Use dedicated callback or larger channel for scanner | Burst of 100+ lines in <1ms (nanobot startup output) |
| Unbounded goroutine creation for notification sends | Goroutine leak over weeks of operation | Use sync.WaitGroup or worker pool for notification goroutines | After weeks of continuous operation with frequent restarts |
| strings.Contains on every log line for all patterns | Linear scan overhead per line | Compile pattern once, short-circuit after trigger found | Minimal impact -- strings.Contains is fast for short patterns |

## Security Mistakes

Domain-specific security issues.

| Mistake | Risk | Prevention |
|---------|------|------------|
| Log injection: nanobot output contains crafted "Starting Telegram bot" | False trigger of Telegram monitor, spurious notifications | Trust nanobot output (same machine, no external injection vector). LOW risk. |
| Pushover token logged | Credential exposure | Never log Pushover tokens; existing codebase handles this correctly |
| Notification content includes sensitive data | Information leak via Pushover | Keep notification content minimal: instance name, status, no ports/commands |

## UX Pitfalls

Common user experience mistakes in this domain.

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Success notification on every startup | Notification fatigue, user ignores notifications | Only notify on failure (existing pattern from v0.5: "conditional notification mode") |
| No notification when Telegram fails silently | User unaware bot is down for hours | This IS the feature we are building -- ensure it works reliably |
| Vague notification message: "Telegram connection failed" | User does not know which instance or what to do | Include instance name, port, and hint: "Check proxy/network settings" |
| Notification arrives 30s after failure (too slow) | User already aware via other means | 30s is acceptable for background monitoring; not a real-time alert system |

## "Looks Done But Isn't" Checklist

Things that appear complete but are missing critical pieces.

- [ ] **Startup notification sends on success**: But does it ALSO send on failure? Verify both paths tested
- [ ] **Telegram monitor detects "Starting Telegram bot"**: But does it handle case sensitivity? Test with actual nanobot output
- [ ] **30-second timeout fires on failure**: But is it cancelled when instance stops? Verify no spurious notification during update
- [ ] **Scanner subscribes to LogBuffer**: But does it ignore historical entries? Test after application restart
- [ ] **Notification is async (goroutine)**: But does it have panic recovery? Every goroutine MUST have `defer recover()`
- [ ] **Notifier nil check**: But does the new code path check `if notifier != nil`? Follow existing pattern
- [ ] **Multiple instances**: Does the code handle 2+ instances starting simultaneously? Test with multi-instance config
- [ ] **Pushover not configured**: Does the feature gracefully degrade when Pushover is disabled? Must not crash or block

## Recovery Strategies

When pitfalls occur despite prevention, how to recover.

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Notification spam (restart loop) | LOW | Add cooldown in next version; user can disable Pushover temporarily |
| Spurious Telegram failure notification | LOW | Add cancel-on-stop in next version; user can ignore the one-off notification |
| Scanner goroutine leak | MEDIUM | Fix with proper context cancellation; requires redeployment; leak is slow (does not crash immediately) |
| Missed Telegram failure (dropped log line) | HIGH | Redesign scanner to use callback instead of subscriber channel; requires code change |
| False positive from historical logs | LOW | Add timestamp filter; one-time spurious notification, no lasting damage |
| Multiple monitors for same instance | LOW | Add cancel-previous logic; extra notification in the meantime is the only symptom |

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Scanner race with LogBuffer write | Log-pattern scanner phase | `go test -race`; test with burst of log lines exceeding channel capacity |
| AfterFunc shared state race | Telegram monitor phase | `go test -race`; verify callback acquires mutex |
| Notification spam on restart | Startup notification phase | Test: restart service 5 times rapidly, count notifications received |
| Historical log false trigger | Log-pattern scanner phase | Test: start scanner AFTER instance already output trigger line |
| Instance stop during active monitor | Telegram monitor phase | Test: trigger update while Telegram monitor is active, verify no spurious notification |
| Scanner goroutine leak | Log-pattern scanner phase | Test: run 100 update cycles, check `runtime.NumGoroutine()` stabilizes |
| String matching too narrow | Log-pattern scanner phase | Test against actual nanobot output captured in LogBuffer |
| 30s timeout too short | Telegram monitor phase | Test with configurable timeout; verify under proxy environment |
| Missing failure notification | Startup notification phase | Test: start instance with invalid command, verify failure notification |
| Notification blocks startup | Startup notification phase | Benchmark: verify startup time unchanged with Pushover configured vs not |
| Multiple monitors same instance | Telegram monitor phase | Test: crash and restart instance during 30s window, verify single notification |

## Existing Patterns to Reuse

The project already has patterns that should be extended for these features.

| Existing Pattern | Location | How to Reuse |
|-----------------|----------|--------------|
| Async notification with panic recovery | `internal/notification/manager.go:sendNotification()` | Copy exact goroutine pattern for startup and Telegram notifications |
| `time.AfterFunc` with mutex-protected state | `internal/notification/manager.go:confirmAndNotify()` | Same pattern for Telegram timeout: lock mutex in callback, verify state is current |
| Notifier nil check | `internal/api/trigger.go:77` | Always check `if notifier != nil` before calling |
| Notifier interface (single method) | `internal/api/trigger.go:30-32` | Do NOT extend interface -- keep `Notify(title, message) error` |
| Context cancellation for goroutines | `internal/lifecycle/starter.go:captureLogs()` | Use same pattern for scanner goroutine lifecycle |
| LogBuffer subscriber pattern | `internal/logbuffer/subscriber.go` | Subscribe/Unsubscribe for real-time log stream; add timestamp filter |
| Graceful degradation | `internal/notifier/notifier.go:IsEnabled()` | All notification paths must handle disabled Pushover gracefully |
| Instance error wrapping | `internal/instance/errors.go` | Wrap Telegram monitor errors in InstanceError for consistency |

## Sources

**Go Concurrency and Timer Patterns:**
- [Go Race Detector](https://go.dev/blog/race-detector) -- Official blog on using `-race` flag (HIGH confidence)
- [time.AfterFunc runs callback in new goroutine](https://www.reddit.com/r/golang/comments/13echul/can_the_timer_returned_by_timeafterfunc_be_safely/) -- AfterFunc behavior documentation (HIGH confidence)
- [Go Timer Reset race conditions](https://groups.google.com/g/golang-codereviews/c/ky9VwFpPzpg) -- Official Go code review on Reset safety (HIGH confidence)
- [100 Go Mistakes and How to Avoid Them](https://100go.co/) -- Community compilation of common mistakes (HIGH confidence)

**Notification Patterns:**
- [Pushover API rate limits](https://pushover.net/api) -- 7,500 messages/month free, 500ms between calls (HIGH confidence)
- [Building a queue for Pushover API](https://stackoverflow.com/questions/34246132/building-a-queue-for-sending-notifications-to-the-pushover-api) -- Rate limiting strategy (MEDIUM confidence)

**String Matching:**
- [Benchmarking: Substrings vs Regex in Go](https://betterprogramming.pub/benchmarking-in-go-substrings-vs-regular-expressions-a84de7f0eb02) -- strings.Contains is the right choice for simple substring matching (HIGH confidence)
- [strings.Contains wraps strings.Index](http://www.ebmesem.com/2015/06/17/on-go-string-cruise.html) -- No performance difference, Contains is more readable (HIGH confidence)

**Existing Codebase (highest confidence):**
- `internal/notification/manager.go` -- AfterFunc + mutex pattern, async notification with panic recovery
- `internal/logbuffer/subscriber.go` -- Subscribe/Unsubscribe pattern with history replay
- `internal/logbuffer/buffer.go` -- Non-blocking subscriber send (select+default drop pattern)
- `internal/lifecycle/starter.go` -- Log capture goroutine lifecycle with context cancellation
- `internal/instance/lifecycle.go` -- Instance start/stop with LogBuffer.Clear() and PID tracking
- `internal/instance/manager.go` -- StartAllInstances with graceful degradation pattern
- `internal/notifier/notifier.go` -- IsEnabled() and Notify() with graceful degradation
- `internal/api/trigger.go` -- Async notification pattern, Notifier interface definition
- `cmd/nanobot-auto-updater/main.go` -- Auto-start goroutine with panic recovery and context timeout

**Context from Project:**
- `.planning/quick/260406-fge-nanobot-telegram-httpx-connecterror-open/260406-fge-PLAN.md` -- Telegram connection fails under OpenClash proxy due to TLS MITM; relevant for timeout calibration (HIGH confidence)

---
*Pitfalls research for: Instance Startup Notifications and Telegram Connection Monitoring for nanobot-auto-updater*
*Researched: 2026-04-06*

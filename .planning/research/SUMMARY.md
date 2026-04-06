# Project Research Summary

**Project:** nanobot-auto-updater v0.9
**Domain:** Instance startup notifications and log-pattern-based Telegram connection monitoring for an existing Go Windows service
**Researched:** 2026-04-06
**Confidence:** HIGH

## Executive Summary

This milestone adds two notification features to the existing nanobot-auto-updater Go service: (1) Pushover notifications reporting instance startup results after auto-start, and (2) a log-pattern-triggered Telegram connection monitor that detects "Starting Telegram bot" in subprocess output, waits up to 30 seconds for connection confirmation, and alerts on failure or timeout. Both features are narrow in scope and integrate cleanly into the established architecture -- no new external dependencies, no new goroutine patterns, no structural changes.

The recommended approach is to build these as two independent additions wired into `main.go`. The startup notification is a single function call after `StartAllInstances()` that formats the `AutoStartResult` into a Pushover message. The Telegram monitor is a new self-contained `internal/telegram/` package that subscribes to the existing `LogBuffer` per-instance, scans for patterns using `strings.Contains`, and uses `time.AfterFunc` for the 30-second timeout. Both reuse the existing `Notifier` interface, async goroutine-with-panic-recovery pattern, and graceful degradation when Pushover is not configured.

The key risks are concurrency pitfalls: `time.AfterFunc` callbacks accessing shared state without mutex protection (data race), log entries being dropped by the non-blocking subscriber channel (missed trigger), historical log replay causing false triggers on restart, and notification spam during service restart loops. All have clear prevention strategies drawn from patterns already present in the codebase, particularly the `NotificationManager`'s mutex-protected `AfterFunc` callback pattern and the context-cancellation lifecycle pattern from `captureLogs`. The one validation gap is exact log pattern strings from nanobot's python-telegram-bot output, which must be confirmed against real subprocess logs before hardcoding.

## Key Findings

### Recommended Stack

Zero new dependencies. Everything uses Go stdlib (`strings`, `time`, `context`, `sync`) and existing internal packages (`logbuffer`, `notifier`, `notification`). This is the lowest-risk milestone possible from a dependency standpoint.

**Core technologies:**
- `strings.Contains()` for log pattern matching -- sufficient for fixed literal patterns, no regex overhead
- `time.AfterFunc()` for 30-second timeout with `Timer.Stop()` cancellation -- same pattern as existing `NotificationManager` cooldown timer
- `logbuffer.LogBuffer.Subscribe()` / `Unsubscribe()` for real-time log streaming to the Telegram monitor
- `notifier.Notifier` interface (`IsEnabled()`, `Notify(title, message)`) for all Pushover delivery -- no interface changes needed

### Expected Features

**Must have (table stakes):**
- Instance startup result notification (Pushover) after `StartAllInstances()` completes -- users need to know if instances failed at boot
- Per-instance failure details in the notification message (instance name, port, error) from `AutoStartResult.Failed`
- Telegram connection monitoring triggered by "Starting Telegram bot" log pattern with 30-second timeout
- Failure notification when Telegram connection does not succeed within the timeout window

**Should have (differentiators):**
- Configurable log patterns per instance in config.yaml -- makes the monitor generic rather than Telegram-specific
- Success notification for Telegram connection (optional, to avoid notification fatigue)
- Configurable timeout duration to account for proxy environments (OpenClash)

**Defer (v2+):**
- Regex pattern support (strings.Contains is sufficient for known patterns)
- Per-instance notification routing to different Pushover users
- Generic log watcher framework (keep Telegram-specific for v0.9)

### Architecture Approach

Two independent additions to the existing `main.go` wiring. The startup notification is a function call; the Telegram monitor is a new `internal/telegram/` package with its own struct, lifecycle, and tests. Only `main.go` is modified for integration.

**Major components:**
1. `sendStartupNotification()` in `main.go` -- formats `AutoStartResult` into Pushover message, sends async with panic recovery
2. `TelegramMonitor` in `internal/telegram/monitor.go` -- subscribes to LogBuffer, detects patterns, manages timeout state machine, sends notifications
3. Pattern constants in `internal/telegram/patterns.go` -- trigger, success, and failure strings as configurable constants

### Critical Pitfalls

1. **AfterFunc callback data race** -- callback runs in a new goroutine; any shared state must be mutex-protected (follow `NotificationManager.confirmAndNotify` pattern exactly)
2. **Log entry dropped by non-blocking subscriber** -- if the scanner goroutine blocks on anything besides channel read, entries get dropped. Never block in the scanner loop; hand off notifications to a separate goroutine
3. **Historical log false trigger** -- `Subscribe()` replays buffer history. Filter by timestamp: only process entries written after scanner initialization
4. **Notification spam on restart loop** -- aggregate all instances into a single notification; add cooldown to suppress duplicate startup notifications within N minutes
5. **Spurious timeout during instance stop** -- cancel the Telegram monitor's context when the instance is stopped for update; otherwise a "failed" notification fires 30s after an intentional stop

## Implications for Roadmap

### Phase 1: Startup Notification
**Rationale:** Simplest possible change (one function, no new packages, no new goroutines beyond the established async pattern). Validates the Pushover notification path in the startup context before building the more complex Telegram monitor on top.
**Delivers:** Pushover notification after auto-start with per-instance success/failure details
**Addresses:** Startup result notification (table stakes)
**Avoids:** Pitfall 3 (notification spam -- aggregate into single message), Pitfall 9 (failure path must trigger notification), Pitfall 10 (async send, never block startup)

### Phase 2: Telegram Monitor Core
**Rationale:** Self-contained package with zero coupling to the rest of the application. Depends only on the `LogSubscriber` interface (satisfied by `LogBuffer` via duck typing). Can be developed and fully tested in isolation with mocks.
**Delivers:** `TelegramMonitor` struct with pattern detection, 30-second timeout state machine, and notification trigger
**Uses:** `strings.Contains`, `time.AfterFunc`, `context.WithCancel`
**Implements:** `internal/telegram/` package with `monitor.go`, `patterns.go`, `monitor_test.go`
**Avoids:** Pitfall 1 (non-blocking scanner loop), Pitfall 2 (mutex in AfterFunc callback), Pitfall 4 (timestamp filter for historical logs), Pitfall 6 (context lifecycle for clean goroutine shutdown)

### Phase 3: Telegram Monitor Integration
**Rationale:** Minimal wiring -- create monitors per instance in `main.go`, store references, stop on shutdown. Phase 1 validated notifications work; Phase 2 validated monitor logic works. This phase just connects them.
**Delivers:** Active Telegram monitoring for all started instances
**Uses:** `main.go` startup and shutdown wiring, `instanceManager.GetLogBuffer()`
**Avoids:** Pitfall 5 (cancel monitor on instance stop), Pitfall 11 (single monitor per instance, cancel previous on restart trigger)

### Phase 4: E2E Validation
**Rationale:** Final integration test against real nanobot output. Verify exact log patterns, timeout calibration, multi-instance behavior, and graceful degradation without Pushover.
**Delivers:** Validated milestone ready for deployment
**Avoids:** Pitfall 7 (verify patterns against real nanobot output), Pitfall 8 (tune timeout for proxy environment)

### Phase Ordering Rationale

- Startup notification first because it is trivially simple and proves the notification delivery path works in the startup flow, de-risking the later Telegram monitor integration.
- Telegram monitor core second because it is a pure-logic package with no integration points, allowing full test coverage with mocks before any wiring touches `main.go`.
- Integration third because it is the narrowest change (only `main.go`), and both dependencies are already validated.
- E2E last for final validation, pattern verification, and timeout tuning.
- This ordering ensures every phase builds on validated foundations and no phase introduces untested coupling.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 2:** Exact log patterns from python-telegram-bot output need verification against real nanobot stdout. The trigger pattern ("Starting Telegram bot") and success/failure patterns are estimated and must be confirmed.
- **Phase 4:** Timeout calibration under OpenClash proxy environment -- 30 seconds may be too short or too long depending on proxy latency characteristics.

Phases with standard patterns (skip research-phase):
- **Phase 1:** Well-established notification pattern already used in `TriggerHandler` and `NotificationManager`. Copy the pattern.
- **Phase 3:** Simple wiring following the existing `main.go` component creation and shutdown patterns.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Zero new dependencies. All capabilities verified against existing codebase APIs and Go stdlib. |
| Features | HIGH | Feature scope is narrow and well-bounded. All integration points (AutoStartResult, LogBuffer.Subscribe, Notifier.Notify) confirmed in source. |
| Architecture | HIGH | Minimal structural change. New package follows existing patterns. Only main.go modified for wiring. |
| Pitfalls | HIGH | All pitfalls derived from codebase analysis of existing concurrency patterns. Prevention strategies backed by working code in the same project. |

**Overall confidence:** HIGH

### Gaps to Address

- **Exact log pattern strings:** The trigger pattern ("Starting Telegram bot") and success/failure patterns are based on python-telegram-bot library behavior analysis, not verified against actual nanobot subprocess output. Must capture real nanobot stdout during Phase 2 implementation to confirm. Implementation should isolate patterns in `patterns.go` for easy adjustment.
- **Timeout calibration under proxy:** The 30-second default is a reasonable guess but the target environment runs through OpenClash proxy with TLS MITM, which may increase connection latency. Should be configurable and tuned based on observed connection times after deployment.
- **Startup notification on success vs failure only:** There is a design tension between FEATURES.md (notify only on failure) and ARCHITECTURE.md (notify on both success and failure). The recommendation is to notify on both for startup (user needs confirmation after automatic boot) but only on failure for Telegram (avoid notification fatigue). This should be resolved during Phase 1 requirements.

## Sources

### Primary (HIGH confidence)
- Existing codebase: `internal/logbuffer/buffer.go`, `internal/logbuffer/subscriber.go` -- Subscribe/Unsubscribe pattern, non-blocking send, history replay
- Existing codebase: `internal/notifier/notifier.go` -- Notifier interface, IsEnabled(), Notify()
- Existing codebase: `internal/notification/manager.go` -- AfterFunc + mutex pattern, async notification with panic recovery
- Existing codebase: `internal/instance/manager.go` -- AutoStartResult, StartAllInstances() return type
- Existing codebase: `cmd/nanobot-auto-updater/main.go` -- Component wiring, auto-start goroutine, shutdown sequence
- Existing codebase: `internal/lifecycle/starter.go` -- Log capture via captureLogs() writing to LogBuffer
- Go stdlib documentation: `time.AfterFunc`, `strings.Contains`, `context.WithTimeout`

### Secondary (MEDIUM confidence)
- python-telegram-bot library: known log patterns for bot startup, polling, and connection errors -- exact strings need validation against real nanobot output
- Pushover API: rate limits (7,500 messages/month free tier, 500ms between calls)

### Tertiary (LOW confidence)
- None -- all findings are backed by direct codebase analysis or official Go documentation

---
*Research completed: 2026-04-06*
*Ready for roadmap: yes*

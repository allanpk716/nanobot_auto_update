---
phase: 47
reviewers: [opencode]
reviewed_at: 2026-04-10T12:00:00+08:00
plans_reviewed: [47-01-PLAN.md, 47-02-PLAN.md]
unavailable_reviewers:
  - gemini (not installed)
  - codex (not installed)
  - claude (skipped — running inside Claude Code)
---

# Cross-AI Plan Review — Phase 47

## OpenCode Review

### Plan 47-01: Extract AppComponents/AppStartup/AppShutdown + Implement ServiceHandler

**Summary:** The plan is well-structured and follows a logical two-task decomposition: extract shared lifecycle logic from main.go, then implement the svc.Handler. The research phase was thorough—identifying anti-patterns (blocking `r` channel, missing StopPending), existing shutdown order, and the `svc/debug` test approach. However, there are several gaps around AppStartup error handling, startup cancellation semantics, and missing component types in AppComponents.

**Strengths:**
- **TDD-first approach**: Test list (AllComponents, NilComponents, PartialComponents, APIServerContext) covers the critical nil-safety path well
- **Anti-pattern awareness**: Identifying the "must not block without reading `r`" and "must report StopPending before slow shutdown" pitfalls shows genuine understanding of the `svc.Handler` contract
- **Shutdown order preservation**: Explicitly matching existing main.go order avoids subtle behavioral regressions
- **Build-tag strategy**: Follows existing pattern from servicedetect.go/servicedetect_windows.go
- **Nil-safety in AppShutdown**: Recognizing that apiServer and healthMonitor can be nil is important

**Concerns:**
- **[HIGH] AppStartup error handling undefined**: main.go has multiple fatal paths during startup. AppStartup must define a clear error/rollback contract — if the 4th of 6 components fails, what happens to the first 3? The plan mentions extracting lines 148-274 but doesn't address rollback semantics.
- **[HIGH] AppComponents is missing key dependencies**: main.go creates at least 3 additional objects: `notif` (notifier), `instanceManager`, and `selfUpdater`. These are needed by apiServer and the auto-start goroutine. The plan should clarify whether these are part of AppComponents.
- **[HIGH] Auto-start goroutine ownership unclear**: Lines 246-274 launch a critical goroutine calling `instanceManager.StartAllInstances()` and `notif.NotifyStartupResult()`. This is not one of the 6 components but must run after startup. Who launches it in service mode?
- **[MEDIUM] Execute startup failure behavior**: If AppStartup fails inside Execute, the plan doesn't specify the state transition. Should it report Stopped with a non-zero exit code? The return values need explicit definition for the failure path.
- **[MEDIUM] Test approach relies on channel timing**: Plan should note that `svc/debug` package provides `NewConsoleService()` specifically designed for this. Hand-rolling channel manipulation for unit tests is fragile.
- **[LOW] AppStartup signature**: `version` parameter is only used for logging and passing to apiServer. Consider whether it's better as a field on a higher-level struct.

**Suggestions:**
- Define AppStartup to return `(*AppComponents, error)` with explicit rollback logic in the error path
- Add `InstanceManager`, `SelfUpdater`, and `Notifier` to AppComponents (or a separate "deps" struct)
- Specify the Execute failure path: report `svc.Stopped` with `exitCode=1` on AppStartup error
- Consider using `svc/debug` for test infrastructure rather than hand-rolling channel manipulation
- Add a test case for AppStartup failure mid-initialization (rollback scenario)

**Risk Assessment: MEDIUM** — The svc.Handler implementation is straightforward, but AppStartup extraction is the real risk. main.go has 8+ initialization steps with complex dependency chains. Missing any dependency or getting the error/rollback semantics wrong could cause resource leaks in service mode.

---

### Plan 47-02: RunService Wrapper and main.go Service Mode Branch

**Summary:** Clean, minimal wiring plan connecting ServiceHandler into main.go. Two-task split (code changes + human verification) is appropriate. Correctly preserves console mode and defers svc.Run to after config/logger initialization. Main risk is total dependency on Plan 47-01 correctness.

**Strengths:**
- **Minimal scope**: Only adds RunService wrapper and `if inService` branch
- **Console mode preservation**: Explicitly states "Console mode path completely unchanged"
- **Correct initialization ordering**: svc.Run happens after config load and logger init
- **Build-tag safety**: Non-Windows stub returns error, preventing accidental use on wrong platforms
- **Human verification checkpoint**: Good safety net for main.go changes

**Concerns:**
- **[MEDIUM] RunService error handling in main.go**: The plan shows `lifecycle.RunService(); return` but doesn't define what happens if RunService returns an error. Should it `log.Fatal`? `os.Exit(1)`?
- **[MEDIUM] svc.Run blocking semantics**: svc.Run blocks until Execute returns, so startup errors inside Execute won't surface as Go errors to main.go. The plan should acknowledge this explicitly.
- **[LOW] Verification relies on manual testing**: Could be strengthened with automated cross-platform build verification.

**Suggestions:**
- Define RunService error handling: `if err := lifecycle.RunService(...); err != nil { logger.Error(...); os.Exit(1) }`
- Note that svc.Run blocks and the `return` after it only executes when Execute returns
- Add `GOOS=linux go build` step to verify non-Windows stub compiles

**Risk Assessment: LOW** — Straightforward wiring code. Main risk is dependency on Plan 47-01, not from this plan itself.

---

## Consensus Summary

**Single reviewer (OpenCode).** Gemini and Codex were unavailable. Cross-AI confidence is limited — findings should be validated against codebase before acting.

### Key Findings (Priority Order)

1. **AppStartup error/rollback contract undefined** [HIGH] — If component initialization fails partway through, the plan doesn't specify how already-started components are cleaned up. This is the most critical gap.

2. **AppComponents missing internal dependencies** [HIGH] — `notif`, `instanceManager`, `selfUpdater` are created in main.go but not listed in AppComponents. These are required by apiServer and the auto-start goroutine.

3. **Auto-start goroutine ownership unclear** [HIGH] — The auto-start goroutine (StartAllInstances + NotifyStartupResult) needs explicit assignment to either AppStartup or Execute.

4. **Execute failure path needs explicit definition** [MEDIUM] — Return values and SCM state transitions for startup failure should be specified in the plan.

5. **RunService error handling in main.go** [MEDIUM] — Error handling for RunService return value needs to be defined (log + exit).

### Agreed Strengths
(From single reviewer — not cross-validated)
- TDD-first approach with nil-safety coverage
- Anti-pattern awareness from research phase
- Shutdown order preservation matching existing main.go
- Build-tag strategy following established project pattern
- Minimal scope of Plan 47-02

### Action Items for Planner

1. **Before execution**: Add rollback logic to AppStartup error path — if component N fails, stop components 1..N-1
2. **Before execution**: Decide whether `instanceManager`, `selfUpdater`, `notif` belong in AppComponents or are local to AppStartup
3. **Before execution**: Specify Execute failure return values: `(true, 1)` for service-specific error
4. **Before execution**: Define RunService error handling in main.go
5. **During execution**: Consider svc/debug for test infrastructure instead of hand-rolled channel tests

---

*Review generated with [Claude Code](https://claude.ai/code) via [Happy](https://happy.engineering)*

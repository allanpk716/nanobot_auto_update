---
phase: 48
reviewers: [opencode]
reviewed_at: 2026-04-11T12:00:00+08:00
plans_reviewed: [48-01-PLAN.md, 48-02-PLAN.md]
---

# Cross-AI Plan Review — Phase 48

## OpenCode Review

### Plan 48-01: ServiceManager Core (Wave 1)

**Summary**: A well-structured plan that creates the Windows service registration/unregistration logic with proper idempotency, cross-platform stubs, and recovery policy configuration. The design aligns closely with all 9 user decisions (D-01 through D-09) and follows the existing codebase patterns established in Phases 46-47.

**Strengths**:
- **Idempotent design** — Both RegisterService and UnregisterService handle the "already done" case gracefully (D-04, D-06), which is critical for a tool users will run repeatedly.
- **Correct dependency usage** — golang.org/x/sys/windows/svc/mgr is already in go.mod; no new dependencies needed.
- **Recovery policy is production-grade** — 3x restart at 60s with 24h reset (D-07) and RecoveryActionsOnNonCrashFailures(true) is the right approach for a monitoring service.
- **Follows established build-tag pattern** — //go:build windows / //go:build !windows matches the existing Phase 46-47 pattern.
- **Proper admin check** — Token elevation check via OpenCurrentProcessToken (D-08) is the canonical Windows approach.
- **Threat model is thorough** — 5 threats with STRIDE classification, explicit accept/mitigate decisions.

**Concerns**:
- **[MEDIUM] UnregisterService stop-wait blocking with no context cancellation** — The 30-second poll loop has no context.Context support. If the caller needs to abort, the goroutine blocks for up to 30s.
- **[MEDIUM] No timeout on mgr.Connect()** — If SCM is unresponsive, Connect() can block indefinitely.
- **[LOW] Service description hardcoded in Chinese** — Per D-03, description is hardcoded rather than using config. Design decision but limits customizability.
- **[LOW] Test coverage is thin** — TestIsAdmin only verifies no panic, TestRegisterService_NonAdminOrNonWindows is conditionally useful. Acceptable since real integration testing happens with actual service registration.
- **[LOW] Error message quality** — Plan doesn't fully specify error message format. Wrapping errors with context is important for diagnostics.

**Suggestions**:
- Add context.Context parameter to UnregisterService for the stop-wait loop, even if initial caller passes context.Background().
- Wrap all SCM errors with fmt.Errorf("registerService: %w", err) style context.
- Consider logging the full exe path during registration for auditability.
- Add defensive check at top of RegisterService (if cfg.Service.ServiceName == "" return error).

**Risk Assessment**: LOW

---

### Plan 48-02: Wire ServiceManager into main.go (Wave 2)

**Summary**: A clean integration plan that replaces the Phase 48 placeholder blocks in main.go with the three service management cases. The case analysis (service+off, console+on, console+off) covers all combinations of inService × auto_start correctly.

**Strengths**:
- **Three-case analysis is exhaustive** — All combinations of service mode × auto_start are covered.
- **Case 1 (service + auto_start off) is correctly defensive** — Does NOT attempt to uninstall from within service context (would be suicidal). Logs warning instead.
- **Case 3 (uninstall) is non-fatal** — Failed uninstall logs warning and continues to console mode, matching D-06.
- **Exit codes are correct** — Code 2 for successful registration, code 1 for non-admin or failure.
- **Wave dependency is correct** — Depends on 48-01 which provides the APIs.

**Concerns**:
- **[HIGH] Case 1 doesn't guide the user to fix the config** — When running as service with auto_start: false, only a vague warning is logged. Should log an actionable message like "To stop running as a service, set auto_start: false and run from console."
- **[MEDIUM] Case 3 unregister + continue may confuse users** — User might not realize what happened. A clear log message would help.
- **[MEDIUM] Race condition window between register and exit** — Brief window where SCM may not have fully committed. Unlikely in practice.
- **[LOW] Line number references may drift** — Should reference content markers rather than absolute line numbers.

**Suggestions**:
- Case 1: Add clear actionable log message telling user how to transition from service to console mode.
- Case 3: Log service name and "switched to console mode" after successful uninstall.
- Replace line number references with content-based anchors.
- Consider brief time.Sleep after RegisterService() before exit as defensive measure.

**Risk Assessment**: LOW

---

### Overall Assessment

| Dimension | Rating |
|-----------|--------|
| **Goal Alignment** | Excellent — all 9 decisions (D-01 through D-09) are implemented |
| **Dependency Ordering** | Correct — Wave 1 (core) before Wave 2 (integration), Phase 47 complete |
| **Edge Case Coverage** | Good — idempotency, cross-platform, admin check, non-fatal uninstall |
| **Security** | Adequate — admin check prevents EoP, threat model is thorough |
| **Test Strategy** | Acceptable — limited by SCM mocking difficulty, verification checklist compensates |
| **Overall Risk** | **LOW** |

---

## Consensus Summary

> Note: Only one external reviewer (OpenCode) was available. Gemini and Codex CLIs are not installed. Review conclusions should be weighed accordingly.

### Agreed Strengths
- Idempotent design for both register and unregister operations
- Proper build-tag pattern matching existing Phase 46-47 conventions
- Production-grade recovery policy (3x restart, 24h reset, non-crash failures)
- Correct three-case analysis covering all service_mode × auto_start combinations
- Non-fatal uninstall behavior (warn + continue to console mode)

### Agreed Concerns (Priority Order)
1. **Case 1 logging is not actionable** — Service running with auto_start: false should tell user HOW to fix it, not just warn
2. **No context.Context on UnregisterService** — 30s blocking poll loop is not cancellable
3. **Error wrapping could be more explicit** — SCM errors should be wrapped with operation context

### Divergent Views
- N/A (single reviewer)

### Recommendations for Plan Revision
1. Enhance Case 1 log message to be actionable (add hint about running from console to uninstall)
2. Enhance Case 3 log message to clearly indicate mode transition
3. Consider adding defensive cfg.Service.ServiceName empty check at top of RegisterService
4. Consider wrapping SCM errors with operation context consistently
5. Reference placeholder code by content markers, not line numbers

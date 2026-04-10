---
phase: 46
reviewers: [opencode]
reviewed_at: 2026-04-10T12:00:00+08:00
plans_reviewed: [46-01-PLAN.md, 46-02-PLAN.md]
---

# Cross-AI Plan Review — Phase 46

## OpenCode Review

### Plan 01: ServiceConfig Struct, Validate(), Table-Driven Tests, Config Integration

**Summary**

Plan 01 is well-structured and follows the exact sub-config pattern established in the codebase (`APIConfig`, `SelfUpdateConfig`, etc.). The TDD approach with 10 table-driven test cases is thorough, and the `*bool` (nil-able) design for `AutoStart` correctly implements D-02 (unconfigured = current behavior). The plan cleanly addresses MGR-01.

**Strengths**
- Follows the **identical sub-config pattern** (struct with `yaml`+`mapstructure` tags, `Validate() error` method, wired into `Config` via `Service ServiceConfig` field, defaults in `defaults()`, viper `SetDefault` in `Load()`, validation via `errors.Join` in `Config.Validate()`)
- `AutoStart *bool` (pointer) is the correct Go idiom for distinguishing "not set" (nil) from "explicitly false" — matches D-02
- D-12 correctly modeled: validation skips entirely when `AutoStart` is nil or false
- 10 table-driven tests cover the important boundary conditions (nil/false/true, regex boundaries, length limits)
- Threat model T-46-01 (service_name injection via regex) is appropriate

**Concerns**
- **MEDIUM**: The plan says `Validate()` returns an `error`, but the codebase uses `fmt.Errorf` returning single errors, while `Config.Validate()` uses `errors.Join` for aggregate. Plan 01 Task 2 says "with errors.Join" — confirm this matches the actual `Config.Validate()` pattern. If `ServiceConfig.Validate()` only returns a single error, it should return `fmt.Errorf(...)` like other sub-configs, not collect with `errors.Join`.
- **LOW**: No test case for `service_name` at max length (though no max length constraint exists — only regex). Consider whether an unbounded `service_name` is acceptable (e.g., SCM has a 256-char limit for service names).
- **LOW**: The plan mentions "defaults() sets AutoStart=nil" — this is implicit (Go zero value for `*bool`), so no explicit action needed, but should be documented as a comment to avoid confusion.

**Suggestions**
- Add a test case for `service_name` exactly matching the regex boundary: single character, all-numeric name, and mixed case.
- Consider adding `service_name` max length validation (SCM limit is 256 chars) as a defense-in-depth measure, even if not explicitly required by user decisions.
- The plan should specify the viper key prefix: `service.service_name` and `service.display_name` — this is implied but worth stating explicitly.

---

### Plan 02: svc.IsWindowsService() Detection Wrapper, main.go Entry Branching

**Summary**

Plan 02 correctly implements the startup order from D-06 (`svc.IsWindowsService()` before `config.Load()`) and handles the four mode-mismatch scenarios per D-07 and D-08. The build-tag split (`_windows.go` / non-Windows stub) follows the project's existing pattern in `internal/lifecycle/`. However, there are meaningful concerns around the interaction between service detection and the existing `daemon.go` startup path.

**Strengths**
- **D-06 startup order is correct and critical**: `svc.IsWindowsService()` is a pure OS check requiring no config, so calling it before `config.Load()` avoids the "service can't find config.yaml" chicken-and-egg problem
- Build-tag approach (`servicedetect_windows.go` + `servicedetect.go`) matches the existing pattern in `internal/lifecycle/` (`starter.go`, `stopper.go`, etc.)
- D-07 (warn + continue) and D-08 (register + exit 2) are correctly specified
- Threat model T-46-03 acknowledges that privilege checking is deferred to Phase 48

**Concerns**
- **HIGH — Interaction with existing `daemon.go` startup**: `main.go` currently calls `lifecycle.MakeDaemon()` implicitly via the startup sequence. The plan modifies `main.go` but does not describe how `IsServiceMode()` interacts with the daemon launch path. If `MakeDaemon()` re-launches the process, the child may have a different session context. The plan needs an explicit statement about where `IsServiceMode()` sits relative to `MakeDaemon()`.
- **HIGH — D-08 register service in console mode**: Plan says "register service, exit code 2" but gives no detail on *how* the registration happens. Service registration requires `advapi32.dll` via `golang.org/x/sys/windows/svc/mgr` — this is non-trivial and may be scope creep. If registration is deferred, the plan should say so explicitly.
- **MEDIUM — main.go modification scope**: The existing `main.go` has a specific startup order (flags → config → logger → lifecycle → ...). Inserting `IsServiceMode()` between `flag.Parse()` and `config.Load()` means logging isn't initialized yet when "Detected Windows service mode" is logged. The plan should specify using `fmt.Println` or `log.Println` for pre-logger messages, or defer the log message until after logger init.
- **MEDIUM — Error handling for `svc.IsWindowsService()`**: The function signature returns `(bool, error)`. The plan doesn't specify what happens if `IsWindowsService()` itself returns an error. Should this be treated as console mode? Fatal exit?
- **LOW — `servicedetect.go` stub**: On non-Windows, this is fine for compilation, but the plan should note this is for cross-platform build support only.

**Suggestions**
- **Crucial**: Add an explicit statement about `MakeDaemon()` interaction. Either show it's called before `IsServiceMode()`, after, or is being removed/replaced by this phase.
- **Crucial**: Clarify D-08 implementation scope. If service registration is deferred to Phase 48, state this explicitly: "D-08 is partially implemented — console+autoStart logs intent and exits code 2; actual SCM registration is Phase 48 scope."
- Specify the logging mechanism for pre-config startup messages (before logger init).
- Handle `IsWindowsService()` error explicitly: recommend treating error as `inService=false` (console mode) with a WARN-level log after logger init.
- Consider wrapping the detection in a function that also logs the detection result *after* logger init, rather than at detection time.

---

### Overall Risk Assessment (OpenCode)

**Risk Level: MEDIUM**

**Justification**: Plan 01 is low-risk — it follows an established pattern with TDD. Plan 02 carries medium risk due to two unresolved questions: (1) the interaction with the existing `MakeDaemon()` path in `main.go`, and (2) the scope boundary of D-08 (service registration from console). Both concerns can be mitigated with explicit scoping decisions before execution. The core technical approach (svc.IsWindowsService() + build tags + config sub-struct) is sound and well-aligned with the codebase.

---

## Consensus Summary

Reviewed by 1 external AI (OpenCode). Key findings:

### Agreed Strengths
- Plan 01 follows the established sub-config pattern perfectly — low implementation risk
- D-06 startup order (svc detection before config load) is correct and avoids chicken-and-egg problems
- Build-tag approach for platform abstraction is clean and consistent with existing code
- TDD with 10 test cases provides good coverage for ServiceConfig validation
- *bool pointer pattern correctly handles nil/false/true tristate

### Top Concerns (Prioritized)

| # | Severity | Concern | Plan | Recommendation |
|---|----------|---------|------|----------------|
| 1 | HIGH | `MakeDaemon()` interaction not addressed — where does IsServiceMode() sit relative to the daemon fork? | 02 | Add explicit statement about MakeDaemon() call order; likely IsServiceMode() should be called BEFORE MakeDaemon() |
| 2 | HIGH | D-08 scope unclear — actual SCM registration or just log+exit? | 02 | Clarify: Phase 46 only logs intent and exits code 2; Phase 48 implements actual registration |
| 3 | MEDIUM | Pre-logger logging — slog not initialized when "Detected Windows service mode" is logged | 02 | Use `log.Println` or defer log until after logger init; or use `fmt.Fprintf(os.Stderr, ...)` for pre-config messages |
| 4 | MEDIUM | `errors.Join` vs single error in ServiceConfig.Validate() | 01 | ServiceConfig.Validate() returns single `fmt.Errorf(...)` — errors.Join is only in Config.Validate() aggregation |
| 5 | MEDIUM | svc.IsWindowsService() error handling unspecified | 02 | Treat error as console mode (false) with warning after logger init |
| 6 | LOW | No service_name max length validation (SCM has 256-char limit) | 01 | Consider adding defense-in-depth; not required by user decisions |

### Divergent Views
Only one reviewer — no divergent views to report.

### Action Items Before Execution

1. **Resolve Concern #1**: Read `main.go` to determine exact call order of `MakeDaemon()` vs proposed `IsServiceMode()` insertion point
2. **Resolve Concern #2**: Confirm D-08 scope with user or add explicit note in plan that registration is Phase 48 scope
3. **Resolve Concern #3**: Decide logging strategy for pre-config messages in main.go
4. **Address Concern #4**: Verify that ServiceConfig.Validate() uses `fmt.Errorf` (single error) matching other sub-configs

---

*Review generated: 2026-04-10*
*Reviewers: OpenCode (1 external AI)*

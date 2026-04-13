---
phase: 53
reviewers: [opencode]
reviewed_at: 2026-04-12T08:06:00Z
plans_reviewed: [53-01-PLAN.md, 53-02-PLAN.md, 53-03-PLAN.md]
---

# Cross-AI Plan Review — Phase 53

## OpenCode Review

### 1. Summary

Overall, the three plans form a well-structured, logically ordered implementation for the Instance Management UI. The wave-based execution order is correct (foundation → CRUD dialogs → advanced editor), API contracts are accurately documented against the actual backend code, and security considerations (XSS, auth) are thoughtfully addressed. The plans are implementable as-is, but several medium-severity issues around bidirectional sync loop prevention, UI language consistency with existing code, graceful API failure handling, and the copy dialog's port suggestion logic need attention before execution.

### 2. Strengths

- **Accurate API contracts**: All endpoint paths, request/response formats, and error structures match the actual Go backend implementations (`instance_config_handler.go`, `instance_lifecycle_handler.go`, `nanobot_config_handler.go`)
- **Clean wave decomposition**: Foundation (modal/toast/cards) → CRUD dialogs → hybrid editor. Each wave is independently testable with placeholder stubs in earlier waves
- **Explicit self-update preservation**: Plan 01 calls out preserving all self-update module code with specific function names and the `authToken` variable reuse
- **Threat models per plan**: Each plan has a STRIDE threat register with appropriate mitigations. XSS prevention via `textContent` is consistently enforced
- **Detailed CSS specifications**: Full CSS code is provided, reducing ambiguity for the executor agent
- **Decision traceability**: Every D-XX and UI-XX requirement is mapped to specific plan tasks with grep-able artifacts
- **Plan 03 Task 2 integration verification**: Comprehensive checklist covering API endpoints, XSS, decision coverage (D-01 through D-16), and requirement coverage (UI-01 through UI-06)

### 3. Concerns

#### HIGH Severity

- **Bidirectional sync infinite loop risk (Plan 03)**: When a form field changes → updates JSON textarea → triggers textarea `input` event → updates form fields → could trigger form `input` events again. The plan does not mention a guard flag (e.g., `isSyncing`) to break the cycle. This will cause performance issues or stack overflow in practice.

- **UI language inconsistency (Plan 01)**: The existing `home.js` uses Chinese text ("端口:", "状态:", "运行中", "已停止", "重启实例"). All three plans specify English text for new/rewritten elements ("Port:", "Running", "Stopped", "No instances configured"). Mixing languages in the same UI will look broken to users. Either all text should be Chinese (matching existing code) or a deliberate language switch should be documented.

#### MEDIUM Severity

- **loadInstances() dual-API failure handling (Plan 01)**: `Promise.all` fails fast — if either `instance-configs` (auth required) or `instances/status` fails, the entire function rejects and shows a single error state. If the token fetch fails (non-localhost access), users lose even the status view that currently works without auth. Consider `Promise.allSettled` to gracefully degrade.

- **Copy dialog port suggestion (Plan 02)**: The plan says "pre-filled with next available port if possible, otherwise sourcePort+1". The frontend cannot reliably determine port availability without querying all existing configs. Since the backend already auto-increments (`instance_config_handler.go:456-483`), the frontend should suggest `sourcePort + 1` and let server-side validation catch conflicts.

- **Token acquisition failure in loadInstances (Plan 01)**: The `getToken()` helper fetches from `/api/v1/web-config` (localhost-only). If this fails, the `instance-configs` fetch will fail. The current code only fetches `instances/status` (no auth) and works from any host. Plan 01 should preserve the fallback of showing status-only cards when auth is unavailable.

- **Long-running start/stop operations (Plan 01)**: Backend start has a 60-second timeout (`instance_lifecycle_handler.go:58`), stop has 30 seconds. The plan's `handleLifecycleAction` doesn't mention a client-side timeout or UX for long waits beyond the button loading state.

- **Edit dialog name field behavior (Plan 02)**: The plan says name is "readonly" but the backend PUT endpoint docs say "name must match path or be empty." If the readonly name field's value is included in the PUT body, it's fine. But if the field is `disabled` (not `readonly`), its value won't be included in form serialization. Ensure `readOnly` attribute is used (not `disabled`).

- **Delete dialog stale isRunning value (Plan 02)**: The `showDeleteDialog(name, isRunning)` receives the running status from the time the card was last rendered (up to 5 seconds stale via polling interval). An instance could start/stop between card render and button click. The warning text may be inaccurate.

#### LOW Severity

- **Escape key to close modal (Plan 01)**: No keyboard event handler for Escape key to close the modal. Standard UX expectation for modal dialogs.

- **Toast stacking order (Plan 01)**: Plan says "newest on top" but CSS uses `flex-direction: column` which stacks top-to-bottom. New toasts appended to the container will appear at the bottom. Either prepend instead of append, or use `flex-direction: column-reverse`.

- **API key field type (Plan 03)**: The API key input uses `type="text"` — should be `type="password"` with a show/hide toggle for security, even on a localhost admin tool.

- **Nanobot config save without restart prompt (Plan 03)**: After saving nanobot config, the backend returns a hint about restarting. The plan shows a success toast but doesn't surface the restart hint to the user. Consider adding a "Restart to apply?" action in the toast or a secondary prompt.

- **Missing responsive handling for hybrid editor (Plan 03)**: The `hybrid-editor` uses `grid-template-columns: 1fr 1fr` which will break on narrow screens. Should have a media query to stack vertically on small viewports.

- **No form dirty detection (Plan 02/03)**: Closing the modal after editing form fields doesn't warn about unsaved changes. Easy to accidentally discard work.

### 4. Suggestions

- **Add sync guard to Plan 03**: Introduce a `let syncGuard = false;` flag. Set `syncGuard = true` before programmatic updates to textarea/form, check `if (syncGuard) return` at the top of event handlers, reset `syncGuard = false` after update. This is the standard pattern for preventing bidirectional sync loops.

- **Unify UI language**: Either continue using Chinese for all user-facing text (consistent with existing self-update module: "检测更新", "已是最新版本", etc.) or explicitly document a one-time language migration in Plan 01. Recommend Chinese to match the existing ~300 lines of Chinese UI text in `home.js`.

- **Use `Promise.allSettled` in loadInstances()**: If `instance-configs` fails (auth issue), still render cards from `instances/status` with basic info. If `instances/status` fails, render cards from configs without running indicators. This preserves backward compatibility for non-localhost access.

- **Prepend toasts, don't append**: Change toast insertion to `toastContainer.insertBefore(toast, toastContainer.firstChild)` so newest appears on top as specified in D-14.

- **Add Escape key handler**: `document.addEventListener('keydown', (e) => { if (e.key === 'Escape') closeModal(); })` — trivial addition, major UX improvement.

- **Add client-side timeout to handleLifecycleAction**: Set a 65s `AbortController` timeout for start and 35s for stop, matching backend timeouts plus margin. Show "Operation timed out" toast on abort.

- **Add responsive media query for hybrid editor**: `@media (max-width: 768px) { .hybrid-editor { grid-template-columns: 1fr; } }` to stack on mobile.

- **Surface restart hint after nanobot config save**: After successful save, show a toast like "Config saved. Restart instance to apply changes." or offer an inline "Restart Now" button.

### 5. Risk Assessment

**Overall Risk: MEDIUM**

The plans are well-designed and the wave ordering is correct. The main risks are:

1. **Bidirectional sync loop (HIGH impact, fixable)**: The infinite loop risk in Plan 03 is the most technically dangerous issue. Without a sync guard, the implementation will either degrade performance or break entirely. Fix is straightforward (one boolean flag) but must be specified before execution.

2. **Language inconsistency (MEDIUM impact, cosmetic)**: Mixed Chinese/English UI will confuse users and look unprofessional. Easy fix (standardize on one language) but touches every new UI string across all three plans.

3. **Auth-dependent regression (MEDIUM impact)**: Current `home.js` works from any host (status-only). Plan 01's `loadInstances()` makes instance-configs (auth required) a hard dependency. Non-localhost users lose all functionality. Graceful degradation via `Promise.allSettled` mitigates this.

4. **Scope containment (LOW risk)**: The plans are appropriately scoped. No scope creep detected. Deferred items (syntax highlighting, schema validation, batch ops) are correctly excluded. The three plans modify exactly 3 files across all waves, keeping the change surface small.

The plans are **ready for execution with minor amendments** to address the sync guard and language consistency issues. All other concerns are defensive improvements that could be addressed during implementation without plan revision.

---

## Consensus Summary

*Review by 1 external AI system (OpenCode). Gemini CLI auth failure (401). Claude CLI skipped (running inside Claude Code).*

### Agreed Strengths
- Wave-based execution order is correct and logical
- API contracts accurately documented against backend code
- XSS prevention via textContent consistently enforced
- Decision traceability (D-XX) and requirement coverage (UI-XX) is thorough
- Self-update module preservation explicitly specified
- Threat models present in all 3 plans

### Agreed Concerns (Priority-Ranked)
1. **[HIGH] Bidirectional sync infinite loop** — Plan 03 needs a sync guard flag (`isSyncing`) to prevent form↔JSON event cascading
2. **[HIGH] UI language inconsistency** — Existing code uses Chinese, plans specify English; must unify before execution
3. **[MEDIUM] Promise.all fail-fast in loadInstances()** — Use `Promise.allSettled` for graceful degradation when auth fails
4. **[MEDIUM] Copy dialog port suggestion** — Simplify to `sourcePort + 1`, let server validate
5. **[MEDIUM] Edit dialog name readonly vs disabled** — Ensure `readOnly` (not `disabled`) so value is included in request body
6. **[MEDIUM] Long-running start/stop UX** — Add client-side AbortController timeout matching backend timeouts
7. **[LOW] Escape key to close modal** — Trivial UX improvement
8. **[LOW] Toast stacking order** — Prepend instead of append for "newest on top"
9. **[LOW] Restart hint after nanobot config save** — Surface backend's restart suggestion to user

### Divergent Views
*Only 1 reviewer — no divergent views to report.*

---

*Reviewed: 2026-04-12 by OpenCode (via GitHub Copilot)*

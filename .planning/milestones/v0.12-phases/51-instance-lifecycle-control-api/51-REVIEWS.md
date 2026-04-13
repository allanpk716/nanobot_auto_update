---
phase: 51
reviewers: [opencode]
reviewed_at: 2026-04-12T00:00:00+08:00
plans_reviewed: [51-01-PLAN.md, 51-02-PLAN.md]
---

# Cross-AI Plan Review — Phase 51

## OpenCode Review

### Plan 01: InstanceLifecycleHandler with HandleStart/HandleStop + route registration

#### Summary

The plan is well-aligned with existing codebase conventions — struct-based handler with scoped logger, `writeJSONError` for errors, Go 1.22+ route patterns, and `AuthMiddleware` wrapping. However, it has a **critical gap**: no consideration of the `InstanceManager`'s `TryLockUpdate()`/`UnlockUpdate()` concurrency guard, which means individual start/stop operations could conflict with an in-progress `TriggerUpdate` or self-update.

#### Strengths

- Follows established handler pattern (`struct` + `NewXxxHandler` constructor + `HandleXxx` methods), consistent with `SelfUpdateHandler`, `TriggerHandler`, etc.
- Uses `context.Background()` for start operations — correct to prevent orphaned processes on client disconnect
- 409 Conflict for wrong-state operations matches existing convention (self-update already uses 409)
- Reuses existing `StartAfterUpdate()`/`StopForUpdate()` which handle Telegram monitor lifecycle — avoids reinventing
- Route registration pattern (`mux.Handle("POST ...", authMiddleware(http.HandlerFunc(h.HandleStart)))`) exactly matches existing `instance-config` routes

#### Concerns

- **HIGH: No update-lock coordination** — `InstanceManager.TryLockUpdate()`/`UnlockUpdate()` guards concurrent updates. The plan calls `StartAfterUpdate`/`StopForUpdate` directly without checking or acquiring this lock. If a `TriggerUpdate` is in progress, individual start/stop could race. Need either: (a) check `isUpdating` and return 409, or (b) acquire the lock per-operation.
- **HIGH: No success response format specified** — The plan says "return JSON response" but doesn't define the schema. Existing handlers use `{"status": "...", "message": "..."}` (self-update) or return resource data (instance-config). Should specify exactly what's returned on 200 OK.
- **MEDIUM: Constructor missing `getToken func() string`** — The handler struct only has `im` and `logger`, but auth middleware is applied at route level, not in the handler. This is actually correct (auth is a middleware wrapper), but the plan should clarify this design choice.
- **MEDIUM: No request body expected but no method validation** — Since Go 1.22 `mux` handles method routing, `HandleStart`/`HandleStop` registered under `POST` won't receive other methods. But plan should note this relies on the router, matching existing patterns.
- **LOW: Timeout value justification** — 60s start / 30s stop are reasonable but should document why these specific values. `StartAfterUpdate` internally has its own configurable timeout (default 30s from config). The outer context timeout (60s) should be > inner timeout to let the inner logic control its own deadline.

#### Suggestions

- **Add update-lock check**: Before calling `StartAfterUpdate`/`StopForUpdate`, check `im.TryLockUpdate()` and return 409 if locked. Use `defer im.UnlockUpdate()`. This mirrors how `TriggerUpdate` and `SelfUpdateHandler` coordinate.
- **Define success response**: e.g., `{"status": "ok", "message": "Instance 'x' started", "instance": {"name": "x", "running": true}}` or keep it minimal like the restart handler.
- **Consider using the existing restart handler as reference** — `web.NewInstanceRestartHandler` already does stop-then-start. The plan should reference it for response format consistency.
- **Handle `GetLifecycle` error explicitly** — `GetLifecycle(name)` returns `(*InstanceLifecycle, error)`. Map specific errors to appropriate status codes (not just 404).

---

### Plan 02: Comprehensive handler tests for start/stop/auth/error scenarios

#### Summary

Good test coverage of the primary error paths (404, 409, 400, 401) and JSON format validation. However, the plan depends on creating a "real InstanceManager with test config" which is heavy and may not support all scenarios (especially "already running" state). Tests also miss success-path and timeout scenarios.

#### Strengths

- Auth tests (7, 8) reuse the established `withAuth`/`authenticatedRequest` pattern — consistent with `auth_test.go`
- Path parameter testing via `req.SetPathValue("name", ...)` matches `instance_config_handler_test.go` pattern
- JSON format validation tests (9, 10) ensure error response contracts
- `mockNotifier` implementing `instance.Notifier` follows the struct-based mock convention

#### Concerns

- **HIGH: Test 2 (AlreadyRunning 409) is noted as "integration test limitation"** — This is a core success criterion test. With a real `InstanceManager`, you can't easily set a running PID. Options: (a) extract an interface for lifecycle operations and mock it, or (b) use the `instance` package's internal state manipulation. The plan should resolve this instead of leaving it as a note.
- **MEDIUM: No success-path tests** — Tests 1-10 cover only error cases. Missing: what happens when start/stop succeeds (200 OK, correct JSON body). This is critical for verifying the plan actually works.
- **MEDIUM: Real InstanceManager is heavy for unit tests** — Creating a real `InstanceManager` calls `NewInstanceManager(cfg, logger, notifier)` which creates `InstanceLifecycle` per config instance and may have side effects. Consider testing against a lightweight interface/mock instead.
- **LOW: No concurrent request tests** — Two simultaneous `POST /start` requests for the same instance should not cause double-start. The update-lock concern from Plan 01 applies here too.

#### Suggestions

- **Add success-path tests**: `TestHandleStart_Success` and `TestHandleStop_Success` — even if they need mocking, they verify the happy path works.
- **Resolve "already running" test**: Either (a) define a `LifecycleController` interface that the handler depends on (enabling mock), or (b) test at integration level with actual process management. The interface approach is cleaner and matches how `TriggerHandler` depends on `TriggerUpdater` interface.
- **Add timeout test**: Verify that a context timeout returns appropriate error (504 Gateway Timeout, matching existing convention).
- **Use testify consistently**: The newer tests (`instance_config_handler_test.go`) use `testify/assert` + `require`. New tests should follow this convention.

---

## Overall Risk Assessment (OpenCode)

**Risk Level: MEDIUM**

**Justification**: The core design is sound and well-aligned with existing patterns. The main risk is the **missing update-lock coordination** (HIGH concern in Plan 01), which could cause data races between individual lifecycle operations and bulk updates. This is a straightforward fix but must be addressed before execution. The test plan's reliance on real `InstanceManager` may cause tests to be flaky or incomplete, but this is a quality concern rather than a functional risk. Success criteria 1 and 2 are achievable with the proposed plans; success criterion 3 (auth) is well-covered by the existing `AuthMiddleware` and the planned auth tests.

---

## Consensus Summary

*Single reviewer (OpenCode). No multi-reviewer consensus available.*

### Key Concerns (Prioritized)

1. **Update-lock coordination missing** (HIGH) — Start/stop operations could race with bulk TriggerUpdate. Must add `TryLockUpdate()`/`UnlockUpdate()` or equivalent guard.
2. **Success response format undefined** (HIGH) — Plans specify error formats but not success response schema.
3. **AlreadyRunning test gap** (HIGH) — Core success criterion test left as "integration limitation" without resolution.
4. **No success-path tests** (MEDIUM) — Only error cases are tested; happy path is unverified.

### Divergent Views

N/A — single reviewer.

---

*Reviewed by: OpenCode (GitHub Copilot)*
*Review date: 2026-04-12*

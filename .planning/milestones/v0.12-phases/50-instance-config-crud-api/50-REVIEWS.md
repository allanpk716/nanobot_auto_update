---
phase: 50
reviewers: [opencode, claude-internal]
reviewed_at: 2026-04-11T12:00:00+08:00
plans_reviewed: [50-01-PLAN.md, 50-02-PLAN.md]
---

# Cross-AI Plan Review — Phase 50

## OpenCode Review

### Summary

Plan 01 is well-structured and closely follows established codebase patterns (handler struct + `Handle` methods, `authMiddleware` wrapping, Go 1.22 method routing). The task decomposition into 3 focused tasks with clear acceptance criteria and threat modeling is commendable. However, there are critical concurrency issues in the read-modify-write design and a shared-state mutation risk that need to be addressed before implementation.

### Strengths

- **Excellent pattern adherence**: Handler struct + `NewInstanceConfigHandler` constructor + `Handle*` methods exactly mirror `SelfUpdateHandler`, `TriggerHandler` patterns. Route registration follows the existing `mux.Handle("POST ...", authMiddleware(...))` style perfectly.
- **Thorough decision trail**: D-01 through D-17 in CONTEXT.md provide clear rationale for every design choice. The threat model with STRIDE analysis maps threats to mitigations well.
- **Correct viper integration**: `SaveConfig` correctly uses `v.Set("instances", ...)` + `v.WriteConfig()` rather than manually serializing YAML. This preserves viper's internal state consistency for non-instance config sections.
- **Proper separation of concerns**: Handler doesn't touch `globalHotReload.current` — delegates to hot-reload watcher for instance rebuild. Clean boundary between CRUD persistence and hot-reload side effects.
- **Zero `NewServer` signature change**: All 6 routes are self-contained, reading config via `config.GetCurrentConfig()` without needing additional dependencies injected into the server.

### Concerns

- **HIGH: Read-modify-write race condition** — The handler reads `GetCurrentConfig()` (shared pointer), mutates it (`cfg.Instances = append(...)`), then calls `SaveConfig()`. Two concurrent create requests could both read the same instance list, both append different instances, and the second `SaveConfig` overwrites the first's addition — **data loss**. The `configMu` mutex only serializes the viper write, not the full read-modify-write cycle.
- **HIGH: Handler mutates shared live state** — `GetCurrentConfig()` returns a pointer to `globalHotReload.current`. When `HandleCreate` does `cfg.Instances = append(cfg.Instances, newInstance)`, it modifies the live config. If `SaveConfig()` subsequently fails, the in-memory state is already corrupted. The handler should work on a **deep copy** before mutating.
- **HIGH: Delete's StopAllNanobots stops ALL instances** — During the window between `StopAllNanobots()` and `SaveConfig()`, all instances are stopped. If `SaveConfig()` fails, instances remain stopped with no recovery path.
- **MEDIUM: No deep copy between read and write** — Even if `SaveConfig` succeeds, the time between mutating `globalHotReload.current` and hot-reload detecting the file change is up to 500ms. During this window, `GetCurrentConfig()` returns mutated state that doesn't match disk.
- **MEDIUM: Copy endpoint empty body edge case** — `json.NewDecoder(r.Body).Decode(&req)` on empty body returns `io.EOF`. The plan mentions handling this but doesn't specify implementation.
- **MEDIUM: `startup_timeout: 0` semantics unclear** — `0` is accepted but means "no timeout" — the API contract should document this explicitly.
- **LOW: Validation error aggregation** — Only field validation OR uniqueness errors are returned, never both together. Consider collecting all errors in one pass.
- **LOW: `AutoStart nil` handling in update** — Should `nil` mean "keep current value" or "use default (true)"? Plan doesn't address partial updates.

### Suggestions

1. Add `UpdateConfig(fn func(*Config) error) error` to config package — higher-order function that acquires mutex, deep-copies config, calls mutation callback, then writes.
2. Deep-copy the config before returning from handler usage.
3. Handle empty body in Copy endpoint explicitly.
4. Consider `StopInstance(name)` instead of `StopAllNanobots` for delete.
5. Document `startup_timeout: 0` behavior.

---

## Claude Internal Review

### Summary

Plan 01 demonstrates strong adherence to existing codebase conventions and provides a thorough CONTEXT.md that maps every decision to a rationale. The two-plan split (implementation + TDD) is appropriate. However, the concurrency model has a critical gap: `configMu` in `SaveConfig` only protects the viper write, not the entire read-modify-write cycle in the handler. Combined with direct mutation of `globalHotReload.current` (a shared pointer returned by `GetCurrentConfig()`), this creates both data loss and state corruption risks under concurrent requests. Plan 02's testing strategy struggles with the global state dependency — the plan acknowledges this explicitly but doesn't provide a clean resolution.

### Strengths

- **Comprehensive CONTEXT.md**: The 17 decisions (D-01 through D-17) with canonical references create an excellent handoff document. Every design choice is traceable.
- **Correct viper integration pattern**: Using `v.Set("instances", cfg.Instances)` + `v.WriteConfig()` is the right approach — it lets viper handle serialization and preserves other config sections.
- **Threat model is actionable**: STRIDE analysis maps threats to specific mitigations rather than being performative. T-50-04 (StopAllNanobots) is honestly marked as "accept" rather than over-engineering a solution.
- **Minimal surface area**: No `NewServer` signature change, no new dependencies, handler is self-contained with just a logger.

### Concerns

- **HIGH: Race condition in read-modify-write** — Verified: `GetCurrentConfig()` returns `globalHotReload.current` directly (a `*Config` pointer). The handler does `cfg.Instances = append(...)` which may modify the shared slice backing array. Then `SaveConfig` only locks during the viper write. Two concurrent creates: both get same pointer, both append to same slice, second write overwrites first. Data loss.
- **HIGH: `SaveConfig` failure leaves inconsistent state** — If `viper.WriteConfig()` fails after the handler has already mutated `globalHotReload.current`, there's no rollback. In-memory state diverges from disk. Need either: (a) work on a copy, or (b) restore original on failure.
- **MEDIUM: `append` on shared slice** — In Go, `append` to a slice with remaining capacity modifies the underlying array in-place. Even if the handler assigns back to `cfg.Instances`, the original `globalHotReload.current.Instances` may already be corrupted if the slice had capacity. A proper deep copy is needed.
- **MEDIUM: HandleDelete's `StopAllNanobots` kills ALL nanobot.exe processes system-wide** — Verified: `StopAllNanobots` uses `tasklist` to find ALL `nanobot.exe` processes, not just the auto-updater's managed instances. This means it stops nanobots that the auto-updater doesn't manage. For Phase 50 this is accepted but should be documented as a known limitation.
- **MEDIUM: Plan 02 test setup complexity** — The plan deliberates through 6+ approaches before settling on `config.Load()` + `config.WatchConfig()`. This starts real fsnotify watchers and 500ms debounce timers in tests. On Windows, file system event timing is less predictable than Linux/macOS, making `time.Sleep(600ms)` unreliable.
- **LOW: `InstanceConfig.Validate()` returns single error** — The existing `Validate()` returns one `error`, not a collection. To return multiple validation errors (as D-14 requires), the handler needs to either: call validate for each field individually, or refactor Validate to return an error list. The plan doesn't address this mismatch.

### Suggestions

1. **Implement `UpdateConfig(fn func(*Config) error) error`** — This single function solves the race condition, the shared-state mutation, and the failure rollback in one abstraction. It should: (a) lock configMu, (b) deep-copy current config, (c) call fn with the copy, (d) write to viper if fn returns nil, (e) unlock. The handler just calls `UpdateConfig(func(cfg *Config) error { cfg.Instances = append(...); return nil })`.
2. **Inject config reader into handler** — Change `NewInstanceConfigHandler` to accept a `getConfig func() *Config` parameter. In production pass `config.GetCurrentConfig`, in tests pass a closure returning a fixed config. This eliminates the global state dependency in tests entirely.
3. **Use `io.ReadAll` + conditional unmarshal for Copy** — Read the body bytes first, check if empty, then unmarshal only if non-empty. This handles the empty body case cleanly.
4. **Add `StopInstanceByName(name string)` to lifecycle package** — Even as a Phase 51 TODO, having this function makes the delete endpoint much safer.
5. **Validate returns error slice** — Either extend `InstanceConfig.Validate()` to return `[]error` or create a separate `ValidateAll()` method that collects all validation errors for the 422 response.

---

## Consensus Summary

### Agreed Strengths

1. **Pattern adherence** — Both reviewers highlight that the plans closely follow existing codebase patterns (handler struct, auth middleware, Go 1.22 routing). This reduces integration risk.
2. **Viper integration correctness** — Both agree that using `v.Set()` + `v.WriteConfig()` is the right approach for persisting config changes.
3. **Separation of concerns** — The handler correctly delegates hot-reload to the existing watcher rather than trying to trigger it directly.
4. **Comprehensive decision documentation** — The 17 decisions in CONTEXT.md provide excellent traceability.

### Agreed Concerns

1. **CRITICAL: Read-modify-write race condition** (both HIGH) — Both reviewers independently identified that `configMu` only protects the viper write, not the full cycle. Concurrent API requests can cause data loss. Both suggest an `UpdateConfig` higher-order function as the fix.

2. **CRITICAL: Shared state mutation without deep copy** (both HIGH) — Both identified that `GetCurrentConfig()` returns a pointer to shared state. Mutating it directly risks corruption on `SaveConfig` failure. Both recommend working on a deep copy.

3. **HIGH: Test setup complexity and flakiness risk** (both raised) — Both note that Plan 02's approach of using real file watchers in tests is fragile, especially on Windows. Both suggest either injecting a config reader dependency or adding test helper functions.

4. **MEDIUM: Delete stops all instances** (both MEDIUM) — Both note the operational impact of `StopAllNanobots` on delete. Accepted for Phase 50 but should be tracked for improvement.

5. **MEDIUM: Copy endpoint empty body handling** (both MEDIUM) — Both note that `json.Decode` on empty body returns `io.EOF` and the plan doesn't provide a concrete implementation.

### Divergent Views

1. **StopAllNanobots scope** — OpenCode focuses on the recovery path when SaveConfig fails after stopping. Claude internal focuses on the fact that `StopAllNanobots` kills ALL system-wide `nanobot.exe` processes, including those not managed by the auto-updater. The system-wide kill is the more concerning issue and should be clearly documented as a known limitation.

2. **Validate error aggregation** — OpenCode flags this as LOW (suggest collecting all errors in one pass). Claude internal raises it to MEDIUM because the existing `Validate()` API returns a single error, requiring either handler workarounds or API changes to produce the multi-error 422 response format specified in D-14.

### Recommended Actions Before Implementation

1. **Add `UpdateConfig(fn func(*Config) error) error`** to config package — solves race condition + state corruption + failure rollback (both reviewers agree)
2. **Inject config reader** `func() *Config` into handler constructor — enables clean unit testing without file watchers
3. **Handle empty body in Copy** — use `io.ReadAll` + conditional unmarshal
4. **Address multi-error validation** — either refactor Validate to return `[]error` or create a dedicated validation collector in the handler
5. **Document StopAllNanobots limitation** — add a clear comment and track Phase 51 for targeted instance stop

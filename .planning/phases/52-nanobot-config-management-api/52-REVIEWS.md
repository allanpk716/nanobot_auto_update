---
phase: 52
reviewers: [opencode]
reviewed_at: 2026-04-12T04:25:00Z
plans_reviewed: [52-01-PLAN.md, 52-02-PLAN.md]
skipped: [gemini (API 401 auth error), claude (running inside Claude Code)]
---

# Cross-AI Plan Review — Phase 52

## OpenCode Review

### PLAN 52-01: NanobotConfigManager Core + GET/PUT Endpoints

#### Summary

Plan 52-01 establishes a clean separation between nanobot config management logic (NanobotConfigManager) and API routing (NanobotConfigHandler). The file layout follows existing project conventions well. However, there are several significant gaps around Windows-specific path handling, route path consistency with requirements, missing DELETE cleanup, and the lack of nanobot config schema definition that need addressing before implementation.

#### Strengths

- **Clean separation of concerns**: Isolating nanobot config logic in `internal/nanobot/config_manager.go` avoids polluting the existing `internal/config/` package and maintains single-responsibility
- **Follows existing patterns**: Handler struct + constructor injection, Go 1.22 mux patterns, authMiddleware wrapping all match the established conventions in `server.go:108-116`
- **Mutex for write protection**: `sync.Mutex` is appropriate given low write frequency; aligns with the project's existing approach in `config.UpdateConfig()`
- **Regex-based config path parsing**: Using `--config\s+["']?([^"'\s]+)["']?` is pragmatic for extracting config paths from start commands
- **Non-blocking callback design** (in plan 52-02): Warns but doesn't fail the primary operation — good resilience pattern

#### Concerns

- **HIGH — Windows tilde expansion**: The plan uses `~/.nanobot-{name}/config.json` as the default config path. On Windows, `~` is not natively expanded by the OS. The plan must explicitly call out `os.UserHomeDir()` for home directory resolution. The CONTEXT.md D-03 already specifies this, and the RESEARCH code example shows the correct implementation, but the PLAN task description should be more explicit.
- **HIGH — Route path inconsistency**: The requirements specify `GET /api/v1/instances/{name}/nanobot-config`, but the plan uses `GET /api/v1/instance-configs/{name}/nanobot-config`. While the plan's choice better matches existing Phase 50 routes (`/api/v1/instance-configs/{name}`), the requirements and plan contradict each other. This must be reconciled before implementation.
- **HIGH — No DELETE integration**: Phase 50's `HandleDelete` stops nanobot processes but does not clean up the nanobot config directory. Plan 52-02 adds callbacks for Create and Copy but omits Delete. When an instance is deleted, its `~/.nanobot-{name}/` directory with config.json will be orphaned on disk.
- **MEDIUM — No nanobot config schema defined in plan**: `GenerateDefaultConfig(port, workspace)` is described textually but the actual JSON schema is not shown in the plan. CONTEXT.md specifics section has the full example, but the plan should reference it explicitly.
- **MEDIUM — Config path edge cases on Windows**: `ParseConfigPath` must handle backslash paths (`C:\Users\...`), quoted paths with spaces, UNC paths (`\\server\share\...`), and forward slashes. The regex `[^"'\s]+` should handle most cases but edge cases need explicit test coverage.
- **MEDIUM — ReadConfig after WriteConfig race**: NanobotConfigManager has its own `sync.Mutex`, but `config.UpdateConfig()` uses a separate mutex. There is a window where auto-updater config is updated but nanobot config hasn't been written yet (or vice versa).
- **LOW — T-52-03 (Path traversal) accepted too easily**: Instance names are admin-controlled but `InstanceConfig.Validate()` only checks for non-empty name — does not restrict characters. Consider adding a character whitelist.
- **LOW — Missing Content-Type validation on PUT**: Handler should validate `Content-Type: application/json` on PUT requests.

#### Suggestions

1. Add explicit `os.UserHomeDir()` handling in `ParseConfigPath` — construct path using `filepath.Join()`, not string concatenation with `~/`
2. Define the default nanobot config.json schema explicitly in the plan (reference CONTEXT.md specifics)
3. Add `onDeleteInstance` callback (or document as out of scope with rationale) to prevent orphaned nanobot config directories
4. Add a path safety check in `ParseConfigPath` — validate instance names contain only `[a-zA-Z0-9_-]`
5. Reconcile the API route path — update either requirements or plan to use consistent path
6. Consider an integration test for the full flow: create instance → verify nanobot config dir → read via API → update via API → verify file changed

#### Risk Assessment: MEDIUM

Core design is sound and follows established patterns. Main risks: (1) Windows tilde expansion, (2) route path inconsistency, (3) missing DELETE cleanup. All addressable with small additions.

---

### PLAN 52-02: Callback Injection + Comprehensive Tests

#### Summary

Plan 52-02 introduces callback injection into the existing InstanceConfigHandler to trigger nanobot config creation/cloning during instance lifecycle events. The non-blocking callback pattern is well-chosen for resilience. However, the callback mechanism introduces hidden coupling that isn't visible from the InstanceConfigHandler's type signature, and the callback ordering relative to the config write could lead to inconsistent states.

#### Strengths

- **Non-blocking callbacks**: Warning on callback failure without failing the primary operation is the right trade-off
- **Comprehensive test coverage plan**: Listed test categories are thorough and well-organized
- **Setter methods for callbacks**: Avoids changing `NewInstanceConfigHandler`'s constructor signature
- **White-box test access**: Tests in same package can access internal types, matching existing patterns

#### Concerns

- **HIGH — Callback timing relative to config.UpdateConfig()**: The callback fires after the config file is persisted to disk. If callback fails, auto-updater config shows instance exists but no nanobot config. Next GET to `/nanobot-config` will return file-not-found. This inconsistent state should be documented or mitigated.
- **HIGH — Hidden coupling via callbacks**: `InstanceConfigHandler` gains implicit dependencies through setter-injected callbacks. This coupling is invisible from the constructor. Future developers modifying `HandleCreate` may not realize callbacks exist.
- **MEDIUM — Missing Windows-specific test cases**: Test plan doesn't explicitly call out `C:\Users\xxx\...` style paths, paths with spaces, backslash vs forward slash handling on Windows.
- **MEDIUM — No test for concurrent callback + config mutation**: Plan mentions concurrent writes testing but doesn't test: request A creates instance (triggers callback), request B simultaneously reads that instance's nanobot config.
- **MEDIUM — CloneConfig field update list unclear**: Plan says "update port and workspace" but requirements (NC-04) say "port and name fields updated." Plan must clarify which fields get updated during cloning.
- **LOW — Callback setter has no thread safety**: `SetOnCreateInstance` called while request being processed could cause data race (fine in practice since setters called once during setup).
- **LOW — Integration test scope unclear**: Should specify whether tests use existing `setupIntegrationTest` pattern or are unit-level.

#### Suggestions

1. Add lazy creation fallback on GET — if HandleGet discovers nanobot config file doesn't exist for a known instance, auto-create it (with warning log)
2. Document callback contract clearly with comment block on `InstanceConfigHandler`
3. Expand test matrix for Windows paths: backslash paths, spaces in paths, no --config flag
4. Clarify which fields CloneConfig updates — define exact list (port, workspace, name?)
5. Add test for inconsistent state scenario: create instance → simulate callback failure → GET nanobot-config → verify behavior
6. Consider a `NanobotConfigCallbacks` struct instead of individual setters for discoverability

#### Risk Assessment: MEDIUM

Callback injection approach is pragmatic and avoids breaking `NewServer` constructor, but introduces a hidden contract. Inconsistent state from failed callbacks is the primary risk — without a self-healing mechanism, users will encounter confusing 404 errors.

---

## Consensus Summary

### Agreed Strengths (validated by plan quality)

- Clean separation of concerns — independent `internal/nanobot/` package
- Follows established project patterns (handler struct, constructor injection, route registration)
- Non-blocking callback design for create/copy flows
- Comprehensive test plan covering all major code paths
- Mutex-protected writes for concurrency safety
- Regex-based --config parsing is pragmatic

### Agreed Concerns

1. **Route path inconsistency** (HIGH) — requirements say `/api/v1/instances/{name}/...` but plan uses `/api/v1/instance-configs/{name}/...`. Must reconcile before implementation.
2. **Missing DELETE cleanup** (HIGH) — no `onDeleteInstance` callback. Orphaned nanobot config directories after instance deletion.
3. **Inconsistent state from failed callbacks** (HIGH) — instance exists in config.yaml but nanobot config dir missing. Consider lazy-creation fallback on GET.
4. **Hidden coupling via callbacks** (HIGH) — future developers may miss the implicit dependency. Needs clear documentation.
5. **Windows path handling** (MEDIUM) — tilde expansion must use `os.UserHomeDir()`, test plan needs Windows-specific path cases.
6. **CloneConfig field update list** (MEDIUM) — NC-04 says "port and name" but plan only updates "port and workspace". Needs clarification.

### Divergent Views

No divergent views — single reviewer.

### Resolved by Existing Artifacts

Several OpenCode concerns are already addressed in the CONTEXT.md and RESEARCH.md:
- **Windows tilde expansion**: CONTEXT.md D-03 explicitly specifies `os.UserHomeDir()`. RESEARCH.md code example shows correct implementation with `filepath.Join(homeDir, ...)`.
- **Default config schema**: CONTEXT.md specifics section contains the full JSON example. PLAN 52-01 Task 1 includes `GenerateDefaultConfig` with field-level specification.
- **Route path**: PLAN uses `/api/v1/instance-configs/` which is consistent with Phase 50's existing routes. The requirements text uses `/api/v1/instances/` which is less consistent. Plan's choice is arguably better.

### Action Items for Implementation

| Priority | Item | Action |
|----------|------|--------|
| HIGH | Add onDeleteInstance or document rationale | Add to PLAN 52-02 or document as deferred |
| HIGH | Lazy creation fallback on GET | Consider adding auto-create on HandleGet when file missing |
| MEDIUM | Clarify CloneConfig field updates | Align with NC-04 — determine if "name" field exists in nanobot config |
| MEDIUM | Expand Windows path tests | Add C:\Users\... and backslash path test cases |
| LOW | Add Content-Type validation on PUT | Minor enhancement |
| LOW | Document callback contract | Add comments on InstanceConfigHandler struct |

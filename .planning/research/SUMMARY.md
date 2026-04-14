# Project Research Summary

**Project:** Nanobot Auto Updater
**Domain:** Go Windows service with embedded vanilla JS Web UI for nanobot instance management
**Researched:** 2026-04-13
**Confidence:** HIGH

## Executive Summary

v0.18.0 enhances the existing embedded Web UI with three feature groups: delete button protection (DEL-01/02), config directory customization (CFG-01/02/03), and JSON editor upgrade with syntax highlighting and real-time validation (EDT-01/02). The project is a single-binary Go Windows service that embeds all static assets via `embed.FS` -- no build step, no npm, no CDN. This constraint is the single most important factor driving all technology decisions.

Research converged strongly on two key conclusions. First, DEL-02 (delete confirmation dialog) is already shipped in v0.12 as UI-05 -- zero work required. DEL-01 is trivially a single line (`deleteBtn.disabled = isRunning`) because `isRunning` is already passed to `createInstanceCard()`. Second, the JSON editor integration is the highest-risk and highest-value change: replacing the existing `<textarea>` with a syntax-highlighting editor while preserving the `syncGuard` bidirectional sync pattern between form fields and JSON. The recommended editor is **Ace Editor v1.43.6** (`src-min-noconflict` build) at ~531 KB total, vendored into `internal/web/static/ace/` and served from embed.FS. STACK.md and FEATURES.md both independently converge on Ace Editor; ARCHITECTURE.md initially considered vanilla-jsoneditor but Ace is the stronger choice for this project's zero-build-step constraint (single script tag, no ES modules, built-in JSON worker for validation, and `setValue(str, -1)` does not trigger the change event -- a property that makes the syncGuard pattern cleaner than with a textarea).

The config directory features (CFG-02/03) require backend Go changes: a new `ConfigDir` field on `InstanceConfig`, a new `ParseConfigPathWithDir()` function, and path validation to prevent traversal attacks. CFG-01 (integrated config editor in create dialog) is the most complex feature overall and should be the final phase, depending on both the editor integration and the backend config directory work. The recommended build order is: DEL -> EDT -> CFG-backend -> CFG-frontend -> CFG-01 (integrated dialog).

Key risks include: breaking the syncGuard pattern during editor integration (mitigated by Ace's `setValue` not firing change events), editor container rendering invisible in the modal (mitigated by explicit height + `editor.resize()` call), and stale status after stop causing delete button to stay disabled (mitigated by optimistic UI state after lifecycle actions).

## Key Findings

### Recommended Stack

**Ace Editor v1.43.6 (`src-min-noconflict`)** is the sole new dependency. It replaces the plain `<textarea>` with a syntax-highlighted JSON editor providing real-time validation via a built-in Web Worker. Six files totaling ~531 KB (minified, no gzip) are vendored into `internal/web/static/ace/` and automatically included by the existing `//go:embed static/*` directive. No npm, no webpack, no CDN -- consistent with the project's zero-build-step philosophy.

**Core technologies:**
- **Ace Editor v1.43.6** (src-min-noconflict) -- JSON syntax highlighting + real-time validation via worker-json.js. Single `<script>` tag integration. `setValue(str, -1)` does not trigger change events, making syncGuard preservation trivial. ~531 KB embedded.
- **Existing embed.FS** -- All static assets baked into Go binary. No changes to embed directive needed; new `ace/` subdirectory is auto-included.
- **Existing vanilla JS patterns** -- syncGuard bidirectional sync, showModal/closeModal, toast notifications, createInstanceCard with isRunning parameter. All preserved and extended, not replaced.

**Why Ace over alternatives evaluated:**
- Monaco Editor (~5 MB) is designed for full IDEs, requires complex worker setup, and violates the binary size constraint.
- CodeMirror 6 requires ES module imports or a bundler, both of which conflict with the zero-build-step approach.
- vanilla-jsoneditor (evaluated in ARCHITECTURE.md) uses CodeMirror 6 internally, inheriting the same ES module complexity.

### Expected Features

**Must have (table stakes) -- this milestone:**
- **DEL-01: Delete button disabled when running** -- Users expect destructive actions to be disabled on active resources. One-line change: `deleteBtn.disabled = isRunning`.
- **EDT-01: JSON syntax highlighting** -- A plain textarea for JSON config editing is sub-par. Ace Editor provides bracket matching, line numbers, and color-coded tokens for strings/numbers/booleans/keys.
- **EDT-02: Real-time JSON validation** -- Ace's `worker-json.js` runs in a Web Worker, annotating errors inline (red wavy underlines, gutter markers) without blocking the main thread. Enhancement of existing `try { JSON.parse }` pattern.
- **CFG-02: Custom config directory** -- Users managing multiple environments need control over where nanobot configs live. Requires backend `config_dir` field and `ParseConfigPathWithDir()`.
- **CFG-03: Auto-create directory / read existing** -- Partially implemented. `WriteConfig()` already calls `MkdirAll`. New logic: read existing config.json from custom directory if present.
- **DEL-02: Delete confirmation** -- Already shipped in v0.12 (UI-05). Zero work.

**Should have (competitive) -- this milestone:**
- **CFG-01: Integrated create+config dialog** -- Combines two separate dialog flows. Most complex feature. Depends on EDT-01/02 and CFG-02/03. Reduces friction for the common "create then configure" workflow.

**Defer (v2+):**
- JSON format/pretty-print button -- Nice to have, trigger when users report pasting minified JSON.
- Unsaved changes indicator -- Trigger when users lose edits by accidentally closing the dialog.
- Config templates, diff view, import/export -- Future milestone considerations.

### Architecture Approach

All changes integrate into the existing vanilla JS frontend (`home.js`, ~1462 lines) and Go backend API. No new architectural patterns or frameworks are introduced. The embed.FS single-binary model is preserved.

**Major components:**
1. **home.js createInstanceCard()** -- DEL-01 adds `deleteBtn.disabled = isRunning`. The existing 5-second polling re-render naturally reflects state changes.
2. **home.js showNanobotConfigDialog()** -- EDT-01/02 replaces `<textarea>` with Ace Editor div. The `syncGuard` bidirectional sync is preserved by mapping `textarea.value` reads/writes to `editor.getValue()`/`editor.setValue()`. Ace's `setValue(str, -1)` does NOT fire the change event, making the guard pattern actually cleaner than with a textarea.
3. **ConfigManager (config_manager.go)** -- CFG-02/03 adds `ParseConfigPathWithDir(startCommand, instanceName, configDir)` with precedence: explicit `configDir` > `--config` flag in start_command > `~/.nanobot/config.json` default. Path validation prevents traversal attacks.
4. **InstanceConfig struct (config.go)** -- CFG-02 adds `ConfigDir string` field. Flows through POST API -> HandleCreate -> onCreateInstance callback -> ConfigManager.
5. **Ace Editor vendored files** -- 6 files in `internal/web/static/ace/`: ace.js, mode-json.js, worker-json.js, worker-base.js, theme-chrome.js, ext-error_marker.js. Served by existing static file handler. Web Workers load correctly because Ace resolves worker URLs relative to `basePath`.

**Files modified (total):** home.js, home.html, style.css, config.go, instance_config_handler.go, config_manager.go, nanobot_config_handler.go, server.go.
**Files added:** 6 Ace Editor files in `internal/web/static/ace/`.

### Critical Pitfalls

1. **CDN dependency breaks offline deployment** -- The project runs on internal networks. Any `<script src="https://cdn...">` tag breaks when there is no internet. Must vendor all Ace files into embed.FS. Set `ace.config.set('basePath', '/static/ace/')`.

2. **syncGuard pattern breaks with editor integration** -- If the editor library's `onChange` fires on programmatic `setValue`, the bidirectional sync enters an infinite loop. Ace Editor's `setValue(str, -1)` does NOT fire change events, which makes this a non-issue for Ace specifically. But every code path that calls `editor.setValue()` must still be wrapped in `syncGuard` acquire/release as a safety invariant.

3. **Ace Editor container invisible in modal** -- If the container `<div>` has no explicit height, Ace renders at zero height (blank area). Must set `style="height: 400px; width: 100%"` and call `editor.resize()` after the modal becomes visible.

4. **Delete button state drifts after stop** -- The 5-second status poll may return stale "running" after a stop action because the process hasn't fully exited. Mitigate with optimistic UI: after stop API returns 200, immediately enable delete button; only revert if a future poll explicitly says running.

5. **Config directory path traversal** -- CFG-02 allows user-specified paths. Must validate: reject paths containing `..`, reject absolute paths outside user home directory, reject Windows reserved device names (NUL, CON, PRN, etc.). All directory creation must go through mutex-protected `ConfigManager.WriteConfig()`.

## Implications for Roadmap

Based on research, suggested phase structure:

### Phase 1: Delete Button Protection (DEL-01)
**Rationale:** Trivially simple -- 3 lines of JS + 1 CSS rule. Zero dependencies on other features. Immediate safety improvement. Builds confidence that the 5-second polling re-render correctly reflects state changes. DEL-02 is already done (v0.12 UI-05).
**Delivers:** Delete button disabled when instance is running, with tooltip. Delete confirmation already exists.
**Addresses:** DEL-01, DEL-02 (done)
**Avoids:** Pitfall 4 (delete button state drift) by leveraging existing polling re-render. Optional: add optimistic state after stop for faster feedback.
**Files:** home.js, style.css

### Phase 2: JSON Editor Integration (EDT-01, EDT-02)
**Rationale:** The core infrastructure change. Replaces `<textarea>` with Ace Editor in the existing config dialog. This is the foundation for CFG-01 (integrated create+config dialog). EDT-01 and EDT-02 are tightly coupled and should be delivered together because Ace Editor provides both for free -- syntax highlighting via `mode-json.js` and validation via `worker-json.js`. Independent of DEL and CFG features, can be built in parallel with Phase 3.
**Delivers:** JSON syntax highlighting, bracket matching, line numbers, real-time validation with error annotations, and enhanced error display using existing `#nb-json-error` div.
**Uses:** Ace Editor v1.43.6 (6 vendored files, ~531 KB)
**Implements:** Editor instance lifecycle (create on modal open, destroy on close), syncGuard preservation, debounced form sync, validation error display.
**Avoids:** Pitfall 1 (CDN -- files vendored), Pitfall 2 (syncGuard -- Ace's setValue is safe), Pitfall 3 (container height -- explicit height + resize call), Pitfall 7 (editor not destroyed -- destroy in closeModal), Pitfall 8 (timer conflict -- verify modal isolation from 5s poll).
**Files:** 6 new Ace files in `internal/web/static/ace/`, home.html, home.js, style.css

### Phase 3: Config Directory Backend (CFG-02, CFG-03)
**Rationale:** Full-stack changes starting with Go backend. Adding `ConfigDir` to InstanceConfig, creating `ParseConfigPathWithDir()`, and implementing path validation. Backend-first allows isolated testing via curl/httpie before frontend integration. The callback chain (server.go -> ConfigManager) must be verified end-to-end.
**Delivers:** API accepts `config_dir` field, custom directory takes precedence over `--config` flag, path traversal prevention, auto-creation of directories (existing `MkdirAll` in `WriteConfig`), reading existing config.json from custom directory.
**Implements:** ParseConfigPathWithDir(), modified CreateDefaultConfig(), path validation in HandleCreate.
**Avoids:** Pitfall 5 (path traversal -- server-side validation), Pitfall 6 (MkdirAll race -- all paths through mutex-protected WriteConfig).
**Files:** config.go, instance_config_handler.go, config_manager.go, nanobot_config_handler.go, server.go

### Phase 4: Config Directory Frontend + Integrated Dialog (CFG-01)
**Rationale:** Depends on both Phase 2 (editor component to embed in create dialog) and Phase 3 (backend API accepting config_dir). The frontend config_dir field addition is simple. CFG-01 (combining create and config editing into one dialog) is the most complex feature and should be the final phase -- it embeds the Ace Editor hybrid editor into the create dialog flow.
**Delivers:** Config directory field in create dialog, integrated config editing in create dialog (two-step: create instance then edit config in same dialog).
**Uses:** Ace Editor (from Phase 2), config_dir API (from Phase 3)
**Avoids:** Pitfall 10 (config read-after-write race -- ensure config is ready before editor loads).
**Files:** home.js

### Phase Ordering Rationale

- DEL first because it is zero-dependency, lowest-risk, and delivers immediate visible value.
- EDT second because it is independent (no API changes) and is the foundational change that CFG-01 depends on. It is also the highest-risk integration point due to syncGuard preservation, so getting it done early isolates the risk.
- CFG-backend third because it is pure Go, can be tested independently via API calls, and provides the foundation for CFG-frontend.
- CFG-frontend last because it depends on both EDT (editor to embed) and CFG-backend (API accepting config_dir). CFG-01 (integrated dialog) is the capstone feature.

Note: Phases 2 and 3 can be built in parallel since they have no mutual dependencies.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 2 (EDT):** Moderate -- Ace Editor integration with embed.FS and Web Workers is well-documented but the syncGuard preservation and modal lifecycle need careful implementation. No external research needed; all patterns are in the research files.
- **Phase 4 (CFG-01):** Moderate -- Combining two dialog flows (create + config edit) into one has no established pattern in this codebase. Implementation approach (two-step vs. buffered vs. atomic) needs to be decided during planning.

Phases with standard patterns (skip research-phase):
- **Phase 1 (DEL):** Trivially simple. One conditional disabled attribute. No research needed.
- **Phase 3 (CFG backend):** Standard Go backend work. New struct field, new function with fallback to existing logic, path validation. Patterns are well-established in the codebase.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Ace Editor v1.43.6 verified via GitHub API and cdnjs. File sizes verified via direct download. Integration pattern verified against existing codebase (syncGuard, modal system, embed.FS). |
| Features | HIGH | All 7 requirements analyzed against existing codebase. DEL-02 confirmed already shipped. DEL-01 confirmed trivial (single line). Feature dependencies mapped explicitly. |
| Architecture | HIGH | All integration points identified with exact line numbers. Component boundaries clear. No new architectural patterns needed. |
| Pitfalls | MEDIUM | All pitfalls derived from codebase analysis and Go stdlib docs (HIGH confidence). CDN/pitfall findings from web search are MEDIUM confidence. Some pitfalls are theoretical and may not manifest (e.g., timer conflict with modal). |

**Overall confidence:** HIGH

### Gaps to Address

- **Ace Editor `setValue` behavior:** STACK.md states `setValue(str, -1)` does not fire change events. This is documented but should be verified during Phase 2 implementation before relying on it for syncGuard logic. If it does fire, a simple `syncGuard` wrapper still works -- just adds one more guard point.
- **Web Worker loading from embed.FS:** Ace spawns `worker-json.js` via `new Worker(url)`. Research confirms this works because the Go server serves all files under `/static/`. Should verify in Phase 2 integration testing by disconnecting network and confirming validation still works.
- **CFG-01 dialog design:** Multiple approaches exist (two-step create-then-edit vs. single atomic form vs. tabbed wizard). No research was done on UX best practices for combined create+configure flows. Decision needed during Phase 4 planning.
- **ARCHITECTURE.md vs STACK.md editor recommendation:** ARCHITECTURE.md evaluated vanilla-jsoneditor while STACK.md evaluated Ace Editor. Both are valid but Ace is the stronger choice for this project's constraints (no ES modules, no bundler, single script tag, proven embed.FS compatibility). This summary recommends Ace Editor.

## Sources

### Primary (HIGH confidence)
- Codebase analysis: home.js (1462 lines), style.css, home.html, config_manager.go, nanobot_config_handler.go, instance_config_handler.go, server.go, handler.go, config.go
- GitHub API -- `api.github.com/repos/ajaxorg/ace-builds/tags`: v1.43.6 confirmed as latest
- jsDelivr CDN -- `cdn.jsdelivr.net/npm/ace-builds@1.43.6/src-min-noconflict/`: All required files verified, sizes measured by direct download
- Go stdlib: os.MkdirAll race condition (golang/go#1736), Windows reserved names (golang/go#24556)

### Secondary (MEDIUM confidence)
- Ace Editor documentation -- basePath configuration, setValue behavior, worker loading, annotation API (based on training data and cdnjs file listing)
- Competitor analysis: Portainer, PM2, Docker Desktop UX patterns for delete protection and config editing
- vanilla-jsoneditor npm page and jsDelivr CDN -- library capabilities evaluated as alternative
- CodeMirror 6 documentation -- ruled out due to ES module requirement

### Tertiary (LOW confidence)
- npm page for ace-builds (shows v1.39.0, stale cache -- GitHub tags are authoritative)
- Specific Ace Editor version numbers and CDN URL patterns -- verified but should be re-confirmed at integration time

---
*Research completed: 2026-04-13*
*Ready for roadmap: yes*

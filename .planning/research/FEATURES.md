# Feature Landscape

**Domain:** Embedded Web UI enhancements for JSON config editing, delete protection, and config directory management
**Researched:** 2026-04-13
**Scope:** v0.18.0 milestone -- Instance management Web UI enhancements (7 requirements: DEL-01, DEL-02, CFG-01, CFG-02, CFG-03, EDT-01, EDT-02)
**Confidence:** HIGH (based on existing codebase analysis, current UI patterns, and established JSON editor ecosystem knowledge)

## Table Stakes

Features users expect. Missing = the management UI feels incomplete or unsafe.

| Feature | Why Expected | Complexity | Dependencies on Existing | Notes |
|---------|--------------|------------|--------------------------|-------|
| Delete button disabled when instance running (DEL-01) | Deleting a running instance can cause orphaned processes, corrupted state, and data loss. Every admin panel that manages running services disables destructive actions on active resources. Portainer, Docker Desktop, PM2 all do this. | Low | `createInstanceCard()` in home.js already has `isRunning` parameter. Just add `deleteBtn.disabled = isRunning` condition. | Currently the delete dialog (UI-05) shows a warning for running instances but still allows deletion. Changing to disable-first is a strict safety upgrade. |
| Delete confirmation dialog (DEL-02) | Already built in v0.12 (UI-05). The `showDeleteDialog()` function fetches instance config, shows running warning, and has cancel/delete buttons. This requirement is already satisfied. | None | Already exists in home.js lines 443-534. | **Already shipped in v0.12.** No additional work needed. This requirement is a no-op. |
| JSON syntax highlighting (EDT-01) | The current `nanobot-json-textarea` is a plain `<textarea>` with monospace font and no color. For a JSON config editor editing complex nested structures (agents.defaults, providers, channels, gateway), plain text makes it hard to spot structure. Every modern config editor provides syntax highlighting. | Medium | Current hybrid editor in `showNanobotConfigDialog()` (home.js lines 536-788). The `syncGuard` bidirectional sync between form fields and JSON textarea. | Must preserve the syncGuard pattern. The highlighted editor must still support `getValue()`/`setValue()` for sync. |
| Real-time JSON validation (EDT-02) | Current implementation already has `try { JSON.parse(jsonStr) } catch(e)` with error display in `#nb-json-error` div (lines 713-734). This is table stakes for any JSON editor. The improvement is to enhance the UX: show error location (line, column), highlight the problematic area, and provide instant feedback as user types (not just on save). | Low-Medium | Current validation in JSON->Form sync listener and save handler. The `#nb-json-error` div already exists. | Enhancement of existing behavior, not a new feature. The baseline is already working. |

## Differentiators

Features that set the product apart from other management UIs. Not required, but valuable.

| Feature | Value Proposition | Complexity | Dependencies on Existing | Notes |
|---------|-------------------|------------|--------------------------|-------|
| Config directory field in create dialog (CFG-02) | Currently, config directory is auto-derived from `start_command --config` flag or defaults to `~/.nanobot/`. Allowing users to specify a custom config directory gives them full control over where nanobot configs live. This matters for users with multiple environments or shared storage. | Medium | `ParseConfigPath()` in config_manager.go extracts from `--config` flag. Need to add a new UI field and pass the value through the create API. Backend needs to store config_dir per instance and use it when resolving paths. | Requires backend change: new `config_dir` field in `InstanceConfig` struct, modification to `ParseConfigPath` to accept explicit directory. |
| Auto-create config directory on startup (CFG-03) | When a user specifies a custom config directory that does not exist, the system should create it automatically and optionally read any existing config.json from it. This prevents "directory not found" errors and makes the first-time setup smooth. | Low | `WriteConfig()` already calls `os.MkdirAll(dir, 0755)` (config_manager.go line 177). `HandleGet` already has lazy-creation fallback for missing config files. | Partially implemented. The auto-create on write exists; what is new is: (1) creating the directory at instance startup time rather than on first config write, and (2) reading existing config.json from the directory if it already exists when creating the instance. |
| Integrated config editor in create dialog (CFG-01) | Currently, creating an instance is a two-step process: create the instance, then open the config editor. Integrating the config editor into the create dialog lets users set up everything in one flow. Reduces friction for the common "create + configure" workflow. | Medium-High | Existing `showCreateDialog()` and `showNanobotConfigDialog()` are separate functions. The hybrid editor HTML template from nanobot config dialog needs to be embedded into the create dialog. | This is the most complex feature. Requires: (1) combining two separate dialog flows, (2) creating an instance first (to get API endpoints), (3) then editing config in same dialog, or (4) buffering config edits and submitting both atomically. |
| Error line highlighting in JSON editor | When JSON has a syntax error, highlight the exact line and column where the error occurs (red underline, gutter marker). This is a quality-of-life feature that makes fixing errors much faster than reading "Unexpected token at position 142". | Low-Medium | Depends on EDT-01 (syntax highlighting editor). Native feature of Ace Editor and CodeMirror. | If using Ace Editor, this comes free with JSON mode. If using custom approach, requires manual error position parsing from `JSON.parse` error messages. |
| JSON format/pretty-print button | A "Format" button that reformats the JSON with proper indentation (2 spaces). Users may paste minified JSON or accidentally break formatting. | Low | Current save handler already does `JSON.stringify(jsonObj, null, 2)`. | Simple button that parses and re-stringifies. Add to the JSON editor toolbar area. |

## Anti-Features

Features that seem good but create problems. Explicitly exclude.

| Anti-Feature | Why Requested | Why Problematic | Alternative |
|--------------|---------------|-----------------|-------------|
| Monaco Editor for JSON editing | VS Code-quality editing with IntelliSense, schema validation, auto-complete | Monaco is 2-5 MB (min) / ~700 KB (gzip). This project uses `embed.FS` for single-binary deployment with no build step. Adding a 700 KB+ dependency would significantly increase binary size. Monaco requires AMD loader, web workers, and complex setup. Overkill for editing a single config.json file. | Ace Editor (~300 KB, ~90 KB gzip, CDN or embedded) or CodeMirror 6 (~40 KB gzipped). For this use case, even a custom lightweight highlighter is sufficient. |
| JSON Schema validation | Validate config against a formal schema (e.g., "gateway.port must be number") | Requires maintaining a schema that matches nanobot's internal config structure. Every nanobot version change would require schema updates. The nanobot project does not publish a formal JSON Schema. Server-side validation is better handled by nanobot itself when it reads the config. | Keep validation to syntax correctness (valid JSON). Let nanobot handle semantic validation on startup. |
| Auto-save on edit | Save config changes automatically as user types | Risk of saving broken JSON mid-edit. Conflicts with the existing pattern where the user explicitly clicks "Save". Also, the config API (`PUT /nanobot-config`) does a full file write -- auto-save would cause excessive I/O and race conditions with form<->JSON sync. | Keep explicit save button. Add unsaved changes indicator (dot in title or "unsaved" badge) as a lighter alternative. |
| Undo/redo in JSON editor | Ctrl+Z / Ctrl+Y support for JSON editing | Ace Editor and CodeMirror provide this built-in, but if using a custom textarea approach, implementing a proper undo stack is surprisingly complex (handle multi-character input, selection replacement, programmatic changes from sync). The syncGuard bidirectional sync would complicate undo/redo because form changes programmatically update the JSON textarea. | Use Ace Editor if undo/redo is important (it handles this natively). Otherwise, accept that undo is limited to the browser's native textarea undo (which is lost when the textarea value is set programmatically). |
| Delete button hidden for running instances | Remove the delete button entirely when instance is running, instead of disabling it | Users may not realize delete is even possible. When they stop the instance, the button suddenly appears, which is unexpected. Hidden actions are confusing -- better to show them as disabled with a clear reason. | Disable with tooltip explaining "Stop the instance first to enable deletion". Consistent with how edit/copy buttons are always visible. |

## Feature Dependencies

```
EDT-01 (Syntax Highlighting)
    └──requires──> Choosing JSON editor approach (Ace/CodeMirror/custom)
                   └──EDT-02 (Real-time Validation) is enhanced by EDT-01
                       └──Both must preserve syncGuard bidirectional sync

CFG-01 (Integrated Config in Create Dialog)
    └──requires──> EDT-01 (syntax highlighting editor to embed)
    └──requires──> CFG-02 (config directory field to pass to backend)
    └──requires──> Backend API change (atomic create+config or two-step)

CFG-02 (Custom Config Directory)
    └──requires──> Backend: new config_dir field in InstanceConfig
    └──requires──> Backend: modified ParseConfigPath to accept explicit dir
    └──enhances──> CFG-01 (config dir is part of create dialog form)

CFG-03 (Auto-create Directory / Read Existing)
    └──requires──> CFG-02 (directory path must come from user or auto-derived)
    └──enhances──> CFG-01 (show existing config in create dialog if directory exists)

DEL-01 (Delete Button Disabled)
    └──standalone──> No dependencies. Pure UI change in createInstanceCard().

DEL-02 (Delete Confirmation Dialog)
    └──already exists──> Shipped in v0.12 (UI-05). No work needed.
```

### Dependency Notes

- **EDT-01 and EDT-02 are tightly coupled**: The JSON editor library choice determines how validation is implemented. Ace Editor provides both for free. A custom approach requires separate implementation for each.
- **CFG-01 is the highest-complexity feature**: It combines two existing separate flows. The implementation order should be: (1) get syntax highlighting working in existing config dialog (EDT-01/EDT-02), (2) add config directory field (CFG-02), (3) then combine into create dialog (CFG-01).
- **CFG-02 requires backend changes**: The current `InstanceConfig` struct does not have a `config_dir` field. The `ParseConfigPath()` function currently extracts from `start_command --config` flag. Supporting an explicit directory requires both a struct change and a function change.
- **DEL-01 is independent and trivial**: A single line change in `createInstanceCard()`. Can be done in any phase.

## MVP Definition

### Launch With (this milestone)

Minimum viable product -- what is needed to fulfill all 7 requirements.

- [x] **DEL-02: Delete confirmation** -- Already shipped in v0.12 (UI-05). No work.
- [ ] **DEL-01: Delete button disabled when running** -- One-line UI change in `createInstanceCard()`. Add `deleteBtn.disabled = isRunning;` and a tooltip.
- [ ] **EDT-01 + EDT-02: JSON editor with syntax highlighting and real-time validation** -- Replace plain `<textarea>` with Ace Editor in JSON mode. Ace provides syntax highlighting, error annotations (gutter markers, red underlines), and `session.getAnnotations()` for reading errors. Must preserve `syncGuard` bidirectional sync.
- [ ] **CFG-02: Config directory field in create dialog** -- Add `<input>` field to `buildInstanceFormHtml()`. Add `config_dir` to `InstanceConfig` struct. Modify `ParseConfigPath()` to accept explicit directory.
- [ ] **CFG-03: Auto-create directory, read existing config** -- `WriteConfig()` already calls `MkdirAll`. Add logic to read existing config.json when directory already exists.
- [ ] **CFG-01: Integrated config editor in create dialog** -- Embed the hybrid editor (form + JSON) into the create dialog. Two-step: create instance first, then edit config in same dialog flow.

### Add After Validation (future milestones)

- [ ] **Format/pretty-print button** -- Trigger for adding: users report pasting minified JSON frequently.
- [ ] **Unsaved changes indicator** -- Trigger for adding: users accidentally close dialog losing edits.
- [ ] **JSON folding/collapse** -- Trigger for adding: configs become deeply nested and hard to navigate.

### Future Consideration (v2+)

- [ ] **Config templates** -- Pre-defined configs for common setups (gateway-only, gateway+telegram, etc.)
- [ ] **Config diff view** -- Show what changed before saving
- [ ] **Config import/export** -- Upload/download config.json files

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| DEL-01: Delete button disabled | HIGH (safety, prevents accidents) | LOW (one line) | P1 |
| EDT-01: JSON syntax highlighting | HIGH (usability, readability) | MEDIUM (Ace Editor integration) | P1 |
| EDT-02: Real-time JSON validation | HIGH (usability, error prevention) | LOW-MEDIUM (Ace provides it free) | P1 |
| CFG-02: Config directory field | MEDIUM (flexibility) | MEDIUM (frontend + backend) | P1 |
| CFG-03: Auto-create/read directory | MEDIUM (smooth setup) | LOW (partially done) | P1 |
| CFG-01: Integrated create+config dialog | MEDIUM (reduces friction) | HIGH (combine two flows) | P2 |
| DEL-02: Delete confirmation | HIGH (safety) | NONE (already done) | Done |
| Format button | LOW (nice to have) | LOW | P3 |
| Unsaved changes indicator | LOW (nice to have) | LOW | P3 |

**Priority key:**
- P1: Must have for this milestone (6 features)
- P2: Should have, add when possible (1 feature)
- P3: Nice to have, future consideration

## Implementation Approach: JSON Editor Selection

The critical decision for EDT-01/EDT-02 is which editor approach to use. Given the project constraints:

| Criterion | Ace Editor (recommended) | CodeMirror 6 | Custom textarea overlay |
|-----------|------------------------|--------------|------------------------|
| Bundle size | ~300 KB / ~90 KB gzip | ~40 KB gzip (but needs ESM build) | 0 KB |
| CDN setup | Simple `<script>` tags | Needs ESM imports or bundler | None (pure vanilla) |
| JSON syntax highlighting | Built-in mode | Built-in mode | Must implement manually |
| JSON validation | Built-in worker (error markers in gutter) | Via `@codemirror/lint` plugin | Manual `JSON.parse` + error display |
| `getValue()`/`setValue()` API | Yes (`editor.getValue()` / `editor.setValue()`) | Yes (more complex state management) | Yes (textarea `.value`) |
| SyncGuard compatibility | Yes -- replace textarea reads with `editor.getValue()` | Yes -- but more setup | Already working |
| No build step | Yes -- pure CDN `<script>` | No -- needs ESM bundler or esm.sh CDN | Yes |
| embed.FS compatible | Yes -- either embed minified files or use CDN | Difficult without bundler | Already embedded |
| Error line/column highlighting | Yes -- automatic gutter markers | Yes -- via lint plugin | Manual implementation |
| Themes | Many built-in themes | Modular theme system | Manual CSS |

**Recommendation: Ace Editor**

Ace Editor is the best fit because:
1. **No build step** -- This project has zero JavaScript tooling (no npm, no bundler). Ace works with plain `<script>` tags from CDN or embedded files.
2. **embed.FS compatible** -- The minified Ace files (ace.js ~300KB, mode-json.js ~10KB) can be embedded in the Go binary via `//go:embed static/*`. Alternatively, load from CDN with a `<script>` tag (requires internet access).
3. **JSON mode includes validation** -- Setting `session.setMode("ace/mode/json")` automatically enables a web worker that validates JSON and shows error annotations (red markers in gutter, error messages on hover).
4. **Simple API** -- `editor.getValue()` and `editor.setValue()` map directly to the existing `textarea.value` reads/writes. The syncGuard pattern is preserved with minimal changes.
5. **Acceptable size** -- ~90 KB gzipped is reasonable for a config editor feature. The overall binary is already ~23,400 LOC Go + embedded assets.

**CDN vs embedded decision:**
- **CDN approach** (simpler): Add `<script src="https://cdnjs.cloudflare.com/ajax/libs/ace/1.32.6/ace.js"></script>` and `mode-json.js` to `home.html`. Requires internet access on the machine running the updater. Problematic if the machine has no internet (but the updater itself needs internet for GitHub Releases, so this is likely acceptable).
- **Embedded approach** (self-contained): Download minified ace.js and mode-json.js, place in `internal/web/static/vendor/`, embed via `//go:embed static/vendor/*`. Binary size increases ~300 KB. No internet dependency. Consistent with the single-binary deployment philosophy.
- **Recommendation**: Embed the files. The project strongly values self-contained single-binary deployment (embed.FS for all static assets, self-update mechanism, no external runtime dependencies). Adding a CDN dependency breaks this principle.

## Competitor Feature Analysis

| Feature | Portainer (Docker) | PM2 (Node.js) | Our Approach |
|---------|-------------------|---------------|--------------|
| Delete button protection | Disables delete for running containers, shows tooltip | Stops process before delete | Disable delete button when running, show tooltip |
| Delete confirmation | Modal with container name, force option | Inline confirmation | Already have modal (UI-05), keep as-is |
| Config editing | Text editor for env vars, compose YAML | JSON/process file editor with syntax highlighting | Ace Editor JSON mode with syntax highlighting + validation |
| Create + configure flow | Separate steps (create then configure) | Separate steps | Two-step in same dialog (create then edit config) |
| Config directory management | Volume mapping (Docker concept) | cwd option | Config directory field in create dialog |

## Phase Ordering Recommendation

Based on dependencies, recommended implementation order:

1. **Phase A: DEL-01** -- Delete button disabled. Independent, trivial, immediate safety improvement.
2. **Phase B: EDT-01 + EDT-02** -- JSON editor upgrade. Core infrastructure change. Replace textarea with Ace Editor in existing config dialog. This is the foundation for CFG-01.
3. **Phase C: CFG-02 + CFG-03** -- Config directory backend + frontend. Backend changes needed before CFG-01 can work.
4. **Phase D: CFG-01** -- Integrated create+config dialog. Depends on both Phase B (editor component) and Phase C (config directory field).

## Sources

- Existing codebase analysis: home.js (1462 lines), style.css (932 lines), config_manager.go (301 lines), nanobot_config_handler.go (146 lines)
- [CSS-Tricks: Editable Textarea with Syntax Highlighting](https://css-tricks.com/creating-an-editable-textarea-that-supports-syntax-highlighted-code/) -- the overlay technique for custom syntax highlighting
- [GoMakeThings: Vanilla JS Syntax Highlighting](https://gomakethings.com/how-to-add-syntax-highlighting-to-code-as-a-user-types-in-realtime-with-vanilla-javascript/) -- pure vanilla approach without libraries
- [CodeMirror Reference Manual](https://codemirror.net/docs/ref/) -- CodeMirror 6 modular architecture documentation
- [CodeMirror Forum: Minimal Setup](https://discuss.codemirror.net/t/minimal-setup-because-by-default-v6-is-50kb-compared-to-v5/4514) -- bundle size discussion
- Ace Editor CDN: cdnjs.cloudflare.com/ajax/libs/ace -- current version 1.32.6
- [Reddit: Lightweight vanilla.js JSON editor](https://www.reddit.com/r/javascript/comments/1ltue6z/share_a_lightweight_json_editor/) -- zero-dependency approach
- UX patterns for delete protection: Based on analysis of Portainer, PM2, Docker Desktop, and standard admin panel patterns (training data, MEDIUM confidence)

---
*Feature research for: v0.18.0 Instance Management Enhancements*
*Researched: 2026-04-13*

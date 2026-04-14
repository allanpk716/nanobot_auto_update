# Pitfalls Research

**Domain:** Embedded vanilla JS web UI enhancements (JSON editor, UI state management, config directory management)
**Researched:** 2026-04-13
**Confidence:** MEDIUM (based on codebase analysis, Go stdlib docs, and web search; LOW confidence for CDN-only findings)

## Critical Pitfalls

Mistakes that cause rewrites, broken features, or deployment failures.

---

### Pitfall 1: CDN Dependencies Break Single-Binary Offline Deployment

**What goes wrong:**
Adding a JSON editor library via CDN `<script>` tags (e.g., `cdn.jsdelivr.net/npm/jsoneditor@9.10.3/dist/jsoneditor.min.js`) breaks the core value proposition of `embed.FS` single-binary deployment. When the application runs on a machine without internet access -- a common scenario for Windows services behind firewalls -- the JSON editor fails to load. The rest of the UI works (embedded), but the editor area is blank or throws `ReferenceError: JSONEditor is not defined`.

**Why it happens:**
The project uses `//go:embed static/*` in `internal/web/handler.go` (lines 14-15) to serve all static assets from the Go binary. This makes the application fully self-contained -- no external files needed at runtime. Developers adding a new JS library naturally reach for a CDN `<script>` tag because that is how vanilla JS projects typically work, but it contradicts the architectural constraint of single-binary deployment established in Key Decision "embed.FS + no build tools" (Phase 23).

**How to avoid:**
Download the JS library files (and their CSS), place them in `internal/web/static/vendor/` or similar, and reference them via local paths (`/static/vendor/jsoneditor.min.js`). This keeps everything inside the embed.FS boundary. For version pinning, store the vendor files in the repository -- the binary will always have them.

For Ace Editor specifically (~100KB gzipped for JSON mode), the three files needed are:
- `ace.js` (core)
- `mode-json.js` (JSON syntax highlighting)
- `theme-<name>.js` (chosen theme)

Set `ace.config.set("basePath", "/static/vendor/ace/")` so Ace can find its mode/theme workers from the embedded path.

**Warning signs:**
- HTML contains `<script src="https://cdn...">` or `<link href="https://cdn...">`
- Browser DevTools shows 404 or net::ERR_INTERNET_DISCONNECTED for library files
- "Works on my machine" but fails on the actual deployment Windows server

**Phase to address:**
EDT-01 phase (JSON editor integration) -- this must be decided before writing any editor code.

---

### Pitfall 2: JSON Editor Replaces syncGuard Bidirectional Sync With Conflicting State

**What goes wrong:**
The existing nanobot config editor in `home.js` (lines 663-735) uses a `syncGuard` boolean flag to prevent infinite loops between the structured form (left panel) and the JSON textarea (right panel). If a JSON editor library with its own internal state model (Ace Editor, Monaco, jsoneditor) replaces the plain `<textarea>`, the syncGuard pattern breaks because the library holds its own copy of the content. Editing in the editor triggers the library's `onChange` callback, which tries to sync to the form, which tries to sync back to the editor, causing either lost edits or infinite loops.

**Why it happens:**
The current syncGuard works because both the form and the `<textarea>` share the same DOM element -- setting `textarea.value` directly mutates the single source of truth. A JSON editor library introduces a JavaScript object model: `ace.edit()` returns an editor instance with its own internal state. The library's `getValue()`/`setValue()` methods may trigger or suppress their own `onChange` events depending on implementation, making the syncGuard flag unreliable.

**How to avoid:**
Design a clear "source of truth" architecture before integrating any editor library:
1. **Single source of truth**: The raw JSON string lives in a plain JavaScript variable, NOT in the editor instance.
2. **Editor is a view**: The editor displays and edits this variable, but all mutations flow through a single update function.
3. **Guard with a "programmatic update" flag**: When syncing FROM form TO editor, set a flag. The editor's `onChange` handler checks this flag and skips the reverse sync.
4. **Debounce the editor onChange**: Use `setTimeout(fn, 200)` debounce to prevent rapid-fire sync while the user is typing.

```
User types in form --> update jsonVar --> syncGuard=true --> editor.setValue(jsonVar) --> syncGuard=false
User types in editor --> onChange fires --> if syncGuard: return --> update jsonVar --> update form fields
```

**Warning signs:**
- Edits in the form don't appear in the editor (or vice versa)
- Browser DevTools shows rapid CPU usage / stack overflow on edit
- Values reset after a brief flash

**Phase to address:**
EDT-01 + EDT-02 phase (JSON editor + validation). Must be resolved before implementing bidirectional sync.

---

### Pitfall 3: Delete Button State Drifts From Actual Instance Status

**What goes wrong:**
The delete button is supposed to be disabled when an instance is running and enabled when stopped (DEL-01). The current implementation rebuilds the entire card grid every 5 seconds via `setInterval(loadInstances, 5000)` (home.js line 1108). During the 5-second polling interval, if a user clicks "Stop" on a running instance, the card grid rebuilds after the stop completes (`loadInstances()` in `handleLifecycleAction` line 1082), but the delete button state on the NEW card depends entirely on what the status API returns at that exact moment. If the status API returns stale data (the process hasn't fully exited yet), the delete button stays disabled even though the user just stopped the instance.

**Why it happens:**
The UI has no client-side state model. All state is derived from the server response. The `isRunning` variable on line 842 is a snapshot from the most recent API call. After a stop action, `loadInstances()` fetches fresh data, but the instance process may not have fully terminated by the time the status API responds. The status detection uses PID-based checks (`FindProcessByPID` via `proc.IsRunning()`), which has already caused a regression (see debug file `instance-status-stopped-regression.md` where `proc.Status()` was not implemented on Windows).

**How to avoid:**
1. **Optimistic UI state**: When the user clicks "Stop", immediately mark the instance as "stopping" locally. After the stop API returns 200, set local state to "stopped" regardless of what the next status poll says. Only revert to "running" if a future status poll explicitly says running.
2. **Debounce delete button state checks**: After a lifecycle action, add a short delay (500-1000ms) before the first status poll to let the process fully terminate.
3. **Server-side consistency**: Ensure the DELETE API (line 510 in home.js) returns 409 if the instance is still running, rather than silently proceeding. The frontend currently shows a warning but doesn't enforce the constraint.

**Warning signs:**
- User stops an instance, delete button stays disabled until next 5-second poll
- User sees inconsistent state: "stopped" label but delete is grayed out
- Status indicator and delete button state disagree for 1-5 seconds after actions

**Phase to address:**
DEL-01 phase (delete button state protection). Must be designed alongside EDT-01/02 since both modify the card rendering.

---

### Pitfall 4: Real-Time JSON Validation Performance on Every Keystroke

**What goes wrong:**
EDT-02 requires real-time JSON validation on every keystroke. If using a naive approach (`JSON.parse(textarea.value)` on every `input` event), the validation works fine for small configs but causes noticeable lag for large nanobot configs (50+ lines, deeply nested objects). The UI appears to freeze while parsing.

**Why it happens:**
The current nanobot config has 5 top-level sections (agents, channels, providers, gateway, tools) with nested objects. `JSON.parse()` is O(n) in string length, which is fast, but the problem is the DOM update that follows: after parsing, the code syncs to form fields (5 DOM element updates) and then back to the textarea (stringifying the entire object). This round-trip on every keystroke creates jank.

**How to avoid:**
1. **Debounce validation**: Only validate after 300ms of inactivity, not on every keystroke. This is the single most impactful change.
2. **Separate validation from sync**: Parse JSON for validation (syntax check), but only sync to form fields when validation passes. The current code already does this partially (lines 706-708 catch parse errors), but it still attempts `JSON.parse` on every keystroke.
3. **Editor library validation**: If using Ace Editor with JSON mode, enable the built-in worker-based validation (`editor.session.setMode("ace/mode/json")`) which runs in a Web Worker thread and doesn't block the main thread. This offloads parsing entirely.
4. **For the form-to-JSON sync direction**: The form fields don't need JSON.parse -- they can directly set values on the JSON object without full serialization.

```javascript
// Debounced validation (300ms idle)
var validationTimer = null;
editor.session.on('change', function() {
    clearTimeout(validationTimer);
    validationTimer = setTimeout(function() {
        validateAndSync(editor.getValue());
    }, 300);
});
```

**Warning signs:**
- Typing in the editor feels sluggish
- Browser DevTools Performance tab shows long "Parsing" or "Scripting" blocks on input
- CPU usage spikes while typing in the JSON editor

**Phase to address:**
EDT-02 phase (real-time validation). Must be tested with a realistically large config (100+ lines).

---

### Pitfall 5: Custom Config Directory Path Traversal and ParseConfigPath Conflict

**What goes wrong:**
CFG-02 introduces user-specified config save directories. The existing `ParseConfigPath` function in `config_manager.go` (lines 50-78) uses a regex to extract `--config` from the `start_command` string. If the user specifies a custom directory in the create-instance dialog, there is no field for it in the current API (`POST /api/v1/instance-configs` accepts `name`, `port`, `start_command`, `startup_timeout`, `auto_start` -- no config directory field). Adding a new field means the server must validate that the path is safe (not a path traversal like `../../Windows/System32`) and that it doesn't conflict with the regex-based path resolution.

**Why it happens:**
The current architecture ties config path to `start_command` via regex. A custom config directory is orthogonal to `start_command` -- it's a separate piece of metadata. The conflict arises because `ParseConfigPath` has no awareness of a user-specified override. If both a `--config` flag in `start_command` AND a custom directory field are provided, which takes precedence?

**How to avoid:**
1. **Define precedence clearly**: Custom directory field takes precedence over `--config` in start_command. If the user specifies both, the custom directory wins.
2. **Path validation**: Reject paths that contain `..`, are absolute paths outside the user home directory, or reference Windows reserved device names (NUL, CON, PRN, etc.).
3. **Extend ParseConfigPath or create a new function**: Add a `ParseConfigPathWithOverride(startCommand, instanceName, customDir string)` function that checks the custom directory first, then falls back to the existing regex logic.
4. **Server-side validation**: Add the path validation in the `POST /api/v1/instance-configs` handler (Phase 50's `validateInstanceConfig` function).

**Warning signs:**
- User can create config in arbitrary filesystem locations
- `ParseConfigPath` and custom directory return different paths for the same instance
- Config file written to one path but nanobot reads from another

**Phase to address:**
CFG-02 phase (custom config directory). Must be designed before extending the API schema.

---

### Pitfall 6: os.MkdirAll Race Condition on Concurrent Instance Creation

**What goes wrong:**
CFG-03 requires auto-creating config directories when instances start. The existing `WriteConfig` function in `config_manager.go` (line 176) already calls `os.MkdirAll` before writing. However, if multiple API requests arrive simultaneously to create instances that share a config directory (e.g., two instances with `--config ~/.nanobot-shared/config.json`), both goroutines call `MkdirAll` concurrently. While `os.MkdirAll` is generally safe for concurrent use in Go, the Go standard library has a known race condition (tracked in [golang/go#1736](https://github.com/golang/go/issues/1736)) where the `stat` -> `mkdir` sequence is not atomic. On Windows, this can produce `"file exists"` errors in edge cases.

**Why it happens:**
The `ConfigManager.mu` mutex (line 166-167) serializes writes to the same ConfigManager instance, which protects against this race for writes. But if `CreateDefaultConfig` is called from multiple goroutines through different code paths (e.g., the lazy-creation fallback in `HandleGet` line 74, plus the explicit creation in instance copy), both paths go through `WriteConfig` which is mutex-protected. The real risk is if a new code path calls `os.MkdirAll` directly without going through `WriteConfig`.

**How to avoid:**
1. **Always go through WriteConfig**: Any directory creation must go through `ConfigManager.WriteConfig()` which holds the mutex. Do NOT call `os.MkdirAll` directly from API handlers.
2. **Handle `os.IsExist` gracefully**: Even though MkdirAll usually returns nil for existing directories, add an `os.IsExist(err)` check as defensive coding.
3. **Windows reserved names**: Reject config directory names that match Windows reserved device names (NUL, CON, PRN, AUX, COM1-9, LPT1-9). The Go issue [#24556](https://github.com/golang/go/issues/24556) documents `MkdirAll` failures with these names.

**Warning signs:**
- `"file exists"` error logged during concurrent instance operations
- Config file written but directory creation reports error
- Windows-specific test failures not reproducible on other platforms

**Phase to address:**
CFG-02/CFG-03 phase (config directory creation). Verify that all new directory creation paths go through the mutex-protected `WriteConfig`.

---

## Moderate Pitfalls

### Pitfall 7: Ace Editor Container Height Invisible in Modal

**What goes wrong:**
Ace Editor renders into a container `<div>`, but if the container has no explicit CSS height (e.g., `height: 400px`), the editor is invisible -- it renders with zero height. This is one of the most common Ace Editor integration mistakes. In the existing code, the modal body (`modal-body`) doesn't set an explicit height on the JSON textarea container. When replacing the `<textarea>` with an Ace Editor div, the editor will appear completely blank.

**How to avoid:**
1. Set explicit height on the Ace Editor container: `style="height: 400px; width: 100%"` (or use CSS class).
2. Call `editor.resize()` after the modal becomes visible -- Ace Editor calculates its layout at creation time, and if the modal is hidden (`display: none`), the layout is wrong.
3. Test with the modal open on different screen sizes.

**Warning signs:**
- Config dialog opens but the editor area is blank/zero-height
- Editor content only becomes visible after resizing the browser window

**Phase to address:**
EDT-01 phase.

---

### Pitfall 8: Ace Editor Not Destroyed on Modal Close (Memory Leak)

**What goes wrong:**
Each time the user opens the nanobot config dialog, a new Ace Editor instance is created (`ace.edit("container")`). When the user closes the modal, the editor instance is not destroyed -- it remains in memory holding references to DOM elements, event listeners, and Web Workers. After opening/closing the dialog 10+ times, memory usage grows noticeably.

**Why it happens:**
The current modal system (`showModal` / `closeModal` in home.js) uses `innerHTML = ''` to clear the modal body. This removes the DOM elements but does not call Ace Editor's `editor.destroy()` method. The editor instance still holds references to the removed DOM nodes.

**How to avoid:**
1. Store the editor instance in a variable accessible from the close handler.
2. Call `editor.destroy()` in the `closeModal` function (or in a before-close hook).
3. Alternatively, reuse the same editor instance across dialog opens by updating its value instead of recreating.

```javascript
var activeEditor = null;

function closeModal() {
    if (activeEditor) {
        activeEditor.destroy();
        activeEditor = null;
    }
    document.getElementById('modal-container').style.display = 'none';
}
```

**Phase to address:**
EDT-01 phase.

---

### Pitfall 9: Multiple setInterval Timers Without Coordination

**What goes wrong:**
The existing code already has `setInterval(loadInstances, 5000)` (line 1108) for instance status polling. Adding a JSON editor with its own interval (e.g., validation polling) creates multiple independent timers that can fire simultaneously, causing DOM thrashing. If the user is typing in the JSON editor when the 5-second poll fires and rebuilds the entire card grid, focus may be lost or the modal may be disrupted.

**Why it happens:**
The 5-second poll calls `loadInstances()` which sets `instancesGrid.innerHTML = ''` (line 826). This destroys and recreates all card DOM elements. If the nanobot config dialog is open at that moment, the dialog itself is safe (it's a separate modal element), but if the editor is embedded in the card (unlikely for current design) or if any state depends on card DOM elements, the rebuild causes issues.

**How to avoid:**
1. Do NOT rebuild the entire card grid on every poll. Use diff-based updates: only update cards whose state has changed.
2. Cancel or pause the instance poll timer while a modal is open (since the user is focused on the modal, they don't need live card updates).
3. If keeping the current approach, at minimum ensure the modal is never affected by the poll.

**Warning signs:**
- Modal flickers or loses focus every 5 seconds
- Typing in the editor is interrupted by DOM rebuilds
- DevTools shows rapid DOM mutations from the poll timer

**Phase to address:**
EDT-01 phase (verify isolation). Consider diff-based card updates as a follow-up improvement.

---

### Pitfall 10: Config File Read-After-Write Race

**What goes wrong:**
When creating a new instance (CFG-01 integrates config editing into the create dialog), the flow is: (1) user fills form, (2) click create, (3) POST API creates instance in config.yaml, (4) the on-create callback in ConfigManager creates the nanobot config.json, (5) the UI reloads. If the user immediately clicks "Configure" on the newly created instance card, the GET API might read the config file before the on-create callback has finished writing it, returning a "not found" error that triggers the lazy-creation fallback (creating a SECOND default config, potentially overwriting user-specified values from step 2).

**Why it happens:**
The on-create callback is non-blocking (fires in a goroutine, see `callback injection` pattern in Key Decisions). The `loadInstances()` call after POST success triggers a card rebuild, and the user can interact with the new card before the background goroutine finishes.

**How to avoid:**
1. Make the config file creation synchronous with the instance creation API (include it in the POST handler, not as a callback).
2. OR: If using callbacks, have the POST handler return the config file status in its response so the frontend knows whether the config is ready.
3. OR: The lazy-creation fallback in HandleGet (line 70-84) should check if the file is "currently being created" (using an in-progress map) to avoid double-creation.

**Phase to address:**
CFG-01 phase (config editing in create dialog).

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-Term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| CDN script tags for JSON editor | No repo size increase, no rebuild needed | Breaks offline deployment, version drift, CDN outages | Never -- violates single-binary constraint |
| `setInterval` polling instead of state management | Simple to implement, no architecture change | State drift, race conditions, unnecessary DOM churn | Current acceptable, but should be consolidated before adding more timers |
| String concatenation for HTML templates (current pattern) | No template engine needed | Hard to read, XSS risk if data sneaks in | Acceptable for current scope -- new editor UI should use DOM API exclusively |
| SyncGuard boolean flag | Simple loop prevention | Fragile with editor libraries that have their own event systems | Replace with proper source-of-truth architecture when adding editor |
| Full DOM rebuild every 5 seconds | Simplest possible implementation | Flicker, focus loss, cannot scale to complex cards | Short-term acceptable -- must refactor before adding interactive card features |
| Inline editor styles | Faster to write | Inconsistent theming, hard to maintain | Never -- project already has a CSS architecture in style.css |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Ace Editor with embed.FS | Using CDN basePath, workers fail to load from embedded path | Download all Ace files to `static/vendor/ace/`, set `ace.config.set("basePath", "/static/vendor/ace/")` |
| Ace Editor theme/mode loading | Only including `ace.js`, theme/mode scripts silently fail | Include `ace.js`, `mode-json.js`, and chosen `theme-*.js` in the embedded static files |
| Ace Editor in a modal | Container div has no explicit height, editor is invisible | Set explicit height: `style="height: 400px; width: 100%"`. Call `editor.resize()` after modal visible. |
| Ace Editor container resize | Editor doesn't resize when modal opens | Call `editor.resize()` after `showModal()` completes |
| ConfigManager MkdirAll | New code path calls `os.MkdirAll` directly, bypassing mutex | All directory creation MUST go through `ConfigManager.WriteConfig()` |
| Windows path handling | Using forward slashes or hardcoded `~` without expansion | Always use `filepath.Join()` and `os.UserHomeDir()` (already done in config_manager.go -- maintain this) |
| ParseConfigPath for custom directories | New CFG-02 feature allows user-specified config directory, but ParseConfigPath still regex-scans start_command | Create `ParseConfigPathWithOverride(startCommand, instanceName, customDir string)` that checks custom dir first |
| Ace Editor setValue cursor reset | `editor.setValue(str)` moves cursor to position 0 | Use `editor.setValue(str, -1)` to preserve cursor position |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Full DOM rebuild on every 5-second poll | Cards flicker, scroll position resets, input focus lost | Diff-based update: only update changed cards, preserve DOM elements | 5+ instances with different states changing frequently |
| JSON.stringify + innerHTML on every keystroke | Typing lag, cursor jumps to end of editor | Debounce 300ms, use editor library's internal model | Configs > 50 lines |
| Multiple setInterval without coordination | Timers pile up, redundant API calls | Single requestAnimationFrame/setTimeout loop, or request-based refresh | User performs rapid actions while polls overlap |
| Ace Editor with large JSON (> 500KB) | Editor becomes sluggish, scrolling jank | Enable virtual scrolling, limit config size | Unlikely for nanobot configs (typically < 5KB), but possible with large tool configs |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| innerHTML with user-supplied JSON in editor | XSS if JSON contains `<script>` or event handlers rendered via innerHTML | Use DOM API exclusively for all dynamic content (established pattern). Ace Editor renders its own content safely. |
| Allowing arbitrary JSON keys in nanobot config | User could inject keys that nanobot interprets dangerously (e.g., `exec` commands) | Server-side validation whitelist of allowed top-level keys. Current WriteConfig has no validation. |
| Config file path traversal | User specifies config directory as `../../etc/` or `C:\Windows\System32\` | Validate config path is under user home directory, reject absolute paths outside `~/.nanobot*` |
| API key exposure in JSON editor | API keys visible in the JSON editor textarea even when the form field is password-masked | Low priority (localhost-only management UI). The hybrid editor already has this issue. Document as known limitation. |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Editor appears broken with no height | User sees a blank/empty area and thinks the feature doesn't work | Always set explicit height. Test in the modal before considering it done. |
| JSON validation error appears while user is mid-type | User gets constant red error messages while typing, feels punishing | Debounce validation 300ms. Only show error after user pauses typing. Clear error immediately when fixed. |
| Delete button enables/disables seemingly randomly | User cannot predict when delete is allowed, creates anxiety | Show tooltip explaining why button is disabled ("Instance is running -- stop it first") |
| Auto-save without explicit save button | User makes changes and doesn't realize they need to save, changes lost | Keep explicit save button. Consider unsaved changes warning on modal close. |
| Form and JSON editor show different values | User doesn't know which is authoritative | Current syncGuard approach is correct. Add a visual indicator showing which editor has focus. |

## "Looks Done But Isn't" Checklist

- [ ] **JSON editor renders in modal**: Often missing explicit height on container -- verify editor is visible and correctly sized within the modal. Test by opening the nanobot config dialog and confirming the editor area is not collapsed.
- [ ] **Syntax highlighting works offline**: Often missing theme/mode JS files in embed.FS -- verify by disconnecting network and checking that JSON still has colored syntax.
- [ ] **Bidirectional sync survives rapid edits**: Often the syncGuard breaks when a library editor is involved -- verify by typing quickly in the form, then switching to the editor, then back. Values should stay consistent.
- [ ] **Delete button state correct after stop**: Often the first poll after stop returns stale "running" -- verify by stopping an instance and checking that delete becomes enabled within 2 seconds.
- [ ] **Custom config directory creates correctly**: Often MkdirAll succeeds but the path doesn't match what nanobot expects -- verify by creating an instance with a custom directory, then checking the filesystem.
- [ ] **Config validation catches real errors**: Often only checks `JSON.parse` success -- verify by introducing actual invalid structures (e.g., `"port": "not-a-number"`) and checking that the user gets a helpful message.
- [ ] **Editor resize works on window/modal resize**: Often the editor doesn't resize when the modal is resized or opened on different screen sizes -- call `editor.resize()` on window resize and after modal open.
- [ ] **No memory leaks from editor instances**: Often editor instances are created each time the modal opens but never destroyed -- verify by opening/closing the config dialog 10 times and checking memory usage in DevTools.
- [ ] **Delete button disabled for running instances**: Verify that clicking delete on a running instance shows a clear message (tooltip or warning) instead of just being grayed out.
- [ ] **Save button validates before submit**: Verify that clicking save with invalid JSON shows a clear error message and does NOT send the request to the server.

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| CDN dependency in production | LOW | Download the library files, add to embed.FS, update HTML `<script>` tags to local paths, rebuild binary |
| syncGuard infinite loop | LOW | Add a guard counter (max 10 iterations), or use requestAnimationFrame to break the loop |
| Delete button stuck disabled | LOW | Add a manual "refresh" button, or increase poll frequency after lifecycle actions |
| Editor renders blank | LOW | Check container height, check browser console for library load errors, verify basePath configuration |
| Config directory creation race | MEDIUM | MkdirAll already handles this safely via mutex in WriteConfig. If custom paths bypass it, route through WriteConfig |
| Path traversal exploit | HIGH | Add server-side path validation before allowing custom directories. Audit all file write paths. |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| CDN breaks offline deployment | EDT-01 | Load UI with network disabled, verify JSON editor loads and functions |
| Editor breaks syncGuard | EDT-01 | Open config dialog, edit in form, verify JSON updates. Edit in editor, verify form updates. Repeat 10 times rapidly. |
| Delete button state drift | DEL-01 | Stop a running instance, verify delete enables within 2s. Start it again, verify delete disables. |
| Validation performance | EDT-02 | Type rapidly in a 100-line JSON config, verify no perceptible lag (> 100ms per keystroke) |
| Editor height invisible | EDT-01 | Open config dialog on different screen sizes, verify editor is visible and usable |
| Editor not destroyed on close | EDT-01 | Open/close config dialog 10 times, check DevTools Memory tab for growing heap |
| Custom config directory path traversal | CFG-02 | Try creating instance with path `../../Windows/System32`, verify rejection |
| MkdirAll race on concurrent create | CFG-02 | Send 5 simultaneous POST requests for instances with the same config directory, verify all succeed |
| Ace Editor worker basePath wrong | EDT-01 | Disconnect network, open config dialog, verify syntax highlighting works (not falling back to plain text) |
| Multiple timers without coordination | EDT-01 | Open config dialog, wait 10 seconds, verify no flicker or focus loss from instance poll |
| Config read-after-write race | CFG-01 | Create instance with custom config, immediately click Configure, verify config loads (not "not found") |

## Sources

**Codebase Analysis (HIGH confidence):**
- `internal/web/static/home.js` -- 1462 lines, syncGuard pattern (lines 663-735), loadInstances polling (line 1108), modal system (lines 42-59)
- `internal/nanobot/config_manager.go` -- ConfigManager, ParseConfigPath, WriteConfig with MkdirAll (line 176-177)
- `internal/api/nanobot_config_handler.go` -- HandleGet with lazy-creation fallback (lines 70-84), HandlePut
- `internal/web/handler.go` -- `//go:embed static/*` (line 14-15)
- `.planning/debug/instance-buttons-always-disabled.md` -- Historical debug: config API failure causing disabled buttons
- `.planning/debug/instance-status-stopped-regression.md` -- Historical debug: proc.Status() not implemented on Windows

**Go Standard Library (HIGH confidence):**
- [os.MkdirAll race condition -- golang/go#1736](https://github.com/golang/go/issues/1736) -- Known race in stat->mkdir sequence
- [os.MkdirAll "file exists" on concurrent calls -- golang/go#75114](https://github.com/golang/go/issues/75114) -- Recent fix for concurrent directory creation
- [os.MkdirAll fails with NUL on Windows -- golang/go#24556](https://github.com/golang/go/issues/24556) -- Windows reserved device names

**JSON Editor Libraries (MEDIUM confidence -- web search):**
- [Monaco Editor web workers without bundler -- microsoft/monaco-editor#793](https://github.com/microsoft/monaco-editor/issues/793) -- JSON features require workers, tricky without build tools
- Ace Editor CDN setup (cdnjs.com/ajax/libs/ace/) -- basePath configuration, container height requirements, worker paths
- [7 Best JSON Editor Libraries for React in 2025](https://www.merge-json-files.com/blog/best-json-editor-for-react) -- Size comparison: dedicated JSON editors ~50KB vs Monaco ~2-4MB

**UI State Patterns (MEDIUM confidence -- web search):**
- [Enable/Disable Delete Button with Javascript -- Stack Overflow](https://stackoverflow.com/questions/38414433/enable-disable-delete-button-with-javascript) -- Button state synchronization issues
- UI state race conditions with polling: stale state, concurrent updates, missing cancellation -- general vanilla JS pattern

**LOW confidence findings (web search only, not verified with official docs):**
- Specific Ace Editor version numbers and CDN URL patterns -- verify at integration time
- ESM-only package migration breaking `<script>` tags in 2025 ecosystem -- not relevant if vendoring files
- CDN outage statistics -- not directly relevant since we won't use CDN

---
*Pitfalls research for: nanobot-auto-updater v0.18.0 embedded vanilla JS UI enhancements*
*Researched: 2026-04-13*

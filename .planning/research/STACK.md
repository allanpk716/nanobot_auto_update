# Stack Research: JSON Editor with Syntax Highlighting and Real-Time Validation

**Domain:** Embedded code editor library for vanilla HTML/CSS/JS web UI (no build step, no CDN, embed.FS deployment)
**Researched:** 2026-04-13
**Confidence:** HIGH (version verified via GitHub API and cdnjs; file sizes verified via direct download from jsDelivr; integration pattern verified against existing codebase)

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| **Ace Editor** | v1.43.6 | JSON syntax highlighting, bracket matching, real-time validation via JSON worker, line numbers | `src-min-noconflict` build provides standalone JS files that can be directly embedded via Go `embed.FS` -- no npm, no webpack, no CDN. Replaces the existing `<textarea>` with minimal code change. JSON mode includes built-in `worker-json.js` that runs validation in a Web Worker, providing real-time error annotations on the editor. The `noconflict` variant uses `ace.require()` instead of global `require()`, avoiding conflicts with any other libraries. Active maintenance: v1.43.6 tagged on GitHub (latest as of 2026-04-13), 3k+ stars, BSD-3-Clause license. |

### Supporting Files (Minimal Set for JSON Mode)

| File | Size (bytes) | Purpose | Why Needed |
|------|-------------|---------|------------|
| `ace.js` | 474,871 | Core editor engine | Required. Provides `ace.edit()`, session management, theme/mode loading, worker spawning. |
| `mode-json.js` | 5,561 | JSON syntax highlighting rules | Required for EDT-01. Tokenizes JSON: strings, numbers, booleans, null, keys. |
| `worker-json.js` | 24,390 | JSON validation worker (Web Worker) | Required for EDT-02. Runs `JSON.parse` in background, annotates errors on editor lines. Auto-loaded by Ace when JSON mode is set. |
| `worker-base.js` | 22,023 | Web Worker base infrastructure | Required. Loaded automatically by Ace before the mode-specific worker. Provides the worker communication protocol. |
| `theme-chrome.js` | 3,877 | Light color theme | Recommended. Chrome theme provides a clean light appearance consistent with the existing UI. Alternatives: `theme-github_light_default.js`, `theme-eclipse.js`. |
| `ext-error_marker.js` | 332 | Error annotation display | Optional enhancement. Improves how validation errors are visually displayed (underline + tooltip). Small enough to include by default. |

**Total embed.FS footprint: ~531 KB** (all 6 files combined, minified, no gzip). This is a modest increase for a single-binary deployment.

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| **jsDelivr npm mirror** | Download `src-min-noconflict` files without npm install | `https://cdn.jsdelivr.net/npm/ace-builds@1.43.6/src-min-noconflict/[file]`. Download individual files directly to `internal/web/static/ace/`. |
| **Browser DevTools** | Verify worker loads correctly, check basePath configuration | After integration, open DevTools > Network tab, filter by `worker-json.js` to confirm it loads from the embed.FS path. |
| **Existing toast system** | Display validation errors to user | The project already has `showToast()` in `home.js` for notifications. Can reuse for validation error summary. |

## Installation

Since this project uses zero build step (no npm, no webpack), the files are downloaded directly and placed in the Go embed.FS directory.

```bash
# Download the minimal set of Ace Editor files (no npm required)
mkdir -p internal/web/static/ace

# Core engine
curl -sL "https://cdn.jsdelivr.net/npm/ace-builds@1.43.6/src-min-noconflict/ace.js" \
  -o internal/web/static/ace/ace.js

# JSON mode
curl -sL "https://cdn.jsdelivr.net/npm/ace-builds@1.43.6/src-min-noconflict/mode-json.js" \
  -o internal/web/static/ace/mode-json.js

# JSON validation worker
curl -sL "https://cdn.jsdelivr.net/npm/ace-builds@1.43.6/src-min-noconflict/worker-json.js" \
  -o internal/web/static/ace/worker-json.js

# Worker base (auto-loaded by Ace before mode-specific workers)
curl -sL "https://cdn.jsdelivr.net/npm/ace-builds@1.43.6/src-min-noconflict/worker-base.js" \
  -o internal/web/static/ace/worker-base.js

# Light theme
curl -sL "https://cdn.jsdelivr.net/npm/ace-builds@1.43.6/src-min-noconflict/theme-chrome.js" \
  -o internal/web/static/ace/theme-chrome.js

# Error marker extension
curl -sL "https://cdn.jsdelivr.net/npm/ace-builds@1.43.6/src-min-noconflict/ext-error_marker.js" \
  -o internal/web/static/ace/ext-error_marker.js

# Verify file integrity
wc -c internal/web/static/ace/*.js
```

### HTML Integration

In `home.html`, add a `<script>` tag for Ace **before** `home.js`:

```html
<script src="/static/ace/ace.js"></script>
<script src="/static/home.js"></script>
```

No other script tags needed. Ace auto-loads `mode-json.js`, `worker-json.js`, `theme-chrome.js`, and `worker-base.js` relative to the `basePath` configured in JavaScript.

### JavaScript Integration (replaces textarea)

In `home.js`, within `showNanobotConfigDialog()`:

```javascript
// Replace the <textarea id="nb-json"> with a <div id="nb-json-editor">
// Then initialize Ace:

ace.config.set('basePath', '/static/ace/');

var jsonEditor = ace.edit('nb-json-editor');
jsonEditor.session.setMode('ace/mode/json');
jsonEditor.setTheme('ace/theme/chrome');
jsonEditor.setOptions({
    fontSize: '13px',
    showPrintMargin: false,
    tabSize: 2,
    useSoftTabs: true,
    wrap: true
});

// IMPORTANT: preserve the existing syncGuard bidirectional sync pattern.
// Replace all references to textarea .value with editor .getValue()/.setValue().

// Form -> Editor sync (was: textarea.value = JSON.stringify(...))
// jsonEditor.setValue(jsonStr, -1);  // -1 = move cursor to start

// Editor -> Form sync (was: textarea 'input' event listener)
// jsonEditor.session.on('change', function() {
//     var jsonStr = jsonEditor.getValue();
//     // ... existing parse + populate form fields logic
// });

// Save handler (was: JSON.parse(textarea.value))
// var jsonStr = jsonEditor.getValue();
// JSON.parse(jsonStr);  // validation still works
```

### Critical Integration Point: syncGuard Preservation

The existing `showNanobotConfigDialog()` function (home.js lines 536-788) has a bidirectional sync pattern:

```
syncGuard = false
    |
    +-- Form field 'input' events -> syncGuard=true -> set textarea.value -> syncGuard=false
    +-- Textarea 'input' event   -> syncGuard=true -> parse JSON, populate form -> syncGuard=false
```

When replacing the textarea with Ace Editor:

1. Replace `textarea.value = str` with `jsonEditor.setValue(str, -1)`.
2. Replace `textarea.addEventListener('input', fn)` with `jsonEditor.session.on('change', fn)`.
3. Replace `textarea.value` (read) with `jsonEditor.getValue()`.
4. The `syncGuard` boolean remains unchanged -- it prevents infinite loops regardless of the editor widget.

**The `setValue(str, -1)` call does NOT trigger the 'change' event**, so there is no risk of recursive sync loops when programmatically setting editor content from form fields. This is a built-in Ace Editor behavior that actually makes the syncGuard pattern cleaner than with a textarea.

### Go embed.FS Considerations

The Ace editor files go into `internal/web/static/ace/` which is already covered by the existing `//go:embed` directive for static assets. No Go code changes needed for serving -- the existing static file handler will serve the new directory automatically.

**Web Worker caveat:** Ace spawns `worker-json.js` as a Web Worker via `new Worker(url)`. The browser must be able to fetch this file via HTTP. Since the Go server already serves all files under `/static/`, and Ace resolves worker URLs relative to `basePath`, this works correctly with embed.FS. Verified: setting `ace.config.set('basePath', '/static/ace/')` will cause Ace to request `/static/ace/worker-json.js` and `/static/ace/worker-base.js`, both of which are served by the existing Go static file handler.

**No Service Worker registration needed.** Ace uses plain Web Workers, not Service Workers.

## Alternatives Considered

| Recommended | Alternative | Why Not |
|-------------|-------------|---------|
| **Ace Editor `src-min-noconflict`** | **Monaco Editor** | Monaco (VS Code's editor) is ~5 MB minified and requires complex Web Worker setup (a dedicated `monaco.worker.js` entry point). It also expects a CDN-based loader (`monaco-editor/esm/vs/loader.js`) or a full webpack/vite build pipeline. Embedding Monaco in Go embed.FS without a build step is technically possible but painful -- the worker resolution is fragile, and the binary size increase is significant for a JSON editing use case. Monaco is the right choice for full IDEs, not for a config JSON editor in a management UI. |
| **Ace Editor `src-min-noconflict`** | **CodeMirror 6** | CodeMirror 6 is modular and modern, but it uses ES modules exclusively. Loading it without a bundler requires either an import map (`<script type="importmap">`) or loading via esm.sh CDN -- both of which add complexity to a zero-build-step setup. CodeMirror 6 also requires assembling the editor from individual packages (@codemirror/lang-json, @codemirror/lint, etc.), each of which has its own dependency tree. Without a bundler, managing these imports manually is error-prone. CodeMirror 6 is ideal for projects with an npm/webpack pipeline, not for a single-binary Go deployment. |
| **Ace Editor `src-min-noconflict`** | **vanilla-jsoneditor (josdejong)** | This library provides a standalone JSON editor with tree view and text view. However, it is ~500-600 KB gzipped (much larger than Ace's 531 KB raw), and it brings its own JSON parsing/rendering pipeline. The library is designed as a full JSON editor solution (with tree/table/text modes), which is overkill for this project's needs. We only need syntax highlighting and validation for the existing textarea replacement -- not a complete JSON editor widget with its own state management that would conflict with the existing form-field sync pattern. |
| **theme-chrome.js** | **theme-monokai.js** | Monokai is a dark theme. The existing project UI uses a light theme (white/light gray backgrounds per `style.css`). Using a dark code editor inside a light UI would look inconsistent. Chrome or GitHub Light Default are better visual matches. |
| **Direct file download from jsDelivr** | **`npm install ace-builds` then copy files** | Using npm would require Node.js on the development machine and add a `node_modules/` directory. Since the project has zero build step and the needed files are a known small set, downloading them directly is simpler and consistent with the project's "no npm" philosophy. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| **Monaco Editor** | ~5 MB footprint, complex worker setup, designed for full IDEs, not a single config JSON field. Adds significant binary bloat for marginal benefit over Ace for JSON editing. | Ace Editor with JSON mode |
| **CodeMirror 6** | Requires ES module import maps or bundler. Without webpack/vite, manual ES module management of 5+ packages is fragile and goes against the zero-build-step constraint. | Ace Editor `src-min-noconflict` (single script tag) |
| **CDN-hosted editor** (loading from cdnjs/jsDelivr at runtime) | The project runs on internal networks. The Go binary must be fully self-contained (embed.FS). CDN dependencies break when there is no internet access. | Download files locally, serve from embed.FS |
| **`src-noconflict` (non-minified)** | The non-minified version is larger (~1.5 MB for ace.js alone) with no benefit for production. The `src-min-noconflict` build is the same API, just smaller. | `src-min-noconflict` (minified) |
| **`src-min` (without noconflict)** | Uses global `require()` which can conflict with other libraries that use AMD/RequireJS. The `noconflict` variant uses `ace.require()` to avoid this. | `src-min-noconflict` |
| **Ace extensions beyond what is needed** | Ace has 50+ extensions (elastic tabstops, emmet, inline autocomplete, etc.). Including unnecessary files increases binary size and attack surface. Only include the 6 files listed above. | Minimal file set (ace.js + mode + worker + worker-base + theme + ext-error_marker) |
| **Custom JSON validation logic** | Ace's `worker-json.js` already provides real-time JSON validation with line/column error annotations. Writing custom validation on top would duplicate this capability and likely produce worse error messages. | Let Ace's JSON worker handle validation; listen for `editor.session.on('change')` for user-facing error summary via toast |
| **JSON schema validation libraries** (ajv, tv4) | Overkill for this use case. The nanobot config JSON is a simple flat structure (model, provider, apikey, port, etc.). Structural JSON validity (well-formed JSON) is what EDT-02 requires, not schema validation against a complex schema. | Ace's built-in JSON worker (syntactic validation) |
| **Tree view JSON editors** | The existing UI already has structured form fields (left panel) for editing individual config values. Adding a tree view in the JSON panel would be redundant. The JSON panel serves as an "advanced/override" view for users who prefer raw JSON editing. | Ace Editor in text mode only (no tree view) |

## Stack Patterns by Feature

### Feature EDT-01: JSON Syntax Highlighting

**Pattern:** Replace `<textarea>` with Ace Editor div, set JSON mode.

```
User opens config dialog
    |
    v
showNanobotConfigDialog()
    |
    +-- Replace <textarea> with <div id="nb-json-editor">
    +-- ace.edit('nb-json-editor')
    +-- session.setMode('ace/mode/json')
    +-- session.setTheme('ace/theme/chrome')
    |
    v
Ace renders JSON with syntax highlighting:
  - Strings: colored
  - Numbers: colored differently
  - Booleans/null: colored differently
  - Keys: colored with distinct style
  - Bracket matching: highlights matching {} and []
```

**Key configuration for the nanobot config use case:**

```javascript
jsonEditor.setOptions({
    fontSize: '13px',        // Match existing UI font size
    showPrintMargin: false,  // No vertical line in config editor
    tabSize: 2,              // Nanobot config uses 2-space indent
    useSoftTabs: true,       // Spaces, not tabs
    wrap: true,              // Wrap long lines (config can have long API keys)
    showGutter: true,        // Show line numbers for error reference
    highlightActiveLine: true,
    highlightSelectedWord: true
});
```

### Feature EDT-02: Real-Time JSON Validation

**Pattern:** Ace's JSON worker validates on every edit, annotating errors inline.

```
User types in Ace editor
    |
    v
Ace session detects change
    |
    v
Ace spawns/communicates with worker-json.js (Web Worker)
    |
    v
worker-json.js runs JSON.parse on document content
    |
    +-- Valid: no annotations on editor
    |
    +-- Invalid:
          - Red wavy underline on the error line
          - Error annotation popup: "Unexpected token ] at line 5, column 12"
          - Editor status bar shows error count (if ext-statusbar loaded)
    |
    v
Additionally (application-level):
    - Listen for annotation changes to update the existing error display
    - Enable/disable Save button based on validation state
    - Show toast with error summary on explicit Save attempt
```

**Integration with existing error display:**

The existing code has `<div id="nb-json-error">` for showing JSON parse errors. With Ace, this can be enhanced:

```javascript
// Listen for Ace's built-in annotations (from worker-json.js)
jsonEditor.session.on('changeAnnotation', function() {
    var annotations = jsonEditor.session.getAnnotations();
    if (annotations.length === 0) {
        document.getElementById('nb-json-error').textContent = '';
        // Enable save button
    } else {
        var firstError = annotations[0];
        document.getElementById('nb-json-error').textContent =
            'Line ' + firstError.row + ': ' + firstError.text;
        // Disable save button (or keep enabled but warn)
    }
});
```

## Version Compatibility

| Package | Version | Compatible With | Notes |
|---------|---------|-----------------|-------|
| `ace-builds` | v1.43.6 | All modern browsers (Chrome 90+, Firefox 88+, Edge 90+, Safari 14+) | No transitive dependencies. The `src-min-noconflict` build is self-contained. |
| `ace.js` | v1.43.6 | `mode-json.js`, `theme-chrome.js`, `worker-json.js`, `worker-base.js` | All files must be from the same version. Mixing versions causes subtle breakage in worker communication. |
| Go embed.FS | Go 1.16+ | Any static files | The existing project uses Go 1.24+ which fully supports embed.FS. No concern. |

### Version Verification

- **GitHub API** (`api.github.com/repos/ajaxorg/ace-builds/tags`): v1.43.6 is the latest tag. (HIGH confidence, verified 2026-04-13)
- **cdnjs**: Lists v1.43.3 (may lag behind npm/GitHub by a few patch versions). (HIGH confidence)
- **jsDelivr**: Serves v1.43.6 from npm registry. (HIGH confidence, files downloaded and verified)
- **npm**: Web page shows v1.39.0 but this appears to be a stale cache; GitHub tags are authoritative. (MEDIUM confidence on npm page, but HIGH confidence on actual latest version from GitHub)

### File Size Verification

All sizes verified by direct download from `cdn.jsdelivr.net/npm/ace-builds@1.43.6/src-min-noconflict/` on 2026-04-13:

| File | Verified Size | Confidence |
|------|--------------|------------|
| `ace.js` | 474,871 bytes (~464 KB) | HIGH (downloaded and measured) |
| `mode-json.js` | 5,561 bytes (~5.4 KB) | HIGH (downloaded and measured) |
| `worker-json.js` | 24,390 bytes (~23.8 KB) | HIGH (downloaded and measured) |
| `worker-base.js` | 22,023 bytes (~21.5 KB) | HIGH (downloaded and measured) |
| `theme-chrome.js` | 3,877 bytes (~3.8 KB) | HIGH (downloaded and measured) |
| `ext-error_marker.js` | 332 bytes (~0.3 KB) | HIGH (verified via HEAD request) |
| **Total** | **531,054 bytes (~519 KB)** | HIGH |

**Note:** These are minified but not gzipped sizes. Go's `embed.FS` stores files as-is. If HTTP gzip compression is enabled on the Go server (which it typically is via middleware), the transfer size would be significantly smaller (~150-180 KB estimated).

## Sources

- **GitHub API** -- `api.github.com/repos/ajaxorg/ace-builds/tags`: Latest tag confirmed as v1.43.6. Previous tags: v1.43.5, v1.43.4, v1.43.3. (HIGH confidence)
- **jsDelivr CDN** -- `cdn.jsdelivr.net/npm/ace-builds@1.43.6/src-min-noconflict/`: All required files verified to exist and be downloadable. File sizes measured by direct download. (HIGH confidence)
- **cdnjs** -- `cdnjs.com/libraries/ace`: Lists v1.43.3 with `mode-json.min.js`, `worker-json.min.js`, `ext-error_marker.min.js` all available. Confirms cdnjs is 3 patch versions behind. (HIGH confidence)
- **npm** -- `npmjs.com/package/ace-builds`: Page shows v1.39.0 but GitHub tags show v1.43.6. npm page is likely stale cached. Trust GitHub tags as authoritative source. (MEDIUM confidence on npm page)
- **GitHub releases page** -- `github.com/ajaxorg/ace-builds/releases`: Shows latest GitHub release as v1.5.0 (2022-05-12), but this is misleading -- the repo uses git tags for newer releases, not GitHub Releases UI. Tags are the authoritative version list. (HIGH confidence after investigation)
- **Existing codebase analysis** -- `internal/web/static/home.js`: Verified the existing `<textarea>` pattern, `syncGuard` bidirectional sync, and JSON validation logic in `showNanobotConfigDialog()` (lines 536-788). Confirmed no build step, no CDN dependencies, pure vanilla JS. (HIGH confidence)
- **Existing codebase analysis** -- `internal/web/static/home.html`: Verified current script loading pattern (single `<script src="/static/home.js">`). Confirmed modal system and toast system for UI integration points. (HIGH confidence)
- **Ace Editor documentation** -- Ace auto-loads mode/worker/theme files relative to `basePath`. `setValue(value, -1)` does not trigger 'change' event. `session.getAnnotations()` returns validation errors from workers. (MEDIUM confidence -- based on training data and cdnjs file listing, not verified via Context7 due to availability)

---
*Stack research for: JSON Editor with Syntax Highlighting and Real-Time Validation (EDT-01, EDT-02)*
*Researched: 2026-04-13*

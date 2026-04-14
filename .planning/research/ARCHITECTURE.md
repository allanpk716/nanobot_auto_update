# Architecture: Instance Management Enhancement (v0.18.0)

**Domain:** Go Windows service -- enhancing the embedded Web UI with delete protection, config directory customization, and JSON editor syntax highlighting.
**Researched:** 2026-04-13
**Confidence:** HIGH

## Executive Summary

v0.18.0 adds three enhancement areas to the existing nanobot-auto-updater Web UI: (1) delete button state protection that disables deletion while an instance is running, (2) config directory customization allowing users to specify where nanobot config files are stored, and (3) JSON editor enhancement replacing the plain `<textarea>` with a library that provides syntax highlighting and real-time validation.

All three features integrate into the existing vanilla JS frontend (`home.js`, ~1462 lines) and Go backend API. The key architectural constraint is the `embed.FS` deployment model -- all static files are baked into the single binary, so any new JS library must either be embedded (increasing binary size) or loaded from CDN (breaking offline usage).

The config directory feature (CFG-01/02/03) is the most complex, requiring full-stack changes: new API field, modified config path resolution, and frontend dialog expansion. The delete button protection (DEL-01/02) is the simplest, touching only frontend JS. The JSON editor enhancement (EDT-01/02) sits in between -- a new JS library integration with careful attention to the existing `syncGuard` bidirectional sync mechanism.

---

## Current Architecture (Integration Points)

### Existing Component Inventory

```
internal/web/static/home.js               -- Vanilla JS frontend (~1462 lines)
internal/web/static/home.html              -- HTML shell (57 lines, loads home.js + style.css)
internal/web/static/style.css              -- All styles including .hybrid-editor, .nanobot-json-textarea
internal/web/handler.go                    -- embed.FS declaration: //go:embed static/*
internal/api/server.go                     -- HTTP mux, handler registration, callback wiring
internal/api/instance_config_handler.go    -- Instance CRUD (instanceConfigRequest struct, HandleCreate)
internal/api/nanobot_config_handler.go     -- NanobotConfig GET/PUT (ParseConfigPath-based)
internal/api/instance_lifecycle_handler.go -- Start/Stop with TryLockUpdate guard, 409 on wrong state
internal/nanobot/config_manager.go         -- ParseConfigPath, CreateDefaultConfig, WriteConfig, CloneConfig
internal/config/config.go                  -- Viper-based YAML config (InstanceConfig struct)
```

### Existing Data Flow: Instance Card Rendering

```
loadInstances() -- called every 5s via setInterval
  -> Promise.allSettled([
       fetch('/api/v1/instances/status'),     // no auth, returns [{name, running}]
       fetch('/api/v1/instance-configs', {auth})  // auth required, returns [{name, port, ...}]
     ])
  -> Build statusMap: {name -> running (bool)}
  -> Build configMap: {name -> {name, port, start_command, ...}}
  -> Clear #instances-grid
  -> For each config in configMap:
       createInstanceCard(config, statusMap[config.name])
         -> Creates card DOM with buttons: 编辑/复制/删除/配置
         -> deleteBtn passes isRunning to showDeleteDialog()
         -> statusText updated based on isRunning
```

### Existing Data Flow: Nanobot Config Editing (Hybrid Editor)

```
showNanobotConfigDialog(instanceName)
  -> fetch('/api/v1/instances/{name}/nanobot-config', {auth})
  -> Build modal with two-panel layout:
       LEFT: Form fields (model, provider, apikey, port, telegram token)
       RIGHT: <textarea id="nb-json" class="nanobot-json-textarea">
  -> syncGuard = false (prevents infinite loop)
  -> form <-> JSON bidirectional sync:
       form change -> syncGuard=true -> update JSON -> syncGuard=false
       JSON change -> syncGuard=true -> update form fields -> syncGuard=false
  -> Save button: PUT /api/v1/instances/{name}/nanobot-config
```

### Existing Data Flow: Config Path Resolution

```
NanobotConfigManager.ParseConfigPath(startCommand, instanceName)
  -> regex: --config\s+["']?([^"'\s]+)["']?
  -> If match: expand ~, return absolute path (e.g., C:\Users\alice\custom\config.json)
  -> If no match: fallback to ~/.nanobot/config.json

resolveWorkspace(startCommand, instanceName)
  -> With --config: "~/.nanobot-{instanceName}"
  -> Without --config: "~/.nanobot"
```

### Existing Patterns to Reuse

| Pattern | Where Used | How to Apply for v0.18 |
|---------|-----------|----------------------|
| `isRunning` parameter in `createInstanceCard()` | home.js line 874 | DEL-01: Already passed, just add `deleteBtn.disabled = isRunning` |
| 5-second status polling via `setInterval` | home.js line ~871 | DEL-01: Cards re-rendered with updated isRunning on each poll |
| `buildInstanceFormHtml(options)` | home.js line 117 | CFG-01/02: Add config_dir field to this form template |
| `showModal(title, bodyContent, footerHtml)` | home.js | EDT-01: Replace textarea inside modal body with JSON editor |
| `syncGuard` bidirectional sync | home.js showNanobotConfigDialog | EDT-01: Must be preserved when replacing textarea |
| Callback injection (onCreateInstance) | server.go line 145 | CFG-02: New config_dir field flows through this callback |
| `ParseConfigPath()` | nanobot/config_manager.go | CFG-02: Override with explicit config_dir when provided |
| `text` security (textContent for user data) | Throughout home.js | All new DOM: use textContent, innerHTML only for static templates |

---

## Recommended Architecture

### New vs Modified Files

| File | Type | Feature | Change |
|------|------|---------|--------|
| `internal/web/static/home.js` | Modified | DEL-01, DEL-02, CFG-01, CFG-02, EDT-01, EDT-02 | Delete btn disabled state, create dialog config_dir, JSON editor integration |
| `internal/web/static/home.html` | Modified | EDT-01 | Add `<script>` tag for JSON editor library |
| `internal/web/static/style.css` | Modified | EDT-01, DEL-01 | JSON editor container styles, disabled button styles |
| `internal/api/instance_config_handler.go` | Modified | CFG-02 | Add `ConfigDir` to request/response structs |
| `internal/nanobot/config_manager.go` | Modified | CFG-02, CFG-03 | Accept explicit config_dir parameter |
| `internal/api/server.go` | Modified | CFG-02 | Pass config_dir through callback wiring |
| `internal/api/nanobot_config_handler.go` | Modified | CFG-03 | Use config_dir from instance config |
| `internal/config/config.go` | Modified | CFG-02 | Add `config_dir` to InstanceConfig struct |
| `internal/web/static/jsoneditor.min.js` | **New** | EDT-01 | Embedded JSON editor library (if embed approach chosen) |
| `internal/web/static/jsoneditor.min.css` | **New** | EDT-01 | JSON editor styles (if embed approach chosen) |

### Component Boundaries

```
                         Frontend (embed.FS)
                    +---------------------------+
                    |   home.html (script tags) |
                    |   home.js (vanilla JS)     |
                    |   style.css                |
                    |   [jsoneditor.*] (new)     |
                    +--------+------------------+
                             |
                    +--------+------------------+
                    |   home.js Integration     |
                    |                           |
                    | createInstanceCard()       |--- DEL-01: deleteBtn.disabled = isRunning
                    | showDeleteDialog()         |--- DEL-02: confirmation dialog (already exists)
                    | showCreateDialog()         |--- CFG-01/02: config_dir field + nanobot editor
                    | showNanobotConfigDialog()  |--- EDT-01/02: replace textarea with editor
                    +--------+------------------+
                             |
              fetch (REST API, Bearer Token auth)
                             |
                    +--------+------------------+
                    |   Backend API             |
                    |                           |
                    | POST /api/v1/             |--- CFG-02: instanceConfigRequest.ConfigDir
                    |   instance-configs        |
                    | PUT /api/v1/instances/    |--- CFG-03: use config_dir in path resolution
                    |   {name}/nanobot-config   |
                    | GET /api/v1/instances/    |--- EDT-01: returns config for editor
                    |   {name}/nanobot-config   |
                    +--------+------------------+
                             |
                    +--------+------------------+
                    |   ConfigManager           |
                    |   (nanobot package)       |
                    |                           |
                    | ParseConfigPath()         |--- CFG-02: override with explicit config_dir
                    | CreateDefaultConfig()     |--- CFG-03: MkdirAll + WriteConfig
                    | WriteConfig()             |--- CFG-03: existing MkdirAll handles dir creation
                    +---------------------------+
```

---

## Feature 1: Delete Button State Protection (DEL-01, DEL-02)

### Design

DEL-01 disables the delete button when an instance is running. DEL-02 adds a confirmation dialog before deletion. The confirmation dialog already exists (`showDeleteDialog()` at home.js line 443), so DEL-02 is already satisfied. DEL-01 requires only a conditional `disabled` attribute.

### Data Flow

```
loadInstances() -- every 5 seconds
  -> fetch('/api/v1/instances/status') -> [{name, running}]
  -> createInstanceCard(config, isRunning)
     -> deleteBtn = createElement('button')
     -> [NEW] if (isRunning) {
                   deleteBtn.disabled = true;
                   deleteBtn.title = '实例运行中，无法删除';
                   deleteBtn.classList.add('btn-disabled');
                }
     -> click handler: showDeleteDialog(config.name, isRunning)
        (existing: shows warning if running, proceeds to DELETE)
```

### Implementation Details

The change is a 3-line addition in `createInstanceCard()` around line 1031-1035 of `home.js`:

```javascript
var deleteBtn = document.createElement('button');
deleteBtn.className = 'btn-secondary btn-delete-danger';
deleteBtn.textContent = '删除';
// NEW: DEL-01 - disable delete button when instance is running
if (isRunning) {
    deleteBtn.disabled = true;
    deleteBtn.title = '实例运行中，无法删除';
}
deleteBtn.addEventListener('click', function() { showDeleteDialog(config.name, isRunning); });
```

The 5-second polling already re-renders all cards on each cycle (line ~860: clears `#instances-grid`, rebuilds all cards). When an instance stops, `isRunning` becomes `false`, the card is rebuilt with an enabled delete button. No additional polling or event wiring is needed.

### CSS Addition (DEL-01)

Add a `btn-disabled` style to `style.css` to visually distinguish disabled buttons:

```css
.btn-secondary:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}
```

### Key Decision: No Backend Change Needed

The backend already returns `running: true/false` in the `/api/v1/instances/status` response. The frontend already has `isRunning` in scope. No API modification required. The `isRunning` parameter is also passed to `showDeleteDialog()` which already shows a running warning (line 445-447), so DEL-02's confirmation behavior already exists.

---

## Feature 2: Config Directory Customization (CFG-01, CFG-02, CFG-03)

### Design

Currently, the nanobot config path is derived from `start_command` via regex. CFG-02 allows users to explicitly specify a config directory in the create instance dialog. CFG-03 auto-creates the directory if it does not exist, and reads existing config files from it.

### Data Flow

```
1. Frontend: showCreateDialog()
   -> Add "配置目录" input field to buildInstanceFormHtml()
   -> User fills: name, port, start_command, startup_timeout, auto_start, [config_dir (new)]
   -> POST /api/v1/instance-configs
      body: { name, port, start_command, startup_timeout, auto_start, config_dir }

2. Backend: HandleCreate() (instance_config_handler.go)
   -> instanceConfigRequest.ConfigDir = "C:\Users\alice\nanobot-myinstance"
   -> Store in InstanceConfig (config.go): ic.ConfigDir = request.ConfigDir
   -> Call onCreateInstance callback with config_dir parameter

3. Backend: onCreateInstance callback (server.go line 145)
   -> nanobotConfigManager.CreateDefaultConfig(name, port, startCommand, configDir)

4. Backend: ConfigManager.CreateDefaultConfig() (config_manager.go)
   -> If configDir is provided: configPath = filepath.Join(configDir, "config.json")
   -> Else: existing ParseConfigPath(startCommand, name) logic
   -> workspace = configDir or resolveWorkspace(startCommand, name)
   -> GenerateDefaultConfig(port, workspace)
   -> WriteConfig(configPath, defaultConfig)  // existing MkdirAll handles dir creation

5. Backend: GET /api/v1/instances/{name}/nanobot-config
   -> Read config_dir from InstanceConfig
   -> If configDir: configPath = filepath.Join(configDir, "config.json")
   -> If config exists: return it
   -> If config does not exist: auto-create default (existing lazy-creation fallback)
```

### New Request/Response Fields

```go
// instance_config_handler.go
type instanceConfigRequest struct {
    Name           string `json:"name"`
    Port           uint32 `json:"port"`
    StartCommand   string `json:"start_command"`
    StartupTimeout uint32 `json:"startup_timeout"`
    AutoStart      *bool  `json:"auto_start"`
    ConfigDir      string `json:"config_dir"`  // NEW: optional, explicit config directory
}

type instanceConfigResponse struct {
    Name           string `json:"name"`
    Port           uint32 `json:"port"`
    StartCommand   string `json:"start_command"`
    StartupTimeout uint32 `json:"startup_timeout"`
    AutoStart      *bool  `json:"auto_start"`
    ConfigDir      string `json:"config_dir"`  // NEW: expose to frontend
}
```

### Modified ConfigManager Functions

```go
// config_manager.go - modified signatures

// CreateDefaultConfig now accepts optional configDir parameter.
// If configDir is non-empty, it overrides the path derived from startCommand.
func (cm *ConfigManager) CreateDefaultConfig(
    instanceName string,
    port uint32,
    startCommand string,
    configDir string,  // NEW: explicit config directory, empty = derive from startCommand
) error

// ParseConfigPathWithDir returns the config path given an explicit directory.
// Falls back to ParseConfigPath if configDir is empty.
func (cm *ConfigManager) ParseConfigPathWithDir(
    startCommand string,
    instanceName string,
    configDir string,  // NEW: explicit config directory
) (string, error)
```

### Frontend: Create Dialog Changes

In `buildInstanceFormHtml()` (line 117), add a new form field after the startup_timeout field:

```javascript
// New field in buildInstanceFormHtml:
'<div class="form-group full-width">' +
    '<label for="field-config-dir">配置目录</label>' +
    '<input type="text" id="field-config-dir" value="" placeholder="留空则自动生成 (~/.nanobot-{名称})">' +
    '<span class="field-hint">指定 nanobot 配置文件保存目录。留空使用默认路径。</span>' +
    '<span class="field-error" id="error-config-dir"></span>' +
'</div>'
```

In the submit handler (line 195-201), add:

```javascript
var configDir = document.getElementById('field-config-dir').value.trim();
var body = {
    name: ...,
    port: ...,
    start_command: ...,
    startup_timeout: ...,
    auto_start: ...,
    config_dir: configDir  // NEW: empty string = auto-generate
};
```

### Key Decision: config_dir is Optional

If the user leaves the config directory field empty, the existing `ParseConfigPath()` logic applies (derive from `--config` in `start_command`, fallback to `~/.nanobot/config.json`). This preserves backward compatibility: existing instances and the default create flow are unchanged.

### Config Directory Validation

The backend must validate `config_dir`:
- Must be an absolute path (start with `C:\` or `/`)
- Must not contain `..` (path traversal prevention)
- Must be within the user's home directory or an allowed base path (optional security)
- Empty string is valid (means auto-generate)

---

## Feature 3: JSON Editor Enhancement (EDT-01, EDT-02)

### Design

Replace the plain `<textarea id="nb-json">` in `showNanobotConfigDialog()` with a JSON editor library that provides syntax highlighting and real-time validation. The existing `syncGuard` bidirectional sync must be preserved.

### Library Evaluation

| Criterion | vanilla-jsoneditor v3.8.0 | CodeMirror 6 |
|-----------|---------------------------|-------------|
| Dependencies | Zero | ES modules, multiple packages |
| Bundle size | ~50KB min+gzip | ~100-130KB with JSON lang |
| CDN availability | jsDelivr (single file) | esm.sh (ES module CDN) |
| Embed feasibility | Single .js file, easy to embed | Multiple files, ES module imports |
| Syntax highlighting | Yes (built-in) | Yes (JSON language mode) |
| Real-time validation | Yes (JSON schema) | Via linter addon |
| API simplicity | `createJSONEditor(container, props)` | More boilerplate (View, State, Extensions) |
| Vanilla JS compatible | Yes, no framework required | Yes, framework-agnostic |
| Theme/style isolation | Customizable, manageable | Highly customizable but complex |
| Confidence | MEDIUM (npm page verified) | MEDIUM (esm.sh CDN verified) |

### Recommendation: vanilla-jsoneditor

Use `vanilla-jsoneditor` v3.8.0 because:
1. **Zero dependencies** -- a single JS file, trivially embedded or loaded from CDN
2. **Smaller bundle** -- ~50KB vs ~100-130KB for CodeMirror 6
3. **Simpler API** -- `createJSONEditor()` with a content object, set/get via `editor.set()` / `editor.get()`
4. **embed.FS compatible** -- one `.js` file and one `.css` file to add to `internal/web/static/`

### Delivery Decision: Embed vs CDN

| Approach | Pros | Cons |
|----------|------|------|
| **Embed in binary** | Works offline, single-binary model preserved, no external dependency | Binary size increases ~50KB, library updates require rebuild |
| **CDN (jsDelivr)** | No binary size increase, library always latest, simpler deployment | Requires internet, breaks offline model, CDN outage breaks editor |

**Recommendation: Embed in binary.** The project's core design principle is single-binary deployment with no external dependencies. A 50KB increase is negligible compared to the existing binary size (~10MB+ Go binary). The editor must work in the same offline environments where the service runs.

### Integration Architecture

The JSON editor replaces the `<textarea>` inside `showNanobotConfigDialog()`. The critical integration point is the `syncGuard` bidirectional sync:

```
showNanobotConfigDialog(instanceName)
  -> fetch nanobot config
  -> Build modal HTML (same as current, but <textarea> replaced with <div id="nb-json-editor">)
  -> Initialize vanilla-jsoneditor:
       editor = createJSONEditor(document.getElementById('nb-json-editor'), {
           content: { json: currentConfig },
           onChange: function() {
               if (syncGuard) return;  // PREVENT infinite loop
               syncGuard = true;
               var updatedJson = editor.get();
               var jsonStr = JSON.stringify(updatedJson, null, 2);
               // Update form fields from JSON
               updateFormFieldsFromJson(updatedJson);
               // Update validation error display
               document.getElementById('nb-json-error').textContent = '';
               syncGuard = false;
           }
       })

  -> Form field change handler (existing pattern):
       if (syncGuard) return;
       syncGuard = true;
       var currentJson = editor.get();  // GET from editor instead of textarea
       updateJsonFromFormFields(currentJson);
       editor.set(currentJson);          // SET to editor instead of textarea.value
       syncGuard = false;

  -> Save button:
       var config = editor.get();        // GET from editor
       PUT /api/v1/instances/{name}/nanobot-config
```

### Modified HTML Structure

```javascript
// Replace in showNanobotConfigDialog() bodyHtml:
// BEFORE:
'<textarea id="nb-json" class="nanobot-json-textarea"></textarea>' +
'<div id="nb-json-error" class="json-error"></div>'

// AFTER:
'<div id="nb-json-editor" style="height: 100%; min-height: 400px;"></div>' +
'<div id="nb-json-error" class="json-error"></div>'
```

### Validation Strategy (EDT-02)

vanilla-jsoneditor provides built-in JSON validation:
- Invalid JSON is highlighted with error indicators in the editor
- The `onChange` callback can catch parse errors: wrap `editor.get()` in try/catch
- Display error message in the existing `#nb-json-error` div

```javascript
onChange: function() {
    if (syncGuard) return;
    syncGuard = true;
    try {
        var updatedJson = editor.get();
        document.getElementById('nb-json-error').textContent = '';
        updateFormFieldsFromJson(updatedJson);
    } catch (e) {
        document.getElementById('nb-json-error').textContent =
            'JSON 语法错误: ' + e.message;
    }
    syncGuard = false;
}
```

### CSS Integration

The existing `.hybrid-editor-right` and `.nanobot-json-textarea` styles need adjustment:
- Remove `.nanobot-json-textarea` styles (textarea is gone)
- Add container styles for `#nb-json-editor`:
  ```css
  #nb-json-editor {
      height: 100%;
      min-height: 400px;
      border: 1px solid var(--border-color);
      border-radius: 4px;
      overflow: hidden;
  }
  /* Override vanilla-jsoneditor default height if needed */
  #nb-json-editor .cm-editor {
      height: 100% !important;
  }
  ```

### Editor Lifecycle: Create and Destroy

The modal system creates/destroys the editor instance with the modal:
- **Create**: After modal HTML is injected into DOM, call `createJSONEditor()`
- **Destroy**: In `closeModal()` or when navigating away, call `editor.destroy()` to prevent memory leaks

```javascript
var currentJsonEditor = null;

function showNanobotConfigDialog(instanceName) {
    // ... existing code ...
    showModal(title, bodyHtml, footerHtml);
    // Create editor AFTER modal is in DOM
    currentJsonEditor = createJSONEditor(
        document.getElementById('nb-json-editor'), { ... }
    );
}

function closeModal() {
    if (currentJsonEditor) {
        currentJsonEditor.destroy();
        currentJsonEditor = null;
    }
    // ... existing closeModal code ...
}
```

---

## Patterns to Follow

### Pattern 1: syncGuard Bidirectional Sync Preservation

**What:** A boolean flag that prevents form-to-editor and editor-to-form updates from creating an infinite loop.
**When:** Any bidirectional data binding between form inputs and a JSON editor.
**Why:** The `onChange` callback fires when the editor content changes (programmatically or by user). Without the guard, setting editor content from a form change triggers `onChange`, which updates the form, which triggers form change, which sets the editor, ad infinitum.

```javascript
var syncGuard = false;

// Editor -> Form
editor.onChange(function() {
    if (syncGuard) return;
    syncGuard = true;
    try {
        var json = editor.get();
        updateFormFieldsFromJson(json);
    } catch (e) { /* show error */ }
    syncGuard = false;
});

// Form -> Editor
formInput.addEventListener('input', function() {
    if (syncGuard) return;
    syncGuard = true;
    var json = editor.get();
    json.agents.defaults.model = this.value;
    editor.set(json);
    syncGuard = false;
});
```

### Pattern 2: Card Re-rendering with Polling (No Incremental Updates)

**What:** On each 5-second poll, the entire instance grid is cleared and rebuilt from scratch.
**When:** Adding button state that depends on server-side status (like `isRunning`).
**Why:** The existing pattern is full re-render. Adding a disabled attribute is trivial because the card is recreated from scratch with the latest `isRunning` value. No need for incremental DOM diffing or state management.

```javascript
// Existing pattern (preserved):
grid.innerHTML = '';  // Clear all cards
configMap.forEach(function(config) {
    var isRunning = statusMap[config.name] || false;
    var card = createInstanceCard(config, isRunning);  // Rebuild with current state
    grid.appendChild(card);
});
```

### Pattern 3: Optional Override of Existing Resolution

**What:** When a user provides an explicit value (config_dir), it takes precedence over the derived value (from start_command regex).
**When:** Adding user-specified overrides to auto-derived values.
**Why:** The existing `ParseConfigPath()` logic is valuable as a default. The new feature adds an explicit override without breaking the default behavior.

```go
func (cm *ConfigManager) CreateDefaultConfig(name string, port uint32, startCommand string, configDir string) error {
    var configPath string
    var workspace string

    if configDir != "" {
        // User-specified directory takes precedence
        configPath = filepath.Join(configDir, "config.json")
        workspace = configDir  // Use the directory itself as workspace
    } else {
        // Existing auto-derive logic
        var err error
        configPath, err = ParseConfigPath(startCommand, name)
        if err != nil { return err }
        workspace = resolveWorkspace(startCommand, name)
    }

    defaultConfig := GenerateDefaultConfig(port, workspace)
    return cm.WriteConfig(configPath, defaultConfig)
}
```

### Pattern 4: Editor Instance Lifecycle Tied to Modal

**What:** Create the JSON editor when the modal opens, destroy it when the modal closes.
**When:** Integrating third-party DOM widgets into a modal system.
**Why:** Prevents memory leaks from orphaned editor instances. The editor's internal event listeners and DOM references must be cleaned up.

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Loading JSON Editor from CDN

**What:** Adding a `<script src="https://cdn.jsdelivr.net/npm/vanilla-jsoneditor@3.8.0">` tag to `home.html`.
**Why bad:** The nanobot-auto-updater is designed for single-binary deployment with no external dependencies. CDN loading requires internet access, breaks offline usage, and introduces a third-party availability dependency.
**Instead:** Download the library files (`vanilla-jsoneditor.umd.js`, `style.css`), place them in `internal/web/static/`, and reference via `/static/vanilla-jsoneditor.umd.js`. The `embed.FS` directive (`//go:embed static/*`) automatically includes them in the binary.

### Anti-Pattern 2: Incremental DOM Updates for Button State

**What:** Selecting existing delete buttons by query selector and toggling their `disabled` attribute on each poll.
**Why bad:** The existing architecture does full card re-rendering on each poll. Adding incremental update logic alongside it creates two conflicting DOM manipulation strategies -- one clears and rebuilds, the other patches individual elements. This leads to race conditions and stale state.
**Instead:** Set `deleteBtn.disabled = isRunning` during card creation in `createInstanceCard()`. The full re-render on each poll naturally reflects the latest state.

### Anti-Pattern 3: Bypassing syncGuard in Editor Integration

**What:** Setting editor content without checking/guarding the `syncGuard` flag.
**Why bad:** The `syncGuard` is the only mechanism preventing infinite update loops between form and editor. If any code path sets editor content without guarding, it will trigger `onChange`, which updates the form, which triggers form change handlers, which set the editor again.
**Instead:** Every code path that calls `editor.set()` must be wrapped in `syncGuard` acquire/release. Document this invariant clearly in a code comment.

### Anti-Pattern 4: Replacing ParseConfigPath Entirely

**What:** Modifying `ParseConfigPath()` to accept a `configDir` parameter, breaking the existing signature.
**Why bad:** `ParseConfigPath()` is a pure function used in multiple places (config_manager.go, nanobot_config_handler.go). Changing its signature requires updating all callers. The function is well-tested and stable.
**Instead:** Add a new function `ParseConfigPathWithDir(startCommand, name, configDir)` that delegates to `ParseConfigPath()` when `configDir` is empty. Keep the original function unchanged.

---

## Suggested Build Order

```
Phase 1: Delete Button Protection (LOW complexity, frontend-only)
  Files:
    internal/web/static/home.js     -- add deleteBtn.disabled in createInstanceCard()
    internal/web/static/style.css   -- add :disabled button style
  Test: manual (create instance, start it, verify delete button disabled)
  Dependencies: NONE
  Rationale: 3-line change, no API modification, no new files.
             Lowest risk, highest confidence. Validates that the 5-second
             polling re-render correctly reflects state changes.

Phase 2: Config Directory Backend (MEDIUM complexity, Go-only)
  Files:
    internal/config/config.go                  -- add ConfigDir to InstanceConfig
    internal/api/instance_config_handler.go    -- add ConfigDir to request/response structs
    internal/nanobot/config_manager.go         -- add ParseConfigPathWithDir(), modify CreateDefaultConfig()
    internal/api/nanobot_config_handler.go     -- use config_dir from instance config
    internal/api/server.go                     -- pass config_dir through callback
  Test: unit tests for ParseConfigPathWithDir, CreateDefaultConfig with config_dir
  Dependencies: NONE (backend changes are independent)
  Rationale: Full-stack changes but backend-first isolates the Go logic.
             Can be tested with curl/httpie before any frontend changes.
             The callback chain (server.go -> ConfigManager) must be verified.

Phase 3: Config Directory Frontend (LOW complexity, frontend-only)
  Files:
    internal/web/static/home.js     -- add config_dir field to create dialog, submit handler
  Test: manual (create instance with config_dir, verify directory created)
  Dependencies: Phase 2 (API must accept config_dir)
  Rationale: Simple form field addition. Depends on Phase 2 API being available.
             Can use the existing buildInstanceFormHtml() pattern.

Phase 4: JSON Editor Integration (MEDIUM complexity, frontend-only)
  Files:
    internal/web/static/vanilla-jsoneditor.umd.js  -- new: embedded library
    internal/web/static/vanilla-jsoneditor.css      -- new: library styles
    internal/web/static/home.html                   -- add <script> and <link> tags
    internal/web/static/style.css                   -- editor container styles
    internal/web/static/home.js                     -- replace textarea with editor, update syncGuard
  Test: manual (open nanobot config dialog, verify syntax highlighting and validation)
  Dependencies: NONE (independent of DEL and CFG features)
  Rationale: Can be built in parallel with Phases 1-3.
             Most complex frontend change due to syncGuard preservation.
             Binary size increase (~50KB) should be verified after embedding.
```

### Phase Ordering Rationale

1. **Phase 1 (DEL) first** because it is the simplest possible change. Three lines of JavaScript, zero API changes. It builds confidence in the polling re-render mechanism and delivers visible value immediately.

2. **Phase 2 (CFG backend) second** because the config directory feature requires full-stack changes. Building the backend first allows isolated testing via API calls before integrating with the frontend.

3. **Phase 3 (CFG frontend) third** because it depends on Phase 2's API. The frontend change is a simple form field addition, straightforward once the API is ready.

4. **Phase 4 (EDT) can be built in parallel** with Phases 1-3 because it is completely independent -- it only touches the nanobot config dialog's textarea replacement. However, it is ordered last because it is the most complex frontend integration and benefits from having the other features tested first.

### Phase Complexity Assessment

| Phase | New Files | Modified Files | API Changes | Risk |
|-------|-----------|---------------|-------------|------|
| 1: DEL | 0 | 2 (home.js, style.css) | 0 | LOW |
| 2: CFG backend | 0 | 5 (Go files) | 1 (new field) | MEDIUM |
| 3: CFG frontend | 0 | 1 (home.js) | 0 | LOW |
| 4: EDT | 2 (JS + CSS) | 3 (home.html, style.css, home.js) | 0 | MEDIUM |

---

## embed.FS Compatibility

All static files in `internal/web/static/` are embedded into the Go binary via:

```go
// internal/web/handler.go line 15
//go:embed static/*
var staticFiles embed.FS
```

**Implications for EDT-01:**
- New library files must be placed in `internal/web/static/` before build
- The `//go:embed static/*` glob automatically picks up new files
- No build system changes needed
- Binary size increases by the library file size (~50KB for vanilla-jsoneditor min)
- Library updates require rebuilding and redeploying the binary

**No implications for DEL-01 or CFG-01/02/03** -- these features only modify existing files.

---

## Scalability Considerations

| Concern | Current State | After v0.18 |
|---------|--------------|-------------|
| Binary size | ~10MB (Go binary) | ~10.05MB (+50KB for embedded JSON editor) |
| Memory per page load | ~1-2MB (DOM + JS) | ~1.05-2.1MB (JSON editor adds ~50KB) |
| Polling frequency | 5 seconds | 5 seconds (unchanged) |
| API response size | ~200 bytes per instance | ~250 bytes (adds config_dir field) |

All changes are within the "no impact" threshold for a single-user Windows service tool.

---

## Sources

- Code review of `internal/web/static/home.js` -- createInstanceCard() (line 874-1046), showNanobotConfigDialog() (line 536-788), showCreateDialog() (line 166-240), loadInstances() (line 791-871), buildInstanceFormHtml() (line 117-149) (HIGH confidence -- direct source)
- Code review of `internal/web/handler.go` -- embed.FS declaration (line 15) (HIGH confidence -- direct source)
- Code review of `internal/api/instance_config_handler.go` -- instanceConfigRequest struct (line 19-25), HandleCreate() (line 269-311) (HIGH confidence -- direct source)
- Code review of `internal/nanobot/config_manager.go` -- ParseConfigPath() (line 50-78), CreateDefaultConfig() (line 193-216), WriteConfig() (line 166-187) (HIGH confidence -- direct source)
- Code review of `internal/api/server.go` -- callback wiring (line 145-153), route registration (HIGH confidence -- direct source)
- Code review of `internal/api/nanobot_config_handler.go` -- HandleGet() (line 46-98), HandlePut() (line 104-145) (HIGH confidence -- direct source)
- Code review of `internal/web/static/home.html` -- script/style loading (HIGH confidence -- direct source)
- vanilla-jsoneditor npm page and jsDelivr CDN -- library capabilities, bundle size, API (MEDIUM confidence -- web-verified)
- CodeMirror 6 documentation and esm.sh CDN -- library capabilities, bundle size (MEDIUM confidence -- web-verified)
- `.planning/PROJECT.md` -- v0.18.0 milestone requirements, existing architecture decisions (HIGH confidence -- project documentation)

---

*Architecture research for: v0.18.0 Instance Management Enhancement*
*Researched: 2026-04-13*

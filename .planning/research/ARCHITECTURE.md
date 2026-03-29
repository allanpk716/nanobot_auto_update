# Architecture Patterns: Self-Update Integration (v0.8)

**Domain:** Self-updating Go Windows service (nanobot-auto-updater)
**Researched:** 2026-03-29
**Overall confidence:** HIGH

## Executive Summary

The self-update feature requires adding one new internal package (`internal/selfupdate/`) and modifying four existing files (`main.go`, `server.go`, `help.go`, `config.go`). The `creativeprojects/go-selfupdate` library handles the core binary replacement with built-in rollback support. The integration follows the same dependency-injection pattern already used by TriggerHandler and UpdateLogger: create the component in `main.go`, inject into the API handler, test via interface mocking.

The most significant architectural decision is the restart-after-update strategy. Since this is a long-running Windows service (not a CLI tool), the update handler must respond to the client first, then initiate graceful shutdown. A self-spawned child process starts the new binary after the old one exits. A separate CI/CD pipeline (GitHub Actions + GoReleaser) handles the build-and-release side, producing correctly-named release assets that `go-selfupdate` can discover.

---

## Current Architecture (Integration Points)

### Existing Component Inventory

```
cmd/nanobot-auto-updater/main.go     -- Entry point, wires all components
internal/api/server.go               -- HTTP mux, handler registration
internal/api/trigger.go              -- POST /api/v1/trigger-update handler
internal/api/query.go                -- GET /api/v1/update-logs handler
internal/api/help.go                 -- GET /api/v1/help handler
internal/api/auth.go                 -- Bearer token middleware
internal/config/config.go            -- Main config struct + Load()
internal/config/api.go               -- APIConfig struct
internal/notifier/notifier.go        -- Pushover notification sender
internal/updater/updater.go          -- nanobot updater (uv tool install)
internal/updatelog/updatelog.go      -- JSONL log persistence
```

### Existing Dependency Injection Pattern

The project follows a consistent wiring pattern in `main.go`:

```
main.go creates:
  -> logger (slog.Logger)
  -> config (*config.Config) via config.Load()
  -> updateLogger (*updatelog.UpdateLogger) via updatelog.NewUpdateLogger()
  -> instanceManager (*instance.InstanceManager) via instance.NewInstanceManager()
  -> notif (*notifier.Notifier) via notifier.NewWithConfig()
  -> apiServer (*api.Server) via api.NewServer(cfg, im, cfg, Version, logger, updateLogger, notif)
```

All components receive `*slog.Logger` as the first argument. Handlers receive dependencies through constructor injection. Testing uses interface-based mocking (e.g., `TriggerUpdater` interface, `Notifier` interface defined locally in `trigger.go`).

### Key Existing Patterns to Reuse

| Pattern | Where Used | How to Apply |
|---------|-----------|--------------|
| Interface in handler file | `trigger.go` defines `TriggerUpdater`, `Notifier` | Define `SelfUpdateChecker`/`SelfUpdateExecutor` in handler file |
| Nil-safe component | `healthMonitor`, `updateLogger` checked for nil | Check `selfUpdater != nil` before registering routes |
| Constructor with `logger.With("source", ...)` | All handlers | Same for SelfUpdateHandler |
| Async notification + panic recovery | TriggerHandler start/complete notifications | Same for self-update start/complete |
| `atomic.Bool` concurrency control | TriggerHandler update-in-progress | Same for self-update-in-progress |
| Bearer token middleware reuse | All auth-protected routes | Wrap self-update routes with `authMiddleware` |

---

## Recommended Architecture for Self-Update

### New Components

| Component | Package | Responsibility |
|-----------|---------|---------------|
| `SelfUpdater` | `internal/selfupdate/` | Check GitHub latest release, download, replace exe, signal restart |
| `SelfUpdateHandler` | `internal/api/` | HTTP handler for check/update endpoints |
| GitHub Actions Workflow | `.github/workflows/release.yml` | Build + publish release on tag push |
| GoReleaser Config | `.goreleaser.yaml` | Build configuration for Windows amd64 binary |

### Modified Components

| Component | Change | Why |
|-----------|--------|-----|
| `main.go` | Create SelfUpdater, pass to api.NewServer; add restart channel to shutdown logic | Wire new component; enable restart-after-update |
| `api/server.go` | Accept SelfUpdater interface, register new routes | Add self-update API endpoints |
| `api/help.go` | Add self-update endpoints to help response | Document new API endpoints |
| `config/config.go` | Add `SelfUpdateConfig` to Config struct | Self-update settings (repo slug, enabled) |
| `go.mod` | Add `creativeprojects/go-selfupdate` dependency | Core self-update library |

### Component Boundaries

```
                              main.go
                                 |
                    +------------+------------+------------+
                    |            |            |            |
              SelfUpdater   apiServer    notif (exist)  restartCh
                    |            |
                    |     +------+------+
                    |     |      |      |
                    |  SelfUpdate  TriggerHandler (exist)
                    |  Handler     QueryHandler (exist)
                    |     |
            +-------+-------+
            |               |
     go-selfupdate     notifier.Notifier
     (external lib)    (update notifications)
```

### Data Flow: Version Check (GET /api/v1/self-update/check)

```
1. Client -> GET /api/v1/self-update/check
2. AuthMiddleware validates Bearer token
3. SelfUpdateCheckHandler.Handle()
   a. Call SelfUpdater.CheckLatest(ctx)
4. SelfUpdater.CheckLatest()
   a. selfupdate.DetectLatest(ctx, repository) -> latest release info
   b. Compare latest.Version() with current Version (from ldflags)
   c. Return CheckResult{CurrentVersion, LatestVersion, UpdateAvailable, ReleaseNotes}
5. Handler returns JSON response
```

### Data Flow: Self-Update (POST /api/v1/self-update)

```
1. Client -> POST /api/v1/self-update
2. AuthMiddleware validates Bearer token
3. SelfUpdateHandler.Handle()
   a. Check update not already in progress (atomic.Bool)
   b. Send start notification (async goroutine + panic recovery)
   c. Call SelfUpdater.CheckAndUpdate(ctx)
4. SelfUpdater.CheckAndUpdate()
   a. selfupdate.DetectLatest(ctx, repository) -> get latest release info
   b. Compare versions -> if no update needed, return status "up_to_date"
   c. If update available:
      i.   Log update start (logger.Info)
      ii.  Call selfupdate.UpdateTo(ctx, release, exePath)
           - Library creates .new file with new binary
           - Library renames current exe to .old (backup)
           - Library renames .new to original name
           - On failure: library rolls back (.old -> original)
      iii. Return UpdateResult{PreviousVersion, NewVersion, Success}
5. Handler receives result:
   a. Send completion notification (async goroutine + panic recovery)
   b. Write JSON response to client
   c. If update succeeded: signal restartCh to trigger graceful shutdown
6. main.go receives restartCh signal:
   a. Run existing shutdown sequence (stop all components)
   b. Self-spawn new process: exec.Command(exePath, os.Args[1:]...).Start()
   c. os.Exit(0)
```

---

## New Internal Package: `internal/selfupdate/`

### File Structure

```
internal/selfupdate/
  selfupdate.go        -- SelfUpdater struct, CheckLatest(), CheckAndUpdate()
  selfupdate_test.go   -- Unit tests with mock source
```

### Core Types

```go
// selfupdate.go

// CheckResult holds the result of a version check
type CheckResult struct {
    CurrentVersion  string
    LatestVersion   string
    UpdateAvailable bool
    ReleaseNotes    string
}

// UpdateResult holds the result of an update attempt
type UpdateResult struct {
    PreviousVersion string
    NewVersion      string
    Success         bool
    Error           error
}

// SelfUpdater handles self-update operations
type SelfUpdater struct {
    logger      *slog.Logger
    repository  selfupdate.Repository
    current     string            // current version (from ldflags)
    exePath     string            // path to current executable
    updating    atomic.Bool       // prevent concurrent updates
    notifier    Notifier          // update notifications (interface)
    shutdownCh  chan struct{}     // signal main goroutine to restart
}

// Notifier interface for dependency injection (matches api.Notifier pattern)
type Notifier interface {
    Notify(title, message string) error
}
```

### Key Design Decisions

**1. Use `creativeprojects/go-selfupdate` directly.**

The library handles: GitHub Release API interaction, platform-specific binary detection (expects `nanobot-auto-updater_windows_amd64.zip`), archive decompression, atomic binary replacement with built-in rollback, and checksum validation. Rolling a custom solution would require reimplementing all of this with the same edge cases.

**2. Separate check and update endpoints.**

`GET /api/v1/self-update/check` is read-only and safe (no side effects). `POST /api/v1/self-update` triggers the actual replacement. This lets callers preview before committing, and matches the RESTful pattern of the existing API.

**3. Shutdown channel pattern for restart signaling.**

The SelfUpdater receives a `chan struct{}` that signals `main.go` to begin graceful shutdown. The main goroutine's `<-sigChan` block expands to also listen on this channel via `select`. This keeps OS-specific signal logic in `main.go` where it belongs, not in the selfupdate package.

```go
// In main.go, replace:
<-sigChan

// With:
select {
case <-sigChan:
    logger.Info("Shutdown signal received")
case <-restartCh:
    logger.Info("Restart signal received (self-update)")
    // Self-spawn before exit
    restartProcess(exePath, os.Args[1:])
}
```

**4. Self-spawn restart strategy.**

After successful binary replacement, before calling `os.Exit`:
```go
func restartProcess(exePath string, args []string) error {
    cmd := exec.Command(exePath, args...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.SysProcAttr = &windows.SysProcAttr{
        HideWindow:    true,
        CreationFlags: windows.CREATE_NO_WINDOW,
    }
    return cmd.Start()
}
```

This uses the existing `windows.SysProcAttr` pattern already present in `internal/updater/updater.go`. The old process exits cleanly, the new process starts with the same arguments.

**5. Backup file management by the library.**

`go-selfupdate/update.Apply()` creates `.old` files. On Windows, it cannot delete `.old` files (running process lock), so it hides them instead. On next startup, the application should check for and clean up `.old` files as a startup task in `main.go`.

---

## New HTTP API Endpoints

### GET /api/v1/self-update/check

```
Auth: Bearer Token (required)
Response: {
  "current_version": "v0.7.0",
  "latest_version": "v0.8.0",
  "update_available": true,
  "release_notes": "## What's Changed\n..."
}
```

### POST /api/v1/self-update

```
Auth: Bearer Token (required)
Response (success): {
  "status": "update_scheduled",
  "previous_version": "v0.7.0",
  "new_version": "v0.8.0",
  "message": "Update downloaded. Server will restart shortly."
}
Response (up to date): {
  "status": "up_to_date",
  "current_version": "v0.7.0"
}
Response (conflict): {
  "status": "update_in_progress",
  "error": "An update is already in progress"
}
```

### Route Registration (in server.go)

```go
// Self-update endpoints (auth-protected, nil-safe)
if selfUpdater != nil {
    checkHandler := NewSelfUpdateCheckHandler(selfUpdater, logger)
    updateHandler := NewSelfUpdateHandler(selfUpdater, logger)
    mux.Handle("GET /api/v1/self-update/check",
        authMiddleware(http.HandlerFunc(checkHandler.Handle)))
    mux.Handle("POST /api/v1/self-update",
        authMiddleware(http.HandlerFunc(updateHandler.Handle)))
}
```

Following the existing nil-safe pattern: if SelfUpdater is nil (self-update disabled in config), routes are not registered.

---

## CI/CD: GitHub Actions + GoReleaser

### Why GoReleaser Over Raw Build Steps

GoReleaser handles: cross-compilation, naming conventions, checksum generation, and release creation automatically. It produces the exact archive naming format `go-selfupdate` requires (`{cmd}_{goos}_{goarch}.zip`). The alternative -- manual build steps in the workflow YAML -- requires more code and is error-prone for asset naming.

### .goreleaser.yaml (minimal)

```yaml
builds:
  - main: ./cmd/nanobot-auto-updater
    binary: nanobot-auto-updater
    goos:
      - windows
    goarch:
      - amd64
    ldflags:
      - -s -w -X main.Version={{.Version}}
    env:
      - CGO_ENABLED=0

archives:
  - format: zip
    name_template: >-
      {{ .ProjectName }}_
      {{- tolower .Os }}_
      {{- .Arch }}

checksum:
  name_template: "checksums.txt"

release:
  github:
    owner: HQGroup
    name: nanobot-auto-updater
```

This produces: `nanobot-auto-updater_windows_amd64.zip` containing `nanobot-auto-updater.exe`, which matches the naming convention `go-selfupdate` expects.

### .github/workflows/release.yml

```yaml
name: Release
on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  release:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### Release Process

```bash
git tag v0.8.0
git push origin v0.8.0
# GitHub Actions automatically builds, packages, creates release with assets
```

---

## Patterns to Follow

### Pattern 1: Interface-Based Handler Injection

**What:** Define a local interface in the handler file for the SelfUpdater dependency.
**When:** Always (matches existing `TriggerUpdater` and `Notifier` patterns in `trigger.go`).

```go
// In api/selfupdate.go
type SelfUpdateChecker interface {
    CheckLatest(ctx context.Context) (*selfupdate.CheckResult, error)
}

type SelfUpdateExecutor interface {
    CheckAndUpdate(ctx context.Context) (*selfupdate.UpdateResult, error)
}
```

### Pattern 2: Nil-Safe Component Registration

**What:** Only register routes if the SelfUpdater component is non-nil.
**When:** When self-update may be disabled via config.
**Why:** Matches existing nil-safe patterns (healthMonitor, updateLogger).

### Pattern 3: Async Notification with Panic Recovery

**What:** Send update start/completion notifications in a goroutine with defer/recover.
**When:** After triggering update and after update completes.
**Why:** Matches existing TriggerHandler pattern exactly -- notification failure must not block the update flow.

### Pattern 4: Atomic Bool for Concurrency Control

**What:** Use `sync/atomic.Bool` to prevent concurrent self-updates.
**When:** At the start of the update handler.
**Why:** Self-update is a single-instance operation. Matches existing `atomic.Bool` pattern in `instance/manager.go`.

### Pattern 5: Context-Aware Logging

**What:** Pre-inject component name via `logger.With("source", "selfupdate")`.
**When:** In every constructor.
**Why:** All log output includes the source automatically. Matches existing pattern.

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Using inconshreveable/go-update Directly

**What:** Using the original `go-update` library instead of `creativeprojects/go-selfupdate`.
**Why bad:** `go-update` is unmaintained (last update 2016). It only handles binary replacement, not GitHub Release discovery, asset selection, or decompression.
**Instead:** Use `creativeprojects/go-selfupdate` which wraps `go-update` and adds GitHub Release API integration, asset matching, decompression, and proper naming conventions.

### Anti-Pattern 2: Downloading exe via Raw HTTP

**What:** Implementing your own HTTP download + file write for the update.
**Why bad:** Misses edge cases (atomic replacement, Windows file locking, checksum validation, archive decompression, rollback on failure).
**Instead:** Let `selfupdate.UpdateTo()` handle download + decompression + replacement with rollback.

### Anti-Pattern 3: Blocking the HTTP Handler During Restart

**What:** Calling `os.Exit(0)` directly in the handler goroutine.
**Why bad:** The HTTP response never reaches the client. The connection is abruptly terminated.
**Instead:** Send the HTTP response first, then signal shutdown via channel. A separate goroutine in `main.go` initiates the restart sequence.

### Anti-Pattern 4: Mixing CI/CD and Go Code in Same Phase

**What:** Implementing GoReleaser config, GitHub Actions workflow, and Go self-update code all in one phase.
**Why bad:** CI/CD pipeline can be tested independently (push a tag). Self-update Go code is tested with mocks. Mixing them creates a phase that is harder to validate incrementally.
**Instead:** Separate into distinct phases. CI/CD first (can be tested with a dummy tag), then Go code (tested with mocks), then E2E integration.

---

## Configuration Extension

### New Config Section

```yaml
# config.yaml addition
self_update:
  enabled: true                                # Enable/disable self-update API
  repository: "HQGroup/nanobot-auto-updater"   # GitHub owner/repo slug
```

### Config Struct Extension

```go
// In config/config.go or new file config/selfupdate.go

type SelfUpdateConfig struct {
    Enabled    bool   `yaml:"enabled" mapstructure:"enabled"`
    Repository string `yaml:"repository" mapstructure:"repository"`
}

// Add to Config struct
type Config struct {
    Instances  []InstanceConfig   `yaml:"instances" mapstructure:"instances"`
    Pushover   PushoverConfig     `yaml:"pushover" mapstructure:"pushover"`
    API        APIConfig          `yaml:"api" mapstructure:"api"`
    Monitor    MonitorConfig      `yaml:"monitor" mapstructure:"monitor"`
    HealthCheck HealthCheckConfig `yaml:"health_check" mapstructure:"health_check"`
    SelfUpdate SelfUpdateConfig   `yaml:"self_update" mapstructure:"self_update"` // NEW
}
```

Defaults in `defaults()`: `enabled: true`, `repository: "HQGroup/nanobot-auto-updater"`.

---

## Restart Strategy Detail

Three options for restart-after-update, in order of preference:

### Option A: Self-Spawn (Recommended)

After graceful shutdown completes, spawn the new binary:
```go
cmd := exec.Command(exePath, os.Args[1:]...)
cmd.SysProcAttr = &windows.SysProcAttr{
    HideWindow:    true,
    CreationFlags: windows.CREATE_NO_WINDOW,
}
cmd.Start()
os.Exit(0)
```

**Pros:** No external dependency. Uses existing `windows.SysProcAttr` pattern.
**Cons:** Brief overlap where old and new process coexist. Old process must fully exit before new binds to the same port.

**Mitigation for port conflict:** Add retry logic in `main.go` for HTTP server port binding. If port is in use (old process still shutting down), retry with backoff (100ms, 200ms, 400ms) up to 3 attempts. This is a startup concern, not a self-update concern -- it just happens to be triggered by self-update.

### Option B: External Process Manager

If the application runs under NSSM or Windows Service, the process manager restarts automatically after `os.Exit(0)`.

**Pros:** Clean. No process management code.
**Cons:** Requires external setup. Not portable.

### Option C: Delayed Restart Script

Write a temporary batch script that waits for the old process to exit, then starts the new exe.

**Pros:** Guaranteed no port conflict.
**Cons:** Fragile. Requires managing temp script files. Violates project convention of avoiding new scripts.

**Recommendation:** Option A with port-binding retry in `main.go`.

---

## Suggested Build Order (Phase Dependencies)

```
Phase 1: CI/CD Pipeline (NO Go code changes)
  .goreleaser.yaml
  .github/workflows/release.yml
  Test: push v0.7.1-test tag, verify release appears with correct assets
  Dependencies: NONE
  Rationale: Establish the release pipeline first so there is something
             to update from. Independent of all Go code changes.

Phase 2: SelfUpdater Core Component + Config
  internal/selfupdate/selfupdate.go
  internal/selfupdate/selfupdate_test.go
  internal/config/selfupdate.go (or extend config.go)
  Test: unit tests with mock source
  Dependencies: go.mod update (add go-selfupdate dependency)
  Rationale: Core logic first, then wire into existing system.
             Config can be combined since it is small.

Phase 3: HTTP API Integration
  internal/api/selfupdate.go (new handler file)
  internal/api/server.go (route registration, add selfUpdater param)
  internal/api/help.go (add endpoint docs)
  cmd/nanobot-auto-updater/main.go (wire SelfUpdater)
  Test: handler unit tests with mock SelfUpdater
  Dependencies: Phase 2
  Rationale: Wire everything together. Follows the exact pattern of
             how TriggerHandler was integrated (Phase 28).

Phase 4: Restart Mechanism + Notification Integration
  Extend main.go shutdown logic (add restartCh to select)
  Add self-update notifications (via existing Notifier)
  Add .old file cleanup on startup
  Test: E2E test with test-updater.exe pattern
  Dependencies: Phase 3
  Rationale: Restart mechanism is the riskiest part; isolate it so
             it can be tested thoroughly.

Phase 5: E2E Validation
  Full integration test: trigger self-update via API, verify restart
  Test: manual + automated E2E
  Dependencies: ALL previous phases
  Rationale: Final validation after all pieces are in place.
```

### Phase Ordering Rationale

1. **CI/CD first** because it produces no code changes and can be validated independently. If the pipeline has issues (naming, permissions), fix them before writing any self-update code.
2. **Core component second** because the API handler and main.go wiring both depend on the SelfUpdater type existing.
3. **HTTP API third** because it is pure integration glue following well-established project patterns. Low risk.
4. **Restart mechanism fourth** because it touches the shutdown sequence and is the only part that could break the running service. Isolated for focused testing.
5. **E2E last** because it validates the complete flow end-to-end.

---

## Scalability Considerations

| Concern | Behavior | Notes |
|---------|----------|-------|
| Concurrent self-update requests | `atomic.Bool` prevents; only one at a time | Single-instance operation by nature |
| Release asset download size | ~15MB (Go binary) | One-time download per update |
| Restart downtime | ~2-5 seconds | Process restart + port binding retry |
| GitHub API rate limits | 60 requests/hour (unauthenticated) | Only one check per API call; not a concern |

Self-update is inherently a single-instance operation. There is no "scale" concern -- the updater updates itself, not multiple copies.

---

## Sources

- [creativeprojects/go-selfupdate](https://github.com/creativeprojects/go-selfupdate) -- Primary library, actively maintained, Windows-tested, rollback support (HIGH confidence)
- [go-selfupdate/update package docs](https://pkg.go.dev/github.com/creativeprojects/go-selfupdate/update) -- Apply() step-by-step including Windows .old file handling (HIGH confidence)
- [go-selfupdate package docs](https://pkg.go.dev/github.com/creativeprojects/go-selfupdate) -- Full API reference for DetectLatest, UpdateSelf, UpdateTo (HIGH confidence)
- [softprops/action-gh-release](https://github.com/softprops/action-gh-release) -- GitHub Action for creating releases (HIGH confidence)
- [GoReleaser GitHub Actions docs](https://goreleaser.com/ci/actions/) -- Official workflow integration guide (HIGH confidence)
- [GoReleaser goreleaser-action](https://github.com/goreleaser/goreleaser-action) -- Official GitHub Action (HIGH confidence)
- [Stack Overflow: Self-update while running](https://stackoverflow.com/questions/55247194/how-to-self-update-application-while-running) -- Windows rename-running-exe pattern (MEDIUM confidence)
- [inconshreveable/go-update](https://github.com/inconshreveable/go-update) -- Original library, unmaintained since 2016; NOT recommended, listed for comparison only (LOW confidence)

---

*Architecture research for: v0.8 Self-Update milestone*
*Researched: 2026-03-29*

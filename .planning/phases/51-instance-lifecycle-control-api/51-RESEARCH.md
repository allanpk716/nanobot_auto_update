# Phase 51: Instance Lifecycle Control API - Research

**Researched:** 2026-04-11
**Domain:** Golang HTTP API / Instance Process Lifecycle Management
**Confidence:** HIGH

## Summary

Phase 51 adds two authenticated API endpoints for instance lifecycle control: start and stop. The project already has all the infrastructure needed -- `InstanceLifecycle.StopForUpdate()` and `InstanceLifecycle.StartAfterUpdate()` exist in the instance package, the `AuthMiddleware` with Bearer token validation is in the api package, and a very similar `NewInstanceRestartHandler` already demonstrates the stop-then-start pattern in `internal/web/handler.go`.

The primary work is creating an `InstanceLifecycleHandler` in the `internal/api` package that exposes `POST /api/v1/instances/{name}/start` and `POST /api/v1/instances/{name}/stop` endpoints, wiring them into the server with auth middleware, and writing comprehensive tests. The existing `InstanceManager.GetLifecycle(name)` method returns the `InstanceLifecycle` for a given instance name, providing the direct bridge from HTTP route to lifecycle operations.

**Primary recommendation:** Create a new `InstanceLifecycleHandler` in `internal/api/` (matching the Phase 50 pattern of `InstanceConfigHandler`), reuse existing `AuthMiddleware` and `InstanceManager.GetLifecycle()`, and write handler tests using the same `setupInstanceConfigTest` / `withAuth` helper patterns established in Phase 50.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| LC-01 | User can start a stopped instance via POST /api/v1/instances/{name}/start | `InstanceLifecycle.StartAfterUpdate()` already exists in instance package; handler wraps it with 409 for already-running instances |
| LC-02 | User can stop a running instance via POST /api/v1/instances/{name}/stop | `InstanceLifecycle.StopForUpdate()` already exists in instance package; handler wraps it with 409 for already-stopped instances |
| LC-03 | All CRUD and lifecycle endpoints require Bearer token authentication | `AuthMiddleware` in `internal/api/auth.go` with `subtle.ConstantTimeCompare`; route registration wraps handler with `authMiddleware()` |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| net/http (stdlib) | Go 1.24 | HTTP server, ServeMux routing | Project standard, method+path patterns with `{name}` wildcards [VERIFIED: codebase] |
| stretchr/testify | v1.11.1 | Assertions and test helpers | Project standard for all handler tests [VERIFIED: go.mod] |
| slog (stdlib) | Go 1.24 | Structured logging | Project standard, context-aware logging with `.With()` [VERIFIED: codebase] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| encoding/json (stdlib) | Go 1.24 | JSON request/response serialization | All API handlers |
| context (stdlib) | Go 1.24 | Timeout/cancellation for lifecycle ops | Start/Stop operations with configurable timeouts |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| New handler file | Extend web/handler.go | web/handler.go is for unauthenticated UI endpoints; api/ package is the correct home for authenticated lifecycle endpoints |
| New Interface for mock | Direct testing like Phase 50 | Phase 50 tests use real config + handler, no mocking of InstanceManager. Start/Stop tests need similar approach but can verify handler logic without actual process management |

**Installation:**
No new dependencies required -- all needed packages are already in go.mod.

## Architecture Patterns

### Recommended Project Structure
```
internal/api/
    instance_lifecycle_handler.go       # NEW: HandleStart, HandleStop methods
    instance_lifecycle_handler_test.go   # NEW: Handler tests with auth, 404, 409 cases
    server.go                            # MODIFY: Register 2 new routes
    auth.go                              # EXISTING: Reuse AuthMiddleware
    instance_config_handler.go           # EXISTING: Reference pattern for handler structure
```

### Pattern 1: Handler in api package with method receivers
**What:** HTTP handlers are methods on a struct (e.g., `InstanceLifecycleHandler`) that holds injected dependencies (logger, config reader, instance manager reference).
**When to use:** All authenticated API endpoints.
**Example:**
```go
// Source: [VERIFIED: internal/api/instance_config_handler.go]
type InstanceLifecycleHandler struct {
    im     *instance.InstanceManager
    logger *slog.Logger
}

func NewInstanceLifecycleHandler(im *instance.InstanceManager, logger *slog.Logger) *InstanceLifecycleHandler {
    return &InstanceLifecycleHandler{
        im:     im,
        logger: logger.With("source", "api-instance-lifecycle"),
    }
}

func (h *InstanceLifecycleHandler) HandleStart(w http.ResponseWriter, r *http.Request) {
    name := r.PathValue("name")
    // ...
}
```

### Pattern 2: Route registration with auth middleware
**What:** All CRUD/lifecycle routes wrap the handler with `authMiddleware()`.
**When to use:** Every endpoint that requires authentication.
**Example:**
```go
// Source: [VERIFIED: internal/api/server.go lines 111-116]
mux.Handle("POST /api/v1/instances/{name}/start",
    authMiddleware(http.HandlerFunc(lifecycleHandler.HandleStart)))
mux.Handle("POST /api/v1/instances/{name}/stop",
    authMiddleware(http.HandlerFunc(lifecycleHandler.HandleStop)))
```

### Pattern 3: JSON error responses with RFC 7807 format
**What:** All error responses use `writeJSONError(w, statusCode, errorCode, message)` which returns `{"error": "code", "message": "text"}`.
**When to use:** Every error response from any handler.
**Example:**
```go
// Source: [VERIFIED: internal/api/auth.go lines 110-129]
writeJSONError(w, http.StatusNotFound, "not_found", fmt.Sprintf("Instance %q not found", name))
writeJSONError(w, http.StatusConflict, "conflict", fmt.Sprintf("Instance %q is already running", name))
```

### Pattern 4: Test helpers with httptest.ServeMux
**What:** Tests register routes on a fresh `http.NewServeMux()` with `SetPathValue()` to inject path parameters, then use `mux.ServeHTTP(rec, req)` to execute.
**When to use:** All handler unit tests.
**Example:**
```go
// Source: [VERIFIED: internal/api/instance_config_handler_test.go]
mux := http.NewServeMux()
mux.Handle("POST /api/v1/instances/{name}/start", withAuth(handler.HandleStart, token))
req := authenticatedRequest("POST", "/api/v1/instances/test-existing/start", token, nil)
req.SetPathValue("name", "test-existing")
rec := httptest.NewRecorder()
mux.ServeHTTP(rec, req)
```

### Anti-Patterns to Avoid
- **Direct process management in handlers:** Never call `lifecycle.StopNanobot()` or `lifecycle.StartNanobotWithCapture()` directly from handlers. Always go through `InstanceManager.GetLifecycle()` -> `InstanceLifecycle.StopForUpdate()` / `StartAfterUpdate()`.
- **Non-JSON error responses:** Never use `http.Error()` for error responses in api package handlers. Always use `writeJSONError()` for consistent RFC 7807 format.
- **Registering lifecycle routes without auth:** Every new route must be wrapped with `authMiddleware()` to satisfy LC-03.
- **Using r.Context() for long-running operations:** The HTTP request context is cancelled when the client disconnects. For start/stop operations that may take up to 30s (startup timeout), use a detached context with timeout.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Process start/stop | Custom OS command execution | `InstanceLifecycle.StartAfterUpdate()` / `StopForUpdate()` | Handles PID tracking, log capture, Telegram monitor lifecycle, error wrapping |
| Instance lookup by name | Linear search in handler | `InstanceManager.GetLifecycle(name)` | Returns `InstanceError` with proper context |
| Auth validation | Custom token check | `AuthMiddleware(getToken, logger)` | Constant-time comparison, JSON error format, dynamic token getter |
| JSON error responses | Manual JSON encoding | `writeJSONError(w, status, code, msg)` | Consistent RFC 7807 format, Content-Type header |
| Running state check | Port-based detection | `InstanceLifecycle.IsRunning()` | PID-based detection, accurate for multi-instance |

**Key insight:** The restart handler in `internal/web/handler.go` already demonstrates the exact pattern needed (stop + start with error handling). The lifecycle handler is simpler -- it only does one operation (start OR stop) per request, not both.

## Common Pitfalls

### Pitfall 1: Starting an already-running instance
**What goes wrong:** Calling `StartAfterUpdate()` on an instance with `pid > 0` starts a second process, causing port conflicts and orphaned processes.
**Why it happens:** `StartAfterUpdate()` does not check if the instance is already running before starting.
**How to avoid:** Handler must check `inst.IsRunning()` before calling `StartAfterUpdate()`. Return 409 Conflict if already running.
**Warning signs:** Two processes on the same port, PID leaks.

### Pitfall 2: Stopping an already-stopped instance
**What goes wrong:** `StopForUpdate()` returns nil when `pid == 0` (instance never started), but the user might expect a 409 or a different status.
**Why it happens:** `StopForUpdate()` is designed to be idempotent for update workflows (returns nil when not running).
**How to avoid:** Handler should check `inst.IsRunning()` first. Return 409 Conflict if already stopped, so the user gets clear feedback.
**Warning signs:** Silent "success" when the instance was already stopped confuses users.

### Pitfall 3: Using request context for long operations
**What goes wrong:** Using `r.Context()` for `StartAfterUpdate()` which can take up to 30s (startup_timeout). If the HTTP client disconnects, the context is cancelled and the process may be orphaned.
**Why it happens:** The request context is bound to the HTTP connection lifetime.
**How to avoid:** Create a new `context.WithTimeout(context.Background(), startupTimeout)` for start operations, matching the existing pattern in `NewInstanceRestartHandler` (which does use `r.Context()` -- but this is an existing pattern to be aware of). The stop operation should use the request context with a reasonable timeout.
**Warning signs:** Instance process starts but HTTP response is lost; process becomes orphaned.

### Pitfall 4: Hot-reload recreating instances during lifecycle operations
**What goes wrong:** If config changes while a start/stop is in progress, the hot-reload mechanism (`OnInstancesChange`) may recreate the `InstanceManager` and its instances, invalidating the `InstanceLifecycle` pointer the handler is operating on.
**Why it happens:** Hot-reload triggers full replace (StopAll -> recreate -> StartAll) when instances config changes.
**How to avoid:** This is a pre-existing concern in the restart handler too. For Phase 51, the start/stop endpoints do NOT modify config, so hot-reload is not triggered. No action needed beyond awareness.
**Warning signs:** Nil pointer dereference after config change during operation.

### Pitfall 5: Missing Telegram monitor lifecycle management
**What goes wrong:** Starting an instance should create a Telegram monitor, stopping should cancel it. These are handled inside `StartAfterUpdate()` and `StopForUpdate()` respectively.
**Why it happens:** The handler might be tempted to call lower-level functions that skip monitor management.
**How to avoid:** Always use the high-level `StartAfterUpdate()` and `StopForUpdate()` methods which handle Telegram monitor lifecycle automatically.
**Warning signs:** Telegram monitoring stops working after API-triggered start/stop.

## Code Examples

### InstanceLifecycleHandler with start/stop methods

```go
// internal/api/instance_lifecycle_handler.go
package api

import (
    "context"
    "encoding/json"
    "fmt"
    "log/slog"
    "net/http"
    "time"

    "github.com/HQGroup/nanobot-auto-updater/internal/instance"
)

type InstanceLifecycleHandler struct {
    im     *instance.InstanceManager
    logger *slog.Logger
}

func NewInstanceLifecycleHandler(im *instance.InstanceManager, logger *slog.Logger) *InstanceLifecycleHandler {
    return &InstanceLifecycleHandler{
        im:     im,
        logger: logger.With("source", "api-instance-lifecycle"),
    }
}

func (h *InstanceLifecycleHandler) HandleStart(w http.ResponseWriter, r *http.Request) {
    name := r.PathValue("name")
    if name == "" {
        writeJSONError(w, http.StatusBadRequest, "bad_request", "Instance name required")
        return
    }

    inst, err := h.im.GetLifecycle(name)
    if err != nil {
        writeJSONError(w, http.StatusNotFound, "not_found", fmt.Sprintf("Instance %q not found", name))
        return
    }

    if inst.IsRunning() {
        writeJSONError(w, http.StatusConflict, "conflict", fmt.Sprintf("Instance %q is already running", name))
        return
    }

    // Use detached context to survive client disconnection
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    if err := inst.StartAfterUpdate(ctx); err != nil {
        h.logger.Error("Failed to start instance", "instance", name, "error", err)
        writeJSONError(w, http.StatusInternalServerError, "internal_error",
            fmt.Sprintf("Failed to start instance %q: %v", name, err))
        return
    }

    h.logger.Info("Instance started via API", "instance", name)
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "message": fmt.Sprintf("Instance %q started", name),
        "running": true,
    })
}

func (h *InstanceLifecycleHandler) HandleStop(w http.ResponseWriter, r *http.Request) {
    name := r.PathValue("name")
    if name == "" {
        writeJSONError(w, http.StatusBadRequest, "bad_request", "Instance name required")
        return
    }

    inst, err := h.im.GetLifecycle(name)
    if err != nil {
        writeJSONError(w, http.StatusNotFound, "not_found", fmt.Sprintf("Instance %q not found", name))
        return
    }

    if !inst.IsRunning() {
        writeJSONError(w, http.StatusConflict, "conflict", fmt.Sprintf("Instance %q is not running", name))
        return
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := inst.StopForUpdate(ctx); err != nil {
        h.logger.Error("Failed to stop instance", "instance", name, "error", err)
        writeJSONError(w, http.StatusInternalServerError, "internal_error",
            fmt.Sprintf("Failed to stop instance %q: %v", name, err))
        return
    }

    h.logger.Info("Instance stopped via API", "instance", name)
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "message": fmt.Sprintf("Instance %q stopped", name),
        "running": false,
    })
}
```

### Route registration in server.go

```go
// In NewServer(), after instance config handler registration:
// Instance lifecycle control endpoints (Phase 51: LC-01, LC-02, LC-03)
lifecycleHandler := NewInstanceLifecycleHandler(im, logger)
mux.Handle("POST /api/v1/instances/{name}/start",
    authMiddleware(http.HandlerFunc(lifecycleHandler.HandleStart)))
mux.Handle("POST /api/v1/instances/{name}/stop",
    authMiddleware(http.HandlerFunc(lifecycleHandler.HandleStop)))
```

### Test pattern following Phase 50 conventions

```go
// internal/api/instance_lifecycle_handler_test.go
func TestHandleStart_InstanceNotRunning(t *testing.T) {
    handler, token := setupLifecycleTest(t)
    // Instance "test-existing" is created but not started (pid=0)
    mux := http.NewServeMux()
    mux.Handle("POST /api/v1/instances/{name}/start", withAuth(handler.HandleStart, token))

    req := authenticatedRequest("POST", "/api/v1/instances/test-existing/start", token, nil)
    req.SetPathValue("name", "test-existing")
    rec := httptest.NewRecorder()
    mux.ServeHTTP(rec, req)

    // Will likely be 500 (process start fails in test env) or needs mocking
    // The key test is that the handler logic is correct
}

func TestHandleStart_AlreadyRunning(t *testing.T) {
    // Test that starting a running instance returns 409
}

func TestHandleStop_AlreadyStopped(t *testing.T) {
    // Test that stopping a stopped instance returns 409
}

func TestHandleStart_NotFound(t *testing.T) {
    // Test that non-existent instance returns 404
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Port-based running detection | PID-based running detection (gopsutil) | Phase 21-25 | `IsRunning()` uses PID tracking, not port listening -- more accurate for multi-instance |
| Legacy Manager (port-based) | InstanceLifecycle (PID-based + log capture + Telegram monitor) | Phase 21 | Always use InstanceLifecycle, never legacy Manager |
| Unauthenticated endpoints | Bearer token auth (RFC 6750) | Phase 28 | All mutation endpoints must use AuthMiddleware |

**Deprecated/outdated:**
- `lifecycle.Manager.StartAfterUpdate()`: Returns error "deprecated" -- use `InstanceLifecycle.StartAfterUpdate()` instead [VERIFIED: internal/lifecycle/manager.go line 70]

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | 409 Conflict is the appropriate status for "already running" or "already stopped" | Architecture Patterns | Low -- 409 is standard REST semantics; could also use 200 with status message but 409 is more informative |
| A2 | Start operation should use detached context (not request context) | Common Pitfalls | Medium -- if using r.Context(), client disconnect could cancel the startup; the restart handler currently uses r.Context() which is a pre-existing pattern |
| A3 | Handler should live in `internal/api/` package, not `internal/web/` | Architecture Patterns | Low -- web package is for unauthenticated UI handlers; api package is for authenticated JSON API handlers |

**If this table is empty:** All claims in this research were verified or cited -- no user confirmation needed.

## Open Questions

1. **Should start/stop of an instance that is in the wrong state return 409 or 200?**
   - What we know: The restart handler in web/handler.go does not check running state before stop/start. The Phase 51 success criteria say "start a stopped instance" and "stop a running instance" -- implying the operation should only apply to the correct state.
   - What's unclear: Whether 409 Conflict or 200 OK with a status message is preferred for idempotent operations.
   - Recommendation: Use 409 Conflict -- gives the client clear feedback that the operation was a no-op due to wrong state. This is a common REST pattern (e.g., Kubernetes uses 409 for "already exists" / "conflict").

2. **Should the handler use request context or detached context for lifecycle operations?**
   - What we know: StartAfterUpdate can take up to `startup_timeout` seconds (default 30s). The existing restart handler uses `r.Context()`.
   - What's unclear: Whether client disconnection during a 30-second startup should cancel the operation.
   - Recommendation: Use `context.WithTimeout(context.Background(), timeout)` for start operations (survive client disconnect) and `context.WithTimeout(r.Context(), timeout)` for stop operations (faster, client should wait). This prevents orphaned processes on client disconnect.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go runtime | All code | Yes | go1.24.11 | -- |
| testify | Handler tests | Yes | v1.11.1 | -- |
| net/http test infrastructure | Handler tests | Yes | stdlib | -- |

**Missing dependencies with no fallback:**
None -- all dependencies already available in the project.

**Missing dependencies with fallback:**
None.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + stretchr/testify v1.11.1 |
| Config file | None (standard go test) |
| Quick run command | `go test ./internal/api/... -run TestHandle -v -count=1` |
| Full suite command | `go test ./internal/api/... -v -count=1` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| LC-01 | Start stopped instance via POST /api/v1/instances/{name}/start | unit | `go test ./internal/api/... -run TestHandleStart -v -count=1` | Wave 0 |
| LC-01 | Start returns 409 when instance already running | unit | `go test ./internal/api/... -run TestHandleStart_AlreadyRunning -v -count=1` | Wave 0 |
| LC-01 | Start returns 404 for non-existent instance | unit | `go test ./internal/api/... -run TestHandleStart_NotFound -v -count=1` | Wave 0 |
| LC-02 | Stop running instance via POST /api/v1/instances/{name}/stop | unit | `go test ./internal/api/... -run TestHandleStop -v -count=1` | Wave 0 |
| LC-02 | Stop returns 409 when instance already stopped | unit | `go test ./internal/api/... -run TestHandleStop_AlreadyStopped -v -count=1` | Wave 0 |
| LC-02 | Stop returns 404 for non-existent instance | unit | `go test ./internal/api/... -run TestHandleStop_NotFound -v -count=1` | Wave 0 |
| LC-03 | Start/stop return 401 without Bearer token | unit | `go test ./internal/api/... -run TestLifecycleAuth_Required -v -count=1` | Wave 0 |
| LC-03 | Start/stop return 401 with wrong Bearer token | unit | `go test ./internal/api/... -run TestLifecycleAuth_WrongToken -v -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/api/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/api/instance_lifecycle_handler.go` -- new handler with HandleStart and HandleStop
- [ ] `internal/api/instance_lifecycle_handler_test.go` -- tests for all handler behaviors + auth
- [ ] `internal/api/server.go` -- add 2 route registrations with auth middleware

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | Yes | Bearer token via `AuthMiddleware` with `subtle.ConstantTimeCompare` |
| V3 Session Management | No | Stateless Bearer token, no sessions |
| V4 Access Control | Yes | Auth middleware on all lifecycle routes |
| V5 Input Validation | Yes | Path parameter validation (name non-empty, instance exists) |
| V6 Cryptography | Yes | Constant-time token comparison (timing attack prevention) |

### Known Threat Patterns for Golang HTTP API

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Timing attack on token | Information Disclosure | `subtle.ConstantTimeCompare` in `validateBearerToken()` |
| Process injection via command | Tampering | `StartAfterUpdate` uses `exec.CommandContext` with fixed command from config (not user input) |
| Denial of service (concurrent start) | Denial of Service | `IsRunning()` check before start; PID-based state prevents double-start |

## Sources

### Primary (HIGH confidence)
- [Codebase: internal/api/server.go] - Route registration patterns, ServeMux with method+path wildcards
- [Codebase: internal/api/auth.go] - AuthMiddleware, validateBearerToken, writeJSONError
- [Codebase: internal/api/instance_config_handler.go] - Handler pattern for Phase 50 CRUD
- [Codebase: internal/api/instance_config_handler_test.go] - Test patterns with withAuth, setupIntegrationTest
- [Codebase: internal/instance/lifecycle.go] - StartAfterUpdate, StopForUpdate, IsRunning methods
- [Codebase: internal/instance/manager.go] - GetLifecycle, GetInstanceStatuses methods
- [Codebase: internal/web/handler.go] - NewInstanceRestartHandler (stop+start pattern reference)
- [Codebase: internal/instance/errors.go] - InstanceError structure
- [Codebase: go.mod] - Go 1.24.11, testify v1.11.1

### Secondary (MEDIUM confidence)
- REST API conventions for 409 Conflict status code [ASSUMED: standard HTTP semantics]

### Tertiary (LOW confidence)
None -- all findings verified against codebase.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - No new dependencies; all existing codebase patterns
- Architecture: HIGH - Directly follows Phase 50 handler pattern and existing restart handler pattern
- Pitfalls: HIGH - Identified from code analysis of existing lifecycle methods and restart handler

**Research date:** 2026-04-11
**Valid until:** 2026-05-11 (stable codebase, no external dependencies)

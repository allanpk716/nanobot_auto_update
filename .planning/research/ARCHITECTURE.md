# Architecture Research

**Domain:** HTTP API Service + Monitoring Service Integration
**Researched:** 2026-03-16
**Confidence:** HIGH

## Executive Summary

This research covers the integration of HTTP API and monitoring services with the existing nanobot-auto-updater architecture. The system will evolve from a cron-scheduled tool to a continuously running service with two concurrent components: an HTTP API server for on-demand update triggers, and a background monitoring service for Google connectivity checks. Both services will coordinate with the existing `InstanceManager` for update operations.

## Existing Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     Main Application                         │
│  (cmd/nanobot-auto-updater/main.go)                         │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────┐          ┌──────────────────────┐        │
│  │   Scheduler  │──────>   │  InstanceManager     │        │
│  │  (cron-based)│          │  (internal/instance) │        │
│  └──────────────┘          └──────────┬───────────┘        │
│                                        │                     │
│                             ┌──────────┴──────────┐        │
│                             │                     │        │
│                    ┌────────▼──────┐    ┌────────▼──────┐ │
│                    │ InstanceLife  │    │ InstanceLife  │ │
│                    │ (instance #1) │    │ (instance #2) │ │
│                    └───────────────┘    └───────────────┘ │
├─────────────────────────────────────────────────────────────┤
│  Supporting Services:                                        │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                  │
│  │ Updater  │  │ Notifier │  │ Logging  │                  │
│  │(internal)│  │(internal)│  │(internal)│                  │
│  └──────────┘  └──────────┘  └──────────┘                  │
└─────────────────────────────────────────────────────────────┘
```

## Proposed Architecture (v0.3)

### System Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     Main Application                         │
│  (cmd/nanobot-auto-updater/main.go)                         │
│                                                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │         Context + Signal Handler (Root)                │  │
│  │  - Creates root context with cancellation              │  │
│  │  - Handles SIGINT/SIGTERM                              │  │
│  │  - Coordinates graceful shutdown                       │  │
│  └───────────────────────┬───────────────────────────────┘  │
│                          │                                   │
│         ┌────────────────┼────────────────┐                │
│         │                │                │                │
│  ┌──────▼─────┐   ┌─────▼──────┐  ┌─────▼──────┐        │
│  │ HTTP API   │   │ Monitoring │  │ Instance   │        │
│  │ Server     │   │ Service    │  │ Manager    │        │
│  │ (NEW)      │   │ (NEW)      │  │ (EXISTING) │        │
│  └──────┬─────┘   └─────┬──────┘  └─────┬──────┘        │
│         │               │                │                │
│         │               │                │                │
│         └───────────────┴────────────────┘                │
│                         │                                   │
│                  Shared Coordination                        │
│                  - Update Lock (sync.Mutex)                 │
│                  - InstanceManager                          │
└─────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Implementation |
|-----------|----------------|----------------|
| **HTTP API Server** (NEW) | Exposes `/api/v1/trigger-update` endpoint, authenticates requests via Bearer token | `internal/api/server.go` - `net/http.Server` with custom handlers |
| **Monitoring Service** (NEW) | Checks Google connectivity every 15 min, sends notifications on failure/recovery | `internal/monitor/service.go` - goroutine with `time.Ticker` |
| **Instance Manager** (EXISTING) | Coordinates stop→update→start lifecycle for all nanobot instances | `internal/instance/manager.go` - unchanged |
| **Update Lock** (NEW) | Prevents concurrent updates from API and monitoring triggers | `sync.Mutex` in main or shared coordinator |
| **Config** (MODIFIED) | Adds API port, Bearer token, monitoring interval fields | `internal/config/config.go` - new fields added |
| **Notifier** (EXISTING) | Sends Pushover notifications for failures and recovery | `internal/notifier/notifier.go` - add recovery notification method |

### New Components Detail

#### 1. HTTP API Server (`internal/api/`)

**Structure:**
```
internal/api/
├── server.go          # HTTP server setup and lifecycle
├── handlers.go        # Request handlers (trigger-update, health)
├── middleware.go      # Authentication, logging middleware
└── server_test.go     # Unit tests
```

**Key Responsibilities:**
- Start HTTP server on configured port (default: 8080)
- Validate Bearer token via middleware
- Acquire update lock before triggering update
- Return JSON responses (success/error)
- Graceful shutdown on context cancellation

**Integration Pattern:**
```go
// Server wraps http.Server with lifecycle management
type Server struct {
    httpServer *http.Server
    manager    *instance.Manager
    logger     *slog.Logger
    mu         *sync.Mutex  // Shared update lock
}

func (s *Server) Start(ctx context.Context) error {
    // Start HTTP server in goroutine
    go func() {
        if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
            s.logger.Error("HTTP server error", "error", err)
        }
    }()

    // Wait for context cancellation
    <-ctx.Done()

    // Graceful shutdown with timeout
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    return s.httpServer.Shutdown(shutdownCtx)
}
```

#### 2. Monitoring Service (`internal/monitor/`)

**Structure:**
```
internal/monitor/
├── service.go         # Monitoring goroutine with ticker
├── checker.go         # HTTP connectivity checker
├── state.go           # State tracking (last status, failure count)
└── service_test.go    # Unit tests
```

**Key Responsibilities:**
- Check Google connectivity every 15 minutes (configurable)
- Track consecutive failures (state machine)
- Trigger update on connectivity failure
- Send recovery notification when connectivity restored
- Coordinate with update lock to avoid conflicts

**Integration Pattern:**
```go
type Service struct {
    interval     time.Duration
    checker      *ConnectivityChecker
    manager      *instance.Manager
    notifier     *notifier.Notifier
    logger       *slog.Logger
    mu           *sync.Mutex  // Shared update lock
    lastStatus   ConnectivityStatus
}

func (s *Service) Run(ctx context.Context) error {
    ticker := time.NewTicker(s.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            s.checkAndAct(ctx)
        }
    }
}

func (s *Service) checkAndAct(ctx context.Context) {
    status := s.checker.Check(ctx)

    // State transitions
    if status == Failed && s.lastStatus == Connected {
        // Trigger update on first failure
        s.triggerUpdate(ctx)
        s.notifier.NotifyFailure("Connectivity Lost", ...)
    } else if status == Connected && s.lastStatus == Failed {
        // Send recovery notification
        s.notifier.NotifyRecovery(...)
    }

    s.lastStatus = status
}
```

#### 3. Shared Update Lock

**Purpose:** Prevent race condition when both monitoring service and API try to trigger update simultaneously.

**Pattern:**
```go
// In main.go or coordinator
var updateMu sync.Mutex

// In HTTP handler
func (h *Handler) TriggerUpdate(w http.ResponseWriter, r *http.Request) {
    if !h.updateMu.TryLock() {
        // Update already in progress
        respondError(w, http.StatusConflict, "Update already in progress")
        return
    }
    defer h.updateMu.Unlock()

    // Proceed with update
    result, err := h.manager.UpdateAll(r.Context())
    ...
}

// In monitoring service (similar pattern)
func (s *Service) triggerUpdate(ctx context.Context) {
    if !s.updateMu.TryLock() {
        s.logger.Info("Update already in progress, skipping")
        return
    }
    defer s.updateMu.Unlock()

    result, err := s.manager.UpdateAll(ctx)
    ...
}
```

## Configuration Changes

### Modified Config Structure

```yaml
# config.yaml

# REMOVED: cron - no longer used
# cron: "0 3 * * *"

# NEW: API Server configuration
api:
  enabled: true
  port: 8080
  bearer_token: "your-secret-token-here"

# NEW: Monitoring configuration
monitoring:
  enabled: true
  check_interval: 15m
  target_url: "https://www.google.com"

# EXISTING: Pushover configuration (moved from env vars)
pushover:
  api_token: "..."
  user_key: "..."

# EXISTING: Instance configuration
instances:
  - name: "gateway"
    port: 18790
    start_command: "python -m nanobot.gateway"
    startup_timeout: 30s
```

### Config Struct Changes

```go
// internal/config/config.go

type Config struct {
    // REMOVED: Cron string - no longer needed

    // NEW
    Api        ApiConfig        `yaml:"api" mapstructure:"api"`
    Monitoring MonitoringConfig `yaml:"monitoring" mapstructure:"monitoring"`

    // EXISTING
    Instances  []InstanceConfig `yaml:"instances" mapstructure:"instances"`
    Pushover   PushoverConfig   `yaml:"pushover" mapstructure:"pushover"`
}

type ApiConfig struct {
    Enabled    bool   `yaml:"enabled" mapstructure:"enabled"`
    Port       int    `yaml:"port" mapstructure:"port"`
    BearerToken string `yaml:"bearer_token" mapstructure:"bearer_token"`
}

type MonitoringConfig struct {
    Enabled      bool          `yaml:"enabled" mapstructure:"enabled"`
    CheckInterval time.Duration `yaml:"check_interval" mapstructure:"check_interval"`
    TargetURL    string        `yaml:"target_url" mapstructure:"target_url"`
}

// Validate() adds:
// - ApiConfig: port range validation, token not empty if enabled
// - MonitoringConfig: interval >= 1 minute, valid URL
```

## Data Flow

### HTTP API Update Trigger Flow

```
[Client Request]
POST /api/v1/trigger-update
Authorization: Bearer <token>
    ↓
[Auth Middleware] → Validate Bearer token
    ↓ (invalid)
    → 401 Unauthorized
    ↓ (valid)
[Handler] → TryLock(updateMu)
    ↓ (locked)
    → 409 Conflict "Update in progress"
    ↓ (acquired)
    → InstanceManager.UpdateAll(ctx)
    ↓
[InstanceManager]
    → Stop all instances
    → UV update
    → Start all instances
    ↓
[Handler] → Unlock(updateMu)
    ↓
[Response] → JSON result (200 or 500)
```

### Monitoring Service Flow

```
[Monitoring Goroutine]
    ↓ (every 15 min)
[ConnectivityChecker.Check()]
    → HTTP GET https://www.google.com
    ↓ (timeout 10s)
[Status Determination]
    ↓ (success)
    → Update lastStatus = Connected
    → If previous was Failed: NotifyRecovery()
    ↓ (failure)
    → Update lastStatus = Failed
    → If previous was Connected:
        → TryLock(updateMu)
        → If acquired: triggerUpdate()
        → NotifyFailure("Connectivity Lost")
```

### Graceful Shutdown Flow

```
[SIGINT/SIGTERM Received]
    ↓
[Signal Handler]
    → Call cancel() on root context
    ↓
[Concurrent Shutdown]
    ├─> [HTTP Server]
    │       → Stop accepting new connections
    │       → Wait for in-flight requests (5s timeout)
    │       → Close all connections
    │
    ├─> [Monitoring Service]
    │       → <-ctx.Done()
    │       → Stop ticker
    │       → Exit goroutine
    │
    └─> [Instance Manager]
            → (No active coordination needed)
            → Existing operations complete naturally
    ↓
[Main] → All services stopped → Exit
```

## Goroutine Lifecycle Management

### Pattern: errgroup with Context

**Use `golang.org/x/sync/errgroup`** to coordinate HTTP server and monitoring service:

```go
// main.go
func main() {
    // ... config, logger setup ...

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Signal handler
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        <-sigChan
        logger.Info("Shutdown signal received")
        cancel()
    }()

    // Shared resources
    var updateMu sync.Mutex
    manager := instance.NewInstanceManager(cfg, logger)

    // Use errgroup for coordination
    g, ctx := errgroup.WithContext(ctx)

    // Start HTTP API server
    if cfg.Api.Enabled {
        apiServer := api.NewServer(cfg.Api, manager, &updateMu, logger)
        g.Go(func() error {
            return apiServer.Start(ctx)
        })
    }

    // Start monitoring service
    if cfg.Monitoring.Enabled {
        monitorSvc := monitor.NewService(cfg.Monitoring, manager, notif, &updateMu, logger)
        g.Go(func() error {
            return monitorSvc.Run(ctx)
        })
    }

    // Wait for all goroutines
    if err := g.Wait(); err != nil {
        logger.Error("Service error", "error", err)
        os.Exit(1)
    }

    logger.Info("Application shutdown complete")
}
```

**Benefits:**
- Automatic cancellation: when any goroutine returns error, all others receive cancellation
- Clean shutdown coordination
- No goroutine leaks

## Project Structure

```
nanobot-auto-updater/
├── cmd/
│   └── nanobot-auto-updater/
│       └── main.go              # MODIFIED: Add errgroup coordination
├── internal/
│   ├── api/                     # NEW PACKAGE
│   │   ├── server.go            # HTTP server lifecycle
│   │   ├── handlers.go          # /api/v1/trigger-update
│   │   ├── middleware.go        # Bearer token auth
│   │   └── server_test.go
│   ├── monitor/                 # NEW PACKAGE
│   │   ├── service.go           # Monitoring goroutine + ticker
│   │   ├── checker.go           # HTTP connectivity check
│   │   ├── state.go             # Connectivity state tracking
│   │   └── service_test.go
│   ├── config/
│   │   └── config.go            # MODIFIED: Add ApiConfig, MonitoringConfig
│   ├── instance/                # EXISTING - UNCHANGED
│   │   ├── manager.go
│   │   ├── lifecycle.go
│   │   └── errors.go
│   ├── notifier/                # MODIFIED
│   │   └── notifier.go          # Add NotifyRecovery() method
│   ├── updater/                 # EXISTING - UNCHANGED
│   ├── lifecycle/               # EXISTING - UNCHANGED
│   └── logging/                 # EXISTING - UNCHANGED
└── config.yaml                  # MODIFIED: Add api, monitoring sections
```

## Build Order (Suggested Implementation Phases)

### Phase 1: Configuration Foundation
**Goal:** Extend config to support new services

**Changes:**
1. Add `ApiConfig` and `MonitoringConfig` structs to `internal/config/config.go`
2. Add validation for new config fields
3. Update `config.yaml` with new sections (disabled by default)
4. Add config tests for new fields

**Dependencies:** None (standalone)

**Validation:** Unit tests pass, config loads correctly

---

### Phase 2: Monitoring Service Core
**Goal:** Implement connectivity monitoring without triggering updates

**Changes:**
1. Create `internal/monitor/` package
2. Implement `ConnectivityChecker` with HTTP GET to Google
3. Implement `Service` with ticker-based goroutine
4. Add state tracking (Connected/Failed transitions)
5. Add logger integration
6. Unit tests with mocked HTTP client

**Dependencies:** Phase 1 (config)

**Validation:** Monitoring logs connectivity status every 15 min, no update triggers yet

---

### Phase 3: HTTP API Server
**Goal:** Implement HTTP API with authentication

**Changes:**
1. Create `internal/api/` package
2. Implement `Server` with `net/http.Server`
3. Implement `/api/v1/trigger-update` handler (stub for now)
4. Implement Bearer token authentication middleware
5. Implement `/health` endpoint for health checks
6. Add graceful shutdown logic
7. Unit tests for handlers and middleware

**Dependencies:** Phase 1 (config)

**Validation:** HTTP server starts, rejects requests without valid token, returns 200 on health check

---

### Phase 4: Shared Update Lock + Integration
**Goal:** Connect services to InstanceManager with coordination

**Changes:**
1. Add `sync.Mutex` update lock in main.go
2. Wire HTTP API handler to call `InstanceManager.UpdateAll()`
3. Wire monitoring service to call `InstanceManager.UpdateAll()` on failure
4. Implement `TryLock()` pattern in both services
5. Integration tests for concurrent trigger attempts

**Dependencies:** Phase 2, Phase 3

**Validation:**
- HTTP trigger starts update
- Monitoring failure triggers update
- Concurrent triggers handled correctly (409 Conflict)

---

### Phase 5: Notification Enhancements
**Goal:** Add recovery notifications

**Changes:**
1. Add `NotifyRecovery()` method to `internal/notifier/notifier.go`
2. Wire monitoring service to send recovery notification
3. Add tests for new notification path

**Dependencies:** Phase 2 (monitoring)

**Validation:** Recovery notification sent when connectivity restores after failure

---

### Phase 6: Main Application Coordination
**Goal:** Wire everything together in main.go

**Changes:**
1. Import `golang.org/x/sync/errgroup`
2. Create root context with cancellation
3. Initialize HTTP server and monitoring service conditionally
4. Use errgroup to coordinate goroutines
5. Update signal handler to cancel context
6. Integration tests

**Dependencies:** Phase 1-5

**Validation:**
- Both services start and run
- Graceful shutdown on SIGINT
- No goroutine leaks

---

### Phase 7: Remove Legacy Cron
**Goal:** Clean up old scheduler code

**Changes:**
1. Remove `internal/scheduler/` package (no longer used)
2. Remove `Cron` field from Config struct
3. Update main.go to remove scheduler initialization
4. Update documentation

**Dependencies:** Phase 6 (all new functionality working)

**Validation:** Application runs without cron, only API + monitoring

---

### Phase 8: End-to-End Testing
**Goal:** Validate entire system

**Tests:**
1. Start application with both services enabled
2. Test HTTP trigger via curl with Bearer token
3. Simulate connectivity failure (mock) → verify update triggered
4. Simulate connectivity recovery → verify notification sent
5. Test concurrent triggers → verify lock behavior
6. Test graceful shutdown → verify no goroutine leaks

**Dependencies:** All phases

**Validation:** All E2E tests pass

## Architectural Patterns

### Pattern 1: Context-Based Cancellation

**What:** Use `context.Context` for all long-running operations and goroutine coordination.

**When:** HTTP server, monitoring ticker, update operations.

**Example:**
```go
func (s *Server) Start(ctx context.Context) error {
    // Start server in goroutine
    errCh := make(chan error, 1)
    go func() {
        if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
            errCh <- err
        }
    }()

    // Wait for cancellation or error
    select {
    case <-ctx.Done():
        // Graceful shutdown
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        return s.httpServer.Shutdown(shutdownCtx)
    case err := <-errCh:
        return err
    }
}
```

**Trade-offs:**
- ✅ Clean shutdown without dangling goroutines
- ✅ Propagates cancellation automatically
- ❌ Requires all code to respect context

---

### Pattern 2: TryLock for Non-Blocking Coordination

**What:** Use `sync.Mutex.TryLock()` to avoid blocking when update is already in progress.

**When:** HTTP handler and monitoring trigger both try to start update.

**Example:**
```go
func (h *Handler) TriggerUpdate(w http.ResponseWriter, r *http.Request) {
    if !h.updateMu.TryLock() {
        respondJSON(w, http.StatusConflict, map[string]string{
            "error": "Update already in progress",
        })
        return
    }
    defer h.updateMu.Unlock()

    // Proceed with update
    result, err := h.manager.UpdateAll(r.Context())
    ...
}
```

**Trade-offs:**
- ✅ Non-blocking, immediate feedback
- ✅ Simple coordination mechanism
- ❌ Clients must retry if needed

---

### Pattern 3: errgroup for Goroutine Coordination

**What:** Use `golang.org/x/sync/errgroup` to coordinate multiple goroutines with error propagation.

**When:** Main function coordinating HTTP server and monitoring service.

**Example:**
```go
g, ctx := errgroup.WithContext(ctx)

// Start HTTP server
g.Go(func() error {
    return apiServer.Start(ctx)
})

// Start monitoring service
g.Go(func() error {
    return monitorSvc.Run(ctx)
})

// Wait for all goroutines
if err := g.Wait(); err != nil {
    logger.Error("Service error", "error", err)
    os.Exit(1)
}
```

**Trade-offs:**
- ✅ Automatic error propagation
- ✅ Context cancellation to all goroutines
- ✅ No goroutine leaks
- ❌ Requires external dependency (x/sync)

---

### Pattern 4: Middleware Chain for Authentication

**What:** Chain middleware functions for authentication and logging.

**When:** HTTP API authentication.

**Example:**
```go
func AuthMiddleware(token string, logger *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                respondError(w, http.StatusUnauthorized, "Missing Authorization header")
                return
            }

            if !strings.HasPrefix(authHeader, "Bearer ") {
                respondError(w, http.StatusUnauthorized, "Invalid Authorization header format")
                return
            }

            providedToken := strings.TrimPrefix(authHeader, "Bearer ")
            if providedToken != token {
                respondError(w, http.StatusUnauthorized, "Invalid token")
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}

// Usage
mux := http.NewServeMux()
mux.HandleFunc("/api/v1/trigger-update", handlers.TriggerUpdate)
authenticated := AuthMiddleware(cfg.BearerToken, logger)(mux)
```

**Trade-offs:**
- ✅ Separates auth logic from business logic
- ✅ Reusable across endpoints
- ❌ Slight complexity increase

## Anti-Patterns to Avoid

### Anti-Pattern 1: Blocking Update Lock

**What people do:** Use `sync.Mutex.Lock()` in HTTP handler, causing long wait times.

**Why it's wrong:** HTTP client times out while waiting for lock, poor UX.

**Do this instead:** Use `TryLock()` and return HTTP 409 Conflict immediately if update in progress.

---

### Anti-Pattern 2: Goroutine Leak from Ticker

**What people do:** Create `time.Ticker` without `Stop()` in goroutine.

**Why it's wrong:** Ticker goroutine continues running after context cancellation, causing memory leak.

**Do this instead:**
```go
func (s *Service) Run(ctx context.Context) error {
    ticker := time.NewTicker(s.interval)
    defer ticker.Stop()  // ALWAYS defer Stop()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            s.checkAndAct(ctx)
        }
    }
}
```

---

### Anti-Pattern 3: Ignoring Context in HTTP Client

**What people do:** Use `http.Get()` without context timeout for connectivity checks.

**Why it's wrong:** Request hangs indefinitely, monitoring service stalls.

**Do this instead:**
```go
func (c *Checker) Check(ctx context.Context) ConnectivityStatus {
    req, err := http.NewRequestWithContext(ctx, "GET", c.targetURL, nil)
    if err != nil {
        return Failed
    }

    resp, err := c.client.Do(req)
    if err != nil {
        return Failed
    }
    defer resp.Body.Close()

    if resp.StatusCode == 200 {
        return Connected
    }
    return Failed
}
```

---

### Anti-Pattern 4: Shared Global State

**What people do:** Use global variables for update lock or last monitoring status.

**Why it's wrong:** Hard to test, makes dependencies implicit, causes race conditions.

**Do this instead:** Pass dependencies explicitly via struct fields or constructor parameters.

```go
// BAD
var updateMu sync.Mutex  // Global variable

// GOOD
type Server struct {
    updateMu *sync.Mutex  // Explicit dependency
}

func NewServer(updateMu *sync.Mutex) *Server {
    return &Server{updateMu: updateMu}
}
```

---

### Anti-Pattern 5: HTTP Server Without Graceful Shutdown

**What people do:** Call `httpServer.Close()` immediately on shutdown signal.

**Why it's wrong:** In-flight requests are abruptly terminated, clients receive connection errors.

**Do this instead:** Use `httpServer.Shutdown(ctx)` with timeout:
```go
shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
    s.logger.Error("Graceful shutdown failed, forcing close", "error", err)
    s.httpServer.Close()
}
```

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| **Single user (current)** | Current architecture is optimal - single process, in-memory lock |
| **10 concurrent users** | Add rate limiting middleware, queue update requests instead of rejecting with 409 |
| **100 concurrent users** | Consider external lock service (Redis), separate API and worker processes |
| **1000+ users** | Not applicable - this is a personal tool, not SaaS |

### Scaling Priorities

1. **First bottleneck:** Concurrent update requests - solve with queue + worker pattern
2. **Second bottleneck:** Long-running updates blocking new requests - solve with separate worker process

**Note:** These scaling considerations are theoretical. Current design is optimized for single-user personal tool.

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| Google (monitoring target) | HTTP GET with context timeout | Use `http.Client` with 10s timeout |
| Pushover (notifications) | Existing `Notifier` abstraction | Add new `NotifyRecovery()` method |
| UV package manager | Existing `Updater` abstraction | No changes needed |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| HTTP API ↔ InstanceManager | Direct method call | Synchronous, returns `UpdateResult` |
| Monitoring ↔ InstanceManager | Direct method call | Synchronous, called from ticker goroutine |
| Monitoring ↔ Notifier | Direct method call | Async notification sending |
| All components ↔ Logger | Slog logger injection | Pre-injected via constructor |

## Testing Strategy

### Unit Tests
- **Config:** Validate new fields, validation logic
- **API:** Handler logic with mocked InstanceManager
- **Monitoring:** Connectivity checker with mocked HTTP client, state transitions
- **Notifier:** New `NotifyRecovery()` method

### Integration Tests
- HTTP server + authentication middleware
- Monitoring service + ticker + state tracking
- Update lock coordination between API and monitoring

### End-to-End Tests
- Start full application with both services
- Trigger update via API → verify update runs
- Simulate connectivity failure → verify update triggered
- Test graceful shutdown → verify no goroutine leaks

## Sources

### Official Documentation
- [Go net/http package](https://pkg.go.dev/net/http) - Server.Shutdown() for graceful shutdown
- [Go sync package](https://pkg.go.dev/sync) - Mutex.TryLock() for non-blocking coordination
- [golang.org/x/sync/errgroup](https://pkg.go.dev/golang.org/x/sync/errgroup) - Goroutine coordination

### Architecture Patterns
- [How to Use Graceful Shutdown in a Go Cloud Run Service with Context Cancellation](https://oneuptime.com/blog/post/2026-02-17-how-to-implement-graceful-shutdown-in-a-go-cloud-run-service-with-context-cancellation/view) - Context-based shutdown pattern (HIGH confidence)
- [Go Channel Patterns: A Complete Guide](https://oneuptime.com/blog/post/2026-01-23-go-channel-patterns/view) - Ticker and context usage (HIGH confidence)
- [How to Use errgroup for Parallel Operations in Go](https://oneuptime.com/blog/post/2026-01-07-go-errgroup/view) - errgroup coordination pattern (HIGH confidence)
- [How to Implement Middleware in Go Web Applications](https://oneuptime.com/blog/post/2026-01-26-go-middleware/view) - Authentication middleware pattern (HIGH confidence)
- [How to Implement Background Job Processing in Go](https://oneuptime.com/blog/post/2026-01-30-go-background-job-processing/view) - Worker coordination (HIGH confidence)

### Community Resources
- [Golang Ticker Best Practices](https://www.reddit.com/r/golang/comments/hpw4q9/golang_ticker_best_practices_using_tickers_in_a/) - Ticker cleanup patterns (MEDIUM confidence)
- [Standards for user authentication for REST APIs?](https://www.reddit.com/r/golang/comments/axou3k/standards_for_user_authentication_for_rest_apis/) - Bearer token usage (MEDIUM confidence)

---
*Architecture research for: HTTP API + Monitoring Service Integration*
*Researched: 2026-03-16*

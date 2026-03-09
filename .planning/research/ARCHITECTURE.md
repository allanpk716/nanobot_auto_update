# Architecture Research: Multi-Instance Management

**Domain:** Multi-instance process management for Windows auto-updater
**Researched:** 2026-03-09
**Confidence:** HIGH

## Executive Summary

This research addresses **extending the existing single-instance nanobot auto-updater to support multiple instances**. The current architecture has clean separation between lifecycle management, updating, and scheduling, making it straightforward to add multi-instance orchestration.

**Key Recommendation:** Add a new `internal/instance` package with an `InstanceManager` orchestrator that coordinates stop-all → update → start-all workflows, while preserving the existing single-instance lifecycle logic in `internal/lifecycle`.

## Current Architecture Analysis

### Existing System Structure

```
┌─────────────────────────────────────────────────────────────┐
│                    cmd/nanobot-auto-updater                 │
│                    (Main Entry Point)                       │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   scheduler  │  │   updater    │  │   notifier   │      │
│  │  (cron jobs) │  │  (UV logic)  │  │  (Pushover)  │      │
│  └──────┬───────┘  └──────┬───────┘  └──────────────┘      │
│         │                 │                                  │
├─────────┴─────────────────┴──────────────────────────────────┤
│                    internal/config                           │
│              (Single-instance configuration)                 │
├─────────────────────────────────────────────────────────────┤
│                    internal/lifecycle                        │
│              (Single-instance management)                    │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐    │
│  │ detector │  │ stopper  │  │ starter  │  │ manager  │    │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘    │
└─────────────────────────────────────────────────────────────┘
```

### Current Component Responsibilities

| Component | Responsibility | Current Implementation |
|-----------|----------------|------------------------|
| config | Configuration loading, validation | Single `NanobotConfig` (port, timeout, repo_path) |
| lifecycle.Manager | Orchestrates stop/start for single instance | `StopForUpdate()`, `StartAfterUpdate()` methods |
| lifecycle.detector | Process detection | Port-based and process name detection |
| lifecycle.stopper | Process termination | Graceful → force kill with timeout |
| lifecycle.starter | Process startup | Background spawn with verification |
| updater | UV-based update logic | GitHub main → PyPI fallback |
| scheduler | Cron job management | Single update job with overlap prevention |
| notifier | Failure/success notifications | Pushover integration |

## Recommended Multi-Instance Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    cmd/nanobot-auto-updater                 │
│                    (Main Entry Point)                       │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────────────────────────────────────────────┐   │
│  │           internal/instance (NEW PACKAGE)             │   │
│  │                  InstanceManager                       │   │
│  │  ┌────────────────────────────────────────────────┐   │   │
│  │  │  Instance Orchestrator (supervisor pattern)    │   │   │
│  │  │  - Manage all nanobot instances                │   │   │
│  │  │  - Coordinate stop all → update → start all    │   │   │
│  │  │  - Collect results and failures                │   │   │
│  │  └────────────────────────────────────────────────┘   │   │
│  └──────────────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   scheduler  │  │   updater    │  │   notifier   │      │
│  │  (unchanged) │  │  (unchanged) │  │  (enhanced)  │      │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │
│         │                 │                 │               │
├─────────┴─────────────────┴─────────────────┴───────────────┤
│                    internal/config (EXTENDED)                │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Config struct with Instances []InstanceConfig       │   │
│  │  Each InstanceConfig: Name, Port, StartupTimeout,    │   │
│  │                       RepoPath                       │   │
│  └──────────────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────────────┤
│              internal/lifecycle (EXTENDED)                   │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  InstanceLifecycle (per-instance lifecycle manager)   │  │
│  │  - StopForUpdate(ctx) → error                         │  │
│  │  - StartAfterUpdate(ctx) → error                      │  │
│  │  - Uses existing detector/stopper/starter logic       │  │
│  └───────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  Existing detector/stopper/starter (unchanged)        │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### Component Responsibilities (Multi-Instance)

| Component | Responsibility | Changes Required |
|-----------|----------------|------------------|
| **config** | Load instance list from YAML | **EXTEND:** Add `Instances []InstanceConfig` field, validation for unique names/ports |
| **instance** (NEW) | Coordinate multi-instance operations | **CREATE:** New package with `InstanceManager` orchestrator |
| **lifecycle** | Per-instance lifecycle operations | **EXTEND:** Add `InstanceLifecycle` wrapper that uses instance name for logging context |
| **updater** | UV update logic | **UNCHANGED:** Already global operation (updates nanobot binary) |
| **scheduler** | Cron job management | **MINIMAL:** Update job function to call `InstanceManager.UpdateAll()` |
| **notifier** | Pushover notifications | **EXTEND:** Add `NotifyInstanceFailure(name, operation, err)` method |

## Recommended Project Structure

```
internal/
├── config/
│   ├── config.go              # EXTEND: Add InstanceConfig struct, Instances slice
│   └── config_test.go         # EXTEND: Add multi-instance validation tests
│
├── instance/                  # NEW PACKAGE
│   ├── manager.go             # InstanceManager - orchestrates all instances
│   ├── manager_test.go        # Unit tests for InstanceManager
│   └── types.go               # Result types (InstanceResult, UpdateAllResult)
│
├── lifecycle/
│   ├── detector.go            # UNCHANGED
│   ├── stopper.go             # UNCHANGED
│   ├── starter.go             # UNCHANGED
│   ├── manager.go             # EXTEND: Add InstanceLifecycle wrapper
│   └── manager_test.go        # EXTEND: Add InstanceLifecycle tests
│
├── updater/
│   ├── updater.go             # UNCHANGED (global nanobot update)
│   └── checker.go             # UNCHANGED
│
├── scheduler/
│   └── scheduler.go           # MINIMAL CHANGE (update job function)
│
└── notifier/
    └── notifier.go            # EXTEND: Add instance-specific notification methods

cmd/
└── nanobot-auto-updater/
    └── main.go                # MODIFY: Initialize InstanceManager, update job logic
```

### Structure Rationale

- **internal/instance/**: New package for multi-instance coordination logic (single responsibility: orchestration)
- **internal/config/**: Extended in-place because instance configuration is natural extension of app config
- **internal/lifecycle/**: Extended minimally with `InstanceLifecycle` wrapper to preserve existing single-instance logic
- **internal/updater/**: Unchanged because UV updates the global nanobot binary (not instance-specific)
- **internal/scheduler/**: Minimal changes because scheduling is unchanged (only job function calls new orchestrator)

## Architectural Patterns

### Pattern 1: Supervisor/Orchestrator Pattern

**What:** A central `InstanceManager` coordinates lifecycle operations across multiple instances, similar to Erlang supervisor trees or process managers.

**When to use:** When managing multiple related processes that need coordinated start/stop/update operations.

**Trade-offs:**
- ✅ Clear separation of concerns (orchestration vs. individual lifecycle)
- ✅ Easy to add per-instance error handling and continuation
- ✅ Simple to aggregate results for notification
- ❌ Adds one layer of indirection
- ❌ Requires careful error aggregation design

**Example:**

```go
// internal/instance/manager.go
package instance

import (
    "context"
    "fmt"
    "log/slog"

    "github.com/HQGroup/nanobot-auto-updater/internal/config"
    "github.com/HQGroup/nanobot-auto-updater/internal/lifecycle"
)

// InstanceManager coordinates multi-instance operations
type InstanceManager struct {
    instances map[string]*lifecycle.InstanceLifecycle // keyed by instance name
    logger    *slog.Logger
}

func NewInstanceManager(cfg *config.Config, logger *slog.Logger) *InstanceManager {
    instances := make(map[string]*lifecycle.InstanceLifecycle)
    for _, instCfg := range cfg.Instances {
        instances[instCfg.Name] = lifecycle.NewInstanceLifecycle(instCfg, logger)
    }
    return &InstanceManager{
        instances: instances,
        logger:    logger,
    }
}

// StopAllInstances stops all instances, collecting errors but continuing on failure
func (m *InstanceManager) StopAllInstances(ctx context.Context) []InstanceResult {
    m.logger.Info("Stopping all instances", "count", len(m.instances))

    var results []InstanceResult
    for name, lc := range m.instances {
        m.logger.Info("Stopping instance", "name", name)
        err := lc.StopForUpdate(ctx)
        results = append(results, InstanceResult{
            Name:      name,
            Operation: "stop",
            Success:   err == nil,
            Error:     err,
        })
        if err != nil {
            m.logger.Error("Failed to stop instance", "name", name, "error", err)
            // Continue with other instances (don't return early)
        }
    }

    return results
}

// StartAllInstances starts all instances, continuing on failure
func (m *InstanceManager) StartAllInstances(ctx context.Context) []InstanceResult {
    m.logger.Info("Starting all instances", "count", len(m.instances))

    var results []InstanceResult
    for name, lc := range m.instances {
        m.logger.Info("Starting instance", "name", name)
        err := lc.StartAfterUpdate(ctx)
        results = append(results, InstanceResult{
            Name:      name,
            Operation: "start",
            Success:   err == nil,
            Error:     err,
        })
        if err != nil {
            m.logger.Error("Failed to start instance", "name", name, "error", err)
            // Continue with other instances
        }
    }

    return results
}
```

### Pattern 2: Configuration Extension Pattern

**What:** Extend existing configuration structures with slice of instance configs, maintaining backward compatibility.

**When to use:** When adding multi-instance support to a single-instance system.

**Trade-offs:**
- ✅ Maintains backward compatibility (old configs still work)
- ✅ Natural YAML structure (`instances:` list)
- ✅ Validation logic stays centralized in config package
- ❌ Slightly more complex validation (unique names, ports)

**Example:**

```go
// internal/config/config.go
package config

// InstanceConfig holds configuration for a single nanobot instance
type InstanceConfig struct {
    Name           string        `yaml:"name" mapstructure:"name"`
    Port           uint32        `yaml:"port" mapstructure:"port"`
    StartupTimeout time.Duration `yaml:"startup_timeout" mapstructure:"startup_timeout"`
    RepoPath       string        `yaml:"repo_path" mapstructure:"repo_path"`
}

// Config holds the main application configuration (EXTENDED)
type Config struct {
    Cron      string           `yaml:"cron" mapstructure:"cron"`
    Instances []InstanceConfig `yaml:"instances" mapstructure:"instances"` // NEW
    Pushover  PushoverConfig   `yaml:"pushover" mapstructure:"pushover"`
}

// Validate validates the entire Config (EXTENDED)
func (c *Config) Validate() error {
    if err := ValidateCron(c.Cron); err != nil {
        return err
    }

    // Validate unique instance names
    names := make(map[string]bool)
    ports := make(map[uint32]bool)

    for _, inst := range c.Instances {
        if names[inst.Name] {
            return fmt.Errorf("duplicate instance name: %s", inst.Name)
        }
        if ports[inst.Port] {
            return fmt.Errorf("duplicate port %d in instance %s", inst.Port, inst.Name)
        }
        names[inst.Name] = true
        ports[inst.Port] = true

        if err := inst.Validate(); err != nil {
            return fmt.Errorf("instance %s: %w", inst.Name, err)
        }
    }

    return nil
}
```

### Pattern 3: Context-Aware Logging Pattern

**What:** Each instance lifecycle operation includes instance name in all log messages for traceability.

**When to use:** When managing multiple concurrent processes in logs.

**Trade-offs:**
- ✅ Easy to trace which instance an operation belongs to
- ✅ Simple to implement with logger.With("instance", name)
- ✅ Helps debugging multi-instance issues
- ❌ Slightly more verbose logs

**Example:**

```go
// internal/lifecycle/manager.go (EXTENDED)
package lifecycle

// InstanceLifecycle wraps Manager with instance-specific context
type InstanceLifecycle struct {
    manager *Manager
    name    string
    logger  *slog.Logger // Pre-configured with instance name
}

func NewInstanceLifecycle(cfg InstanceConfig, logger *slog.Logger) *InstanceLifecycle {
    instanceLogger := logger.With("instance", cfg.Name)

    managerCfg := Config{
        Port:           cfg.Port,
        StartupTimeout: cfg.StartupTimeout,
    }

    return &InstanceLifecycle{
        manager: NewManager(managerCfg, instanceLogger),
        name:    cfg.Name,
        logger:  instanceLogger,
    }
}

func (il *InstanceLifecycle) StopForUpdate(ctx context.Context) error {
    il.logger.Info("Stopping instance for update")
    return il.manager.StopForUpdate(ctx)
}

func (il *InstanceLifecycle) StartAfterUpdate(ctx context.Context) error {
    il.logger.Info("Starting instance after update")
    return il.manager.StartAfterUpdate(ctx)
}
```

## Data Flow

### Multi-Instance Update Flow

```
[Cron Trigger / --update-now]
    ↓
[main.go] → [InstanceManager.UpdateAll()]
    ↓
┌─────────────────────────────────────────┐
│  Phase 1: Stop All Instances            │
│  ┌──────────────────────────────────┐   │
│  │ for each instance:               │   │
│  │   - detector.IsNanobotRunning()  │   │
│  │   - stopper.StopNanobot()        │   │
│  │   - collect result               │   │
│  │   - continue on failure          │   │
│  └──────────────────────────────────┘   │
└─────────────────────────────────────────┘
    ↓
[Check if any instances stopped successfully]
    ↓
┌─────────────────────────────────────────┐
│  Phase 2: Update Binary (ONCE)          │
│  ┌──────────────────────────────────┐   │
│  │ updater.Update()                 │   │
│  │   - GitHub main → PyPI fallback  │   │
│  └──────────────────────────────────┘   │
└─────────────────────────────────────────┘
    ↓
[If update succeeded]
    ↓
┌─────────────────────────────────────────┐
│  Phase 3: Start All Instances           │
│  ┌──────────────────────────────────┐   │
│  │ for each instance:               │   │
│  │   - starter.StartNanobot()       │   │
│  │   - collect result               │   │
│  │   - continue on failure          │   │
│  └──────────────────────────────────┘   │
└─────────────────────────────────────────┘
    ↓
[Aggregate results]
    ↓
┌─────────────────────────────────────────┐
│  Phase 4: Notifications                 │
│  ┌──────────────────────────────────┐   │
│  │ if any failures:                 │   │
│  │   - notifier.NotifyFailures()    │   │
│  │     with instance names/details  │   │
│  │ if all success:                  │   │
│  │   - notifier.NotifySuccess()     │   │
│  └──────────────────────────────────┘   │
└─────────────────────────────────────────┘
    ↓
[Return UpdateAllResult (JSON for --update-now)]
```

### Configuration Loading Flow

```
[config.yaml]
    ↓
[viper.ReadInConfig()]
    ↓
┌─────────────────────────────────────────┐
│  instances:                              │
│    - name: "instance1"                   │
│      port: 18790                         │
│      startup_timeout: 30s                │
│    - name: "instance2"                   │
│      port: 18791                         │
│      startup_timeout: 30s                │
└─────────────────────────────────────────┘
    ↓
[config.Load()]
    ↓
┌─────────────────────────────────────────┐
│  Validation:                             │
│  - Unique names?                        │
│  - Unique ports?                        │
│  - Each instance valid?                 │
└─────────────────────────────────────────┘
    ↓
[Config struct with Instances slice]
    ↓
[InstanceManager initialization]
    ↓
┌─────────────────────────────────────────┐
│  for each InstanceConfig:               │
│    - Create InstanceLifecycle           │
│    - Add to map by name                 │
└─────────────────────────────────────────┘
```

### Key Data Flows

1. **Instance Configuration Flow:** YAML → viper → Config.Instances[] → InstanceManager.instances map (keyed by name)
2. **Stop-All Flow:** InstanceManager.StopAll() → sequential stop attempts → collect InstanceResult[] → continue on errors
3. **Start-All Flow:** InstanceManager.StartAll() → sequential start attempts → collect InstanceResult[] → continue on errors
4. **Notification Flow:** InstanceResult[] aggregation → filter failures → NotifyFailures(failedInstances) with instance names

## Integration Points

### Existing Code Integration

| Component | Integration Pattern | Notes |
|-----------|---------------------|-------|
| **config** | Direct extension | Add `Instances []InstanceConfig` to existing `Config` struct |
| **lifecycle.Manager** | Wrapped by InstanceLifecycle | Existing single-instance logic preserved, InstanceLifecycle adds context |
| **updater.Updater** | Called once by InstanceManager | Update remains global (updates nanobot binary for all instances) |
| **scheduler** | Update job function modified | Replace direct update call with `instanceManager.UpdateAll()` |
| **notifier** | Extended with instance methods | Add `NotifyInstanceFailures(results []InstanceResult)` |
| **main.go** | Initialization changes | Create InstanceManager, pass to scheduler job function |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| **instance ↔ lifecycle** | Method calls | InstanceManager calls InstanceLifecycle methods |
| **instance ↔ updater** | Method calls | InstanceManager calls Updater.Update() once |
| **instance ↔ notifier** | Method calls | InstanceManager calls Notifier with aggregated results |
| **lifecycle ↔ detector/stopper/starter** | Function calls | InstanceLifecycle delegates to existing functions |

## Build Order and Dependencies

### Phase 1: Configuration Extension (NO DEPENDENCIES)
**Files to modify:**
- `internal/config/config.go` — Add `InstanceConfig` struct, extend `Config` with `Instances []InstanceConfig`, add validation
- `internal/config/config_test.go` — Add multi-instance validation tests

**Why first:** No other code depends on this, can be tested independently

**Estimated effort:** 1-2 hours (includes tests)

### Phase 2: Lifecycle Extension (DEPENDS ON: Phase 1)
**Files to modify:**
- `internal/lifecycle/manager.go` — Add `InstanceLifecycle` wrapper struct, `NewInstanceLifecycle()` constructor
- `internal/lifecycle/manager_test.go` — Add InstanceLifecycle tests

**Why second:** Needs `InstanceConfig` from Phase 1, wraps existing Manager

**Estimated effort:** 1-2 hours (includes tests)

### Phase 3: Instance Package Creation (DEPENDS ON: Phases 1-2)
**Files to create:**
- `internal/instance/types.go` — Define `InstanceResult`, `UpdateAllResult` structs
- `internal/instance/manager.go` — Implement `InstanceManager` with `StopAllInstances()`, `StartAllInstances()`, `UpdateAll()`
- `internal/instance/manager_test.go` — Unit tests for InstanceManager

**Why third:** Needs `InstanceLifecycle` from Phase 2 and config from Phase 1

**Estimated effort:** 3-4 hours (includes tests and result aggregation logic)

### Phase 4: Notifier Extension (DEPENDS ON: Phase 3)
**Files to modify:**
- `internal/notifier/notifier.go` — Add `NotifyInstanceFailures(results []InstanceResult)` method

**Why fourth:** Needs `InstanceResult` type from Phase 3

**Estimated effort:** 30 minutes - 1 hour

### Phase 5: Main Integration (DEPENDS ON: Phases 1-4)
**Files to modify:**
- `cmd/nanobot-auto-updater/main.go` — Initialize InstanceManager, modify update job function, modify --update-now logic

**Why last:** Integrates all components, end-to-end testing

**Estimated effort:** 2-3 hours (includes integration testing)

### Total Estimated Effort
**7.5 - 12 hours** across all phases

## Anti-Patterns

### Anti-Pattern 1: Parallel Instance Operations (Premature Optimization)

**What people do:** Use goroutines + sync.WaitGroup to stop/start instances in parallel

**Why it's wrong:**
- Adds unnecessary complexity for managing 2-3 instances
- Windows process management is not CPU-bound
- Error handling becomes more complex with concurrent operations
- Logging becomes harder to trace without careful coordination

**Do this instead:** Sequential loop with continue-on-error. Simple, easy to debug, fast enough for small N instances.

**Code to avoid:**
```go
// DON'T DO THIS - Premature parallelization
var wg sync.WaitGroup
var mu sync.Mutex
results := []InstanceResult{}

for name, lc := range m.instances {
    wg.Add(1)
    go func(name string, lc *lifecycle.InstanceLifecycle) {
        defer wg.Done()
        err := lc.StopForUpdate(ctx)
        mu.Lock()
        results = append(results, InstanceResult{Name: name, Error: err})
        mu.Unlock()
    }(name, lc)
}
wg.Wait()
```

**Do this instead:**
```go
// DO THIS - Simple sequential processing
var results []InstanceResult
for name, lc := range m.instances {
    if err := lc.StopForUpdate(ctx); err != nil {
        m.logger.Error("Failed to stop instance", "name", name, "error", err)
        results = append(results, InstanceResult{Name: name, Error: err})
        // Continue with next instance
    }
}
```

### Anti-Pattern 2: Global Instance Registry

**What people do:** Create a global map/slice of instances accessible from anywhere

**Why it's wrong:**
- Makes testing difficult (global state)
- Unclear ownership (who modifies the registry?)
- Hard to reason about lifecycle (when is registry populated?)
- Violates dependency injection principles

**Do this instead:** Pass InstanceManager explicitly through constructor/function parameters

### Anti-Pattern 3: Per-Instance Updater Instances

**What people do:** Create a separate `updater.Updater` instance for each nanobot instance

**Why it's wrong:**
- UV updates the global nanobot binary (not instance-specific)
- Multiple updater instances would duplicate work
- No benefit since update is not instance-specific

**Do this instead:** Single `updater.Updater` instance, called once by InstanceManager before starting instances

### Anti-Pattern 4: Port-Based Instance Identification

**What people do:** Use port number as instance identifier in logs/errors

**Why it's wrong:**
- Users configure instance by name, not port
- Ports are implementation details, not user-facing identifiers
- Hard to understand errors like "Failed to stop instance on port 18790"

**Do this instead:** Always use instance name for identification, include port only in debug logs

## Scaling Considerations

| Concern | 2-3 Instances | 10+ Instances | 50+ Instances |
|---------|---------------|---------------|---------------|
| **Stop/Start time** | Sequential is fine (< 10 seconds) | Sequential still OK (< 30 seconds) | Consider parallelization with worker pool |
| **Error aggregation** | Simple slice | Group by error type for readability | Consider error summarization (failed: 5/50) |
| **Notification noise** | Send all failures | Group failures in single notification | Only notify on critical failures, log rest |
| **Log volume** | Manageable | Consider log sampling | Centralized logging with filtering |

### When to Parallelize

**Threshold:** Consider parallel stop/start when:
1. Managing 10+ instances
2. Total sequential time exceeds 30 seconds
3. User feedback indicates slowness

**How to parallelize safely:**
```go
// Worker pool pattern for N > 10 instances
func (m *InstanceManager) StopAllInstancesParallel(ctx context.Context, maxWorkers int) []InstanceResult {
    jobs := make(chan string, len(m.instances))
    results := make(chan InstanceResult, len(m.instances))

    // Start workers
    var wg sync.WaitGroup
    for i := 0; i < maxWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for name := range jobs {
                err := m.instances[name].StopForUpdate(ctx)
                results <- InstanceResult{Name: name, Error: err}
            }
        }()
    }

    // Send jobs
    for name := range m.instances {
        jobs <- name
    }
    close(jobs)

    // Wait and collect results
    go func() {
        wg.Wait()
        close(results)
    }()

    var allResults []InstanceResult
    for result := range results {
        allResults = append(allResults, result)
    }

    return allResults
}
```

## Sources

- [Reddit: Process Monitoring/Control Design Patterns in Go](https://www.reddit.com/r/golang/comments/mk7zfm/which_design_pattern_for_process_monitoringcontrol/) — Community discussion on process lifecycle management
- [Medium: Goroutine Management Strategies and Patterns](https://dsysd-dev.medium.com/mastering-goroutine-management-in-go-strategies-and-patterns-20645b113851) — Comprehensive guide on goroutine lifecycle patterns
- [Level Up: Context-Based Goroutine Management](https://levelup.gitconnected.com/how-to-use-context-to-manage-your-goroutines-like-a-boss-ef1e478919e6) — Using context.Context for lifecycle control
- [Suture: Supervisor Trees for Go](https://www.jerf.org/iri/post/2930/) — Erlang-style supervisor tree implementation in Go (reference for pattern, but overkill for this use case)
- [Go Design Patterns GitHub Repository](https://github.com/tmrts/go-patterns) — Curated collection of idiomatic Go patterns
- [Dependency Lifecycle Management in Go](https://www.jacoelho.com/blog/2025/05/dependency-lifecycle-management-in-go/) — Patterns for managing component initialization and startup ordering
- [GitHub Issue: Windows Job Objects for Process Groups](https://github.com/golang/go/issues/17608) — Discussion on Windows process group management (not needed for this use case)
- [GitHub Gist: Managing Child Processes on Windows](https://gist.github.com/hallazzang/76f3970bfc949831808bbebc8ca15209) — Practical examples of Windows process lifecycle management

---
*Architecture research for: Multi-instance nanobot management integration*
*Researched: 2026-03-09*

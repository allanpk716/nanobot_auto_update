# Stack Research

**Domain:** Multi-instance nanobot process management (Windows service)
**Researched:** 2026-03-09
**Confidence:** HIGH

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| golang.org/x/sync/errgroup | v0.18.0 (latest) | Concurrent goroutine coordination with error propagation | Provides built-in error handling for parallel operations - ideal for managing multiple nanobot stop/start operations simultaneously with proper error collection |
| sync.Map | stdlib (Go 1.24+) | Thread-safe process state tracking | Go 1.24+ uses HashTrieMap backing with lock-free reads - perfect for read-heavy state tracking scenarios where multiple goroutines check instance status |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| context | stdlib | Cancellation and timeout propagation | Use for coordinating stop/start operations across multiple instances with deadlines |
| log/slog | stdlib (already in use) | Structured logging with instance context | Use for multi-instance logging with instance IDs embedded in log attributes |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| go test -race | Detect race conditions in concurrent code | Essential for validating multi-instance state management |
| pprof | Profile goroutine usage | Monitor goroutine leaks when managing multiple concurrent process operations |

## Installation

```bash
# Add errgroup for concurrent process management
go get golang.org/x/sync@v0.18.0

# No additional installs needed - sync.Map and context are in stdlib
```

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| External process managers (supervisord, systemd) | Overkill for simple multi-instance coordination - we're managing Python processes, not orchestrating containers | Use existing os/exec + errgroup for lightweight coordination |
| Temporal/Cadence workflow engines | Designed for distributed workflows across services - our use case is single-machine, short-lived operations | Use errgroup with context for simple parallel execution |
| Third-party process pools | Add dependency complexity when we just need parallel stop/start of 2-5 instances | Use errgroup.Group for simple parallel execution |
| Docker/Kubernetes | Project constraint: single Windows machine, no containerization | Use native Windows process management (already implemented) |

## Integration with Existing Codebase

### Pattern 1: Multi-Instance Stop (using errgroup)

**Why this approach:**
- errgroup provides automatic cancellation: if one instance fails to stop, we can continue stopping others
- Built-in error collection: collect all stop errors to report which instances failed
- Context integration: timeout per instance with shared deadline

**Integration points:**
```go
// Existing: lifecycle.Manager.StopForUpdate() - single instance
// New: lifecycle.MultiManager.StopAllForUpdate() - multiple instances

import "golang.org/x/sync/errgroup"

func (m *MultiManager) StopAllForUpdate(ctx context.Context) map[string]error {
    g, ctx := errgroup.WithContext(ctx)
    errors := make(map[string]error)
    var mu sync.Mutex

    for name, instance := range m.instances {
        name := name // capture loop variable
        instance := instance

        g.Go(func() error {
            err := instance.manager.StopForUpdate(ctx)
            if err != nil {
                mu.Lock()
                errors[name] = err
                mu.Unlock()
            }
            return nil // Don't fail fast - continue stopping others
        })
    }

    g.Wait() // Wait for all goroutines
    return errors
}
```

### Pattern 2: Multi-Instance State Tracking (using sync.Map)

**Why this approach:**
- sync.Map optimized for read-heavy workloads (status checks)
- No locking overhead for concurrent reads
- Go 1.24+ HashTrieMap provides lock-free reads via atomics

**Integration points:**
```go
// Existing: IsNanobotRunning(port) returns (bool, int32, string, error)
// New: MultiInstanceStatus stores state for all instances

type InstanceStatus struct {
    Name           string
    Port           uint32
    IsRunning      bool
    PID            int32
    DetectionMethod string
    LastChecked    time.Time
}

type MultiManager struct {
    instances map[string]*InstanceConfig
    status    sync.Map // map[string]*InstanceStatus - key is instance name
}

func (m *MultiManager) RefreshAllStatus(ctx context.Context) error {
    g, ctx := errgroup.WithContext(ctx)

    for name, cfg := range m.instances {
        name := name
        port := cfg.Port

        g.Go(func() error {
            running, pid, method, err := IsNanobotRunning(port)
            if err != nil {
                return fmt.Errorf("instance %s: %w", name, err)
            }

            m.status.Store(name, &InstanceStatus{
                Name:           name,
                Port:           port,
                IsRunning:      running,
                PID:            pid,
                DetectionMethod: method,
                LastChecked:    time.Now(),
            })
            return nil
        })
    }

    return g.Wait()
}

func (m *MultiManager) GetStatus(name string) (*InstanceStatus, bool) {
    if v, ok := m.status.Load(name); ok {
        return v.(*InstanceStatus), true
    }
    return nil, false
}
```

### Pattern 3: Configuration Structure Extension

**Why this approach:**
- Reuse existing viper/yaml infrastructure
- Backward compatible: single instance = default behavior
- No new dependencies

**Integration points:**
```go
// Existing: config.Config with single NanobotConfig
// New: config.Config with map of instances

type Config struct {
    Cron      string                  `yaml:"cron" mapstructure:"cron"`
    Instances map[string]NanobotConfig `yaml:"instances" mapstructure:"instances"` // NEW
    Pushover  PushoverConfig          `yaml:"pushover" mapstructure:"pushover"`
}

// defaults() migration strategy:
// If old config format (single nanobot), auto-migrate to instances["default"]
```

## Stack Patterns by Variant

**If managing 2-3 instances (most likely):**
- Use errgroup without semaphore (no concurrency limiting needed)
- Because overhead is minimal, simplicity over optimization

**If managing 5+ instances:**
- Use errgroup.WithSemaphore(n) to limit concurrent operations
- Because Windows process operations may have resource contention
- Start with limit of 5 concurrent operations

**If instances have dependencies (instance A must stop before instance B):**
- Use separate errgroup.Group for each dependency level
- Because errgroup doesn't support DAG dependencies
- NOT RECOMMENDED: Over-engineering for current requirements - keep instances independent

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| golang.org/x/sync@v0.18.0 | Go 1.24.11 (current) | Full compatibility |
| errgroup v0.14+ (2025-04) | All Go 1.24+ | Panic trapping added in v0.14 |
| sync.Map (Go 1.24+) | Go 1.24.11 | HashTrieMap optimization available |

## Sources

- [pkg.go.dev/golang.org/x/sync](https://pkg.go.dev/golang.org/x/sync) — Version v0.18.0 (Oct 2025), verified errgroup panic trapping feature — HIGH confidence
- [pkg.go.dev/golang.org/x/sync/errgroup](https://pkg.go.dev/golang.org/x/sync/errgroup) — Official API documentation — HIGH confidence
- [dev.to/jones_charles_ad50858dbc0/go-concurrency-made-easy-mastering-errgroup](https://dev.to/jones_charles_ad50858dbc0/go-concurrency-made-easy-mastering-errgroup-for-error-handling-and-task-control-219) — errgroup usage patterns for concurrent task management — MEDIUM confidence
- [victoriametrics.com/blog/go-sync-map](https://victoriametrics.com/blog/go-sync-map/) — sync.Map internals and use cases for state tracking — HIGH confidence
- [github.com/puzpuzpuz/go-concurrent-map-bench](https://github.com/puzpuzpuz/go-concurrent-map-bench) — Go 1.24 HashTrieMap backing for sync.Map — HIGH confidence
- [reddit.com/r/golang/comments/1raw8jl](https://www.reddit.com/r/golang/comments/1raw8jl/benchmarking_5_concurrent_map_implementations_in/) — sync.Map performance benchmarks — MEDIUM confidence

---
*Stack research for: Multi-instance nanobot management*
*Researched: 2026-03-09*

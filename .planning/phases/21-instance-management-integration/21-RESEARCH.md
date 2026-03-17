# Phase 21: Instance Management Integration - Research

**Researched:** 2026-03-17
**Domain:** Go instance lifecycle management with log buffer integration
**Confidence:** HIGH

## Summary

Phase 21 将 LogBuffer 集成到现有的 InstanceLifecycle 和 InstanceManager 架构中，为每个 nanobot 实例提供独立的日志缓冲能力。核心挑战是在保持现有更新流程（stop→update→start）向后兼容的同时，实现实例启动时自动创建 LogBuffer、停止时保留缓冲、重启时清空缓冲的生命周期管理。

**Primary recommendation:** 在 InstanceLifecycle 结构中添加 LogBuffer 字段，在 InstanceManager 中提供按名称访问 LogBuffer 的方法，使用 StartNanobotWithCapture 替代 StartNanobot 实现自动日志捕获。

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| INST-01 | 系统将 LogBuffer 集成到 InstanceLifecycle 结构中 | InstanceLifecycle 结构添加 logBuffer 字段，构造函数接收 LogBuffer 或创建新实例 |
| INST-02 | 系统在 InstanceManager 中管理所有实例的 LogBuffer | InstanceManager 添加 GetLogBuffer(instanceName) 方法，返回对应实例的 LogBuffer |
| INST-03 | 系统在实例启动时创建对应的 LogBuffer | StartAfterUpdate 调用 StartNanobotWithCapture(il.logBuffer) 创建并启动日志捕获 |
| INST-04 | 系统在实例停止时保留 LogBuffer (可查看历史日志) | StopForUpdate 不清空 logBuffer 字段，LogBuffer 保留在内存中可继续访问 |
| INST-05 | 系统在实例重启时清空 LogBuffer (重新开始缓冲) | StartAfterUpdate 在调用 StartNanobotWithCapture 前调用 logBuffer.Clear() 清空历史 |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| sync.RWMutex | Go stdlib | Thread-safe LogBuffer access | Project already uses RWMutex for LogBuffer (Phase 19) |
| context | Go stdlib | Goroutine lifecycle management | Project uses context for capture goroutines (Phase 20) |
| log/slog | Go stdlib | Structured logging with instance context | Project uses slog throughout for context-aware logging |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| logbuffer | Local package | Circular buffer for log entries | Each instance gets its own LogBuffer instance |
| lifecycle | Local package | Process lifecycle management | StartNanobotWithCapture for log capture integration |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| InstanceLifecycle embedding LogBuffer | Centralized map in InstanceManager | Embedding provides better encapsulation, each instance owns its buffer |
| Clear on StopForUpdate | Clear on StartAfterUpdate | Clearing on start allows viewing logs after stop, better UX |

**Installation:**
No new dependencies required. All packages are part of the project or Go standard library.

**Version verification:** Go stdlib packages are tied to Go version (currently in use). Local packages logbuffer and lifecycle are already implemented in Phases 19-20.

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── instance/
│   ├── manager.go          # Add GetLogBuffer(instanceName) method
│   ├── lifecycle.go        # Add logBuffer field to InstanceLifecycle
│   └── errors.go           # Already has InstanceError (no changes needed)
├── logbuffer/
│   ├── buffer.go           # Add Clear() method for INST-05
│   └── subscriber.go       # No changes needed
└── lifecycle/
    ├── starter.go          # StartNanobotWithCapture already implemented
    └── capture.go          # captureLogs already implemented
```

### Pattern 1: InstanceLifecycle with LogBuffer Embedding
**What:** Add LogBuffer field to InstanceLifecycle struct, initialized in constructor
**When to use:** Every instance needs independent log buffer throughout its lifecycle
**Example:**
```go
// Source: internal/instance/lifecycle.go
type InstanceLifecycle struct {
    config    config.InstanceConfig
    logger    *slog.Logger
    logBuffer *logbuffer.LogBuffer // INST-01: Add LogBuffer field
}

func NewInstanceLifecycle(cfg config.InstanceConfig, baseLogger *slog.Logger) *InstanceLifecycle {
    instanceLogger := baseLogger.With("instance", cfg.Name).With("component", "instance-lifecycle")

    // INST-03: Create LogBuffer on instance creation
    logBuffer := logbuffer.NewLogBuffer(instanceLogger)

    return &InstanceLifecycle{
        config:    cfg,
        logger:    instanceLogger,
        logBuffer: logBuffer,
    }
}
```

### Pattern 2: InstanceManager LogBuffer Access
**What:** Add GetLogBuffer method to InstanceManager to access instance buffers by name
**When to use:** HTTP API needs to retrieve LogBuffer for SSE streaming or history retrieval
**Example:**
```go
// Source: internal/instance/manager.go
// INST-02: Get LogBuffer by instance name
func (m *InstanceManager) GetLogBuffer(instanceName string) (*logbuffer.LogBuffer, error) {
    for _, inst := range m.instances {
        if inst.config.Name == instanceName {
            return inst.logBuffer, nil
        }
    }
    return nil, &InstanceError{
        InstanceName: instanceName,
        Operation:    "get_log_buffer",
        Err:          fmt.Errorf("instance not found"),
    }
}
```

### Pattern 3: Start with Capture Integration
**What:** Modify StartAfterUpdate to use StartNanobotWithCapture instead of StartNanobot
**When to use:** Every instance start should capture logs automatically
**Example:**
```go
// Source: internal/instance/lifecycle.go
func (il *InstanceLifecycle) StartAfterUpdate(ctx context.Context) error {
    il.logger.Info("Starting instance after update")

    // INST-05: Clear LogBuffer on restart (fresh start)
    il.logBuffer.Clear()

    startupTimeout := il.config.StartupTimeout
    if startupTimeout == 0 {
        startupTimeout = 30 * time.Second
        il.logger.Debug("Using default startup timeout", "timeout", startupTimeout)
    }

    // INST-03: Use StartNanobotWithCapture with instance's LogBuffer
    if err := lifecycle.StartNanobotWithCapture(
        ctx,
        il.config.StartCommand,
        il.config.Port,
        startupTimeout,
        il.logger,
        il.logBuffer, // Pass instance's LogBuffer
    ); err != nil {
        il.logger.Error("Failed to start instance", "error", err)
        return &InstanceError{
            InstanceName: il.config.Name,
            Operation:    "start",
            Port:         il.config.Port,
            Err:          fmt.Errorf("failed to start instance: %w", err),
        }
    }

    il.logger.Info("Instance started successfully with log capture")
    return nil
}
```

### Pattern 4: LogBuffer Clear Method
**What:** Add Clear method to LogBuffer to reset buffer for instance restart
**When to use:** Instance restart (INST-05) needs fresh buffer
**Example:**
```go
// Source: internal/logbuffer/buffer.go
// INST-05: Clear buffer for instance restart
func (lb *LogBuffer) Clear() {
    lb.mu.Lock()
    defer lb.mu.Unlock()

    // Reset circular buffer
    lb.head = 0
    lb.size = 0
    lb.entries = [5000]LogEntry{} // Clear all entries

    // Note: Subscribers continue to receive new logs after clear
    // They will see the restart as a gap in log sequence
    lb.logger.Debug("LogBuffer cleared for instance restart")
}
```

### Anti-Patterns to Avoid
- **Clear on StopForUpdate:** Breaks INST-04 requirement, users can't view logs after stop
- **Shared LogBuffer in InstanceManager:** Breaks instance isolation, all instances would share one buffer
- **Not clearing on restart:** Breaks INST-05 requirement, old logs would persist across restarts
- **Creating new LogBuffer on each start:** Breaks INST-04, buffer reference would change and historical access would be lost

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| LogBuffer lifecycle | Custom buffer management in InstanceManager | Embed in InstanceLifecycle | Better encapsulation, instance owns its buffer |
| Instance name lookup | Linear search in GetLogBuffer | Linear search (instances typically < 10) | Simple, performant for small instance count |
| Buffer clear logic | Manual entry zeroing | Add Clear() method to LogBuffer | Encapsulates buffer state management |

**Key insight:** Instance count is typically small (< 10 instances based on config examples), so linear search in GetLogBuffer is acceptable. If instance count grows significantly in future, could add name→lifecycle map for O(1) lookup.

## Common Pitfalls

### Pitfall 1: Clearing LogBuffer on Stop instead of Start
**What goes wrong:** Users can't view historical logs after instance stops (violates INST-04)
**Why it happens:** Developer thinks "stop = cleanup" and clears buffer
**How to avoid:** Only clear LogBuffer in StartAfterUpdate, not in StopForUpdate
**Warning signs:** Tests show empty buffer after stop operation

### Pitfall 2: Not passing LogBuffer to StartNanobotWithCapture
**What goes wrong:** Logs not captured, or captured to wrong buffer
**Why it happens:** Forgetting to add logBuffer parameter when calling StartNanobotWithCapture
**How to avoid:** StartNanobotWithCapture signature requires LogBuffer parameter, compiler will catch missing argument
**Warning signs:** Build fails with "not enough arguments in call to StartNanobotWithCapture"

### Pitfall 3: Creating new LogBuffer on each start
**What goes wrong:** Historical logs lost, buffer reference changes, subscribers disconnected
**Why it happens:** Creating new LogBuffer in StartAfterUpdate instead of reusing existing one
**How to avoid:** InstanceLifecycle owns single LogBuffer instance, only clear it, never replace it
**Warning signs:** Tests show empty history after restart when logs should exist from before stop

### Pitfall 4: Missing Clear method in LogBuffer
**What goes wrong:** Cannot implement INST-05 (clear on restart)
**Why it happens:** LogBuffer from Phase 19 doesn't have Clear method
**How to avoid:** Add Clear() method to LogBuffer package in this phase
**Warning signs:** Cannot call il.logBuffer.Clear() in StartAfterUpdate

## Code Examples

### InstanceLifecycle with LogBuffer (Complete)
```go
// Source: internal/instance/lifecycle.go
package instance

import (
    "context"
    "fmt"
    "log/slog"
    "time"

    "github.com/HQGroup/nanobot-auto-updater/internal/config"
    "github.com/HQGroup/nanobot-auto-updater/internal/lifecycle"
    "github.com/HQGroup/nanobot-auto-updater/internal/logbuffer"
)

// InstanceLifecycle wraps lifecycle operations with instance-specific context.
type InstanceLifecycle struct {
    config    config.InstanceConfig
    logger    *slog.Logger
    logBuffer *logbuffer.LogBuffer // INST-01: LogBuffer for this instance
}

// NewInstanceLifecycle creates an instance lifecycle manager with LogBuffer.
func NewInstanceLifecycle(cfg config.InstanceConfig, baseLogger *slog.Logger) *InstanceLifecycle {
    instanceLogger := baseLogger.With("instance", cfg.Name).With("component", "instance-lifecycle")

    // INST-03: Create LogBuffer for this instance
    logBuffer := logbuffer.NewLogBuffer(instanceLogger)

    return &InstanceLifecycle{
        config:    cfg,
        logger:    instanceLogger,
        logBuffer: logBuffer,
    }
}

// StopForUpdate stops the instance before update.
// INST-04: LogBuffer is preserved (not cleared) on stop.
func (il *InstanceLifecycle) StopForUpdate(ctx context.Context) error {
    il.logger.Info("Starting stop-before-update process")
    // ... existing stop logic unchanged ...
    // Note: il.logBuffer remains intact, users can still access historical logs
    return nil
}

// StartAfterUpdate starts the instance after update with log capture.
// INST-03: Uses StartNanobotWithCapture with instance's LogBuffer.
// INST-05: Clears LogBuffer before start (restart = fresh start).
func (il *InstanceLifecycle) StartAfterUpdate(ctx context.Context) error {
    il.logger.Info("Starting instance after update")

    // INST-05: Clear LogBuffer on restart
    il.logBuffer.Clear()

    startupTimeout := il.config.StartupTimeout
    if startupTimeout == 0 {
        startupTimeout = 30 * time.Second
    }

    // INST-03: Use StartNanobotWithCapture with instance's LogBuffer
    if err := lifecycle.StartNanobotWithCapture(
        ctx,
        il.config.StartCommand,
        il.config.Port,
        startupTimeout,
        il.logger,
        il.logBuffer, // Pass instance's LogBuffer
    ); err != nil {
        return &InstanceError{
            InstanceName: il.config.Name,
            Operation:    "start",
            Port:         il.config.Port,
            Err:          fmt.Errorf("failed to start instance: %w", err),
        }
    }

    il.logger.Info("Instance started successfully with log capture")
    return nil
}

// GetLogBuffer returns the instance's LogBuffer.
// INST-02: Used by InstanceManager to expose buffer access.
func (il *InstanceLifecycle) GetLogBuffer() *logbuffer.LogBuffer {
    return il.logBuffer
}
```

### InstanceManager GetLogBuffer Method
```go
// Source: internal/instance/manager.go
// INST-02: Get LogBuffer by instance name
func (m *InstanceManager) GetLogBuffer(instanceName string) (*logbuffer.LogBuffer, error) {
    for _, inst := range m.instances {
        if inst.config.Name == instanceName {
            return inst.GetLogBuffer(), nil
        }
    }
    return nil, &InstanceError{
        InstanceName: instanceName,
        Operation:    "get_log_buffer",
        Err:          fmt.Errorf("instance not found"),
    }
}
```

### LogBuffer Clear Method
```go
// Source: internal/logbuffer/buffer.go
// INST-05: Clear buffer for instance restart
func (lb *LogBuffer) Clear() {
    lb.mu.Lock()
    defer lb.mu.Unlock()

    // Reset circular buffer state
    lb.head = 0
    lb.size = 0
    lb.entries = [5000]LogEntry{}

    lb.logger.Debug("LogBuffer cleared")
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| StartNanobot without capture | StartNanobotWithCapture with LogBuffer | Phase 20 | Logs now captured to buffer for SSE streaming |
| No log persistence during lifecycle | LogBuffer persists across stop/start | Phase 21 (this phase) | Users can view logs after instance stop |

**Deprecated/outdated:**
- None in this phase. This phase builds on Phase 19 (LogBuffer) and Phase 20 (StartNanobotWithCapture).

## Open Questions

1. **Should Clear() notify subscribers?**
   - What we know: Subscribers receive logs via channel, Clear resets buffer state
   - What's unclear: Should we send a special "buffer cleared" event to subscribers?
   - Recommendation: No special notification needed. Subscribers will see a gap in log sequence after restart. Phase 22 (SSE) can add metadata if needed.

2. **What if instance start fails after Clear()?**
   - What we know: Clear() is called before StartNanobotWithCapture
   - What's unclear: If start fails, buffer is empty but instance state is "stopped"
   - Recommendation: Acceptable. Buffer is empty because instance didn't run. Next successful start will populate buffer. No need to restore old logs.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing package (stdlib) |
| Config file | none - tests self-contained |
| Quick run command | `go test ./internal/instance -run TestInstanceLifecycle -v` |
| Full suite command | `go test ./internal/instance -v` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| INST-01 | InstanceLifecycle contains LogBuffer field | unit | `go test ./internal/instance -run TestNewInstanceLifecycle_LogBuffer -v` | ❌ Wave 0 |
| INST-02 | InstanceManager.GetLogBuffer returns correct buffer | unit | `go test ./internal/instance -run TestInstanceManager_GetLogBuffer -v` | ❌ Wave 0 |
| INST-03 | StartAfterUpdate calls StartNanobotWithCapture | unit | `go test ./internal/instance -run TestInstanceLifecycle_StartWithCapture -v` | ❌ Wave 0 |
| INST-04 | StopForUpdate preserves LogBuffer content | unit | `go test ./internal/instance -run TestInstanceLifecycle_StopPreservesBuffer -v` | ❌ Wave 0 |
| INST-05 | StartAfterUpdate clears LogBuffer before start | unit | `go test ./internal/instance -run TestInstanceLifecycle_StartClearsBuffer -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/instance -run <specific_test> -v`
- **Per wave merge:** `go test ./internal/instance ./internal/logbuffer -v`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/instance/lifecycle_test.go` — add tests for INST-01, INST-03, INST-04, INST-05
- [ ] `internal/instance/manager_test.go` — add test for INST-02 (GetLogBuffer)
- [ ] `internal/logbuffer/buffer_test.go` — add test for Clear() method (INST-05 support)
- [ ] Framework install: none required — Go testing package already in use

## Sources

### Primary (HIGH confidence)
- Phase 19 implementation: internal/logbuffer/buffer.go, internal/logbuffer/subscriber.go
- Phase 20 implementation: internal/lifecycle/starter.go (StartNanobotWithCapture), internal/lifecycle/capture.go
- Existing architecture: internal/instance/manager.go, internal/instance/lifecycle.go

### Secondary (MEDIUM confidence)
- Phase 20 research: .planning/phases/20-log-capture-integration/20-RESEARCH.md
- Phase 20 summary: .planning/phases/20-log-capture-integration/20-02-SUMMARY.md

### Tertiary (LOW confidence)
- None - all research based on verified project implementation

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All packages already implemented in project (Phase 19, 20) or Go stdlib
- Architecture: HIGH - InstanceLifecycle and InstanceManager patterns already established (Phase 7-10)
- Pitfalls: HIGH - Based on direct requirement analysis and existing codebase understanding

**Research date:** 2026-03-17
**Valid until:** 30 days (stable architecture, no external dependencies)

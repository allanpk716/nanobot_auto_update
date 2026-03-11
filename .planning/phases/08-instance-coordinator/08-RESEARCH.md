# Phase 8: 实例协调器 - Research

**Researched:** 2026-03-11
**Domain:** Go coordinator pattern, error aggregation, graceful degradation
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### 协调器结构设计
- **包位置**: 创建 `internal/instance/manager.go` 文件
- **结构体**:
  ```go
  type InstanceManager struct {
      instances []*InstanceLifecycle
      logger    *slog.Logger
  }
  ```
- **构造函数**: `NewInstanceManager(config *config.Config, baseLogger *slog.Logger) *InstanceManager`
- 构造时遍历 config.Instances,为每个实例创建 InstanceLifecycle 包装器并注入 logger 上下文
- **生命周期管理**: InstanceManager 持有所有实例的 InstanceLifecycle 对象,负责协调单个实例的生命周期

#### 协调器方法接口
- **单一方法**: `UpdateAll(ctx context.Context) (*UpdateResult, error)` 执行完整更新流程
  - 停止所有 → UV 更新 → 启动所有
  - 内部调用 StopAll()、performUpdate()、StartAll() 私有方法
  - 返回 UpdateResult 结构体和包含成功/失败详情
- **分步方法**(可选): 可以添加 StopAll()、PerformUpdate()、StartAll() 公共方法,便于测试和
  - 如果不存在,使用单一 UpdateAll() 方法即可
- **错误处理**: 使用自定义 UpdateError 收集多个 InstanceError
  - UpdateError 完整流程失败时返回 error
  - 部分失败时返回 UpdateResult + error
  - 不返回 error

#### 优雅降级策略
- **停止失败**: 记录错误
  - **跳过 UV 更新** - 避免文件冲突
  - 继续停止其他实例
  - 返回 UpdateResult 标记停止失败
- **启动失败**: 记录错误
  - **继续启动其他实例** - 最大化服务可用性
  - 返回 UpdateResult 标记启动失败
- **错误继续**: 不立即中止流程,继续尝试启动其他实例

#### 错误聚合格式
- **UpdateResult 结构**:
  ```go
  type UpdateResult struct {
      Stopped    []string // 成功停止的实例名称列表
      Started    []string // 成功启动的实例名称列表
      StopFailed  []*InstanceError // 停止失败的实例错误
      StartFailed []*InstanceError // 启动失败的实例错误
  }
  ```
- **UpdateError 结构**:
  ```go
  type UpdateError struct {
      Errors []*InstanceError
  }

  func (e *UpdateError) Error() string {
      successCount := len(e.Errors)
      totalCount := len(e.Errors) + successCount
      msg := fmt.Sprintf("更新失败 (%d/%d 实例成功):\n", successCount, totalCount)
      for _, err := range e.Errors {
          msg += fmt.Sprintf("  ✗ %s\n", err.InstanceName)
      }
      return msg
  }

  func (e *UpdateError) Unwrap() []error {
      return e.Errors
  }
  ```
- **复用 InstanceError**: Phase 7 的 InstanceError 包含 InstanceName, Operation, Port, Err 字段
  - UpdateResult 直接使用 `[]*InstanceError` 存储失败实例
  - UpdateError 复合 InstanceError 列表
  - 无需额外封装

### Claude's Discretion
- InstanceManager 的具体命名(如 InstanceCoordinator vs InstanceSupervisor)
- UpdateError 的 Error() 方法实现细节(如使用图标、 实例名称对齐)
- 是否添加分步方法(StopAll/StartAll)的决策

### Deferred Ideas (OUT OF SCOPE)
None — 讨论保持在阶段范围内

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| LIFECYCLE-01 | 按顺序停止所有配置的实例(串行执行) | 使用 InstanceManager.StopAll() 串行调用每个 InstanceLifecycle.StopForUpdate() |
| LIFECYCLE-02 | 按顺序启动所有配置的实例(串行执行) | 使用 InstanceManager.StartAll() 串行调用每个 InstanceLifecycle.StartAfterUpdate() |
| LIFECYCLE-03 | 优雅降级 - 某个实例失败时继续操作其他实例 | 错误聚合模式 + 不提前返回,继续处理其他实例 |
| ERROR-02 | 错误聚合 - 收集所有实例错误,不丢失任何失败信息 | UpdateResult 结构体 + InstanceError 列表 + errors.Join 模式 |

</phase_requirements>

## Summary

Phase 8 需要实现 InstanceManager 协调器,负责编排所有实例的停止→更新→启动流程。核心挑战在于错误聚合和优雅降级:当某个实例操作失败时,系统应记录错误但继续处理其他实例,最终返回包含所有成功/失败状态的 UpdateResult。

**Primary recommendation:** 使用 Go 标准的 errors.Join 聚合错误,结合项目已有的 InstanceError 自定义错误类型,实现 UpdateResult 结构体来清晰分离成功/失败状态。串行执行所有操作以简化实现和调试。

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go 标准库 errors | 1.20+ | errors.Join 聚合多个错误 | Go 1.20 引入的标准错误聚合方案,支持错误链遍历 |
| Go 标准库 context | 1.20+ | 上下文传递和取消控制 | 协调器操作需要支持上下文取消 |
| Go 标准库 log/slog | 1.21+ | 结构化日志记录 | 项目统一使用 slog 注入上下文日志 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/WQGroup/logger | - | 项目日志库 | 在 main.go 中初始化 logger,传递给 InstanceManager |
| internal/instance/lifecycle | Phase 7 | 实例生命周期包装器 | 每个实例的 StopForUpdate/StartAfterUpdate 方法 |
| internal/instance/errors | Phase 7 | InstanceError 错误类型 | 复用已有的实例错误封装 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| errors.Join | 第三方库 (hashicorp/go-multierror) | Go 标准库更简洁,无需引入额外依赖 |
| 串行执行 | 并发执行 (goroutines) | 串行更简单,便于调试和日志追踪;并发实现复杂度高,需要错误同步 |
| UpdateResult 结构 | 仅返回 error | UpdateResult 清晰分离成功/失败状态,便于 Phase 9 构建通知 |

**Installation:**
无新依赖需要安装,全部使用 Go 标准库和项目已有代码。

## Architecture Patterns

### Recommended Project Structure
```
internal/instance/
├── errors.go           # InstanceError (Phase 7 已完成)
├── lifecycle.go        # InstanceLifecycle (Phase 7 已完成)
├── manager.go          # InstanceManager (Phase 8 新增)
├── manager_test.go     # InstanceManager 单元测试 (Wave 0)
└── result.go           # UpdateResult 和 UpdateError (Phase 8 新增)
```

### Pattern 1: Coordinator Pattern (协调器模式)

**What:** InstanceManager 作为协调器,管理多个 InstanceLifecycle 对象的协调操作,但不实现具体的停止/启动逻辑。

**When to use:** 当需要对多个对象执行批量操作,且需要优雅降级和错误聚合时。

**Example:**
```go
// Source: CONTEXT.md + 项目架构模式
type InstanceManager struct {
    instances []*InstanceLifecycle
    logger    *slog.Logger
}

func NewInstanceManager(config *config.Config, baseLogger *slog.Logger) *InstanceManager {
    logger := baseLogger.With("component", "instance-manager")

    instances := make([]*InstanceLifecycle, 0, len(config.Instances))
    for _, cfg := range config.Instances {
        lifecycle := NewInstanceLifecycle(cfg, baseLogger)
        instances = append(instances, lifecycle)
    }

    return &InstanceManager{
        instances: instances,
        logger:    logger,
    }
}

func (m *InstanceManager) UpdateAll(ctx context.Context) (*UpdateResult, error) {
    result := &UpdateResult{}

    // Step 1: Stop all instances
    m.stopAll(ctx, result)

    // Step 2: Perform UV update (skip if any stop failed)
    if len(result.StopFailed) > 0 {
        m.logger.Warn("Skipping UV update due to stop failures",
            "failed_count", len(result.StopFailed))
        // Jump to start phase to recover instances
    } else {
        if err := m.performUpdate(ctx); err != nil {
            return result, fmt.Errorf("UV update failed: %w", err)
        }
    }

    // Step 3: Start all instances
    m.startAll(ctx, result)

    return result, nil
}
```

### Pattern 2: Error Aggregation Pattern (错误聚合模式)

**What:** 收集所有操作中的错误,不提前返回,最终返回结构化的错误聚合结果。

**When to use:** 批量操作需要"继续处理其他项"的场景,避免静默失败。

**Example:**
```go
// Source: Go 1.20+ errors.Join 最佳实践
// 参考: https://oscarchou.com/posts/golang/handle-multiple-errors-in-go/

func (m *InstanceManager) stopAll(ctx context.Context, result *UpdateResult) {
    m.logger.Info("Stopping all instances", "total_count", len(m.instances))

    for _, inst := range m.instances {
        if err := inst.StopForUpdate(ctx); err != nil {
            m.logger.Error("Failed to stop instance",
                "instance", inst.config.Name,
                "error", err)
            result.StopFailed = append(result.StopFailed, err)
        } else {
            result.Stopped = append(result.Stopped, inst.config.Name)
        }
    }

    m.logger.Info("Stop phase completed",
        "success_count", len(result.Stopped),
        "failed_count", len(result.StopFailed))
}
```

### Pattern 3: Result Struct Pattern (结果结构模式)

**What:** 使用结构体清晰分离成功/失败状态,而不是仅返回 error。

**When to use:** 批量操作需要详细报告哪些成功、哪些失败时。

**Example:**
```go
// Source: CONTEXT.md 决策
type UpdateResult struct {
    Stopped     []string         // 成功停止的实例名称
    Started     []string         // 成功启动的实例名称
    StopFailed  []*InstanceError // 停止失败的实例错误
    StartFailed []*InstanceError // 启动失败的实例错误
}

// UpdateError 实现 error 接口,用于完整流程失败时的错误报告
type UpdateError struct {
    Errors []*InstanceError
}

func (e *UpdateError) Error() string {
    if len(e.Errors) == 0 {
        return "更新失败: 无错误详情"
    }

    var msg strings.Builder
    msg.WriteString(fmt.Sprintf("更新失败 (%d 个实例失败):\n", len(e.Errors)))
    for _, err := range e.Errors {
        msg.WriteString(fmt.Sprintf("  ✗ %s\n", err.InstanceName))
    }
    return msg.String()
}

// Unwrap 返回错误列表,支持 errors.Is/As 遍历
func (e *UpdateError) Unwrap() []error {
    errs := make([]error, len(e.Errors))
    for i, err := range e.Errors {
        errs[i] = err
    }
    return errs
}
```

### Anti-Patterns to Avoid

- **Anti-pattern 1: 提前返回错误**
  - 错误: `if err := inst.Stop(); err != nil { return err }` (停止在第一个失败)
  - 正确: 记录错误继续处理其他实例,最终返回 UpdateResult

- **Anti-pattern 2: 忽略错误**
  - 错误: `inst.Stop()` (不检查错误)
  - 正确: 检查错误并记录到 UpdateResult

- **Anti-pattern 3: 并发执行未考虑日志混乱**
  - 错误: 使用 goroutines 并发停止实例,日志输出混乱难以调试
  - 正确: 串行执行,日志清晰可追踪

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 错误聚合 | 自定义 MultiError 结构体 | Go 标准库 errors.Join | 标准库方案,支持错误链,无需额外依赖 |
| 实例生命周期 | 在 InstanceManager 中实现停止/启动逻辑 | InstanceLifecycle 包装器 | 已有 Phase 7 实现,复用避免重复代码 |
| 日志上下文 | 每次手动传递 instance 名称 | logger.With() 预注入字段 | 项目统一模式,所有日志自动包含上下文 |

**Key insight:** InstanceManager 的职责是"协调"而非"实现",具体操作委托给 InstanceLifecycle。错误聚合使用 Go 标准库而非自定义实现。

## Common Pitfalls

### Pitfall 1: 停止失败后仍执行 UV 更新

**What goes wrong:** 某个实例停止失败(进程仍占用端口/文件),但系统继续执行 UV 更新,导致文件冲突或更新失败。

**Why it happens:** 没有检查 StopFailed 数组就直接执行更新。

**How to avoid:**
```go
func (m *InstanceManager) UpdateAll(ctx context.Context) (*UpdateResult, error) {
    result := &UpdateResult{}

    // Stop all instances
    m.stopAll(ctx, result)

    // CRITICAL: Check stop failures before update
    if len(result.StopFailed) > 0 {
        m.logger.Warn("Skipping UV update due to stop failures")
        // Jump to start phase to recover instances
    } else {
        if err := m.performUpdate(ctx); err != nil {
            return result, fmt.Errorf("UV update failed: %w", err)
        }
    }

    // Start all instances
    m.startAll(ctx, result)

    return result, nil
}
```

**Warning signs:**
- UV 更新失败提示"文件被占用"
- 实例启动失败提示"端口已被使用"

### Pitfall 2: UpdateError 的 Error() 方法计算错误

**What goes wrong:** Error() 方法中的 successCount 计算逻辑错误,导致错误消息不准确。

**Why it happens:** 直接使用 `len(e.Errors)` 作为 successCount,混淆了失败数量和成功数量。

**How to avoid:**
```go
func (e *UpdateError) Error() string {
    // WRONG: successCount := len(e.Errors)
    // RIGHT: 需要传入总实例数才能计算成功数量
    msg := fmt.Sprintf("更新失败 (%d 个实例失败):\n", len(e.Errors))
    for _, err := range e.Errors {
        msg += fmt.Sprintf("  ✗ %s\n", err.InstanceName)
    }
    return msg
}
```

**Warning signs:**
- 错误消息显示"0/0 实例成功"
- 成功数量和失败数量不匹配总实例数

### Pitfall 3: Unwrap() 返回类型错误

**What goes wrong:** UpdateError.Unwrap() 返回 `[]*InstanceError` 而非 `[]error`,导致 errors.Is/As 无法正常工作。

**Why it happens:** Go 的 errors.Is/As 需要 `[]error` 类型,而非具体类型切片。

**How to avoid:**
```go
// WRONG
func (e *UpdateError) Unwrap() []*InstanceError {
    return e.Errors
}

// RIGHT
func (e *UpdateError) Unwrap() []error {
    errs := make([]error, len(e.Errors))
    for i, err := range e.Errors {
        errs[i] = err
    }
    return errs
}
```

**Warning signs:**
- `errors.As(updateErr, &instanceErr)` 总是返回 false
- 无法从 UpdateError 中提取具体的 InstanceError

### Pitfall 4: 日志丢失实例上下文

**What goes wrong:** InstanceManager 的日志没有包含 instance 字段,导致无法追踪哪个实例的操作。

**Why it happens:** InstanceManager.logger 没有预注入 component 字段,或者调用 instance 方法时没有使用 instance 的 logger。

**How to avoid:**
```go
func NewInstanceManager(config *config.Config, baseLogger *slog.Logger) *InstanceManager {
    // 注入 component 字段
    logger := baseLogger.With("component", "instance-manager")

    // InstanceLifecycle 会自动注入 instance 字段
    instances := make([]*InstanceLifecycle, 0, len(config.Instances))
    for _, cfg := range config.Instances {
        lifecycle := NewInstanceLifecycle(cfg, baseLogger)
        instances = append(instances, lifecycle)
    }

    return &InstanceManager{
        instances: instances,
        logger:    logger,
    }
}
```

**Warning signs:**
- 日志输出缺少 `instance=xxx` 字段
- 无法从日志中定位具体实例的操作

## Code Examples

Verified patterns from official sources and project context:

### 协调器主方法 (UpdateAll)

```go
// Source: CONTEXT.md 决策 + Go 标准模式
func (m *InstanceManager) UpdateAll(ctx context.Context) (*UpdateResult, error) {
    m.logger.Info("Starting full update process", "instance_count", len(m.instances))

    result := &UpdateResult{}

    // Phase 1: Stop all instances (graceful degradation)
    m.stopAll(ctx, result)

    // Phase 2: UV update (skip if any instance failed to stop)
    if len(result.StopFailed) > 0 {
        m.logger.Warn("Skipping UV update due to stop failures",
            "failed_count", len(result.StopFailed),
            "failed_instances", extractNames(result.StopFailed))
    } else {
        if err := m.performUpdate(ctx); err != nil {
            // Critical failure: UV update failed
            m.logger.Error("UV update failed, cannot start instances", "error", err)
            return result, fmt.Errorf("UV update failed: %w", err)
        }
    }

    // Phase 3: Start all instances (graceful degradation)
    m.startAll(ctx, result)

    // Log final result
    m.logger.Info("Update process completed",
        "stopped_success", len(result.Stopped),
        "stopped_failed", len(result.StopFailed),
        "started_success", len(result.Started),
        "started_failed", len(result.StartFailed))

    return result, nil
}

// extractNames 辅助函数,从 InstanceError 中提取实例名称
func extractNames(errs []*InstanceError) []string {
    names := make([]string, len(errs))
    for i, err := range errs {
        names[i] = err.InstanceName
    }
    return names
}
```

### 停止所有实例 (stopAll)

```go
// Source: 错误聚合最佳实践
func (m *InstanceManager) stopAll(ctx context.Context, result *UpdateResult) {
    m.logger.Info("Starting stop phase", "instance_count", len(m.instances))

    for _, inst := range m.instances {
        // 每个实例的日志已预注入 instance 和 component 字段
        if err := inst.StopForUpdate(ctx); err != nil {
            m.logger.Error("Failed to stop instance",
                "error", err,
                "port", inst.config.Port)

            // 记录失败但不返回,继续停止其他实例
            result.StopFailed = append(result.StopFailed, err)
        } else {
            result.Stopped = append(result.Stopped, inst.config.Name)
        }
    }

    m.logger.Info("Stop phase completed",
        "success", len(result.Stopped),
        "failed", len(result.StopFailed))
}
```

### 启动所有实例 (startAll)

```go
func (m *InstanceManager) startAll(ctx context.Context, result *UpdateResult) {
    m.logger.Info("Starting start phase", "instance_count", len(m.instances))

    for _, inst := range m.instances {
        if err := inst.StartAfterUpdate(ctx); err != nil {
            m.logger.Error("Failed to start instance",
                "error", err,
                "port", inst.config.Port)

            // 记录失败但不返回,继续启动其他实例
            result.StartFailed = append(result.StartFailed, err)
        } else {
            result.Started = append(result.Started, inst.config.Name)
        }
    }

    m.logger.Info("Start phase completed",
        "success", len(result.Started),
        "failed", len(result.StartFailed))
}
```

### 执行 UV 更新 (performUpdate)

```go
func (m *InstanceManager) performUpdate(ctx context.Context) error {
    m.logger.Info("Starting UV update")

    // 复用 Phase 2 的 Updater 结构
    updater := NewUpdater(m.logger)

    updateResult, err := updater.Update(ctx)
    if err != nil {
        m.logger.Error("UV update failed", "error", err)
        return err
    }

    m.logger.Info("UV update completed successfully", "result", updateResult)
    return nil
}
```

### UpdateResult 和 UpdateError 结构

```go
// Source: CONTEXT.md + Go 标准错误模式

// UpdateResult 包含更新流程的所有结果
type UpdateResult struct {
    Stopped     []string         `json:"stopped"`      // 成功停止的实例名称
    Started     []string         `json:"started"`      // 成功启动的实例名称
    StopFailed  []*InstanceError `json:"stop_failed"`  // 停止失败的实例错误
    StartFailed []*InstanceError `json:"start_failed"` // 启动失败的实例错误
}

// HasErrors 检查是否有任何失败
func (r *UpdateResult) HasErrors() bool {
    return len(r.StopFailed) > 0 || len(r.StartFailed) > 0
}

// UpdateError 聚合所有实例错误
type UpdateError struct {
    Errors []*InstanceError
}

func (e *UpdateError) Error() string {
    if len(e.Errors) == 0 {
        return "更新失败: 无错误详情"
    }

    var msg strings.Builder
    msg.WriteString(fmt.Sprintf("更新失败 (%d 个实例失败):\n", len(e.Errors)))
    for _, err := range e.Errors {
        msg.WriteString(fmt.Sprintf("  ✗ %s\n", err.InstanceName))
    }
    return msg.String()
}

// Unwrap 返回错误列表,支持 errors.Is/As 遍历
func (e *UpdateError) Unwrap() []error {
    errs := make([]error, len(e.Errors))
    for i, err := range e.Errors {
        errs[i] = err
    }
    return errs
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| 返回第一个错误 | 错误聚合 (errors.Join) | Go 1.20 (2023) | 批量操作不再静默失败,完整报告所有错误 |
| 自定义 MultiError 库 | Go 标准库 errors.Join | Go 1.20 (2023) | 减少第三方依赖,统一错误处理模式 |
| 仅返回 error | 返回 Result + error | 项目 Phase 8 (2026) | 清晰分离成功/失败状态,便于通知和日志 |

**Deprecated/outdated:**
- **hashicorp/go-multierror**: Go 1.20+ 不再需要,标准库 errors.Join 已足够
- **提前返回错误**: 批量操作应收集所有错误,而非在第一个失败时返回

## Open Questions

1. **UpdateAll() 是否需要返回 error?**
   - What we know: CONTEXT.md 提到"部分失败时返回 UpdateResult + error"
   - What's unclear: 完全成功时是否也返回 error? 还是仅在 UV 更新失败时返回 error?
   - Recommendation: 仅在 UV 更新失败(关键错误)时返回 error,实例部分失败通过 UpdateResult.HasErrors() 判断

2. **是否添加分步公共方法?**
   - What we know: CONTEXT.md 标记为"Claude's Discretion"
   - What's unclear: 是否需要暴露 StopAll()/StartAll() 公共方法?
   - Recommendation: 优先实现私有方法,如果测试需要再提升为公共方法

3. **UpdateError 的 Error() 方法格式**
   - What we know: CONTEXT.md 提供了基本格式,包含 ✗ 图标
   - What's unclear: 是否需要对齐实例名称? 是否需要包含端口信息?
   - Recommendation: 简单格式优先,仅包含实例名称,Phase 9 通知可能需要更详细格式

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (标准库) |
| Config file | none — 使用 \*_test.go 文件 |
| Quick run command | `go test ./internal/instance -v -run TestInstanceManager` |
| Full suite command | `go test ./internal/instance -v -cover` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| LIFECYCLE-01 | 按顺序停止所有实例 | unit | `go test ./internal/instance -v -run TestStopAll` | ❌ Wave 0 |
| LIFECYCLE-02 | 按顺序启动所有实例 | unit | `go test ./internal/instance -v -run TestStartAll` | ❌ Wave 0 |
| LIFECYCLE-03 | 优雅降级 - 继续操作其他实例 | unit | `go test ./internal/instance -v -run TestGracefulDegradation` | ❌ Wave 0 |
| ERROR-02 | 错误聚合 - 收集所有失败信息 | unit | `go test ./internal/instance -v -run TestUpdateResult` | ❌ Wave 0 |
| Success-01 | 停止失败跳过 UV 更新 | unit | `go test ./internal/instance -v -run TestStopFailureSkipUpdate` | ❌ Wave 0 |
| Success-06 | 收集所有操作结果 | integration | `go test ./internal/instance -v -run TestUpdateAllIntegration` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/instance -v -run <specific-test>`
- **Per wave merge:** `go test ./internal/instance -v -cover`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/instance/manager.go` — InstanceManager 实现
- [ ] `internal/instance/manager_test.go` — InstanceManager 单元测试
- [ ] `internal/instance/result.go` — UpdateResult 和 UpdateError 结构
- [ ] `internal/instance/result_test.go` — UpdateResult 和 UpdateError 单元测试
- [ ] Mock InstanceLifecycle — 创建 mock 对象用于测试(避免依赖真实进程)

**Note:** Wave 0 需要创建完整的测试套件,优先实现 manager.go 和 result.go,再补充测试

## Sources

### Primary (HIGH confidence)
- [Go 1.20 Release Notes - errors.Join](https://go.dev/doc/go1.20#errors) - 官方错误聚合功能介绍
- [CONTEXT.md](.planning/phases/08-instance-coordinator/08-CONTEXT.md) - Phase 8 用户决策
- [internal/instance/lifecycle.go](internal/instance/lifecycle.go) - Phase 7 已实现的生命周期包装器
- [internal/instance/errors.go](internal/instance/errors.go) - Phase 7 已实现的错误类型

### Secondary (MEDIUM confidence)
- [The Pragmatic Way to Handle Multiple Errors in Go](https://oscarchou.com/posts/golang/handle-multiple-errors-in-go/) - errors.Join 实践指南 (2024)
- [Best Practices for Secure Error Handling in Go](https://blog.jetbrains.com/go/2026/03/02/secure-go-error-handling-best-practices/) - JetBrains 官方错误处理最佳实践 (2026)
- [Error-Handling Patterns in Go Every Developer Should Know](https://medium.com/@virtualik/error-handling-patterns-in-go-every-developer-should-know-8962777c935b) - Go 错误处理模式总结

### Tertiary (LOW confidence)
- [7 Powerful Golang Concurrency Patterns](https://cristiancurteanu.com/7-powerful-golang-concurrency-patterns-that-will-transform-your-code-in-2025/) - 并发模式参考 (串行 vs 并发决策)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - 全部使用 Go 标准库,无第三方依赖
- Architecture: HIGH - 基于项目已有模式 (InstanceError, InstanceLifecycle),协调器模式清晰
- Pitfalls: HIGH - 基于项目 CONTEXT.md 决策和 Go 错误处理最佳实践

**Research date:** 2026-03-11
**Valid until:** 2026-04-11 (Go 标准库稳定,长期有效)

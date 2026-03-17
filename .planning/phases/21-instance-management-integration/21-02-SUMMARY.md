---
phase: 21-instance-management-integration
plan: 02
subsystem: instance-management
tags: [tdd, logbuffer, lifecycle, integration]
requires: [21-01]
provides:
  - InstanceLifecycle.logBuffer field
  - InstanceLifecycle.GetLogBuffer() method
  - InstanceManager.GetLogBuffer(instanceName) method
  - StartAfterUpdate with LogBuffer integration
affects:
  - internal/instance/lifecycle.go
  - internal/instance/manager.go
  - internal/instance/lifecycle_test.go
  - internal/instance/manager_test.go
tech-stack:
  added:
    - LogBuffer integration in InstanceLifecycle
    - StartNanobotWithCapture usage in instance lifecycle
  patterns:
    - TDD workflow (RED-GREEN-REFACTOR)
    - Dependency injection (LogBuffer per instance)
    - Delegation pattern (InstanceManager → InstanceLifecycle)
key-files:
  created: []
  modified:
    - internal/instance/lifecycle.go
    - internal/instance/manager.go
    - internal/instance/lifecycle_test.go
    - internal/instance/manager_test.go
decisions:
  - Clear LogBuffer before process start (fresh start after update)
  - Preserve LogBuffer on stop (keep logs for debugging)
  - Delegate GetLogBuffer from manager to lifecycle instance
metrics:
  duration: 8min
  completed_date: 2026-03-17
  tasks: 4
  files_modified: 4
  commits: 8
  test_coverage:
    new_tests: 7
    all_passed: true
---

# Phase 21 Plan 02: Instance Management LogBuffer Integration Summary

## 一句话总结

将 LogBuffer 集成到 InstanceLifecycle 和 InstanceManager,实现每个实例独立的日志缓冲区管理,支持重启时清除和停止时保留。

## 实现的功能

### INST-01: InstanceLifecycle LogBuffer 字段
- 每个 InstanceLifecycle 拥有独立的 LogBuffer 实例
- NewInstanceLifecycle 自动创建 LogBuffer
- GetLogBuffer() 方法返回实例的 LogBuffer
- 测试验证不同实例的 LogBuffer 独立性

### INST-02: InstanceManager.GetLogBuffer 方法
- GetLogBuffer(instanceName) 按名称返回实例的 LogBuffer
- 错误处理:实例不存在时返回 InstanceError
- 委托给 InstanceLifecycle.GetLogBuffer()

### INST-03: StartAfterUpdate 使用 StartNanobotWithCapture
- 替换 StartNanobot 为 StartNanobotWithCapture
- 传递实例的 logBuffer 参数给 StartNanobotWithCapture
- 进程输出自动捕获到实例的 LogBuffer

### INST-04: StopForUpdate 保留 LogBuffer
- StopForUpdate 不清除 LogBuffer(已由设计保证)
- 保留日志用于调试停止过程中的问题
- 测试验证 buffer 内容在停止后保持不变

### INST-05: StartAfterUpdate 清除 LogBuffer
- 在启动进程前清除 LogBuffer(Clear() 调用)
- 确保每次更新后是全新的日志开始
- 旧日志被丢弃,新进程日志被捕获

## 技术实现细节

### 1. InstanceLifecycle 结构体修改

```go
type InstanceLifecycle struct {
    config    config.InstanceConfig
    logger    *slog.Logger
    logBuffer *logbuffer.LogBuffer  // INST-01: 新增字段
}

func NewInstanceLifecycle(cfg config.InstanceConfig, baseLogger *slog.Logger) *InstanceLifecycle {
    instanceLogger := baseLogger.With("instance", cfg.Name).With("component", "instance-lifecycle")
    logBuffer := logbuffer.NewLogBuffer(instanceLogger)  // INST-01: 创建 LogBuffer

    return &InstanceLifecycle{
        config:    cfg,
        logger:    instanceLogger,
        logBuffer: logBuffer,
    }
}

func (il *InstanceLifecycle) GetLogBuffer() *logbuffer.LogBuffer {  // INST-01
    return il.logBuffer
}
```

### 2. StartAfterUpdate 修改

```go
func (il *InstanceLifecycle) StartAfterUpdate(ctx context.Context) error {
    il.logger.Info("Starting instance after update")

    // INST-05: 清除 LogBuffer(全新开始)
    il.logBuffer.Clear()

    // INST-03: 使用 StartNanobotWithCapture 并传递 logBuffer
    if err := lifecycle.StartNanobotWithCapture(
        ctx,
        il.config.StartCommand,
        il.config.Port,
        startupTimeout,
        il.logger,
        il.logBuffer,  // 传递实例的 LogBuffer
    ); err != nil {
        // ... 错误处理 ...
    }

    il.logger.Info("Instance started successfully with log capture")
    return nil
}
```

### 3. InstanceManager.GetLogBuffer 方法

```go
func (m *InstanceManager) GetLogBuffer(instanceName string) (*logbuffer.LogBuffer, error) {
    for _, inst := range m.instances {
        if inst.config.Name == instanceName {
            return inst.GetLogBuffer(), nil  // 委托给 InstanceLifecycle
        }
    }
    return nil, &InstanceError{
        InstanceName: instanceName,
        Operation:    "get_log_buffer",
        Err:          fmt.Errorf("instance not found"),
    }
}
```

## TDD 执行流程

### Task 1: Add LogBuffer field to InstanceLifecycle
- **RED**: 编写 3 个测试(创建、GetLogBuffer、独立性)
- **GREEN**: 添加 logBuffer 字段、创建 LogBuffer、实现 GetLogBuffer()
- **提交**: 2 commits (test + feat)

### Task 2: Modify StartAfterUpdate
- **RED**: 编写 2 个测试(清除 buffer、使用 StartNanobotWithCapture)
- **GREEN**: 在 StartAfterUpdate 中清除 buffer 并调用 StartNanobotWithCapture
- **提交**: 2 commits (test + feat)

### Task 3: Verify StopForUpdate preserves buffer
- **RED**: 编写测试验证 buffer 在停止后被保留
- **GREEN**: 无需修改代码(StopForUpdate 已保留 buffer)
- **提交**: 1 commit (test only)

### Task 4: Add InstanceManager.GetLogBuffer
- **RED**: 编写测试验证按名称获取 buffer 和错误处理
- **GREEN**: 实现 GetLogBuffer 方法并委托给 InstanceLifecycle
- **提交**: 2 commits (test + feat)

## 测试覆盖

### 新增测试(7个)
1. `TestNewInstanceLifecycle_LogBuffer` - 验证 LogBuffer 自动创建
2. `TestInstanceLifecycle_GetLogBuffer` - 验证 GetLogBuffer() 返回非空
3. `TestInstanceLifecycle_IndependentLogBuffers` - 验证实例独立性
4. `TestInstanceLifecycle_StartClearsBuffer` - 验证启动时清除旧日志
5. `TestInstanceLifecycle_StartWithCapture` - 验证使用 StartNanobotWithCapture
6. `TestInstanceLifecycle_StopPreservesBuffer` - 验证停止时保留日志
7. `TestInstanceManager_GetLogBuffer` - 验证 manager 按 name 获取 buffer

### 测试结果
- 所有 7 个新测试通过
- 所有现有测试仍然通过
- `go test ./internal/instance -v`: 26 tests PASS
- `go test ./internal/logbuffer -v`: 14 tests PASS

## 代码变更统计

- **修改文件**: 4 个
  - `internal/instance/lifecycle.go`: +18 行 (logBuffer 字段和方法)
  - `internal/instance/manager.go`: +16 行 (GetLogBuffer 方法)
  - `internal/instance/lifecycle_test.go`: +142 行 (6 个新测试)
  - `internal/instance/manager_test.go`: +53 行 (1 个新测试)

- **提交次数**: 8 commits (4x RED tests + 4x GREEN implementations)

## 偏离计划的情况

**无偏离** - 计划执行完全按照 PLAN.md 进行。

所有 4 个任务按 TDD 流程完成,没有发现 bug 或需要修复的问题。

## 后续依赖

此计划为后续阶段提供基础:

- **Phase 22**: HTTP API 可以通过 InstanceManager.GetLogBuffer() 获取实例的 LogBuffer,实现 SSE 流式传输
- **Phase 23**: 前端可以通过 SSE 接收实时日志更新

## 验证的真理(Must-Haves)

计划中定义的真理已全部验证:

- ✅ Each InstanceLifecycle has its own LogBuffer instance
- ✅ InstanceManager.GetLogBuffer(name) returns correct LogBuffer
- ✅ StopForUpdate preserves LogBuffer content
- ✅ StartAfterUpdate clears LogBuffer before starting
- ✅ StartAfterUpdate uses StartNanobotWithCapture with instance's LogBuffer

## 验收标准

所有验收标准已满足:

- [x] InstanceLifecycle contains logBuffer field (INST-01)
- [x] InstanceLifecycle.GetLogBuffer() method exists (INST-01)
- [x] InstanceManager.GetLogBuffer() method exists (INST-02)
- [x] StartAfterUpdate uses StartNanobotWithCapture with logBuffer (INST-03)
- [x] StopForUpdate preserves LogBuffer content (INST-04)
- [x] StartAfterUpdate clears LogBuffer before start (INST-05)
- [x] All new tests pass
- [x] All existing tests still pass

## Self-Check: PASSED

**文件验证:**
- ✅ internal/instance/lifecycle.go 存在
- ✅ internal/instance/manager.go 存在
- ✅ internal/instance/lifecycle_test.go 存在
- ✅ internal/instance/manager_test.go 存在
- ✅ 21-02-SUMMARY.md 存在

**提交验证:**
- ✅ 06ab98e: test(21-02): add failing tests for LogBuffer field in InstanceLifecycle
- ✅ 1e94440: feat(21-02): add LogBuffer field to InstanceLifecycle
- ✅ c3d57d2: test(21-02): add failing tests for StartAfterUpdate LogBuffer integration
- ✅ a21a4b1: feat(21-02): modify StartAfterUpdate to use StartNanobotWithCapture
- ✅ c79f00d: test(21-02): add test for StopForUpdate LogBuffer preservation
- ✅ a3cfd20: test(21-02): add failing test for InstanceManager.GetLogBuffer
- ✅ fa1cf15: feat(21-02): add GetLogBuffer method to InstanceManager

所有任务完成,所有提交已创建。

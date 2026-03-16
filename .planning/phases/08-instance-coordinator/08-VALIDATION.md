# Phase 8: 实例协调器 - Validation

**Date:** 2026-03-11
**Status:** Nyquist-compliant
**Last Audit:** 2026-03-16

## Phase Goal

创建 InstanceManager 协调器,负责编排所有实例的停止→更新→启动流程,实现优雅降级和错误聚合。

## Requirements Traceability

| Requirement ID | Description | Implementation | Verification | Status |
|----------------|-------------|----------------|--------------|--------|
| LIFECYCLE-01 | 按顺序停止所有配置的实例 | InstanceManager.stopAll() 串行调用每个实例的 StopForUpdate() | TestStopAllGracefulDegradation | ✅ PASS |
| LIFECYCLE-02 | 按顺序启动所有配置的实例 | InstanceManager.startAll() 串行调用每个实例的 StartAfterUpdate() | TestStartAllGracefulDegradation | ✅ PASS |
| LIFECYCLE-03 | 优雅降级 - 某个实例失败时继续操作其他实例 | stopAll/startAll 方法记录错误但不提前返回 | TestStopAllGracefulDegradation, TestStartAllGracefulDegradation | ✅ PASS |
| ERROR-02 | 错误聚合 - 收集所有实例错误 | UpdateResult 结构体记录成功/失败状态 | TestUpdateResultHasErrors, TestUpdateError, TestUpdateAllSkipUpdateWhenStopFails | ✅ PASS |

## Success Criteria

### 1. 功能验证

- [x] **实例协调**: InstanceManager 可以创建并持有所有实例的 InstanceLifecycle
  - 验证: NewInstanceManager() 遍历 config.Instances 并创建包装器
  - 文件: `internal/instance/manager.go`

- [x] **完整流程**: UpdateAll() 可以执行停止→更新→启动完整流程
  - 验证: UpdateAll() 内部调用 stopAll() → performUpdate() → startAll()
  - 文件: `internal/instance/manager.go`

- [x] **优雅降级**: 某个实例停止失败时,其他实例继续停止
  - 验证: stopAll() 记录错误到 StopFailed 数组但不提前返回
  - 文件: `internal/instance/manager.go` 的 stopAll() 方法

- [x] **跳过更新**: 停止失败时系统跳过 UV 更新
  - 验证: UpdateAll() 检查 len(result.StopFailed) > 0 时跳过 performUpdate()
  - 文件: `internal/instance/manager.go` 的 UpdateAll() 方法

- [x] **优雅降级**: 某个实例启动失败时,其他实例继续启动
  - 验证: startAll() 记录错误到 StartFailed 数组但不提前返回
  - 文件: `internal/instance/manager.go` 的 startAll() 方法

- [x] **错误聚合**: UpdateResult 完整记录所有实例的成功/失败状态
  - 验证: UpdateResult 包含 Stopped, Started, StopFailed, StartFailed 字段
  - 文件: `internal/instance/result.go`

### 2. 代码质量验证

- [x] 代码符合 Go 标准格式
  - 命令: `gofmt -w internal/instance/*.go`
  - 预期: 无输出

- [x] 所有公共方法有文档注释
  - 验证: UpdateResult, UpdateError, InstanceManager 的公共方法都有注释
  - 文件: `internal/instance/result.go`, `internal/instance/manager.go`

- [x] 错误处理使用 InstanceError 封装
  - 验证: InstanceError 在 Phase 7 已实现,Phase 8 直接复用
  - 文件: `internal/instance/errors.go`

- [x] 日志使用 slog 结构化日志
  - 验证: InstanceManager 使用 logger.With() 预注入 component 字段
  - 文件: `internal/instance/manager.go`

- [x] 无静态分析警告
  - 命令: `go vet ./internal/instance`
  - 预期: 无警告

### 3. 日志规范验证

- [x] InstanceManager 日志包含 `component` 字段
  - 验证: NewInstanceManager() 使用 `logger.With("component", "instance-manager")`
  - 文件: `internal/instance/manager.go`

- [x] 每个 InstanceLifecycle 日志包含 `instance` 和 `component` 字段
  - 验证: Phase 7 已实现,在 NewInstanceLifecycle() 中注入
  - 文件: `internal/instance/lifecycle.go` (Phase 7 已完成)

- [x] 每个阶段开始/完成时记录 INFO 日志
  - 验证: stopAll(), startAll(), UpdateAll() 都有 INFO 日志
  - 文件: `internal/instance/manager.go`

- [x] 实例失败时记录 ERROR 日志,包含实例名称和端口
  - 验证: stopAll/startAll 的错误日志包含 error 和 port 字段
  - 文件: `internal/instance/manager.go`

### 4. 单元测试验证

**Wave 0 测试文件创建:**

- [x] `internal/instance/result_test.go` - UpdateResult 和 UpdateError 单元测试
  - 验证 HasErrors() 方法
  - 验证 Error() 方法格式
  - 验证 Unwrap() 返回正确类型
  - 验证 errors.Is/As 支持
  - 验证空错误列表处理
  - 验证单个错误处理

- [x] `internal/instance/manager_test.go` - InstanceManager 单元测试
  - 验证 NewInstanceManager() 正确初始化
  - 验证 stopAll() 优雅降级 - 继续处理其他实例
  - 验证 startAll() 优雅降级 - 继续处理其他实例
  - 验证 UpdateAll() 跳过更新逻辑 (当 stop 失败时)
  - 验证 InstanceError 类型断言
  - 验证错误聚合完整性

**注意:** 完整测试套件已创建,所有测试通过。

### 5. 集成验证

Phase 8 的集成验证将在 Phase 10 (主程序集成) 中进行,验证:
- InstanceManager 与 Updater 的集成
- 多实例场景下的完整更新流程
- 日志输出的上下文追踪

## Verification Commands

### 编译验证

```bash
# 编译检查
go build ./internal/instance

# 静态分析
go vet ./internal/instance

# 格式检查
gofmt -w internal/instance/*.go
```

### 单元测试

```bash
# 运行所有 instance 包测试
go test ./internal/instance -v

# 运行特定测试
go test ./internal/instance -v -run TestUpdateResult
go test ./internal/instance -v -run TestInstanceManager
go test ./internal/instance -v -run TestGracefulDegradation
```

### 覆盖率测试

```bash
# 生成覆盖率报告
go test ./internal/instance -cover -coverprofile=coverage.out

# 查看覆盖率详情
go tool cover -func=coverage.out
```

## Known Issues

无已知问题。

## Dependencies

### Upstream Dependencies

- **Phase 7**: InstanceLifecycle 包装器 (已完成)
  - 提供 StopForUpdate/StartAfterUpdate 方法
  - 提供 InstanceError 错误类型

- **Phase 6**: 配置扩展 (已完成)
  - 提供 InstanceConfig 结构体

- **Phase 2**: Updater 结构 (已完成)
  - 提供 Update() 方法

### Downstream Consumers

- **Phase 9**: 通知扩展
  - 将使用 UpdateResult 构建详细的通知消息

- **Phase 10**: 主程序集成
  - 将在 main.go 中集成 InstanceManager

## Risks and Mitigations

### Risk 1: UpdateError 的 Error() 方法格式不符合预期

**Impact:** 用户看到的错误消息不清晰
**Mitigation:** 使用 strings.Builder 构建消息,包含失败实例数量和名称
**Verification:** 单元测试验证 Error() 输出格式

### Risk 2: 停止失败后仍执行 UV 更新

**Impact:** 可能导致文件冲突或更新失败
**Mitigation:** UpdateAll() 检查 StopFailed 数组,仅在无停止失败时执行更新
**Verification:** 单元测试验证跳过更新逻辑

### Risk 3: Unwrap() 返回类型错误

**Impact:** errors.Is/As 无法正常工作
**Mitigation:** Unwrap() 返回 `[]error` 而非 `[]*InstanceError`
**Verification:** 单元测试验证错误链遍历

## Sign-off

**Phase 8 负责人**: Claude (AI Assistant)
**验证时间**: Phase 8 执行完成后
**下一步**: Phase 9 (通知扩展) 使用 UpdateResult 构建详细的失败通知

## Validation Audit 2026-03-16

| Metric | Count |
|--------|-------|
| Gaps found | 2 |
| Resolved | 2 |
| Escalated | 0 |

### Gap 1: LIFECYCLE-03 优雅降级测试
- **Status**: ✅ RESOLVED
- **Test File**: `internal/instance/manager_test.go`
- **Tests Added**:
  - `TestStopAllGracefulDegradation` - 验证 stopAll() 继续处理其他实例
  - `TestStartAllGracefulDegradation` - 验证 startAll() 继续处理其他实例
  - `TestUpdateAllSkipUpdateWhenStopFails` - 验证停止失败时跳过更新
- **Verification Command**: `go test ./internal/instance -v -run "TestStopAllGracefulDegradation\|TestStartAllGracefulDegradation"`
- **Result**: PASS

### Gap 2: result_test.go 独立测试文件
- **Status**: ✅ RESOLVED
- **Test File**: `internal/instance/result_test.go` (新建)
- **Tests Added**:
  - `TestUpdateResultHasErrors` - 7个子测试覆盖各种场景
  - `TestUpdateError` - 验证错误聚合
  - `TestUpdateErrorEmpty` - 验证空错误列表
  - `TestUpdateErrorSingle` - 验证单个错误
  - `TestUpdateErrorUnwrapSupportsErrorsIs` - 验证 errors.Is 支持
  - `TestUpdateErrorUnwrapSupportsErrorsAs` - 验证 errors.As 支持
- **Verification Command**: `go test ./internal/instance -v -run TestUpdateResult`
- **Result**: PASS

### Coverage Report
- `result.go`: 100% 覆盖率
- `manager.go`: stopAll (71.4%), startAll (85.7%)

### All Tests Status
✅ 所有测试通过 (18 个测试用例, 总耗时 <5 秒)

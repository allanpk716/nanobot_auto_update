# Phase 8: 实例协调器 - Context

**Gathered:** 2026-03-11
**Status:** Ready for planning

<domain>
## Phase Boundary

创建 InstanceManager 协调所有实例的停止→更新→启动流程。系统按顺序停止所有配置的实例,执行一次全局 UV 更新,然后按顺序启动所有配置的实例。失败时记录错误但继续其他实例,收集所有实例的操作结果。不涉及具体的停止/启动逻辑实现(Phase 7), 也不涉及通知发送(Phase 9)。

</domain>

<decisions>
## Implementation Decisions

### 协调器结构设计
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

### 协调器方法接口
- **单一方法**: `UpdateAll(ctx context.Context) (*UpdateResult, error)` 执行完整更新流程
  - 停止所有 → UV 更新 → 启动所有
  - 内部调用 StopAll()、performUpdate()、StartAll() 私有方法
  - 返回 UpdateResult 结构体和包含成功/失败详情
- **分步方法**(可选): 可以添加 StopAll()、PerformUpdate()、StartAll() 公共方法,便于测试和  - 如果不存在,使用单一 UpdateAll() 方法即可- **错误处理**: 使用自定义 UpdateError 收集多个 InstanceError
  - UpdateError 完整流程失败时返回 error
  - 部分失败时返回 UpdateResult + error
  - 不返回 error

### 优雅降级策略
- **停止失败**: 讟记录错误
  - **跳过 UV 更新** - 避免文件冲突
  - 继续停止其他实例
  - 返回 UpdateResult 标记停止失败
- **启动失败**: 讋记录错误`
  - **继续启动其他实例** - 最大化服务可用性
  - 返回 UpdateResult 标记启动失败
- **错误继续**: 不立即中止流程,继续尝试启动其他实例

### 错误聚合格式
- **UpdateResult 结构**:
  ```go
  type UpdateResult struct {
      Stopped    []string // 成功停止的实例名称列表
      Started   []string // 成功启动的实例名称列表
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
  - UpdateError 夌合 InstanceError 列表
  - 无需额外封装

### Claude's Discretion
- InstanceManager 的具体命名(如 InstanceCoordinator vs InstanceSupervisor)
- UpdateError 的 Error() 方法实现细节(如使用图标、 实例名称对齐)
- 是否添加分步方法(StopAll/StartAll)的决策

</decisions>

<specifics>
## Specific Ideas

- **串行执行符合"优雅降级"理念** - 简单实现, 便于调试
- **跳过更新**的安全策略 - 避免未停止实例占用文件导致更新失败
- **结构化 UpdateResult** - 清晰分离成功/失败状态
  便于 Phase 9 构建通知
- **友好的错误消息** - 包含成功比例和失败详情
  用户可以快速定位问题
- **测试友好** - 单元测试验证各组件
  分步测试逐步构建
  集成测试验证完整流程
- **日志注入** - 构造时注入实例和组件字段
  所有日志自动包含上下文

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- **internal/instance/lifecycle.go**: InstanceLifecycle 包装器
  提供 StopForUpdate/StartAfterUpdate 方法
  可直接调用
- **internal/instance/errors.go**: InstanceError 自定义错误类型
  支持结构化错误消息和 errors.Is/As 错误链遍历
- **internal/lifecycle/*.go**: 生命周期管理函数
  IsNanobotRunning, StopNanobot, StartNanobot
- **internal/updater/updater.go**: Updater 结构
  执行全局更新
- **internal/notifier/notifier.go**: Notifier 结构
  发送 Pushover 通知
- **internal/config/config.go**: Config 结构
  包含 Instances []InstanceConfig

### Established Patterns
- **错误处理**: 使用自定义错误类型(InstanceError) + errors.Join 聚合错误
- **配置加载**: Load() 返回 Config, 包含 Instances 字段
- **生命周期管理**: InstanceLifecycle 提供 StopForUpdate/StartAfterUpdate 接口
- **日志注入**: slog.With() 在构造时注入实例和组件字段

### Integration Points
- **主程序集成**: Phase 10 的 main.go 将使用 InstanceManager 执行多实例更新流程
- **通知扩展**: Phase 9 的 Notifier 将使用 Phase 8 的 UpdateResult 结构化信息构建 Pushover 通知
- **测试策略**: 需要单元测试验证每个组件(InstanceManager, UpdateAll, UpdateResult) + 集成测试验证完整流程

</code_context>

<deferred>
## Deferred Ideas
None — 讨论保持在阶段范围内

</deferred>

---

*Phase: 08-instance-coordinator*
*Context gathered: 2026-03-11*

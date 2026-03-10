# Phase 7: 生命周期扩展 - Context

**Gathered:** 2026-03-10
**Status:** Ready for planning

<domain>
## Phase Boundary

为每个实例提供独立的上下文感知生命周期管理,每个实例的日志包含实例名称,可以针对特定实例执行停止/启动操作,复用现有的 v1.0 生命周期逻辑。此阶段专注于为每个实例创建包装器,不涉及多实例协调(Phase 8)或错误聚合(Phase 9)。

</domain>

<decisions>
## Implementation Decisions

### 日志上下文注入
- **格式**: 使用 slog 结构化字段,不使用消息前缀
- **字段**: `instance`(实例名称) + `component`("instance-lifecycle")
- **注入位置**: InstanceLifecycle 构造时调用 `logger.With("instance", config.Name).With("component", "instance-lifecycle")` 创建实例专属 logger
- **日志示例**: `2026-03-10 15:00:00.123 - [INFO]: Starting stop... component=instance-lifecycle instance=nanobot-main port=18790`

### 生命周期包装器设计
- **包位置**: 创建 `internal/instance` 包,定义 `InstanceLifecycle` 包装器
- **结构字段**:
  - `config` (InstanceConfig) - 实例配置,包含 name、port、start_command、startup_timeout
  - `logger` (*slog.Logger) - 预注入实例上下文的 logger
- **方法接口**: 与现有 `lifecycle.Manager` 一致
  - `StopForUpdate(ctx context.Context) error`
  - `StartAfterUpdate(ctx context.Context) error`
- **内部实现**: 直接调用 `lifecycle` 包的函数,不委托给 Manager
  - 停止: 调用 `lifecycle.IsNanobotRunning()` + `lifecycle.StopNanobot()`
  - 启动: 调用重构后的 `lifecycle.StartNanobot()` (接收 command 参数)

### 启动命令定制化
- **命令执行**: 使用 Shell 执行方式 `exec.Command("cmd", "/c", command)`
- **命令格式**: `start_command` 作为完整命令字符串,支持管道、重定向等复杂命令
- **命令示例**:
  - `"nanobot gateway --port 18790"`
  - `"python C:/nanobot/main.py --config C:/nanobot/config.yaml"`
  - `"cmd /c start /min nanobot gateway"`
- **现有代码调整**: 重构 `lifecycle.StartNanobot()` 接收 `command string` 参数,不再固定为 `"nanobot gateway"`
- **启动验证**: 继续使用端口监听验证,传递实例配置的 `port` 参数给启动函数

### 实例错误上下文
- **错误信息内容**:
  - 实例名称 (必需)
  - 操作类型 (推荐) - "stop" 或 "start"
  - 实例端口 (推荐)
  - 底层错误详情
- **错误格式**: 结构化格式,示例 `"停止实例 'nanobot-main' 失败 (port=18790): taskkill returned exit code 1"`
- **错误类型**: 定义 `InstanceError` 自定义错误类型
  ```go
  type InstanceError struct {
      InstanceName string
      Operation    string // "stop" or "start"
      Port         uint32
      Err          error
  }

  func (e *InstanceError) Error() string {
      return fmt.Sprintf("%s实例 %q 失败 (port=%d): %v",
          e.operationText(), e.InstanceName, e.Port, e.Err)
  }

  func (e *InstanceError) Unwrap() error {
      return e.Err
  }
  ```
- **错误使用**: InstanceLifecycle 的 StopForUpdate/StartAfterUpdate 方法返回 InstanceError,调用者可以类型断言提取结构化信息

### Claude's Discretion
- InstanceLifecycle 结构体的具体命名(例如 InstanceLifecycle vs InstanceManager)
- 错误消息的中英文选择(示例使用中文,实际实现可调整)
- InstanceError 的具体方法实现细节

</decisions>

<specifics>
## Specific Ideas

- 日志使用结构化字段便于后续日志分析和过滤,符合现代日志管理最佳实践
- 包装器构造时注入 logger 避免在每个方法中重复注入代码,符合 DRY 原则
- Shell 执行方式给用户最大灵活性,支持任意复杂的启动命令
- 自定义错误类型便于 Phase 8 错误聚合和 Phase 9 通知构建,调用者可以程序化提取实例信息
- 重构 StartNanobot 函数保持代码简洁,避免代码重复

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- **internal/lifecycle/manager.go**: 现有生命周期管理器,提供 StopForUpdate/StartAfterUpdate 方法模式可参考
- **internal/lifecycle/stopper.go**: StopNanobot() 函数实现优雅停止 + 超时强制终止,可直接调用
- **internal/lifecycle/starter.go**: StartNanobot() 函数需要重构以接收 command 参数,启动验证逻辑可复用
- **internal/lifecycle/detector.go**: IsNanobotRunning() 函数通过端口检测进程,可直接调用
- **internal/config/instance.go**: InstanceConfig 结构已定义 name、port、start_command、startup_timeout 字段
- **internal/logging/logging.go**: 自定义 slog handler 已实现结构化字段追加,支持 logger.With() 注入字段

### Established Patterns
- **生命周期模式**: Manager 提供 StopForUpdate(ctx) error 和 StartAfterUpdate(ctx) error 方法,错误返回影响调用者决策
- **日志注入模式**: 使用 slog.Logger.With() 预注入上下文字段,所有日志自动包含这些字段
- **错误处理模式**: 使用 fmt.Errorf 包装错误,提供上下文信息;Phase 7 需要扩展为自定义错误类型
- **配置模式**: InstanceConfig 包含实例特定配置,Validate() 方法验证配置有效性

### Integration Points
- **配置加载**: internal/config.Load() 返回的 Config 结构包含 Instances []InstanceConfig,Phase 7 需要为每个 InstanceConfig 创建 InstanceLifecycle
- **主程序集成**: Phase 10 的 main.go 将使用 Phase 7 的 InstanceLifecycle 执行多实例更新流程
- **错误聚合**: Phase 8 的 InstanceManager 将调用 Phase 7 的 InstanceLifecycle,收集所有实例的 InstanceError
- **通知构建**: Phase 9 的 Notifier 将使用 Phase 7 的 InstanceError 结构化信息构建 Pushover 通知

</code_context>

<deferred>
## Deferred Ideas

None — 讨论保持在阶段范围内

</deferred>

---

*Phase: 07-lifecycle-extension*
*Context gathered: 2026-03-10*

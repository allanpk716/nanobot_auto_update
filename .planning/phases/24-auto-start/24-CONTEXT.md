# Phase 24: Auto-start - Context

**Gathered:** 2026-03-20
**Status:** Ready for planning

<domain>
## Phase Boundary

应用启动时自动启动所有配置的实例，无需用户手动干预。此阶段专注于启动逻辑的自动化，不涉及健康监控（Phase 25）、网络监控（Phase 26-27）或 HTTP API 触发更新（Phase 28）。

**核心功能：**
- 应用启动时按配置顺序启动所有 `auto_start: true` 的实例
- 提供详细的启动进度日志和汇总状态
- 优雅降级：某实例失败不影响其他实例
- 与现有 InstanceManager 和 InstanceLifecycle 集成

**成功标准：**
1. 用户启动应用后，所有配置的实例自动按顺序启动
2. 用户可以通过日志看到每个实例的启动状态（成功或失败）
3. 某个实例启动失败时，其他实例仍然继续启动
4. 所有实例启动完成后，用户可以在日志中看到汇总状态（成功/失败数量）

</domain>

<decisions>
## Implementation Decisions

### 配置选项
- **实例级别 auto_start 开关**
  - 在 `InstanceConfig` 中添加 `AutoStart bool` 字段（默认 `true`）
  - 实例配置示例：
    ```yaml
    instances:
      - name: "gateway"
        port: 18790
        start_command: "python -m nanobot.gateway"
        auto_start: false  # 跳过自动启动
      - name: "worker"
        port: 18791
        start_command: "python -m nanobot.worker"
        # auto_start 默认 true，无需显式配置
    ```
  - `auto_start: false` 的实例在启动阶段被跳过
  - 无全局开关，简化实现

### 启动时机
- **API 服务器启动后启动**
  - 启动顺序：配置加载 → InstanceManager 创建 → API 服务器启动 → 自动启动实例
  - API 服务器先准备好，用户可以通过 `/api/v1/status` 等端点查看启动状态
  - 实例启动在 goroutine 中执行，不阻塞 API 服务器启动
  - 实现位置：在 `main.go` 中，API 服务器启动 goroutine 之后启动实例

### 失败重试策略
- **无重试机制**
  - 实例启动失败后直接记录错误，继续启动其他实例
  - 简单实现，避免复杂的重试逻辑
  - 失败信息通过日志记录（Phase 25 健康监控会持续检查实例状态）
  - 用户可以通过手动触发更新（Phase 28 HTTP API）重新启动失败的实例

### 日志输出格式
- **每个实例启动时记录 INFO**
  - 启动前：`Starting instance "gateway" (port=18790)...`
  - 启动后：`Instance "gateway" started successfully (duration=2.3s)`
  - 包含实例名、端口、耗时
- **失败实例的 ERROR 详情**
  - 错误日志：`Failed to start instance "worker" (port=18791): <InstanceError details>`
  - 包含实例名、端口、错误原因、底层错误
  - 使用 `InstanceError` 结构化错误（继承 Phase 7）
- **汇总日志**
  - 所有实例启动完成后：`Auto-start completed: 2 started, 1 failed (failed: [worker])`
  - 包含成功数量、失败数量、失败实例名称列表

### 失败后的应用行为
- **继续运行应用**
  - 即使部分实例启动失败，应用仍然继续运行
  - 已启动的实例正常提供服务（如 API 访问、SSE 流式日志）
  - 失败信息通过日志记录，不退出应用
  - Phase 25 健康监控会定期检查实例状态并记录日志

### Claude's Discretion
- 日志消息的具体措辞（中文/英文）
- 汇总日志的格式（图标、对齐、颜色）
- 耗时的格式（秒 vs 毫秒）
- 实例启动 goroutine 的错误处理（如何记录 panic 等异常）

</decisions>

<specifics>
## Specific Ideas

- **实例级别控制灵活性高** - 某些实例可能需要手动启动（如调试中的实例），跳过自动启动避免干扰
- **API 优先启动** - 确保 HTTP API 先准备好，用户可以立即访问状态端点，提升用户体验
- **简单重试策略** - 无重试保持简单，依赖健康监控发现问题，避免复杂的重试逻辑
- **详细日志输出** - 每个实例的启动状态和汇总信息清晰可见，用户可以快速定位问题
- **优雅降级** - 部分失败不影响整体服务，已启动的实例继续工作
- **复用现有错误处理** - InstanceError 已提供结构化错误，无需额外封装

</specifics>

<canonical_refs>
## Canonical References

### Phase 24 需求
- `.planning/REQUIREMENTS.md` § AUTOSTART — 自动启动需求 (AUTOSTART-01, 02, 03, 04)
- `.planning/ROADMAP.md` § Phase 24 — Auto-start 阶段目标和成功标准

### 现有架构参考
- `.planning/phases/07-lifecycle-extension/07-CONTEXT.md` — InstanceLifecycle 包装器和 InstanceError 设计
- `.planning/phases/08-instance-coordinator/08-CONTEXT.md` — InstanceManager 协调器和优雅降级策略

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **internal/instance/lifecycle.go**: `InstanceLifecycle` 提供 `StartAfterUpdate(ctx)` 方法
  - 直接调用即可启动单个实例
  - 返回 `*InstanceError` 结构化错误
- **internal/instance/manager.go**: `InstanceManager` 持有所有实例的 `InstanceLifecycle`
  - 已有 `instances []*InstanceLifecycle` 字段
  - 可以添加 `StartAllInstances()` 方法
- **internal/instance/errors.go**: `InstanceError` 自定义错误类型
  - 包含 `InstanceName`, `Operation`, `Port`, `Err` 字段
  - 支持 `Error()` 和 `Unwrap()` 方法
- **internal/config/instance.go**: `InstanceConfig` 结构
  - 已有 `Name`, `Port`, `StartCommand`, `StartupTimeout` 字段
  - 需要添加 `AutoStart bool` 字段
- **cmd/nanobot-auto-updater/main.go**: 主程序入口
  - 已有 API 服务器启动逻辑
  - 需要在 API 启动后添加实例自动启动逻辑

### Established Patterns
- **串行启动**: Phase 8 确定的串行执行模式，按配置顺序依次启动
- **优雅降级**: Phase 8 确定的失败不中断流程，继续处理其他实例
- **上下文感知日志**: Phase 7 确定的 `logger.With("instance", name)` 预注入模式
- **InstanceError 错误链**: Phase 7 确定的结构化错误，支持 `errors.Is/As` 遍历
- **配置验证**: Phase 6 确定的 `Validate()` 方法模式

### Integration Points
- **配置加载**: `config.Load()` 需要解析新增的 `auto_start` 字段
- **InstanceManager 扩展**: 添加 `StartAllInstances()` 方法，遍历 `auto_start: true` 的实例并启动
- **main.go 集成**: 在 API 服务器启动 goroutine 后调用 `instanceManager.StartAllInstances()`
- **Phase 25 健康监控**: 依赖 Phase 24 的实例启动状态，定期检查实例是否运行
- **Phase 28 HTTP API**: 可以通过 API 触发重新启动失败的实例

</code_context>

<deferred>
## Deferred Ideas

None — 讨论保持在阶段范围内

</deferred>

---

*Phase: 24-auto-start*
*Context gathered: 2026-03-20*

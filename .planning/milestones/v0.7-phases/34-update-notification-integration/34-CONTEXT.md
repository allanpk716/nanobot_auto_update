# Phase 34: Update Notification Integration - Context

**Gathered:** 2026-03-29
**Status:** Ready for planning

<domain>
## Phase Boundary

在 HTTP API 触发的 nanobot 更新流程中，将现有 Notifier 注入 TriggerHandler，实现更新开始和完成时的 Pushover 通知发送。非阻塞异步发送，Pushover 未配置时优雅降级。

**核心功能:**
- 更新开始通知：TriggerHandler 收到请求后、执行 TriggerUpdate 前发送
- 更新完成通知：更新完成后发送，包含三态状态和实例结果
- 非阻塞发送：通知在独立 goroutine 中执行
- 优雅降级：Pushover 未配置时跳过通知

**不包含:**
- Cron 触发的更新通知（Out of Scope）
- 通知模板自定义（Out of Scope）
- 通知频率限制（Out of Scope）

</domain>

<decisions>
## Implementation Decisions

### 通知内容格式
- **D-01: 开始通知格式**
  - 标题: "Nanobot 更新开始"
  - 消息: "触发来源: api-trigger\n待更新实例数: {N}"
  - 中文内容，与现有 Pushover 通知风格一致（Phase 27 用中文）

- **D-02: 完成通知格式**
  - 标题根据状态动态生成:
    - success: "Nanobot 更新成功"
    - partial_success: "Nanobot 更新部分成功"
    - failed: "Nanobot 更新失败"
  - 消息包含汇总信息:
    - 耗时（秒）: "耗时: {X.X}s"
    - 总实例数、成功数、失败数
    - 失败实例名称列表（如有）
  - 不包含每个实例的详细错误信息（通知应简洁，详细查看日志）

### 注入方式
- **D-03: 与 UpdateLogger 相同的注入模式**
  - TriggerHandler 增加 `notifier *notifier.Notifier` 字段
  - NewTriggerHandler 增加 `notif *notifier.Notifier` 参数
  - NewServer 增加 `notif *notifier.Notifier` 参数
  - main.go 将已创建的 Notifier 传入 NewServer
  - 与 Phase 30 注入 UpdateLogger 的模式完全一致

### 通知发送时机
- **D-04: 开始通知在 UUID 生成之后、TriggerUpdate 之前**
  - 流程: 生成 UUID → 记录开始时间 → 发送开始通知 → TriggerUpdate → ...
  - 开始通知中包含 update_id 可选（但不需要，通知内容保持简洁）

- **D-05: 完成通知在 UpdateLog 记录之后、HTTP 响应之前**
  - 流程: ... → TriggerUpdate → 记录 UpdateLog → 发送完成通知 → 返回 HTTP 响应
  - 完成通知在 goroutine 中发送，不阻塞 HTTP 响应

### 优雅降级
- **D-06: 复用现有 IsEnabled() 机制**
  - Notifier.IsEnabled() 返回 false 时，Notify() 已自动跳过并记录 DEBUG
  - 无需额外判断，直接调用 Notify() 即可
  - 与 Phase 27 行为一致

### 通知发送失败处理
- **D-07: 与 Phase 27 一致 — 仅记录 ERROR 日志**
  - 通知发送失败记录 ERROR 日志，不影响更新流程
  - 不重试
  - HTTP 响应和 UpdateLog 记录不受影响

### Claude's Discretion
- 通知内容的具体措辞和格式细节
- goroutine 中错误处理的具体实现
- 开始通知和完成通知是否使用同一个 goroutine
- 日志字段的具体命名

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 需求参考
- `.planning/REQUIREMENTS.md` § UNOTIF-01, UNOTIF-02, UNOTIF-03, UNOTIF-04 — 更新通知需求

### 现有架构参考
- `.planning/phases/27-network-monitoring-notifications/27-CONTEXT.md` — Pushover 通知模式（异步发送、优雅降级、失败处理）
- `.planning/phases/28-http-api-trigger/28-CONTEXT.md` — HTTP API 触发更新端点和 TriggerHandler
- `.planning/phases/30-log-structure-and-recording/30-CONTEXT.md` — UpdateLogger 注入 TriggerHandler 模式

### 关键代码文件
- `internal/api/trigger.go` — TriggerHandler 实现，需增加 notifier 字段和通知调用
- `internal/notifier/notifier.go` — Notifier 实现，Notify() 和 IsEnabled() 方法
- `internal/api/server.go` — NewServer 需增加 notifier 参数传递给 TriggerHandler
- `cmd/nanobot-auto-updater/main.go` — 需将 Notifier 传入 NewServer

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **internal/notifier/notifier.go**: Notifier 已实现完整通知功能
  - `Notify(title, message string) error`: 通用通知发送
  - `IsEnabled() bool`: 检查 Pushover 是否配置
  - `NotifyUpdateResult(result *instance.UpdateResult)`: 现有的多实例失败通知（仅失败时发送）
  - 未配置时返回 disabled notifier，Notify() 跳过并记录 DEBUG
- **internal/api/trigger.go**: TriggerHandler 已有完整更新流程
  - 已注入 UpdateLogger（Phase 30 模式）
  - Handle() 方法有 UUID 生成、超时控制、错误处理
  - 增加 notifier 字段只需在关键点插入通知调用
- **internal/updatelog/logger.go**: UpdateLogger 注入模式可复用
  - NewTriggerHandler 接收 *updatelog.UpdateLogger 参数
  - Nil-safe 检查（if h.updateLogger != nil）
- **cmd/nanobot-auto-updater/main.go**: Notifier 已在 main.go 中创建
  - `notif := notifier.NewWithConfig(...)` 已存在
  - 只需将其传入 api.NewServer() 调用

### Established Patterns
- **异步通知发送**: `go func() { if err := notifier.Notify(...); err != nil { ... } }()`（Phase 27）
- **非阻塞错误处理**: 通知失败仅记录 ERROR 日志，不影响主流程（Phase 27）
- **Nil-safe 组件**: handler 检查 nil，非阻塞错误日志（Phase 30）
- **依赖注入**: main.go 创建组件 → NewServer 接收 → 传给 Handler（Phase 30 UpdateLogger 模式）
- **三态状态分类**: success/partial_success/failed（Phase 30）

### Integration Points
- **TriggerHandler 构造函数**: 增加 `*notifier.Notifier` 参数
- **NewServer 构造函数**: 增加 `*notifier.Notifier` 参数，传给 NewTriggerHandler
- **main.go api.NewServer 调用**: 增加 `notif` 参数
- **Handle() 方法**: 在 TriggerUpdate 前发送开始通知，在结果返回后发送完成通知

</code_context>

<specifics>
## Specific Ideas

- **通知内容简洁实用**: 与 Phase 27 网络通知保持一致风格，标题传达关键信息，消息补充细节
- **完成通知不包含详细错误**: 通知应简洁，失败详情通过查看 UpdateLog API 获取
- **完全复用现有模式**: 注入方式、异步发送、降级策略都已有成熟模式，无需创新
- **最小改动范围**: 主要改动集中在 TriggerHandler，增加 Notifier 字段和两处通知调用

</specifics>

<deferred>
## Deferred Ideas

None — 讨论保持在阶段范围内

</deferred>

---

*Phase: 34-update-notification-integration*
*Context gathered: 2026-03-29*

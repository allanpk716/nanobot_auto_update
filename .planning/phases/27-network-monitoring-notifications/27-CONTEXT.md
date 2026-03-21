# Phase 27: Network Monitoring Notifications - Context

**Gathered:** 2026-03-21
**Status:** Ready for planning

<domain>
## Phase Boundary

当网络连通性状态变化时(从连通→不连通,或不连通→连通),向用户发送 Pushover 通知。Phase 26 已完成连通性监控和状态追踪,本阶段专注于状态变化时的通知逻辑,包括冷却时间防止通知风暴、通知内容设计、异步发送和失败处理。

**核心功能:**
- 检测连通性状态变化(成功→失败 或 失败→成功)
- 状态变化后等待冷却时间(1 分钟)确认,避免通知风暴
- 冷却期满后发送 Pushover 通知
- 简洁的通知内容(状态+错误类型)
- 异步发送通知,不阻塞监控循环
- 处理 Pushover 未配置场景(WARN 日志提醒)

**成功标准:**
1. 连通性从失败恢复为成功时,用户收到 Pushover 恢复通知
2. 连通性从成功变为失败时,用户收到 Pushover 失败通知
3. 状态变化后有 1 分钟冷却时间确认,避免频繁通知
4. Pushover 未配置时,记录 WARN 日志提醒用户配置通知

**不包含:**
- 详细诊断信息(持续时间、时间戳、目标 URL) — 保持简洁
- 通知重试机制 — 失败仅记录 ERROR 日志
- 通知配置开关 — Pushover 可选配置即可

</domain>

<decisions>
## Implementation Decisions

### 通知触发时机和频率控制
- **1 分钟冷却时间防止通知风暴**
  - 状态变化后等待 1 分钟确认,避免网络抖动时的频繁通知
  - 冷却逻辑: 状态变化 → 启动 1 分钟 timer → 冷却期满检查状态 → 如果仍保持新状态则发送通知
  - 适合快速响应场景,同时过滤短期波动
  - 实现: 使用 `time.AfterFunc()` 或 goroutine + `time.Sleep()`

### 通知内容设计
- **简洁通知(状态+错误类型)**
  - 失败通知标题: "网络连通性检查失败"
  - 失败通知内容: "{错误类型}" (如 "连接超时"、"DNS 解析失败")
  - 恢复通知标题: "网络连通性已恢复"
  - 恢复通知内容: "" (空内容,标题已足够)
  - **不包含**: 持续时间、时间戳、目标 URL、HTTP 状态码等详细信息
  - 保持简洁,用户可查看应用日志了解详细诊断信息

### Pushover 未配置处理
- **WARN 日志提醒用户配置**
  - 状态变化时如果 Pushover 未配置(Notifier.IsEnabled() == false),记录 WARN 日志
  - 日志消息: "网络连通性状态变化，但 Pushover 通知未配置。请在 config.yaml 中设置 pushover.api_token 和 pushover.user_key"
  - 包含状态变化方向: "从连通变为不连通" 或 "从不连通变为连通"
  - 确保用户知道错过了通知机会

### 通知发送模式
- **异步发送(独立 goroutine)**
  - 通知发送在独立 goroutine 中执行,不阻塞监控循环
  - 实现: `go func() { if err := notifier.Notify(title, message); err != nil { ... } }()`
  - 即使 Pushover API 慢或失败,也不影响连通性检查的定时执行
  - 推荐做法,确保监控系统稳定性

### 通知发送失败处理
- **仅记录 ERROR 日志,不重试**
  - 通知发送失败时记录 ERROR 日志,包含错误详情
  - 日志示例: "发送连通性变化通知失败 - 错误: pushover API timeout"
  - 不实现重试机制(保持简单)
  - 服务继续运行,下次状态变化会再次尝试通知

### 架构组织
- **独立的 NotificationManager**
  - 创建独立的 `NotificationManager` 结构体,负责:
    - 监听 NetworkMonitor 的状态变化事件
    - 管理冷却时间逻辑(timer + 状态确认)
    - 调用 Notifier 发送通知
  - NetworkMonitor 保持纯粹的状态监控职责
  - NotificationManager 处理通知相关逻辑
  - 职责分离,更易测试和维护
  - 通过依赖注入接收 NetworkMonitor 和 Notifier

### 集成和生命周期
- **在 main.go 中集成**
  - NetworkMonitor 启动后创建 NotificationManager
  - NotificationManager 订阅 NetworkMonitor 的状态变化
  - 实现方式(二选一,由 Claude 决定):
    - 方案 A: NetworkMonitor 提供 `Subscribe() <-chan ConnectivityChangeEvent` channel
    - 方案 B: NotificationManager 定期轮询 `NetworkMonitor.GetState()` + 内部状态追踪
  - 启动顺序: API 服务器 → 健康监控 → 网络监控 → 通知管理器
  - 关闭顺序: 通知管理器 → 网络监控 → 健康监控 → API 服务器

### Claude's Discretion
- 订阅机制的具体实现(channel vs 轮询)
- NotificationManager 的具体结构设计
- 冷却时间 timer 的管理方式(重置、取消)
- 通知失败时的具体日志格式
- 通知发送 goroutine 的错误处理细节

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 27 需求
- `.planning/REQUIREMENTS.md` § MONITOR-04, MONITOR-05 — 连通性变化通知需求
- `.planning/ROADMAP.md` § Phase 27 — Network Monitoring Notifications 阶段目标和成功标准

### 现有实现参考
- `.planning/phases/26-network-monitoring-core/26-CONTEXT.md` — NetworkMonitor 实现、ConnectivityState 结构、状态追踪模式
- `internal/network/monitor.go` — NetworkMonitor 实现,GetState() 方法,状态变化检测(118-124 行)
- `internal/notifier/notifier.go` — Notifier 实现,Notify() 方法,IsEnabled() 检查
- `internal/config/config.go` — PushoverConfig 配置结构
- `config.yaml` § pushover — Pushover 配置示例

### 架构模式参考
- `.planning/phases/24-auto-start/24-CONTEXT.md` — 生命周期集成模式
- `.planning/phases/25-instance-health-monitoring/25-CONTEXT.md` — HealthMonitor 模式(独立结构体 + 生命周期管理)
- `internal/health/monitor.go` — HealthMonitor 参考实现(goroutine + context + 优雅关闭)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **internal/network/monitor.go**: NetworkMonitor 已实现连通性监控和状态追踪
  - `ConnectivityState` 结构: IsConnected bool, LastCheck time.Time
  - `GetState()` 方法: 返回当前状态,供外部订阅
  - 状态变化检测: 118-124 行记录状态变化日志,已实现状态对比逻辑
  - `checkConnectivity()` 方法: 每次检查后更新 `nm.state`
- **internal/notifier/notifier.go**: Notifier 已实现 Pushover 通知
  - `Notify(title, message string) error`: 通用通知发送
  - `IsEnabled() bool`: 检查 Pushover 是否配置
  - 支持配置文件和环境变量两种配置方式
  - 未配置时返回 disabled notifier(enabled=false)
- **internal/config/config.go**: PushoverConfig 配置已存在
  - `Pushover.ApiToken` 和 `Pushover.UserKey` 字段
  - 可选配置,无默认值
- **internal/health/monitor.go**: 生命周期管理模式可复用
  - 独立结构体 + Start()/Stop() 方法
  - goroutine + context + context.CancelFunc 模式
  - 优雅关闭和资源清理

### Established Patterns
- **独立管理器模式**: Phase 25 的 HealthMonitor 模式,封装独立监控逻辑
- **异步通知发送**: Phase 9 的 Notifier.Notify() 在 goroutine 中调用
- **上下文感知日志**: Phase 7 确定的结构化日志模式,logger.With("component", "xxx")
- **优雅关闭**: Phase 25 确定的 context 取消信号传播模式
- **依赖注入**: main.go 创建组件并注入依赖

### Integration Points
- **NetworkMonitor 状态访问**: NotificationManager 需要访问 NetworkMonitor.GetState() 或订阅状态变化事件
- **Notifier 通知发送**: NotificationManager 调用 Notifier.Notify() 发送通知
- **main.go 生命周期管理**: 创建 NotificationManager 并在应用启动/关闭时调用 Start()/Stop()
- **配置加载**: Notifier 已通过 config.Pushover 配置,无需额外配置扩展

</code_context>

<specifics>
## Specific Ideas

- **1 分钟冷却时间平衡速度和稳定性**: 足够快地通知用户网络问题,同时过滤掉短期网络抖动
- **简洁通知快速传达关键信息**: 用户一眼就能看出是失败还是恢复,错误类型帮助快速定位问题
- **WARN 日志强烈提醒配置**: 确保用户不会错过重要的连通性变化通知
- **独立 NotificationManager 职责清晰**: 网络监控专注连通性测试,通知管理器专注通知逻辑,易维护易测试
- **异步发送不阻塞监控**: 即使 Pushover API 响应慢,也不影响每 15 分钟的连通性检查
- **不重试保持简单**: 通知失败记录日志即可,下次状态变化会再次尝试,避免复杂的重试逻辑

</specifics>

<deferred>
## Deferred Ideas

- **详细诊断信息通知** — 当前简洁通知足够,如需详细信息可查看应用日志
- **通知重试机制** — 当前失败仅记录日志,如需提高通知可靠性可添加重试逻辑
- **通知配置开关** — 当前 Pushover 可选配置即可,如需更细粒度控制可添加 enable_notify 开关
- **可配置冷却时间** — 当前固定 1 分钟,如需灵活性可添加到 monitor 配置中
- **多种通知渠道** — 当前仅 Pushover,如需支持邮件、Slack 等需要新的抽象层

</deferred>

---

*Phase: 27-network-monitoring-notifications*
*Context gathered: 2026-03-21*

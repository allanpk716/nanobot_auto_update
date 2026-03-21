# Phase 27: Network Monitoring Notifications - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-21
**Phase:** 27-network-monitoring-notifications
**Areas discussed:** 通知触发时机、通知内容、未配置处理、发送模式、冷却时间、架构组织、失败处理

---

## 通知触发时机和频率控制策略

| Option | Description | Selected |
|--------|-------------|----------|
| 每次状态变化都发送通知 | 每次连通性从成功变为失败,或从失败变为成功时,都立即发送通知。简单直接,适合网络较稳定的场景。 | |
| 添加冷却时间防止通知风暴 | 状态变化后等待一段时间(如 1-2 分钟),如果状态保持不变再发送通知。避免网络抖动时的通知风暴,但会延迟通知。 | ✓ |
| 失败立即通知,恢复延迟确认 | 失败时立即通知,恢复时可以延迟确认后再通知。优先让用户知道故障,恢复通知可以更从容。 | |
| 通过配置文件控制通知开关 | 在配置文件中添加 monitor.notify_on_failure 和 notify_on_recovery 开关,让用户选择是否启用失败/恢复通知。灵活但增加配置复杂度。 | |

**User's choice:** 添加冷却时间防止通知风暴，恢复时添加冷却时间
**Notes:** 用户选择了"添加冷却时间防止通知风暴"，并明确指出恢复时也添加冷却时间。这意味着失败和恢复都应该有冷却确认时间。

---

## 通知内容设计

| Option | Description | Selected |
|--------|-------------|----------|
| 简洁通知(仅状态+错误类型) | 失败: 标题+错误类型。恢复: 标题+恢复时间。简洁明了,适合快速了解状态。 | ✓ |
| 详细通知(包含时间和诊断信息) | 失败: 错误类型+持续时间+时间戳+目标URL。恢复: 恢复时间+中断持续时间+时间戳。提供完整诊断信息。 | |
| 失败简洁,恢复详细 | 失败: 简洁通知。恢复: 详细通知(用户需要知道中断持续了多久,失败原因可能不重要了)。平衡简洁性和信息量。 | |

**User's choice:** 简洁通知(仅状态+错误类型)
**Notes:** 用户选择了简洁通知风格，不包含持续时间、时间戳等详细信息。保持通知简洁快速。

---

## Pushover 未配置处理

| Option | Description | Selected |
|--------|-------------|----------|
| 记录 DEBUG 日志并跳过通知 | 状态变化时,如果 Pushover 未配置,仅记录 DEBUG 日志。用户通过日志知道状态变化但未收到通知。不报错。 | |
| 记录 WARN 日志提醒用户 | 状态变化时,如果 Pushover 未配置,记录 WARN 日志提醒用户配置通知。确保用户知道错过了通知机会。 | ✓ |
| 记录 INFO 日志说明通知未启用 | 状态变化时,如果 Pushover 未配置,记录 INFO 日志说明监控正常工作但通知未启用。中性态度,不警告也不隐藏。 | |

**User's choice:** 记录 WARN 日志提醒用户(更强烈)
**Notes:** 用户选择 WARN 日志级别，强烈提醒用户配置通知。确保用户不会错过重要的连通性变化通知。

---

## 通知发送模式

| Option | Description | Selected |
|--------|-------------|----------|
| 同步发送(阻塞检查循环) | 在状态变化的 goroutine 中直接调用 Notifier.Notify()。简单直接,但如果 Pushover API 慢或失败,会阻塞下次连通性检查。 | |
| 异步发送(独立 goroutine,推荐) | 将通知发送放入独立的 goroutine 执行。不阻塞连通性检查循环,即使 Pushover API 慢也不影响监控。推荐做法。 | ✓ |
| 带缓冲队列的异步发送 | 使用 channel 作为通知队列,后台 worker 消费队列发送通知。最复杂但最可靠,支持重试、批量发送等高级功能。适合高可靠性场景。 | |

**User's choice:** 异步发送(独立 goroutine,推荐)
**Notes:** 用户选择推荐的异步发送方式，确保监控系统稳定性不受 Pushover API 影响。

---

## 冷却时间长度

| Option | Description | Selected |
|--------|-------------|----------|
| 1 分钟（快速响应） | 状态变化后等待 1 分钟确认。快速通知，适合需要及时了解网络的场景。 | ✓ |
| 2 分钟（平衡，推荐） | 状态变化后等待 2 分钟确认。平衡响应速度和稳定性，推荐。 | |
| 5 分钟（保守） | 状态变化后等待 5 分钟确认。更保守，避免短期波动，但通知延迟较长。 | |

**User's choice:** 1 分钟（快速响应）
**Notes:** 用户选择 1 分钟冷却时间，快速响应用户对网络状态的需求。

---

## 架构组织

| Option | Description | Selected |
|--------|-------------|----------|
| 在 NetworkMonitor 内部实现 | 在 NetworkMonitor 内部添加冷却逻辑：状态变化后启动 timer，冷却期满后再检查状态，如果仍保持则发送通知。简单直接，监控和通知耦合在一起。 | |
| 独立的 NotificationManager（推荐） | 创建独立的 NotificationManager，监听 NetworkMonitor 的状态变化事件，内部处理冷却逻辑和通知发送。职责分离，更易测试和维护，推荐。 | ✓ |
| Channel 事件驱动 | 在 NetworkMonitor 中检测到状态变化后，通过 channel 发送事件到通知 goroutine，通知 goroutine 处理冷却和发送。中等复杂度，监控和通知通过 channel 解耦。 | |

**User's choice:** 独立的 NotificationManager（推荐）
**Notes:** 用户选择推荐的职责分离架构，创建独立的 NotificationManager 处理通知逻辑。

---

## 通知发送失败处理

| Option | Description | Selected |
|--------|-------------|----------|
| 仅记录 ERROR 日志 | 通知发送失败时记录 ERROR 日志，包含错误详情。服务继续运行，下次状态变化会再次尝试。简单可靠。 | ✓ |
| 重试一次（1 分钟后） | 通知发送失败时记录 ERROR 日志，并在 1 分钟后重试一次。如果重试仍失败，记录 ERROR 并放弃。提高通知成功率，但增加复杂度。 | |
| 降级为 WARN 日志（非关键） | 通知发送失败时记录 WARN 日志（非 ERROR），表示非关键错误。监控服务的核心是连通性检查，通知是辅助功能，失败不应视为严重错误。 | |

**User's choice:** 仅记录 ERROR 日志
**Notes:** 用户选择简单可靠的错误处理方式，不实现重试机制。下次状态变化会再次尝试通知。

---

## Claude's Discretion

- 订阅机制的具体实现(channel vs 轮询)
- NotificationManager 的具体结构设计
- 冷却时间 timer 的管理方式(重置、取消)
- 通知失败时的具体日志格式
- 通知发送 goroutine 的错误处理细节

## Deferred Ideas

- 详细诊断信息通知 — 当前简洁通知足够,如需详细信息可查看应用日志
- 通知重试机制 — 当前失败仅记录日志,如需提高通知可靠性可添加重试逻辑
- 通知配置开关 — 当前 Pushover 可选配置即可,如需更细粒度控制可添加 enable_notify 开关
- 可配置冷却时间 — 当前固定 1 分钟,如需灵活性可添加到 monitor 配置中
- 多种通知渠道 — 当前仅 Pushover,如需支持邮件、Slack 等需要新的抽象层

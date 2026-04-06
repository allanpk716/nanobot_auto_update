# Phase 43: Telegram Monitor Integration - Context

**Gathered:** 2026-04-06
**Status:** Ready for planning

<domain>
## Phase Boundary

将 Phase 42 构建的 TelegramMonitor 接入实例生命周期管理。每个实例启动后自动激活 Telegram 连接监控，停止时立即取消正在进行的监控。实例不产生 "Starting Telegram bot" 日志时零开销运行。

Requirements: TELE-07 (无 trigger 不启动监控), TELE-09 (停止时取消监控)
Depends on: Phase 42 (monitor core complete and tested)

</domain>

<decisions>
## Implementation Decisions

### Wiring 架构
- **D-01:** TelegramMonitor 由 InstanceLifecycle 内部管理 — 添加 `telegramMonitor` 字段，`StartAfterUpdate()` 成功后创建并 goroutine 启动，`StopForUpdate()` 停止进程前调用 `monitor.Stop()`
- **D-02:** 与现有模式一致 — 每个 InstanceLifecycle 拥有自己的 LogBuffer、logger，TelegramMonitor 同级管理

### Notifier 注入
- **D-03:** 构造函数注入 — 修改 `NewInstanceLifecycle(cfg, baseLogger, notifier)` 签名增加 Notifier 参数
- **D-04:** Notifier 不可变 — 构造后不替换，与项目 DI 模式一致
- **D-05:** 需更新所有 NewInstanceLifecycle 调用点（manager.go 中的 NewInstanceManager）

### 测试策略
- **D-06:** 单元测试为主 — Mock LogSubscriber + Notifier，验证 InstanceLifecycle 中 monitor 的创建/启动/停止逻辑
- **D-07:** 与 Phase 42 测试风格一致 — 快速执行，race detector 验证

### Claude's Discretion
- monitor goroutine 的 panic recovery 实现细节
- 日志消息的具体措辞
- 测试用例的边界条件选择

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Telegram Monitor Core (Phase 42 产出)
- `internal/telegram/monitor.go` — TelegramMonitor struct, Start/Stop/processEntry, duck-typed interfaces (LogSubscriber, Notifier)
- `internal/telegram/patterns.go` — TriggerPattern, SuccessPattern, FailurePattern 常量
- `internal/telegram/monitor_test.go` — StopCancelsTimer (TELE-09 验证), ContextCancelledBeforeTrigger (TELE-07 验证)

### Instance Lifecycle (集成目标)
- `internal/instance/lifecycle.go` — InstanceLifecycle struct, StartAfterUpdate, StopForUpdate, GetLogBuffer
- `internal/instance/manager.go` — InstanceManager, StartAllInstances (auto-start), startAll (update flow)

### Notification (依赖)
- `internal/notifier/notifier.go` — Notifier struct, satisfies telegram.Notifier via duck typing

### Log Infrastructure
- `internal/logbuffer/buffer.go` — LogBuffer, Write, GetHistory, Clear
- `internal/logbuffer/subscriber.go` — Subscribe/Unsubscribe channel mechanics, history replay

### Wiring Reference
- `cmd/nanobot-auto-updater/main.go` — 主接线入口，Notifier 和 InstanceManager 创建位置

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `telegram.TelegramMonitor`: 完整的状态机实现，Start() 阻塞读取 subscription channel，Stop() 取消 timer + context
- `telegram.LogSubscriber` / `telegram.Notifier`: 鸭子类型接口，已被 `*logbuffer.LogBuffer` 和 `*notifier.Notifier` 满足
- `logbuffer.LogBuffer.Subscribe()`: 返回 `<-chan LogEntry`（容量 100），先发送历史再实时推送
- `logbuffer.LogBuffer.Unsubscribe(ch)`: 清理 subscriber goroutine
- `notifier.Notifier`: 已注入 main.go，可直接传给 InstanceLifecycle

### Established Patterns
- 每个 InstanceLifecycle 拥有独立 LogBuffer — 天然支持 per-instance 监控
- 构造函数注入（不可变依赖）— NewInstanceLifecycle 已有 cfg + logger 模式
- 鸭子类型接口 — 避免包循环依赖
- 异步 goroutine + panic recovery — 通知不阻塞主流程
- 优雅降级 — 失败不中断整体

### Integration Points
- `InstanceLifecycle.StartAfterUpdate()` → 启动成功后 → 创建 TelegramMonitor + `go monitor.Start(ctx)`
- `InstanceLifecycle.StopForUpdate()` → 停止进程前 → `monitor.Stop()` 取消监控
- `NewInstanceLifecycle()` → 新增 Notifier 参数 → 存储 notifier 用于创建 TelegramMonitor
- `NewInstanceManager()` → 传递 Notifier 到每个 NewInstanceLifecycle 调用
- `main.go` auto-start goroutine → 无需修改（InstanceLifecycle 内部处理）

</code_context>

<specifics>
## Specific Ideas

- TELE-07 已由 monitor 设计保证：Start() 阻塞读取 channel，只在看到 trigger 后激活状态机，无 trigger 则零开销
- TELE-09 已由 monitor.Stop() 保证：取消 timer + cancel context，不发 timeout/failure 通知
- TelegramMonitor.Start() 需要独立 context（非 instance lifecycle ctx），以便 Stop() 能独立取消
- main.go 中的 Notifier 变量（`notif`）已满足 telegram.Notifier 接口

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 43-telegram-monitor-integration*
*Context gathered: 2026-04-06*

# Phase 43: Telegram Monitor Integration - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-06
**Phase:** 43-telegram-monitor-integration
**Areas discussed:** Wiring 架构, Notifier 注入, 测试策略

---

## Wiring 架构

| Option | Description | Selected |
|--------|-------------|----------|
| InstanceLifecycle 内部 | 添加 telegramMonitor 字段，StartAfterUpdate 后创建，StopForUpdate 前停止 | ✓ |
| InstanceManager 外部管理 | 用 map 跟踪 monitor，解耦但传递层次多 | |

**User's choice:** InstanceLifecycle 内部
**Notes:** 与现有 InstanceLifecycle 拥有 LogBuffer/logger 的模式一致

---

## Notifier 注入

| Option | Description | Selected |
|--------|-------------|----------|
| 构造函数注入 | 修改 NewInstanceLifecycle 签名增加 notifier 参数 | ✓ |
| 方法注入 | 添加 SetTelegramNotifier() 方法，运行时可变 | |

**User's choice:** 构造函数注入
**Notes:** 不可变依赖，与项目 DI 模式一致

---

## 测试策略

| Option | Description | Selected |
|--------|-------------|----------|
| 单元测试为主 | Mock LogSubscriber + Notifier，快速可靠 | ✓ |
| 单元 + 集成测试 | 额外增加真实 LogBuffer + TelegramMonitor 集成测试 | |

**User's choice:** 单元测试为主
**Notes:** 与 Phase 42 测试风格一致，race detector 验证

---

## Claude's Discretion

- monitor goroutine panic recovery 实现细节
- 日志消息措辞
- 测试边界条件选择

## Deferred Ideas

None — discussion stayed within phase scope

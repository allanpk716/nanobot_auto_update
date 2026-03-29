# Phase 35: Notification Integration Testing - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-29
**Phase:** 35-notification-integration-testing
**Areas discussed:** 通知验证策略, E2E 测试范围, Pushover 失败模拟

---

## 通知验证策略

| Option | Description | Selected |
|--------|-------------|----------|
| 方案 A: Interface 重构 | 定义 Notifier interface，TriggerHandler 改用接口，测试注入 recording mock | ✓ |
| 方案 B: Fake Pushover Server | 不改生产代码，用 fake HTTP server 拦截 Pushover API 请求验证内容 | |
| 方案 C: 仅 helper 测试 | 零改动，仅测试辅助函数 + nil/disabled 覆盖 | |

**User's choice:** 方案 A: Interface 重构
**Notes:** Interface 允许精确验证每次 Notify 调用的 title 和 message，改动范围可控（trigger.go 字段类型 + server.go 参数类型）

---

## E2E 测试范围

| Option | Description | Selected |
|--------|-------------|----------|
| 4 个 E2E 测试 | 每个成功标准 1 个测试，与 Phase 33 模式一致 | ✓ |
| 6+ 个细粒度测试 | 拆分更细，成功/部分成功/失败各一个完成通知测试 | |
| 2 个综合测试 | 正常流 + 异常流 2 个大测试 | |

**User's choice:** 4 个 E2E 测试
**Notes:** 与 Phase 33 的 4 测试结构保持一致，简单明确

---

## Pushover 失败模拟

| Option | Description | Selected |
|--------|-------------|----------|
| Mock 返回 error | recordingNotifier.Notify() 返回 error，验证 HTTP 200 + JSON + UpdateLog 正常 | ✓ |
| Fake HTTP 500 server | 启动返回 500 的 fake HTTP server + 真实 Notifier | |

**User's choice:** Mock 返回 error
**Notes:** Interface mock 足够模拟失败场景，无需 fake server 的复杂度

---

## Claude's Discretion

- Interface 定义的具体位置（trigger.go 内 or 独立文件）
- recordingNotifier 的具体实现细节
- 每个 E2E 测试的内部结构（table-driven vs 独立函数）
- 异步通知的测试同步机制

## Deferred Ideas

None — 讨论保持在阶段范围内

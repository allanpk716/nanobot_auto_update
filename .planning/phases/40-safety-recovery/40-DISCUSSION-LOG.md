# Phase 40: Safety & Recovery - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-30
**Phase:** 40-safety-recovery
**Areas discussed:** 重启策略, 通知集成, .old 处理逻辑, 端口重绑策略

---

## 重启策略

| Option | Description | Selected |
|--------|-------------|----------|
| 直接退出（推荐） | 更新成功后 cmd.Start + os.Exit(0)，跳过 graceful shutdown。简单快速 | ✓ |
| 优雅关闭后重启 | 先走完整的 graceful shutdown，然后 cmd.Start + os.Exit(0)。更安全但更慢 | |
| 不重启 | 标记状态为 updated，等待用户手动重启。最安全但需用户介入 | |

**User's choice:** 直接退出（推荐）
**Notes:** PoC 已验证此模式，端口重用风险低（旧进程立即释放资源）

---

## 通知集成

| Option | Description | Selected |
|--------|-------------|----------|
| 复用 Notifier interface（推荐） | 通过构造函数注入，与 TriggerHandler 保持一致 | ✓ |
| 直接调用 notifier | 不通过 interface，直接依赖 *notifier.Notifier。更简单但耦合更高 | |

**User's choice:** 复用 Notifier interface（推荐）

---

## 通知发送时机

| Option | Description | Selected |
|--------|-------------|----------|
| 开始 + 完成（推荐） | 开始时发一次（含版本信息），完成时再发一次（含结果）。与 TriggerHandler 模式一致 | ✓ |
| 仅完成时 | 只在成功或失败时发送，减少通知打扰 | |

**User's choice:** 开始 + 完成（推荐）

---

## .old 处理逻辑

| Option | Description | Selected |
|--------|-------------|----------|
| 状态文件标记（推荐） | 更新成功后写 .update-success 状态文件，启动时检查状态文件决定清理 vs 恢复 | ✓ |
| 仅清理，不恢复 | 总是清理 .old 文件，不做自动恢复。最简单但不满足 SAFE-04 | |
| 启动计数器 | 记录启动次数，首次启动恢复 .old，非首次清理。复杂度较高 | |

**User's choice:** 状态文件标记（推荐）
**Notes:** 状态文件存在 → 正常清理 .old。状态文件不存在但 .old 存在 → 上次更新后崩溃 → 恢复 .old

---

## 端口重绑策略

| Option | Description | Selected |
|--------|-------------|----------|
| 重试机制（推荐） | 端口绑定失败时等待 500ms 重试，最多 5 次（共 2.5s）。简单实用 | ✓ |
| 固定延迟 | 新进程启动后先 Sleep 1s。最简单但固定延迟 | |
| 不处理 | 正常绑定不加重试，失败就记日志。风险较高 | |

**User's choice:** 重试机制（推荐）

---

## Claude's Discretion

- 状态文件的具体命名和存放路径
- 通知消息的具体格式和标题
- 端口重试的具体实现方式
- .old 恢复的具体实现（文件复制 vs rename）
- 日志字段命名和上下文注入方式

## Deferred Ideas

None — discussion stayed within phase scope

# Requirements: v0.7 Update Lifecycle Notifications

**Milestone:** v0.7 — Update Lifecycle Notifications
**Created:** 2026-03-29
**Status:** Active

## Goal

在 HTTP API 触发的 nanobot 更新流程中，发送 Pushover 通知告知用户更新状态。

## Update Notifications (UNOTIF)

### UNOTIF-01: Update Start Notification
用户在更新流程开始时收到 Pushover 通知，包含触发来源和待更新实例数量。

**Acceptance criteria:**
- TriggerHandler 收到有效更新请求后，在执行 TriggerUpdate 之前发送 Pushover 通知
- 通知标题包含"更新开始"
- 通知内容包含触发来源（api-trigger）和实例数量
- 异步发送，不阻塞更新流程

### UNOTIF-02: Update Completion Notification
用户在更新流程完成后收到 Pushover 通知，包含更新状态、耗时和各实例简要结果。

**Acceptance criteria:**
- 更新完成后（无论成功或失败）发送 Pushover 通知
- 通知包含三态状态：success / partial_success / failed
- 通知包含更新耗时（秒）
- 通知包含各实例的停止/启动状态摘要
- 使用与 v0.6 UpdateLog 相同的三态分类逻辑

### UNOTIF-03: Non-blocking Notification
通知发送为异步非阻塞，发送失败不影响更新流程和 API 响应。

**Acceptance criteria:**
- 通知发送在独立 goroutine 中执行
- Pushover API 错误不影响 TriggerHandler 的正常响应
- 通知失败记录 ERROR 级别日志
- 更新结果和 UpdateLog 记录不受通知失败影响

### UNOTIF-04: Graceful Degradation
Pushover 未配置时，跳过通知发送并记录日志（非阻塞降级）。

**Acceptance criteria:**
- Notifier.IsEnabled() 返回 false 时跳过所有通知发送
- 记录 DEBUG 级别日志（非警告，通知是可选功能）
- 不影响更新流程的任何功能

## Future Requirements

- 定时更新（cron）触发的更新通知
- 通知模板自定义
- 通知发送频率限制或冷却机制

## Out of Scope

- **Cron-triggered notifications**: 本次仅关注 HTTP API 触发的更新通知，定时更新不在范围内
- **Notification templates**: 不支持自定义通知内容模板
- **Rate limiting**: 不限制通知发送频率
- **Other notification channels**: 仅支持 Pushover，不扩展到其他通知渠道

## Traceability

| REQ-ID | Phase | Plan | Status |
|--------|-------|------|--------|
| UNOTIF-01 | Phase 34 | 34-01 | Complete |
| UNOTIF-02 | Phase 34 | 34-01 | Complete |
| UNOTIF-03 | Phase 34 | 34-01 | Complete |
| UNOTIF-04 | Phase 34 | 34-01 | Complete |

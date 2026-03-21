---
phase: 27
slug: network-monitoring-notifications
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-21
---

# Phase 27 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify (已存在) |
| **Config file** | None — standard Go test pattern |
| **Quick run command** | `go test ./internal/notification/... -v` |
| **Full suite command** | `go test ./... -v -race` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/notification/... -v`
- **After every plan wave:** Run `go test ./... -v -race`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 27-01-01 | 01 | 1 | MONITOR-04 | unit | `go test ./internal/notification -run TestRecoveryNotification -v` | ❌ W0 | ⬜ pending |
| 27-01-02 | 01 | 1 | MONITOR-05 | unit | `go test ./internal/notification -run TestFailureNotification -v` | ❌ W0 | ⬜ pending |
| 27-01-03 | 01 | 1 | MONITOR-04/05 | unit | `go test ./internal/notification -run TestCooldownTimer -v` | ❌ W0 | ⬜ pending |
| 27-01-04 | 01 | 1 | MONITOR-04/05 | unit | `go test ./internal/notification -run TestDisabledNotifier -v` | ❌ W0 | ⬜ pending |
| 27-01-05 | 01 | 1 | MONITOR-04/05 | unit | `go test ./internal/notification -run TestAsyncNotification -v` | ❌ W0 | ⬜ pending |
| 27-01-06 | 01 | 1 | MONITOR-04/05 | unit | `go test ./internal/notification -run TestStateChangeDetection -v` | ❌ W0 | ⬜ pending |
| 27-02-01 | 02 | 2 | MONITOR-04/05 | integration | 手动验证 main.go 启动/关闭顺序 | N/A | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/notification/manager_test.go` — NotificationManager 核心逻辑测试(stubs for MONITOR-04/05)
- [ ] `internal/notification/manager_test.go` — 冷却时间测试
- [ ] `internal/notification/manager_test.go` — Mock NetworkMonitor 和 Notifier
- [ ] `internal/network/monitor_test.go` — GetState() 线程安全测试(如果修改 NetworkMonitor)

*如果 Phase 27 仅新增 NotificationManager,测试框架已存在,仅需编写测试文件。*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| 应用生命周期集成 | MONITOR-04/05 | 需要 main.go 启动应用,观察组件启动顺序和日志输出 | 1. 启动应用 2. 检查日志中 "通知管理器已启动" 消息 3. 使用 Ctrl+C 关闭应用 4. 检查日志中 "通知管理器已停止" 消息 |
| 冷却时间实际效果 | MONITOR-04/05 | 需要等待 1+ 分钟观察真实 timer 行为 | 1. 断开网络 2. 等待 15 分钟触发连通性检查 3. 观察日志 "启动冷却确认" 4. 在 1 分钟内恢复网络 5. 观察日志 "冷却期内状态已恢复,取消通知" |

*All phase behaviors have automated verification except integration tests.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

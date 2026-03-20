---
phase: 24
slug: auto-start
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-03-20
---

# Phase 24 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — Wave 0 installs |
| **Quick run command** | `go test -v -short ./internal/instance` |
| **Full suite command** | `go test -v ./internal/instance ./internal/config ./cmd/nanobot-auto-updater` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -v -short ./internal/instance`
- **After every plan wave:** Run `go test -v ./internal/instance ./internal/config ./cmd/nanobot-auto-updater`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 24-00-01 | 00 | 0 | AUTOSTART-01 | unit | `grep -n "TestInstanceConfigAutoStart" internal/config/instance_test.go` | ✅ W0 | ⬜ pending |
| 24-00-02 | 00 | 0 | AUTOSTART-02 | unit | `grep -n "TestStartAllInstances" internal/instance/manager_test.go` | ✅ W0 | ⬜ pending |
| 24-00-03 | 00 | 0 | AUTOSTART-03 | unit | `grep -n "TestStartAllInstancesGracefulDegradation" internal/instance/manager_test.go` | ✅ W0 | ⬜ pending |
| 24-00-04 | 00 | 0 | AUTOSTART-04 | unit | `grep -n "TestStartAllInstancesSummary" internal/instance/manager_test.go` | ✅ W0 | ⬜ pending |
| 24-01-01 | 01 | 1 | AUTOSTART-01 | unit | `go test -v -run TestInstanceConfigAutoStart ./internal/config` | ✅ W0 | ⬜ pending |
| 24-02-01 | 02 | 2 | AUTOSTART-01 | unit | `go test -v -run TestInstanceLifecycleHelpers ./internal/instance` | ✅ W0 | ⬜ pending |
| 24-02-02 | 02 | 2 | AUTOSTART-02 | unit | `go test -v -run TestStartAllInstances ./internal/instance` | ✅ W0 | ⬜ pending |
| 24-02-03 | 02 | 2 | AUTOSTART-03 | unit | `go test -v -run TestStartAllInstances_GracefulDegradation ./internal/instance` | ✅ W0 | ⬜ pending |
| 24-02-04 | 02 | 2 | AUTOSTART-04 | unit | `go test -v -run TestStartAllInstances_Summary ./internal/instance` | ✅ W0 | ⬜ pending |
| 24-03-01 | 03 | 3 | AUTOSTART-01 | integration | `grep -n "StartAllInstances" cmd/nanobot-auto-updater/main.go` | ✅ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `internal/config/instance_test.go` — test stubs for AutoStart field (AUTOSTART-01) — Plan 24-00
- [x] `internal/instance/manager_test.go` — test stubs for StartAllInstances (AUTOSTART-02, 03, 04) — Plan 24-00
- [x] `internal/instance/manager_test.go` — test stubs for InstanceLifecycle helpers (AUTOSTART-01 indirect) — Plan 24-00
- [x] No additional framework install needed — go test available

*Wave 0 plan (24-00-PLAN.md) created to address Nyquist compliance.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| 应用启动日志格式 | AUTOSTART-02 | 日志格式和措辞难以自动化验证 | 启动应用，检查日志输出包含 "正在启动实例", "实例启动成功", "自动启动完成" |
| 汇总日志可读性 | AUTOSTART-04 | 日志汇总格式需要人工确认清晰度 | 启动应用，确认汇总日志显示成功/失败数量和失败实例名称 |
| API 优先启动体验 | AUTOSTART-02 | 启动时序需要人工确认 | 启动应用，确认 API 端点先于实例启动可用 |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (Plan 24-00 created)
- [x] No watch-mode flags
- [x] Feedback latency < 10s
- [x] `nyquist_compliant: true` set in frontmatter
- [x] AUTOSTART-03 and AUTOSTART-04 have independent test coverage (24-02-03, 24-02-04)

**Approval:** pending

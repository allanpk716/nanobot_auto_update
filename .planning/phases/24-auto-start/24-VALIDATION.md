---
phase: 24
slug: auto-start
status: draft
nyquist_compliant: false
wave_0_complete: false
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
| 24-01-01 | 01 | 1 | AUTOSTART-01 | unit | `go test -v -run TestAutoStartDefault ./internal/config` | ❌ W0 | ⬜ pending |
| 24-01-02 | 01 | 1 | AUTOSTART-01 | unit | `go test -v -run TestAutoStartValidation ./internal/config` | ❌ W0 | ⬜ pending |
| 24-02-01 | 02 | 1 | AUTOSTART-02 | unit | `go test -v -run TestStartAllInstances ./internal/instance` | ❌ W0 | ⬜ pending |
| 24-02-02 | 02 | 1 | AUTOSTART-02 | unit | `go test -v -run TestStartAllInstancesSkipDisabled ./internal/instance` | ❌ W0 | ⬜ pending |
| 24-03-01 | 03 | 2 | AUTOSTART-03 | unit | `go test -v -run TestStartAllInstancesGracefulDegradation ./internal/instance` | ❌ W0 | ⬜ pending |
| 24-04-01 | 04 | 2 | AUTOSTART-04 | integration | `go test -v -run TestAutoStartIntegration ./cmd/nanobot-auto-updater` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/config/instance_test.go` — test stubs for AutoStart field (AUTOSTART-01)
- [ ] `internal/instance/manager_test.go` — test stubs for StartAllInstances (AUTOSTART-02, 03)
- [ ] `cmd/nanobot-auto-updater/main_test.go` — integration test stubs for auto-start (AUTOSTART-04)
- [ ] No additional framework install needed — go test available

*Existing infrastructure covers all phase requirements with go test framework.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| 应用启动日志格式 | AUTOSTART-02 | 日志格式和措辞难以自动化验证 | 启动应用，检查日志输出包含 "Starting instance", "started successfully", "Auto-start completed" |
| 汇总日志可读性 | AUTOSTART-04 | 日志汇总格式需要人工确认清晰度 | 启动应用，确认汇总日志显示成功/失败数量和失败实例名称 |
| API 优先启动体验 | AUTOSTART-02 | 启动时序需要人工确认 | 启动应用，确认 API 端点先于实例启动可用 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

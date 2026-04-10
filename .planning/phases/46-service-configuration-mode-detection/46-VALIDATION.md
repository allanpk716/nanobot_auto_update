---
phase: 46
slug: service-configuration-mode-detection
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-10
---

# Phase 46 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify (assert/require) |
| **Config file** | none |
| **Quick run command** | `go test ./internal/config/... -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/config/... -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 46-01-01 | 01 | 1 | MGR-01 | — | N/A | unit | `go test ./internal/config/... -run TestServiceConfig -v` | Wave 0 | pending |
| 46-01-02 | 01 | 1 | MGR-01 | — | N/A | unit | `go test ./internal/config/... -run TestServiceConfig_Defaults -v` | Wave 0 | pending |
| 46-01-03 | 01 | 1 | MGR-01 | T-46-02 | service_name 仅字母数字 | unit | `go test ./internal/config/... -run TestServiceConfigValidate -v` | Wave 0 | pending |
| 46-02-01 | 02 | 1 | SVC-01 | T-46-01, T-46-03 | 检测管理员权限+服务环境 | manual | N/A (Phase 47 集成) | N/A | pending |

*Status: pending / green / red / flaky*

---

## Wave 0 Requirements

- [ ] `internal/config/service_test.go` — stubs for MGR-01 (ServiceConfig 解析、默认值、验证)
- [ ] SVC-01 集成测试标记为 manual-only，Phase 47 实现 svc.Handler 时补充

*Existing infrastructure covers most phase requirements (Go testing framework already in place).*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| svc.IsWindowsService() 在 SCM 上下文返回 true | SVC-01 | 需要 Windows Service 控制管理器环境 | sc create test && sc start test && 检查日志 |
| 控制台运行 + auto_start=true 自动注册服务 | SVC-01 (D-08) | 需要管理员权限 + SCM 交互 | 以管理员运行程序，检查退出码=2 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

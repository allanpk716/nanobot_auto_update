---
phase: 21
slug: instance-management-integration
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-17
---

# Phase 21 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing package (stdlib) |
| **Config file** | none - tests self-contained |
| **Quick run command** | `go test ./internal/instance -run TestInstanceLifecycle -v` |
| **Full suite command** | `go test ./internal/instance ./internal/logbuffer -v` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/instance -run <specific_test> -v`
- **After every plan wave:** Run `go test ./internal/instance ./internal/logbuffer -v`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 21-01-01 | 01 | 1 | INST-05 | unit | `go test ./internal/logbuffer -run TestLogBuffer_Clear -v` | ❌ W0 | ⬜ pending |
| 21-02-01 | 02 | 1 | INST-01 | unit | `go test ./internal/instance -run TestNewInstanceLifecycle_LogBuffer -v` | ❌ W0 | ⬜ pending |
| 21-02-02 | 02 | 1 | INST-03 | unit | `go test ./internal/instance -run TestInstanceLifecycle_StartWithCapture -v` | ❌ W0 | ⬜ pending |
| 21-02-03 | 02 | 1 | INST-04 | unit | `go test ./internal/instance -run TestInstanceLifecycle_StopPreservesBuffer -v` | ❌ W0 | ⬜ pending |
| 21-02-04 | 02 | 1 | INST-05 | unit | `go test ./internal/instance -run TestInstanceLifecycle_StartClearsBuffer -v` | ❌ W0 | ⬜ pending |
| 21-03-01 | 03 | 1 | INST-02 | unit | `go test ./internal/instance -run TestInstanceManager_GetLogBuffer -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/logbuffer/buffer_test.go` — add test for Clear() method (INST-05 support)
- [ ] `internal/instance/lifecycle_test.go` — add tests for INST-01, INST-03, INST-04, INST-05
- [ ] `internal/instance/manager_test.go` — add test for INST-02 (GetLogBuffer)
- [ ] Framework install: none required — Go testing package already in use

*Existing infrastructure covers all phase requirements.*

---

## Manual-Only Verifications

All phase behaviors have automated verification.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

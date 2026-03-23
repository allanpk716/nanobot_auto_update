---
phase: 28
slug: http-api-trigger
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-23
---

# Phase 28 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + stretchr/testify v1.11.1 |
| **Config file** | none - 使用 *_test.go 文件 |
| **Quick run command** | `go test ./internal/api/... -v -run TestTrigger` |
| **Full suite command** | `go test ./... -v` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/api/... ./internal/instance/... -v`
- **After every plan wave:** Run `go test ./... -v`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 28-01-01 | 01 | 1 | API-02 | unit | `go test ./internal/api/... -v -run TestAuthMiddleware` | ❌ W0 | ⬜ pending |
| 28-01-02 | 01 | 1 | API-05 | unit | `go test ./internal/api/... -v -run TestAuthMiddleware_Unauthorized` | ❌ W0 | ⬜ pending |
| 28-02-01 | 02 | 1 | API-06 | unit | `go test ./internal/instance/... -v -run TestTriggerUpdate_Concurrent` | ❌ W0 | ⬜ pending |
| 28-02-02 | 02 | 1 | API-03 | integration | `go test ./internal/instance/... -v -run TestTriggerUpdate` | ❌ W0 | ⬜ pending |
| 28-03-01 | 03 | 1 | API-01 | unit | `go test ./internal/api/... -v -run TestTriggerHandler_Handle` | ❌ W0 | ⬜ pending |
| 28-03-02 | 03 | 1 | API-04 | unit | `go test ./internal/api/... -v -run TestTriggerHandler_JSONResponse` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/api/trigger_test.go` — stubs for API-01, API-04, API-05
- [ ] `internal/api/auth_test.go` — stubs for API-02
- [ ] `internal/instance/manager_test.go` — update to cover API-03, API-06 (add TestTriggerUpdate)
- [ ] Framework install: no additional install needed (testify already in go.mod)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| None | - | - | - |

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

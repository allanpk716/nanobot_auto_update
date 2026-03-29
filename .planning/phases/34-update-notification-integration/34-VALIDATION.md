---
phase: 34
slug: update-notification-integration
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-29
---

# Phase 34 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + stretchr/testify v1.11.1 |
| **Config file** | none — Go standard testing |
| **Quick run command** | `go test ./internal/api/ -count=1 -run TestTriggerHandler -v` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/api/ -count=1 -run TestTriggerHandler -v`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 34-01-01 | 01 | 1 | UNOTIF-01 | unit | `go test ./internal/api/ -count=1 -run TestTriggerHandler_StartNotification -v` | Wave 0 | pending |
| 34-01-02 | 01 | 1 | UNOTIF-02 | unit | `go test ./internal/api/ -count=1 -run TestTriggerHandler_CompletionNotification -v` | Wave 0 | pending |
| 34-01-03 | 01 | 1 | UNOTIF-03 | unit | `go test ./internal/api/ -count=1 -run TestTriggerHandler_NotificationFailureNonBlocking -v` | Wave 0 | pending |
| 34-01-04 | 01 | 1 | UNOTIF-04 | unit | `go test ./internal/api/ -count=1 -run TestTriggerHandler_DisabledNotification -v` | Wave 0 | pending |

*Status: pending / green / red / flaky*

---

## Wave 0 Requirements

- [ ] `internal/api/trigger_test.go` — test cases for UNOTIF-01/02/03/04, update `newTestHandler()` signature with notifier param
- [ ] Note: concrete `*notifier.Notifier` type — use nil or disabled notifier for testing

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Pushover notification received on phone | UNOTIF-01/02 | External service delivery verification | Trigger update via API, check phone for notification |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

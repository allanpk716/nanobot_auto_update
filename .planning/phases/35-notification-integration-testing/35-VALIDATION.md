---
phase: 35
slug: notification-integration-testing
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-29
---

# Phase 35 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (stdlib) v1.24.11 |
| **Config file** | none |
| **Quick run command** | `go test ./internal/api/ -count=1 -run "TestE2E_Notification" -v` |
| **Full suite command** | `go test ./internal/api/ -count=1 -v` |
| **Race detection command** | `go test ./internal/api/ -race -count=1 -v` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/api/ -count=1 -v`
- **After every plan wave:** Run `go test ./internal/api/ -race -count=1 -v`
- **Before `/gsd:verify-work`:** Full suite with race detection must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 35-01-01 | 01 | 1 | UNOTIF-01~04 | E2E | `go test ./internal/api/ -run TestE2E_Notification -v` | Wave 0 (new) | pending |
| 35-01-02 | 01 | 1 | Interface refactor | unit | `go test ./internal/api/ -count=1 -v` | Wave 0 (modify) | pending |

*Status: pending / green / red / flaky*

---

## Wave 0 Requirements

- [ ] `internal/api/trigger.go` — Add Notifier interface definition, update TriggerHandler field type and NewTriggerHandler parameter type
- [ ] `internal/api/server.go` — Update NewServer parameter type from `*notifier.Notifier` to `Notifier`
- [ ] `internal/api/trigger_test.go` — Update newTestHandler parameter type from `*notifier.Notifier` to `Notifier`
- [ ] `internal/api/integration_test.go` — Add 4 new TestE2E_Notification_* functions + recordingNotifier mock

---

## Manual-Only Verifications

All phase behaviors have automated verification.

---

## Validation Sign-Off

- [ ] All tasks have automated verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

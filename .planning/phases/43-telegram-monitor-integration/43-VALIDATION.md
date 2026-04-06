---
phase: 43
slug: telegram-monitor-integration
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-06
---

# Phase 43 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify v1.11.1 |
| **Config file** | none — standard `go test` |
| **Quick run command** | `go test ./internal/instance/... -run TestMonitor -count=1 -v` |
| **Full suite command** | `go test ./internal/instance/... -count=1 -v` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/instance/... -run TestMonitor -count=1 -v`
- **After every plan wave:** Run `go test ./internal/instance/... -count=1 -v`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 43-01-01 | 01 | 1 | TELE-07 | — | N/A | unit | `go test ./internal/instance/... -run TestMonitor_NoTriggerNoOverhead -count=1 -v` | W0 | ⬜ pending |
| 43-01-02 | 01 | 1 | TELE-09 | — | N/A | unit | `go test ./internal/instance/... -run TestMonitor_StopCancelsMonitor -count=1 -v` | W0 | ⬜ pending |
| 43-02-01 | 02 | 1 | TELE-07 | — | N/A | unit | `go test ./internal/instance/... -run TestMonitor_CreatedAfterStart -count=1 -v` | W0 | ⬜ pending |
| 43-02-02 | 02 | 1 | TELE-09 | — | N/A | unit | `go test ./internal/instance/... -run TestMonitor_StopNoMonitor -count=1 -v` | W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/instance/lifecycle_monitor_test.go` — covers TELE-07, TELE-09 integration tests
- [ ] Local mock types (mockLogSubscriber, mockNotifier) for instance package tests

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| End-to-end notification delivery | TELE-07 | Requires running instance + Pushover service | Start instance, wait for "Starting Telegram bot" log, verify Pushover notification |

*If none: "All phase behaviors have automated verification."*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

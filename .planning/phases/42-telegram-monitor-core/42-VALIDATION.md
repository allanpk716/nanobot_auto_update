---
phase: 42
slug: telegram-monitor-core
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-06
---

# Phase 42 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — existing |
| **Quick run command** | `go test ./internal/telegram/... -count=1 -v` |
| **Full suite command** | `go test ./internal/telegram/... -count=1 -race -v` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/telegram/... -count=1 -v`
- **After every plan wave:** Run `go test ./internal/telegram/... -count=1 -race -v`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 42-01-01 | 01 | 1 | TELE-01 | — | N/A | unit | `go test ./internal/telegram/... -run TestMonitor -count=1` | ❌ W0 | ⬜ pending |
| 42-01-02 | 01 | 1 | TELE-02 | — | N/A | unit | `go test ./internal/telegram/... -run TestMonitor_Success -count=1` | ❌ W0 | ⬜ pending |
| 42-01-03 | 01 | 1 | TELE-03 | — | N/A | unit | `go test ./internal/telegram/... -run TestMonitor_Failure -count=1` | ❌ W0 | ⬜ pending |
| 42-01-04 | 01 | 1 | TELE-04 | — | N/A | unit | `go test ./internal/telegram/... -run TestMonitor_Timeout -count=1` | ❌ W0 | ⬜ pending |
| 42-01-05 | 01 | 1 | TELE-05 | — | N/A | unit | `go test ./internal/telegram/... -run TestNotification_Success -count=1` | ❌ W0 | ⬜ pending |
| 42-01-06 | 01 | 1 | TELE-06 | — | N/A | unit | `go test ./internal/telegram/... -run TestNotification_Failure -count=1` | ❌ W0 | ⬜ pending |
| 42-01-07 | 01 | 1 | TELE-08 | — | N/A | unit | `go test ./internal/telegram/... -run TestHistoricalReplay -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/telegram/monitor_test.go` — stubs for TELE-01 through TELE-08
- [ ] `internal/telegram/patterns_test.go` — pattern matching tests

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|

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

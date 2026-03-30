---
phase: 39
slug: http-api-integration
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-30
---

# Phase 39 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — existing infrastructure |
| **Quick run command** | `go test ./internal/api/... -run SelfUpdate -v -count=1` |
| **Full suite command** | `go test ./internal/api/... -v -count=1` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/api/... -run SelfUpdate -v -count=1`
- **After every plan wave:** Run `go test ./internal/api/... -v -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 39-01-01 | 01 | 1 | API-03 | unit | `go test ./internal/api/... -run TestSelfUpdateCheck -v` | ❌ W0 | ⬜ pending |
| 39-01-02 | 01 | 1 | API-01 | unit | `go test ./internal/api/... -run TestSelfUpdateAuth -v` | ❌ W0 | ⬜ pending |
| 39-01-03 | 01 | 1 | API-02 | unit | `go test ./internal/api/... -run TestSelfUpdateMutex -v` | ❌ W0 | ⬜ pending |
| 39-02-01 | 02 | 2 | API-04 | unit | `go test ./internal/api/... -run TestHelpEndpoints -v` | ✅ exists | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/api/selfupdate_handler_test.go` — test stubs for SelfUpdateHandler

*Existing infrastructure (httptest, auth middleware tests) covers all other phase requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| None | - | - | All behaviors have automated verification |

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

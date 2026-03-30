---
phase: 38
slug: self-update-core
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-30
---

# Phase 38 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify v1.11.1 |
| **Config file** | none — existing Go test framework |
| **Quick run command** | `go test ./internal/selfupdate/ -v -count=1` |
| **Full suite command** | `go test ./internal/... -v -count=1` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/selfupdate/ -v -count=1`
- **After every plan wave:** Run `go test ./internal/... -v -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 38-01-01 | 01 | 1 | UPDATE-01 | unit | `go test ./internal/selfupdate/ -run TestCheckLatest -v` | ❌ W0 | ⬜ pending |
| 38-01-02 | 01 | 1 | UPDATE-01 | unit | `go test ./internal/selfupdate/ -run TestCheckLatest_APIError -v` | ❌ W0 | ⬜ pending |
| 38-01-03 | 01 | 1 | UPDATE-02 | unit | `go test ./internal/selfupdate/ -run TestNeedUpdate -v` | ❌ W0 | ⬜ pending |
| 38-01-04 | 01 | 1 | UPDATE-02 | unit | `go test ./internal/selfupdate/ -run TestNeedUpdate_Dev -v` | ❌ W0 | ⬜ pending |
| 38-02-01 | 02 | 1 | UPDATE-03 | unit | `go test ./internal/selfupdate/ -run TestVerifyChecksum -v` | ❌ W0 | ⬜ pending |
| 38-02-02 | 02 | 1 | UPDATE-03 | unit | `go test ./internal/selfupdate/ -run TestVerifyChecksum_Invalid -v` | ❌ W0 | ⬜ pending |
| 38-02-03 | 02 | 1 | UPDATE-04 | unit | `go test ./internal/selfupdate/ -run TestUpdate -v` | ❌ W0 | ⬜ pending |
| 38-02-04 | 02 | 1 | UPDATE-05 | unit | `go test ./internal/selfupdate/ -run TestApplyUpdate_OldSavePath -v` | ❌ W0 | ⬜ pending |
| 38-02-05 | 02 | 1 | UPDATE-06 | unit | `go test ./internal/selfupdate/ -run TestCache -v` | ❌ W0 | ⬜ pending |
| 38-02-06 | 02 | 1 | UPDATE-06 | unit | `go test ./internal/selfupdate/ -run TestCache_Expiry -v` | ❌ W0 | ⬜ pending |
| 38-02-07 | 02 | 2 | UPDATE-07 | unit | `go test ./internal/config/ -run TestSelfUpdateConfig -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/selfupdate/selfupdate_test.go` — stubs for UPDATE-01 through UPDATE-06
- [ ] `internal/config/selfupdate_test.go` — stubs for UPDATE-07 (config loading + defaults)
- [ ] No framework install needed — existing Go test framework and testify already available

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| None | — | — | — |

All phase behaviors have automated verification.

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

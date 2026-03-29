---
phase: 36
slug: poc-validation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-29
---

# Phase 36 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify v1.11.1 |
| **Config file** | none (standard go test) |
| **Quick run command** | `go build ./tmp/` |
| **Full suite command** | `go test ./tmp/ -run TestSelfUpdate -v -tags manual -timeout 60s` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go build ./tmp/`
- **After every plan wave:** Run `go test ./tmp/ -run TestSelfUpdate -v -tags manual -timeout 60s`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 36-01-01 | 01 | 1 | VALID-01 | integration | `go test ./tmp/ -run TestSelfUpdate -v -tags manual -timeout 60s` | W0 | pending |
| 36-01-02 | 01 | 1 | VALID-02 | integration | same test, checks `os.Stat("*.old")` | W0 | pending |
| 36-01-03 | 01 | 1 | VALID-03 | integration | same test, polls version file | W0 | pending |

*Status: pending / green / red / flaky*

---

## Wave 0 Requirements

- [ ] `tmp/poc_selfupdate.go` — PoC main program stubs
- [ ] `tmp/poc_selfupdate_test.go` — automated verification test
- [ ] `go get github.com/minio/selfupdate@v0.6.0` — dependency install

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Visual confirmation of .old file in filesystem | VALID-02 | Automated test checks existence but user may want to see it | Run PoC manually, check tmp/ for .old file |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

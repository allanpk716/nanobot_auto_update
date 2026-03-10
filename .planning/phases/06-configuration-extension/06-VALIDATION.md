---
phase: 06
slug: configuration-extension
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-10
---

# Phase 06 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — uses existing Go test infrastructure |
| **Quick run command** | `go test -v ./internal/config/...` |
| **Full suite command** | `go test -v ./...` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -v ./internal/config/...`
- **After every plan wave:** Run `go test -v ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 06-01-01 | 01 | 1 | CONF-01 | unit | `go test -v ./internal/config/... -run TestInstanceConfig` | ❌ W0 | ⬜ pending |
| 06-01-02 | 01 | 1 | CONF-02 | unit | `go test -v ./internal/config/... -run TestValidateUniqueNames` | ❌ W0 | ⬜ pending |
| 06-01-03 | 01 | 1 | CONF-03 | unit | `go test -v ./internal/config/... -run TestValidateUniquePorts` | ❌ W0 | ⬜ pending |
| 06-01-04 | 01 | 1 | CONF-01 | integration | `go test -v ./internal/config/... -run TestBackwardCompatibility` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/config/config_test.go` — test stubs for CONF-01, CONF-02, CONF-03
- [ ] Test fixtures for YAML config files (v1.0 and v2.0 formats)

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| None | N/A | N/A | All phase behaviors have automated verification |

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

---
phase: 11
slug: configuration-extension
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-16
---

# Phase 11 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — Wave 0 installs if needed |
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
| 11-01-01 | 01 | 1 | CONF-01 | unit | `go test -v ./internal/config/ -run TestConfigValidate` | ❌ W0 | ⬜ pending |
| 11-01-02 | 01 | 1 | CONF-02 | unit | `go test -v ./internal/config/ -run TestAPIConfigValidate` | ❌ W0 | ⬜ pending |
| 11-01-03 | 01 | 1 | CONF-03 | unit | `go test -v ./internal/config/ -run TestBearerTokenValidation` | ❌ W0 | ⬜ pending |
| 11-01-04 | 01 | 1 | CONF-04 | unit | `go test -v ./internal/config/ -run TestMonitorConfigValidate` | ❌ W0 | ⬜ pending |
| 11-01-05 | 01 | 1 | CONF-05 | unit | `go test -v ./internal/config/ -run TestDefaultValues` | ❌ W0 | ⬜ pending |
| 11-01-06 | 01 | 1 | CONF-06 | unit | `go test -v ./internal/config/ -run TestConfigLoad` | ❌ W0 | ⬜ pending |
| 11-01-07 | 01 | 1 | SEC-03 | unit | `go test -v ./internal/config/ -run TestTimingSafeComparison` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/config/config_test.go` — test stubs for CONF-01~06
- [ ] `internal/config/security_test.go` — test stubs for SEC-03 timing-safe comparison
- [ ] No framework install needed — Go testing already in use

*Existing infrastructure covers most phase requirements. Wave 0 adds config-specific test coverage.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Config file loading at startup | CONF-06 | Requires full application startup with real config.yaml | 1. Create config.yaml with all settings 2. Run `nanobot-auto-updater` 3. Verify startup logs show loaded config |

*Only startup integration requires manual verification. All config parsing/validation has automated tests.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

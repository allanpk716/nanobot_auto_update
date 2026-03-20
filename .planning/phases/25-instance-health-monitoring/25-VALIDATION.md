---
phase: 25
slug: instance-health-monitoring
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-20
---

# Phase 25 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — Wave 0 installs |
| **Quick run command** | `go test ./internal/health -v` |
| **Full suite command** | `go test ./... -v` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/health -v`
- **After every plan wave:** Run `go test ./... -v`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 25-01-01 | 01 | 1 | HEALTH-01 | unit | `go test ./internal/health -v -run TestMonitor_Start` | ❌ W0 | ⬜ pending |
| 25-01-02 | 01 | 1 | HEALTH-02 | unit | `go test ./internal/health -v -run TestMonitor_StateChange` | ❌ W0 | ⬜ pending |
| 25-01-03 | 01 | 1 | HEALTH-03 | unit | `go test ./internal/health -v -run TestMonitor_StateChange` | ❌ W0 | ⬜ pending |
| 25-01-04 | 01 | 1 | HEALTH-04 | unit | `go test ./internal/config -v -run TestHealthCheckConfig` | ❌ W0 | ⬜ pending |
| 25-02-01 | 02 | 1 | HEALTH-04 | unit | `go test ./internal/config -v -run TestHealthCheckConfig` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/health/monitor_test.go` — stubs for HEALTH-01, HEALTH-02, HEALTH-03
- [ ] `internal/config/health_test.go` — stubs for HEALTH-04
- [ ] `go test ./internal/health -v` — ensure test infrastructure works

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| None | - | - | All automated |

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

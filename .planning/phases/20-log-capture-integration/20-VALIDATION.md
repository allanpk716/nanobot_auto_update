---
phase: 20
slug: log-capture-integration
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-17
---

# Phase 20 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — Wave 0 installs |
| **Quick run command** | `go test -v ./internal/lifecycle/... -run TestLogCapture` |
| **Full suite command** | `go test -v ./internal/lifecycle/...` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -v ./internal/lifecycle/... -run TestLogCapture`
- **After every plan wave:** Run `go test -v ./internal/lifecycle/...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 20-01-01 | 01 | 1 | CAPT-01 | unit | `go test -v ./internal/lifecycle -run TestStartCapture` | ❌ W0 | ⬜ pending |
| 20-01-02 | 01 | 1 | CAPT-02 | unit | `go test -v ./internal/lifecycle -run TestConcurrentRead` | ❌ W0 | ⬜ pending |
| 20-02-01 | 02 | 1 | CAPT-03 | unit | `go test -v ./internal/lifecycle -run TestLogBufferWrite` | ❌ W0 | ⬜ pending |
| 20-03-01 | 03 | 2 | CAPT-04 | unit | `go test -v ./internal/lifecycle -run TestStopCapture` | ❌ W0 | ⬜ pending |
| 20-04-01 | 04 | 2 | CAPT-05 | integration | `go test -v ./internal/lifecycle -run TestProcessStartup` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/lifecycle/capture_test.go` — test stubs for CAPT-01 through CAPT-05
- [ ] Test fixtures for simulating nanobot process stdout/stderr output
- [ ] Integration test setup for full lifecycle capture testing

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Large output handling (>10MB) | CAPT-02 | Requires generating large test data that slows automated tests | Run nanobot with verbose logging, verify no deadlock and memory stable |
| Real-time capture performance | CAPT-03 | Timing-sensitive, not suitable for CI environment | Monitor LogBuffer write latency during nanobot operation |

*If none: "All phase behaviors have automated verification."*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

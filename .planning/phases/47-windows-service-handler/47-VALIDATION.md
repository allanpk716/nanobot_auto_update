---
phase: 47
slug: windows-service-handler
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-10
---

# Phase 47 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go testing |
| **Quick run command** | `go test ./internal/service/... ./internal/lifecycle/... -count=1 -v` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/service/... ./internal/lifecycle/... -count=1 -v`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 47-01-01 | 01 | 1 | SVC-02 | — | N/A | unit | `go test ./internal/lifecycle/... -run TestAppStartup -v` | ✅ W0 | ⬜ pending |
| 47-01-02 | 01 | 1 | SVC-02 | — | N/A | unit | `go test ./internal/lifecycle/... -run TestAppShutdown -v` | ✅ W0 | ⬜ pending |
| 47-02-01 | 02 | 1 | SVC-02 | T-47-01 | Service runs with minimal privileges | unit | `go test ./internal/service/... -run TestServiceHandler -v` | ✅ W0 | ⬜ pending |
| 47-02-02 | 02 | 1 | SVC-03 | — | N/A | unit | `go test ./internal/service/... -run TestGracefulShutdown -v` | ✅ W0 | ⬜ pending |
| 47-02-03 | 02 | 2 | SVC-02 | — | N/A | integration | `go test ./internal/service/... -run TestServiceIntegration -v` | ✅ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/lifecycle/lifecycle_test.go` — stubs for SVC-02, SVC-03
- [ ] `internal/service/handler_test.go` — stubs for SVC-02, SVC-03

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| SCM shows "Running" status | SVC-02 | Requires Windows Service registration | Install service with `sc create`, start it, verify in services.msc |
| SCM shows "Stopped" after stop | SVC-03 | Requires Windows Service registration | Stop service via `sc stop`, verify status in services.msc |
| Shutdown within 30s | SVC-03 | Timing requirement in real SCM | Stop service, measure time to "Stopped" status |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

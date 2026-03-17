---
phase: 20
slug: log-capture-integration
status: draft
nyquist_compliant: true
tdd_mode: true
created: 2026-03-17
---

# Phase 20 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none |
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

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | TDD | Status |
|---------|------|------|-------------|-----------|-------------------|-----|--------|
| 20-01-01 | 01 | 1 | CAPT-01 | unit | `go test -v ./internal/lifecycle -run TestStartCapture` | ✅ TDD | ⬜ pending |
| 20-01-02 | 01 | 1 | CAPT-02 | unit | `go test -v ./internal/lifecycle -run TestConcurrentRead` | ✅ TDD | ⬜ pending |
| 20-02-01 | 02 | 2 | CAPT-04 | integration | `go test -v ./internal/lifecycle -run TestStartNanobotWithCapture -timeout 15s` | ✅ TDD | ⬜ pending |
| 20-02-01 | 02 | 2 | CAPT-05 | integration | `go test -v ./internal/lifecycle -run TestStartNanobotWithCapture -timeout 15s` | ✅ TDD | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## TDD Strategy

This phase uses **TDD mode** — tests are written alongside implementation in each task, not as separate Wave 0 stubs.

**Benefits:**
- Faster iteration cycle — tests provide immediate feedback
- Tests document expected behavior before implementation
- No separate test scaffolding phase required

**Task flow (per `tdd="true"` task):**
1. Write failing test for expected behavior
2. Implement minimal code to pass
3. Refactor if needed
4. Commit when green

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Large output handling (>10MB) | CAPT-02 | Requires generating large test data that slows automated tests | Run nanobot with verbose logging, verify no deadlock and memory stable |
| Real-time capture performance | CAPT-03 | Timing-sensitive, not suitable for CI environment | Monitor LogBuffer write latency during nanobot operation |

*If none: "All phase behaviors have automated verification."*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or TDD mode
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] TDD mode eliminates need for Wave 0 stubs
- [x] No watch-mode flags
- [x] Feedback latency < 10s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

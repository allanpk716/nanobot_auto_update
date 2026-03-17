---
phase: 19
slug: log-buffer-core
status: ready
nyquist_compliant: true
wave_0_complete: false
created: 2026-03-17
---

# Phase 19 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — Wave 0 installs |
| **Quick run command** | `cd internal/logbuffer && go test -v -race -run TestLogBuffer` |
| **Full suite command** | `cd internal/logbuffer && go test -v -race -coverprofile=coverage.out ./...` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -v -race -run TestLogBuffer`
- **After every plan wave:** Run `go test -v -race -coverprofile=coverage.out ./...`
- **Before `/gsd:verify-work`:** Full suite must be green with >80% coverage
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 19-01-01 | 01 | 1 | BUFF-01, BUFF-02 | unit | `go test -v -race -run TestLogBuffer_Write` | ❌ W0 | ⬜ pending |
| 19-01-02 | 01 | 1 | BUFF-03 | unit | `go test -v -race -run TestLogBuffer_FIFO` | ❌ W0 | ⬜ pending |
| 19-01-03 | 01 | 1 | BUFF-02 | unit | `go test -v -race -run TestLogBuffer_Concurrent` | ❌ W0 | ⬜ pending |
| 19-02-01 | 02 | 1 | BUFF-05 | unit | `go test -v -race -run TestLogBuffer_Subscribe` | ❌ W0 | ⬜ pending |
| 19-02-02 | 02 | 1 | BUFF-05 | unit | `go test -v -race -run TestLogBuffer_History` | ❌ W0 | ⬜ pending |
| 19-02-03 | 02 | 1 | BUFF-05 | unit | `go test -v -race -run TestLogBuffer_Unsubscribe` | ❌ W0 | ⬜ pending |
| 19-02-04 | 02 | 1 | BUFF-05 | unit | `go test -v -race -run TestLogBuffer_SlowSubscriber` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/logbuffer/buffer_test.go` — test stubs for all BUFF requirements
- [ ] `internal/logbuffer/buffer.go` — empty file for test imports
- [ ] No additional dependencies — using Go standard library only

---

## Manual-Only Verifications

All phase behaviors have automated verification.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 5s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved

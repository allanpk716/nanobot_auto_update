---
phase: 23
slug: web-ui-and-error-handling
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-19
---

# Phase 23 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testutil package |
| **Config file** | None (test files use hardcoded configs) |
| **Quick run command** | `go test ./internal/web/... -v -run TestHandler` |
| **Full suite command** | `go test ./... -v -race` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/web ./internal/instance -v -race`
- **After every plan wave:** Run `go test ./... -v -race`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 23-01-01 | 01 | 1 | UI-06 | unit | `go test ./internal/web -v -run TestEmbedFS` | ❌ W0 | ⬜ pending |
| 23-01-02 | 01 | 1 | UI-01 | unit | `go test ./internal/web -v -run TestWebHandler` | ❌ W0 | ⬜ pending |
| 23-01-03 | 01 | 1 | UI-05 | unit | `go test ./internal/web -v -run TestConnectionStatus` | ❌ W0 | ⬜ pending |
| 23-02-01 | 02 | 2 | UI-07 | unit | `go test ./internal/instance -v -run TestGetInstanceNames` | ❌ W0 | ⬜ pending |
| 23-03-01 | 03 | 3 | ERR-02 | unit | `go test ./internal/api -v -run TestSSEHandlerError` | ❌ W0 | ⬜ pending |
| 23-03-02 | 03 | 3 | ERR-03 | unit | `go test ./internal/logbuffer -v -run TestWriteError` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/web/handler_test.go` — stubs for UI-01, UI-05, UI-06
- [ ] `internal/web/static/index.html` — main HTML page stub
- [ ] `internal/web/static/style.css` — stylesheet stub
- [ ] `internal/web/static/app.js` — JavaScript stub
- [ ] `internal/instance/manager_test.go` — add TestGetInstanceNames stub (UI-07)
- [ ] `internal/api/sse_test.go` — add TestSSEHandlerError stub (ERR-02)
- [ ] `internal/logbuffer/buffer_test.go` — add TestWriteError stub (ERR-03)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Auto-scroll to latest log | UI-02 | Requires browser DOM interaction | Open `/logs/:instance` in Chrome, verify page auto-scrolls as new logs arrive |
| Pause/resume scroll button | UI-03 | Requires browser DOM interaction | Click pause button, verify auto-scroll stops; click resume, verify auto-scroll resumes |
| stdout/stderr color distinction | UI-04 | Visual inspection required | Verify stdout lines appear in green, stderr lines appear in red with sufficient contrast |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

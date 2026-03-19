---
phase: 22
slug: sse-streaming-api
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-03-17
---

# Phase 22 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (go test) |
| **Config file** | None (Go test auto-discovers *_test.go files) |
| **Quick run command** | `go test -v ./internal/api -run <TestName>` |
| **Full suite command** | `go test -v -race ./internal/api` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -v ./internal/api -run <TestName>`
- **After every plan wave:** Run `go test -v -race ./internal/api`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 22-01-01 | 01 | 1 | SSE-01,02,03,04,05,06 | unit | `grep -n "func.*Handle" internal/api/sse.go` | ✅ | ⬜ pending |
| 22-01-02 | 01 | 1 | SSE-01,02,06,ERR-04 | unit | `go test -v ./internal/api -run TestSSE` | ✅ | ⬜ pending |
| 22-02-01 | 02 | 2 | SSE-07 | unit | `go test -v ./internal/api -run TestNewServer` | ✅ | ⬜ pending |
| 22-02-02 | 02 | 2 | SSE-07 | integration | `go build ./cmd/nanobot-auto-updater` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

**Note:** This phase uses TDD approach where test files are created in the same task as implementation (Task 2 in each plan). This is a valid alternative to separate Wave 0 infrastructure setup.

| File | Purpose | Created In | Status |
|------|---------|------------|--------|
| internal/api/sse_test.go | SSE handler unit tests | Plan 01 Task 2 | ✅ created with TDD |
| internal/api/server_test.go | HTTP server integration tests | Plan 02 Task 1 | ✅ created with TDD |

**Wave 0 complete:** Tests are co-created with implementation using `tdd="true"` attribute on tasks.

---

## Validation Approach

**Type:** TDD (Test-Driven Development)

**Strategy:**
1. Plan 01 Task 1: Implement SSE handler core logic
2. Plan 01 Task 2: Create sse_test.go with test cases
3. Plan 02 Task 1: Implement HTTP server + create server_test.go
4. Plan 02 Task 2: Integrate server into main program

**Feedback Loop:**
- After each task: grep commands verify code structure (< 1 second)
- After Task 2 in each plan: go test verifies functionality (< 5 seconds)
- Race detection enabled to catch concurrency issues early

---

## Coverage Requirements

- SSE-01: `grep "GET /api/v1/logs/{instance}/stream" internal/api/server.go`
- SSE-02: `grep "Content-Type.*text/event-stream" internal/api/sse.go`
- SSE-03: `grep "time.NewTicker.*30.*Second" internal/api/sse.go`
- SSE-04: `grep "Unsubscribe(" internal/api/sse.go`
- SSE-05: `grep "Subscribe()" internal/api/sse.go`
- SSE-06: `grep "event: stderr" internal/api/sse.go`
- SSE-07: `grep "WriteTimeout.*0" internal/api/server.go`
- ERR-04: `go test -v ./internal/api -run TestSSEInstanceNotFound`

---

## Quality Gates

- [ ] All grep commands return exit code 0
- [ ] All go test commands pass with race detection
- [ ] No goroutine leaks (runtime.NumGoroutine() check in tests)
- [ ] Test coverage > 70% (optional: `go test -cover ./internal/api`)
- [ ] Program builds successfully (`go build ./cmd/nanobot-auto-updater`)

---

## Notes

- Tests use Go standard library `testing` package
- httptest.NewRecorder used for HTTP handler testing
- context.WithCancel used to simulate client disconnect
- Race detection enabled for all test runs (-race flag)
- Test files created in same wave as implementation (not separate Wave 0)

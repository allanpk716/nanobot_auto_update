---
phase: 31
slug: file-persistence
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-28
---

# Phase 31 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | testing (stdlib) + testify |
| **Config file** | None (Go convention) |
| **Quick run command** | `go test ./internal/updatelog/ -v -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/updatelog/ -v -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 31-01-01 | 01 | 1 | STORE-01 | unit | `go test ./internal/updatelog/ -run TestWriteToFile -v` | ❌ W0 | ⬜ pending |
| 31-01-02 | 01 | 1 | STORE-01 | unit | `go test ./internal/updatelog/ -run TestConcurrentFileWrite -v` | ❌ W0 | ⬜ pending |
| 31-01-03 | 01 | 1 | STORE-01 | unit | `go test ./internal/updatelog/ -run TestAutoCreateFile -v` | ❌ W0 | ⬜ pending |
| 31-02-01 | 02 | 1 | STORE-02 | unit | `go test ./internal/updatelog/ -run TestCleanupOldLogs -v` | ❌ W0 | ⬜ pending |
| 31-02-02 | 02 | 1 | STORE-02 | unit | `go test ./internal/updatelog/ -run TestCleanupNoBlock -v` | ❌ W0 | ⬜ pending |
| 31-03-01 | 03 | 2 | STORE-01 | integration | `go test ./internal/updatelog/ -run TestClose -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/updatelog/logger_test.go` — 添加 TestWriteToFile, TestConcurrentFileWrite, TestAutoCreateFile
- [ ] `internal/updatelog/logger_test.go` — 添加 TestCleanupOldLogs, TestCleanupNoBlock, TestClose
- [ ] `go get github.com/robfig/cron/v3@v3.0.1` — cron 依赖安装

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| None | - | - | All phase behaviors have automated verification |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

---
phase: 40
slug: safety-recovery
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-30
---

# Phase 40 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — existing infrastructure |
| **Quick run command** | `go test ./internal/api/... ./internal/lifecycle/... ./internal/notifier/... -count=1 -timeout 30s` |
| **Full suite command** | `go test ./... -count=1 -timeout 60s` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/api/... ./internal/lifecycle/... -count=1 -timeout 30s`
- **After every plan wave:** Run `go test ./... -count=1 -timeout 60s`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 40-01-01 | 01 | 1 | SAFE-02 | unit | `go test ./internal/api/... -run TestSelfUpdate -count=1` | ✅ | ⬜ pending |
| 40-01-02 | 01 | 1 | SAFE-01 | unit | `go test ./internal/api/... -run TestRestart -count=1` | ✅ | ⬜ pending |
| 40-02-01 | 02 | 2 | SAFE-03 | unit | `go test ./internal/lifecycle/... -run TestCleanup -count=1` | ✅ | ⬜ pending |
| 40-02-02 | 02 | 2 | SAFE-04 | unit | `go test ./internal/lifecycle/... -run TestRecovery -count=1` | ✅ | ⬜ pending |
| 40-02-03 | 02 | 2 | SAFE-01 | unit | `go test ./internal/api/... -run TestPortRetry -count=1` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

*Existing infrastructure covers all phase requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Self-spawn restart binds original port | SAFE-01 | Requires process replacement + port binding verification | 1. Build binary 2. Start with API port 3. Trigger self-update 4. Verify new process binds same port |
| .old recovery restores old version | SAFE-04 | Requires simulating crash during update | 1. Place .exe.old file 2. Remove .update-success 3. Start program 4. Verify old exe restored |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

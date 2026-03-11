---
phase: 8
slug: instance-coordinator
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-11
---

# Phase 8 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure
Test infrastructure required for Phase 8 validation.
- Unit tests in `internal/instance/manager_test.go`
- Integration tests using test configuration files
- Mock instances for isolated testing

## Sampling Rate
- **After every task commit:** Run `go test ./internal/instance -v -run ^InstanceManager`
- **After every plan wave:** Run full test suite `go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 08-01-01 | 01 | 1 | LIFECYCLE-01, LIFECYCLE-02 | unit | `go test -v -run ^TestInstanceManager_StopAll` | ✅ | pending |
| 08-01-02 | 01 | 1 | LIFECYCLE-03 | unit | `go test -v -run ^TestInstanceManager_StartAll` | ✅ | pending |
| 08-01-03 | 01 | 1 | ERROR-02 | unit | `go test -v -run ^TestUpdateResult_Structure` | ✅ | pending |
| 08-01-04 | 01 | 1 | LIFECYCLE-01, LIFECYCLE-02, LIFECYCLE-03 | integration | `go test -v -run ^TestInstanceManager_UpdateAll` | ✅ | pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements
- **Stop all instances sequentially**: Verify that InstanceManager stops instances one by one
- **Execute UV update once**: Verify single global update after all stops
- **Start all instances sequentially**: Verify that InstanceManager starts instances one by one
- **Graceful degradation on stop failure**: Verify other instances continue stopping when one fails
- **Graceful degradation on start failure**: Verify other instances continue starting when one fails
- **Error aggregation**: Verify all errors collected in UpdateResult
- **Skip UV update on stop failure**: Verify UV update skipped when any stop fails

**Validation approach:**
- Mock InstanceLifecycle for unit tests
- Use test configuration with 3+ instances
- Inject mock updater to control UV update success/failure
- Verify error messages contain instance names and operation details

---

## Validation Sign-Off
- [x] All tasks have `<automated>` verify
- [x] Sampling continuity: All consecutive tasks have automated verify
- [x] Wave 0 covers all LIFECYCLE-01, LIFECYCLE-02, LIFECYCLE-03, ERROR-02 requirements
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending / approved 2026-03-11

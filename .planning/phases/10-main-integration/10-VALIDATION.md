---
phase: 10
slug: main-integration
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-11
---

# Phase 10 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (标准库) |
| **Config file** | 无 - 使用 go test 自动发现 |
| **Quick run command** | `go test -v ./cmd/nanobot-auto-updater -run TestMultiInstance` |
| **Full suite command** | `go test -v ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -v ./cmd/nanobot-auto-updater -run TestMultiInstance`
- **After every plan wave:** Run `go test -v ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 10-01-01 | 01 | 1 | v0.2 定时任务触发 | integration | `go test -v ./cmd/nanobot-auto-updater -run TestScheduledMultiInstanceUpdate` | ❌ W0 | ⬜ pending |
| 10-01-02 | 01 | 1 | v0.2 -run-once 模式 | integration | `go test -v ./cmd/nanobot-auto-updater -run TestUpdateNowMultiInstance` | ❌ W0 | ⬜ pending |
| 10-01-03 | 01 | 1 | v0.2 日志追踪 | unit | `go test -v ./internal/instance -run TestInstanceLifecycleLogging` | ✅ Phase 7 | ⬜ pending |
| 10-01-04 | 01 | 1 | v0.2 资源管理 | manual | 运行 `make build && ./nanobot-auto-updater.exe` 48小时,监控内存 | ❌ W0 | ⬜ pending |
| 10-01-05 | 01 | 1 | v0.2 长期稳定性 | integration | `go test -v ./cmd/nanobot-auto-updater -run TestMultiInstanceLongRunning -timeout 2h` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `cmd/nanobot-auto-updater/main_test.go` — 添加 TestScheduledMultiInstanceUpdate (集成测试,需要 mock scheduler)
- [ ] `cmd/nanobot-auto-updater/main_test.go` — 添加 TestUpdateNowMultiInstance (集成测试,需要 mock InstanceManager)
- [ ] `cmd/nanobot-auto-updater/main_test.go` — 添加 TestMultiInstanceLongRunning (长期运行测试,模拟多次更新周期)
- [ ] `docs\test-plan.md` — 手动测试计划:48小时运行 + 内存监控

*Go 标准库 testing 已内置,无需额外安装测试框架*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| 长期运行资源管理 | v0.2 资源管理 | 需要 24-48 小时持续运行才能检测内存泄漏 | 1. `make build` 构建程序<br>2. 启动程序并配置 5 分钟定时周期<br>3. 使用任务管理器监控内存使用趋势<br>4. 运行 24-48 小时,观察内存是否持续增长 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

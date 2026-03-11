---
phase: 10
slug: main-integration
status: draft
nyquist_compliant: true
wave_0_complete: true
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
| 10-01-00 | 01 | 0 | Wave 0 测试骨架 | unit | `go test -v ./cmd/nanobot-auto-updater -run "Test(Scheduled\|UpdateNow\|LongRunning)"` | ✅ Phase 10 | ⬜ pending |
| 10-01-01 | 01 | 1 | v0.2 定时任务触发 | integration | `go test -v ./cmd/nanobot-auto-updater -run TestScheduledMultiInstanceUpdate` | ✅ Phase 10 | ⬜ pending |
| 10-01-02 | 01 | 1 | v0.2 -run-once 模式 | integration | `go test -v ./cmd/nanobot-auto-updater -run TestUpdateNowMultiInstance` | ✅ Phase 10 | ⬜ pending |
| 10-01-03 | 01 | 1 | v0.2 日志追踪 | unit | `go test -v ./internal/instance -run TestInstanceLifecycleLogging` | ✅ Phase 7 | ⬜ pending |
| 10-01-04 | 01 | 1 | v0.2 资源管理 | manual | 运行 `make build && ./nanobot-auto-updater.exe` 48小时,监控内存 | ❌ W0 | ⬜ pending |
| 10-01-05 | 01 | 1 | v0.2 长期稳定性 | integration | `go test -v ./cmd/nanobot-auto-updater -run TestMultiInstanceLongRunning -timeout 2h` | ✅ Phase 10 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `cmd/nanobot-auto-updater/main_test.go` — TestScheduledMultiInstanceUpdate (Wave 0 骨架,Task 0 创建)
- [x] `cmd/nanobot-auto-updater/main_test.go` — TestUpdateNowMultiInstance (Wave 0 骨架,Task 0 创建)
- [x] `cmd/nanobot-auto-updater/main_test.go` — TestMultiInstanceLongRunning (Wave 0 骨架,Task 0 创建)
- [x] `tmp/test_multi_instance.yaml` — 测试配置文件 (Task 0 创建)
- [x] `tmp/test_legacy.yaml` — Legacy 配置文件 (Task 0 创建)
- [x] `docs/test-plan.md` — 手动测试计划 (Task 3 创建,包含 48 小时运行步骤)

**Nyquist Compliance Note:**
Task 0 在 Wave 0 创建测试骨架 (t.Skip() 占位),Task 2 在 Wave 1 实现测试逻辑。这确保了 Nyquist 采样原则:测试在实现之前预先存在 (即使是骨架形式)。

*Go 标准库 testing 已内置,无需额外安装测试框架*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| 长期运行资源管理 | v0.2 资源管理 | 需要 24-48 小时持续运行才能检测内存泄漏 | 1. `make build` 构建程序<br>2. 启动程序并配置 5 分钟定时周期<br>3. 使用任务管理器监控内存使用趋势<br>4. 运行 24-48 小时,观察内存是否持续增长 |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

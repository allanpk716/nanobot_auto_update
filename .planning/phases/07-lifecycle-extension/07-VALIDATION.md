# Phase 7: 生命周期扩展 - Validation Strategy

---
phase: "07"
phase_slug: "lifecycle-extension"
created: "2026-03-10"
validation_framework: "Go testing (标准库)"
---

## Test Framework Configuration

| Property | Value |
|----------|-------|
| Framework | Go testing (标准库) |
| Config file | none — 使用 \*_test.go 文件 |
| Quick run command | `go test ./internal/instance -v -run TestInstanceLifecycle` |
| Full suite command | `go test ./internal/instance -v -cover` |

## Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| LIFECYCLE-01 (部分) | Stop specific instance by name | unit | `go test ./internal/instance -v -run TestStopForUpdate` | ❌ Wave 0 |
| LIFECYCLE-02 (部分) | Start specific instance by name with custom command | unit | `go test ./internal/instance -v -run TestStartAfterUpdate` | ❌ Wave 0 |
| Success-01 | All logs contain instance name | unit | `go test ./internal/instance -v -run TestLoggerContextInjection` | ❌ Wave 0 |
| Success-02/03 | Can stop/start specific instance | integration | `go test ./internal/instance -v -run TestInstanceLifecycle` | ❌ Wave 0 |
| Success-04 | Reuse existing lifecycle logic | integration | 手动验证 — 检查代码调用 lifecycle.IsNanobotRunning/StopNanobot/StartNanobot | N/A |

## Sampling Rate

- **Per task commit:** `go test ./internal/instance -v -run <specific-test>`
- **Per wave merge:** `go test ./internal/instance -v -cover`
- **Phase gate:** Full suite green before `/gsd:verify-work`

## Wave 0 Gaps

- [ ] `internal/instance/lifecycle_test.go` — 单元测试覆盖 StopForUpdate/StartAfterUpdate 方法
- [ ] `internal/instance/errors_test.go` — 单元测试覆盖 InstanceError 的 Error()/Unwrap() 方法
- [ ] `internal/instance/logger_test.go` — 单元测试验证日志包含 instance 和 component 字段
- [ ] 测试辅助函数 — 创建 mock lifecycle 函数用于单元测试(避免依赖真实进程)

**Note:** Wave 0 需要创建完整的测试套件

## Success Criteria

Phase 7 验证通过条件：
1. ✅ 所有 Wave 0 测试文件创建完成
2. ✅ `go test ./internal/instance -v -cover` 全部通过
3. ✅ 代码覆盖率 ≥ 80%
4. ✅ 所有 LIFECYCLE-01/02 (部分) 需求对应测试通过
5. ✅ 手动验证：检查代码调用 lifecycle.IsNanobotRunning/StopNanobot/StartNanobot

---

*Validation strategy created: 2026-03-10*

---
phase: 29
slug: http-help-endpoint
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-23
---

# Phase 29 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify/assert |
| **Config file** | 无 — 使用 `testing` 包 |
| **Quick run command** | `go test ./internal/api -run TestHelp -v` |
| **Full suite command** | `go test ./internal/api -v` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/api -run TestHelp -v`
- **After every plan wave:** Run `go test ./internal/api -v`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 29-01-01 | 01 | 1 | HELP-01 | unit | `go test ./internal/api -run TestHelpHandler_Success -v` | ❌ W0 | ⬜ pending |
| 29-01-02 | 01 | 1 | HELP-02 | unit | `go test ./internal/api -run TestHelpHandler_Success -v` | ❌ W0 | ⬜ pending |
| 29-01-03 | 01 | 1 | HELP-03 | unit | `go test ./internal/api -run TestHelpHandler_Success -v` | ❌ W0 | ⬜ pending |
| 29-01-04 | 01 | 1 | HELP-04 | unit | `go test ./internal/api -run TestHelpHandler_ContentAccuracy -v` | ❌ W0 | ⬜ pending |
| 29-02-01 | 02 | 1 | HELP-01 | integration | `go test ./internal/api -run TestHelpHandler_Success -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/api/help_test.go` — 测试覆盖 HELP-01, HELP-02, HELP-03, HELP-04
- [ ] `internal/api/help.go` — HelpHandler 实现和响应结构
- [ ] `internal/api/server.go` — GET /api/v1/help 路由注册

*现有测试基础设施已覆盖所有需求,仅需创建新的测试文件。*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| JSON 响应在浏览器中的可读性 | HELP-03 | 主观体验,不影响功能 | 手动在浏览器中访问 http://localhost:8080/api/v1/help 检查格式化 |
| 第三方程序解析 JSON 的易用性 | HELP-03 | 需要真实第三方集成测试 | 使用 curl 或其他工具测试 JSON 解析 |

*All core behaviors have automated verification. Manual tests are for UX validation only.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

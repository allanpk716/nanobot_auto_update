---
phase: 26
slug: network-monitoring-core
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-21
---

# Phase 26 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (标准库) |
| **Config file** | 无 — 使用现有 Go 测试配置 |
| **Quick run command** | `go test -v ./internal/network/... -run TestNetwork` |
| **Full suite command** | `go test -v ./...` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -v ./internal/network/... -run TestNetwork`
- **After every plan wave:** Run `go test -v ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 26-01-01 | 01 | 1 | MONITOR-01,02,03 | unit | `go test -v -run TestCheckConnectivity` | ❌ W0 | ⬜ pending |
| 26-01-02 | 01 | 1 | MONITOR-01 | integration | `go test -v -run TestNetworkMonitorStart` | ❌ W0 | ⬜ pending |
| 26-02-01 | 02 | 1 | MONITOR-06 | integration | `go test -v -run TestMainIntegration` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/network/monitor_test.go` — NetworkMonitor 单元测试和集成测试
- [ ] 测试用例覆盖:
  - HTTP 200 OK 成功场景
  - HTTP 非 200 状态码失败场景
  - DNS 解析失败场景
  - 连接超时场景
  - TLS 握手错误场景
  - 禁用重定向跟随验证
  - 状态追踪和变化检测
  - 优雅关闭测试

*Wave 0 将创建基础测试文件结构,确保测试基础设施就绪*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| 日志格式验证 | MONITOR-02,03 | 日志格式需要人工确认可读性 | 启动应用,触发连通性检查,检查日志格式是否符合预期 |
| 配置文件调整生效 | MONITOR-06 | 需要重启应用验证配置变更 | 修改 config.yaml 中 monitor.interval 和 timeout,重启应用,验证新配置生效 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending

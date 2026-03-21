---
phase: 26-network-monitoring-core
plan: 02
subsystem: network-monitoring
tags: [lifecycle, integration, configuration]
requires: [26-01]
provides: [MONITOR-01, MONITOR-06]
affects: [cmd/nanobot-auto-updater/main.go]
tech_stack:
  added: []
  patterns: [lifecycle-management, graceful-shutdown, startup-order]
key_files:
  created: []
  modified:
    - cmd/nanobot-auto-updater/main.go
decisions:
  - 网络监控始终启动（不检查实例数量，因为监控 Google 不依赖实例）
  - 启动顺序：健康监控启动 → 网络监控启动
  - 关闭顺序：网络监控停止 → 健康监控停止 → API 服务器停止
metrics:
  duration: 84s
  completed_date: 2026-03-21
  tasks: 1
  commits: 1
  files_modified: 1
---

# Phase 26 Plan 02: NetworkMonitor 生命周期集成 Summary

## One-Liner

集成 NetworkMonitor 到 main.go 应用程序生命周期，实现网络监控在健康监控之后启动、在关闭时优先停止，满足 MONITOR-01 和 MONITOR-06 需求。

## What Changed

### Modified Files

- **cmd/nanobot-auto-updater/main.go**：添加 NetworkMonitor 生命周期集成
  - 导入 `internal/network` 包
  - 创建并启动网络监控器（健康监控之后）
  - 优雅关闭网络监控器（健康监控之前）

## Implementation Details

### 生命周期集成

**启动顺序**（按执行顺序）：
1. 配置加载和验证
2. InstanceManager 创建
3. API 服务器启动（goroutine）
4. 健康监控启动（goroutine，如果存在实例）
5. 网络监控启动（goroutine）← **本次添加**
6. 实例自动启动（goroutine）

**关闭顺序**（按执行顺序）：
1. 网络监控停止 ← **本次添加**
2. 健康监控停止
3. API 服务器停止（10 秒超时）

### 配置使用

- **监控间隔**：`cfg.Monitor.Interval`（默认 15 分钟）
- **请求超时**：`cfg.Monitor.Timeout`（默认 10 秒）
- **监控目标**：`https://www.google.com`（硬编码）

### 中文日志

添加启动日志：
```go
logger.Info("网络监控已启动", "interval", cfg.Monitor.Interval)
```

停止日志由 `NetworkMonitor.Stop()` 内部记录。

## Acceptance Criteria Met

- [x] main.go 导入 `internal/network` 包
- [x] main.go 包含 `network.NewNetworkMonitor("https://www.google.com", cfg.Monitor.Interval, cfg.Monitor.Timeout, logger)` 调用
- [x] main.go 包含 `go networkMonitor.Start()` 调用
- [x] main.go 包含 `networkMonitor.Stop()` 调用
- [x] 网络监控在健康监控启动代码之后启动
- [x] 网络监控在健康监控停止之前停止
- [x] 存在中文日志 "网络监控已启动"
- [x] `go build ./cmd/nanobot-auto-updater` 编译成功

## Success Criteria Met

- [x] MONITOR-01: 网络监控在应用启动后自动开始运行
- [x] MONITOR-06: 监控间隔和超时通过 config.yaml 的 monitor.interval 和 monitor.timeout 配置
- [x] 启动顺序: 健康监控启动 → 网络监控启动
- [x] 关闭顺序: 网络监控停止 → 健康监控停止 → API 服务器停止
- [x] 中文日志: "网络监控已启动"

## Test Results

### 编译验证

```bash
$ go build ./cmd/nanobot-auto-updater
BUILD SUCCESS
```

### 单元测试

```bash
$ go test -v ./internal/network/...
PASS
ok  	github.com/HQGroup/nanobot-auto-updater/internal/network
```

所有 9 个测试通过。

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

None.

## Self-Check: PASSED

- [x] Created/modified files exist:
  - FOUND: cmd/nanobot-auto-updater/main.go
- [x] Commits exist:
  - FOUND: 83c6f56
- [x] SUMMARY.md exists:
  - FOUND: 26-02-SUMMARY.md

## Next Steps

Phase 27 将使用 `networkMonitor.GetState()` 实现：
- 连通性状态变化通知
- 连通性恢复检测通知

---

**Commit:** 83c6f56
**Duration:** 84 seconds
**Completed:** 2026-03-21

---
phase: 11-configuration-extension
plan: 02
subsystem: config
tags: [tdd, validation, security]
dependencies:
  requires: [11-01b]
  provides: [APIConfig, MonitorConfig]
  affects: [config.go]
tech_stack:
  added: []
  patterns: [TDD RED-GREEN, Validate() pattern]
key_files:
  created:
    - internal/config/api.go
    - internal/config/monitor.go
  modified:
    - internal/config/api_test.go
    - internal/config/monitor_test.go
decisions:
  - APIConfig validates port range 1-65535
  - Bearer Token minimum 32 chars per SEC-03
  - API timeout minimum 5 seconds
  - Monitor interval minimum 1 minute
  - Monitor timeout minimum 1 second
metrics:
  duration: 5min
  tasks: 2
  files: 4
  tests_added: 43
---

# Phase 11 Plan 02: APIConfig and MonitorConfig Implementation Summary

## One-liner

实现了 APIConfig 和 MonitorConfig 结构体及其验证逻辑，使用 TDD RED-GREEN 循环，共 43 个测试全部通过。

## What Was Done

### Task 1: APIConfig Implementation

创建了 `internal/config/api.go` 包含:
- `APIConfig` 结构体 (Port, BearerToken, Timeout 字段)
- `Validate()` 方法实现:
  - 端口范围验证: 1-65535
  - Bearer Token 长度验证: >= 32 字符 (SEC-03 安全要求)
  - 超时验证: >= 5 秒

更新了 `internal/config/api_test.go`:
- 移除所有 `t.Skip()` 占位符
- 实现完整测试逻辑 (22 个测试)

### Task 2: MonitorConfig Implementation

创建了 `internal/config/monitor.go` 包含:
- `MonitorConfig` 结构体 (Interval, Timeout 字段)
- `Validate()` 方法实现:
  - 间隔验证: >= 1 分钟
  - 超时验证: >= 1 秒

更新了 `internal/config/monitor_test.go`:
- 移除所有 `t.Skip()` 占位符
- 实现完整测试逻辑 (21 个测试)
- 添加 Duration 字符串解析测试

## Deviations from Plan

None - plan executed exactly as written.

## Test Results

```
=== APIConfig Tests (22 tests) ===
TestAPIConfigValidate: 6 subtests - PASS
TestAPIConfigPortValidation: 5 subtests - PASS
TestAPIConfigBearerTokenValidation: 5 subtests - PASS
TestAPIConfigTimeoutValidation: 6 subtests - PASS

=== MonitorConfig Tests (21 tests) ===
TestMonitorConfigValidate: 3 subtests - PASS
TestMonitorConfigIntervalValidation: 8 subtests - PASS
TestMonitorConfigTimeoutValidation: 8 subtests - PASS
TestMonitorConfigDurationParsing: 2 subtests - PASS

Total: 43 tests - ALL PASS
```

## Key Decisions

1. **错误消息格式**: 使用 `api.port must be between 1 and 65535, got %d` 格式，包含字段路径和实际值
2. **SEC-03 合规**: Bearer Token 长度验证在启动时执行，错误消息明确显示字符数
3. **Duration 验证**: 使用 `time.Duration` 比较，支持 YAML 字符串解析 ("15m", "10s")

## Files Modified

| File | Lines | Purpose |
|------|-------|---------|
| internal/config/api.go | 28 | APIConfig struct + Validate() |
| internal/config/api_test.go | 217 | APIConfig test suite |
| internal/config/monitor.go | 24 | MonitorConfig struct + Validate() |
| internal/config/monitor_test.go | 187 | MonitorConfig test suite |

## Next Steps

Plan 11-03 将集成这些结构体到主 Config 结构体，并实现:
- Config 结构体添加 API 和 Monitor 字段
- defaults() 设置默认值
- Validate() 调用子结构体验证
- Load() 函数设置 Viper 默认值

## Commits

- `dc01ff7`: feat(11-02): implement APIConfig validation
- `5c1e27c`: feat(11-02): implement MonitorConfig validation

## Self-Check: PASSED

- [x] internal/config/api.go exists
- [x] internal/config/monitor.go exists
- [x] Commit dc01ff7 verified
- [x] Commit 5c1e27c verified

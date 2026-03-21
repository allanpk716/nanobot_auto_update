---
phase: 26-network-monitoring-core
plan: 01
subsystem: network-monitoring
tags: [monitoring, network, http, tdd]
dependencies:
  requires: []
  provides: [NetworkMonitor, ConnectivityState]
  affects: []
tech-stack:
  added:
    - net/http (Go stdlib)
    - time.Ticker (Go stdlib)
    - context.Context (Go stdlib)
    - net (Go stdlib for error types)
    - crypto/tls (Go stdlib for error types)
  patterns:
    - HTTP HEAD request with redirect disabled
    - Ticker + Context graceful shutdown
    - State tracking with change detection
    - Error classification via type assertion
key-files:
  created:
    - internal/network/monitor.go
    - internal/network/monitor_test.go
  modified: []
decisions:
  - Use HEAD method instead of GET for efficiency
  - Strict HTTP 200 OK only for success criteria
  - Disable redirect following with CheckRedirect
  - Immediate first check on Start() then periodic with Ticker
  - Classify errors by type assertion (DNS, timeout, TLS, connection refused)
  - Track state changes for Phase 27 notification support
metrics:
  duration: 3m
  tasks: 1
  files: 2
  test-coverage: 10 test cases
  completed: 2026-03-21T08:37:25+08:00
---

# Phase 26 Plan 01: NetworkMonitor Core Implementation Summary

## One-Liner

实现了 NetworkMonitor 核心功能，定期向 google.com 发送 HTTP HEAD 请求测试连通性，记录成功/失败日志并追踪状态变化，为 Phase 27 状态变化通知做准备。

## Objective

创建 NetworkMonitor 核心实现，定期向 https://www.google.com 发送 HTTP HEAD 请求测试网络连通性，记录成功/失败日志并追踪状态变化。

**目的:** 实现网络连通性监控核心逻辑，满足 MONITOR-01, MONITOR-02, MONITOR-03 需求

**输出:** internal/network/monitor.go, internal/network/monitor_test.go

## Completed Tasks

### Task 1: 创建 NetworkMonitor 核心实现和测试

**Status:** ✅ 完成

**TDD 流程:**
1. **RED 阶段:** 创建 10 个测试用例，验证测试失败
2. **GREEN 阶段:** 实现 NetworkMonitor 核心功能，所有测试通过
3. **REFACTOR 阶段:** 代码质量良好，无需重构

**实现内容:**

#### 文件 1: internal/network/monitor.go

**核心结构体:**
- `ConnectivityState` - 追踪连通性状态（IsConnected, LastCheck）
- `NetworkMonitor` - 网络监控器（targetURL, interval, timeout, logger, httpClient, state, ctx, cancel）

**核心方法:**
- `NewNetworkMonitor()` - 创建监控器，配置禁用重定向的 HTTP 客户端
- `Start()` - 启动监控循环（立即执行首次检查 + 定期检查）
- `checkConnectivity()` - 检查连通性并记录日志（INFO 成功 / ERROR 失败）
- `performCheck()` - 执行 HTTP HEAD 请求，返回 (是否连通, 状态码, 错误消息)
- `classifyError()` - 分类错误类型（DNS 解析失败、连接超时、TLS 错误、连接拒绝）
- `Stop()` - 优雅停止监控循环
- `GetState()` - 获取当前连通性状态（供 Phase 27 使用）

**关键技术点:**
- HTTP 客户端禁用重定向跟随：`CheckRedirect: func(...) { return http.ErrUseLastResponse }`
- 使用 `time.Ticker` 定期检查，立即执行首次检查
- 使用 `context.Context` 实现优雅关闭
- 错误分类使用类型断言（net.DNSError, net.OpError, tls.CertificateVerificationError）
- 状态追踪：首次检查、状态保持、状态改变检测

#### 文件 2: internal/network/monitor_test.go

**测试覆盖:**
1. `TestNewNetworkMonitor` - 验证构造函数创建正确的 NetworkMonitor
2. `TestCheckConnectivity_Success` - HTTP 200 返回 true，记录 INFO 日志
3. `TestCheckConnectivity_Failure_Non200` - HTTP 非 200 返回 false，记录 ERROR 日志
4. `TestCheckConnectivity_Failure_Timeout` - 请求超时返回 false，错误类型为"连接超时"
5. `TestCheckConnectivity_Failure_DNS` - DNS 解析失败返回 false，错误类型包含"DNS"
6. `TestClassifyError` - 验证错误类型分类正确
7. `TestStateTracking` - 验证状态追踪和变化检测
8. `TestGracefulStop` - 验证 Stop() 能正确停止监控循环
9. `TestDisableRedirect` - 验证禁用重定向跟随，301/302 不跟随
10. `TestGetState` - 验证 GetState 返回当前状态
11. `TestStartImmediateCheck` - 验证 Start 立即执行首次检查

**测试结果:**
```
=== RUN   TestNewNetworkMonitor
--- PASS: TestNewNetworkMonitor (0.00s)
=== RUN   TestCheckConnectivity_Success
--- PASS: TestCheckConnectivity_Success (0.00s)
=== RUN   TestCheckConnectivity_Failure_Non200
--- PASS: TestCheckConnectivity_Failure_Non200 (0.00s)
=== RUN   TestCheckConnectivity_Failure_Timeout
--- PASS: TestCheckConnectivity_Failure_Timeout (2.00s)
=== RUN   TestCheckConnectivity_Failure_DNS
--- PASS: TestCheckConnectivity_Failure_DNS (0.01s)
=== RUN   TestClassifyError
--- PASS: TestClassifyError (0.00s)
=== RUN   TestStateTracking
--- PASS: TestStateTracking (0.01s)
=== RUN   TestGracefulStop
--- PASS: TestGracefulStop (0.30s)
=== RUN   TestDisableRedirect
--- PASS: TestDisableRedirect (0.00s)
=== RUN   TestGetState
--- PASS: TestGetState (0.00s)
=== RUN   TestStartImmediateCheck
--- PASS: TestStartImmediateCheck (0.00s)
PASS
ok  	github.com/HQGroup/nanobot-auto-updater/internal/network	3.122s
```

**验证命令:**
```bash
go test -v ./internal/network/...
```

## Requirements Satisfied

- ✅ **MONITOR-01:** 定期向 https://www.google.com 发送 HTTP HEAD 请求
  - 实现：`Start()` 方法使用 `time.Ticker` 定期调用 `checkConnectivity()`
  - 验证：TestStartImmediateCheck, TestGracefulStop

- ✅ **MONITOR-02:** HTTP 非 200 或网络错误时记录 ERROR 日志，包含错误类型分类
  - 实现：`checkConnectivity()` 方法在失败时记录 ERROR 日志，包含 `error_type` 字段
  - 错误分类：`classifyError()` 方法分类 DNS、超时、TLS、连接拒绝等错误
  - 验证：TestCheckConnectivity_Failure_Non200, TestCheckConnectivity_Failure_Timeout, TestCheckConnectivity_Failure_DNS

- ✅ **MONITOR-03:** HTTP 200 OK 时记录 INFO 日志，包含响应时间
  - 实现：`checkConnectivity()` 方法在成功时记录 INFO 日志，包含 `duration` 和 `status_code` 字段
  - 验证：TestCheckConnectivity_Success

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] 修复 DNSError 字段名错误**

- **发现于:** GREEN 阶段编译错误
- **问题:** `net.DNSError` 类型使用 `Host` 字段，实际应为 `Name` 字段
- **修复:** 将 `netErr.Host` 改为 `netErr.Name`
- **修改文件:** internal/network/monitor.go (line 165)
- **提交:** 40ac4af

**2. [Rule 1 - Bug] 修复 TLS 错误类型断言**

- **发现于:** GREEN 阶段编译错误
- **问题:** `tls.CertificateVerificationError` 类型断言缺少指针 `*`
- **修复:** 将 `innerErr.(tls.CertificateVerificationError)` 改为 `innerErr.(*tls.CertificateVerificationError)`
- **修改文件:** internal/network/monitor.go (line 178, 181)
- **提交:** 40ac4af

## Technical Decisions

### HTTP 方法选择

**决策:** 使用 HEAD 方法而非 GET 方法

**理由:**
- HEAD 方法只请求响应头，不下载响应体
- 减少网络流量和响应时间
- 适合连通性测试场景

**结果:** ✅ 良好 - 测试通过，性能更优

### 成功标准

**决策:** 仅 HTTP 200 OK 算成功，其他状态码一律视为失败

**理由:**
- 严格标准，避免歧义
- 适合基础连通性测试
- 简单明确，易于判断

**结果:** ✅ 良好 - 测试通过，逻辑清晰

### 重定向处理

**决策:** 禁用重定向跟随，使用 `CheckRedirect` 返回 `http.ErrUseLastResponse`

**理由:**
- 严格测试 google.com 直接响应
- 避免跟随 301/302 重定向
- 符合 HEAD 请求最佳实践

**结果:** ✅ 良好 - TestDisableRedirect 验证通过

### 错误分类

**决策:** 使用类型断言分类错误，而非字符串匹配

**理由:**
- 类型断言更可靠，不受错误消息格式变化影响
- Go 标准错误处理模式
- 支持精确错误分类（DNS、超时、TLS、连接拒绝）

**结果:** ✅ 良好 - TestClassifyError, TestCheckConnectivity_Failure_Timeout 验证通过

### 状态追踪

**决策:** 维护 `ConnectivityState` 追踪上一次连通性状态

**理由:**
- 为 Phase 27 状态变化通知做准备
- 记录状态变化（首次检查、状态保持、状态改变）
- 支持连通性恢复检测

**结果:** ✅ 良好 - TestStateTracking 验证通过

### 立即首次检查

**决策:** `Start()` 方法立即执行首次检查，然后定期检查

**理由:**
- 避免等待第一个 interval 才执行检查
- 快速反馈连通性状态
- 符合 Phase 25 HealthMonitor 模式

**结果:** ✅ 良好 - TestStartImmediateCheck 验证通过

## Implementation Notes

### 日志格式

遵循项目规范，使用中文日志：
- 成功: `INFO Google 连通性检查成功 duration=Xms status_code=200`
- 失败: `ERROR Google 连通性检查失败 duration=Xms error_type="错误类型"`
- 状态变化: `WARN 连通性状态改变: 从连通变为不连通`

### 错误类型分类

支持以下错误类型分类：
- **DNS 解析失败:** `net.DNSError` - "DNS 解析失败: {hostname}"
- **连接超时:** `url.Error.Timeout()` - "连接超时"
- **连接被拒绝:** `syscall.ECONNREFUSED` - "连接被拒绝"
- **TLS 证书验证失败:** `tls.CertificateVerificationError` - "TLS 证书验证失败"
- **TLS 未知证书颁发机构:** `x509.UnknownAuthorityError` - "TLS 未知证书颁发机构"
- **HTTP 非 200:** "HTTP 状态码 {code} ({status})"

### 优雅关闭

使用 `context.Context` 实现优雅关闭：
1. `NewNetworkMonitor()` 创建可取消的 context
2. `Start()` 监听 `ctx.Done()` 信号
3. `Stop()` 调用 `cancel()` 取消 context
4. 监控循环收到信号后立即退出

### 并发安全

- `NetworkMonitor` 在独立 goroutine 中运行（非阻塞启动）
- `state` 字段通过 `checkConnectivity()` 单 goroutine 访问，无需互斥锁
- `GetState()` 供外部读取状态，简单返回指针（Phase 27 可能需要线程安全保护）

## Testing Summary

**测试框架:** Go testing (stdlib)

**测试覆盖:**
- 单元测试: 10 个测试用例
- 集成测试: 0 个（Phase 27 添加）
- E2E 测试: 0 个（Phase 27 添加）

**测试命令:**
```bash
# 运行 NetworkMonitor 测试
go test -v ./internal/network/...

# 运行完整测试套件
go test -v ./...
```

**测试结果:** ✅ 所有测试通过 (11/11)

## Files Changed

**创建:**
- `internal/network/monitor.go` (200 行) - NetworkMonitor 核心实现
- `internal/network/monitor_test.go` (383 行) - 单元测试

**总计:** 2 个文件，583 行代码

## Commits

1. **5d8ec0a** - test(26-01): add failing test for NetworkMonitor core implementation
   - 添加 10 个失败的测试用例（RED 阶段）

2. **40ac4af** - feat(26-01): implement NetworkMonitor core functionality
   - 实现 NetworkMonitor 核心功能（GREEN 阶段）
   - 修复 DNSError 字段名和 TLS 错误类型断言

## Next Steps

**Phase 26 Plan 02:** 集成 NetworkMonitor 到 main.go

**待办事项:**
- 在 `main.go` 中创建 NetworkMonitor 实例
- 在健康监控启动后启动网络监控
- 在应用关闭时优雅停止网络监控
- 更新配置加载逻辑（MonitorConfig 已存在）

**依赖:**
- Phase 25 已完成（健康监控已集成）
- Phase 26 Plan 01 已完成（NetworkMonitor 核心实现）

## Self-Check: PASSED

**验证项目:**

✅ **创建文件存在:**
- [x] internal/network/monitor.go (FOUND)
- [x] internal/network/monitor_test.go (FOUND)

✅ **提交存在:**
- [x] 5d8ec0a - test(26-01): add failing test (FOUND)
- [x] 40ac4af - feat(26-01): implement NetworkMonitor (FOUND)

✅ **测试通过:**
- [x] go test -v ./internal/network/... (PASS - 11/11 tests)

✅ **需求满足:**
- [x] MONITOR-01: 定期 HTTP HEAD 请求
- [x] MONITOR-02: ERROR 日志 + 错误分类
- [x] MONITOR-03: INFO 日志 + 响应时间

✅ **代码质量:**
- [x] 中文日志和注释（符合项目规范）
- [x] 结构清晰，职责分离
- [x] 错误处理完善
- [x] 遵循 Go 最佳实践

---

**Plan 执行完成**
**Duration:** 3 分钟
**Status:** ✅ 成功

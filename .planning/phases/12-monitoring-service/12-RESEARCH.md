# Phase 12: Monitoring Service - Research

**Researched:** 2026-03-16
**Domain:** Go 监控服务 + HTTP 连通性检查 + Context 驱动生命周期
**Confidence:** HIGH

## Summary

Phase 12 需要实现一个后台监控服务，每 15 分钟自动检查 Google 连通性，使用 HTTP GET 请求并记录日志。研究的核心发现是：**Go 标准库足以实现所有功能，无需新增依赖**。

关键架构决策:
1. **`time.Ticker` + `context.Context`** - 使用标准库实现固定间隔调度，替代 cron 库
2. **Context 驱动超时** - 使用 `context.WithTimeout()` 控制 HTTP 请求超时 (10秒)，避免 goroutine 泄漏
3. **失败不中断服务** - 监控检查失败时记录日志并继续运行，等待下次周期
4. **优雅停机** - 响应 `ctx.Done()` 信号，清理 ticker 并退出 goroutine

**主要风险:** Goroutine 泄漏 (ticker 未清理)、HTTP 请求超时不当导致资源耗尽、失败时未正确处理导致服务中断。

**Primary recommendation:** 使用 Go 标准库 `time.NewTicker()` + `context.WithTimeout()` + `net/http` 实现监控服务，严格遵循 `defer ticker.Stop()` 模式。

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `time.Ticker` | Go 1.24.11+ (标准库) | 15分钟间隔调度 | 固定间隔比 cron 表达式更简单直接，无需第三方库 |
| `context` | Go 1.24.11+ (标准库) | 超时控制和取消传播 | Go 惯用方式，支持跨 API 边界传播取消信号 |
| `net/http` | Go 1.24.11+ (标准库) | HTTP GET 请求 | 标准库稳定、无依赖风险，Go 1.22+ 增强功能完整 |
| `log/slog` | Go 1.24.11+ (现有) | 结构化日志 | 项目已实现自定义格式 `2024-01-01 12:00:00.123 - [INFO]: message` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `golang.org/x/sync/errgroup` | 待添加 | Goroutine 协调 (Phase 16) | 主函数协调 HTTP API 和监控服务，Phase 12 暂不需要 |
| `github.com/HQGroup/nanobot-auto-updater/internal/config` | 现有 | 监控配置读取 | 读取 MonitorConfig.Interval 和 MonitorConfig.Timeout |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `time.Ticker` | `robfig/cron` | cron 库对于固定间隔过于复杂，Ticker 更轻量直接 |
| `context.WithTimeout()` | `http.Client.Timeout` 字段 | context 支持取消传播，`http.Client.Timeout` 不会传播到 Request.Context |
| 手写 HTTP 检查 | `github.com/projectdiscovery/retryablehttp` | 重试库过重，本项目失败仅需记录日志，15分钟后自动重试 |

**Installation:**
```bash
# Phase 12 无需安装额外依赖
# 所有功能通过 Go 标准库实现
# errgroup 将在 Phase 16 添加: go get golang.org/x/sync/errgroup
```

## Architecture Patterns

### Recommended Project Structure
```
internal/monitor/
├── service.go         # 监控服务主体 + ticker 生命周期
├── checker.go         # HTTP 连通性检查器
├── service_test.go    # 单元测试
└── checker_test.go    # 检查器测试
```

### Pattern 1: Ticker + Context 生命周期

**What:** 使用 `time.NewTicker()` 创建定时器，结合 `context.Context` 实现可取消的监控循环

**When to use:** 所有需要定时执行的后台任务

**Example:**
```go
// Source: Go 官方文档 + 社区最佳实践
// https://pkg.go.dev/context - Context 取消传播
// https://oneuptime.com/blog/post/2026-01-23-go-context/view

func (s *Service) Run(ctx context.Context) error {
    ticker := time.NewTicker(s.interval)
    defer ticker.Stop() // 关键: 始终清理 ticker，防止 goroutine 泄漏

    s.logger.Info("监控服务启动", "interval", s.interval)

    for {
        select {
        case <-ctx.Done():
            s.logger.Info("监控服务停止 - 收到取消信号")
            return ctx.Err()
        case <-ticker.C:
            s.checkAndLog(ctx)
        }
    }
}
```

**关键点:**
1. `defer ticker.Stop()` 必须紧跟 `time.NewTicker()` 之后
2. `select` 同时监听 `ctx.Done()` 和 `ticker.C`
3. 返回 `ctx.Err()` 让调用者知道是正常取消还是其他错误

### Pattern 2: Context 驱动 HTTP 超时

**What:** 使用 `context.WithTimeout()` 控制单个 HTTP 请求的超时

**When to use:** 所有外部 HTTP 请求，特别是监控检查

**Example:**
```go
// Source: Cloudflare Blog - The complete guide to Go net/http timeouts
// https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/

func (c *Checker) CheckConnectivity(ctx context.Context) (bool, error) {
    // 创建带超时的子 context，不影响父 context
    ctx, cancel := context.WithTimeout(ctx, c.timeout)
    defer cancel() // 始终调用 cancel，防止 context 泄漏

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.targetURL, nil)
    if err != nil {
        return false, fmt.Errorf("创建请求失败: %w", err)
    }

    resp, err := c.client.Do(req)
    if err != nil {
        // 区分超时和其他错误
        if ctx.Err() == context.DeadlineExceeded {
            return false, fmt.Errorf("请求超时 (%v): %w", c.timeout, ctx.Err())
        }
        return false, fmt.Errorf("请求失败: %w", err)
    }
    defer resp.Body.Close()

    // 2xx 状态码视为成功 (Google 可能返回 302，但 2xx 更可靠)
    return resp.StatusCode >= 200 && resp.StatusCode < 300, nil
}
```

**为什么不用 `http.Client.Timeout`:**
- `http.Client.Timeout` 不会传播到 `Request.Context()`
- context 取消时，正在进行的请求不会被正确终止
- context 支持跨服务边界的超时传播

### Pattern 3: 失败不中断服务

**What:** 监控检查失败时记录日志，继续等待下次周期，不返回错误

**When to use:** 后台监控服务，失败不应导致服务停止

**Example:**
```go
func (s *Service) checkAndLog(ctx context.Context) {
    startTime := time.Now()
    connected, err := s.checker.CheckConnectivity(ctx)
    duration := time.Since(startTime)

    if err != nil {
        // 记录错误但不中断服务 (MON-05)
        s.logger.Warn("连通性检查失败",
            "error", err.Error(),
            "duration_ms", duration.Milliseconds(),
        )
        return
    }

    if connected {
        s.logger.Info("连通性检查成功",
            "status", "connected",
            "duration_ms", duration.Milliseconds(),
        )
    } else {
        s.logger.Warn("连通性检查失败 - 非2xx响应",
            "status", "disconnected",
            "duration_ms", duration.Milliseconds(),
        )
    }
}
```

### Anti-Patterns to Avoid

- **Ticker 无清理:** 创建 `time.NewTicker()` 但未调用 `defer ticker.Stop()` - 导致 goroutine 泄漏
- **阻塞在 ticker.C:** 只监听 `ticker.C` 不监听 `ctx.Done()` - 无法优雅停机
- **使用 http.Client.Timeout:** 不使用 context 控制超时 - 请求无法被取消
- **失败时返回错误:** 监控检查失败导致 `Run()` 返回错误 - 服务停止运行

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 定时调度 | 自实现 ticker 调度器 | `time.NewTicker()` | 标准库已经足够，自实现容易出错 |
| 超时控制 | 自实现 timeout channel | `context.WithTimeout()` | context 支持取消传播，标准模式 |
| HTTP 客户端 | 使用 `http.DefaultClient` | 创建自定义 `http.Client` | `http.DefaultClient` 无超时，生产环境危险 |
| 日志记录 | 自实现日志格式 | 项目现有 `log/slog` | 保持日志格式一致性 |

**Key insight:** Go 标准库提供了所有必要功能。自实现调度器或超时控制会引入复杂性和潜在 bug。

## Common Pitfalls

### Pitfall 1: Goroutine 泄漏 (Ticker 未清理)

**What goes wrong:** 创建 `time.NewTicker()` 但忘记调用 `defer ticker.Stop()`。Ticker goroutine 永久运行，内存泄漏。

**Why it happens:** 开发者记得 `defer` 关闭文件和 HTTP body，但忘记 ticker 也需要清理。

**How to avoid:**
```go
func (s *Service) Run(ctx context.Context) error {
    ticker := time.NewTicker(s.interval)
    defer ticker.Stop() // 始终在创建后立即 defer

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            // 处理
        }
    }
}
```

**Warning signs:**
- `runtime.NumGoroutine()` 随时间持续增长
- 内存使用缓慢增加
- pprof 显示 goroutine 卡在 `time.Sleep` 或 channel 操作

### Pitfall 2: HTTP 请求无超时

**What goes wrong:** 监控 HTTP 请求没有超时设置，网络挂起时 goroutine 永久阻塞。

**Why it happens:** 使用 `http.Get()` 或 `http.DefaultClient`，没有设置超时。

**How to avoid:**
```go
// 创建带超时的 HTTP 客户端
client := &http.Client{
    Timeout: 30 * time.Second, // 作为后备超时
    Transport: &http.Transport{
        DialContext: (&net.Dialer{
            Timeout:   5 * time.Second, // 连接建立超时
        }).DialContext,
        TLSHandshakeTimeout:   5 * time.Second,
        ResponseHeaderTimeout: 5 * time.Second,
    },
}

// 每个请求使用 context 控制精确超时
ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
defer cancel()
req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
```

**Warning signs:**
- 监控 goroutines 卡在 HTTP 请求中数分钟
- 网络中断期间内存激增
- 外部服务报告速率限制

### Pitfall 3: 监控循环不响应取消

**What goes wrong:** 监控循环只监听 `ticker.C`，不监听 `ctx.Done()`。Ctrl+C 时服务无法优雅停机。

**Why it happens:** 开发者专注于定时执行，忘记添加取消逻辑。

**How to avoid:**
```go
for {
    select {
    case <-ctx.Done(): // 必须监听 context 取消
        return ctx.Err()
    case <-ticker.C:
        // 处理
    }
}
```

**Warning signs:**
- Ctrl+C 后服务不退出
- 需要强制 kill 才能停止
- 日志显示服务仍在运行

### Pitfall 4: 失败时中断服务

**What goes wrong:** 监控检查失败时 `Run()` 返回错误，导致整个监控服务停止。

**Why it happens:** 习惯性地将错误向上传播，但监控服务应该持续运行。

**How to avoid:**
```go
func (s *Service) checkAndLog(ctx context.Context) {
    connected, err := s.checker.CheckConnectivity(ctx)
    if err != nil {
        s.logger.Warn("检查失败", "error", err)
        return // 记录日志后返回，不传播错误
    }
    // ...
}
```

**Warning signs:**
- 一次网络故障后监控服务停止
- 需要手动重启服务

## Code Examples

### 完整的监控服务实现

```go
// Source: Go 官方文档 + 项目架构研究
// internal/monitor/service.go

package monitor

import (
    "context"
    "log/slog"
    "net/http"
    "time"

    "github.com/HQGroup/nanobot-auto-updater/internal/config"
)

// Service 实现后台监控服务
type Service struct {
    interval  time.Duration
    timeout   time.Duration
    targetURL string
    client    *http.Client
    logger    *slog.Logger
}

// NewService 创建新的监控服务实例
func NewService(cfg config.MonitorConfig, logger *slog.Logger) *Service {
    return &Service{
        interval:  cfg.Interval,  // 默认 15 * time.Minute
        timeout:   cfg.Timeout,   // 默认 10 * time.Second
        targetURL: "https://www.google.com",
        client: &http.Client{
            Timeout: cfg.Timeout * 2, // 后备超时
            Transport: &http.Transport{
                DialContext: (&net.Dialer{
                    Timeout: 5 * time.Second,
                }).DialContext,
                TLSHandshakeTimeout:   5 * time.Second,
                ResponseHeaderTimeout: 5 * time.Second,
            },
        },
        logger: logger.With("component", "monitor"),
    }
}

// Run 启动监控服务，阻塞直到 context 被取消
func (s *Service) Run(ctx context.Context) error {
    ticker := time.NewTicker(s.interval)
    defer ticker.Stop()

    s.logger.Info("监控服务启动",
        "interval", s.interval,
        "target_url", s.targetURL,
        "timeout", s.timeout)

    for {
        select {
        case <-ctx.Done():
            s.logger.Info("监控服务停止")
            return ctx.Err()
        case <-ticker.C:
            s.checkAndLog(ctx)
        }
    }
}

// checkAndLog 执行一次连通性检查并记录日志
func (s *Service) checkAndLog(ctx context.Context) {
    startTime := time.Now()
    connected, err := s.checkConnectivity(ctx)
    duration := time.Since(startTime)

    if err != nil {
        s.logger.Warn("连通性检查失败",
            "error", err.Error(),
            "duration_ms", duration.Milliseconds(),
            "connected", false)
        return
    }

    if connected {
        s.logger.Info("连通性检查成功",
            "status", "connected",
            "duration_ms", duration.Milliseconds())
    } else {
        s.logger.Warn("连通性检查失败 - 非预期响应",
            "status", "disconnected",
            "duration_ms", duration.Milliseconds())
    }
}

// checkConnectivity 执行实际的 HTTP 检查
func (s *Service) checkConnectivity(ctx context.Context) (bool, error) {
    ctx, cancel := context.WithTimeout(ctx, s.timeout)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.targetURL, nil)
    if err != nil {
        return false, err
    }

    resp, err := s.client.Do(req)
    if err != nil {
        return false, err
    }
    defer resp.Body.Close()

    return resp.StatusCode >= 200 && resp.StatusCode < 300, nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `robfig/cron` 调度 | `time.Ticker` | v0.3 | 简化依赖，固定间隔更直接 |
| `http.Client.Timeout` 字段 | `context.WithTimeout()` | Go 1.7+ | 支持取消传播，避免 goroutine 泄漏 |
| 阻塞式 `for range ticker.C` | `select` + `ctx.Done()` | Go 1.0+ | 支持优雅停机 |
| `http.DefaultClient` | 自定义 `http.Client` | 最佳实践 | 防止无超时请求 |

**Deprecated/outdated:**
- `http.Get()` 无超时: 生产环境危险，使用自定义 client
- `cron` 表达式调度: 固定间隔用 `time.Ticker` 更简单

## Open Questions

1. **Google 响应状态码处理**
   - What we know: Google 首页可能返回 302 重定向
   - What's unclear: 是否需要跟随重定向，或仅检查可达性
   - Recommendation: 使用 2xx 状态码判定成功，不跟随重定向（重定向也证明网络可达）

2. **首次启动是否立即检查**
   - What we know: Ticker 首次触发是在第一个间隔后
   - What's unclear: 是否需要在启动时立即执行一次检查
   - Recommendation: 启动时立即执行一次检查，提供即时反馈

## Validation Architecture

> nyquist_validation 配置: 未在 config.json 中显式设置，视为启用

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (标准库) |
| Config file | 无 - 使用 Go convention |
| Quick run command | `go test ./internal/monitor/... -v -short` |
| Full suite command | `go test ./internal/monitor/... -v` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| MON-01 | 每 15 分钟自动检查 Google 连通性 | unit | `go test ./internal/monitor/... -run TestServiceInterval -v` | Wave 0 创建 |
| MON-04 | 记录所有监控检查日志 | unit | `go test ./internal/monitor/... -run TestServiceLogging -v` | Wave 0 创建 |
| MON-05 | 监控检查失败时继续运行 | unit | `go test ./internal/monitor/... -run TestServiceFailureContinue -v` | Wave 0 创建 |
| MON-08 | 使用 10 秒超时防止 HTTP 请求挂起 | unit | `go test ./internal/monitor/... -run TestCheckerTimeout -v` | Wave 0 创建 |

### Sampling Rate
- **Per task commit:** `go test ./internal/monitor/... -v -short`
- **Per wave merge:** `go test ./internal/monitor/... -v`
- **Phase gate:** `go test ./internal/monitor/... -v -race` (包含竞态检测)

### Wave 0 Gaps
- [ ] `internal/monitor/service.go` - 监控服务主体
- [ ] `internal/monitor/checker.go` - HTTP 连通性检查器
- [ ] `internal/monitor/service_test.go` - 服务测试
- [ ] `internal/monitor/checker_test.go` - 检查器测试

### Test Scenarios (详细)

**1. 15分钟间隔正确性测试 (MON-01)**
```go
func TestServiceInterval(t *testing.T) {
    // 使用短间隔测试 (100ms 模拟 15 分钟)
    cfg := config.MonitorConfig{
        Interval: 100 * time.Millisecond,
        Timeout:  10 * time.Second,
    }

    ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
    defer cancel()

    svc := NewService(cfg, slog.Default())
    checkCount := 0

    // 验证在 350ms 内执行 3 次检查
    // ...
}
```

**2. HTTP 超时处理测试 (MON-08)**
```go
func TestCheckerTimeout(t *testing.T) {
    // 创建延迟响应的测试服务器
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        time.Sleep(15 * time.Second) // 超过 10 秒超时
        w.WriteHeader(http.StatusOK)
    }))
    defer server.Close()

    checker := NewChecker(server.URL, 10*time.Second)
    ctx := context.Background()

    _, err := checker.CheckConnectivity(ctx)
    if err == nil {
        t.Error("Expected timeout error, got nil")
    }
    if !errors.Is(err, context.DeadlineExceeded) {
        t.Errorf("Expected DeadlineExceeded, got: %v", err)
    }
}
```

**3. 连续失败场景测试 (MON-05)**
```go
func TestServiceFailureContinue(t *testing.T) {
    // 创建始终失败的测试服务器
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusInternalServerError)
    }))
    defer server.Close()

    cfg := config.MonitorConfig{
        Interval: 50 * time.Millisecond,
        Timeout:  1 * time.Second,
    }

    ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
    defer cancel()

    svc := NewService(cfg, slog.Default())
    svc.targetURL = server.URL

    // Run 应该在 context 取消后正常返回，而不是因为检查失败而提前返回
    err := svc.Run(ctx)
    if !errors.Is(err, context.Canceled) {
        t.Errorf("Expected context.Canceled, got: %v", err)
    }
}
```

**4. 优雅停机测试 (MON-05)**
```go
func TestServiceGracefulShutdown(t *testing.T) {
    cfg := config.MonitorConfig{
        Interval: 1 * time.Second,
        Timeout:  10 * time.Second,
    }

    ctx, cancel := context.WithCancel(context.Background())

    svc := NewService(cfg, slog.Default())

    done := make(chan error, 1)
    go func() {
        done <- svc.Run(ctx)
    }()

    // 等待第一次检查完成
    time.Sleep(100 * time.Millisecond)

    // 发送取消信号
    cancel()

    // 验证在合理时间内退出
    select {
    case err := <-done:
        if !errors.Is(err, context.Canceled) {
            t.Errorf("Expected context.Canceled, got: %v", err)
        }
    case <-time.After(1 * time.Second):
        t.Error("Service did not stop within timeout")
    }
}
```

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| MON-01 | 系统每 15 分钟自动检查 Google 连通性 | `time.Ticker` 模式 + `time.NewTicker(interval)` |
| MON-04 | 系统记录所有监控检查日志 | `log/slog` 日志模式 + 检查时间/结果/状态记录 |
| MON-05 | 系统在监控检查失败时继续运行 | 失败不返回错误模式 + `checkAndLog()` 内部处理 |
| MON-08 | 系统使用超时机制 (10秒) | `context.WithTimeout(ctx, 10*time.Second)` + 自定义 HTTP client |

## Sources

### Primary (HIGH confidence)
- [pkg.go.dev/time - Ticker](https://pkg.go.dev/time#Ticker) - Go 官方 Ticker 文档
- [pkg.go.dev/context](https://pkg.go.dev/context) - Go 官方 Context 文档
- [The complete guide to Go net/http timeouts (Cloudflare Blog)](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/) - 超时配置权威指南
- [How to Use Context in Go for Cancellation and Timeouts](https://oneuptime.com/blog/post/2026-01-23-go-context/view) - Context 使用模式

### Secondary (MEDIUM confidence)
- [How to Avoid Common Goroutine Leaks in Go](https://oneuptime.com/blog/post/2026-01-07-go-goroutine-leaks/view) - Goroutine 泄漏预防
- [Go http client timeout vs context timeout (Stack Overflow)](https://stackoverflow.com/questions/64129364) - 社区共识
- [Timeouts in Go: A Comprehensive Guide (Better Stack)](https://betterstack.com/community/guides/scaling-go/golang-timeouts/) - 超时最佳实践

### Tertiary (LOW confidence)
- 无 - 所有关键发现都已通过官方文档或多来源验证

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - 基于 Go 官方文档 + 项目现有架构
- Architecture: HIGH - 基于 Context/Ticker 官方模式和社区最佳实践
- Pitfalls: HIGH - 基于多篇 HIGH confidence 文章 + 官方文档

**Research date:** 2026-03-16
**Valid until:** 30 days (Go 标准库稳定，模式长期有效)

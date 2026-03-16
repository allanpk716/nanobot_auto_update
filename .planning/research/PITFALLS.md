# Pitfalls Research

**Domain:** Adding HTTP API service and monitoring to existing Go application
**Researched:** 2026-03-16
**Confidence:** HIGH

## Executive Summary

本文档记录了为现有 nanobot-auto-updater 添加 HTTP API 服务和监控功能时的常见陷阱。v0.3 里程碑的核心挑战在于:从定时更新工具转变为监控服务 + HTTP API 触发更新模式,同时保持与现有实例管理系统的集成。

**关键发现:**
- Goroutine 泄漏是最高危陷阱,监控 ticker 和 HTTP 服务器都需要严格的清理机制
- 并发更新触发会导致资源冲突和状态不一致,必须在 API 层面实现互斥控制
- 优雅停机实现不当会导致操作中断和数据不一致,需要分阶段停机策略
- HTTP 客户端超时配置不当会导致资源耗尽和重试风暴
- Bearer token 认证的安全细节容易被忽视,需要使用常量时间比较和完整验证

---

## Critical Pitfalls

### Pitfall 1: Goroutine Leaks from Monitoring Ticker

**What goes wrong:**
监控 goroutine 使用 `time.NewTicker()` 启动但从未调用 `ticker.Stop()`。Ticker 永远触发,goroutine 永不退出,无限期消耗内存和 CPU。随着时间推移,goroutine 数量增长,最终导致 OOM 或性能下降。

**Why it happens:**
开发者记得使用 `defer` 关闭文件和 HTTP body,但忘记 ticker 也需要清理。`for range ticker.C` 模式看起来简单正确,但如果没有关闭机制,它会永远阻塞。

**How to avoid:**
```go
// 错误 - Goroutine 泄漏
func startMonitoring() {
    ticker := time.NewTicker(15 * time.Minute)
    go func() {
        for range ticker.C {
            checkGoogleConnectivity()
        }
    }()
}

// 正确 - 使用 context 进行清理
func startMonitoring(ctx context.Context) {
    ticker := time.NewTicker(15 * time.Minute)
    go func() {
        defer ticker.Stop() // 始终清理 ticker
        for {
            select {
            case <-ticker.C:
                checkGoogleConnectivity()
            case <-ctx.Done():
                log.Info("Monitoring goroutine shutting down")
                return
            }
        }
    }()
}
```

**Warning signs:**
- `runtime.NumGoroutine()` 随时间持续增长
- 即使在空闲期间内存使用也缓慢增加
- pprof 显示 goroutine 卡在 `time.Sleep` 或 channel 操作
- 运行数天/数周后,应用程序变得迟缓

**Phase to address:**
**Phase 2 (Monitoring Service Implementation)** - 实现监控服务时,确保每个 ticker 都有 `defer ticker.Stop()`,每个监控 goroutine 都尊重 context 取消。

**Sources:**
- [How to Avoid Common Goroutine Leaks in Go](https://oneuptime.com/blog/post/2026-01-07-go-goroutine-leaks/view) - HIGH confidence, 全面模式
- [Go Goroutines: 7 Critical Pitfalls](https://medium.com/@harshithgowdakt/go-goroutines-7-critical-pitfalls-every-developer-must-avoid-with-real-world-solutions-a436ac0fb4bb) - MEDIUM confidence

---

### Pitfall 2: Concurrent Update Triggers via HTTP API

**What goes wrong:**
多个 HTTP API 请求同时到达 `/api/v1/trigger-update`。每个都触发完整的更新流程。这会导致:
1. 资源竞争(多个 `uv` 进程同时运行)
2. 启动 nanobot 实例时端口冲突
3. 实例状态跟踪中的竞态条件
4. 相同失败重复发送 Pushover 通知
5. 在冗余操作上浪费 CPU/带宽

**Why it happens:**
HTTP API 天生是并发的。没有显式同步,每个请求都生成自己的 goroutine。开发者经常忘记"罕见"的并发事件在规模上或重试风暴期间会变得常见。

**How to avoid:**
```go
type UpdateService struct {
    mu               sync.Mutex
    updateInProgress bool
}

func (s *UpdateService) TriggerUpdate(w http.ResponseWriter, r *http.Request) {
    // 原子检查和设置
    s.mu.Lock()
    if s.updateInProgress {
        s.mu.Unlock()
        http.Error(w, "Update already in progress", http.StatusConflict)
        return
    }
    s.updateInProgress = true
    s.mu.Unlock()

    // 确保完成时清除标志
    defer func() {
        s.mu.Lock()
        s.updateInProgress = false
        s.mu.Unlock()
    }()

    // 执行更新
    if err := s.performUpdate(r.Context()); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "update completed"})
}
```

**更好的方法 - 带任务队列的单个 worker:**
```go
type UpdateService struct {
    updateCh chan struct{}
}

func NewUpdateService() *UpdateService {
    s := &UpdateService{
        updateCh: make(chan struct{}, 1), // 缓冲 - 只有 1 个待处理
    }
    go s.worker()
    return s
}

func (s *UpdateService) worker() {
    for range s.updateCh {
        s.performUpdate(context.Background())
    }
}

func (s *UpdateService) TriggerUpdate(w http.ResponseWriter, r *http.Request) {
    select {
    case s.updateCh <- struct{}{}:
        w.WriteHeader(http.StatusAccepted)
        json.NewEncoder(w).Encode(map[string]string{"status": "update triggered"})
    default:
        http.Error(w, "Update already queued", http.StatusConflict)
    }
}
```

**Warning signs:**
- 同时收到多个相同的失败通知
- 日志显示来自不同请求的交错更新操作
- `uv` 进程报告锁定文件或目录错误
- Nanobot 实例启动失败,提示"端口已被占用"

**Phase to address:**
**Phase 3 (HTTP API Implementation)** - 实现 trigger endpoint 时,在任何测试之前添加 mutex 或 worker 模式。这从一开始就防止竞态条件。

**Sources:**
- [How to Prevent Duplicate API Requests with Deduplication](https://oneuptime.com/blog/post/2026-01-25-prevent-duplicate-api-requests-deduplication-go/view) - MEDIUM confidence

---

### Pitfall 3: HTTP Client Timeout Causing Retry Storms

**What goes wrong:**
监控服务检查 Google 连通性时没有超时或超时很长。当网络挂起时:
1. 监控 goroutine 无限期阻塞
2. 多次检查尝试堆积
3. 网络恢复时,所有阻塞的请求同时完成
4. 产生流量峰值,可能触发速率限制
5. 对于更新:没有指数退避的重试会导致级联故障

**Why it happens:**
默认 `http.Client` 没有超时。开发者忘记配置超时,或设置得太长,认为"更多时间 = 更高可靠性"。但长超时会在中断期间导致资源积累。

**How to avoid:**
```go
// 配置合理的超时
var httpClient = &http.Client{
    Timeout: 10 * time.Second, // 总请求超时
    Transport: &http.Transport{
        DialContext: (&net.Dialer{
            Timeout:   5 * time.Second, // 连接建立
        }).DialContext,
        TLSHandshakeTimeout:   5 * time.Second,
        ResponseHeaderTimeout: 5 * time.Second,
    },
}

// 对于重试,使用带 jitter 的指数退避
func checkWithRetry(ctx context.Context) error {
    maxAttempts := 3
    baseDelay := 1 * time.Second

    for attempt := 0; attempt < maxAttempts; attempt++ {
        if attempt > 0 {
            // 带 jitter 的指数退避
            delay := baseDelay * time.Duration(1<<uint(attempt-1))
            jitter := time.Duration(rand.Int63n(int64(delay / 2)))
            time.Sleep(delay + jitter)
        }

        err := checkGoogleConnectivity(ctx)
        if err == nil {
            return nil
        }

        // context 取消时不重试
        if ctx.Err() != nil {
            return ctx.Err()
        }
    }

    return errors.New("max retry attempts exceeded")
}
```

**Warning signs:**
- 监控 goroutines 卡在 HTTP 请求中数分钟
- 网络中断期间内存激增
- 网络恢复后,巨大的 CPU 峰值
- 外部服务报告来自你的应用程序的速率限制
- 记录了重试尝试,但它们之间没有退避延迟

**Phase to address:**
**Phase 2 (Monitoring Service Implementation)** - 在所有 HTTP 客户端上设置显式超时。如果实现重试,立即添加指数退避。

**Sources:**
- [Timeout Budgets & Retries in Go: How Retry Storms Happen](https://levelup.gitconnected.com/timeout-budgets-retries-in-go-how-retry-storms-happen-stop-them-a6269958647d) - HIGH confidence
- [Mastering Network Timeouts and Retries in Go](https://dev.to/jones_charles_ad50858ddb0/mastering-network-timeouts-and-retries-in-go) - MEDIUM confidence

---

### Pitfall 4: Graceful Shutdown Abandoning In-Flight Operations

**What goes wrong:**
当服务收到 SIGTERM/SIGINT 时:
1. HTTP 服务器调用 `Shutdown()` 并设置 30 秒超时
2. Shutdown 完成但返回"context deadline exceeded"
3. 处理中的请求被中途放弃
4. 更新操作中断,系统处于不一致状态
5. 监控 goroutines 被杀死而未清理

**Why it happens:**
大多数示例展示 `srv.Shutdown(ctx)` 但没有解释超时到期时会发生什么:**处理中的请求被强制终止**。部分写入、未提交事务、半更新文件随之产生。"优雅"关闭变得突然。

**How to avoid:**
```go
func (s *Service) Shutdown(timeout time.Duration) error {
    // 创建关闭 context
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    // 1. 停止接受新的 HTTP 请求
    if err := s.httpServer.Shutdown(ctx); err != nil {
        log.WithError(err).Error("HTTP server shutdown error")
        // 无论如何继续 - 需要清理其他资源
    }

    // 2. 取消所有后台 goroutines
    s.cancelAll() // 取消根 context

    // 3. 等待 goroutines,使用单独的超时
    done := make(chan struct{})
    go func() {
        s.wg.Wait() // 跟踪所有 goroutines 的 WaitGroup
        close(done)
    }()

    select {
    case <-done:
        log.Info("All goroutines stopped gracefully")
        return nil
    case <-time.After(timeout):
        return fmt.Errorf("shutdown timeout: %d goroutines still running",
            runtime.NumGoroutine())
    }
}
```

**关键模式 - 使用单独的关闭阶段:**
```go
func main() {
    // ... 设置 ...

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    log.Info("Shutdown signal received")

    // 阶段 1: 停止接受新请求(快速)
    if err := httpServer.Shutdown(context.Background()); err != nil {
        log.WithError(err).Error("HTTP shutdown failed")
    }

    // 阶段 2: 让处理中的工作完成(更长的超时)
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    if err := service.WaitForCompletion(shutdownCtx); err != nil {
        log.WithError(err).Error("Wait for completion failed")
    }

    // 阶段 3: 清理资源(后台 goroutines 等)
    service.Cleanup()
}
```

**Warning signs:**
- 日志显示关闭期间"context deadline exceeded"
- 重启后检测到数据不一致
- 关闭恰好花费超时时长(表示达到超时)
- 监控显示进程退出时请求仍在处理中

**Phase to address:**
**Phase 4 (Integration & Lifecycle Management)** - 分阶段实现关闭,使用单独的超时。在有处理中请求的负载下测试关闭行为。

**Sources:**
- [Graceful Shutdown in Go: Why Most Implementations Are Wrong](https://medium.com/codex/graceful-shutdown-in-go-why-most-implementations-are-wrong-323ff193f1f8) - HIGH confidence

---

### Pitfall 5: Missing Bearer Token Validation

**What goes wrong:**
HTTP API endpoint 检查 Bearer token 但:
1. 接受空 token 字符串为有效
2. Token 比较区分大小写(不应该)
3. 没有常量时间比较(时序攻击漏洞)
4. 缺少 token 返回通用错误而不是 401
5. Token 记录在错误消息中(安全泄漏)

**Why it happens:**
安全代码通常作为事后补充添加。开发者复制粘贴基本认证示例而不理解安全含义。"它能工作"在他们的脑海中变成"它是安全的"。

**How to avoid:**
```go
func (s *Service) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // 1. 提取 Authorization header
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
            return
        }

        // 2. 解析 Bearer token
        parts := strings.Split(authHeader, " ")
        if len(parts) != 2 || parts[0] != "Bearer" {
            http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
            return
        }

        token := parts[1]
        if token == "" {
            http.Error(w, "Missing bearer token", http.StatusUnauthorized)
            return
        }

        // 3. 常量时间比较以防止时序攻击
        if !hmac.Equal([]byte(token), []byte(s.config.APIToken)) {
            // 不要记录 token!
            log.WithField("remote_addr", r.RemoteAddr).Warn("Invalid API token attempt")
            http.Error(w, "Invalid token", http.StatusUnauthorized)
            return
        }

        // Token 有效,继续到处理程序
        next(w, r)
    }
}
```

**配置验证:**
```go
func (c *Config) Validate() error {
    if c.APIToken == "" {
        return errors.New("api_token is required when HTTP API is enabled")
    }

    if len(c.APIToken) < 32 {
        return errors.New("api_token must be at least 32 characters for security")
    }

    return nil
}
```

**Warning signs:**
- API 接受空字符串为有效 token
- 多次失败的认证尝试使用相同的无效 token(暴力破解)
- Token 出现在日志或错误消息中
- 认证检查返回 500 而不是 401
- 认证失败没有速率限制

**Phase to address:**
**Phase 3 (HTTP API Implementation)** - 在测试 endpoint 之前实现认证中间件。使用 `crypto/hmac.Equal` 进行常量时间比较。从不记录 token。

**Sources:**
- [API Security Myth: The Bearer Model Is Enough](https://corsha.com/blog/api-security-myth-bearer-model-is-enough) - HIGH confidence
- [Secure & Scalable APIs in Go with JWT](https://levelup.gitconnected.com/mastering-jwt-authentication-in-go-10-expert-tips-for-secure-and-scalable-apis-723d16402b16) - MEDIUM confidence

---

### Pitfall 6: Port Conflicts on HTTP Server Startup

**What goes wrong:**
HTTP API 服务器尝试启动但端口已被使用:
1. `http.ListenAndServe()` 立即返回错误
2. 错误未正确处理 - 应用程序继续运行
3. 监控服务工作但 API endpoint 不可达
4. 令人困惑的用户体验 - 没有明确的错误消息
5. 意外启动了同一服务的多个实例

**Why it happens:**
在 goroutine 中启动 HTTP 服务器使错误处理变得棘手。开发者看到"server started"日志但错过了 `ListenAndServe` 的立即错误。应用程序继续,看起来健康,而 API 已损坏。

**How to avoid:**
```go
// 错误 - 错误在 goroutine 中丢失
go func() {
    if err := srv.ListenAndServe(":8080", handler); err != nil {
        log.Fatal(err) // 但这发生在 goroutine 中!
    }
}()

// 正确 - 正确的启动检查
func startHTTPServer(addr string, handler http.Handler) (*http.Server, error) {
    srv := &http.Server{Addr: addr, Handler: handler}

    // channel 接收启动结果
    started := make(chan error, 1)

    go func() {
        err := srv.ListenAndServe()
        // 当服务器停止时发生此错误(正常关闭返回 http.ErrServerClosed)
        started <- err
    }()

    // 给服务器一点时间启动(或失败)
    select {
    case err := <-started:
        // 立即错误意味着启动失败(例如,端口被占用)
        if err != http.ErrServerClosed {
            return nil, fmt.Errorf("HTTP server failed to start: %w", err)
        }
    case <-time.After(100 * time.Millisecond):
        // 没有立即错误,服务器可能成功启动
    }

    return srv, nil
}
```

**更好 - 直接使用 listener:**
```go
func startHTTPServer(addr string, handler http.Handler) (*http.Server, error) {
    // 首先尝试绑定到端口
    listener, err := net.Listen("tcp", addr)
    if err != nil {
        return nil, fmt.Errorf("failed to bind to %s: %w", addr, err)
    }

    srv := &http.Server{Handler: handler}

    go func() {
        if err := srv.Serve(listener); err != http.ErrServerClosed {
            log.WithError(err).Error("HTTP server error")
        }
    }()

    return srv, nil
}
```

**Warning signs:**
- HTTP 服务器记录"starting"但请求超时
- 应用程序运行但 API endpoints 返回 connection refused
- 多个进程绑定到同一端口
- 配置显示端口 8080 但 `netstat` 显示没有监听

**Phase to address:**
**Phase 3 (HTTP API Implementation)** - 启动 HTTP 服务器之前,尝试绑定到端口。如果端口不可用则返回明确错误。测试端口已被占用的情况。

**Sources:**
- [Go: start HTTP server asynchronously but return error if startup failed](https://stackoverflow.com/questions/66878374) - MEDIUM confidence

---

## Moderate Pitfalls

### Pitfall 7: Context Not Propagated to Update Operations

**What goes wrong:**
监控服务或 HTTP API 使用 context 触发更新,但更新操作忽略 context 取消。当请求关闭时,更新继续运行,阻塞优雅关闭直到超时。

**How to avoid:**
通过整个调用链传递 context。在长时间运行的循环中检查 `ctx.Done()`。

```go
func (s *Service) performUpdate(ctx context.Context) error {
    // 开始前检查
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }

    // 将 context 传递给所有操作
    for _, instance := range s.instances {
        if err := s.updateInstance(ctx, instance); err != nil {
            // 检查是否取消
            if ctx.Err() != nil {
                return ctx.Err()
            }
            log.WithError(err).Error("Instance update failed")
        }
    }

    return nil
}
```

**Phase to address:** Phase 3 (HTTP API Implementation)

---

### Pitfall 8: Instance State Corruption During Concurrent Access

**What goes wrong:**
监控 goroutine 检查实例状态而 HTTP API 更新实例。没有同步,读取看到部分更新的状态,导致不正确的行为或崩溃。

**How to avoid:**
为实例状态使用 `sync.RWMutex`,或更好地使用 channel 进行状态变更。

```go
type InstanceManager struct {
    mu        sync.RWMutex
    instances map[string]*Instance
}

func (m *InstanceManager) GetInstance(name string) (*Instance, bool) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    inst, ok := m.instances[name]
    return inst, ok
}

func (m *InstanceManager) UpdateInstance(name string, newInstance *Instance) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.instances[name] = newInstance
}
```

**Phase to address:** Phase 4 (Integration & Lifecycle Management)

---

### Pitfall 9: Pushover Notification on Every Monitoring Check

**What goes wrong:**
每次 Google 连通性检查失败时监控服务都发送 Pushover 通知。如果 Google 宕机数小时,用户会收到数百条通知。

**How to avoid:**
实现通知状态跟踪 - 仅在状态变化时通知(失败 → 恢复,恢复 → 失败)。

```go
type MonitoringService struct {
    lastState    bool // true = connected, false = disconnected
    stateChanged bool
    mu           sync.Mutex
}

func (s *MonitoringService) checkAndNotify(ctx context.Context) {
    connected := s.checkConnectivity(ctx)

    s.mu.Lock()
    stateChanged := (s.lastState != connected)
    s.lastState = connected
    s.mu.Unlock()

    if !stateChanged {
        return // 没有变化,不通知
    }

    if connected {
        s.notify("Google connectivity recovered")
    } else {
        s.notify("Google connectivity lost")
    }
}
```

**Phase to address:** Phase 2 (Monitoring Service Implementation)

---

## Minor Pitfalls

### Pitfall 10: Missing Health Check Endpoint

**What goes wrong:**
没有 `/health` endpoint 供编排器/监控系统检查服务是否健康。外部系统无法检测 HTTP API 何时因死锁或资源耗尽而无响应。

**How to avoid:**
实现简单的健康 endpoint 来检查关键依赖项。

```go
http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    // 检查服务是否健康
    if !service.IsHealthy() {
        w.WriteHeader(http.StatusServiceUnavailable)
        return
    }
    w.WriteHeader(http.StatusOK)
})
```

**Phase to address:** Phase 3 (HTTP API Implementation)

---

### Pitfall 11: Logs Missing Context for Debugging

**What goes wrong:**
日志消息不包含实例名称、操作类型或关联 ID。在调试生产问题时,无法跟踪请求流。

**How to avoid:**
使用带有 context 字段的结构化日志记录。延续 v0.2 的模式(logger.With 预注入)。

```go
func (s *Service) performUpdate(ctx context.Context, instance *Instance) {
    log := s.logger.WithFields(logrus.Fields{
        "instance": instance.Name,
        "port":     instance.Port,
        "operation": "update",
    })

    log.Info("Starting update")
    // ... 其余操作
}
```

**Phase to address:** Phase 3 (HTTP API Implementation) - 将现有日志模式扩展到新组件。

---

### Pitfall 12: Configuration Not Validated at Startup

**What goes wrong:**
只有在调用 API endpoint 时才发现缺少 API token 或端口无效。服务运行数小时/数天,然后在用户尝试使用时失败。

**How to avoid:**
在启动时验证所有配置,快速失败并给出明确错误消息。

```go
func (c *Config) Validate() error {
    var errs []error

    if c.HTTPAPIEnabled {
        if c.APIPort == 0 {
            errs = append(errs, errors.New("api_port required when HTTP API enabled"))
        }
        if c.APIToken == "" {
            errs = append(errs, errors.New("api_token required when HTTP API enabled"))
        }
        if c.APIPort < 1024 || c.APIPort > 65535 {
            errs = append(errs, fmt.Errorf("api_port %d out of valid range (1024-65535)", c.APIPort))
        }
    }

    if c.MonitoringEnabled {
        if c.MonitoringInterval < time.Minute {
            errs = append(errs, fmt.Errorf("monitoring_interval %v too short (minimum 1 minute)", c.MonitoringInterval))
        }
    }

    return errors.Join(errs...)
}
```

**Phase to address:** Phase 5 (Configuration & Validation)

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| 现有 Instance Manager | 不加锁访问实例 | 使用 v0.2 中现有的 mutex 或方法 |
| 现有 Notification System | 直接调用 Pushover 而不去重 | 重用 v0.2 的条件通知模式 |
| 现有 Logging System | 创建没有 context 字段的 logger | 使用 `logger.With` 创建 context-aware logger |
| 现有 Config System | 添加新字段而不验证 | 使用新字段扩展 `Validate()` 方法 |
| Graceful Shutdown | 忘记停止监控 goroutine | 添加到 WaitGroup,通过 context 取消 |
| Context Propagation | 在长时间操作中忽略 context | 在所有阻塞操作中检查 `ctx.Done()` |
| HTTP Server Lifecycle | 假设 `ListenAndServe` 总是成功 | 立即检查绑定错误,使用单独的启动检查 |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| 无缓冲通知 channel | 监控 goroutine 在发送时阻塞 | 使用缓冲 channel 或非阻塞发送 | 读取前 10+ 次监控失败 |
| HTTP client 没有超时 | 网络挂起时 goroutine 泄漏 | 在所有 HTTP 客户端上设置显式超时 | 第一次重大网络中断 |
| 每次请求验证 token | 重复认证检查导致 CPU 使用率高 | 缓存已验证的 token 并带 TTL(小心!) | >100 请求/秒 |
| 在锁下迭代实例列表 | 随实例增长 API 响应变慢 | 复制实例列表,无锁迭代 | >50 个实例 |
| 无限重试无退避 | 网络恢复时流量激增 | 使用指数退避 + jitter | 任何网络中断场景 |
| 监控检查堆积 | 多个检查同时运行 | 使用带缓冲的 ticker 或跳过逻辑 | 监控检查耗时 > 间隔 |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Token 在 URL 查询参数中 | Token 记录在访问日志、代理日志中 | 仅通过 Authorization header 接受 |
| 使用 == 比较 token | 时序攻击揭示 token 长度 | 使用 `crypto/hmac.Equal` |
| 缺少速率限制 | 暴力破解 token 猜测 | 添加速率限制器中间件 |
| Token 记录在错误中 | Token 在日志中暴露 | 仅记录"authentication failed" |
| 没有 token 轮换机制 | 泄露的 token 永久有效 | 记录轮换过程,支持重新加载 |
| HTTP 而不是 HTTPS | Token 在传输中被拦截 | 强制所有 API 调用使用 HTTPS |
| Token 存储在配置文件中 | 配置文件泄露暴露 token | 使用环境变量或加密存储 |

---

## "Looks Done But Isn't" Checklist

- [ ] **Monitoring Service:** 经常缺少 `defer ticker.Stop()` — 验证所有代码路径中的 ticker 清理
- [ ] **HTTP API:** 经常缺少请求超时 — 验证 HTTP 客户端配置了超时
- [ ] **Graceful Shutdown:** 经常放弃 goroutines — 使用 `runtime.NumGoroutine()` 检查验证所有 goroutines 已停止
- [ ] **Concurrent Updates:** 经常缺少 mutex — 验证正确处理多个并发请求
- [ ] **Auth Middleware:** 经常记录 token — 验证 token 从不出现在日志中
- [ ] **Port Binding:** 经常忽略启动错误 — 验证端口被占用时的错误处理
- [ ] **Context Propagation:** 在更新操作中经常被忽略 — 验证在长时间运行的循环中检查 context
- [ ] **Notification Deduplication:** 经常发送重复通知 — 验证状态跟踪防止垃圾邮件
- [ ] **State Tracking:** 监控状态变化经常缺少锁 — 验证并发访问受到保护

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Goroutine leak | LOW | 重启服务,修复代码,部署。重启时释放内存。 |
| Concurrent updates | MEDIUM | 检查实例状态,可能重启实例,修复代码 |
| Missing timeout | LOW | 修复代码,部署。没有持久状态损坏。 |
| Graceful shutdown failure | HIGH | 可能需要手动清理部分更新,重启实例 |
| Auth vulnerability | HIGH | 轮换所有 token,审计访问日志,修复代码 |
| Port conflict | LOW | 停止冲突进程,重启服务 |
| State corruption | HIGH | 审计实例状态,可能从备份恢复 |
| Context not propagated | MEDIUM | 重启服务,修复代码以尊重 context |
| Notification spam | LOW | 服务内部速率限制会自动恢复,用户可暂时关闭通知 |

---

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls:

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Goroutine leak | Phase 2 (Monitoring Service) | 使用 goleak 进行单元测试,验证前后的 goroutine 计数 |
| Concurrent updates | Phase 3 (HTTP API) | 使用并发请求进行集成测试,验证 mutex/worker |
| HTTP client timeout | Phase 2 (Monitoring Service) | 使用挂起的 HTTP 服务器测试,验证超时触发 |
| Graceful shutdown | Phase 4 (Integration) | 测试有处理中请求时的关闭,验证完成 |
| Auth validation | Phase 3 (HTTP API) | 测试空 token、错误 token、时序攻击,验证常量时间 |
| Port conflicts | Phase 3 (HTTP API) | 测试端口被占用时的启动,验证明确错误消息 |
| Context propagation | Phase 3 (HTTP API) | 测试更新期间的取消,验证操作停止 |
| State corruption | Phase 4 (Integration) | 使用并发读/写进行集成测试,验证一致性 |
| Notification spam | Phase 2 (Monitoring Service) | 测试重复失败,验证仅状态变化通知 |
| Missing health check | Phase 3 (HTTP API) | 测试 /health endpoint,验证返回正确状态 |
| Missing log context | Phase 3 (HTTP API) | 代码审查,验证所有日志包含相关 context |
| Config validation | Phase 5 (Configuration) | 测试使用无效配置启动,验证明确错误消息 |

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| 监控服务实现 | 忘记 `defer ticker.Stop()` | 代码模板中包含,审查检查 |
| HTTP API 实现 | 没有并发控制 | 在任何测试之前实现 mutex/worker 模式 |
| 认证中间件 | 使用 == 比较 token | 使用 `crypto/hmac.Equal`,从不记录 token |
| 优雅关闭 | 单一超时用于所有阶段 | 每个阶段使用单独超时(HTTP,goroutines,清理) |
| Context 传播 | 忽略长时间操作中的 context | 在所有阻塞循环中检查 `ctx.Done()` |
| 配置验证 | 启动时缺少必需字段验证 | 验证函数返回聚合错误 |
| HTTP client | 没有超时配置 | 为所有 client 设置显式超时 |
| 监控状态跟踪 | 没有状态变化的锁 | 使用 sync.Mutex 保护 lastState 字段 |

---

## Sources

### High Confidence Sources
- [How to Avoid Common Goroutine Leaks in Go](https://oneuptime.com/blog/post/2026-01-07-go-goroutine-leaks/view) - 全面的模式和生产示例
- [Timeout Budgets & Retries in Go: How Retry Storms Happen & Stop Them](https://levelup.gitconnected.com/timeout-budgets-retries-in-go-how-retry-storms-happen-stop-them-a6269958647d) - 生产验证的模式,真实场景
- [Graceful Shutdown in Go: Why Most Implementations Are Wrong](https://medium.com/codex/graceful-shutdown-in-go-why-most-implementations-are-wrong-323ff193f1f8) - 突出常见错误,详细分析
- [API Security Myth: The Bearer Model Is Enough](https://corsha.com/blog/api-security-myth-bearer-model-is-enough) - 安全最佳实践
- [How to Use Context in Go for Cancellation and Timeouts](https://oneuptime.com/blog/post/2026-01-23-go-context/view) - 官方模式,全面指南

### Medium Confidence Sources
- [Go Goroutines: 7 Critical Pitfalls Every Developer Must Avoid](https://medium.com/@harshithgowdakt/go-goroutines-7-critical-pitfalls-every-developer-must-avoid-with-real-world-solutions-a436ac0fb4bb) - 好的示例,涵盖常见错误
- [How to Prevent Duplicate API Requests with Deduplication in Go](https://oneuptime.com/blog/post/2026-01-25-prevent-duplicate-api-requests-deduplication-go/view) - 去重模式
- [Mastering Network Timeouts and Retries in Go](https://dev.to/jones_charles_ad50858ddb0/mastering-network-timeouts-and-retries-in-go) - 实用指南
- [Secure & Scalable APIs in Go with JWT](https://levelup.gitconnected.com/mastering-jwt-authentication-in-go-10-expert-tips-for-secure-and-scalable-apis-723d16402b16) - 安全提示
- [Mutex with timeout or channels in go](https://silh.medium.com/mutex-with-timeout-or-channels-in-go-d00b736fe45b) - 长时间运行任务的实用模式
- [Go: start HTTP server asynchronously but return error if startup failed](https://stackoverflow.com/questions/66878374) - Stack Overflow 社区答案

### Low Confidence Sources
- None - all findings verified across multiple sources

---

## Gaps to Address

**Topics needing phase-specific research:**
1. **Google 连通性检查的具体实现** - 需要确定检查 URL、预期响应、超时阈值
2. **与 v0.2 实例管理器的集成点** - 需要识别共享状态和锁的精确位置
3. **Pushover API 速率限制** - 需要查看官方文档确定通知频率限制
4. **长期运行稳定性** - 需要实际测试验证 24x7 运行时的资源使用模式

**Validation needed during implementation:**
- 实际测试并发更新触发场景,验证互斥机制有效性
- 测试监控服务的中断和恢复行为
- 验证优雅关闭在有长时间运行更新时的表现
- 测试 Bearer token 认证的各种边缘情况(空 token、格式错误等)

---

*Pitfalls research for: Adding HTTP API and monitoring to existing Go application (v0.3)*
*Researched: 2026-03-16*

# Phase 22: SSE Streaming API - Research

**Researched:** 2026-03-17
**Domain:** Go HTTP Server-Sent Events (SSE)、实时日志流式传输、长连接管理
**Confidence:** HIGH

## Summary

Phase 22 实现基于 Server-Sent Events (SSE) 协议的 HTTP 端点，用于实时推送 nanobot 实例的日志流。核心挑战在于:(1) 正确实现 SSE 协议格式;(2) 支持长连接并防止连接超时;(3) 检测客户端断开连接并清理资源，避免 goroutine 泄漏;(4) 与 Phase 19 的 LogBuffer Subscribe 机制无缝集成;(5) 区分 stdout 和 stderr 日志类型。

研究发现 SSE 是单向服务器推送协议，比 WebSocket 更简单，适合实时日志流场景。Go 标准库 `net/http` 足够实现 SSE，关键点包括：设置正确的 HTTP 头 (`Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`)，使用 `http.Flusher` 立即发送数据，通过 `r.Context().Done()` 检测客户端断开，每 30 秒发送心跳注释 (`: ping\n\n`) 防止代理超时，设置 HTTP 服务器 `WriteTimeout: 0` 支持长连接。

**Primary recommendation:** 使用 Go 标准库 `net/http` 实现 SSE 端点，无需外部 SSE 库。创建 `internal/api/sse.go` 文件，实现 `/api/v1/logs/:instance/stream` 端点，集成 LogBuffer.Subscribe() channel，使用 goroutine 转发日志到 HTTP ResponseWriter，启动独立心跳 goroutine (每 30 秒发送 `: ping\n\n`)，在客户端断开时调用 LogBuffer.Unsubscribe() 清理资源。

<phase_requirements>

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| **SSE-01** | 系统提供 `/api/v1/logs/:instance/stream` SSE 端点用于实时日志流 | 使用 `net/http` 实现，路由路径 `/api/v1/logs/:instance/stream`，从 URL 路径提取实例名称 |
| **SSE-02** | 系统使用 Server-Sent Events 协议推送日志到客户端 | 设置 `Content-Type: text/event-stream` 头，使用 SSE 格式发送日志 (`event: stdout\ndata: ...\n\n`) |
| **SSE-03** | 系统每 30 秒发送 SSE 心跳注释防止连接超时 | 使用 `time.Ticker` 每 30 秒发送 `: ping\n\n` (SSE 注释格式)，通过 proxy 和 load balancer |
| **SSE-04** | 系统检测客户端断开连接并停止发送事件 | 使用 `r.Context().Done()` channel 检测断开，退出时调用 `LogBuffer.Unsubscribe()` 清理 goroutine |
| **SSE-05** | 系统在客户端连接时发送缓冲区中的历史日志 | LogBuffer.Subscribe() 自动发送历史日志 (Phase 19 已实现)，SSE handler 只需转发 channel 数据 |
| **SSE-06** | 系统将 stdout 和 stderr 分别标记为不同事件类型 | 使用 SSE `event:` 字段区分：`event: stdout\n` 或 `event: stderr\n`，客户端通过 EventSource.addEventListener() 监听不同事件 |
| **SSE-07** | 系统设置 HTTP WriteTimeout 为 0 以支持长连接 | 配置 `http.Server{WriteTimeout: 0}`，或为 SSE 路由单独设置 (如果使用 router 中间件) |

</phase_requirements>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| **net/http** (标准库) | Go 1.24.11 | HTTP 服务器和 SSE 实现 | Go 标准库，无需外部依赖，性能稳定，支持 http.Flusher 接口 |
| **context** (标准库) | Go 1.24.11 | 检测客户端断开连接 | 标准库，通过 `r.Context().Done()` 实现，避免 goroutine 泄漏 |
| **time** (标准库) | Go 1.24.11 | 心跳定时器 | 标准库，使用 `time.Ticker` 每 30 秒发送心跳 |
| **fmt** (标准库) | Go 1.24.11 | 格式化 SSE 事件 | 标准库，使用 `fmt.Fprintf` 写入 SSE 格式数据 |
| **github.com/WQGroup/logger** | 项目已使用 | 日志记录 | 项目统一日志库，SSE handler 使用 `logger.With("component", "sse-handler")` 注入上下文 |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| **log/slog** (标准库) | Go 1.24.11 | 结构化日志 | 记录客户端连接/断开、发送失败等事件 |
| **encoding/json** (标准库) | Go 1.24.11 | 序列化日志数据 | 如果需要发送 JSON 格式日志内容 (可选) |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| **net/http 标准库** | **r3labs/sse** (第三方库) | r3labs/sse 提供更高级的 API，但增加外部依赖。对于简单 SSE 场景，标准库足够，且完全控制代码逻辑 |
| **net/http 标准库** | **gin-sse** (Gin 框架扩展) | Gin 框架提供 SSE 辅助函数，但项目未使用 Gin。保持轻量，使用标准库 |
| SSE (单向推送) | **WebSocket** (双向通信) | WebSocket 支持双向通信，但日志流只需要单向推送。SSE 更简单，自动重连，使用标准 HTTP 端口 |
| 手动格式化 SSE | **SSE 库** (如 tmaxmax/go-sse) | SSE 库封装格式化逻辑，但格式化很简单 (`data: ...\n\n`)，手动实现更透明且无依赖 |

**Installation:**

使用 Go 标准库，无需安装额外依赖。

```bash
# 无需 go get，使用标准库即可
```

**Version verification:**

```bash
go version
# 输出: go version go1.24.11 windows/amd64
```

## Architecture Patterns

### Recommended Project Structure

```
internal/
└── api/
    ├── server.go          # HTTP 服务器初始化、路由配置
    ├── sse.go             # SSE handler 实现 (本阶段核心)
    ├── sse_test.go        # SSE handler 单元测试
    └── middleware.go      # 认证中间件 (Bearer Token 验证)
```

### Pattern 1: SSE Handler 标准模式

**What:** 使用 `net/http` 实现 SSE handler，设置正确的 HTTP 头，使用 `http.Flusher` 立即发送数据

**When to use:** 所有 SSE 端点的标准实现模式

**Example:**

```go
// Source: 基于 Go 标准库 SSE 实现最佳实践
// Reference: https://oneuptime.com/blog/post/2026-02-01-go-realtime-applications-sse/view
package api

import (
    "fmt"
    "log/slog"
    "net/http"
)

// SSEHandler 处理 SSE 连接请求
func (s *Server) SSEHandler(w http.ResponseWriter, r *http.Request) {
    // 1. 设置 SSE 必需的 HTTP 头
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    // 可选: CORS 支持 (如果前端和后端不同源)
    // w.Header().Set("Access-Control-Allow-Origin", "*")

    // 2. 获取 Flusher 接口 (必需，用于立即发送数据)
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "Streaming not supported", http.StatusInternalServerError)
        return
    }

    // 3. 从 URL 路径提取实例名称
    instanceName := r.PathValue("instance") // Go 1.22+ 路由参数
    if instanceName == "" {
        http.Error(w, "Instance name required", http.StatusBadRequest)
        return
    }

    // 4. 获取实例的 LogBuffer
    logBuffer, err := s.instanceManager.GetLogBuffer(instanceName)
    if err != nil {
        // 实例不存在
        http.Error(w, fmt.Sprintf("Instance %s not found", instanceName), http.StatusNotFound)
        return
    }

    // 5. 订阅日志流
    logChan := logBuffer.Subscribe()
    defer logBuffer.Unsubscribe(logChan)

    s.logger.Info("SSE client connected", "instance", instanceName)

    // 6. 发送初始事件 (确认连接)
    fmt.Fprintf(w, "event: connected\ndata: {\"instance\":\"%s\"}\n\n", instanceName)
    flusher.Flush()

    // 7. 监听客户端断开
    ctx := r.Context()

    // 8. 启动心跳 goroutine (SSE-03)
    heartbeatTicker := time.NewTicker(30 * time.Second)
    defer heartbeatTicker.Stop()

    // 9. 主循环：转发日志和心跳
    for {
        select {
        case <-ctx.Done():
            // 客户端断开连接
            s.logger.Info("SSE client disconnected", "instance", instanceName)
            return

        case entry, ok := <-logChan:
            if !ok {
                // LogBuffer 关闭了 channel (实例被删除)
                s.logger.Info("LogBuffer channel closed", "instance", instanceName)
                return
            }

            // 发送日志事件 (SSE-06: 根据 Source 字段设置事件类型)
            event := "stdout"
            if entry.Source == "stderr" {
                event = "stderr"
            }

            // 格式化 SSE 事件
            fmt.Fprintf(w, "event: %s\n", event)
            fmt.Fprintf(w, "data: %s\n\n", entry.Content)
            flusher.Flush()

        case <-heartbeatTicker.C:
            // 发送心跳注释 (SSE-03)
            fmt.Fprint(w, ": ping\n\n")
            flusher.Flush()
        }
    }
}
```

### Pattern 2: 集成 LogBuffer Subscribe Channel

**What:** 订阅 LogBuffer channel，转发日志到 SSE 客户端，自动处理历史日志发送

**When to use:** 与 Phase 19 LogBuffer.Subscribe() 集成

**Example:**

```go
// Subscribe 返回的 channel 已包含历史日志 (Phase 19 实现)
// SSE handler 只需在 select 中监听 channel 即可

logChan := logBuffer.Subscribe()
defer logBuffer.Unsubscribe(logChan)

for {
    select {
    case entry := <-logChan:
        // 自动接收历史日志和实时日志
        writeSSEEvent(w, flusher, entry)
    case <-ctx.Done():
        return
    }
}
```

### Pattern 3: SSE 事件格式化

**What:** 使用标准 SSE 格式发送事件，区分 stdout 和 stderr

**When to use:** 发送每条日志到客户端

**Example:**

```go
// SSE 事件格式:
// event: <event-type>\n
// data: <payload>\n
// \n

func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, entry logbuffer.LogEntry) {
    // SSE-06: 根据来源设置事件类型
    eventType := "stdout"
    if entry.Source == "stderr" {
        eventType = "stderr"
    }

    // 写入 SSE 事件
    fmt.Fprintf(w, "event: %s\n", eventType)
    fmt.Fprintf(w, "data: %s\n\n", entry.Content) // 双换行符结束事件

    // 立即发送 (不缓冲)
    flusher.Flush()
}
```

### Pattern 4: 心跳机制防止超时

**What:** 每 30 秒发送 SSE 注释 (`: ping\n\n`)，防止代理和 load balancer 关闭空闲连接

**When to use:** 所有长连接 SSE 端点

**Example:**

```go
// Source: SSE 心跳最佳实践
// Reference: https://github.com/r3labs/sse/issues/101

// 启动心跳 goroutine
heartbeatTicker := time.NewTicker(30 * time.Second)
defer heartbeatTicker.Stop()

for {
    select {
    case <-heartbeatTicker.C:
        // SSE 注释格式 (冒号开头，浏览器 EventSource 忽略)
        fmt.Fprint(w, ": ping\n\n")
        flusher.Flush()
        s.logger.Debug("SSE heartbeat sent", "instance", instanceName)
    }
}
```

### Pattern 5: 客户端断开检测和资源清理

**What:** 使用 `r.Context().Done()` 检测客户端断开，defer 清理订阅

**When to use:** 防止 goroutine 泄漏

**Example:**

```go
// 订阅日志流
logChan := logBuffer.Subscribe()
// 使用 defer 确保退出时取消订阅
defer logBuffer.Unsubscribe(logChan)

// 获取请求 context
ctx := r.Context()

for {
    select {
    case <-ctx.Done():
        // 客户端断开连接 (关闭浏览器、网络中断等)
        // Context 被取消，退出循环
        s.logger.Info("SSE client disconnected",
            "instance", instanceName,
            "reason", ctx.Err()) // context.Canceled 或 context.DeadlineExceeded
        return // 退出 handler，defer 自动调用 Unsubscribe

    case entry := <-logChan:
        // 处理日志
        writeSSEEvent(w, flusher, entry)
    }
}
```

### Pattern 6: HTTP 服务器配置长连接

**What:** 设置 `WriteTimeout: 0` 支持无限时写入，适用于 SSE 长连接

**When to use:** 创建 HTTP 服务器时 (SSE-07)

**Example:**

```go
// Source: Go net/http 超时配置最佳实践
// Reference: https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/

func NewServer(cfg *config.APIConfig, instanceManager *instance.InstanceManager, logger *slog.Logger) *http.Server {
    mux := http.NewServeMux()

    // 注册 SSE handler
    sseHandler := &SSEHandler{
        instanceManager: instanceManager,
        logger:          logger,
    }
    mux.HandleFunc("GET /api/v1/logs/{instance}/stream", sseHandler.Handle)

    // SSE-07: WriteTimeout 设置为 0 支持长连接
    return &http.Server{
        Addr:         fmt.Sprintf(":%d", cfg.Port),
        Handler:      mux,
        ReadTimeout:  10 * time.Second,  // 读取请求超时
        WriteTimeout: 0,                 // 0 = 无超时，支持 SSE 长连接
        IdleTimeout:  120 * time.Second, // Keep-alive 空闲超时
    }
}
```

### Anti-Patterns to Avoid

- **忘记 Flush()**: 写入数据后未调用 `flusher.Flush()`，导致客户端接收延迟或超时
- **不处理客户端断开**: 不监听 `ctx.Done()`，客户端关闭连接后 goroutine 仍运行，导致 goroutine 泄漏
- **不发送心跳**: 代理和 load balancer (如 Nginx 默认 60 秒) 会关闭空闲连接，导致 SSE 连接中断
- **忘记 Unsubscribe**: 不调用 `LogBuffer.Unsubscribe()`，导致 LogBuffer 的 subscribers map 泄漏
- **使用 WriteTimeout > 0**: HTTP 服务器设置 `WriteTimeout` 会导致 SSE 连接超时中断，必须设置为 0
- **在 SSE 中发送二进制数据**: SSE 只支持文本格式，需要发送二进制数据时使用 base64 编码
- **不区分 stdout/stderr**: 所有日志使用相同事件类型，客户端无法区分错误输出和标准输出

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| **SSE 事件格式化** | 手动拼接字符串 `"event:...\ndata:...\n\n"` | `fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)` | 格式化简单但容易出错 (忘记双换行符)，使用 fmt.Fprintf 更清晰 |
| **客户端断开检测** | 自定义心跳机制检测断开 | `r.Context().Done()` | Go 标准库已提供，通过 HTTP server 自动检测 socket 关闭 |
| **HTTP 长连接管理** | 自己实现 connection pool | `http.Server{WriteTimeout: 0}` | 标准库已优化，无需手动管理 |
| **日志订阅管理** | 在 SSE handler 中维护订阅者列表 | `LogBuffer.Subscribe()` + `Unsubscribe()` | Phase 19 已实现，SSE handler 只需调用 API |

**Key insight:** SSE 实现的核心是 **正确设置 HTTP 头** + **使用 Flusher** + **检测客户端断开**。避免过度封装，保持代码简洁透明。

## Common Pitfalls

### Pitfall 1: 忘记调用 Flush() 导致客户端超时

**What goes wrong:** 写入 SSE 事件后未调用 `flusher.Flush()`，数据留在服务器缓冲区，客户端长时间接收不到数据导致超时断开

**Why it happens:** HTTP response 默认启用缓冲，提高性能。SSE 需要立即发送数据，必须手动 Flush

**How to avoid:**
1. 每次写入 SSE 事件后立即调用 `flusher.Flush()`
2. 在心跳发送后调用 `flusher.Flush()`
3. 编写测试验证客户端能立即接收到事件 (延迟 < 100ms)

**Warning signs:** 客户端 EventSource 超时，连接频繁重连，日志延迟 > 5 秒

### Pitfall 2: Goroutine 泄漏 (客户端断开后仍运行)

**What goes wrong:** 客户端关闭浏览器后，SSE handler goroutine 仍在运行，持续从 LogBuffer channel 读取数据，累积导致内存泄漏

**Why it happens:** 未监听 `r.Context().Done()`，客户端断开时 goroutine 不会自动退出

**How to avoid:**
1. 在 `for select` 循环中监听 `<-ctx.Done()`
2. 使用 `defer logBuffer.Unsubscribe(logChan)` 确保退出时清理
3. 编写测试模拟客户端断开，验证 goroutine 数量不增长 (`runtime.NumGoroutine()`)

**Warning signs:** `runtime.NumGoroutine()` 持续增长，内存占用上升，服务器运行几天后崩溃

### Pitfall 3: 代理超时关闭 SSE 连接

**What goes wrong:** Nginx、AWS ALB 等代理默认 60 秒超时，如果 60 秒内无数据传输，代理关闭连接，客户端收到连接错误

**Why it happens:** SSE 是长连接，但代理不知道连接类型，按 HTTP 短连接超时处理

**How to avoid:**
1. **服务端**: 每 30 秒发送心跳注释 `: ping\n\n` (比代理超时短)
2. **Nginx 配置**: `proxy_read_timeout 86400s;` `proxy_buffering off;`
3. **AWS ALB**: 增加 idle timeout 到 300 秒以上

**Warning signs:** 客户端每 60 秒断开重连，日志中出现频繁的 "client disconnected" 消息

### Pitfall 4: HTTP 服务器 WriteTimeout 导致连接中断

**What goes wrong:** HTTP 服务器配置了 `WriteTimeout: 30s`，SSE 连接在 30 秒后被强制中断，即使有数据发送

**Why it happens:** `WriteTimeout` 限制了每次 Write 操作的时间，SSE 长连接需要无限时

**How to avoid:**
1. 设置 `http.Server{WriteTimeout: 0}` (0 表示无超时)
2. 或为 SSE 路由单独创建 HTTP 服务器 (不影响其他 API 路由)
3. 编写测试验证长连接能持续 > 5 分钟

**Warning signs:** SSE 连接在固定时间后被中断 (如 30 秒、1 分钟)，错误日志显示 "write timeout"

### Pitfall 5: 不区分 stdout 和 stderr 日志

**What goes wrong:** 所有日志使用相同事件类型 (`event: message`)，客户端无法区分标准输出和错误输出，影响日志显示样式

**Why it happens:** 忘记在 SSE 事件中设置 `event:` 字段，或始终使用相同的值

**How to avoid:**
1. 根据 `LogEntry.Source` 字段设置事件类型: `event: stdout` 或 `event: stderr`
2. 客户端使用 `EventSource.addEventListener('stdout', ...)` 和 `addEventListener('stderr', ...)` 分别监听
3. 编写测试验证客户端能正确接收不同事件类型

**Warning signs:** Web UI 中所有日志显示相同颜色，无法快速识别错误

### Pitfall 6: 实例不存在时返回错误码错误

**What goes wrong:** 请求不存在的实例时返回 HTTP 500 或 200，而不是 HTTP 404 Not Found

**Why it happens:** 未正确处理 `InstanceManager.GetLogBuffer()` 返回的错误，或返回了通用的错误码

**How to avoid:**
1. 检查 `GetLogBuffer()` 返回的错误类型
2. 如果错误是 "instance not found"，返回 `http.Error(w, "Instance not found", http.StatusNotFound)`
3. 编写测试验证 404 响应码

**Warning signs:** 客户端收到 500 错误，无法区分实例不存在和服务器内部错误

## Code Examples

### 完整的 SSE Handler 实现

```go
// Source: Go SSE 实现最佳实践
// References:
// - https://oneuptime.com/blog/post/2026-02-01-go-realtime-applications-sse/view
// - https://saadkhaleeq.com/server-sent-events-sse-in-golang
package api

import (
    "fmt"
    "log/slog"
    "net/http"
    "time"

    "github.com/HQGroup/nanobot-auto-updater/internal/instance"
    "github.com/HQGroup/nanobot-auto-updater/internal/logbuffer"
)

// Server HTTP API 服务器
type Server struct {
    instanceManager *instance.InstanceManager
    logger          *slog.Logger
}

// NewServer 创建 HTTP API 服务器 (SSE-07: WriteTimeout = 0)
func NewServer(cfg *config.APIConfig, instanceManager *instance.InstanceManager, logger *slog.Logger) *http.Server {
    s := &Server{
        instanceManager: instanceManager,
        logger:          logger.With("component", "api-server"),
    }

    mux := http.NewServeMux()
    // SSE-01: 注册 SSE 端点
    mux.HandleFunc("GET /api/v1/logs/{instance}/stream", s.handleSSE)

    return &http.Server{
        Addr:         fmt.Sprintf(":%d", cfg.Port),
        Handler:      mux,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 0,                 // SSE-07: 无超时，支持长连接
        IdleTimeout:  120 * time.Second,
    }
}

// handleSSE 处理 SSE 日志流请求 (SSE-01 ~ SSE-07)
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
    // 1. 设置 SSE 必需的 HTTP 头
    w.Header().Set("Content-Type", "text/event-stream") // SSE-02
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    // 2. 获取 Flusher 接口
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "Streaming not supported", http.StatusInternalServerError)
        return
    }

    // 3. 从 URL 路径提取实例名称 (SSE-01)
    instanceName := r.PathValue("instance")
    if instanceName == "" {
        http.Error(w, "Instance name required", http.StatusBadRequest)
        return
    }

    // 4. 获取实例的 LogBuffer (SSE-04: ERR-04: 返回 404)
    logBuffer, err := s.instanceManager.GetLogBuffer(instanceName)
    if err != nil {
        s.logger.Warn("Instance not found", "instance", instanceName, "error", err)
        http.Error(w, fmt.Sprintf("Instance %s not found", instanceName), http.StatusNotFound)
        return
    }

    // 5. 订阅日志流 (SSE-05: 自动发送历史日志)
    logChan := logBuffer.Subscribe()
    defer logBuffer.Unsubscribe(logChan) // 确保退出时清理 (SSE-04)

    s.logger.Info("SSE client connected", "instance", instanceName)

    // 6. 发送连接确认事件
    fmt.Fprintf(w, "event: connected\ndata: {\"instance\":\"%s\"}\n\n", instanceName)
    flusher.Flush()

    // 7. 监听客户端断开 (SSE-04)
    ctx := r.Context()

    // 8. 启动心跳定时器 (SSE-03: 每 30 秒)
    heartbeatTicker := time.NewTicker(30 * time.Second)
    defer heartbeatTicker.Stop()

    // 9. 主循环：转发日志和心跳
    for {
        select {
        case <-ctx.Done():
            // SSE-04: 客户端断开连接
            s.logger.Info("SSE client disconnected",
                "instance", instanceName,
                "reason", ctx.Err())
            return // defer 自动调用 Unsubscribe

        case entry, ok := <-logChan:
            if !ok {
                // LogBuffer channel 关闭 (实例被删除)
                s.logger.Info("LogBuffer channel closed", "instance", instanceName)
                return
            }

            // SSE-06: 根据 Source 字段设置事件类型
            s.writeSSEEvent(w, flusher, entry)

        case <-heartbeatTicker.C:
            // SSE-03: 发送心跳注释
            fmt.Fprint(w, ": ping\n\n")
            flusher.Flush()
            s.logger.Debug("SSE heartbeat sent", "instance", instanceName)
        }
    }
}

// writeSSEEvent 写入 SSE 事件 (SSE-06: 区分 stdout/stderr)
func (s *Server) writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, entry logbuffer.LogEntry) {
    // 根据来源设置事件类型
    eventType := "stdout"
    if entry.Source == "stderr" {
        eventType = "stderr"
    }

    // 写入 SSE 事件 (标准格式)
    fmt.Fprintf(w, "event: %s\n", eventType)
    fmt.Fprintf(w, "data: %s\n\n", entry.Content) // 双换行符结束事件

    // 立即发送 (不缓冲)
    flusher.Flush()
}
```

### 客户端 JavaScript 示例 (EventSource API)

```javascript
// 客户端使用 EventSource API 连接 SSE 端点
// Reference: https://developer.mozilla.org/en-US/docs/Web/API/EventSource

const instanceName = 'nanobot-me';
const eventSource = new EventSource(`/api/v1/logs/${instanceName}/stream`);

// 监听连接事件
eventSource.addEventListener('connected', (e) => {
    console.log('SSE connected:', e.data);
    const data = JSON.parse(e.data);
    console.log('Instance:', data.instance);
});

// 监听 stdout 日志 (SSE-06)
eventSource.addEventListener('stdout', (e) => {
    console.log('[STDOUT]', e.data);
    // 显示为绿色
    appendLog(e.data, 'stdout');
});

// 监听 stderr 日志 (SSE-06)
eventSource.addEventListener('stderr', (e) => {
    console.error('[STDERR]', e.data);
    // 显示为红色
    appendLog(e.data, 'stderr');
});

// 错误处理
eventSource.onerror = (e) => {
    if (eventSource.readyState === EventSource.CLOSED) {
        console.log('SSE connection closed');
    } else {
        console.log('SSE connection error, will retry...');
    }
};

// 关闭连接
// eventSource.close();
```

### Nginx 代理配置 (防止超时)

```nginx
# Nginx 配置: 支持 SSE 长连接
# Reference: https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/

location /api/v1/logs/ {
    proxy_pass http://localhost:8080;
    proxy_http_version 1.1;

    # 禁用代理缓冲 (SSE 需要实时发送)
    proxy_buffering off;
    proxy_cache off;

    # 设置长连接超时 (1 小时)
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;

    # 保持 HTTP/1.1 连接
    proxy_set_header Connection '';
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| **WebSocket 双向通信** | SSE 单向推送 | HTML5 标准化 (2014) | SSE 更简单，自动重连，使用标准 HTTP，适合服务器推送场景 |
| **轮询 (Polling)** | SSE 长连接 | HTML5 标准化 (2014) | SSE 减少网络开销，实时性更高，服务器资源消耗更低 |
| **短连接 (WriteTimeout)** | 长连接 (WriteTimeout: 0) | Go 1.0+ | 支持无限时写入，SSE 连接可持续数小时甚至数天 |
| **自定义心跳协议** | SSE 注释格式 (`: ping\n\n`) | SSE 标准化 (2014) | 标准格式，浏览器自动忽略，代理和 load balancer 不中断 |

**Deprecated/outdated:**
- **长轮询 (Long Polling)**: 性能差，延迟高，已被 SSE 替代
- **Flash Socket**: 已废弃，现代浏览器不支持
- **iframe 流**: 兼容性差，性能低，已被 SSE 和 WebSocket 替代

## Open Questions

1. **是否需要认证 SSE 端点?**

   **What we know:**
   - REQUIREMENTS.md 标记日志认证为 "Out of Scope"
   - 依赖现有 API Bearer Token 或 localhost 访问
   - config.yaml 已配置 `api.bearer_token`

   **What's unclear:**
   - SSE 端点是否需要 Bearer Token 认证
   - 如何在 EventSource API 中传递 Bearer Token (不支持自定义 header)

   **Recommendation:**
   - **Phase 22 暂不实现认证**，保持简单，符合 "Out of Scope" 约束
   - 如果需要认证，可以通过 URL query 参数传递 token: `?token=xxx` (安全性较低但简单)
   - 或在 Phase 23 Web UI 中使用同源策略 (前端和后端同源，浏览器自动带 Cookie)

2. **是否需要限制每个实例的 SSE 连接数?**

   **What we know:**
   - 每个 SSE 连接占用一个 goroutine 和一个 LogBuffer subscriber slot
   - LogBuffer subscriber channel 容量 100，慢订阅者会丢弃日志
   - 理论上单个实例可以有无限 SSE 连接

   **What's unclear:**
   - 是否需要限制连接数防止资源耗尽
   - 限制策略 (每个实例最多 N 个连接? 全局最多 M 个连接?)

   **Recommendation:**
   - **Phase 22 不实现连接数限制**，保持简单
   - 如果监控发现连接数异常，可以在 Phase 23 或后续版本添加限制
   - 限制策略: 每个实例最多 10 个并发 SSE 连接，超过时返回 HTTP 429 Too Many Requests

3. **是否需要支持 SSE 重连 (Last-Event-ID)?**

   **What we know:**
   - SSE 标准支持 `Last-Event-ID` header，客户端重连时发送最后一个接收到的事件 ID
   - LogBuffer 有历史日志功能，理论上可以重放错过的事件
   - 但当前 LogEntry 结构没有 ID 字段 (Phase 19 未包含)

   **What's unclear:**
   - 是否需要实现重连机制，确保客户端不错过任何日志
   - 如何为日志生成唯一 ID (递增序号? 时间戳?)

   **Recommendation:**
   - **Phase 22 不实现重连机制**，保持简单
   - 理由: (1) LogBuffer.Subscribe() 已发送历史日志，客户端重连后会收到所有缓冲区日志;(2) 日志是时序数据，错过几条不影响整体理解;(3) 增加复杂度收益不大
   - 如果未来需要，可以在 LogEntry 中添加 `ID int64` 字段 (递增序号)

## Validation Architecture

> workflow.nyquist_validation 在 config.json 中未设置，默认为 true

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go 1.24.11 标准测试框架 (`testing` 包) |
| Config file | 无 (Go 测试不需要配置文件) |
| Quick run command | `go test -v ./internal/api` |
| Full suite command | `go test -v -race ./internal/api` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| **SSE-01** | 提供 `/api/v1/logs/:instance/stream` SSE 端点 | unit | `go test -v -run TestSSEEndpoint ./internal/api` | ❌ Wave 0 |
| **SSE-02** | 使用 SSE 协议推送日志 | unit | `go test -v -run TestSSEEventFormat ./internal/api` | ❌ Wave 0 |
| **SSE-03** | 每 30 秒发送 SSE 心跳注释 | unit | `go test -v -run TestSSEHeartbeat ./internal/api` | ❌ Wave 0 |
| **SSE-04** | 检测客户端断开连接并停止发送事件 | unit | `go test -v -run TestSSEClientDisconnect ./internal/api` | ❌ Wave 0 |
| **SSE-05** | 连接时发送缓冲区中的历史日志 | integration | `go test -v -run TestSSEHistoryLogs ./internal/api` | ❌ Wave 0 |
| **SSE-06** | stdout 和 stderr 分别标记为不同事件类型 | unit | `go test -v -run TestSSEEventTypes ./internal/api` | ❌ Wave 0 |
| **SSE-07** | HTTP WriteTimeout 设置为 0 支持长连接 | unit | `go test -v -run TestSSELongConnection ./internal/api` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test -v ./internal/api` (快速验证)
- **Per wave merge:** `go test -v -race ./internal/api` (完整 race 检测)
- **Phase gate:** `go test -v -race -cover ./internal/api` (覆盖率 > 80%)

### Wave 0 Gaps

- [ ] `internal/api/server.go` - HTTP 服务器初始化、路由配置
- [ ] `internal/api/sse.go` - SSE handler 实现 (本阶段核心)
- [ ] `internal/api/sse_test.go` - SSE handler 单元测试
- [ ] `internal/api/middleware.go` - 认证中间件 (Bearer Token 验证) - 可选，Phase 22 可不实现

## Sources

### Primary (HIGH confidence)

- **OneUptime Blog: How to Build Real-time Applications with Go and SSE**: https://oneuptime.com/blog/post/2026-02-01-go-realtime-applications-sse/view - Go SSE 实现完整指南，包含心跳、客户端断开检测、生产最佳实践 (验证时间: 2026-03-17)
- **Saad Khaleeq: Server-Sent Events (SSE) in Golang**: https://saadkhaleeq.com/server-sent-events-sse-in-golang - SSE 基础实现、EventSource API 使用、Gin 框架示例 (验证时间: 2026-03-17)
- **Go 标准库文档**: net/http, context, fmt - HTTP 服务器、Flusher 接口、Context 取消 (官方文档，无过期风险)
- **项目现有代码**: internal/logbuffer/buffer.go, internal/instance/manager.go - LogBuffer.Subscribe() API、InstanceManager.GetLogBuffer() (已实现，Phase 19/21)

### Secondary (MEDIUM confidence)

- **Stack Overflow: Golang server default timeout with SSE**: https://stackoverflow.com/questions/69864592/golang-server-default-timeout-with-long-polling-server-sent-event-calls-the-c - SSE 超时问题讨论，WriteTimeout 设置建议 (验证时间: 2026-03-17)
- **GitHub Issue: Keep-alive ping avoiding timeout issues**: https://github.com/r3labs/sse/issues/101 - SSE 心跳解决方案，Nginx 代理配置 (验证时间: 2026-03-17)
- **Cloudflare Blog: The complete guide to Go net/http timeouts**: https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/ - HTTP 服务器超时配置最佳实践 (验证时间: 2026-03-17)
- **r3labs/sse GitHub**: https://github.com/r3labs/sse - SSE 库实现参考 (虽然本项目不使用，但可参考设计模式)

### Tertiary (LOW confidence)

- **Medium: How I Implemented Server Sent Events in GO**: https://medium.com/@kristian15994/how-i-implemented-server-sent-events-in-go-3a55edcf4607 - 个人经验分享，实现方式验证 (验证时间: 2026-03-17)

## Metadata

**Confidence breakdown:**
- Standard stack: **HIGH** - Go 标准库稳定，SSE 协议简单标准化，无需外部依赖
- Architecture: **HIGH** - SSE 实现模式成熟，参考多个权威教程和开源项目，与 Phase 19 LogBuffer 集成清晰
- Pitfalls: **HIGH** - 基于 WebSearch 结果和社区经验总结，涵盖超时、goroutine 泄漏、代理配置等常见问题

**Research date:** 2026-03-17
**Valid until:** 2026-04-17 (30 天，Go 标准库和 SSE 协议稳定，不会重大变化)

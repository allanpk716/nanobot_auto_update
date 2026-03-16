# Stack Research: v0.4 实时日志查看

**Domain:** 进程输出捕获 + 内存缓冲 + Server-Sent Events 实时流
**Researched:** 2026-03-16
**Confidence:** HIGH

## Executive Summary

v0.4 里程碑为 nanobot 实例添加实时日志查看功能。研究表明：

1. **进程输出捕获** - 使用 Go 标准库 `os/exec.Cmd` 的 `StdoutPipe()` + `StderrPipe()`，结合 goroutine 并发读取
2. **内存环形缓冲** - 使用 `github.com/smallnest/ringbuffer` (线程安全、实现 `io.ReadWriter` 接口)
3. **Server-Sent Events** - 使用 `github.com/r3labs/sse/v2` (成熟稳定、与现有 `net/http` 服务器无缝集成)

**关键架构决策：**
- 不使用现有 `lifecycle/starter.go` 中的进程分离模式 (`cmd.Process.Release()`)
- 需要保持进程附着以持续读取 stdout/stderr
- 必须处理 `StdoutPipe()` 与 `cmd.Wait()` 的竞态条件 (GitHub Issue #19685)

---

## Recommended Stack

### Core Technologies (v0.4 新增)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `github.com/r3labs/sse/v2` | v2.0.0+ | Server-Sent Events 服务器 | 成熟稳定、简单易用、与现有 `net/http` 服务器无缝集成。支持多流订阅、自动重连、断开检测。相比 `tmaxmax/go-sse` 更轻量，社区使用广泛。 |
| `github.com/smallnest/ringbuffer` | 最新 | 环形缓冲区 (5000 行日志) | 线程安全、实现 `io.ReadWriter` 接口、零分配、高性能。相比手写 slice 更高效，自动覆盖旧日志。支持阻塞/非阻塞模式。 |
| `os/exec` (标准库) | Go 1.24.11+ | 进程输出捕获 | 标准库，使用 `cmd.StdoutPipe()` + `cmd.StderrPipe()` 捕获输出。配合 goroutine 并发读取。 |
| `net/http` (现有) | Go 1.24.11+ | SSE 端点集成 | 现有 HTTP API 服务器，添加 `/logs/{instance}` SSE 端点。无需引入框架。 |

### Supporting Libraries (无需新增)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `bufio.Scanner` (标准库) | Go 1.24.11+ | 按行读取 stdout/stderr | 分割进程输出为独立日志行，配合 ring buffer 存储 |
| `sync.Mutex` (标准库) | Go 1.24.11+ | 保护共享状态 | 保护 ring buffer 并发访问 (smallnest/ringbuffer 已内置线程安全) |
| `context` (标准库) | Go 1.24.11+ | goroutine 生命周期控制 | 取消日志捕获 goroutine、SSE 连接超时控制 |
| `log/slog` (现有) | Go 1.24.11+ | 结构化日志 | 记录日志捕获服务状态、错误信息 |

### Development Tools (现有)

| Tool | Purpose | Notes |
|------|---------|-------|
| Go 1.24.11 | 编译和运行时 | 项目当前版本 |
| `github.com/spf13/viper` | 配置管理 | 现有配置库，可能需要扩展日志缓冲大小配置 |
| `github.com/WQGroup/logger` | 日志记录 | 现有日志库，保持一致性 |

## Installation

```bash
# v0.4 需要安装的新依赖 (2 个)
go get github.com/r3labs/sse/v2
go get github.com/smallnest/ringbuffer

# 现有依赖保持不变:
# - github.com/gregdel/pushover (通知)
# - github.com/spf13/viper (配置)
# - golang.org/x/sys (Windows 系统调用)
# - gopkg.in/natefinch/lumberjack.v2 (日志轮转)
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| `r3labs/sse/v2` | `tmaxmax/go-sse` | 当需要 LLM 流式响应解析 (`sse.Read`)、更现代的 API、spec-compliant 严格验证时使用 `tmaxmax/go-sse`。本项目仅需基础 SSE 服务器，`r3labs/sse` 更简单。 |
| `smallnest/ringbuffer` | 手写 `[]string` + `Mutex` | 当需要自定义缓冲策略、复杂过期逻辑时考虑手写。本项目仅需固定大小环形缓冲，`ringbuffer` 更可靠高效。 |
| `os/exec` + `StdoutPipe()` | `go-cmd/cmd` 包装器 | 当需要非阻塞命令执行、自动重试、复杂进程管理时使用 `go-cmd/cmd`。本项目使用标准库足够，避免额外依赖。 |
| goroutine + channel | 单 goroutine 顺序读取 | 当 stdout/stderr 输出量小、无并发需求时顺序读取。本项目需要并发读取两个流，必须使用 goroutine。 |
| SSE | WebSocket | 当需要双向通信、二进制数据传输、复杂协议时使用 WebSocket。本项目仅需服务器推送日志流，SSE 更简单、HTTP 友好。 |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `cmd.Process.Release()` + `cmd.StdoutPipe()` | 进程分离后无法读取输出，`StdoutPipe()` 会在进程退出后关闭 | 保持进程附着 (`cmd.Wait()`)，在 goroutine 中持续读取 |
| `cmd.Output()` 或 `cmd.CombinedOutput()` | 阻塞直到进程退出，无法实时读取日志 | `cmd.StdoutPipe()` + goroutine 并发读取 |
| `io.ReadAll(stdoutPipe)` | 一次性读取所有输出，内存无限增长 | `bufio.Scanner` 逐行读取 + ring buffer 固定大小 |
| `http.Flusher` 手写 SSE | 需要处理 HTTP 头、重连、事件 ID、心跳等细节 | `r3labs/sse` 库已处理所有边界情况 |
| `http.DefaultClient` | 超时为 0 (无超时)，可能导致连接永久挂起 | 创建自定义 `http.Client{Timeout: 30s}` (现有代码已遵循) |
| 全局变量存储日志缓冲 | 并发访问不安全、难以测试、违反单一职责 | 每个 instance 持有自己的 `*ringbuffer.RingBuffer` |
| `time.Sleep()` 轮询读取 | CPU 浪费、延迟高 | 使用 `bufio.Scanner.Scan()` 阻塞等待新行 |

## Stack Patterns by Variant

### Pattern 1: 进程输出捕获 (并发读取 stdout/stderr)

**实现方式:**
```go
// Source: Go 官方文档 + 社区最佳实践
// 参考: https://github.com/golang/go/issues/19685 (竞态条件处理)

import (
    "bufio"
    "os/exec"
    "sync"
)

type LogCapture struct {
    cmd       *exec.Cmd
    buffer    *ringbuffer.RingBuffer
    logger    *slog.Logger
    ctx       context.Context
    cancel    context.CancelFunc
    wg        sync.WaitGroup
}

func (lc *LogCapture) Start() error {
    stdoutPipe, err := lc.cmd.StdoutPipe()
    if err != nil {
        return fmt.Errorf("failed to create stdout pipe: %w", err)
    }

    stderrPipe, err := lc.cmd.StderrPipe()
    if err != nil {
        return fmt.Errorf("failed to create stderr pipe: %w", err)
    }

    // 启动进程
    if err := lc.cmd.Start(); err != nil {
        return fmt.Errorf("failed to start process: %w", err)
    }

    // 并发读取 stdout 和 stderr
    lc.wg.Add(2)
    go lc.readStream(stdoutPipe, "stdout")
    go lc.readStream(stderrPipe, "stderr")

    // 启动 goroutine 等待进程退出
    go func() {
        lc.wg.Wait() // 等待两个读取 goroutine 完成
        err := lc.cmd.Wait()
        if err != nil {
            lc.logger.Warn("Process exited with error", "error", err)
        } else {
            lc.logger.Info("Process exited successfully")
        }
    }()

    return nil
}

func (lc *LogCapture) readStream(reader io.Reader, streamName string) {
    defer lc.wg.Done()

    scanner := bufio.NewScanner(reader)
    for scanner.Scan() {
        select {
        case <-lc.ctx.Done():
            return
        default:
            line := scanner.Text()
            // 写入 ring buffer (线程安全)
            lc.buffer.Write([]byte(line + "\n"))
        }
    }

    if err := scanner.Err(); err != nil {
        lc.logger.Error("Failed to read stream", "stream", streamName, "error", err)
    }
}

func (lc *LogCapture) Stop() {
    lc.cancel() // 取消所有 goroutine
    lc.wg.Wait() // 等待清理完成
    if lc.cmd.Process != nil {
        lc.cmd.Process.Kill()
    }
}
```

**关键点:**
1. **必须并发读取** stdout 和 stderr，否则一个流阻塞会导致另一个流无法读取
2. **使用 `sync.WaitGroup`** 等待两个读取 goroutine 完成后再调用 `cmd.Wait()`
3. **避免竞态条件** - 参考 GitHub Issue #19685，在读取完成前不要调用 `cmd.Wait()`
4. **Context 取消** - 支持优雅停止日志捕获

**为什么不用 `cmd.CombinedOutput()`:**
- `CombinedOutput()` 会阻塞直到进程退出
- 无法实时读取日志流
- 内存无限增长 (所有输出保存在内存中)

### Pattern 2: 环形缓冲区 (固定大小日志存储)

**实现方式:**
```go
// Source: smallnest/ringbuffer 官方文档
// https://github.com/smallnest/ringbuffer

import "github.com/smallnest/ringbuffer"

type InstanceLogs struct {
    buffer *ringbuffer.RingBuffer
    mu     sync.Mutex // 额外保护读取操作
}

func NewInstanceLogs(maxLines int) *InstanceLogs {
    // 假设每行平均 200 字节
    bufferSize := maxLines * 200
    return &InstanceLogs{
        buffer: ringbuffer.New(bufferSize),
    }
}

func (il *InstanceLogs) WriteLog(line string) {
    // ringbuffer 已线程安全，直接写入
    il.buffer.Write([]byte(line + "\n"))
}

func (il *InstanceLogs) ReadAll() ([]byte, error) {
    il.mu.Lock()
    defer il.mu.Unlock()

    // 读取所有数据但不消费 (Peek)
    data := make([]byte, il.buffer.Length())
    _, err := il.buffer.Read(data)
    if err != nil {
        return nil, err
    }
    return data, nil
}
```

**为什么使用 `smallnest/ringbuffer` 而非手写 `[]string`:**
1. **自动覆盖** - 旧日志自动被新日志覆盖，无需手动管理索引
2. **线程安全** - 内置并发保护，无需额外 `Mutex` (写入时)
3. **零分配** - 固定大小内存，无频繁分配/释放
4. **性能** - O(1) 写入和读取

**配置建议:**
```yaml
log_buffer:
  max_lines: 5000  # 保留最近 5000 行日志
  line_average_size: 200  # 每行平均字节数 (用于计算缓冲区大小)
```

### Pattern 3: SSE 实时推送 (集成现有 HTTP 服务器)

**实现方式:**
```go
// Source: r3labs/sse 官方文档 + 项目现有 HTTP 服务器
// https://github.com/r3labs/sse

import "github.com/r3labs/sse/v2"

type LogStreamingService struct {
    sseServer *sse.Server
    instances map[string]*InstanceLogs
    logger    *slog.Logger
}

func NewLogStreamingService(logger *slog.Logger) *LogStreamingService {
    server := sse.New()
    server.CreateStream("logs") // 创建全局日志流

    return &LogStreamingService{
        sseServer: server,
        instances: make(map[string]*InstanceLogs),
        logger:    logger,
    }
}

// 集成到现有 HTTP 服务器 (main.go)
func (s *LogStreamingService) RegisterRoutes(mux *http.ServeMux) {
    // SSE 端点: GET /api/v1/logs/{instance}
    mux.HandleFunc("GET /api/v1/logs/{instance}", s.handleLogStream)
}

func (s *LogStreamingService) handleLogStream(w http.ResponseWriter, r *http.Request) {
    instanceName := r.PathValue("instance")

    // 验证实例存在
    logs, exists := s.instances[instanceName]
    if !exists {
        http.Error(w, `{"error":"instance not found"}`, http.StatusNotFound)
        return
    }

    // 创建 SSE 流
    streamID := fmt.Sprintf("logs-%s", instanceName)
    s.sseServer.CreateStream(streamID)

    // 在后台 goroutine 中推送日志
    go s.streamLogs(r.Context(), streamID, logs)

    // SSE 服务器处理连接
    s.sseServer.ServeHTTP(w, r)
}

func (s *LogStreamingService) streamLogs(ctx context.Context, streamID string, logs *InstanceLogs) {
    ticker := time.NewTicker(100 * time.Millisecond) // 每 100ms 推送一次
    defer ticker.Stop()

    var lastOffset int

    for {
        select {
        case <-ctx.Done():
            s.logger.Info("SSE stream closed", "stream", streamID)
            return
        case <-ticker.C:
            // 读取新日志并推送
            data, err := logs.ReadNew(lastOffset)
            if err != nil {
                s.logger.Error("Failed to read logs", "error", err)
                continue
            }

            if len(data) > 0 {
                s.sseServer.Publish(streamID, &sse.Event{
                    Data: data,
                })
                lastOffset += len(data)
            }
        }
    }
}
```

**集成到现有 HTTP API 服务器:**
```go
// main.go 或 internal/api/server.go

func main() {
    // 现有 HTTP API 服务器
    mux := http.NewServeMux()

    // 现有端点...
    mux.HandleFunc("POST /api/v1/trigger-update", handleTriggerUpdate)
    mux.HandleFunc("GET /health", handleHealth)

    // 新增: 日志流服务
    logStreaming := NewLogStreamingService(logger)
    logStreaming.RegisterRoutes(mux)

    // 应用认证中间件 (可选: 日志流可能不需要认证)
    handler := authMiddleware(cfg.API.AuthToken)(mux)

    server := &http.Server{
        Addr:         fmt.Sprintf(":%d", cfg.API.Port),
        Handler:      handler,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 0, // SSE 需要长连接，设置为 0 (无超时)
        // IdleTimeout: 120 * time.Second, // 保持连接活跃
    }

    logger.Info("Starting HTTP server", "port", cfg.API.Port)
    if err := server.ListenAndServe(); err != nil {
        logger.Error("HTTP server failed", "error", err)
    }
}
```

**关键配置:**
- **`WriteTimeout: 0`** - SSE 需要长连接，不能设置写入超时
- **`IdleTimeout: 120s`** - 保持空闲连接活跃，避免被服务器断开
- **心跳机制** - 每 15 秒发送注释 `: heartbeat` 保持连接

**为什么选择 `r3labs/sse` 而非 `tmaxmax/go-sse`:**
1. **更简单** - `r3labs/sse` API 更直观，适合基础 SSE 服务器
2. **成熟稳定** - 2015 年开始维护，社区使用广泛
3. **轻量** - 无额外依赖，仅 SSE 功能
4. **足够用** - 本项目无需 LLM 流式解析 (`tmaxmax/go-sse` 的优势)

### Pattern 4: 进程管理变更 (不使用 Release)

**现有代码 (lifecycle/starter.go):**
```go
// 现有模式: 进程分离
cmd := exec.CommandContext(ctx, "cmd", "/c", command)
cmd.Start()
cmd.Process.Release() // 分离进程，父进程退出不影响子进程
```

**v0.4 新模式 (保持附着):**
```go
// 新模式: 保持进程附着以读取输出
cmd := exec.CommandContext(ctx, "cmd", "/c", command)

// 捕获 stdout/stderr
stdoutPipe, _ := cmd.StdoutPipe()
stderrPipe, _ := cmd.StderrPipe()

cmd.Start()

// 在 goroutine 中读取输出
go readStream(stdoutPipe)
go readStream(stderrPipe)

// 不要调用 cmd.Process.Release()
// 调用 cmd.Wait() 等待进程退出
go func() {
    cmd.Wait()
    logger.Info("Process exited")
}()
```

**架构影响:**
- **需要重构 `lifecycle/starter.go`** - 添加可选参数控制是否捕获输出
- **或者新建 `logcapture/capture.go`** - 专门处理带输出捕获的进程启动
- **向后兼容** - 现有不需日志捕获的启动逻辑保持不变

## Version Compatibility

| Package | Version | Compatible With | Notes |
|---------|---------|-----------------|-------|
| Go 1.24.11 | 标准库 | 项目当前版本 | `os/exec`, `bufio`, `context`, `sync` 全部兼容 |
| `github.com/r3labs/sse/v2` | v2.0.0+ | Go 1.24.11 | 成熟稳定，无已知兼容性问题 |
| `github.com/smallnest/ringbuffer` | 最新 | Go 1.24.11 | 线程安全，无依赖 |
| `net/http` (现有) | Go 1.22+ | Go 1.24.11 | ServeMux 方法匹配语法兼容 |

## Integration with Existing Codebase

### 集成点 1: InstanceManager 扩展

**现有结构 (internal/instance/manager.go):**
```go
type Manager struct {
    instances map[string]*InstanceLifecycle
    logger    *slog.Logger
}
```

**v0.4 扩展:**
```go
type Manager struct {
    instances map[string]*InstanceLifecycle
    logCapture map[string]*LogCapture // 新增: 每个 instance 的日志捕获器
    logger    *slog.Logger
}

func (m *Manager) StartInstanceWithLogging(ctx context.Context, name string) error {
    lifecycle := m.instances[name]

    // 创建日志捕获器
    capture := NewLogCapture(lifecycle.config, m.logger)
    if err := capture.Start(); err != nil {
        return err
    }

    m.logCapture[name] = capture
    return nil
}

func (m *Manager) StopInstanceWithLogging(ctx context.Context, name string) error {
    if capture, exists := m.logCapture[name]; exists {
        capture.Stop()
        delete(m.logCapture, name)
    }

    // 调用现有的停止逻辑
    return m.instances[name].StopForUpdate(ctx)
}
```

### 集成点 2: config 扩展

**新增配置 (v0.4):**
```yaml
# 日志缓冲配置
log_buffer:
  max_lines: 5000
  line_average_size: 200

# 现有配置保持不变...
instances:
  - name: "gateway"
    port: 18790
    start_command: "python -m nanobot.gateway"
```

**Go 配置结构:**
```go
type LogBufferConfig struct {
    MaxLines        int `yaml:"max_lines" mapstructure:"max_lines"`
    LineAverageSize int `yaml:"line_average_size" mapstructure:"line_average_size"`
}

type Config struct {
    LogBuffer LogBufferConfig `yaml:"log_buffer" mapstructure:"log_buffer"`
    // ... 现有字段
}
```

### 集成点 3: HTTP API 服务器扩展

**现有服务器 (internal/api/server.go):**
```go
type Server struct {
    config  config.APIConfig
    updater *updater.Updater
    logger  *slog.Logger
}

func (s *Server) Routes() *http.ServeMux {
    mux := http.NewServeMux()
    mux.HandleFunc("POST /api/v1/trigger-update", s.handleTriggerUpdate)
    mux.HandleFunc("GET /health", s.handleHealth)
    return mux
}
```

**v0.4 扩展:**
```go
type Server struct {
    config      config.APIConfig
    updater     *updater.Updater
    logStreamer *LogStreamingService // 新增
    logger      *slog.Logger
}

func (s *Server) Routes() *http.ServeMux {
    mux := http.NewServeMux()

    // 现有端点
    mux.HandleFunc("POST /api/v1/trigger-update", s.handleTriggerUpdate)
    mux.HandleFunc("GET /health", s.handleHealth)

    // 新增: 日志流端点
    mux.HandleFunc("GET /api/v1/logs/{instance}", s.logStreamer.handleLogStream)

    return mux
}
```

## Dependencies to Add (v0.4)

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/r3labs/sse/v2` | v2.0.0+ | SSE 服务器 |
| `github.com/smallnest/ringbuffer` | 最新 | 环形缓冲区 |

## Sources

### Server-Sent Events (SSE)

- [r3labs/sse GitHub Repository](https://github.com/r3labs/sse) — HIGH confidence (官方文档)
- [tmaxmax/go-sse GitHub Repository](https://github.com/tmaxmax/go-sse) — HIGH confidence (对比研究)
- [How to Build Real-time Applications with Go and SSE (OneUptime, Feb 2026)](https://oneuptime.com/blog/post/2026-02-01-go-realtime-applications-sse/view) — MEDIUM confidence (最新实践)
- [Real-Time Data Streaming with Server-Sent Events in Golang (Medium)](https://medium.com/@amineameur/real-time-data-streaming-with-server-sent-events-sse-in-golang-2ded26c9752e) — MEDIUM confidence (教程)
- [Live website updates with Go, SSE, and htmx (ThreeDotsTech)](https://threedots.tech/post/live-website-updates-go-sse-htmx/) — HIGH confidence (权威博客)

### Ring Buffer

- [smallnest/ringbuffer GitHub Repository](https://github.com/smallnest/ringbuffer) — HIGH confidence (官方文档)
- [Ring buffer in Golang (logdy.dev)](https://logdy.dev/blog/post/ring-buffer-in-golang) — MEDIUM confidence (教程)
- [A Practical Guide to Implementing a Generic Ring Buffer in Go (Medium)](https://medium.com/checker-engineering/a-practical-guide-to-implementing-a-generic-ring-buffer-in-go-866d27ec1a05) — MEDIUM confidence (理论)

### Process Output Capture

- [os/exec - Go Packages (官方文档)](https://pkg.go.dev/os/exec) — HIGH confidence (Go 官方)
- [os/exec: data race between StdoutPipe and Wait #19685](https://github.com/golang/go/issues/19685) — HIGH confidence (已知问题)
- [Some Useful Patterns for Go's os/exec (DoltHub Blog)](https://www.dolthub.com/blog/2022-11-28-go-os-exec-patterns/) — MEDIUM confidence (最佳实践)
- [go-cmd/cmd GitHub Repository](https://github.com/go-cmd/cmd) — LOW confidence (替代方案调研)

### Log Buffering

- [How to Implement Log Buffering (OneUptime, Jan 2026)](https://oneuptime.com/blog/post/2026-01-30-log-buffering/view) — MEDIUM confidence (最新实践)

---

*Stack research for: v0.4 实时日志查看功能*
*Researched: 2026-03-16*
*Confidence: HIGH (基于官方文档、权威博客、GitHub Issues)*

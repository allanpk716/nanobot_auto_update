# Architecture Research: Real-time Log Viewing

**Domain:** Real-time log streaming for nanobot instances
**Researched:** 2026-03-16
**Confidence:** HIGH

## Executive Summary

为现有的 nanobot-auto-updater 应用集成实时日志查看功能。核心架构采用三层模式:
1. **日志捕获层** - 修改 `lifecycle/starter.go` 以捕获 stdout/stderr
2. **缓冲层** - 使用 ring buffer 存储每个实例最近 5000 行日志
3. **流式传输层** - 通过 SSE (Server-Sent Events) 实时推送给客户端

集成点主要在现有的 `InstanceLifecycle` 和未来的 HTTP API 服务器。架构设计遵循最小侵入原则,复用现有的日志注入模式和上下文管理。

## Existing Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     Main Application                         │
│  (cmd/nanobot-auto-updater/main.go)                         │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────────────────────────────────────────────┐   │
│  │         InstanceManager (EXISTING)                    │   │
│  │  ┌──────────────┐  ┌──────────────┐                 │   │
│  │  │ InstanceLife │  │ InstanceLife │  ...            │   │
│  │  │  (gateway)   │  │  (worker)    │                 │   │
│  │  └──────┬───────┘  └──────┬───────┘                 │   │
│  │         │                  │                          │   │
│  │    ┌────▼─────┐      ┌────▼─────┐                   │   │
│  │    │Lifecycle │      │Lifecycle │                   │   │
│  │    │Starter   │      │Starter   │                   │   │
│  │    └──────────┘      └──────────┘                   │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                              │
│  Supporting Services (EXISTING):                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                  │
│  │ Updater  │  │ Notifier │  │ Logging  │                  │
│  └──────────┘  └──────────┘  └──────────┘                  │
└─────────────────────────────────────────────────────────────┘
```

**关键集成点:**
- `internal/instance/lifecycle.go` - 实例生命周期管理
- `internal/lifecycle/starter.go` - 进程启动逻辑 (需要修改)
- 未来: `internal/api/` - HTTP API 服务器 (v0.3 中创建)

## Proposed Architecture (v0.4)

### System Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     Main Application                         │
│  (cmd/nanobot-auto-updater/main.go)                         │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │         InstanceManager (MODIFIED)                    │   │
│  │  ┌─────────────────────────────────────────────┐     │   │
│  │  │  InstanceLifecycle (MODIFIED)                │     │   │
│  │  │  - config                                   │     │   │
│  │  │  - logger                                   │     │   │
│  │  │  - logBuffer *LogBuffer (NEW) ←─────────────┼──┐  │   │
│  │  └─────────────────────────────────────────────┘  │  │   │
│  │         │                                          │  │   │
│  │    ┌────▼─────┐                                   │  │   │
│  │    │Lifecycle │ (MODIFIED)                        │  │   │
│  │    │Starter   │                                   │  │   │
│  │    │- cmd     │                                   │  │   │
│  │    │- stdout ─┼─────────────────────┐            │  │   │
│  │    │- stderr ─┼──────────┐          │            │  │   │
│  │    └──────────┘          │          │            │  │   │
│  └──────────────────────────┼──────────┼────────────┘  │   │
│                             │          │               │  │   │
│                             ▼          ▼               │  │   │
│                    ┌──────────────────────┐           │  │   │
│                    │   LogBuffer (NEW)    │           │  │   │
│                    │  - ring buffer       │           │  │   │
│                    │  - 5000 lines        │           │  │   │
│                    │  - broadcaster chan  │◄──────────┘  │   │
│                    │  - subscribers map   │              │   │
│                    └──────────┬───────────┘              │   │
│                               │                          │   │
│  ┌────────────────────────────┼──────────────────────┐  │   │
│  │   HTTP API Server (v0.3)   │                      │  │   │
│  │  ┌──────────────┐          │                      │  │   │
│  │  │ /api/v1/     │          │                      │  │   │
│  │  │ trigger-upd  │          │                      │  │   │
│  │  └──────────────┘          │                      │  │   │
│  │  ┌──────────────┐          │                      │  │   │
│  │  │ /logs/:name  │◄─────────┘                      │  │   │
│  │  │  (SSE)       │  ┌──────────────────────┐      │  │   │
│  │  │  - auth      │  │ LogBufferManager     │      │  │   │
│  │  │  - stream    │  │ (NEW)                │      │  │   │
│  │  └──────────────┘  │ - buffers map        │◄─────┘  │   │
│  │                    │ - GetBuffer(name)    │         │   │
│  │  ┌──────────────┐  │ - Subscribe()        │         │   │
│  │  │ /logs/:name  │  └──────────────────────┘         │   │
│  │  │  /history    │                                   │   │
│  │  │  (JSON)      │                                   │   │
│  │  └──────────────┘                                   │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Implementation |
|-----------|----------------|----------------|
| **LogBuffer** (NEW) | 缓冲单个实例的日志,管理订阅者 | `internal/logbuffer/buffer.go` - ring buffer + broadcaster |
| **LogBufferManager** (NEW) | 管理所有实例的 LogBuffer,提供按名称查找 | `internal/logbuffer/manager.go` - map + sync.RWMutex |
| **Log Capture** (MODIFIED) | 在 `starter.go` 中捕获 stdout/stderr,写入 LogBuffer | `internal/lifecycle/starter.go` - 使用 `cmd.StdoutPipe()` + goroutine |
| **SSE Handler** (NEW) | HTTP handler 处理 `/logs/:name` SSE 连接 | `internal/api/log_handler.go` - 订阅 LogBuffer,推送事件 |
| **History Handler** (NEW) | HTTP handler 返回历史日志 (JSON) | `internal/api/log_handler.go` - 读取 buffer 历史数据 |
| **InstanceLifecycle** (MODIFIED) | 持有 LogBuffer 引用,传递给 starter | `internal/instance/lifecycle.go` - 添加 `logBuffer` 字段 |
| **InstanceManager** (MODIFIED) | 初始化 LogBufferManager,传递给实例 | `internal/instance/manager.go` - 添加 `logBufferManager` 字段 |

## New Components Detail

### 1. LogBuffer (`internal/logbuffer/buffer.go`)

**Purpose:** 存储单个实例的日志并管理订阅者广播

**Structure:**
```go
package logbuffer

import (
    "sync"
    "time"
)

// LogLine represents a single log line with metadata
type LogLine struct {
    Timestamp time.Time `json:"timestamp"`
    Stream    string    `json:"stream"` // "stdout" or "stderr"
    Content   string    `json:"content"`
    LineNum   int64     `json:"line_num"`
}

// LogBuffer is a thread-safe circular buffer for log lines with broadcasting
type LogBuffer struct {
    instanceName string
    maxSize      int                // Maximum number of lines to store
    lines        []LogLine          // Ring buffer storage
    head         int                // Write position
    count        int                // Current number of lines
    lineNum      int64              // Global line counter

    // Broadcasting
    subscribers  map[chan LogLine]bool
    newSub       chan chan LogLine
    removeSub    chan chan LogLine
    broadcast    chan LogLine

    mu           sync.RWMutex
    stopCh       chan struct{}
}

// NewLogBuffer creates a new log buffer for an instance
func NewLogBuffer(instanceName string, maxSize int) *LogBuffer {
    lb := &LogBuffer{
        instanceName: instanceName,
        maxSize:      maxSize,
        lines:        make([]LogLine, maxSize),
        subscribers:  make(map[chan LogLine]bool),
        newSub:       make(chan chan LogLine, 10),
        removeSub:    make(chan chan LogLine, 10),
        broadcast:    make(chan LogLine, 100),
        stopCh:       make(chan struct{}),
    }
    go lb.run()
    return lb
}

// Write adds a log line to the buffer (implements io.Writer for stdout/stderr)
func (lb *LogBuffer) Write(stream string) io.Writer {
    return &logWriter{buffer: lb, stream: stream}
}

// WriteLine adds a log line to the buffer (internal method)
func (lb *LogBuffer) writeLine(stream, content string) {
    lb.mu.Lock()
    line := LogLine{
        Timestamp: time.Now(),
        Stream:    stream,
        Content:   content,
        LineNum:   lb.lineNum,
    }
    lb.lineNum++

    // Ring buffer write
    lb.lines[lb.head] = line
    lb.head = (lb.head + 1) % lb.maxSize
    if lb.count < lb.maxSize {
        lb.count++
    }
    lb.mu.Unlock()

    // Broadcast to subscribers
    select {
    case lb.broadcast <- line:
    default:
        // Channel full, skip (non-blocking)
    }
}

// Subscribe returns a channel for receiving new log lines
func (lb *LogBuffer) Subscribe() chan LogLine {
    ch := make(chan LogLine, 50)
    lb.newSub <- ch
    return ch
}

// Unsubscribe removes a subscriber
func (lb *LogBuffer) Unsubscribe(ch chan LogLine) {
    lb.removeSub <- ch
}

// GetHistory returns the last n lines (or all if n > count)
func (lb *LogBuffer) GetHistory(n int) []LogLine {
    lb.mu.RLock()
    defer lb.mu.RUnlock()

    if n > lb.count {
        n = lb.count
    }

    result := make([]LogLine, n)
    start := (lb.head - n + lb.maxSize) % lb.maxSize

    for i := 0; i < n; i++ {
        result[i] = lb.lines[(start+i)%lb.maxSize]
    }

    return result
}

// run handles subscriber management and broadcasting in a single goroutine
func (lb *LogBuffer) run() {
    for {
        select {
        case <-lb.stopCh:
            return

        case ch := <-lb.newSub:
            lb.subscribers[ch] = true

        case ch := <-lb.removeSub:
            delete(lb.subscribers, ch)
            close(ch)

        case line := <-lb.broadcast:
            for ch := range lb.subscribers {
                select {
                case ch <- line:
                default:
                    // Slow client, skip this line
                }
            }
        }
    }
}

// Close stops the broadcaster and closes all subscriber channels
func (lb *LogBuffer) Close() {
    close(lb.stopCh)
    for ch := range lb.subscribers {
        close(ch)
    }
}

// logWriter implements io.Writer for a specific stream (stdout/stderr)
type logWriter struct {
    buffer *LogBuffer
    stream string
}

func (lw *logWriter) Write(p []byte) (n int, err error) {
    // Split by lines and write each line
    content := string(p)
    lines := strings.Split(content, "\n")

    for _, line := range lines {
        if line != "" {
            lw.buffer.writeLine(lw.stream, line)
        }
    }

    return len(p), nil
}
```

**Key Features:**
- **Ring Buffer**: 固定大小,覆盖最旧日志
- **Thread-Safe**: 使用 `sync.RWMutex` 保护并发访问
- **Broadcasting**: 单 goroutine 管理所有订阅者,避免竞态条件
- **Non-Blocking**: 广播时使用 `select + default` 避免慢客户端阻塞

### 2. LogBufferManager (`internal/logbuffer/manager.go`)

**Purpose:** 管理所有实例的 LogBuffer,提供按名称查找

**Structure:**
```go
package logbuffer

import "sync"

// Manager manages log buffers for all instances
type Manager struct {
    buffers map[string]*LogBuffer
    mu      sync.RWMutex
}

// NewManager creates a new log buffer manager
func NewManager() *Manager {
    return &Manager{
        buffers: make(map[string]*LogBuffer),
    }
}

// GetBuffer returns the log buffer for an instance, creating it if needed
func (m *Manager) GetBuffer(instanceName string, maxSize int) *LogBuffer {
    m.mu.Lock()
    defer m.mu.Unlock()

    if buf, exists := m.buffers[instanceName]; exists {
        return buf
    }

    buf := NewLogBuffer(instanceName, maxSize)
    m.buffers[instanceName] = buf
    return buf
}

// GetBufferIfExists returns the buffer only if it exists
func (m *Manager) GetBufferIfExists(instanceName string) (*LogBuffer, bool) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    buf, exists := m.buffers[instanceName]
    return buf, exists
}

// CloseAll closes all log buffers
func (m *Manager) CloseAll() {
    m.mu.Lock()
    defer m.mu.Unlock()

    for _, buf := range m.buffers {
        buf.Close()
    }
    m.buffers = make(map[string]*LogBuffer)
}
```

### 3. Modified Starter (`internal/lifecycle/starter.go`)

**Changes Required:**
- 添加 `logBuffer *logbuffer.LogBuffer` 参数
- 使用 `cmd.StdoutPipe()` 和 `cmd.StderrPipe()` 捕获输出
- 启动 goroutine 读取并写入 LogBuffer

**Modified Signature:**
```go
// StartNanobot starts nanobot with log capture
func StartNanobot(
    ctx context.Context,
    command string,
    port uint32,
    startupTimeout time.Duration,
    logBuffer *logbuffer.LogBuffer,  // NEW PARAMETER
    logger *slog.Logger,
) error {
    logger.Info("Starting nanobot with log capture", "command", command, "port", port)

    cmd := exec.CommandContext(ctx, "cmd", "/c", command)
    cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")
    cmd.SysProcAttr = &windows.SysProcAttr{
        HideWindow:    true,
        CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP,
    }

    // NEW: Capture stdout and stderr
    if logBuffer != nil {
        stdoutPipe, err := cmd.StdoutPipe()
        if err != nil {
            return fmt.Errorf("failed to create stdout pipe: %w", err)
        }
        stderrPipe, err := cmd.StderrPipe()
        if err != nil {
            return fmt.Errorf("failed to create stderr pipe: %w", err)
        }

        // Stream stdout to log buffer
        go streamToBuffer(stdoutPipe, logBuffer, "stdout", logger)
        // Stream stderr to log buffer
        go streamToBuffer(stderrPipe, logBuffer, "stderr", logger)
    }

    // Start process
    if err := cmd.Start(); err != nil {
        logger.Error("Failed to start nanobot process", "error", err)
        return fmt.Errorf("failed to start nanobot: %w", err)
    }

    logger.Info("Nanobot process started with log capture", "pid", cmd.Process.Pid)

    // Release process
    if err := cmd.Process.Release(); err != nil {
        logger.Warn("Failed to detach nanobot process (non-fatal)", "error", err)
        return fmt.Errorf("failed to detach nanobot process: %w", err)
    }

    // Wait for port
    if err := waitForPortListening(ctx, port, startupTimeout, logger); err != nil {
        return fmt.Errorf("nanobot startup verification failed: %w", err)
    }

    logger.Info("Nanobot startup verified", "port", port)
    return nil
}

// streamToBuffer reads from a pipe and writes to the log buffer
func streamToBuffer(pipe io.Reader, buffer *logbuffer.LogBuffer, stream string, logger *slog.Logger) {
    scanner := bufio.NewScanner(pipe)
    for scanner.Scan() {
        line := scanner.Text()
        buffer.WriteLine(stream, line)
        // Also log to application logger for debugging
        logger.Debug("Captured log line", "stream", stream, "line", line)
    }
    if err := scanner.Err(); err != nil {
        logger.Error("Error reading from pipe", "stream", stream, "error", err)
    }
}
```

**Integration Points:**
- 向后兼容: `logBuffer` 参数可以为 `nil` (禁用日志捕获)
- 复用现有的 `cmd.Start()` 和 `waitForPortListening()` 逻辑
- 不影响进程管理 (Release + 退出逻辑不变)

### 4. SSE Handler (`internal/api/log_handler.go`)

**Purpose:** 处理 `/logs/:name` SSE 连接,实时推送日志流

**Structure:**
```go
package api

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/HQGroup/nanobot-auto-updater/internal/logbuffer"
)

// LogHandler handles log streaming requests
type LogHandler struct {
    bufferManager *logbuffer.Manager
}

// NewLogHandler creates a new log handler
func NewLogHandler(bufferManager *logbuffer.Manager) *LogHandler {
    return &LogHandler{
        bufferManager: bufferManager,
    }
}

// ServeSSE handles GET /api/v1/logs/:name/stream (SSE endpoint)
func (h *LogHandler) ServeSSE(w http.ResponseWriter, r *http.Request) {
    instanceName := r.PathValue("name")
    if instanceName == "" {
        http.Error(w, "instance name required", http.StatusBadRequest)
        return
    }

    // Get buffer
    buffer, exists := h.bufferManager.GetBufferIfExists(instanceName)
    if !exists {
        http.Error(w, "instance not found", http.StatusNotFound)
        return
    }

    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("Access-Control-Allow-Origin", "*")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming not supported", http.StatusInternalServerError)
        return
    }

    // Subscribe to log updates
    subCh := buffer.Subscribe()
    defer buffer.Unsubscribe(subCh)

    // Send initial connection confirmation
    fmt.Fprintf(w, "event: connected\ndata: {\"instance\":\"%s\"}\n\n", instanceName)
    flusher.Flush()

    // Send last 100 lines as history
    history := buffer.GetHistory(100)
    for _, line := range history {
        data, _ := json.Marshal(line)
        fmt.Fprintf(w, "event: log\ndata: %s\n\n", data)
        flusher.Flush()
    }

    // Stream new logs
    ctx := r.Context()

    // Start heartbeat goroutine
    heartbeatCh := make(chan struct{})
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()
        defer close(heartbeatCh)

        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                fmt.Fprint(w, ": heartbeat\n\n")
                flusher.Flush()
            }
        }
    }()

    for {
        select {
        case <-ctx.Done():
            return

        case <-heartbeatCh:
            return

        case line := <-subCh:
            data, _ := json.Marshal(line)
            fmt.Fprintf(w, "event: log\ndata: %s\n\n", data)
            flusher.Flush()
        }
    }
}

// ServeHistory handles GET /api/v1/logs/:name/history (JSON endpoint)
func (h *LogHandler) ServeHistory(w http.ResponseWriter, r *http.Request) {
    instanceName := r.PathValue("name")
    if instanceName == "" {
        http.Error(w, "instance name required", http.StatusBadRequest)
        return
    }

    // Get buffer
    buffer, exists := h.bufferManager.GetBufferIfExists(instanceName)
    if !exists {
        http.Error(w, "instance not found", http.StatusNotFound)
        return
    }

    // Get all history (or query param ?lines=100)
    lines := buffer.GetHistory(5000)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "instance": instanceName,
        "count":    len(lines),
        "lines":    lines,
    })
}
```

**SSE Event Format:**
```
event: connected
data: {"instance":"gateway"}

event: log
data: {"timestamp":"2026-03-16T10:00:00Z","stream":"stdout","content":"Starting server...","line_num":1}

event: log
data: {"timestamp":"2026-03-16T10:00:01Z","stream":"stderr","content":"Warning: ...","line_num":2}

: heartbeat
```

### 5. Modified InstanceLifecycle (`internal/instance/lifecycle.go`)

**Changes Required:**
- 添加 `logBuffer *logbuffer.LogBuffer` 字段
- 在 `StartAfterUpdate()` 中传递给 `lifecycle.StartNanobot()`

**Modified Structure:**
```go
type InstanceLifecycle struct {
    config    config.InstanceConfig
    logger    *slog.Logger
    logBuffer *logbuffer.LogBuffer  // NEW FIELD
}

// NewInstanceLifecycle creates an instance lifecycle manager with log buffer
func NewInstanceLifecycle(cfg config.InstanceConfig, logBuffer *logbuffer.LogBuffer, baseLogger *slog.Logger) *InstanceLifecycle {
    instanceLogger := baseLogger.With("instance", cfg.Name).With("component", "instance-lifecycle")

    return &InstanceLifecycle{
        config:    cfg,
        logger:    instanceLogger,
        logBuffer: logBuffer,  // NEW
    }
}

// StartAfterUpdate starts the instance with log capture
func (il *InstanceLifecycle) StartAfterUpdate(ctx context.Context) error {
    il.logger.Info("Starting instance after update with log capture")

    startupTimeout := il.config.StartupTimeout
    if startupTimeout == 0 {
        startupTimeout = 30 * time.Second
    }

    // Pass log buffer to starter
    if err := lifecycle.StartNanobot(ctx, il.config.StartCommand, il.config.Port, startupTimeout, il.logBuffer, il.logger); err != nil {
        return &InstanceError{
            InstanceName: il.config.Name,
            Operation:    "start",
            Port:         il.config.Port,
            Err:          fmt.Errorf("failed to start instance: %w", err),
        }
    }

    il.logger.Info("Instance started successfully with log capture")
    return nil
}
```

### 6. Modified InstanceManager (`internal/instance/manager.go`)

**Changes Required:**
- 添加 `logBufferManager *logbuffer.Manager` 字段
- 在创建 `InstanceLifecycle` 时传递对应的 `LogBuffer`

**Modified Structure:**
```go
type InstanceManager struct {
    instances        []*InstanceLifecycle
    logBufferManager *logbuffer.Manager  // NEW FIELD
    logger           *slog.Logger
}

// NewInstanceManager creates an instance manager with log buffer support
func NewInstanceManager(cfg *config.Config, baseLogger *slog.Logger) *InstanceManager {
    logger := baseLogger.With("component", "instance-manager")

    // Create log buffer manager
    logBufferManager := logbuffer.NewManager()

    instances := make([]*InstanceLifecycle, 0, len(cfg.Instances))
    for _, instCfg := range cfg.Instances {
        // Create log buffer for this instance (5000 lines)
        logBuf := logBufferManager.GetBuffer(instCfg.Name, 5000)

        // Pass log buffer to instance lifecycle
        lifecycle := NewInstanceLifecycle(instCfg, logBuf, baseLogger)
        instances = append(instances, lifecycle)
    }

    return &InstanceManager{
        instances:        instances,
        logBufferManager: logBufferManager,
        logger:           logger,
    }
}

// GetLogBufferManager returns the log buffer manager (for API handlers)
func (m *InstanceManager) GetLogBufferManager() *logbuffer.Manager {
    return m.logBufferManager
}
```

## Data Flow

### Log Capture Flow

```
[Nanobot Process]
    stdout/stderr
         ↓
[cmd.StdoutPipe() / cmd.StderrPipe()]
         ↓
[bufio.Scanner] (goroutine)
         ↓
[LogBuffer.WriteLine()]
         ↓
[Ring Buffer Storage] (5000 lines)
         ↓
[Broadcast Channel] → [Subscriber Channels]
         ↓                    ↓
    [History API]      [SSE Handler]
```

### SSE Connection Flow

```
[Client: GET /api/v1/logs/gateway/stream]
    ↓
[Auth Middleware] → Validate Bearer token
    ↓
[LogHandler.ServeSSE()]
    ↓
[bufferManager.GetBufferIfExists("gateway")]
    ↓ (exists)
[buffer.Subscribe()] → Create subscriber channel
    ↓
[Send initial history] → Last 100 lines (event: log)
    ↓
[Stream loop]
    ├─> [Heartbeat goroutine] → Every 30s: ": heartbeat"
    └─> [Read from subCh] → New line: event: log with JSON data
         ↓
    [Client receives SSE events]
```

### History API Flow

```
[Client: GET /api/v1/logs/gateway/history]
    ↓
[Auth Middleware] → Validate Bearer token
    ↓
[LogHandler.ServeHistory()]
    ↓
[buffer.GetHistory(5000)] → Read all lines from ring buffer
    ↓
[JSON Response]
{
  "instance": "gateway",
  "count": 3245,
  "lines": [
    {"timestamp": "...", "stream": "stdout", "content": "...", "line_num": 1},
    ...
  ]
}
```

## Project Structure

```
nanobot-auto-updater/
├── cmd/
│   └── nanobot-auto-updater/
│       └── main.go              # MODIFIED: Wire LogBufferManager to API
├── internal/
│   ├── logbuffer/               # NEW PACKAGE
│   │   ├── buffer.go            # LogBuffer with ring buffer + broadcast
│   │   ├── buffer_test.go       # Unit tests
│   │   ├── manager.go           # LogBufferManager
│   │   └── manager_test.go      # Unit tests
│   ├── api/                     # NEW PACKAGE (v0.3)
│   │   ├── server.go            # HTTP server lifecycle
│   │   ├── handlers.go          # /api/v1/trigger-update
│   │   ├── log_handler.go       # NEW: /api/v1/logs/:name/*
│   │   ├── middleware.go        # Bearer token auth
│   │   └── server_test.go
│   ├── instance/                # MODIFIED
│   │   ├── manager.go           # MODIFIED: Add logBufferManager field
│   │   ├── lifecycle.go         # MODIFIED: Add logBuffer field
│   │   └── errors.go
│   ├── lifecycle/               # MODIFIED
│   │   ├── starter.go           # MODIFIED: Add logBuffer parameter, capture stdout/stderr
│   │   ├── stopper.go
│   │   └── detector.go
│   ├── config/
│   │   └── config.go            # UNCHANGED (v0.3 adds API config)
│   ├── updater/                 # UNCHANGED
│   ├── notifier/                # UNCHANGED
│   └── logging/                 # UNCHANGED
└── config.yaml                  # UNCHANGED
```

## Build Order (Suggested Implementation Phases)

### Phase 1: Log Buffer Core (Foundation)
**Goal:** Implement log buffer infrastructure

**Changes:**
1. Create `internal/logbuffer/` package
2. Implement `LogBuffer` with ring buffer + broadcasting
3. Implement `LogBufferManager` for multi-instance support
4. Add comprehensive unit tests (concurrency, ring buffer edge cases)

**Dependencies:** None

**Validation:**
- Unit tests pass with 100% coverage
- Ring buffer correctly handles wrap-around
- Broadcasting works with multiple subscribers
- No goroutine leaks

---

### Phase 2: Integrate Log Capture into Starter
**Goal:** Capture nanobot process stdout/stderr

**Changes:**
1. Modify `internal/lifecycle/starter.go`:
   - Add `logBuffer *logbuffer.LogBuffer` parameter
   - Use `cmd.StdoutPipe()` and `cmd.StderrPipe()`
   - Add `streamToBuffer()` goroutine
2. Add integration tests with mock processes
3. Verify backward compatibility (nil logBuffer works)

**Dependencies:** Phase 1

**Validation:**
- Process starts successfully with log capture
- Logs appear in buffer
- No blocking on slow buffer
- Process lifecycle (start/stop) unaffected

---

### Phase 3: Wire Log Buffers to Instance Lifecycle
**Goal:** Connect LogBuffer to InstanceLifecycle and InstanceManager

**Changes:**
1. Modify `internal/instance/lifecycle.go`:
   - Add `logBuffer *logbuffer.LogBuffer` field
   - Pass logBuffer to `lifecycle.StartNanobot()`
2. Modify `internal/instance/manager.go`:
   - Add `logBufferManager *logbuffer.Manager` field
   - Create buffers for each instance in constructor
   - Add `GetLogBufferManager()` method
3. Update tests with mock log buffers

**Dependencies:** Phase 2

**Validation:**
- Each instance has its own LogBuffer
- Buffers accessible via manager
- Existing update flow (stop→update→start) works unchanged

---

### Phase 4: SSE Handler Implementation
**Goal:** Implement HTTP endpoints for log viewing

**Changes:**
1. Create `internal/api/log_handler.go`:
   - Implement `LogHandler` struct
   - Implement `ServeSSE()` for `/logs/:name/stream`
   - Implement `ServeHistory()` for `/logs/:name/history`
2. Add route registration in `internal/api/server.go`:
   ```go
   mux.HandleFunc("GET /api/v1/logs/{name}/stream", logHandler.ServeSSE)
   mux.HandleFunc("GET /api/v1/logs/{name}/history", logHandler.ServeHistory)
   ```
3. Add unit tests with mock buffers

**Dependencies:** Phase 3, v0.3 API server

**Validation:**
- SSE connection established
- History API returns JSON
- Heartbeat keeps connection alive
- Client disconnect detected via context

---

### Phase 5: Integration Testing
**Goal:** Validate end-to-end log viewing

**Tests:**
1. Start instance with log capture
2. Connect SSE client → verify receives logs in real-time
3. Request history → verify all lines returned
4. Disconnect client → verify no goroutine leak
5. Slow client → verify non-blocking behavior
6. Multiple instances → verify isolation

**Dependencies:** All phases

**Validation:** E2E test passes, memory stable, no leaks

---

### Phase 6: Documentation and Examples
**Goal:** Provide usage documentation

**Changes:**
1. Update `README.md` with log viewing API documentation
2. Add API examples:
   - `curl` commands for history endpoint
   - JavaScript EventSource example for SSE
   - Error handling examples
3. Document configuration (buffer size)

**Dependencies:** Phase 5

**Validation:** Documentation reviewed and tested

## Architectural Patterns

### Pattern 1: Ring Buffer for Memory Efficiency

**What:** Fixed-size circular buffer to store recent logs without unbounded memory growth

**When:** All log capture scenarios

**Trade-offs:**
- ✅ Bounded memory usage (5000 lines × ~200 bytes = ~1MB per instance)
- ✅ No need for log rotation or cleanup
- ✅ Simple implementation with array
- ❌ Loses old logs (acceptable for real-time viewing)

**Example:**
```go
// Ring buffer write
lb.lines[lb.head] = line
lb.head = (lb.head + 1) % lb.maxSize
if lb.count < lb.maxSize {
    lb.count++
}

// Ring buffer read (last n lines)
start := (lb.head - n + lb.maxSize) % lb.maxSize
for i := 0; i < n; i++ {
    result[i] = lb.lines[(start+i)%lb.maxSize]
}
```

---

### Pattern 2: Broadcast Channel Pattern

**What:** Single goroutine manages all subscribers, ensuring thread-safe broadcasting

**When:** Multiple SSE clients need to receive same log stream

**Trade-offs:**
- ✅ Thread-safe (no race conditions)
- ✅ Centralized subscriber management
- ✅ Easy to add/remove subscribers
- ❌ Single goroutine overhead (minimal)

**Example:**
```go
func (lb *LogBuffer) run() {
    for {
        select {
        case ch := <-lb.newSub:
            lb.subscribers[ch] = true

        case line := <-lb.broadcast:
            for ch := range lb.subscribers {
                select {
                case ch <- line:
                default:
                    // Skip slow client
                }
            }
        }
    }
}
```

---

### Pattern 3: SSE with Heartbeat

**What:** Send periodic SSE comments to keep connection alive through proxies

**When:** Long-lived SSE connections (all production deployments)

**Trade-offs:**
- ✅ Prevents proxy timeout (Nginx default 60s)
- ✅ Simple implementation (just `: ping\n\n`)
- ✅ No client-side handling needed
- ❌ Minimal bandwidth overhead (10 bytes every 30s)

**Example:**
```go
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            fmt.Fprint(w, ": heartbeat\n\n")
            flusher.Flush()
        }
    }
}()
```

---

### Pattern 4: Context-Aware Shutdown

**What:** Use `r.Context()` to detect client disconnect and clean up resources

**When:** All SSE handlers

**Trade-offs:**
- ✅ Automatic cleanup on client disconnect
- ✅ Works with HTTP/2 and proxies
- ✅ Standard Go pattern
- ❌ Requires explicit select in loop

**Example:**
```go
ctx := r.Context()

for {
    select {
    case <-ctx.Done():
        // Client disconnected, cleanup and return
        return
    case line := <-subCh:
        // Send log line
    }
}
```

## Anti-Patterns to Avoid

### Anti-Pattern 1: Blocking Writes to Full Buffer

**What people do:** Use unbuffered channels for broadcasting, causing slow clients to block writes

**Why it's wrong:** One slow client blocks log capture for all instances

**Do this instead:** Use buffered channels + `select + default` to skip slow clients

```go
// BAD
for ch := range subscribers {
    ch <- line  // Blocks if client slow
}

// GOOD
for ch := range subscribers {
    select {
    case ch <- line:
    default:
        // Skip this client, continue
    }
}
```

---

### Anti-Pattern 2: Unbounded Log Storage

**What people do:** Append logs to slice without limit, causing memory exhaustion

**Why it's wrong:** Long-running processes accumulate GB of logs, OOM crash

**Do this instead:** Use ring buffer with fixed size

```go
// BAD
lb.lines = append(lb.lines, line)  // Grows unbounded

// GOOD
lb.lines[lb.head] = line
lb.head = (lb.head + 1) % lb.maxSize
```

---

### Anti-Pattern 3: Goroutine Leak in SSE Handler

**What people do:** Start goroutines without tying to context cancellation

**Why it's wrong:** Client disconnects, but goroutines keep running, memory leak

**Do this instead:** Always select on `ctx.Done()` in goroutines

```go
// BAD
go func() {
    for {
        time.Sleep(30 * time.Second)
        fmt.Fprint(w, ": ping\n\n")
    }
}()

// GOOD
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return  // Exit on client disconnect
        case <-ticker.C:
            fmt.Fprint(w, ": ping\n\n")
        }
    }
}()
```

---

### Anti-Pattern 4: Global Log Buffer Map

**What people do:** Use global `map[string]*LogBuffer` without synchronization

**Why it's wrong:** Race conditions, hard to test, implicit dependencies

**Do this instead:** Pass `LogBufferManager` explicitly as dependency

```go
// BAD
var globalBuffers = make(map[string]*LogBuffer)

// GOOD
type InstanceManager struct {
    logBufferManager *logbuffer.Manager  // Explicit dependency
}
```

---

### Anti-Pattern 5: Reading Pipes Without Goroutines

**What people do:** Sequentially read stdout then stderr without goroutines

**Why it's wrong:** If stdout blocks, stderr fills pipe buffer → deadlock

**Do this instead:** Read both streams concurrently with goroutines

```go
// BAD
stdout, _ := cmd.StdoutPipe()
stderr, _ := cmd.StderrPipe()
io.ReadAll(stdout)  // Blocks
io.ReadAll(stderr)  // Never reached

// GOOD
go streamToBuffer(stdoutPipe, buffer, "stdout")
go streamToBuffer(stderrPipe, buffer, "stderr")
```

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| **1-10 instances** | Current design optimal - single process, in-memory buffers |
| **10-50 instances** | Consider reducing buffer size (1000 lines), add compression for history API |
| **50+ instances** | Consider external log aggregation (Loki, Elasticsearch) |

### Scaling Priorities

1. **First bottleneck:** Memory usage (5000 lines × N instances)
   - **Solution:** Reduce buffer size or add size-based eviction (e.g., 1MB max per buffer)

2. **Second bottleneck:** SSE connections (file descriptor limits)
   - **Solution:** Increase `ulimit -n`, add connection timeout (disconnect idle clients)

**Note:** Current design optimized for 1-10 instances (typical personal usage). Horizontal scaling requires external log storage.

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| Nanobot processes | `cmd.StdoutPipe()` + goroutines | Non-blocking reads, graceful degradation |
| HTTP Clients (SSE) | Standard `EventSource` API | Browser auto-reconnects with Last-Event-ID |
| Monitoring (future) | History API endpoint | Poll `/logs/:name/history` for metrics |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| Starter → LogBuffer | `io.Writer` interface | Decoupled, testable |
| LogBuffer → SSE Handler | Channels (broadcast) | Thread-safe, non-blocking |
| InstanceManager → API Server | `GetLogBufferManager()` | Explicit dependency injection |

## Testing Strategy

### Unit Tests
- **LogBuffer:** Ring buffer edge cases, concurrency, broadcasting
- **LogBufferManager:** Multi-instance isolation, GetBuffer creation
- **Starter:** Mock `exec.Cmd`, verify pipes captured
- **SSE Handler:** Mock buffers, verify event format, heartbeat

### Integration Tests
- Start real process with log capture → verify logs appear
- Multiple SSE clients → verify all receive same stream
- Slow client → verify non-blocking behavior

### End-to-End Tests
- Full application with API + instances
- Connect SSE client via `EventSource` → verify real-time logs
- Request history → verify JSON response
- Client disconnect → verify no goroutine leak (use `runtime.NumGoroutine()`)

## Sources

### Official Documentation
- [Go os/exec package](https://pkg.go.dev/os/exec) - StdoutPipe/StderrPipe usage
- [Go net/http package](https://pkg.go.dev/net/http) - SSE with Flusher interface
- [MDN Server-Sent Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events) - SSE protocol specification

### Architecture Patterns
- [How to Build Real-time Applications with Go and SSE](https://oneuptime.com/blog/post/2026-02-01-go-realtime-applications-sse/view) - SSE broker pattern, heartbeat, reconnection (HIGH confidence)
- [smallnest/ringbuffer](https://github.com/smallnest/ringbuffer) - Thread-safe ring buffer implementation in Go (HIGH confidence)
- [Capture stdout from command exec in real time](https://stackoverflow.com/questions/48353768) - Real-time pipe reading with goroutines (HIGH confidence)

### Best Practices
- [Go Channel Patterns](https://oneuptime.com/blog/post/2026-01-23-go-channel-patterns/view) - Non-blocking select, channel buffering (HIGH confidence)
- [Go errgroup for Goroutine Coordination](https://oneuptime.com/blog/post/2026-01-07-go-errgroup/view) - Context cancellation patterns (HIGH confidence)

---
*Architecture research for: Real-time log viewing integration*
*Researched: 2026-03-16*

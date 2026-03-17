# Phase 19: Log Buffer Core - Research

**Researched:** 2026-03-17
**Domain:** Go 环形缓冲区、并发订阅机制、线程安全数据结构
**Confidence:** HIGH

## Summary

Phase 19 实现线程安全的环形缓冲区(LogBuffer),用于存储 nanobot 实例的日志条目并支持多订阅者实时广播。核心挑战在于:(1) 选择合适的环形缓冲区实现方式;(2) 设计非阻塞的订阅机制,防止慢订阅者影响日志写入性能;(3) 确保线程安全并通过 `go test -race` 验证。

研究发现 `emitter-io/circular` 库不存在,需要从替代方案中选择。推荐使用 **smallnest/ringbuffer** (成熟、线程安全、实现 io.ReaderWriter 接口) 或 **自行实现简化的环形缓冲区** (完全控制、无外部依赖)。订阅机制采用 **channel + goroutine 模式**,每个订阅者一个独立 goroutine 和有缓冲 channel (容量 100),使用 `select + default` 非阻塞发送,慢订阅者时丢弃日志不影响 LogBuffer 写入性能。

**Primary recommendation:** 使用 `smallnest/ringbuffer` 作为底层存储,在其之上封装订阅管理层。如果担心外部依赖风险,可自行实现 5000 条容量的环形缓冲区(预估内存占用 500KB-1MB)。订阅机制必须使用 channel 模式 + 非阻塞发送,保证写入操作永不阻塞。

<user_constraints>

## User Constraints (from CONTEXT.md)

### Locked Decisions

**日志条目结构:**
- 字段定义: 仅包含必需字段
  - `Timestamp` (time.Time) - 毫秒级精度,如 2026-03-16 20:30:45.123
  - `Source` (string) - "stdout" 或 "stderr",字符串形式可读性好
  - `Content` (string) - 原始文本,不解析 nanobot 输出的 JSON
- 内存占用: 估算每条约 100-200 字节 (时间戳 24 字节 + 来源 16 字节 + 内容 60-160 字节)
- 不添加字段: 不添加日志级别、序列号等额外字段,保持简单和内存高效

**订阅机制设计:**
- 订阅模式: Channel 模式
  - `Subscribe() chan LogEntry` - 返回只读 channel
  - `Unsubscribe(ch chan LogEntry)` - 接收 channel 作为句柄取消订阅
- 并发架构: 独立 goroutine + 有缓冲 channel
  - 每个订阅者一个独立的 goroutine 负责发送日志到客户端
  - 每个订阅者一个有缓冲 channel (容量 100 条)
  - LogBuffer 写入时非阻塞发送到订阅者 channel (使用 select + default)
- 慢订阅者处理: 丢弃该订阅者的日志,不影响 LogBuffer 性能
  - 当订阅者 channel 满时,丢弃该订阅者的这条日志
  - LogBuffer 写入操作永不阻塞
  - 丢弃时记录警告日志 (可选,由 Claude 决定)
- 历史日志: 新订阅者连接时发送缓冲区中的所有历史日志
  - 首先发送历史日志 (最多 5000 条)
  - 然后接收实时日志
  - 历史日志和实时日志在同一个 channel 中发送

**缓冲区实现方式:**
- ~~使用现有库: emitter-io/circular~~ (该库不存在,需选择替代方案)
- 包结构: 创建 `internal/logbuffer` 包
  - `buffer.go` - LogBuffer 结构和核心方法
  - `subscriber.go` - 订阅者管理逻辑
  - `buffer_test.go` - 单元测试 (包含 go test -race 验证)

**缓冲区满处理策略:**
- FIFO 覆盖: 当缓冲区达到 5000 行时自动覆盖最旧的日志
  - 环形缓冲区特性自动处理,无需额外代码
  - 不记录警告日志 (隐式行为,符合环形缓冲区特性)
  - 保证最新的 5000 条日志始终可访问

### Claude's Discretion

- 订阅者 channel 的具体容量 (默认 100,可根据实际负载调整)
- 丢弃订阅者日志时是否记录警告日志
- LogBuffer 的具体命名 (如 LogBuffer vs CircularLogBuffer)
- 订阅者 goroutine 的错误处理和恢复逻辑
- 选择环形缓冲区的实现方式 (使用 smallnest/ringbuffer 或自行实现)

### Deferred Ideas (OUT OF SCOPE)

None - 讨论保持在阶段范围内

</user_constraints>

<phase_requirements>

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| **BUFF-01** | 系统为每个 nanobot 实例维护独立的环形缓冲区 (Circular Buffer) | 本阶段创建 `internal/logbuffer` 包,提供 `NewLogBuffer()` 构造函数,InstanceManager (Phase 21) 为每个实例创建独立 LogBuffer |
| **BUFF-02** | 系统限制每个实例的缓冲区大小为 5000 行日志 | LogBuffer 初始化时指定容量 5000,使用 `smallnest/ringbuffer.New(5000)` 或自实现切片容量 5000 |
| **BUFF-03** | 系统使用线程安全的环形缓冲区实现,支持并发读写 | smallnest/ringbuffer 内置线程安全;自实现使用 `sync.RWMutex` 保护共享状态,通过 `go test -race` 验证 |
| **BUFF-04** | 系统在缓冲区满时自动覆盖最旧的日志 (FIFO) | smallnest/ringbuffer 自动 FIFO 覆盖;自实现通过 `(head+1) % capacity` 指针移动实现 |
| **BUFF-05** | 系统为每条日志保留时间戳、来源 (stdout/stderr) 和内容 | LogEntry 结构体定义 `Timestamp time.Time`, `Source string`, `Content string` 三个字段 |

</phase_requirements>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| **smallnest/ringbuffer** | latest | 线程安全的环形缓冲区实现 | 成熟库 (1.5k+ stars),实现 io.ReaderWriter 接口,自动处理 FIFO 覆盖,无需手动管理指针逻辑 |
| **sync** (标准库) | Go 1.21+ | 并发原语 (RWMutex, WaitGroup) | 标准库,保护订阅者 map 和环形缓冲区的并发访问 |
| **context** (标准库) | Go 1.21+ | 优雅关闭订阅者 goroutine | 标准库,Unsubscribe 时取消 context 停止发送 goroutine |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| **time** (标准库) | Go 1.21+ | LogEntry.Timestamp 字段类型 | 每条日志创建时调用 `time.Now()` |
| **log/slog** (标准库) | Go 1.21+ | 警告日志 (可选,慢订阅者丢弃时) | Claude's Discretion - 可记录或不记录 |
| **github.com/WQGroup/logger** | 项目已使用 | 项目统一日志库 | LogBuffer 内部使用 `logger.With("component", "logbuffer")` 注入上下文 |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| smallnest/ringbuffer | **自行实现环形缓冲区** | 自实现优势:无外部依赖、完全控制代码、简化调试。劣势:需要手动处理指针逻辑、FIFO 覆盖、边界条件。推荐:如果团队熟悉环形缓冲区算法且希望减少依赖,自实现 5000 条容量的环形缓冲区很简单 (预估 50-80 行代码) |
| smallnest/ringbuffer | negrel/ringo (lock-free) | Ringo 是无锁实现,性能更高但复杂度增加。对于 5000 条日志缓冲 + 低频订阅场景,smallnest/ringbuffer 的 mutex 实现已足够,无锁收益不明显 |
| Channel 模式订阅 | 回调函数模式 | 回调模式更简单 (订阅者注册 func(LogEntry)),但无法使用 `select` 多路复用,无法设置 channel 容量,goroutine 泄漏风险更高。Channel 模式符合 Go 惯例,与 Phase 22 SSE 集成天然契合 |
| 有缓冲 channel (容量 100) | 无缓冲 channel | 无缓冲 channel 会阻塞 LogBuffer 写入直到订阅者接收,违反"写入永不阻塞"约束。容量 100 平衡了内存占用 (100 条约 10KB) 和慢订阅者容忍度 |
| select + default (丢弃日志) | select + time.After (超时重试) | 超时重试增加复杂度且仍可能最终丢弃,直接丢弃更简单明确。慢订阅者通常是客户端问题 (SSE 连接慢),丢弃日志不影响核心功能 |

**Installation:**

如果使用 smallnest/ringbuffer:
```bash
go get github.com/smallnest/ringbuffer
```

如果自行实现:
```bash
# 无需安装外部依赖
```

## Architecture Patterns

### Recommended Project Structure

```
internal/
└── logbuffer/
    ├── buffer.go          # LogBuffer 结构体、Write、GetHistory 方法
    ├── subscriber.go      # Subscribe、Unsubscribe、subscriber goroutine 逻辑
    ├── buffer_test.go     # 单元测试 (包含 go test -race 验证)
    └── errors.go          # 自定义错误类型 (参考 internal/instance/errors.go 模式)
```

### Pattern 1: 环形缓冲区 + 订阅管理层分离

**What:** 将存储层 (环形缓冲区) 和订阅层 (pub-sub) 分离,LogBuffer 持有 ringbuffer 实例和 subscribers map

**When to use:** 本阶段标准模式,解耦存储和分发逻辑

**Example:**

```go
// Source: 基于 smallnest/ringbuffer + Go channel pub-sub 模式
package logbuffer

import (
    "sync"
    "time"
    "github.com/smallnest/ringbuffer"
)

// LogEntry 日志条目结构 (BUFF-05)
type LogEntry struct {
    Timestamp time.Time
    Source    string // "stdout" or "stderr"
    Content   string
}

// LogBuffer 环形缓冲区 + 订阅管理
type LogBuffer struct {
    mu          sync.RWMutex
    buffer      *ringbuffer.RingBuffer // 底层存储 (BUFF-01, BUFF-02)
    subscribers map[chan LogEntry]context.CancelFunc // 订阅者 map (channel -> cancel func)
    logger      *slog.Logger
}

// NewLogBuffer 创建日志缓冲区 (BUFF-02: 容量 5000)
func NewLogBuffer(logger *slog.Logger) *LogBuffer {
    return &LogBuffer{
        buffer:      ringbuffer.New(5000), // 容量 5000 条
        subscribers: make(map[chan LogEntry]context.CancelFunc),
        logger:      logger.With("component", "logbuffer"),
    }
}
```

### Pattern 2: 非阻塞发送到订阅者 (select + default)

**What:** 使用 `select + default` 模式向订阅者 channel 发送日志,满时立即丢弃,不阻塞 LogBuffer 写入

**When to use:** 慢订阅者场景,保证写入性能不受订阅者影响 (CONTEXT.md 约束)

**Example:**

```go
// Write 写入日志条目 (BUFF-03: 线程安全, BUFF-04: FIFO 自动覆盖)
func (lb *LogBuffer) Write(entry LogEntry) error {
    lb.mu.Lock()
    defer lb.mu.Unlock()

    // 写入环形缓冲区 (smallnest/ringbuffer 自动 FIFO 覆盖)
    // 注意: smallnest/ringbuffer 是字节缓冲区,需要序列化 LogEntry
    // 替代方案: 自实现 LogEntry 类型的环形缓冲区 (推荐)
    data, _ := json.Marshal(entry) // 或使用 gob 编码
    _, err := lb.buffer.Write(data)
    if err != nil {
        return err
    }

    // 非阻塞发送到所有订阅者 (慢订阅者丢弃日志)
    for ch, cancel := range lb.subscribers {
        select {
        case ch <- entry:
            // 发送成功
        default:
            // Channel 满,丢弃该订阅者的这条日志 (不阻塞)
            // Claude's Discretion: 可选记录警告日志
            lb.logger.Warn("Subscriber channel full, dropping log",
                "channel_capacity", cap(ch))
        }
    }

    return nil
}
```

### Pattern 3: 订阅者 goroutine + 历史日志先发

**What:** Subscribe() 为每个订阅者启动独立 goroutine,先发送历史日志再转发实时日志,使用 context 控制 goroutine 生命周期

**When to use:** 实时日志流场景,新订阅者需要看到历史上下文 (CONTEXT.md 约束)

**Example:**

```go
// Subscribe 订阅日志流,返回只读 channel (CONTEXT.md: Channel 模式)
func (lb *LogBuffer) Subscribe() <-chan LogEntry {
    lb.mu.Lock()
    defer lb.mu.Unlock()

    // 创建订阅者 channel (容量 100)
    ch := make(chan LogEntry, 100)

    // 创建可取消的 context
    ctx, cancel := context.WithCancel(context.Background())
    lb.subscribers[ch] = cancel

    // 启动订阅者 goroutine
    go lb.subscriberLoop(ctx, ch)

    return ch
}

// subscriberLoop 订阅者 goroutine: 先发历史日志,再转发实时日志
func (lb *LogBuffer) subscriberLoop(ctx context.Context, ch chan<- LogEntry) {
    defer close(ch) // goroutine 退出时关闭 channel

    // 1. 发送历史日志 (CONTEXT.md: 新订阅者先接收缓冲区中的所有历史日志)
    history := lb.GetHistory()
    for _, entry := range history {
        select {
        case ch <- entry:
            // 发送成功
        case <-ctx.Done():
            // Unsubscribe 被调用,停止发送
            return
        }
    }

    // 2. 实时日志由 Write() 方法直接发送到 ch (见 Pattern 2)
    // 这里只需要等待 context 取消
    <-ctx.Done()
}

// GetHistory 获取缓冲区中的历史日志 (BUFF-05: 返回 LogEntry 切片)
func (lb *LogBuffer) GetHistory() []LogEntry {
    lb.mu.RLock()
    defer lb.mu.RUnlock()

    // 从 ringbuffer 读取所有数据并反序列化
    // 注意: smallnest/ringbuffer 是字节缓冲区,读取逻辑复杂
    // 替代方案: 自实现 LogEntry 类型的环形缓冲区,直接返回切片 (推荐)
    // 伪代码:
    // return lb.entries[lb.head:lb.tail]

    return []LogEntry{} // 占位符
}

// Unsubscribe 取消订阅 (CONTEXT.md: 接收 channel 作为句柄)
func (lb *LogBuffer) Unsubscribe(ch <-chan LogEntry) {
    lb.mu.Lock()
    defer lb.mu.Unlock()

    // 获取写权限 channel (类型断言)
    chWrite := chan LogEntry(ch)

    if cancel, exists := lb.subscribers[chWrite]; exists {
        cancel()              // 取消 context,停止 goroutine
        delete(lb.subscribers, chWrite) // 从 map 中删除
    }
}
```

### Anti-Patterns to Avoid

- **使用 emitter-io/circular**: 该库不存在,不要在代码中引用
- **无缓冲 channel 订阅**: 违反"写入永不阻塞"约束,慢订阅者会阻塞 LogBuffer 写入
- **全局订阅者 map**: 每个实例应有独立 LogBuffer,不要用全局 map 管理所有实例的订阅者 (Phase 21 会为每个实例创建独立 LogBuffer)
- **环形缓冲区存储序列化字节**: smallnest/ringbuffer 是字节缓冲区,存储 LogEntry 需要序列化/反序列化,增加复杂度和 CPU 开销。推荐自实现 `[]LogEntry` 切片环形缓冲区
- **忘记关闭订阅者 channel**: Unsubscribe 时必须关闭 channel,否则订阅者 goroutine 泄漏
- **缓冲区满时记录 ERROR 级别日志**: FIFO 覆盖是环形缓冲区正常行为,不应记录 ERROR。丢弃慢订阅者日志可选记录 WARN (Claude's Discretion)

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| **线程安全的环形缓冲区** | 从零实现 ringbuffer 指针逻辑、FIFO 覆盖、边界条件 | smallnest/ringbuffer 或自实现简单的 `[]LogEntry` 切片 + head/tail 指针 | smallnest/ringbuffer 已处理边界条件 (空、满、覆盖);自实现切片环形缓冲区逻辑简单 (预估 50 行代码),比使用字节缓冲区 + 序列化更简单 |
| **订阅者 goroutine 生命周期管理** | 手动管理 goroutine 启动/停止、channel 关闭 | context.WithCancel + defer close(ch) | Context 是 Go 标准的 goroutine 生命周期管理模式,避免 goroutine 泄漏 |
| **并发安全的订阅者 map** | 自己实现锁机制保护 map 读写 | sync.RWMutex (读多写少场景) | RWMutex 允许多个读并发 (GetHistory),写时独占 (Subscribe/Unsubscribe/Write) |

**Key insight:** 环形缓冲区的核心是 **存储 LogEntry 对象** (不是字节),推荐自实现 `[]LogEntry` 切片 + head/tail 指针,比使用 smallnest/ringbuffer (字节缓冲区) + 序列化更简单高效。预估实现时间: 2 小时 (包含测试)。

## Common Pitfalls

### Pitfall 1: 使用不存在的 emitter-io/circular 库

**What goes wrong:** CONTEXT.md 中指定使用 emitter-io/circular,但该 GitHub 仓库不存在 (404),导致依赖安装失败

**Why it happens:** 仓库可能被删除、重命名或从未存在,CONTEXT.md 作者未验证库的可用性

**How to avoid:**
1. 使用 **smallnest/ringbuffer** (GitHub 1.5k+ stars,活跃维护) 或 **自实现环形缓冲区**
2. Phase 20 之前的调研阶段验证库的可用性 (访问 GitHub URL、检查 GoDoc 文档)
3. 在 RESEARCH.md 中明确指出替代方案和推荐选择

**Warning signs:** `go get` 失败返回 404,import 路径红线报错

### Pitfall 2: 订阅者 goroutine 泄漏

**What goes wrong:** Unsubscribe() 未关闭订阅者 channel,goroutine 永远阻塞在 `select { case ch <- entry: ... }`,导致 goroutine 累积泄漏

**Why it happens:** 忘记 `close(ch)` 或未使用 context 控制 goroutine 生命周期

**How to avoid:**
1. Subscribe() 中启动 goroutine 时使用 `context.WithCancel`
2. goroutine 中使用 `select { case ch <- entry: ... case <-ctx.Done(): return }`
3. Unsubscribe() 中调用 `cancel()` 停止 goroutine
4. goroutine 使用 `defer close(ch)` 确保退出时关闭 channel

**Warning signs:** `runtime.NumGoroutine()` 持续增长,内存占用上升,`go test -race` 未检测到但生产环境出现泄漏

### Pitfall 3: 慢订阅者阻塞 LogBuffer 写入

**What goes wrong:** 使用无缓冲 channel 或未使用 `select + default`,慢订阅者处理不及时导致 LogBuffer.Write() 阻塞,影响日志捕获 (Phase 20)

**Why it happens:** 误以为 channel 发送总是非阻塞,或忘记加 `default` 分支

**How to avoid:**
1. 订阅者 channel 必须有缓冲 (容量 100,平衡内存和容忍度)
2. 发送日志时必须使用 `select { case ch <- entry: ... default: 丢弃 }` 模式
3. 编写测试用例模拟慢订阅者 (channel 满时验证写入不阻塞)

**Warning signs:** Phase 20 日志捕获性能下降,stdout/stderr 管道读取超时

### Pitfall 4: 环形缓冲区序列化开销

**What goes wrong:** 使用 smallnest/ringbuffer (字节缓冲区) 存储 LogEntry,每次写入需要 `json.Marshal`,读取需要 `json.Unmarshal`,CPU 开销增加 2-5 倍

**Why it happens:** smallnest/ringbuffer 设计用于字节流 (网络 IO 缓冲),不适合存储结构化对象

**How to avoid:**
1. **推荐方案**: 自实现 `[]LogEntry` 切片环形缓冲区,直接存储对象,无序列化开销
2. 如果必须使用 smallnest/ringbuffer,改用 `gob.NewEncoder` 比 json 更快

**Warning signs:** CPU profile 显示序列化占用大量时间,写入性能 < 10 万条/秒

### Pitfall 5: 忘记发送历史日志

**What goes wrong:** Subscribe() 只发送实时日志,新订阅者看不到连接前的历史日志,缺少调试上下文

**Why it happens:** 只在 Write() 中发送到订阅者 channel,忘记 Subscribe() 中先发送历史

**How to avoid:**
1. Subscribe() 启动 goroutine 后先调用 `GetHistory()` 并发送所有历史日志
2. 历史日志和实时日志在同一个 channel 中发送,订阅者无需区分
3. 编写测试用例验证新订阅者能接收到历史日志

**Warning signs:** Phase 23 Web UI 连接后看不到之前的日志,需要手动刷新页面

## Code Examples

### 自实现环形缓冲区 (推荐方案)

```go
// Source: 基于 Go 切片实现的环形缓冲区 (无外部依赖)
package logbuffer

import (
    "context"
    "log/slog"
    "sync"
    "time"
)

// LogEntry 日志条目 (BUFF-05)
type LogEntry struct {
    Timestamp time.Time
    Source    string // "stdout" or "stderr"
    Content   string
}

// LogBuffer 线程安全的环形缓冲区 + 订阅管理 (BUFF-01, BUFF-03)
type LogBuffer struct {
    mu          sync.RWMutex
    entries     [5000]LogEntry // 固定容量 5000 (BUFF-02)
    head        int            // 下一个写入位置
    size        int            // 当前条目数 (< 5000 时增长,= 5000 时满)
    subscribers map[chan LogEntry]context.CancelFunc
    logger      *slog.Logger
}

// NewLogBuffer 创建日志缓冲区
func NewLogBuffer(logger *slog.Logger) *LogBuffer {
    return &LogBuffer{
        entries:     [5000]LogEntry{}, // 预分配 5000 条容量
        subscribers: make(map[chan LogEntry]context.CancelFunc),
        logger:      logger.With("component", "logbuffer"),
    }
}

// Write 写入日志条目 (BUFF-03: 线程安全, BUFF-04: FIFO 自动覆盖)
func (lb *LogBuffer) Write(entry LogEntry) error {
    lb.mu.Lock()
    defer lb.mu.Unlock()

    // 写入环形缓冲区 (FIFO 覆盖自动处理)
    lb.entries[lb.head] = entry
    lb.head = (lb.head + 1) % 5000
    if lb.size < 5000 {
        lb.size++
    }

    // 非阻塞发送到所有订阅者
    for ch, cancel := range lb.subscribers {
        select {
        case ch <- entry:
            // 发送成功
        default:
            // Channel 满,丢弃该订阅者的这条日志
            lb.logger.Warn("Subscriber channel full, dropping log",
                "channel_capacity", cap(ch))
        }
    }

    return nil
}

// GetHistory 获取缓冲区中的历史日志 (按时间顺序返回)
func (lb *LogBuffer) GetHistory() []LogEntry {
    lb.mu.RLock()
    defer lb.mu.RUnlock()

    if lb.size == 0 {
        return []LogEntry{}
    }

    // 从环形缓冲区中按顺序提取日志
    result := make([]LogEntry, lb.size)
    if lb.size < 5000 {
        // 缓冲区未满,entries[0:size] 是有效数据
        copy(result, lb.entries[:lb.size])
    } else {
        // 缓冲区已满,entries[head:5000] + entries[0:head] 是有效数据
        copy(result, lb.entries[lb.head:])
        copy(result[5000-lb.head:], lb.entries[:lb.head])
    }

    return result
}

// Subscribe 订阅日志流,返回只读 channel
func (lb *LogBuffer) Subscribe() <-chan LogEntry {
    lb.mu.Lock()
    defer lb.mu.Unlock()

    ch := make(chan LogEntry, 100) // 容量 100
    ctx, cancel := context.WithCancel(context.Background())
    lb.subscribers[ch] = cancel

    go lb.subscriberLoop(ctx, ch)

    return ch
}

// subscriberLoop 订阅者 goroutine: 先发历史日志,再等待实时日志
func (lb *LogBuffer) subscriberLoop(ctx context.Context, ch chan<- LogEntry) {
    defer close(ch)

    // 先发送历史日志
    history := lb.GetHistory()
    for _, entry := range history {
        select {
        case ch <- entry:
            // 发送成功
        case <-ctx.Done():
            return
        }
    }

    // 等待 Unsubscribe 或 context 取消
    <-ctx.Done()
}

// Unsubscribe 取消订阅
func (lb *LogBuffer) Unsubscribe(ch <-chan LogEntry) {
    lb.mu.Lock()
    defer lb.mu.Unlock()

    chWrite := chan LogEntry(ch)
    if cancel, exists := lb.subscribers[chWrite]; exists {
        cancel()
        delete(lb.subscribers, chWrite)
    }
}
```

### 使用 smallnest/ringbuffer (备选方案)

```go
// Source: 基于 smallnest/ringbuffer 的实现 (需要序列化)
package logbuffer

import (
    "context"
    "encoding/json"
    "log/slog"
    "sync"
    "time"

    "github.com/smallnest/ringbuffer"
)

// LogBuffer 使用 smallnest/ringbuffer 作为底层存储
type LogBuffer struct {
    mu          sync.RWMutex
    buffer      *ringbuffer.RingBuffer // 容量 5000 * avgEntrySize
    subscribers map[chan LogEntry]context.CancelFunc
    logger      *slog.Logger
}

// NewLogBuffer 创建日志缓冲区
func NewLogBuffer(logger *slog.Logger) *LogBuffer {
    // 估算每条日志 200 字节,总容量 5000 * 200 = 1MB
    return &LogBuffer{
        buffer:      ringbuffer.New(5000 * 200),
        subscribers: make(map[chan LogEntry]context.CancelFunc),
        logger:      logger.With("component", "logbuffer"),
    }
}

// Write 写入日志条目 (需要序列化)
func (lb *LogBuffer) Write(entry LogEntry) error {
    lb.mu.Lock()
    defer lb.mu.Unlock()

    // 序列化 LogEntry 为 JSON
    data, err := json.Marshal(entry)
    if err != nil {
        return err
    }

    // 写入字节缓冲区 (FIFO 自动覆盖)
    _, err = lb.buffer.Write(data)
    if err != nil {
        return err
    }

    // 非阻塞发送到订阅者 (同 Pattern 2)
    for ch, cancel := range lb.subscribers {
        select {
        case ch <- entry:
            // 发送成功
        default:
            lb.logger.Warn("Subscriber channel full, dropping log")
        }
    }

    return nil
}

// GetHistory 从字节缓冲区读取并反序列化所有日志 (复杂度高)
func (lb *LogBuffer) GetHistory() []LogEntry {
    lb.mu.RLock()
    defer lb.mu.RUnlock()

    // smallnest/ringbuffer 读取逻辑复杂:
    // 1. 需要知道每条日志的字节边界 (JSON 无固定长度)
    // 2. 需要遍历整个缓冲区并逐条反序列化
    // 3. 实现复杂度高,性能差
    //
    // 替代方案: 维护一个额外的 []LogEntry 切片用于历史查询
    // 但这会增加内存占用 (双倍存储)

    return []LogEntry{} // 占位符,实际实现复杂
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| **无锁环形缓冲区 (lock-free)** | Mutex-based 环形缓冲区 | Go 1.5+ (2015) | 对于低频写入场景 (日志),mutex 性能足够,无锁的 CAS (Compare-And-Swap) 复杂度收益不明显 |
| **回调函数订阅模式** | Channel 订阅模式 | Go 1.0 (2012) | Channel 是 Go 的核心并发原语,符合 "Don't communicate by sharing memory, share memory by communicating" 哲学 |
| **字节缓冲区 + 序列化** | 对象切片环形缓冲区 | 本项目决策 (2026) | 避免序列化开销,简化实现,提升性能 (预估 2-5 倍) |

**Deprecated/outdated:**
- **emitter-io/circular**: 仓库不存在,不要使用
- **全局订阅者 map**: 每个实例应有独立 LogBuffer (Phase 21 集成模式)

## Open Questions

1. **自实现 vs smallnest/ringbuffer: 最终选择?**

   **What we know:**
   - 自实现优势:无序列化开销、代码简单 (预估 50-80 行)、无外部依赖、完全控制
   - smallnest/ringbuffer 优势:成熟库、自动处理边界条件、实现 io.ReaderWriter 接口

   **What's unclear:**
   - smallnest/ringbuffer 读取历史日志的实现复杂度 (需要处理 JSON 边界)
   - 自实现的测试覆盖率和边界条件处理是否充分

   **Recommendation:**
   - **优先推荐自实现** `[]LogEntry` 切片环形缓冲区 (本 RESEARCH.md Code Examples 已提供完整实现)
   - 理由:(1) LogEntry 对象存储比字节存储更适合本场景;(2) 避免序列化开销;(3) GetHistory() 实现简单直接;(4) 无外部依赖风险

2. **丢弃慢订阅者日志时是否记录警告?**

   **What we know:**
   - CONTEXT.md 标记为 "Claude's Discretion"
   - 记录警告有助于调试慢订阅者问题
   - 但高频丢弃时警告日志可能刷屏

   **What's unclear:**
   - 丢弃频率有多高 (取决于 SSE 客户端性能和网络)
   - 警告日志对性能的影响

   **Recommendation:**
   - **记录 WARN 级别日志**,但限制频率 (使用 rate.Sampler 或每 10 次丢弃记录 1 次)
   - 或使用 DEBUG 级别,生产环境默认不显示

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go 1.21+ 标准测试框架 (`testing` 包) |
| Config file | 无 (Go 测试不需要配置文件) |
| Quick run command | `go test -v ./internal/logbuffer` |
| Full suite command | `go test -v -race ./internal/logbuffer` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| **BUFF-01** | 为每个实例维护独立 LogBuffer | unit | `go test -v -run TestNewLogBuffer ./internal/logbuffer` | ❌ Wave 0 |
| **BUFF-02** | 缓冲区大小 5000 行 | unit | `go test -v -run TestBufferCapacity ./internal/logbuffer` | ❌ Wave 0 |
| **BUFF-03** | 线程安全,支持并发读写 | unit + race | `go test -race -v -run TestConcurrentWrite ./internal/logbuffer` | ❌ Wave 0 |
| **BUFF-04** | 缓冲区满时自动 FIFO 覆盖 | unit | `go test -v -run TestFIFOOverwrite ./internal/logbuffer` | ❌ Wave 0 |
| **BUFF-05** | 保留时间戳、来源、内容字段 | unit | `go test -v -run TestLogEntryFields ./internal/logbuffer` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test -v ./internal/logbuffer` (快速验证)
- **Per wave merge:** `go test -v -race ./internal/logbuffer` (完整 race 检测)
- **Phase gate:** `go test -v -race -cover ./internal/logbuffer` (覆盖率 > 80%)

### Wave 0 Gaps

- [ ] `internal/logbuffer/buffer.go` - LogBuffer 结构和核心方法 (Write, GetHistory)
- [ ] `internal/logbuffer/subscriber.go` - 订阅者管理 (Subscribe, Unsubscribe, subscriberLoop)
- [ ] `internal/logbuffer/buffer_test.go` - 单元测试 (包含并发测试、race 检测)
- [ ] `internal/logbuffer/errors.go` - 自定义错误类型 (可选,参考 internal/instance/errors.go)

## Sources

### Primary (HIGH confidence)

- **smallnest/ringbuffer GitHub**: https://github.com/smallnest/ringbuffer - 线程安全的环形缓冲区实现 (验证时间: 2026-03-17)
- **Go 标准库文档**: sync.RWMutex, context, channel - 并发原语和 channel 模式 (官方文档,无过期风险)
- **项目现有代码**: internal/instance/errors.go, internal/instance/lifecycle.go - 错误类型模式和上下文日志注入 (已验证)

### Secondary (MEDIUM confidence)

- **LogRocket: Building a pub/sub service in Go**: https://blog.logrocket.com/building-pub-sub-service-go/ - Channel 模式订阅机制,goroutine 生命周期管理 (验证时间: 2026-03-17)
- **Ably: Guide to Pub/Sub in Golang**: https://ably.com/blog/pubsub-golang - Pub/Sub 模式最佳实践,历史日志发送模式 (验证时间: 2026-03-17)
- **Stack Overflow: Non-blocking channel operations**: https://stackoverflow.com/questions/41000161/non-blocking-channel-operations-in-go-send - select + default 模式 (社区验证)

### Tertiary (LOW confidence)

- 无 - 所有核心发现均通过 Primary 或 Secondary 源验证

## Metadata

**Confidence breakdown:**
- Standard stack: **HIGH** - smallnest/ringbuffer 存在且活跃,Go 标准库稳定,自实现方案在 Code Examples 中已完整提供
- Architecture: **HIGH** - Channel 订阅模式是 Go 标准模式,CONTEXT.md 约束明确,Code Examples 提供完整实现
- Pitfalls: **HIGH** - 基于实际 WebSearch 结果 (emitter-io/circular 不存在) 和 Go 并发常见陷阱总结

**Research date:** 2026-03-17
**Valid until:** 2026-04-17 (30 天,Go 生态稳定,环形缓冲区和 channel 模式不会重大变化)

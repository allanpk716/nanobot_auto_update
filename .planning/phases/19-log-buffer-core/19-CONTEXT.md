# Phase 19: Log Buffer Core - Context

**Gathered:** 2026-03-17
**Status:** Ready for planning

<domain>
## Phase Boundary

实现线程安全的环形缓冲区基础设施,支持日志存储和实时广播。这是纯基础设施层,不涉及进程日志捕获(Phase 20)、实例管理集成(Phase 21)、SSE API(Phase 22)和 Web UI(Phase 23)。LogBuffer 提供独立的包,可供其他组件调用。

</domain>

<decisions>
## Implementation Decisions

### 日志条目结构
- **字段定义**: 仅包含必需字段
  - `Timestamp` (time.Time) - 毫秒级精度,如 2026-03-16 20:30:45.123
  - `Source` (string) - "stdout" 或 "stderr",字符串形式可读性好
  - `Content` (string) - 原始文本,不解析 nanobot 输出的 JSON
- **内存占用**: 估算每条约 100-200 字节 (时间戳 24 字节 + 来源 16 字节 + 内容 60-160 字节)
- **结构示例**:
  ```go
  type LogEntry struct {
      Timestamp time.Time
      Source    string // "stdout" or "stderr"
      Content   string
  }
  ```
- **不添加字段**: 不添加日志级别、序列号等额外字段,保持简单和内存高效

### 订阅机制设计
- **订阅模式**: Channel 模式
  - `Subscribe() chan LogEntry` - 返回只读 channel
  - `Unsubscribe(ch chan LogEntry)` - 接收 channel 作为句柄取消订阅
- **并发架构**: 独立 goroutine + 有缓冲 channel
  - 每个订阅者一个独立的 goroutine 负责发送日志到客户端
  - 每个订阅者一个有缓冲 channel (容量 100 条)
  - LogBuffer 写入时非阻塞发送到订阅者 channel (使用 select + default)
- **慢订阅者处理**: 丢弃该订阅者的日志,不影响 LogBuffer 性能
  - 当订阅者 channel 满时,丢弃该订阅者的这条日志
  - LogBuffer 写入操作永不阻塞
  - 丢弃时记录警告日志 (可选,由 Claude 决定)
- **历史日志**: 新订阅者连接时发送缓冲区中的所有历史日志
  - 首先发送历史日志 (最多 5000 条)
  - 然后接收实时日志
  - 历史日志和实时日志在同一个 channel 中发送

### 缓冲区实现方式
- **使用现有库**: emitter-io/circular
  - GitHub: https://github.com/emitter-io/circular
  - 成熟的线程安全环形缓冲区实现
  - 需要在其基础上添加订阅机制层
- **包结构**: 创建 `internal/logbuffer` 包
  - `buffer.go` - LogBuffer 结构和核心方法
  - `subscriber.go` - 订阅者管理逻辑
  - `buffer_test.go` - 单元测试 (包含 go test -race 验证)

### 缓冲区满处理策略
- **FIFO 覆盖**: 当缓冲区达到 5000 行时自动覆盖最旧的日志
  - circular 库自动处理,无需额外代码
  - 不记录警告日志 (隐式行为,符合环形缓冲区特性)
  - 保证最新的 5000 条日志始终可访问

### Claude's Discretion
- 订阅者 channel 的具体容量 (默认 100,可根据实际负载调整)
- 丢弃订阅者日志时是否记录警告日志
- LogBuffer 的具体命名 (如 LogBuffer vs CircularLogBuffer)
- 订阅者 goroutine 的错误处理和恢复逻辑

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 需求文档
- `.planning/REQUIREMENTS.md` §BUFF-01 to BUFF-05 - 日志缓冲需求定义 (5000 行容量、线程安全、FIFO 行为、日志字段)
- `.planning/ROADMAP.md` §Phase 19 - 阶段目标和成功标准

### 外部依赖
- `https://github.com/emitter-io/circular` - Go 环形缓冲区库,需要阅读其 README 和示例代码

### 相关上下文
- `.planning/phases/07-lifecycle-extension/07-CONTEXT.md` - 实例生命周期管理,Phase 21 将在此基础集成 LogBuffer
- `.planning/phases/08-instance-coordinator/08-CONTEXT.md` - 实例协调器,Phase 21 将扩展 InstanceManager 管理 LogBuffer

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **internal/instance/manager.go**: InstanceManager 管理所有实例,Phase 21 将在此添加 LogBuffer 管理
- **internal/instance/lifecycle.go**: InstanceLifecycle 提供单实例生命周期管理,Phase 20 将扩展以捕获日志
- **internal/logging/logging.go**: 自定义 slog handler,LogBuffer 可复用日志格式
- **internal/instance/errors.go**: 自定义错误类型模式,LogBuffer 可参考定义 BufferError

### Established Patterns
- **结构化错误**: InstanceError 包含实例名、操作、端口和底层错误,LogBuffer 可定义类似结构
- **上下文日志注入**: 使用 logger.With() 预注入上下文字段,LogBuffer 应注入 "component=logbuffer"
- **并发安全**: 所有公共方法使用互斥锁保护共享状态,通过 go test -race 验证
- **优雅降级**: 失败记录错误但不中断流程,LogBuffer 写入失败不影响进程捕获

### Integration Points
- **Phase 20 集成**: LogCapture 将调用 LogBuffer.Write() 写入日志
- **Phase 21 集成**: InstanceManager 将为每个实例创建独立的 LogBuffer
- **Phase 22 集成**: SSE handler 将调用 LogBuffer.Subscribe() 获取实时日志流
- **主程序集成**: v0.4 的 main.go 将初始化 LogBuffer 并传递给相关组件

</code_context>

<specifics>
## Specific Ideas

- **独立 goroutine + 有缓冲 channel** 解耦了 LogBuffer 写入和订阅者发送,即使订阅者慢也不会阻塞日志捕获
- **Channel 模式** 符合 Go 并发编程惯例,与后续 SSE 集成 (Phase 22) 天然契合
- **使用 emitter-io/circular** 节省开发时间,专注于订阅机制实现
- **FIFO 覆盖** 保证内存占用固定 (5000 条),无 OOM 风险
- **发送历史日志** 让新连接的客户端看到完整上下文,提升调试体验
- **仅必需字段** 保持内存高效,5000 条约 500KB-1MB 内存占用

</specifics>

<deferred>
## Deferred Ideas

None — 讨论保持在阶段范围内

</deferred>

---

*Phase: 19-log-buffer-core*
*Context gathered: 2026-03-17*

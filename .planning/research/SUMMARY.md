# Project Research Summary

**Project:** nanobot-auto-updater v0.4 实时日志查看
**Domain:** 实时日志流式传输 (SSE + 进程输出捕获 + 环形缓冲)
**Researched:** 2026-03-16
**Confidence:** HIGH

## Executive Summary

v0.4 里程碑为现有的 nanobot-auto-updater 应用添加实时日志查看功能,采用三层架构设计:日志捕获层 (修改进程启动逻辑)、缓冲层 (环形缓冲区)、流式传输层 (SSE)。该方案在 Go 生态中是成熟模式,被 Grafana、各种日志聚合工具广泛使用。

**推荐方案:** 使用 `github.com/smallnest/ringbuffer` 实现线程安全的环形缓冲区 (5000 行),通过 `os/exec.Cmd.StdoutPipe()` + goroutine 并发捕获进程输出,利用 `github.com/r3labs/sse/v2` 集成到现有 HTTP API 服务器。该方案简单、高效、资源占用可控 (每实例约 1MB 内存)。

**关键风险:** (1) 进程管道死锁 - 必须并发读取 stdout/stderr; (2) SSE 连接泄漏 - 必须监听 `r.Context().Done()` 事件; (3) HTTP WriteTimeout 破坏长连接 - 需为 SSE 端点禁用超时。所有风险都有明确的预防策略和验证方法。

## Key Findings

### Recommended Stack

v0.4 需要引入 2 个新依赖,复用现有 Go 标准库和 HTTP 服务器。

**Core technologies:**
- **`github.com/r3labs/sse/v2`** (v2.0.0+): SSE 服务器 — 成熟稳定 (2015 年起维护),与现有 `net/http` 无缝集成,自动处理重连和心跳
- **`github.com/smallnest/ringbuffer`** (最新): 环形缓冲区 — 线程安全、零分配、实现 `io.ReadWriter` 接口,避免手写并发逻辑
- **`os/exec` + `bufio.Scanner`** (Go 1.24.11+): 进程输出捕获 — 标准库方案,使用 `StdoutPipe()` + goroutine 并发读取

**Supporting libraries (无需新增):**
- `sync.RWMutex`: 保护共享状态 (ringbuffer 已内置)
- `context`: goroutine 生命周期控制
- `net/http`: 现有 API 服务器 (添加 `/api/v1/logs/{name}/stream` 端点)

**详细依赖列表和版本兼容性:** [STACK.md](.planning/research/STACK.md)

### Expected Features

基于 Grafana、journalctl、Logcat 等日志查看工具的用户期望研究。

**Must have (table stakes):**
- **Auto-scroll to latest logs** — 实时查看器必须显示最新条目,用户期望 "tail -f" 行为
- **Pause/Resume streaming** — 检查特定日志行时暂停自动滚动,所有成熟日志查看器必备
- **Instance selection** — 多实例环境必须支持选择查看哪个实例的日志
- **Basic text search/filter** — 查找特定日志条目是基本需求 (Ctrl+F 或搜索框)
- **Circular buffer (fixed memory)** — 5000 行环形缓冲,防止 OOM,标准做法
- **Real-time updates** — 日志必须立即显示,SSE 自带此能力
- **Clear stdout vs stderr distinction** — 颜色编码或前缀区分错误输出,调试关键

**Should have (competitive):**
- **Built-in Web UI** — 单二进制文件同时提供 API 和 UI,无需外部工具
- **Instance-specific buffers** — 每个实例独立日志历史,隔离日志流
- **Timestamp preservation** — 保留原始日志时间戳,不是接收时间
- **Connection status indicator** — SSE 连接状态视觉反馈 (活跃/重连/断开)
- **Log line highlighting** — 高亮错误或搜索匹配项

**Defer (v2+):**
- **Log persistence to disk** — 不是实时查看器的目标,现有文件日志已处理
- **Complex query language** — MVP 过度工程化,简单文本搜索足够
- **Multiple simultaneous log views** — 合并多实例日志会让关联性更难,先不做
- **Authentication/Authorization** — 依赖现有 API 认证或 localhost-only 绑定

**详细功能矩阵和优先级:** [FEATURES.md](.planning/research/FEATURES.md)

### Architecture Approach

三层架构集成到现有 `nanobot-auto-updater` 应用,遵循最小侵入原则。

**Major components:**
1. **LogBuffer (`internal/logbuffer/buffer.go`)** — 环形缓冲区 (5000 行) + 广播订阅者管理,单 goroutine 处理所有订阅避免竞态条件
2. **LogBufferManager (`internal/logbuffer/manager.go`)** — 管理所有实例的 LogBuffer,按名称查找,线程安全
3. **Modified Starter (`internal/lifecycle/starter.go`)** — 使用 `cmd.StdoutPipe()` + goroutine 捕获输出,向后兼容 (可选 logBuffer 参数)
4. **SSE Handler (`internal/api/log_handler.go`)** — HTTP 端点 `/api/v1/logs/{name}/stream`,订阅 LogBuffer,推送事件,30s 心跳
5. **History Handler (`internal/api/log_handler.go`)** — HTTP 端点 `/api/v1/logs/{name}/history`,返回 JSON 格式历史日志

**Data flow:**
```
[Nanobot Process] stdout/stderr
    ↓ [cmd.StdoutPipe()]
[bufio.Scanner] (goroutine)
    ↓ [LogBuffer.WriteLine()]
[Ring Buffer] (5000 lines)
    ↓ [Broadcast Channel]
[Subscriber Channels]
    ↓                    ↓
[History API]      [SSE Handler]
```

**详细架构图和代码示例:** [ARCHITECTURE.md](.planning/research/ARCHITECTURE.md)

### Critical Pitfalls

基于 Go 社区最佳实践和常见错误研究,前 5 个关键陷阱。

1. **SSE Connection Goroutine Leak** — 客户端断开连接时服务器端 goroutine 继续运行
   - **预防:** 在 SSE handler 的主循环中使用 `select { case <-r.Context().Done(): return; ... }`
   - **验证:** 连接 10 个 SSE 客户端,全部关闭,检查 `runtime.NumGoroutine()` 回到基线

2. **Process stdout/stderr Pipe Deadlock** — `cmd.Wait()` 永久挂起,即使进程已退出
   - **预防:** 使用 `sync.WaitGroup` + 2 个 goroutine 并发读取 stdout/stderr,在 `Wait()` 之前完成
   - **验证:** 捕获产生 10MB 输出的进程,检查 `cmd.Wait()` 在 5 秒内返回

3. **Ring Buffer Data Race** — 并发写入和读取导致数据损坏或崩溃
   - **预防:** 使用 `smallnest/ringbuffer` (已内置线程安全) 或手写时用 `sync.RWMutex` 保护所有操作
   - **验证:** `go test -race` 在 100 个 goroutine 并发写入/读取 10 秒

4. **HTTP WriteTimeout Breaks SSE Streaming** — SSE 连接在 5 分钟后断开 (配置的超时值)
   - **预防:** 设置 `http.Server{WriteTimeout: 0}` (全局禁用) 或 `IdleTimeout: 120s` (仅空闲连接)
   - **验证:** SSE 客户端连接 >10 分钟,验证连接保持活跃且有周期性数据

5. **Memory Leak from Unbounded Log Buffer** — 内存随时间无限增长
   - **预防:** 环形缓冲区固定大小 + 写入时复制字节切片 (`copied := make([]byte, len(line)); copy(copied, line)`)
   - **验证:** 运行 1 小时 (1000 行/秒),检查 `runtime.MemStats.HeapAlloc` 稳定

**详细陷阱分析、警告信号、恢复策略:** [PITFALLS.md](.planning/research/PITFALLS.md)

## Implications for Roadmap

基于研究,建议 6 个阶段的实施路线图,按依赖关系和技术风险排序。

### Phase 1: Log Buffer Core (基础设施层)
**Rationale:** LogBuffer 是所有后续工作的基础,独立于现有代码,可单独测试和验证
**Delivers:** 线程安全的环形缓冲区 + 广播机制
**Addresses:** MVP 必需功能 (Circular buffer, Real-time updates)
**Avoids:** Pitfall 3 (Ring Buffer Data Race), Pitfall 5 (Memory Leak)

**Implementation:**
- 创建 `internal/logbuffer/` 包
- 实现 `LogBuffer` (5000 行环形缓冲 + 订阅者广播)
- 实现 `LogBufferManager` (多实例管理)
- 单元测试:并发安全、环形缓冲边界、广播机制

**Validation:**
- 单元测试 100% 覆盖率
- 无竞态条件 (`go test -race`)
- 内存稳定 (长时间运行测试)

### Phase 2: Integrate Log Capture into Starter (进程输出捕获)
**Rationale:** 在 LogBuffer 可用后,修改进程启动逻辑捕获输出,Phase 1 的下游依赖
**Delivers:** 捕获 nanobot 进程 stdout/stderr 并写入 LogBuffer
**Uses:** `os/exec.Cmd.StdoutPipe()`, `bufio.Scanner`, goroutine
**Implements:** Log Capture 组件 (ARCHITECTURE.md)

**Implementation:**
- 修改 `internal/lifecycle/starter.go` 添加 `logBuffer *logbuffer.LogBuffer` 参数
- 使用 `cmd.StdoutPipe()` + `cmd.StderrPipe()` 捕获输出
- 启动 2 个 goroutine 并发读取流 (防止死锁)
- 向后兼容:logBuffer 参数可为 nil (禁用日志捕获)

**Validation:**
- 进程启动成功且日志出现在 buffer
- 无阻塞 (慢 buffer 不影响进程)
- 进程生命周期 (start/stop) 不受影响

**Avoids:** Pitfall 2 (Process Pipe Deadlock)

### Phase 3: Wire Log Buffers to Instance Lifecycle (集成到实例管理)
**Rationale:** 将 LogBuffer 集成到现有的实例生命周期管理,连接 Phase 1 和 Phase 2
**Delivers:** 每个实例有自己的 LogBuffer,在启动时传递给 Starter
**Uses:** LogBufferManager, InstanceManager, InstanceLifecycle
**Implements:** InstanceManager 和 InstanceLifecycle 修改 (ARCHITECTURE.md)

**Implementation:**
- 修改 `internal/instance/lifecycle.go` 添加 `logBuffer` 字段
- 修改 `internal/instance/manager.go` 添加 `logBufferManager` 字段
- 在 InstanceManager 构造函数中为每个实例创建 LogBuffer
- 在 `StartAfterUpdate()` 中传递 logBuffer 给 `lifecycle.StartNanobot()`

**Validation:**
- 每个实例有独立的 LogBuffer
- Buffer 可通过 manager 访问
- 现有更新流程 (stop→update→start) 工作不变

### Phase 4: SSE Handler Implementation (实时流式传输)
**Rationale:** 在 LogBuffer 和 API 服务器就绪后,实现 SSE 端点暴露日志流
**Delivers:** HTTP 端点 `/api/v1/logs/{name}/stream` (SSE) 和 `/api/v1/logs/{name}/history` (JSON)
**Uses:** `github.com/r3labs/sse/v2`, `net/http`, LogBuffer
**Implements:** SSE Handler 组件 (ARCHITECTURE.md)

**Implementation:**
- 创建 `internal/api/log_handler.go`
- 实现 `LogHandler` 结构体 (持有 `*logbuffer.Manager`)
- 实现 `ServeSSE()` 处理 SSE 连接 (订阅 LogBuffer,推送事件)
- 实现 `ServeHistory()` 返回历史日志 (JSON)
- 在 `internal/api/server.go` 注册路由

**Validation:**
- SSE 连接建立成功
- 历史 API 返回 JSON
- 心跳保持连接活跃 (30s)
- 客户端断开检测 (通过 context)

**Avoids:** Pitfall 1 (SSE Goroutine Leak), Pitfall 4 (HTTP WriteTimeout Breaks SSE)

### Phase 5: Integration Testing (端到端验证)
**Rationale:** 验证所有组件协同工作,发现集成问题
**Delivers:** E2E 测试套件,验证实时日志查看完整流程
**Uses:** 所有 Phase 组件

**Tests:**
1. 启动实例并捕获日志
2. SSE 客户端连接 → 验证实时接收日志
3. 请求历史 API → 验证返回所有行
4. 断开客户端 → 验证无 goroutine 泄漏
5. 慢客户端 → 验证非阻塞行为
6. 多实例 → 验证日志隔离

**Validation:**
- E2E 测试通过
- 内存稳定 (无泄漏)
- 无 goroutine 泄漏

### Phase 6: Documentation and Examples (文档和示例)
**Rationale:** 提供清晰的使用文档,便于用户理解和使用新功能
**Delivers:** README 更新、API 文档、使用示例

**Implementation:**
- 更新 `README.md` 添加日志查看 API 文档
- 添加 `curl` 示例 (history 端点)
- 添加 JavaScript `EventSource` 示例 (SSE 流)
- 文档配置选项 (buffer 大小)

**Validation:**
- 文档审核和测试
- 示例代码可直接运行

### Phase Ordering Rationale

**依赖关系:**
- Phase 2 依赖 Phase 1 (需要 LogBuffer)
- Phase 3 依赖 Phase 1 和 Phase 2 (连接 LogBuffer 到 Starter)
- Phase 4 依赖 Phase 1 和 v0.3 API 服务器 (需要 LogBuffer 和 HTTP 服务器)
- Phase 5 依赖 Phase 1-4 (完整集成)
- Phase 6 依赖 Phase 5 (验证后文档化)

**架构分组:**
- Phase 1-2: 基础设施层 (LogBuffer + 进程捕获)
- Phase 3: 集成层 (连接到实例管理)
- Phase 4: 接口层 (HTTP API)
- Phase 5-6: 验证和文档层

**陷阱避免:**
- Phase 1 避免 Ring Buffer Data Race 和 Memory Leak
- Phase 2 避免 Process Pipe Deadlock
- Phase 4 避免 SSE Goroutine Leak 和 HTTP WriteTimeout Breaks SSE
- Phase 5 验证所有陷阱已被预防

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 4:** SSE 端点与现有认证中间件的集成 — 需研究是否复用 Bearer token 认证或允许匿名访问 (localhost-only 场景)
- **Phase 4:** 前端 Web UI 的实现 — 需研究是否嵌入静态文件到 Go 二进制 (单文件部署) 或分离部署

Phases with standard patterns (skip research-phase):
- **Phase 1:** 环形缓冲区和广播模式 — `smallnest/ringbuffer` 文档完整,Go 并发模式成熟
- **Phase 2:** 进程输出捕获 — Go 官方 `os/exec` 文档详细,社区最佳实践清晰
- **Phase 3:** 依赖注入到现有结构体 — 标准 Go 模式,无特殊风险
- **Phase 5:** 集成测试 — 使用标准 Go 测试框架,无需额外研究

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | 基于 Go 官方文档 (os/exec, net/http)、成熟第三方库 (r3labs/sse, smallnest/ringbuffer)、权威博客 (Cloudflare, OneUptime) |
| Features | HIGH | 基于 Grafana、journalctl、Logcat 等成熟日志查看器对比,MVP 功能定义清晰,优先级合理 |
| Architecture | HIGH | 三层架构模式成熟,集成点明确,代码示例详细,遵循 Go 并发最佳实践 |
| Pitfalls | HIGH | 基于 Go GitHub Issues (#19685, #69060)、Cloudflare 官方博客、社区最佳实践,所有陷阱有预防策略和验证方法 |

**Overall confidence:** HIGH

**Reasoning:**
- SSE + 进程捕获 + 环形缓冲是 Go 生态中的成熟模式,被多个生产系统验证
- 所有依赖都有官方文档和社区支持,无实验性技术
- 架构设计遵循最小侵入原则,向后兼容,风险可控
- 陷阱研究覆盖关键风险,提供预防策略和验证方法

### Gaps to Address

研究过程中发现的不确定领域,需要在规划或实施阶段解决。

**Gap 1: 进程分离模式与日志捕获的兼容性**
- **问题:** 现有 `lifecycle/starter.go` 使用 `cmd.Process.Release()` 分离进程,但 `StdoutPipe()` 需要保持进程附着才能持续读取
- **研究结论:** GitHub Issue #19685 明确指出 `StdoutPipe()` 在进程退出后关闭,无法与 `Release()` 同时使用
- **建议方案:** Phase 2 实施时测试两种方案:
  1. 移除 `Release()`,保持进程附着 (可能需要修改进程管理逻辑)
  2. 使用替代方案:不捕获实时输出,改为定期读取日志文件 (失去实时性)
- **处理时机:** Phase 2 规划时需要决策,可能需要原型验证

**Gap 2: 前端 Web UI 实现方式**
- **问题:** 功能研究建议 "Built-in Web UI",但未研究具体实现方式 (嵌入静态文件 vs 分离部署)
- **建议方案:** Phase 4 规划时研究:
  - 嵌入方案:使用 `go:embed` 将 HTML/CSS/JS 嵌入二进制 (单文件部署)
  - 分离方案:静态文件单独部署 (更灵活但增加部署复杂度)
- **处理时机:** Phase 4 规划时决策,可能需要单独研究前端技术栈

**Gap 3: 日志缓冲大小配置策略**
- **问题:** 研究建议 5000 行环形缓冲,但未研究是否需要用户可配置
- **建议方案:** MVP 使用硬编码 5000 行,Phase 6 后根据用户反馈决定是否添加配置选项
- **配置方式:** 在 `config.yaml` 添加 `log_buffer.max_lines: 5000`
- **处理时机:** Phase 1 实施时决策配置结构,Phase 6 文档化

**Gap 4: SSE 认证与现有 API 认证的关系**
- **问题:** 现有 HTTP API 可能有 Bearer token 认证,研究未明确 SSE 端点是否需要相同认证
- **建议方案:** Phase 4 规划时决策:
  - 复用认证:SSE 端点检查 `Authorization: Bearer <token>` header
  - 放宽认证:localhost-only 场景允许匿名访问日志 (内部工具)
  - URL 参数认证:`/logs/{name}/stream?token=<api_key>` (兼容 EventSource API)
- **处理时机:** Phase 4 规划时与现有认证系统集成

## Sources

### Primary (HIGH confidence)

**Go Official Documentation:**
- [Go os/exec package](https://pkg.go.dev/os/exec) — StdoutPipe/StderrPipe 使用,并发读取模式
- [Go net/http package](https://pkg.go.dev/net/http) — SSE with Flusher interface, Context cancellation
- [MDN Server-Sent Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events) — SSE 协议规范,事件格式,重连机制

**GitHub Issues & Community:**
- [os/exec: data race between StdoutPipe and Wait #19685](https://github.com/golang/go/issues/19685) — Pipe 与 Wait 竞态条件处理
- [r3labs/sse GitHub Repository](https://github.com/r3labs/sse) — SSE 服务器实现,成熟稳定 (2015+)
- [smallnest/ringbuffer GitHub Repository](https://github.com/smallnest/ringbuffer) — 线程安全环形缓冲区

**Authoritative Blogs:**
- [The complete guide to Go net/http timeouts - Cloudflare](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/) — HTTP 超时配置,SSE 长连接处理

### Secondary (MEDIUM confidence)

**SSE Implementation Tutorials:**
- [How to Build Real-time Applications with Go and SSE - OneUptime](https://oneuptime.com/blog/post/2026-02-01-go-realtime-applications-sse/view) — SSE broker 模式,心跳,重连
- [Live website updates with Go, SSE, and htmx - ThreeDotsTech](https://threedots.tech/post/live-website-updates-go-sse-htmx/) — 生产级 SSE 实现
- [Real-Time Data Streaming with Server-Sent Events (SSE) - Dev.to](https://dev.to/serifcolakel/real-time-data-streaming-with-server-sent-events-sse-1gb2) — SSE 最佳实践

**Log Viewer Features:**
- [Logs in Explore - Grafana](https://grafana.com/docs/grafana/latest/visualizations/explore/logs-integration/) — Pause button, live tailing, 日志查看器功能分析
- [How to Monitor Error Logs in Real-Time - Last9](https://last9.io/blog/how-to-monitor-error-logs-in-real-time/) — 交互式日志界面设计

**Concurrency Patterns:**
- [Go Channel Patterns - OneUptime](https://oneuptime.com/blog/post/2026-01-23-go-channel-patterns/view) — 非阻塞 select, channel buffering
- [How to Create Thread-Safe Cache in Go - OneUptime](https://oneuptime.com/blog/post/2026-01-30-go-thread-safe-cache/view) — 并发安全实现

**Memory Management:**
- [Golang Memory Leaks: Detection, Fixes, and Best Practices - Medium](https://medium.com/@mojimich2015/golang-memory-leaks-detection-fixes-and-best-practices-81749e9d698b) — 内存泄漏预防
- [How to Debug Memory Leaks in Go Applications - OneUptime](https://oneuptime.com/blog/post/2026-01-07-go-debug-memory-leaks/view) — 内存调试方法

### Tertiary (LOW confidence)

**Alternative Approaches:**
- [tmaxmax/go-sse GitHub Repository](https://github.com/tmaxmax/go-sse) — SSE 替代实现,更现代但本项目不需要其高级特性
- [go-cmd/cmd GitHub Repository](https://github.com/go-cmd/cmd) — 进程管理替代方案,标准库足够

**Theoretical Background:**
- [Ring buffer in Golang - logdy.dev](https://logdy.dev/blog/post/ring-buffer-in-golang) — 环形缓冲区理论
- [A Practical Guide to Implementing a Generic Ring Buffer in Go - Medium](https://medium.com/checker-engineering/a-practical-guide-to-implementing-a-generic-ring-buffer-in-go-866d27ec1a05) — 通用环形缓冲区实现

---

*Research completed: 2026-03-16*
*Ready for roadmap: yes*

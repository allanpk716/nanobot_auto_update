# Project Research Summary

**Project:** nanobot-auto-updater v0.3
**Domain:** HTTP API Service + Monitoring Service Integration
**Researched:** 2026-03-16
**Confidence:** HIGH

## Executive Summary

v0.3 里程碑将项目从定时更新工具转变为持续运行的服务模式,新增 HTTP API 服务器和后台监控服务两个并发组件。研究表明,使用 Go 标准库 `net/http` 和 `time.Ticker` 足以实现所有功能,无需引入框架或第三方库。关键架构决策包括:

1. **零新增依赖** — 所有新功能通过 Go 1.24.11 标准库实现,与现有 viper/logrus 生态系统完美集成
2. **简单 Bearer Token 认证** — 手写 10 行中间件验证 token,避免 JWT 库的过度工程化
3. **共享更新锁** — 使用 `sync.Mutex.TryLock()` 防止 API 和监控服务并发触发更新,返回 HTTP 409 Conflict
4. **Context 驱动优雅停机** — 使用 `errgroup` 协调多个 goroutine,实现分阶段停机(HTTP → 监控 → 清理)

**主要风险:** Goroutine 泄漏(ticker 未清理)、并发更新冲突、HTTP 超时配置不当导致资源耗尽。研究显示,这些是 Go 服务开发的常见陷阱,通过严格遵循 context 传播和 `defer ticker.Stop()` 模式可有效避免。

## Key Findings

### Recommended Stack

**核心原则:** 标准库优先,避免过度工程化。社区共识明确指向使用 Go 标准库而非框架(来源: Three Dots Labs, Cloudflare Blog)。

**Core technologies:**

- **Go 标准库 `net/http`** — HTTP API 服务器,无需 Gin/Echo 框架,单个 POST 端点足够
- **Go 标准库 `context`** — 超时和取消控制,比 `http.Client.Timeout` 更灵活,避免 goroutine 泄漏
- **`time.Ticker`** — 定时监控调度,替代 `robfig/cron` 库,实现固定 15 分钟间隔
- **`encoding/json`** — API 响应 JSON 编码,标准库足够
- **现有 Pushover 库** — 连通性失败/恢复通知,从环境变量迁移到 YAML 配置
- **`golang.org/x/sync/errgroup`** — Goroutine 协调,实现自动错误传播和取消

**Dependencies to remove:**
- `github.com/robfig/cron/v3` — 不再需要 cron 表达式调度,使用 `time.Ticker` 替代

### Expected Features

**核心功能 (v0.3 必须实现):**

- **Bearer Token 认证** — 用户期望安全的 API 访问控制,静态 token 比对足够
- **JSON 响应格式** — API 必须返回结构化数据,包含 status + message 字段
- **监控服务持续运行** — 后台 goroutine + ticker,每 15 分钟检查 Google 连通性
- **失败通知** — 监控检测到问题时发送 Pushover 通知
- **POST /api/v1/trigger-update** — HTTP 触发更新的主要接口
- **服务日志** — 所有操作记录日志,延续 v0.2 的 WQGroup/logger 模式

**增强功能 (v0.3.x 可选):**

- **恢复通知** — 连通性恢复时发送通知,提供完整事件生命周期可见性
- **优雅停机** — 处理 Ctrl+C,停止监控,完成处理中请求
- **健康检查端点** — `/health` 返回 200 OK,允许外部监控

**明确不做 (Anti-features):**

- **JWT 认证** — 单用户内部服务过度复杂,静态 token 足够
- **多监控目标** — 增加指数级复杂度,单目标 + 多实例模式更简单
- **响应体内容检查** — Google 首页内容因地区变化,仅检查状态码更可靠
- **重试逻辑** — 隐藏真实连通性问题,延迟故障检测
- **数据库历史记录** — 超出项目范围,日志提供足够历史

### Architecture Approach

**架构演进:** 从 cron 调度器转变为双服务并发模型(HTTP API + 监控服务),通过共享锁协调更新操作。

**Major components:**

1. **HTTP API Server** (`internal/api/`) — 暴露 `/api/v1/trigger-update` 端点,Bearer token 认证,尝试获取更新锁,触发更新
2. **Monitoring Service** (`internal/monitor/`) — 每 15 分钟检查 Google 连通性,状态变化时触发更新和通知,使用 ticker + context
3. **Instance Manager** (`internal/instance/`, 现有) — 协调所有 nanobot 实例的停止→更新→启动生命周期,无需修改
4. **Shared Update Lock** (`sync.Mutex`) — 防止 API 和监控服务并发触发更新,使用 `TryLock()` 非阻塞模式
5. **Config Extension** (`internal/config/`, 修改) — 新增 API port、Bearer token、监控间隔字段,YAML 配置

**数据流:**

- **HTTP API 触发:** 客户端请求 → Auth 中间件 → TryLock(成功) → InstanceManager.UpdateAll() → JSON 响应
- **监控触发:** Ticker.C → 检查连通性 → 状态变化 → TryLock(成功) → InstanceManager.UpdateAll() → 通知
- **并发冲突:** TryLock(失败) → HTTP 409 Conflict 或监控跳过

**Goroutine 生命周期:** 使用 `errgroup.WithContext()` 协调 HTTP 服务器和监控服务,任一 goroutine 错误触发全局取消,signal handler 调用 `cancel()` 触发优雅停机。

### Critical Pitfalls

研究识别出 12 个陷阱,其中 6 个为 Critical 级别,必须在对应阶段重点防范:

1. **Goroutine 泄漏(监控 ticker)** — 使用 `time.NewTicker()` 但未调用 `defer ticker.Stop()`,ticker 永久触发,goroutine 永不退出。**避免方法:** 所有 ticker 创建后立即 `defer ticker.Stop()`,使用 `select` 监听 `ctx.Done()`
2. **并发更新触发(HTTP API)** — 多个请求同时到达,触发多个更新流程,导致资源竞争、端口冲突、重复通知。**避免方法:** 使用 `sync.Mutex` 或任务队列,实现单个 worker 模式
3. **HTTP 客户端超时导致重试风暴** — 监控服务无超时或超时过长,网络挂起时 goroutine 阻塞,恢复时流量激增。**避免方法:** 设置显式超时(连接 5s,总请求 10s),实现指数退避重试
4. **优雅停机放弃处理中操作** — 单一超时用于所有阶段,处理中请求被强制终止,系统处于不一致状态。**避免方法:** 分阶段停机(HTTP 5s → goroutines 等待 → 清理),使用 WaitGroup 跟踪 goroutine
5. **Bearer token 验证安全细节** — 接受空 token、使用 `==` 比较(时序攻击)、记录 token。**避免方法:** 使用 `crypto/hmac.Equal` 常量时间比较,从不记录 token,验证 token 长度 ≥32
6. **端口冲突启动失败** — `ListenAndServe()` 在 goroutine 中返回错误,但应用程序继续运行,API 不可达。**避免方法:** 使用 `net.Listen()` 先绑定端口,检查错误后再启动服务器

## Implications for Roadmap

基于研究发现,建议分 8 个阶段实施:

### Phase 1: Configuration Foundation
**Rationale:** 配置是所有服务的基础,必须首先扩展以支持新功能
**Delivers:** 新增 API 和监控配置字段,验证逻辑,单元测试
**Addresses:** Configuration from YAML (FEATURES.md table stakes)
**Avoids:** Config validation (PITFALLS.md #12) - 启动时验证,快速失败

### Phase 2: Monitoring Service Core
**Rationale:** 监控服务独立于 HTTP API,可单独测试,无外部依赖
**Delivers:** 连通性检查 goroutine + ticker,状态跟踪,日志集成
**Uses:** Go 标准库 `net/http` + `context` (STACK.md)
**Implements:** Monitoring Service component (ARCHITECTURE.md)
**Avoids:** Goroutine leak (PITFALLS.md #1), HTTP client timeout (PITFALLS.md #3), Notification spam (PITFALLS.md #9)

### Phase 3: HTTP API Server
**Rationale:** HTTP API 独立于监控服务,可单独测试认证和路由
**Delivers:** HTTP 服务器,POST endpoint,Bearer token 中间件,健康检查
**Uses:** Go 标准库 `net/http` (STACK.md)
**Implements:** HTTP API Server component (ARCHITECTURE.md)
**Avoids:** Auth validation (PITFALLS.md #5), Port conflicts (PITFALLS.md #6), Concurrent updates (PITFALLS.md #2)

### Phase 4: Shared Update Lock + Integration
**Rationale:** 连接两个新服务到现有 InstanceManager,实现核心协调逻辑
**Delivers:** sync.Mutex 更新锁,TryLock 模式,并发触发测试
**Implements:** Shared Update Lock (ARCHITECTURE.md)
**Avoids:** Concurrent update triggers (PITFALLS.md #2), Context not propagated (PITFALLS.md #7), State corruption (PITFALLS.md #8)

### Phase 5: Notification Enhancements
**Rationale:** 扩展现有 Notifier 支持恢复通知,监控服务依赖此功能
**Delivers:** NotifyRecovery() 方法,状态变化通知逻辑
**Addresses:** Recovery Notifications (FEATURES.md P2)
**Avoids:** Pushover notification on every check (PITFALLS.md #9)

### Phase 6: Main Application Coordination
**Rationale:** 主函数集成所有组件,使用 errgroup 协调 goroutine 生命周期
**Delivers:** errgroup 协调,context 取消,signal handler,集成测试
**Uses:** `golang.org/x/sync/errgroup` (STACK.md)
**Implements:** Graceful shutdown flow (ARCHITECTURE.md)
**Avoids:** Graceful shutdown abandoning operations (PITFALLS.md #4)

### Phase 7: Remove Legacy Cron
**Rationale:** 所有新功能验证通过后,清理旧代码,避免技术债务
**Delivers:** 移除 `internal/scheduler/` 包,移除 Config.Cron 字段
**Addresses:** Dependencies to remove (STACK.md)

### Phase 8: End-to-End Testing
**Rationale:** 完整系统验证,确保所有组件协同工作,无 goroutine 泄漏
**Delivers:** E2E 测试套件,覆盖 API 触发、监控触发、并发冲突、优雅停机
**Validates:** All critical pitfalls avoided (PITFALLS.md)

### Phase Ordering Rationale

**依赖关系驱动顺序:**
1. Phase 1 (Config) 无依赖,必须首先完成
2. Phase 2 (Monitoring) 和 Phase 3 (HTTP API) 都依赖 Phase 1,但彼此独立,可并行开发
3. Phase 4 (Integration) 依赖 Phase 2 和 3,需要两个服务都已实现
4. Phase 5 (Notification) 依赖 Phase 2,监控服务需要通知能力
5. Phase 6 (Main Coordination) 依赖 Phase 1-5,集成所有组件
6. Phase 7 (Remove Cron) 依赖 Phase 6,确保新功能完全替代旧功能
7. Phase 8 (E2E Testing) 依赖所有阶段,作为最终验证

**架构模式驱动分组:**
- Phase 1-3 按组件边界分组,每个阶段交付独立可测试的组件
- Phase 4-5 按集成点分组,连接组件与现有系统
- Phase 6-8 按生命周期分组,从主协调到清理到验证

**陷阱避免策略:**
- Phase 2 专门处理 3 个 critical pitfalls (goroutine leak, timeout, notification spam)
- Phase 3 专门处理 3 个 critical pitfalls (auth, port, concurrent)
- Phase 4 验证并发控制的正确性
- Phase 6 验证优雅停机的完整性
- Phase 8 最终验证所有陷阱已被避免

### Research Flags

**Phases likely needing deeper research during planning:**

- **Phase 2 (Monitoring Service):** Google 连通性检查的具体实现细节需要研究 — 目标 URL、超时阈值、失败判定条件、与 v0.2 实例管理器的集成点
- **Phase 4 (Integration):** 并发更新触发的边缘情况需要实际测试验证 — TryLock 失败时的重试策略、长时间运行更新对监控 ticker 的影响

**Phases with standard patterns (skip research-phase):**

- **Phase 1 (Configuration):** 标准的 viper + mapstructure 模式,v0.2 已验证
- **Phase 3 (HTTP API):** 标准库 net/http + middleware 模式,文档充分
- **Phase 5 (Notification):** 扩展现有 Notifier,无需新研究
- **Phase 7 (Remove Cron):** 简单的代码删除,无技术复杂性

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | 基于 Go 官方文档、Cloudflare 权威指南、Three Dots Labs 实践经验,多个来源一致推荐标准库优先 |
| Features | MEDIUM | 基于 API 监控领域通用实践,但具体功能优先级依赖用户反馈,可能在实施中调整 |
| Architecture | HIGH | 基于官方 errgroup 文档、context 传播模式、Cloudflare timeout 指南,架构模式经过生产验证 |
| Pitfalls | HIGH | 基于 5 个 HIGH confidence 来源 + 6 个 MEDIUM confidence 来源,所有 critical pitfalls 有明确预防策略 |

**Overall confidence:** HIGH

### Gaps to Address

**实现期间需要验证的领域:**

1. **Google 连通性检查的稳定性:** 研究建议检查状态码,但实际 Google 首页可能返回 302 重定向或其他状态码。需要在实现时测试真实响应,可能需要调整判定逻辑(例如 2xx 视为成功,或跟随重定向)。

2. **与 v0.2 InstanceManager 的集成细节:** 研究假设 InstanceManager 已有并发保护,但需要代码审查确认。如果缺少锁保护,Phase 4 需要添加额外的同步机制。

3. **Pushover API 速率限制:** 研究未找到官方速率限制文档,需要查看 Pushover API 文档确认通知频率限制,避免触发反垃圾机制。如果限制严格,可能需要调整通知去重逻辑。

4. **长期运行稳定性:** 研究基于理论模式,需要 24-48 小时实际运行测试验证资源使用模式、goroutine 泄漏检测、日志轮转行为。建议 Phase 8 增加压力测试和长时间运行测试。

5. **Bearer token 安全存储:** 研究建议使用配置文件,但未解决 token 安全存储问题。生产环境可能需要环境变量或加密存储,需要在 Phase 1 确定最终方案。

## Sources

### Primary (HIGH confidence)

**Stack Research:**
- [pkg.go.dev/net/http (Official Documentation)](https://pkg.go.dev/net/http) — Go HTTP server 官方文档
- [The complete guide to Go net/http timeouts (Cloudflare Blog)](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/) — 超时配置权威指南
- [When You Shouldn't Use Frameworks in Go (Three Dots Labs)](https://threedots.tech/episode/when-you-should-not-use-frameworks/) — 标准库优先原则
- [Go http client timeout vs context timeout (Stack Overflow)](https://stackoverflow.com/questions/64129364) — 社区共识

**Pitfalls Research:**
- [How to Avoid Common Goroutine Leaks in Go](https://oneuptime.com/blog/post/2026-01-07-go-goroutine-leaks/view) — 全面模式和生产示例
- [Timeout Budgets & Retries in Go: How Retry Storms Happen](https://levelup.gitconnected.com/timeout-budgets-retries-in-go-how-retry-storms-happen-stop-them-a6269958647d) — 生产验证模式
- [Graceful Shutdown in Go: Why Most Implementations Are Wrong](https://medium.com/codex/graceful-shutdown-in-go-why-most-implementations-are-wrong-323ff193f1f8) — 详细分析
- [API Security Myth: The Bearer Model Is Enough](https://corsh.com/blog/api-security-myth-bearer-model-is-enough) — 安全最佳实践
- [How to Use Context in Go for Cancellation and Timeouts](https://oneuptime.com/blog/post/2026-01-23-go-context/view) — 官方模式

**Architecture Research:**
- [Go sync package (Official Documentation)](https://pkg.go.dev/sync) — Mutex.TryLock() 官方文档
- [golang.org/x/sync/errgroup (Official Documentation)](https://pkg.go.dev/golang.org/x/sync/errgroup) — Goroutine 协调
- [How to Use Graceful Shutdown in a Go Cloud Run Service with Context Cancellation](https://oneuptime.com/blog/post/2026-02-17-how-to-implement-graceful-shutdown-in-a-go-cloud-run-service-with-context-cancellation/view) — Context 驱动停机模式
- [How to Use errgroup for Parallel Operations in Go](https://oneuptime.com/blog/post/2026-01-07-go-errgroup/view) — errgroup 协调模式
- [How to Implement Middleware in Go Web Applications](https://oneuptime.com/blog/post/2026-01-26-go-middleware/view) — 认证中间件模式

### Secondary (MEDIUM confidence)

**Stack Research:**
- [Choosing a Go Web Framework in 2026: A Minimalist's Guide](https://medium.com/@samayun_pathan/choosing-a-go-web-framework-in-2026-a-minimalists-guide-to-gin-fiber-chi-echo-and-beppo-c79b31b8474d) — 社区实践
- [Go's http.ServeMux Is All You Need](https://dev.to/leapcell/gos-httpservemux-is-all-you-need-1mam) — Go 1.22+ 特性
- [Timeouts in Go: A Comprehensive Guide (Better Stack)](https://betterstack.com/community/guides/scaling-go/golang-timeouts/) — 最佳实践总结

**Features Research:**
- [SigNoz: The Ultimate Guide to API Monitoring in 2026](https://signoz.io/blog/api-monitoring-complete-guide/) — 行业标准实践
- [Netdata: Monitor Everything is an Anti-Pattern!](https://www.netdata.cloud/blog/monitor-everything-is-an-anti-pattern/) — 权威分析
- [AWS DevOps Guidance: Anti-patterns for Continuous Monitoring](https://docs.aws.amazon.com/wellarchitected/latest/devops-guidance/anti-patterns-for-continuous-monitoring.html) — 官方文档
- [Postman: What are HTTP status codes?](https://blog.postman.com/what-are-http-status-codes/) — 权威 API 工具供应商

**Pitfalls Research:**
- [Go Goroutines: 7 Critical Pitfalls Every Developer Must Avoid](https://medium.com/@harshithgowdakt/go-goroutines-7-critical-pitfalls-every-developer-must-avoid-with-real-world-solutions-a436ac0fb4bb) — 常见错误
- [Mastering Network Timeouts and Retries in Go](https://dev.to/jones_charles_ad50858ddb0/mastering-network-timeouts-and-retries-in-go) — 实用指南
- [Secure & Scalable APIs in Go with JWT](https://levelup.gitconnected.com/mastering-jwt-authentication-in-go-10-expert-tips-for-secure-and-scalable-apis-723d16402b16) — 安全提示

### Tertiary (LOW confidence)

- [core-go/health (GitHub)](https://github.com/core-go/health) — 可选库调研,最终未采用
- [brpaz/go-healthcheck (GitHub)](https://github.com/brpaz/go-healthcheck) — 可选库调研,最终未采用

---
*Research completed: 2026-03-16*
*Ready for roadmap: yes*

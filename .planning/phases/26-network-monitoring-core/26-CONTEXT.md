# Phase 26: Network Monitoring Core - Context

**Gathered:** 2026-03-21
**Status:** Ready for planning

<domain>
## Phase Boundary

定期监控网络连通性,通过 HTTP 请求测试 Google 的可达性,记录成功和失败日志,提供可配置的检查间隔和超时时间。此阶段专注于连通性测试的核心监控,不涉及状态变化通知(Phase 27)或 HTTP API 触发更新(Phase 28)。

**核心功能:**
- 定期发送 HTTP HEAD 请求到 https://www.google.com
- 检查 HTTP 200 OK 响应作为连通性成功标准
- 记录成功和失败日志,包含响应时间、状态码和错误类型
- 追踪上一次连通性状态,为 Phase 27 状态变化通知做准备
- 与应用生命周期集成,API 服务器启动后启动监控

**成功标准:**
1. 系统定期(默认间隔)向 google.com 发送 HTTP 请求测试连通性
2. 请求失败时,用户可以在 ERROR 日志中看到失败的详细信息(响应时间、错误类型)
3. 请求成功时,用户可以在 INFO 日志中看到成功的记录(响应时间、HTTP 状态码)
4. 用户可以通过配置文件调整监控间隔和请求超时时间

</domain>

<decisions>
## Implementation Decisions

### 监控目标和方法
- **HTTP HEAD 请求到 https://www.google.com**
  - 使用 HEAD 方法而非 GET,减少流量和耗时
  - 测试基础连通性,不下载响应体
  - 目标 URL: `https://www.google.com`
  - 禁用 HTTP 重定向跟随,严格测试直接响应
  - 实现: 使用 Go 标准库 `net/http.Client` 发送 HEAD 请求

### 成功标准
- **仅 HTTP 200 OK 才算成功**
  - 严格标准,只有 200 状态码视为连通性成功
  - 所有其他 HTTP 状态码(包括 2xx、3xx、4xx、5xx)视为失败
  - 简单明确,适合基础连通性测试
  - 记录实际收到的状态码用于调试

### 日志详情
- **记录全面的诊断信息**
  - 成功日志: INFO 级别,包含响应时间(ms)、HTTP 状态码(200)
  - 失败日志: ERROR 级别,包含响应时间(ms)、错误类型分类
  - 响应时间: 记录从发起请求到收到响应头的总耗时
  - 不细分 DNS 解析、TCP 连接、TLS 握手各阶段时间(保持简单)
  - 日志示例:
    ```
    INFO  Google 连通性检查成功 duration=234ms status_code=200
    ERROR Google 连通性检查失败 duration=5000ms error_type="连接超时"
    ```

### 失败分类
- **统一 ERROR 日志 + 错误类型标注**
  - 所有失败统一记录为 ERROR 日志级别
  - 在日志消息中标注错误类型,帮助快速定位问题
  - 基础错误分类(覆盖常见场景):
    - DNS 解析失败: `net.DNSError`
    - 连接超时: `context.DeadlineExceeded` 或 `net.Error.Timeout() == true`
    - 连接拒绝: `syscall.ECONNREFUSED`
    - TLS 握手错误: `tls.CertificateError`, `x509.UnknownAuthorityError`
    - HTTP 非 200 响应: 根据状态码分类 (3xx, 4xx, 5xx)
  - 不使用不同的日志级别区分错误严重程度(保持简单)

### HTTP 客户端配置
- **禁用重定向跟随**
  - 配置 `http.Client.CheckRedirect` 返回 `http.ErrUseLastResponse`
  - 避免跟随 301/302 重定向,严格测试 google.com 直接响应
  - 适合 HEAD 请求场景
- **使用标准 HTTP 客户端**
  - 不设置自定义 User-Agent(使用 Go 默认)
  - 不验证 TLS 证书(使用系统默认信任链)
  - 超时使用配置的 `monitor.timeout` (默认 10s)

### 启动时机和生命周期
- **API 服务器启动后启动**
  - 启动顺序: 配置加载 → InstanceManager 创建 → API 服务器启动 → 健康监控启动 → 网络监控启动
  - 与 Phase 25 健康监控相同,确保 API 服务器先准备好
  - 网络监控在独立 goroutine 中运行,不阻塞其他组件
  - 实现位置: 在 `main.go` 中,API 服务器启动 goroutine 和健康监控启动之后

### 状态追踪
- **追踪上一次连通性状态**
  - 类似 Phase 25 健康监控,维护 `lastState` 变量记录上一次连通性状态
  - 每次检查后更新状态
  - 在日志中标注状态变化(首次检查、状态保持、状态改变)
  - 为 Phase 27 连通性变化通知做准备
  - 状态定义:
    ```go
    type ConnectivityState struct {
        IsConnected bool      // true: 连通, false: 不连通
        LastCheck   time.Time // 上次检查时间
    }
    ```

### 优雅关闭
- **与启动顺序相反**
  - 关闭顺序: 网络监控先停 → 健康监控后停 → API 服务器最后停
  - 使用 `context.Context` 实现优雅关闭
  - 监控循环监听 `ctx.Done()` 信号,收到后立即退出
  - 在应用 shutdown 钩子中调用 `networkMonitor.Stop()`

### Claude's Discretion
- 日志消息的具体措辞(中文/英文)
- 响应时间格式(毫秒 vs 秒)
- 错误类型标注格式(括号、冒号、等号)
- 监控循环的初始延迟(立即检查 vs 等待第一个 interval)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 26 需求
- `.planning/REQUIREMENTS.md` § MONITOR — 网络监控需求 (MONITOR-01, 02, 03, 06)
- `.planning/ROADMAP.md` § Phase 26 — Network Monitoring Core 阶段目标和成功标准

### 配置参考
- `internal/config/monitor.go` — MonitorConfig 配置结构和验证逻辑
- `config.yaml` — monitor 配置示例 (interval: 15m, timeout: 10s)

### 架构模式参考
- `.planning/phases/24-auto-start/24-CONTEXT.md` — Auto-start 生命周期集成模式
- `.planning/phases/25-instance-health-monitoring/25-RESEARCH.md` — HealthMonitor 实现模式 (time.Ticker + goroutine + context)
- `internal/health/monitor.go` — HealthMonitor 参考实现 (状态追踪、定期检查、优雅关闭)

### Go 标准库文档
- `net/http` — HTTP 客户端实现
- `context` — 优雅关闭和超时控制
- `time` — Ticker 定时器

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **internal/config/monitor.go**: `MonitorConfig` 配置结构已存在
  - 已有 `Interval` (默认 15m) 和 `Timeout` (默认 10s) 字段
  - 已有 `Validate()` 方法,验证范围合理
  - 无需扩展配置结构
- **internal/health/monitor.go**: `HealthMonitor` 实现模式可复用
  - `Start()` 方法: time.Ticker + goroutine + context 模式
  - 状态追踪: `states map[string]*InstanceHealthState` 模式
  - 优雅关闭: `ctx context.Context` + `cancel context.CancelFunc` 模式
  - 首次检查: 立即执行一次初始检查
- **internal/config/config.go**: `Config` 结构已集成 `Monitor` 字段
  - 配置加载和验证已完成
  - 默认值已设置

### Established Patterns
- **独立监控结构体**: Phase 25 确定的 `HealthMonitor` 模式,封装监控逻辑和状态
- **time.Ticker 定期检查**: Phase 25 确定的定期检查模式,支持 Stop() 防止 goroutine 泄漏
- **Context 优雅关闭**: Phase 25 确定的 context 取消信号传播模式
- **状态 map 追踪**: Phase 25 确定的状态追踪模式,记录上一次状态
- **API 后启动**: Phase 24 确定的 API 服务器优先启动模式
- **上下文感知日志**: Phase 7 确定的结构化日志模式

### Integration Points
- **配置加载**: `config.Load()` 已解析 `monitor` 配置,无需修改
- **main.go 集成**: 在 API 服务器启动和健康监控启动后,创建并启动 `NetworkMonitor`
- **生命周期管理**: 在应用 shutdown 钩子中,先停止网络监控,再停止健康监控
- **Phase 27 通知**: Phase 27 将使用本阶段追踪的状态变化发送 Pushover 通知

</code_context>

<specifics>
## Specific Ideas

- **HEAD 请求更高效**: 比 GET 请求更快且不下载响应体,减少流量和耗时,适合定期监控场景
- **严格 200 OK 标准**: 简单明确的成功标准,避免歧义,适合基础连通性测试
- **全面日志信息**: 记录响应时间、状态码、错误类型,提供丰富的诊断信息,帮助快速定位问题
- **状态追踪为通知准备**: 追踪上一次连通性状态,为 Phase 27 的状态变化通知功能做准备
- **禁用重定向跟随**: 严格测试 google.com 直接响应,避免跟随重定向影响测试结果
- **基础错误分类**: 覆盖常见失败场景,实现简单但足够诊断问题
- **与 Phase 25 一致**: 启动时机、生命周期管理模式与健康监控保持一致,降低理解成本

</specifics>

<deferred>
## Deferred Ideas

- **状态变化通知** — Phase 27 专门处理连通性状态变化时的 Pushover 通知
- **多目标监控** — 当前仅监控 google.com,如需监控多个端点(如 github.com、自定义服务器)需要新的配置和实现
- **详细的分阶段耗时** — 当前仅记录总耗时,如需 DNS 解析、TCP 连接、TLS 握手各阶段时间需要更复杂的实现
- **自适应检查间隔** — 当前使用固定间隔,如需根据历史连通性动态调整检查间隔需要新的算法

</deferred>

---

*Phase: 26-network-monitoring-core*
*Context gathered: 2026-03-21*

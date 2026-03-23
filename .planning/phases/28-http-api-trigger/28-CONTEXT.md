# Phase 28: HTTP API Trigger - Context

**Gathered:** 2026-03-23
**Status:** Ready for planning

<domain>
## Phase Boundary

用户通过 HTTP API 远程触发 nanobot 的完整更新流程（停止→更新→启动）。此阶段专注于 API 端点设计、认证、并发控制和响应格式，不涉及更新逻辑本身（复用 Phase 05 的流程）或实例管理（复用 Phase 24 的机制）。

**核心功能：**
- POST /api/v1/trigger-update 端点
- Bearer Token 认证（RFC 6750 标准）
- 同步执行完整更新流程并返回结果
- 并发控制：防止重复更新请求
- JSON 格式的请求和响应

**成功标准：**
1. 用户发送 POST /api/v1/trigger-update 请求（带 Bearer Token）可以触发更新
2. 认证失败的请求返回 401 错误，不触发更新
3. 更新流程执行完整的停止-更新-启动过程
4. 用户收到 JSON 格式的更新结果（成功/失败、详细信息）
5. 更新过程中重复请求被拒绝，用户收到"更新进行中"的错误消息

</domain>

<decisions>
## Implementation Decisions

### 认证机制
- **Bearer Token 传递方式**
  - 使用标准 Authorization header: `Authorization: Bearer <token>`
  - 符合 RFC 6750 标准，与大多数 HTTP 客户端和工具兼容
  - 示例请求：
    ```http
    POST /api/v1/trigger-update HTTP/1.1
    Host: localhost:8080
    Authorization: Bearer your-secret-token-here
    Content-Type: application/json
    ```
- **Token 验证**
  - 从配置文件的 `api.bearer_token` 读取
  - 使用 `strings.TrimPrefix(authHeader, "Bearer ")` 提取 token
  - 使用 `subtle.ConstantTimeCompare` 进行常量时间比较（防止时序攻击）
- **认证失败响应**
  - HTTP 状态码: 401 Unauthorized
  - JSON 格式响应：
    ```json
    {
      "error": "unauthorized",
      "message": "Invalid or missing Bearer token"
    }
    ```
  - 符合 RFC 7807 Problem Details 标准

### 并发控制
- **状态跟踪**
  - 使用 `atomic.Bool` 标志跟踪更新状态（`isUpdating`）
  - 在 InstanceManager 或专用 UpdateHandler 中维护
  - 开始更新前检查并设置标志，更新完成后重置
- **重复请求处理**
  - 如果 `isUpdating` 为 true，立即返回错误
  - HTTP 状态码: 409 Conflict
  - JSON 格式响应：
    ```json
    {
      "error": "conflict",
      "message": "Update already in progress"
    }
  ```
  - 客户端可以稍后重试（建议等待至少 30-60 秒）
- **实现要点**
  - 使用 `atomic.Bool` 而非 mutex，因为只需要简单的 true/false 状态
  - 确保 defer 重置标志，避免死锁
  - 日志记录重复请求被拒绝的事件

### 响应格式
- **复用 Phase 05 CLI 格式**
  - 保持一致性，客户端可以复用现有的 JSON 解析逻辑
  - 成功响应（200 OK）：
    ```json
    {
      "success": true,
      "version": "1.2.3",
      "source": "github",
      "message": "Update completed"
    }
    ```
  - 失败响应（200 OK，但 success=false）：
    ```json
    {
      "success": false,
      "error": "Network timeout",
      "exit_code": 1
    }
    ```
  - 注意：即使更新失败，HTTP 状态码仍为 200（因为 HTTP 请求本身成功）
- **HTTP 状态码使用**
  - 200 OK: 请求成功处理（更新可能成功或失败，查看 JSON 的 success 字段）
  - 401 Unauthorized: 认证失败
  - 409 Conflict: 更新进行中
  - 504 Gateway Timeout: 更新超时（超过 api.timeout）
  - 500 Internal Server Error: 服务器内部错误（如配置错误）

### 更新范围和超时
- **更新范围**
  - 更新所有配置的实例（与 Phase 24 自动启动行为一致）
  - 调用 `InstanceManager.UpdateAllInstances()` 方法（需要新增）
  - 按配置顺序依次执行：停止 → 更新 → 启动
  - 优雅降级：某实例失败不影响其他实例（继承 Phase 8 模式）
- **超时设置**
  - 使用配置文件的 `api.timeout` (默认 30s) 作为整个更新流程的超时
  - 使用 `context.WithTimeout` 创建带超时的 context
  - 超时后返回 504 Gateway Timeout + JSON 错误：
    ```json
    {
      "error": "timeout",
      "message": "Update operation timed out after 30s"
    }
    ```
  - 客户端可以调整 `config.yaml` 中的 `api.timeout` 值

### 日志记录
- **统一日志格式**
  - 与定时更新和自动启动使用相同的日志格式和级别
  - 添加 `source=api-trigger` 字段区分触发来源
  - 示例：
    - INFO: `Starting API-triggered update for all instances` (source=api-trigger)
    - INFO: `Instance "gateway" started successfully` (source=api-trigger, instance=gateway)
    - ERROR: `Failed to update instance "worker"` (source=api-trigger, instance=worker, error=...)
- **日志级别**
  - INFO: 更新开始、实例启动/停止、更新完成
  - ERROR: 实例失败、认证失败、超时
  - WARN: 重复请求被拒绝
- **上下文感知**
  - 使用 `logger.With("source", "api-trigger")` 预注入字段
  - 所有日志自动包含触发来源标识

### Claude's Discretion
- Bearer Token 验证的具体实现细节（如何提取、如何比较）
- atomic.Bool 的初始化位置（InstanceManager vs 专用 Handler）
- 错误消息的具体措辞（中文/英文）
- 日志字段的具体命名（source vs trigger vs api_call）
- 更新流程的具体实现（是否需要新增 UpdateAllInstances 方法）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 28 需求
- `.planning/REQUIREMENTS.md` § API — HTTP API 触发更新需求 (API-01, 02, 03, 04, 05, 06)
- `.planning/ROADMAP.md` § Phase 28 — HTTP API Trigger 阶段目标和成功标准

### 现有架构参考
- `.planning/phases/05-cli-immediate-update/05-CONTEXT.md` — CLI 立即更新的 JSON 格式和更新流程
- `.planning/phases/24-auto-start/24-CONTEXT.md` — 实例自动启动逻辑和失败处理
- `.planning/phases/08-instance-coordinator/08-CONTEXT.md` — InstanceManager 协调器和优雅降级策略

### 外部标准
- RFC 6750: Bearer Token Usage — https://tools.ietf.org/html/rfc6750
- RFC 7807: Problem Details for HTTP APIs — https://tools.ietf.org/html/rfc7807

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **internal/api/server.go**: HTTP API 服务器框架
  - 已有 `NewServer()` 和路由注册逻辑
  - 可以在 `NewServer()` 中添加新的 `/api/v1/trigger-update` 路由
  - 已配置 WriteTimeout=0 支持长时间请求
- **internal/config/config.go**: 配置结构
  - 已有 `APIConfig.BearerToken` 字段
  - 已有 `APIConfig.Timeout` 字段（默认 30s）
  - 已有验证逻辑（最小 token 长度 32 字符）
- **internal/instance/manager.go**: InstanceManager
  - 已有 `StartAllInstances()` 方法（Phase 24）
  - 需要新增 `UpdateAllInstances()` 方法
  - 提供 `instances []*InstanceLifecycle` 访问
- **internal/instance/lifecycle.go**: InstanceLifecycle
  - 已有 `Stop()`, `Update()`, `Start()` 方法
  - 可以组合调用实现完整更新流程
- **cmd/nanobot-auto-updater/main.go**: 主程序入口
  - 已有 API 服务器启动和优雅关闭逻辑
  - 已有 auto-start goroutine 模式（Phase 24）

### Established Patterns
- **Bearer Token 认证**: 使用 `subtle.ConstantTimeCompare` 进行常量时间比较
- **atomic.Bool 并发控制**: Go 1.19+ 的原子布尔类型，线程安全
- **JSON 响应格式**: 复用 Phase 05 的格式，保持一致性
- **上下文感知日志**: 使用 `logger.With()` 预注入字段
- **优雅降级**: Phase 8 确定的失败不中断流程模式
- **Context 超时**: 使用 `context.WithTimeout` 控制长时间操作

### Integration Points
- **API 路由注册**: 在 `api.NewServer()` 中添加 `POST /api/v1/trigger-update`
- **认证中间件**: 创建 `AuthMiddleware()` 验证 Bearer Token
- **InstanceManager 扩展**: 添加 `UpdateAllInstances(ctx) error` 方法
- **日志注入**: 使用 `logger.With("source", "api-trigger")` 标记 API 触发的更新
- **错误处理**: 返回统一的 JSON 错误格式，区分 4xx 和 5xx 错误

</code_context>

<specifics>
## Specific Ideas

- **标准 Bearer Token** 确保与所有 HTTP 客户端兼容，无需自定义 header
- **atomic.Bool** 提供简单的并发控制，避免复杂的 mutex 或 channel 机制
- **复用 CLI JSON 格式** 让客户端可以复用现有的解析逻辑，减少集成成本
- **409 Conflict** 明确告知客户端更新进行中，符合 HTTP 语义
- **统一日志格式** 便于日志分析和调试，可以通过 source 字段过滤 API 触发的更新
- **同步模式** 简化客户端实现，无需轮询状态端点
- **优雅降级** 确保部分实例失败不影响整体更新流程

</specifics>

<deferred>
## Deferred Ideas

None — 讨论保持在阶段范围内

</deferred>

---

*Phase: 28-http-api-trigger*
*Context gathered: 2026-03-23*

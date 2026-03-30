# Phase 39: HTTP API Integration - Context

**Gathered:** 2026-03-30
**Status:** Ready for planning

<domain>
## Phase Boundary

将 Phase 38 完成的 `selfupdate.Updater` 包暴露为 HTTP API 端点，用户可远程检查自更新版本和触发自更新，同时更新 Help 接口包含自更新端点说明。本阶段不涉及重启/通知/清理机制（Phase 40）。

**成功标准：**
1. POST /api/v1/self-update 端点需要 Bearer Token 认证，认证失败返回 401
2. 自更新与 nanobot 更新互斥，并发请求返回 409 Conflict
3. GET /api/v1/self-update/check 只读检查最新版本，返回当前版本和最新版本信息
4. Help 接口包含自更新相关端点的使用说明

</domain>

<decisions>
## Implementation Decisions

### 自更新执行模型
- **D-01:** 异步执行 — POST /api/v1/self-update 立即返回 202 Accepted，后台 goroutine 执行更新。客户端通过 GET /api/v1/self-update/check 轮询进度。exe 替换后进程仍在内存中运行旧代码（重启由 Phase 40 处理）。

### 互斥锁策略
- **D-02:** 复用 isUpdating — 自更新和 trigger-update 共享同一个 `atomic.Bool` (`isUpdating`) 锁。任何一个进行中，另一个都返回 409 Conflict。符合 API-02 互斥要求。

### 响应格式
- **D-03:** Check 端点详细模式 — GET /api/v1/self-update/check 返回：
  - `current_version`: 当前版本号
  - `latest_version`: 最新版本号
  - `needs_update`: 是否需要更新 (bool)
  - `release_notes`: Release 说明
  - `published_at`: 发布时间
  - `download_url`: 下载链接
  - `self_update_status`: 当前自更新状态 (`idle` / `updating` / `updated` / `failed`)
  - `self_update_error`: 最近一次自更新错误信息（仅 failed 时）
- **D-04:** Update 端点 — POST /api/v1/self-update 返回 202 Accepted + `{ "status": "accepted", "message": "Self-update started" }`。不返回更新结果（异步执行）。

### 状态轮询
- **D-05:** 复用 check 端点 — 客户端轮询 GET /api/v1/self-update/check，检查 `self_update_status` 字段获取进度。无需新增专用状态端点。

### 错误码策略
- **D-06:** 标准复用 — 与 trigger-update 保持一致：
  - 401 Unauthorized: 认证失败（Bearer Token 无效或缺失）
  - 409 Conflict: 更新进行中（isUpdating=true）
  - 500 Internal Server Error: 内部错误
  - 503 Service Unavailable: 自更新未配置（self_update 配置缺失）

### Help 端点集成
- **D-07:** 直接添加 — 在 help.go 的 `getEndpoints()` 中添加 `self_update_check` 和 `self_update` 两个端点说明，与现有端点保持相同格式。

### Claude's Discretion
- SelfUpdateHandler 的具体 struct 设计和字段
- 状态管理实现（atomic.Value 或专用 struct + mutex）
- 日志字段命名和上下文注入方式
- 测试策略和 mock 方式

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 自更新核心包
- `internal/selfupdate/selfupdate.go` — Updater struct + CheckLatest() + NeedUpdate() + Update() API
- `internal/selfupdate/selfupdate_test.go` — 测试模式参考

### 现有 HTTP API 架构
- `internal/api/server.go` — Server struct + NewServer() 路由注册模式
- `internal/api/trigger.go` — TriggerHandler 模式（struct + constructor + Handle method）
- `internal/api/auth.go` — AuthMiddleware + validateBearerToken + writeJSONError
- `internal/api/help.go` — HelpHandler + getEndpoints() 端点列表

### 并发控制参考
- `internal/instance/manager.go` — isUpdating atomic.Bool 模式

### 配置参考
- `internal/config/config.go` — Config struct + SelfUpdateConfig 字段

### 需求追踪
- `.planning/ROADMAP.md` — Phase 39 成功标准（4 条）
- `.planning/phases/38-self-update-core/38-CONTEXT.md` — Phase 38 上下文和 API 设计

### 项目规范
- `CLAUDE.md` — 项目规则：使用中文回答，临时文件放 tmp/

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/api/auth.go:AuthMiddleware()` — 直接复用 Bearer Token 认证中间件
- `internal/api/auth.go:writeJSONError()` — 直接复用 JSON 错误响应（RFC 7807）
- `internal/api/trigger.go:TriggerHandler` — 复用 Handler struct 模式（constructor + Handle method）
- `internal/api/help.go:getEndpoints()` — 添加自更新端点说明
- `internal/selfupdate/selfupdate.go:Updater` — CheckLatest() + NeedUpdate() + Update() 全部就绪
- `internal/instance/manager.go:isUpdating` — atomic.Bool 并发控制锁

### Established Patterns
- Handler struct + constructor + Handle method（Phase 28, 29, 32）
- AuthMiddleware 包装受保护端点（Phase 28）
- logger.With("source", "...") 上下文感知日志
- writeJSONError(w, statusCode, code, message) 统一错误格式
- 上下文感知日志: source=api-trigger / source=api-help / source=api-self-update
- config.APIConfig.BearerToken 用于认证

### Integration Points
- `internal/api/server.go:NewServer()` — 添加自更新路由注册，需要接收 *selfupdate.Updater
- `internal/api/help.go:getEndpoints()` — 添加 self_update_check 和 self_update 端点
- `internal/instance/manager.go` — isUpdating 锁需要从 TriggerHandler 和 SelfUpdateHandler 双向访问
- `cmd/nanobot-auto-updater/main.go` — 创建 Updater 实例并传入 NewServer()

</code_context>

<specifics>
## Specific Ideas

- 路由设计：`POST /api/v1/self-update`（触发）和 `GET /api/v1/self-update/check`（检查）
- 状态字段 self_update_status 值：idle / updating / updated / failed
- 异步更新 goroutine 完成后更新内部状态，客户端轮询 check 端点查看
- isUpdating 锁需要从 InstanceManager 暴露出来，或通过接口抽象传递给两个 Handler
- Help 端点新增两个端点说明：self_update_check (GET, auth required) 和 self_update (POST, auth required)

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 39-http-api-integration*
*Context gathered: 2026-03-30*

# Phase 50: Instance Config CRUD API - Context

**Gathered:** 2026-04-11
**Status:** Ready for planning

<domain>
## Phase Boundary

通过验证过的 REST API 管理实例配置（创建、读取、更新、删除、复制），变更自动持久化到 config.yaml 并触发热重载。

具体交付：
1. POST 创建新实例配置（含验证）→ 写入 config.yaml
2. PUT 更新已有实例配置 → 写入 config.yaml
3. DELETE 删除实例配置（自动停止运行中实例）→ 写入 config.yaml
4. GET 查询单个/全部实例配置
5. POST copy 复制实例配置（自动生成名称/端口）
6. 所有端点 Bearer Token 认证保护

不包括：实例启停控制（Phase 51）、nanobot 配置管理（Phase 52）、管理界面（Phase 53）。

</domain>

<decisions>
## Implementation Decisions

### API 路由设计
- **D-01:** 使用独立资源路径 `/api/v1/instance-configs`，避免与现有 `/api/v1/instances` 路由混淆
- **D-02:** RESTful 端点设计：
  - `GET /api/v1/instance-configs` — 列表
  - `POST /api/v1/instance-configs` — 创建
  - `GET /api/v1/instance-configs/{name}` — 单个详情
  - `PUT /api/v1/instance-configs/{name}` — 更新
  - `DELETE /api/v1/instance-configs/{name}` — 删除
  - `POST /api/v1/instance-configs/{name}/copy` — 复制
- **D-03:** 所有端点使用 `authMiddleware` 保护（复用现有 Bearer Token constant-time comparison）
- **D-04:** 复制操作用 `POST .../copy` 子资源路径，非标准 REST 但语义清晰

### JSON 字段命名
- **D-05:** JSON 请求/响应字段名与 YAML 配置一致（snake_case）：`name`, `port`, `start_command`, `startup_timeout`（秒数）, `auto_start`
- **D-06:** `startup_timeout` API 层使用秒数（uint32），内部转换为 Go time.Duration

### 配置持久化与热重载
- **D-07:** CRUD 操作直接写 config.yaml，依赖现有 500ms debounce 热重载机制处理实例重启
- **D-08:** 写入使用 `viper.WriteConfig()`（viper 已持有全局实例，直接调用）
- **D-09:** CRUD handler 流程：修改内存 Config → 写 YAML → 返回响应 → 热重载异步处理实例重启
- **D-10:** 不需要绕过热重载机制，全量替换（stop all → recreate → start all）在 CRUD 场景可接受

### 复制实例策略
- **D-11:** 默认名称：原名称 + `-copy` 后缀（如 `gateway` → `gateway-copy`）。用户可在请求体中覆盖 `name` 字段
- **D-12:** 默认端口：原端口 + 1 递增，直到找到未被占用的端口。用户可在请求体中覆盖 `port` 字段
- **D-13:** 不在 Phase 50 处理 nanobot 配置目录创建。复制只克隆 auto-updater 配置。nanobot 目录和配置留给 Phase 52

### 验证与错误处理
- **D-14:** 验证失败返回 422 Unprocessable Entity + 详细字段错误数组。格式：`{"error": "validation_error", "message": "...", "errors": [{"field": "name", "message": "..."}]}`
- **D-15:** 复用现有验证逻辑：`InstanceConfig.Validate()` + `Config.Validate()`（含唯一性检查）
- **D-16:** 删除运行中实例时，自动先停止该实例再从配置中删除（使用现有 StopAllNanobots 逻辑）
- **D-17:** 实例不存在时返回 404 Not Found

### Claude's Discretion
- Handler 结构设计（struct with Handle method vs factory function）
- 配置读写的并发安全机制（mutex 保护 viper 写操作）
- CRUD handler 与 viper/热重载的具体集成方式
- copy 端点的请求体结构（哪些字段可覆盖）
- 列表端点的响应格式（直接返回 Config.Instances 数组或包装为对象）
- 写入失败时的错误恢复策略

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 配置系统
- `internal/config/config.go` — Config 根结构体、Load()、ReloadConfig()、GetViper()、Config.Validate()（含唯一性检查）
- `internal/config/instance.go` — InstanceConfig 结构体、Validate()、ShouldAutoStart()
- `internal/config/hotreload.go` — WatchConfig()、GetCurrentConfig()、HotReloadCallbacks（含 OnInstancesChange 全量替换回调）
- `config.yaml` — 现有配置文件示例（了解实际 YAML 格式）

### HTTP API 模式
- `internal/api/server.go` — NewServer() 路由注册方式、Go 1.22 方法路由语法
- `internal/api/auth.go` — AuthMiddleware()、writeJSONError()、认证和错误响应模式
- `internal/api/trigger_handler.go` — Handler struct + Handle method 模式参考
- `internal/api/webconfig_handler.go` — localhostOnly() 中间件、handler factory 模式参考
- `internal/api/selfupdate_handler.go` — 复杂 handler（多端点、异步操作、状态管理）参考

### 实例管理
- `internal/instance/manager.go` — InstanceManager、GetLifecycle()、GetInstanceConfigs()、GetInstanceNames()
- `internal/instance/lifecycle.go` — StartAllInstances()、StopAllNanobots()（删除时停止实例需调用）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `InstanceConfig.Validate()` — 已实现字段级验证（name 非空、port 1-65535、start_command 非空、startup_timeout >= 5s）
- `Config.Validate()` — 已实现跨实例验证（唯一名称 validateUniqueNames、唯一端口 validateUniquePorts）
- `AuthMiddleware()` — 完整的 Bearer Token 认证中间件，支持动态 token getter
- `writeJSONError()` — 统一的 JSON 错误响应函数
- `GetCurrentConfig()` — 热重载模块提供的当前配置读取
- `GetViper()` — 全局 viper 实例获取
- Go 1.22 `http.NewServeMux()` — 支持方法路由和路径参数 `{name}`

### Established Patterns
- Handler struct + Handle method 模式（TriggerHandler、SelfUpdateHandler）
- `r.PathValue("name")` 获取路径参数（Go 1.22）
- `w.Header().Set("Content-Type", "application/json")` + `json.NewEncoder(w).Encode()` 响应
- `logger.With("source", "api-xxx")` 上下文感知日志
- 配置热重载：500ms debounce + OnInstancesChange 全量替换
- viper 全局实例管理（viperInstance）

### Integration Points
- `internal/api/server.go` NewServer() — 注册新 CRUD 路由
- `internal/config/config.go` — 需要**新增** SaveConfig() 函数
- `internal/config/hotreload.go` GetCurrentConfig() — CRUD 读取当前配置
- `internal/instance/lifecycle.go` StopAllNanobots() — 删除运行中实例时调用
- `cmd/nanobot-auto-updater/main.go` — NewServer() 调用点，可能需要传入新依赖

</code_context>

<specifics>
## Specific Ideas

- viper.WriteConfig() 可能改变 YAML 格式（丢失注释、调整字段顺序），但对于自动管理的配置文件这是可接受的
- 热重载的全量替换策略（stop all → recreate → start all）意味着每次 CRUD 操作后所有实例会重启，在单实例或少量实例场景下延迟可接受
- 复制时的端口递增策略需要遍历现有实例检查端口占用，类似 validateUniquePorts 的逻辑

</specifics>

<deferred>
## Deferred Ideas

- nanobot 配置目录创建和默认 config.json — Phase 52（Nanobot Config Management API）
- 智能增量热重载（只重启被修改的实例）— 可作为未来优化
- 批量操作 API（批量创建/删除）— 未来里程碑

</deferred>

---
*Phase: 50-instance-config-crud-api*
*Context gathered: 2026-04-11*

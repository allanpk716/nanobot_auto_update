# Phase 52: Nanobot Config Management API - Context

**Gathered:** 2026-04-12
**Status:** Ready for planning

<domain>
## Phase Boundary

通过 API 读写每个实例的 nanobot config.json，含自动目录和默认配置创建。集成到 Phase 50 的 create/copy 流程中。

具体交付：
1. GET /api/v1/instance-configs/{name}/nanobot-config — 读取实例的 nanobot config.json
2. PUT /api/v1/instance-configs/{name}/nanobot-config — 更新实例的 nanobot config.json
3. 创建新实例时自动创建 nanobot 配置目录 + 默认 config.json（集成到 Phase 50 handler）
4. 复制实例时克隆 nanobot config.json 到新目录，更新 port 和 workspace（集成到 Phase 50 handler）

不包括：实例启停控制（Phase 51 已完成）、管理界面（Phase 53）、nanobot config schema 验证（未来）。

</domain>

<decisions>
## Implementation Decisions

### 目录路径解析
- **D-01:** 从 start_command 的 `--config` 参数解析 nanobot 配置路径（使用正则或字符串匹配提取 `--config <path>` 后的值）
- **D-02:** 当 start_command 中不含 `--config` 参数时，fallback 到 `~/.nanobot-{name}/config.json` 规则
- **D-03:** 路径中 `~` 解析为用户 home 目录（`os.UserHomeDir()`）

### 默认配置模板
- **D-04:** 新实例的默认 nanobot config.json 包含完整结构（agents、channels、providers、gateway、tools），敏感值留空字符串
- **D-05:** 自动参数化的字段：`gateway.port` = 实例配置端口，`agents.defaults.workspace` = 实例配置目录路径
- **D-06:** 保留合理的默认值：model="glm-5-turbo", provider="zhipu", maxTokens=131072, temperature=0.7, gateway.host="0.0.0.0"
- **D-07:** telegram 频道默认 disabled=true、token=""、allowFrom=[]
- **D-08:** providers 结构保留（zhipu/groq/aihubmix）但 apiKey 全部留空

### Create/Copy 集成
- **D-09:** 将 NanobotConfigManager（或等效功能）注入到 Phase 50 的 InstanceConfigHandler 中
- **D-10:** Create 流程：创建实例配置后 → 创建 nanobot 目录 → 写入默认 config.json → 返回响应
- **D-11:** Copy 流程：复制实例配置后 → 创建新 nanobot 目录 → 复制源实例 config.json → 更新 gateway.port 和 workspace → 返回响应
- **D-12:** 需要修改 Phase 50 的 InstanceConfigHandler（新增 NanobotConfigManager 依赖注入），以及 NewServer() 构造函数

### 运行中实例行为
- **D-13:** PUT nanobot-config 只写文件，不自动重启实例
- **D-14:** 响应中可包含实例运行状态提示（informational），但不触发任何操作
- **D-15:** 用户通过 Phase 51 的 lifecycle API（POST .../stop + POST .../start）手动重启使配置生效

### Claude's Discretion
- nanobot config 写入时的 JSON 格式验证（至少确保是合法 JSON）
- 文件写入并发安全机制（mutex 保护同实例的并发写入）
- NanobotConfigManager 的具体 struct 设计和接口定义
- 目录创建时的错误处理（权限不足、路径冲突等）
- 正则/字符串匹配 --config 参数的具体实现方式
- 响应 JSON 的包装格式（直接返回 config.json 内容或包装为对象）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 配置系统
- `internal/config/config.go` — Config 根结构体、UpdateConfig() 持久化函数（viper 写入 config.yaml）
- `internal/config/instance.go` — InstanceConfig 结构体（含 start_command 字段，Phase 52 从中解析 --config 路径）
- `internal/config/hotreload.go` — WatchConfig()、GetCurrentConfig()、HotReloadCallbacks

### HTTP API 模式（Phase 50 已实现，Phase 52 需修改）
- `internal/api/instance_config_handler.go` — InstanceConfigHandler（CRUD handler，Phase 52 需注入 NanobotConfigManager）
- `internal/api/instance_lifecycle_handler.go` — InstanceLifecycleHandler（Phase 51 handler 模式参考）
- `internal/api/server.go` — NewServer() 路由注册（Phase 52 需新增 nanobot-config 路由 + 修改构造函数传参）
- `internal/api/auth.go` — AuthMiddleware()、writeJSONError() 复用

### 实例管理
- `internal/instance/manager.go` — InstanceManager、GetLifecycle()、GetInstanceConfigs()
- `internal/instance/lifecycle.go` — StartAllInstances()、StopAllNanobots()

### 配置文件参考
- `config.yaml` — 了解实际 YAML 格式和 start_command 中 --config 路径写法

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `InstanceConfigHandler` — Phase 50 的 CRUD handler，Phase 52 需修改其构造函数注入 NanobotConfigManager
- `config.UpdateConfig(fn)` — config.yaml 写入函数（仅用于 auto-updater 配置，不适用于 nanobot config.json）
- `AuthMiddleware()` — Bearer Token 认证中间件，nanobot-config 端点复用
- `writeJSONError()` — 统一 JSON 错误响应
- `r.PathValue("name")` — Go 1.22 路径参数提取

### Established Patterns
- Handler struct + `NewXxxHandler(dependencies, logger)` 构造模式
- `mux.Handle("METHOD /path", authMiddleware(http.HandlerFunc(handler.HandleXxx)))` 路由注册
- `logger.With("source", "api-xxx")` 上下文感知日志
- `errors.As()` 分发自定义错误类型（validationError、notFoundError）
- `json.NewDecoder(r.Body).Decode(&req)` 请求解析 + `json.NewEncoder(w).Encode()` 响应

### Integration Points
- `internal/api/instance_config_handler.go` HandleCreate/HandleCopy — 注入 nanobot 目录创建调用
- `internal/api/server.go` NewServer() — 新增 nanobot-config 路由注册
- `cmd/nanobot-auto-updater/main.go` — 可能需要调整 NewServer() 调用传参
- 文件系统 — nanobot config.json 的读写（独立于 viper，直接 os.ReadFile/os.WriteFile）

</code_context>

<specifics>
## Specific Ideas

### 用户提供的 nanobot config.json 完整示例
```json
{
  "agents": {
    "defaults": {
      "workspace": "~/.nanobot-web-clipper",
      "model": "glm-5-turbo",
      "provider": "zhipu",
      "maxTokens": 131072,
      "temperature": 0.7,
      "maxToolIterations": 100,
      "memoryWindow": 50
    }
  },
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "...",
      "allowFrom": ["..."],
      "proxy": null
    }
  },
  "providers": {
    "zhipu": { "apiKey": "...", "apiBase": "https://open.bigmodel.cn/api/coding/paas/v4/", "extraHeaders": null },
    "groq": { "apiKey": "...", "apiBase": null, "extraHeaders": null },
    "aihubmix": { "apiKey": "...", "apiBase": null, "extraHeaders": null }
  },
  "gateway": { "host": "0.0.0.0", "port": 18792 },
  "tools": {
    "web": { "search": { "apiKey": "...", "maxResults": 5 } },
    "exec": { "timeout": 60 },
    "restrictToWorkspace": false,
    "mcpServers": {}
  }
}
```

### 默认配置模板（创建新实例时使用）
基于完整示例，清空敏感值，参数化 port 和 workspace：
- `agents.defaults.workspace` → `~/.nanobot-{instance_name}`
- `gateway.port` → `{instance_port}`
- 所有 `apiKey`/`token` → `""`
- `channels.telegram.enabled` → `false`

</specifics>

<deferred>
## Deferred Ideas

- nanobot config schema 验证（ENC-01）— 未来里程碑
- nanobot config 模板库（ENC-02）— 未来里程碑
- 智能增量热重载（只重启被修改的实例）— 可作为未来优化
- PUT 后自动重启实例 — 当前手动重启，未来可考虑可选参数

</deferred>

---
*Phase: 52-nanobot-config-management-api*
*Context gathered: 2026-04-12*

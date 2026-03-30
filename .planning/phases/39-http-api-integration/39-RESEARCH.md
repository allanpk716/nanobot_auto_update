# Phase 39: HTTP API Integration - Research

**Researched:** 2026-03-30
**Domain:** Go HTTP API handler integration (net/http + existing project patterns)
**Confidence:** HIGH

## Summary

Phase 39 的核心任务是将 Phase 38 完成的 `selfupdate.Updater` 包暴露为两个 HTTP API 端点（GET check + POST update），并更新 Help 接口。这是一个纯集成层工作——所有底层能力（版本检查、下载、校验、替换）已在 Phase 38 实现，所有 HTTP 基础设施（认证、路由、错误格式、Handler 模式）已在 Phase 28-29 建立。本阶段的工作量集中在：新建 `selfupdate_handler.go`（Handler struct + constructor + Handle/HandleUpdate/HandleCheck methods），修改 `server.go`（注册路由 + 注入 Updater），修改 `help.go`（添加端点说明），修改 `main.go`（创建 Updater 实例并传入）。

**Primary recommendation:** 严格复用 TriggerHandler 的 struct+constructor+Handle 模式，复用 AuthMiddleware 和 writeJSONError，通过 InstanceManager 暴露的 `IsUpdating()` 和新增互斥方法共享 isUpdating 锁。状态管理推荐 `atomic.Value` 存储 `SelfUpdateStatus` struct。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** 异步执行 -- POST /api/v1/self-update 立即返回 202 Accepted，后台 goroutine 执行更新。exe 替换后进程仍在内存中运行旧代码（重启由 Phase 40 处理）。
- **D-02:** 复用 isUpdating -- 自更新和 trigger-update 共享同一个 `atomic.Bool` (`isUpdating`) 锁。任何一个进行中，另一个都返回 409 Conflict。
- **D-03:** Check 端点详细模式 -- GET /api/v1/self-update/check 返回：current_version, latest_version, needs_update, release_notes, published_at, download_url, self_update_status, self_update_error。
- **D-04:** Update 端点 -- POST /api/v1/self-update 返回 202 Accepted + `{ "status": "accepted", "message": "Self-update started" }`。
- **D-05:** 复用 check 端点 -- 客户端轮询 GET /api/v1/self-update/check 检查 `self_update_status` 字段。
- **D-06:** 错误码 -- 401/409/500/503，与 trigger-update 保持一致。
- **D-07:** help.go getEndpoints() 添加 self_update_check 和 self_update 两个端点说明。

### Claude's Discretion
- SelfUpdateHandler 的具体 struct 设计和字段
- 状态管理实现（atomic.Value 或专用 struct + mutex）
- 日志字段命名和上下文注入方式
- 测试策略和 mock 方式

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| API-01 | POST /api/v1/self-update 端点需要 Bearer Token 认证，认证失败返回 401 | 复用 AuthMiddleware (auth.go:70)，与 trigger-update 完全相同的包装方式 |
| API-02 | 自更新与 nanobot 更新互斥，并发请求返回 409 Conflict | 共享 InstanceManager.isUpdating atomic.Bool (D-02)，需暴露方法或接口 |
| API-03 | GET /api/v1/self-update/check 只读检查最新版本，返回当前版本和最新版本信息 | 调用 Updater.NeedUpdate() + 状态查询，响应格式见 D-03 |
| API-04 | Help 接口包含自更新相关端点的使用说明 | 在 help.go getEndpoints() 添加两个条目，与现有格式一致 (D-07) |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| net/http (stdlib) | Go 1.24 | HTTP server, routing, handler | 项目已使用 Go 1.24 + net/http + 方法路由模式 |
| sync/atomic (stdlib) | Go 1.24 | 并发控制 | 项目已用 atomic.Bool 做互斥，扩展为 atomic.Value 存状态 |
| log/slog (stdlib) | Go 1.24 | 结构化日志 | 项目统一使用 slog |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| stretchr/testify | current | 断言和测试工具 | selfupdate_test.go 和 trigger_test.go 已使用 |
| net/http/httptest (stdlib) | Go 1.24 | HTTP handler 测试 | 所有 api/*_test.go 使用 |

**Installation:** 无需新依赖，全部使用 Go 标准库和项目已有依赖。

**Version verification:** Go 1.24.11 已确认在目标机器安装。

## Architecture Patterns

### Recommended Project Structure
```
internal/api/
  server.go                    # 新增路由注册 + Updater 注入
  selfupdate_handler.go        # 新建: SelfUpdateHandler + Handle/HandleCheck/HandleUpdate
  selfupdate_handler_test.go   # 新建: 单元测试 + mock
  auth.go                      # 不修改: 复用 AuthMiddleware + writeJSONError
  trigger.go                   # 不修改: 参考模式
  help.go                      # 修改: getEndpoints() 添加两个条目
internal/instance/
  manager.go                   # 修改: 暴露 isUpdating 的 CAS 操作给 SelfUpdateHandler
cmd/nanobot-auto-updater/
  main.go                      # 修改: 创建 Updater 实例并传入 NewServer
```

### Pattern 1: Handler Struct + Constructor (复用 TriggerHandler 模式)
**What:** HTTP handler 作为 struct，构造函数注入依赖，Handle 方法处理请求
**When to use:** 所有新 API 端点
**Example:**
```go
// Source: internal/api/trigger.go (项目内已建立的模式)
type SelfUpdateHandler struct {
    updater         SelfUpdateChecker  // 接口: NeedUpdate() + Update()
    version         string             // 当前版本号 (从 main.go 注入)
    instanceManager UpdateMutex        // 接口: TryLock() + Unlock() + IsUpdating()
    status          *SelfUpdateStatus  // atomic.Value 指向的状态
    logger          *slog.Logger
}

func NewSelfUpdateHandler(updater SelfUpdateChecker, version string, im UpdateMutex, logger *slog.Logger) *SelfUpdateHandler {
    return &SelfUpdateHandler{
        updater:         updater,
        version:         version,
        instanceManager: im,
        status:          &SelfUpdateStatus{Status: "idle"},
        logger:          logger.With("source", "api-self-update"),
    }
}
```

### Pattern 2: 接口抽象实现 mock 和解耦 (复用 TriggerUpdater 模式)
**What:** 在 handler 包内定义最小接口，用 duck typing 解耦具体依赖
**When to use:** 需要外部依赖时 (trigger.go 已用此模式)
**Example:**
```go
// Source: internal/api/trigger.go:23 (项目内模式)
type SelfUpdateChecker interface {
    NeedUpdate(currentVersion string) (bool, *selfupdate.ReleaseInfo, error)
    Update(currentVersion string) error
}

type UpdateMutex interface {
    TryLockUpdate() bool   // atomic.Bool CompareAndSwap
    UnlockUpdate()         // atomic.Bool Store(false)
    IsUpdating() bool      // atomic.Bool Load
}
```

### Pattern 3: 异步执行 + 状态追踪
**What:** POST 立即返回 202，后台 goroutine 执行，atomic.Value 追踪状态
**When to use:** D-01 异步执行要求
**Example:**
```go
type SelfUpdateStatus struct {
    Status string // "idle" / "updating" / "updated" / "failed"
    Error  string // 最近一次错误信息 (仅 failed 时)
}

// handler 内:
func (h *SelfUpdateHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
    // 检查互斥锁
    if !h.instanceManager.TryLockUpdate() {
        writeJSONError(w, http.StatusConflict, "conflict", "Update already in progress")
        return
    }

    // 更新状态为 updating
    h.status.Store(&SelfUpdateStatus{Status: "updating"})

    // 返回 202
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusAccepted)
    json.NewEncoder(w).Encode(map[string]string{"status": "accepted", "message": "Self-update started"})

    // 后台 goroutine 执行更新
    go func() {
        defer h.instanceManager.UnlockUpdate()
        if err := h.updater.Update(h.version); err != nil {
            h.status.Store(&SelfUpdateStatus{Status: "failed", Error: err.Error()})
            h.logger.Error("Self-update failed", "error", err)
            return
        }
        h.status.Store(&SelfUpdateStatus{Status: "updated"})
        h.logger.Info("Self-update completed successfully")
    }()
}
```

### Pattern 4: InstanceManager 暴露互斥方法
**What:** 不暴露 atomic.Bool 字段本身，而是暴露语义化方法
**When to use:** D-02 共享 isUpdating 锁
**Example:**
```go
// internal/instance/manager.go 新增方法:
// TryLockUpdate 尝试获取更新锁，返回是否成功
func (m *InstanceManager) TryLockUpdate() bool {
    return m.isUpdating.CompareAndSwap(false, true)
}

// UnlockUpdate 释放更新锁
func (m *InstanceManager) UnlockUpdate() {
    m.isUpdating.Store(false)
}
```

### Anti-Patterns to Avoid
- **直接暴露 atomic.Bool:** 不要导出 isUpdating 字段或其指针，暴露语义化方法
- **在 Handle 方法内 switch method:** 使用 Go 1.22 方法路由 (`POST /path`, `GET /path`)，不需要手动检查 r.Method
- **返回 200 表示异步接受:** 必须返回 202 Accepted (D-04)
- **在 POST handler 中等待更新完成:** 这违反 D-01 异步要求
- **新建独立的互斥锁:** 必须复用 isUpdating (D-02)，不能新建另一个 atomic.Bool

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON 错误响应 | 自己构造 JSON | writeJSONError() (auth.go:113) | 已有 RFC 7807 格式实现 |
| Bearer Token 认证 | 自己解析 Header | AuthMiddleware() (auth.go:70) | 已有 constant-time 比较实现 |
| Handler 路由注册 | 手写 ServeMux 分发 | Go 1.22 方法路由 | `mux.Handle("POST /api/v1/self-update", handler)` 更简洁 |
| 版本比较逻辑 | 在 handler 里比较 | Updater.NeedUpdate() | Phase 38 已实现 semver + dev 逻辑 |
| 下载/校验/替换逻辑 | 在 handler 里做 | Updater.Update() | Phase 38 已实现完整管道 |

**Key insight:** 这个阶段是纯粹的胶水代码（glue code），不应有任何底层逻辑实现。所有能力来自 selfupdate.Updater 和现有 api 基础设施。

## Common Pitfalls

### Pitfall 1: isUpdating 锁未释放 (goroutine panic)
**What goes wrong:** 后台 goroutine panic 导致 UnlockUpdate() 永远不执行，系统死锁
**Why it happens:** goroutine 中没有 panic recovery
**How to avoid:** 在 goroutine 内使用 `defer func() { if r := recover(); r != nil { ... } }()` 模式，与 trigger.go 的通知 goroutine 一致
**Warning signs:** 更新请求一直返回 409 Conflict

### Pitfall 2: 先返回 202 后检查互斥
**What goes wrong:** 如果先写响应再启动 goroutine，并发请求可能在 CAS 检查前都通过
**Why it happens:** 错误的代码顺序
**How to avoid:** TryLockUpdate() 必须在写 202 响应之前执行。顺序：TryLock -> 更新状态 -> 写 202 -> 启动 goroutine
**Warning signs:** 两个 POST 同时返回 202

### Pitfall 3: atomic.Value 存储不一致类型
**What goes wrong:** Store 不同类型导致 Load 时 panic
**Why it happens:** Go 的 atomic.Value 要求所有 Store 的值类型一致
**How to avoid:** 初始化时就 Store 一个 `&SelfUpdateStatus{Status: "idle"}`，之后只 Store 相同类型
**Warning signs:** 运行时 panic: sync/atomic: store of inconsistently typed value

### Pitfall 4: check 端点在更新中调用 NeedUpdate 触发 GitHub API
**What goes wrong:** 更新进行中时调用 check，NeedUpdate() 可能请求 GitHub API（缓存过期时）
**Why it happens:** check 端点直接调用 NeedUpdate()，而 Updater 有 1 小时缓存
**How to avoid:** 这是可接受行为（Updater 内部有缓存），但文档应说明 check 端点可能受缓存影响
**Warning signs:** 更新期间额外的 GitHub API 请求

### Pitfall 5: main.go 未传递 Updater 或 version 给 NewServer
**What goes wrong:** 编译错误或运行时空指针
**Why it happens:** NewServer 签名变更但调用方未更新
**How to avoid:** 先改 NewServer 签名，立即修改 main.go 调用处，确保编译通过
**Warning signs:** 编译失败

## Code Examples

### Server 路由注册 (server.go 修改)
```go
// Source: 基于现有 server.go:82-83 模式扩展
// 创建 SelfUpdateHandler
selfUpdateHandler := NewSelfUpdateHandler(selfUpdater, version, im, logger)
mux.Handle("GET /api/v1/self-update/check",
    authMiddleware(http.HandlerFunc(selfUpdateHandler.HandleCheck)))
mux.Handle("POST /api/v1/self-update",
    authMiddleware(http.HandlerFunc(selfUpdateHandler.HandleUpdate)))
```

### main.go 集成
```go
// Source: 基于现有 main.go:125-141 模式
// Create selfupdate.Updater
selfUpdater := selfupdate.NewUpdater(
    selfupdate.SelfUpdateConfig{
        GithubOwner: cfg.SelfUpdate.GithubOwner,
        GithubRepo:  cfg.SelfUpdate.GithubRepo,
    },
    logger,
)

// 传入 NewServer (需修改 NewServer 签名增加参数)
apiServer, err = api.NewServer(&cfg.API, instanceManager, cfg, Version, logger, updateLogger, notif, selfUpdater)
```

### Help 端点条目 (help.go getEndpoints 修改)
```go
// Source: 基于现有 help.go:62-67 格式
"self_update_check": {
    Method:      "GET",
    Path:        "/api/v1/self-update/check",
    Auth:        "required",
    Description: "检查自更新版本信息（当前版本、最新版本、更新状态）",
},
"self_update": {
    Method:      "POST",
    Path:        "/api/v1/self-update",
    Auth:        "required",
    Description: "触发自更新（异步执行，返回 202 Accepted）",
},
```

### Check 端点响应结构
```go
// D-03 定义的响应格式
type SelfUpdateCheckResponse struct {
    CurrentVersion  string `json:"current_version"`
    LatestVersion   string `json:"latest_version"`
    NeedsUpdate     bool   `json:"needs_update"`
    ReleaseNotes    string `json:"release_notes"`
    PublishedAt     string `json:"published_at"`
    DownloadURL     string `json:"download_url"`
    SelfUpdateStatus string `json:"self_update_status"` // idle/updating/updated/failed
    SelfUpdateError string `json:"self_update_error,omitempty"` // 仅 failed 时
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Go 1.22 前 http.ServeMux 手动方法检查 | Go 1.22+ 方法路由 `"POST /path"` | Go 1.22 (2024-02) | 项目已使用，handler 不需要 r.Method 检查 |
| 手动 mutex 保护状态 | atomic.Value 存储 immutable struct | Go 1.20+ | 推荐: 避免 mutex，状态对象不可变 |

**Deprecated/outdated:**
- 无相关过时模式

## Open Questions

1. **InstanceManager 互斥方法命名**
   - What we know: 需要暴露 CAS 和 Store 操作给 SelfUpdateHandler
   - What's unclear: 方法名用 `TryLockUpdate/UnlockUpdate` 还是其他
   - Recommendation: 用 `TryLockUpdate()` + `UnlockUpdate()`，语义清晰且与 TriggerUpdate 内部使用的 CAS 模式一致

2. **SelfUpdateHandler 一个 struct 还是两个 Handler**
   - What we know: 有两个路由 (GET check + POST update)
   - What's unclear: 是一个 struct 的两个方法，还是两个独立 struct
   - Recommendation: 一个 struct 两个方法 (HandleCheck + HandleUpdate)，因为它们共享 updater、version、status、logger，拆分无意义

3. **NewServer 签名变更方式**
   - What we know: 当前 NewServer 已有 7 个参数
   - What's unclear: 是否应该引入 Options 模式
   - Recommendation: 直接新增第 8 个参数 `selfUpdater *selfupdate.Updater`，项目内已有多参数先例，Options 模式过度设计

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.24 | 编译运行 | Yes | 1.24.11 | -- |
| stretchr/testify | 测试 | Yes | in go.mod | -- |
| 内置 HTTP server | 运行时 | Yes | stdlib | -- |

**Missing dependencies with no fallback:**
- 无

**Missing dependencies with fallback:**
- 无

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify/assert + testify/require |
| Config file | 无 -- Go 原生测试 |
| Quick run command | `go test ./internal/api/... -run TestSelfUpdate -v` |
| Full suite command | `go test ./internal/api/... -v` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| API-01 | POST 认证失败返回 401 | unit | `go test ./internal/api/... -run TestSelfUpdate_Auth -v` | Wave 0 |
| API-02 | 自更新与 nanobot 更新互斥返回 409 | unit | `go test ./internal/api/... -run TestSelfUpdate_Conflict -v` | Wave 0 |
| API-03 | GET check 返回版本信息 + 状态 | unit | `go test ./internal/api/... -run TestSelfUpdate_Check -v` | Wave 0 |
| API-04 | Help 包含自更新端点说明 | unit | `go test ./internal/api/... -run TestHelp_SelfUpdateEndpoints -v` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/api/... -v`
- **Per wave merge:** `go test ./internal/api/... ./internal/selfupdate/... ./internal/instance/... -v`
- **Phase gate:** `go test ./internal/api/... -v` 全部通过

### Wave 0 Gaps
- [ ] `internal/api/selfupdate_handler_test.go` -- covers API-01, API-02, API-03 (mock SelfUpdateChecker + mock UpdateMutex)
- [ ] `internal/api/help_test.go` -- 扩展 TestHelp_SelfUpdateEndpoints covers API-04

### Test Mock 策略
复用项目内 trigger_test.go 的 mock 模式:
- `mockSelfUpdateChecker`: 实现 `SelfUpdateChecker` 接口，可配置返回值
- `mockUpdateMutex`: 实现 `UpdateMutex` 接口，记录锁操作
- 使用 `httptest.NewRequest` + `httptest.NewRecorder` 测试 handler
- 异步 goroutine 测试使用 `time.Sleep(50ms)` 等待 (与 integration_test.go 一致)

## Sources

### Primary (HIGH confidence)
- `internal/api/server.go` -- 路由注册模式 (直接阅读源码)
- `internal/api/trigger.go` -- Handler struct+constructor+Handle 模式 (直接阅读源码)
- `internal/api/auth.go` -- AuthMiddleware + writeJSONError (直接阅读源码)
- `internal/api/help.go` -- getEndpoints() 格式 (直接阅读源码)
- `internal/instance/manager.go` -- isUpdating atomic.Bool + TriggerUpdate CAS 模式 (直接阅读源码)
- `internal/selfupdate/selfupdate.go` -- Updater API: CheckLatest/NeedUpdate/Update (直接阅读源码)
- `cmd/nanobot-auto-updater/main.go` -- 集成点: NewServer 调用 (直接阅读源码)

### Secondary (MEDIUM confidence)
- Go 1.22 release notes -- 方法路由语法 (训练知识，已通过项目源码验证)
- Go sync/atomic 文档 -- atomic.Value 使用模式 (标准库知识)

### Tertiary (LOW confidence)
- 无

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- 无新依赖，全部复用已有
- Architecture: HIGH -- 直接复用项目内已建立的 Handler 模式
- Pitfalls: HIGH -- 基于 trigger.go 的实际实现和已知并发问题总结

**Research date:** 2026-03-30
**Valid until:** 2026-04-30 (Go 生态稳定，项目内部 API 不变)

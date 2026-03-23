# Phase 28: HTTP API Trigger - Research

**Researched:** 2026-03-23
**Domain:** HTTP API, Bearer Token Authentication, Concurrent Update Control
**Confidence:** HIGH

## Summary

Phase 28 实现 HTTP API 远程触发更新功能，核心是通过 POST /api/v1/trigger-update 端点触发完整的停止-更新-启动流程。研究涵盖 Bearer Token 认证（RFC 6750）、并发控制（atomic.Bool）、JSON 响应格式和超时处理。

**Primary recommendation:** 使用标准 Go net/http 包配合 subtle.ConstantTimeCompare 实现 Bearer Token 认证，使用 atomic.Bool 进行并发控制，复用 InstanceManager.UpdateAll() 方法执行更新流程。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### 认证机制
- **Bearer Token 传递方式**
  - 使用标准 Authorization header: `Authorization: Bearer <token>`
  - 符合 RFC 6750 标准，与大多数 HTTP 客户端和工具兼容
- **Token 验证**
  - 从配置文件的 `api.bearer_token` 读取
  - 使用 `strings.TrimPrefix(authHeader, "Bearer ")` 提取 token
  - 使用 `subtle.ConstantTimeCompare` 进行常量时间比较（防止时序攻击）
- **认证失败响应**
  - HTTP 状态码: 401 Unauthorized
  - JSON 格式响应
  - 符合 RFC 7807 Problem Details 标准

#### 并发控制
- **状态跟踪**
  - 使用 `atomic.Bool` 标志跟踪更新状态（`isUpdating`）
  - 在 InstanceManager 或专用 UpdateHandler 中维护
- **重复请求处理**
  - 如果 `isUpdating` 为 true，立即返回错误
  - HTTP 状态码: 409 Conflict
  - 客户端可以稍后重试（建议等待至少 30-60 秒）

#### 响应格式
- **复用 Phase 05 CLI 格式** - 保持一致性
- **HTTP 状态码使用**
  - 200 OK: 请求成功处理（更新可能成功或失败，查看 JSON 的 success 字段）
  - 401 Unauthorized: 认证失败
  - 409 Conflict: 更新进行中
  - 504 Gateway Timeout: 更新超时
  - 500 Internal Server Error: 服务器内部错误

#### 更新范围和超时
- **更新范围**
  - 更新所有配置的实例（与 Phase 24 自动启动行为一致）
  - 调用 `InstanceManager.UpdateAllInstances()` 方法（需要新增）
  - 优雅降级：某实例失败不影响其他实例
- **超时设置**
  - 使用配置文件的 `api.timeout` (默认 30s) 作为整个更新流程的超时
  - 使用 `context.WithTimeout` 创建带超时的 context

#### 日志记录
- **统一日志格式** - 与定时更新和自动启动使用相同的日志格式
- **添加 `source=api-trigger` 字段区分触发来源**
- **上下文感知** - 使用 `logger.With("source", "api-trigger")` 预注入字段

### Claude's Discretion
- Bearer Token 验证的具体实现细节（如何提取、如何比较）
- atomic.Bool 的初始化位置（InstanceManager vs 专用 Handler）
- 错误消息的具体措辞（中文/英文）
- 日志字段的具体命名（source vs trigger vs api_call）
- 更新流程的具体实现（是否需要新增 UpdateAllInstances 方法）

### Deferred Ideas (OUT OF SCOPE)
None - 讨论保持在阶段范围内

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| API-01 | 提供 POST /api/v1/trigger-update 端点 | 在 server.go 中注册路由，创建 TriggerUpdateHandler |
| API-02 | 请求需要 Bearer Token 认证 | 使用 subtle.ConstantTimeCompare 实现安全验证 |
| API-03 | 触发完整的停止→更新→启动流程 | 复用 InstanceManager.UpdateAll() 方法 |
| API-04 | 返回 JSON 格式的更新结果 | 定义 APIUpdateResult 结构体，复用 UpdateResult |
| API-05 | 认证失败时返回 401 错误 | 返回标准 JSON 错误格式 |
| API-06 | 更新过程中拒绝重复请求 | 使用 atomic.Bool 实现 isUpdating 标志 |

</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| net/http | Go 1.24.11 | HTTP server and routing | Go 标准库，项目已使用 |
| crypto/subtle | Go 1.24.11 | ConstantTimeCompare for token validation | 防止时序攻击的标准方案 |
| sync/atomic | Go 1.24.11 | atomic.Bool for concurrent state | Go 1.19+ 内置，线程安全 |
| encoding/json | Go 1.24.11 | JSON encoding/decoding | Go 标准库，项目已使用 |
| context | Go 1.24.11 | Timeout and cancellation | 项目已广泛使用 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| strings | Go 1.24.11 | TrimPrefix for token extraction | Bearer token 提取 |
| fmt | Go 1.24.11 | Formatted strings | 错误消息和日志 |
| time | Go 1.24.11 | Duration handling | 超时设置 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| net/http | gin/echo | 标准库足够，无需引入外部依赖 |
| atomic.Bool | sync.Mutex | atomic.Bool 更简洁，只有布尔状态 |
| subtle.ConstantTimeCompare | == comparison | == 有时序攻击风险 |

**Installation:**
无需安装额外依赖，全部使用 Go 标准库。

**Version verification:**
```
go version go1.24.11 windows/amd64
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── api/
│   ├── server.go           # HTTP server setup (已有)
│   ├── sse.go              # SSE handler (已有)
│   ├── trigger.go          # Trigger update handler (新增)
│   └── auth.go             # Bearer token validation (新增)
├── instance/
│   ├── manager.go          # InstanceManager (已有 UpdateAll 方法)
│   └── result.go           # UpdateResult (已有)
└── config/
    └── api.go              # APIConfig (已有 BearerToken 字段)
```

### Pattern 1: Bearer Token Authentication Middleware
**What:** 使用 Authorization header 传递 token，用 subtle.ConstantTimeCompare 验证
**When to use:** 所有需要认证的 API 端点
**Example:**
```go
// Source: RFC 6750 + Go standard library
package api

import (
    "crypto/subtle"
    "encoding/json"
    "net/http"
    "strings"
)

// AuthMiddleware validates Bearer token from Authorization header
func AuthMiddleware(expectedToken string, logger *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                writeJSONError(w, http.StatusUnauthorized, "unauthorized", "Missing Authorization header")
                return
            }

            // Extract token from "Bearer <token>"
            token := strings.TrimPrefix(authHeader, "Bearer ")
            if token == authHeader {
                // TrimPrefix didn't change anything, meaning "Bearer " prefix was missing
                writeJSONError(w, http.StatusUnauthorized, "unauthorized", "Invalid Authorization header format")
                return
            }

            // Constant-time comparison to prevent timing attacks
            if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
                logger.Warn("Authentication failed", "reason", "invalid_token")
                writeJSONError(w, http.StatusUnauthorized, "unauthorized", "Invalid Bearer token")
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}

func writeJSONError(w http.ResponseWriter, status int, errorCode, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]string{
        "error":   errorCode,
        "message": message,
    })
}
```

### Pattern 2: Concurrent Update Control with atomic.Bool
**What:** 使用 atomic.Bool 跟踪更新状态，防止并发更新
**When to use:** 任何可能并发触发的更新操作
**Example:**
```go
// Source: Go 1.19+ sync/atomic
package instance

import "sync/atomic"

type InstanceManager struct {
    instances   []*InstanceLifecycle
    logger      *slog.Logger
    isUpdating  atomic.Bool // 并发控制标志
}

// TriggerUpdate 触发 API 更新，返回是否被拒绝
func (m *InstanceManager) TriggerUpdate(ctx context.Context) (*UpdateResult, error) {
    // 尝试设置更新标志
    if !m.isUpdating.CompareAndSwap(false, true) {
        // 已经在更新中
        return nil, ErrUpdateInProgress
    }
    defer m.isUpdating.Store(false) // 确保更新完成后重置

    // 执行更新
    return m.UpdateAll(ctx)
}
```

### Pattern 3: Context Timeout for Long Operations
**What:** 使用 context.WithTimeout 控制长时间操作
**When to use:** 任何可能超时的操作（更新、HTTP 请求等）
**Example:**
```go
// Source: Go context package
func (h *TriggerHandler) Handle(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
    defer cancel()

    result, err := h.instanceManager.TriggerUpdate(ctx)
    if err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            writeJSONError(w, http.StatusGatewayTimeout, "timeout",
                fmt.Sprintf("Update operation timed out after %v", h.timeout))
            return
        }
        // Handle other errors...
    }
    // Return success response...
}
```

### Anti-Patterns to Avoid
- **字符串比较 token:** 使用 `==` 比较密码/token 会导致时序攻击，必须使用 `subtle.ConstantTimeCompare`
- **mutex 保护布尔值:** 对于简单的 true/false 状态，atomic.Bool 比 mutex 更高效
- **忘记 defer 重置标志:** 会导致更新永远卡住，必须使用 defer 确保重置
- **HTTP 200 表示所有成功:** 更新失败但 HTTP 成功时，需要在 JSON 中体现，不能改变 HTTP 状态码

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Token 验证 | `==` 字符串比较 | `subtle.ConstantTimeCompare` | 时序攻击风险 |
| 并发状态 | `sync.Mutex` 保护 bool | `atomic.Bool` | 更简洁、更高效 |
| 超时控制 | goroutine + channel | `context.WithTimeout` | 标准模式，可传播 |
| JSON 错误 | 手动拼接字符串 | `json.NewEncoder(w).Encode()` | 转义安全 |
| 路由注册 | 自定义路由器 | `http.ServeMux` (Go 1.22+) | 支持方法匹配 |

**Key insight:** Go 标准库已提供所有必要工具，无需引入外部依赖。保持简洁。

## Common Pitfalls

### Pitfall 1: Timing Attack in Token Validation
**What goes wrong:** 使用 `==` 比较字符串会在不匹配时立即返回，攻击者可以通过响应时间推断正确的 token
**Why it happens:** 字符串比较的早期退出优化
**How to avoid:** 始终使用 `subtle.ConstantTimeCompare`
**Warning signs:** 认证代码中看到 `token == expectedToken`

### Pitfall 2: Deadlock from Forgotten Flag Reset
**What goes wrong:** 更新完成后 atomic.Bool 未重置，后续所有更新请求都被拒绝
**Why it happens:** 错误处理路径中忘记调用 `isUpdating.Store(false)`
**How to avoid:** 使用 `defer isUpdating.Store(false)` 确保在任何情况下都重置
**Warning signs:** 第一次更新成功，后续更新全部返回 409

### Pitfall 3: Context Cancellation Not Propagated
**What goes wrong:** 超时后更新继续执行，资源浪费
**Why it happens:** 未将 context 传递给子操作
**How to avoid:** 确保 `ctx` 传递到所有长时间操作（StopForUpdate、StartAfterUpdate、UV Update）
**Warning signs:** 超时后日志仍显示更新操作在进行

### Pitfall 4: HTTP Status Code Misuse
**What goes wrong:** 更新失败时返回 HTTP 500，客户端无法区分"请求失败"和"服务器错误"
**Why it happens:** 混淆"HTTP 请求成功"和"业务操作成功"
**How to avoid:** HTTP 200 + JSON success=false 表示更新失败，HTTP 5xx 只用于服务器错误
**Warning signs:** 客户端需要解析 HTTP 状态码判断更新结果

### Pitfall 5: Missing Authorization Header Check
**What goes wrong:** 空 Authorization header 导致 TrimPrefix 后 token 为空，可能与空配置的 BearerToken 匹配
**Why it happens:** 未检查 header 是否存在
**How to avoid:** 先检查 header 是否为空，再提取 token
**Warning signs:** 空请求也能通过认证

## Code Examples

Verified patterns from existing codebase:

### Bearer Token Extraction and Validation
```go
// Source: RFC 6750 + existing project patterns
func validateBearerToken(r *http.Request, expectedToken string) (string, error) {
    authHeader := r.Header.Get("Authorization")
    if authHeader == "" {
        return "", errors.New("missing Authorization header")
    }

    token := strings.TrimPrefix(authHeader, "Bearer ")
    if token == authHeader {
        // Prefix not found, meaning format is invalid
        return "", errors.New("invalid Authorization header format")
    }

    if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
        return "", errors.New("invalid token")
    }

    return token, nil
}
```

### Trigger Update Handler Structure
```go
// Source: Based on existing internal/web/handler.go patterns
package api

type TriggerHandler struct {
    instanceManager *instance.InstanceManager
    config          *config.APIConfig
    logger          *slog.Logger
}

func NewTriggerHandler(im *instance.InstanceManager, cfg *config.APIConfig, logger *slog.Logger) *TriggerHandler {
    return &TriggerHandler{
        instanceManager: im,
        config:          cfg,
        logger:          logger.With("source", "api-trigger"),
    }
}

func (h *TriggerHandler) Handle(w http.ResponseWriter, r *http.Request) {
    // 1. Validate method
    if r.Method != http.MethodPost {
        writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST method is supported")
        return
    }

    // 2. Validate Bearer token
    if err := validateBearerToken(r, h.config.BearerToken); err != nil {
        h.logger.Warn("Authentication failed", "error", err)
        writeJSONError(w, http.StatusUnauthorized, "unauthorized", err.Error())
        return
    }

    // 3. Check for concurrent update
    if h.instanceManager.IsUpdating() {
        h.logger.Warn("Update already in progress, request rejected")
        writeJSONError(w, http.StatusConflict, "conflict", "Update already in progress")
        return
    }

    // 4. Execute update with timeout
    ctx, cancel := context.WithTimeout(r.Context(), h.config.Timeout)
    defer cancel()

    result, err := h.instanceManager.TriggerUpdate(ctx)
    if err != nil {
        if errors.Is(err, instance.ErrUpdateInProgress) {
            writeJSONError(w, http.StatusConflict, "conflict", "Update already in progress")
            return
        }
        if errors.Is(err, context.DeadlineExceeded) {
            writeJSONError(w, http.StatusGatewayTimeout, "timeout",
                fmt.Sprintf("Update operation timed out after %v", h.config.Timeout))
            return
        }
        writeJSONError(w, http.StatusInternalServerError, "internal_error", err.Error())
        return
    }

    // 5. Return JSON result
    h.logger.Info("Update completed", "success", !result.HasErrors())
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(APIUpdateResult{
        Success:     !result.HasErrors(),
        Stopped:     result.Stopped,
        Started:     result.Started,
        StopFailed:  result.StopFailed,
        StartFailed: result.StartFailed,
    })
}
```

### API Update Result JSON Format
```go
// Source: Based on existing internal/instance/result.go
type APIUpdateResult struct {
    Success     bool             `json:"success"`
    Stopped     []string         `json:"stopped,omitempty"`
    Started     []string         `json:"started,omitempty"`
    StopFailed  []*InstanceError `json:"stop_failed,omitempty"`
    StartFailed []*InstanceError `json:"start_failed,omitempty"`
    Error       string           `json:"error,omitempty"`
}

// InstanceError JSON serialization (existing)
type InstanceError struct {
    InstanceName string `json:"instance"`
    Operation    string `json:"operation"`
    Port         uint32 `json:"port"`
    Error        string `json:"error"`
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `sync.Mutex` 保护 bool | `atomic.Bool` (Go 1.19+) | Go 1.19 | 更简洁、更高效 |
| 手动路由匹配 | `http.ServeMux` 方法匹配 | Go 1.22 | 支持方法限制 |
| `http.Error` 文本响应 | JSON 错误格式 | RFC 7807 | 客户端友好 |

**Deprecated/outdated:**
- `gorilla/mux` for simple routing: Go 1.22+ 标准库已支持方法匹配
- `==` for token comparison: 存在时序攻击风险

## Open Questions

1. **InstanceManager.IsUpdating() 方法位置**
   - What we know: atomic.Bool 需要在 InstanceManager 中
   - What's unclear: 是否需要暴露 IsUpdating() 方法，还是在 TriggerUpdate() 内部处理
   - Recommendation: 添加 `IsUpdating() bool` 和 `TriggerUpdate(ctx) (*UpdateResult, error)` 两个方法

2. **错误消息语言**
   - What we know: 项目日志使用中文
   - What's unclear: API 错误消息是否使用中文
   - Recommendation: API 错误消息使用英文，便于国际化

## Environment Availability

Step 2.6: SKIPPED (无外部依赖 - 全部使用 Go 标准库)

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + stretchr/testify v1.11.1 |
| Config file | none - 使用 *_test.go 文件 |
| Quick run command | `go test ./internal/api/... -v -run TestTrigger` |
| Full suite command | `go test ./... -v` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| API-01 | POST /api/v1/trigger-update 端点 | unit | `go test ./internal/api/... -v -run TestTriggerHandler_Handle` | ❌ Wave 0 |
| API-02 | Bearer Token 认证 | unit | `go test ./internal/api/... -v -run TestAuthMiddleware` | ❌ Wave 0 |
| API-03 | 触发完整更新流程 | integration | `go test ./internal/instance/... -v -run TestTriggerUpdate` | ❌ Wave 0 |
| API-04 | 返回 JSON 格式结果 | unit | `go test ./internal/api/... -v -run TestTriggerHandler_JSONResponse` | ❌ Wave 0 |
| API-05 | 认证失败返回 401 | unit | `go test ./internal/api/... -v -run TestAuthMiddleware_Unauthorized` | ❌ Wave 0 |
| API-06 | 拒绝并发更新 | unit | `go test ./internal/instance/... -v -run TestTriggerUpdate_Concurrent` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/api/... ./internal/instance/... -v`
- **Per wave merge:** `go test ./... -v`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/api/trigger_test.go` — covers API-01, API-04, API-05
- [ ] `internal/api/auth_test.go` — covers API-02
- [ ] `internal/instance/manager_test.go` — update to cover API-03, API-06 (add TestTriggerUpdate)
- [ ] Framework install: no additional install needed (testify already in go.mod)

## Sources

### Primary (HIGH confidence)
- Go 1.24.11 标准库文档 - net/http, crypto/subtle, sync/atomic
- 现有代码库分析 - internal/api/server.go, internal/instance/manager.go
- RFC 6750: Bearer Token Usage - https://tools.ietf.org/html/rfc6750
- RFC 7807: Problem Details for HTTP APIs - https://tools.ietf.org/html/rfc7807

### Secondary (MEDIUM confidence)
- Phase 24 CONTEXT.md - 自动启动模式和生命周期管理
- Phase 05 CONTEXT.md - JSON 响应格式参考

### Tertiary (LOW confidence)
None - 所有信息均来自官方文档和现有代码

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - 全部使用 Go 标准库，版本确认
- Architecture: HIGH - 基于现有代码模式，复用 InstanceManager
- Pitfalls: HIGH - 安全最佳实践，常见并发问题

**Research date:** 2026-03-23
**Valid until:** 30 days (Go 标准库稳定)

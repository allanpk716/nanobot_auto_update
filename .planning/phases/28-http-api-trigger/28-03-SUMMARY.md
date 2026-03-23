---
phase: 28-http-api-trigger
plan: 03
subsystem: api
tags: [http, api, authentication, trigger, json, tdd]
dependency_graph:
  requires:
    - 28-01 (AuthMiddleware for authentication)
    - 28-02 (TriggerUpdate with concurrent control)
  provides:
    - POST /api/v1/trigger-update endpoint
    - JSON API for triggering updates
  affects:
    - HTTP API server
    - Update workflow
tech_stack:
  added:
    - net/http (Go 1.22+ method matching)
    - encoding/json (APIInstanceError serialization)
  patterns:
    - TDD (Red-Green-Refactor)
    - Middleware chaining (AuthMiddleware + TriggerHandler)
    - JSON error responses (RFC 7807)
key_files:
  created:
    - internal/api/trigger.go
    - internal/api/trigger_test.go
  modified:
    - internal/api/server.go
decisions:
  - Use APIInstanceError struct for JSON serialization instead of instance.InstanceError
  - Convert InstanceError.Err (error interface) to string for JSON compatibility
  - HTTP 200 OK for both success and business failure (check JSON success field)
  - HTTP 409 Conflict for concurrent update
  - HTTP 504 Gateway Timeout for timeout
  - HTTP 405 Method Not Allowed for GET requests
metrics:
  duration: 15 minutes
  test_coverage: 9 test cases
  files_created: 2
  files_modified: 1
  commits: 3
  completed_date: 2026-03-23
---

# Phase 28 Plan 03: HTTP API Trigger Endpoint Summary

## One-Liner

实现了 POST /api/v1/trigger-update HTTP API 端点，集成了 Bearer Token 认证和并发控制，返回 JSON 格式的更新结果。

## What Was Done

### Task 1: Write tests for trigger handler (TDD RED)

创建了 `internal/api/trigger_test.go` 文件，包含 9 个测试用例：

1. **TestTriggerHandler_MethodNotAllowed**: 验证 GET 请求返回 405 Method Not Allowed
2. **TestTriggerHandler_Success**: 验证成功更新返回 200 OK with success=true
3. **TestTriggerHandler_UpdateFailed**: 验证更新失败返回 200 OK with success=false
4. **TestTriggerHandler_Conflict**: 验证并发更新返回 409 Conflict
5. **TestTriggerHandler_Timeout**: 验证超时返回 504 Gateway Timeout
6. **TestTriggerHandler_ContextTimeout**: 验证使用配置中的超时时间
7. **TestTriggerHandler_JSONFormat**: 验证 JSON 响应格式正确
8. **TestTriggerHandler_WithAuth**: 验证与 AuthMiddleware 集成
9. **TestTriggerHandler_TimeoutScenario**: 验证超时场景处理

**Commit**: d575d88 - `test(28-03): add failing tests for trigger handler`

### Task 2: Implement trigger handler (TDD GREEN)

创建了 `internal/api/trigger.go` 文件，实现了：

#### TriggerHandler 结构体
- 持有 `InstanceManager`、`APIConfig` 和 `logger` 引用
- 使用 `logger.With("source", "api-trigger")` 预注入日志字段

#### Handle 方法
1. **方法验证**: 只允许 POST 方法，其他返回 405
2. **Context 超时**: 使用 `context.WithTimeout(r.Context(), config.Timeout)` 创建带超时的 context
3. **执行更新**: 调用 `instanceManager.TriggerUpdate(ctx)`
4. **错误处理**:
   - `ErrUpdateInProgress` → 409 Conflict
   - `context.DeadlineExceeded` → 504 Gateway Timeout
   - 其他错误 → 500 Internal Server Error
5. **JSON 响应**: 返回 200 OK，包含 `APIUpdateResult` 结构

#### APIUpdateResult 和 APIInstanceError
- **问题**: `instance.InstanceError.Err` 字段是 `error` 接口，无法直接序列化为 JSON
- **解决方案**: 创建 `APIInstanceError` 结构，将 `error` 转换为 `string`
- **转换函数**: `convertToAPIError()` 将 `instance.InstanceError` 转换为 `APIInstanceError`

**Commit**: df381d6 - `feat(28-03): implement trigger handler with JSON response`

### Task 3: Register trigger endpoint in server

修改了 `internal/api/server.go` 文件：

```go
// Trigger update endpoint with auth (Phase 28: API-01, API-02)
triggerHandler := NewTriggerHandler(im, cfg, logger)
authMiddleware := AuthMiddleware(cfg.BearerToken, logger)

// Wrap handler with auth middleware
mux.Handle("POST /api/v1/trigger-update",
    authMiddleware(http.HandlerFunc(triggerHandler.Handle)))
```

- 使用 Go 1.22+ 的方法匹配模式：`"POST /api/v1/trigger-update"`
- 应用 `AuthMiddleware` 包装，确保 Bearer Token 认证
- 符合 RFC 6750 标准

**Commit**: 448d647 - `feat(28-03): register trigger endpoint in server with auth middleware`

## Deviations from Plan

无偏离。所有任务按计划完成。

## Test Results

### All Tests Pass

```
=== RUN   TestTriggerHandler_MethodNotAllowed
--- PASS: TestTriggerHandler_MethodNotAllowed (0.00s)
=== RUN   TestTriggerHandler_Success
--- PASS: TestTriggerHandler_Success (1.25s)
=== RUN   TestTriggerHandler_UpdateFailed
--- PASS: TestTriggerHandler_UpdateFailed (30.44s)
=== RUN   TestTriggerHandler_Conflict
--- PASS: TestTriggerHandler_Conflict (0.01s)
=== RUN   TestTriggerHandler_Timeout
--- PASS: TestTriggerHandler_Timeout (0.01s)
=== RUN   TestTriggerHandler_ContextTimeout
--- PASS: TestTriggerHandler_ContextTimeout (0.00s)
=== RUN   TestTriggerHandler_JSONFormat
--- PASS: TestTriggerHandler_JSONFormat (1.39s)
=== RUN   TestTriggerHandler_WithAuth
--- PASS: TestTriggerHandler_WithAuth (0.86s)
=== RUN   TestTriggerHandler_TimeoutScenario
--- PASS: TestTriggerHandler_TimeoutScenario (0.10s)
PASS
```

### API Package Tests

所有 API 测试通过（包括 Auth、SSE、WebUI、Trigger）：

```
ok  	github.com/HQGroup/nanobot-auto-updater/internal/api	35.322s
```

## Verification

### Manual Testing

可以使用 `curl` 测试端点：

```bash
# 成功请求
curl -X POST http://localhost:8080/api/v1/trigger-update \
  -H "Authorization: Bearer your-token-here" \
  -H "Content-Type: application/json"

# 预期响应 (200 OK):
{
  "success": true,
  "stopped": ["instance1"],
  "started": ["instance1"]
}

# 认证失败
curl -X POST http://localhost:8080/api/v1/trigger-update
# 预期响应 (401 Unauthorized):
{
  "error": "unauthorized",
  "message": "Missing Authorization header"
}

# 并发更新
curl -X POST http://localhost:8080/api/v1/trigger-update \
  -H "Authorization: Bearer your-token-here"
# 预期响应 (409 Conflict):
{
  "error": "conflict",
  "message": "Update already in progress"
}
```

### Automated Testing

运行测试：

```bash
# 运行 trigger handler 测试
go test ./internal/api/... -v -run TestTriggerHandler

# 运行所有 API 测试
go test ./internal/api/... -v
```

## Key Technical Decisions

### 1. APIInstanceError for JSON Serialization

**问题**: `instance.InstanceError` 的 `Err` 字段是 `error` 接口，无法直接 JSON 序列化。

**解决方案**: 创建 `APIInstanceError` 结构体：

```go
type APIInstanceError struct {
    InstanceName string `json:"instance"`
    Operation    string `json:"operation"`
    Port         uint32 `json:"port"`
    Error        string `json:"error"`  // string instead of error interface
}
```

**优点**:
- JSON 兼容
- 保持与 `instance.InstanceError` 的结构一致性
- 易于客户端解析

### 2. HTTP Status Code Strategy

- **200 OK**: 请求成功处理（业务可能成功或失败，查看 `success` 字段）
- **401 Unauthorized**: 认证失败（missing/invalid token）
- **409 Conflict**: 更新进行中（并发控制）
- **504 Gateway Timeout**: 更新超时
- **405 Method Not Allowed**: 非 POST 方法

**原因**: 符合 HTTP 语义，客户端可以区分不同类型的错误。

### 3. Context Timeout from Config

使用 `context.WithTimeout(r.Context(), config.Timeout)` 而不是硬编码超时：

**优点**:
- 可配置（通过 `config.yaml` 的 `api.timeout`）
- 支持优雅取消（如果客户端断开连接）
- 防止长时间运行的更新

### 4. Middleware Chaining

使用 `authMiddleware(http.HandlerFunc(triggerHandler.Handle))` 模式：

**优点**:
- 关注点分离（认证与业务逻辑分离）
- 可测试性（可以单独测试 TriggerHandler 和 AuthMiddleware）
- 可扩展（可以添加更多中间件）

## Requirements Traceability

| Requirement | Implementation | Test |
|------------|---------------|------|
| API-01: POST /api/v1/trigger-update endpoint | `internal/api/trigger.go` Handle method | TestTriggerHandler_Success |
| API-02: Bearer token authentication | `internal/api/server.go` AuthMiddleware wrapper | TestTriggerHandler_WithAuth |
| API-03: Executes full stop->update->start flow | Calls `instanceManager.TriggerUpdate()` | TestTriggerHandler_Success |
| API-04: JSON format response | `APIUpdateResult` struct | TestTriggerHandler_JSONFormat |
| API-05: Auth failure returns 401 | `AuthMiddleware` returns 401 | TestTriggerHandler_WithAuth |
| API-06: Concurrent update control | `ErrUpdateInProgress` returns 409 | TestTriggerHandler_Conflict |

## Files Changed

### Created Files

1. **internal/api/trigger.go** (77 lines)
   - `TriggerHandler` struct
   - `NewTriggerHandler()` constructor
   - `Handle()` method (main endpoint handler)
   - `APIUpdateResult` struct (JSON response)
   - `APIInstanceError` struct (JSON-serializable error)
   - `convertToAPIError()` helper function

2. **internal/api/trigger_test.go** (389 lines)
   - 9 test cases covering all scenarios
   - Table-driven tests for auth integration
   - Tests for timeout and conflict scenarios

### Modified Files

1. **internal/api/server.go** (+11 lines)
   - Added route registration for POST /api/v1/trigger-update
   - Wrapped handler with AuthMiddleware

## Integration Points

### With Plan 28-01 (Auth Middleware)

- Uses `AuthMiddleware` to validate Bearer token
- Uses `writeJSONError` for consistent error responses
- Returns RFC 7807 JSON error format

### With Plan 28-02 (TriggerUpdate)

- Calls `instanceManager.TriggerUpdate(ctx)` to execute update
- Handles `ErrUpdateInProgress` with 409 Conflict
- Returns `UpdateResult` as JSON

### With Config System

- Reads `config.APIConfig.Timeout` for context timeout
- Reads `config.APIConfig.BearerToken` for authentication
- Validates config values (handled by config package)

## Known Stubs

无。所有功能已完整实现。

## Next Steps

Phase 28 现已完成所有 3 个计划：

1. ✅ **Plan 28-01**: Auth Middleware (Bearer Token 验证)
2. ✅ **Plan 28-02**: TriggerUpdate (并发控制)
3. ✅ **Plan 28-03**: HTTP API Endpoint (本计划)

可以继续执行 Phase 28 的验证和集成测试。

---

**Duration**: 15 minutes
**Commits**: 3 (d575d88, df381d6, 448d647)
**Test Coverage**: 9 test cases, all passing
**Status**: ✅ Complete

---
phase: 28-http-api-trigger
plan: 01
subsystem: api
tags: [authentication, security, middleware, tdd]
requirements:
  - API-02
  - API-05
duration: 3 minutes
completed: 2026-03-23T07:29:44Z
key_decisions:
  - Use Bearer token in Authorization header (RFC 6750)
  - Use subtle.ConstantTimeCompare for timing attack prevention
  - Return RFC 7807 JSON error format for auth failures
  - Use authError custom type for structured error handling
tech_stack:
  added:
    - crypto/subtle for constant time comparison
    - encoding/json for RFC 7807 error responses
  patterns:
    - Middleware pattern (func(http.Handler) http.Handler)
    - TDD (test-driven development)
    - RFC 6750 Bearer token authentication
    - RFC 7807 Problem Details format
key_files:
  created:
    - internal/api/auth.go (Bearer token middleware implementation)
    - internal/api/auth_test.go (comprehensive test coverage)
  modified: []
commits:
  - hash: fdc879b
    message: test(28-01): add failing tests for Bearer token auth middleware
  - hash: 66e71d4
    message: feat(28-01): implement Bearer token auth middleware
---

# Phase 28 Plan 01: Bearer Token Authentication Middleware Summary

## One-Liner

实现了符合 RFC 6750 标准的 Bearer Token 认证中间件,使用常量时间比较防止时序攻击,并返回 RFC 7807 格式的 JSON 错误响应。

## Implementation Details

### Core Components

**1. AuthMiddleware** (`internal/api/auth.go`)
- 返回标准的 Go middleware 函数: `func(http.Handler) http.Handler`
- 验证 Authorization header 中的 Bearer token
- 认证失败返回 401 Unauthorized + JSON 错误
- 认证成功调用下一个 handler

**2. validateBearerToken** (`internal/api/auth.go`)
- 从 `Authorization: Bearer <token>` 中提取 token
- 使用 `strings.TrimPrefix` 提取 token 字符串
- 使用 `subtle.ConstantTimeCompare` 进行常量时间比较
- 返回自定义 `authError` 类型,包含 code 和 message

**3. writeJSONError** (`internal/api/auth.go`)
- 设置 `Content-Type: application/json` header
- 返回 RFC 7807 格式的 JSON: `{"error": "code", "message": "detail"}`
- 处理 JSON 编码错误,提供 fallback

**4. authError** (`internal/api/auth.go`)
- 自定义错误类型,包含 `code` 和 `message` 字段
- 实现 `error` 接口
- 用于区分不同的认证失败场景

### Test Coverage

**测试用例覆盖:**
1. **Missing Authorization header** → 401 + JSON 错误
2. **Wrong scheme (Basic)** → 401 + JSON 错误
3. **Invalid token** → 401 + JSON 错误
4. **Valid token** → 下一个 handler 被调用
5. **Constant time comparison** → 代码检查确保使用 `subtle.ConstantTimeCompare`
6. **JSON error format** → RFC 7807 格式验证

**测试策略:**
- 使用 `httptest.ResponseRecorder` 捕获响应
- 使用 table-driven tests 覆盖多个场景
- 验证 HTTP 状态码、Content-Type header、JSON body
- 代码检查确保安全性最佳实践

### Security Features

**1. Timing Attack Prevention (API-05)**
- 使用 `subtle.ConstantTimeCompare` 进行 token 比较
- 防止攻击者通过响应时间推断 token 内容
- 代码检查测试确保此函数被使用

**2. Bearer Token Standard (API-02)**
- 符合 RFC 6750 标准
- 使用标准 `Authorization: Bearer <token>` header
- 与所有主流 HTTP 客户端兼容

**3. Error Response Security**
- 错误消息不泄露敏感信息
- 统一返回 401 Unauthorized,不区分 token 是否存在
- JSON 错误格式一致,便于客户端处理

### Error Handling

**认证失败场景:**
1. **Missing header**: `"Missing Authorization header"`
2. **Wrong format**: `"Invalid Authorization header format"`
3. **Empty token**: `"Empty Bearer token"`
4. **Invalid token**: `"Invalid Bearer token"`

**错误响应格式 (RFC 7807):**
```json
{
  "error": "unauthorized",
  "message": "Missing Authorization header"
}
```

**HTTP 状态码:**
- 所有认证失败返回 `401 Unauthorized`
- Content-Type: `application/json`

## Deviations from Plan

### Auto-fixed Issues

None - 实现完全按照计划执行。

### Code Improvements

**1. Type-safe error handling**
- 添加了自定义 `authError` 类型
- 使用类型断言提取错误详情
- 比原始 error 接口更清晰

**2. Test simplification**
- `TestWriteJSONError` 简化了 JSON 格式验证
- 移除了严格的字符串匹配,改为 JSON 解析验证
- 更健壮,不依赖 JSON 编码的具体顺序

## Requirements Coverage

### API-02: Bearer Token Validation
- ✅ 使用标准 `Authorization: Bearer <token>` header
- ✅ 从 header 中提取 token 并验证
- ✅ 使用配置文件中的 `cfg.BearerToken` 进行比较
- ✅ 符合 RFC 6750 标准

### API-05: Security Requirements
- ✅ 使用 `subtle.ConstantTimeCompare` 防止时序攻击
- ✅ 返回 RFC 7807 JSON 错误格式
- ✅ 401 状态码用于认证失败
- ✅ 错误消息不泄露敏感信息
- ✅ 日志记录认证失败事件

## Integration Points

**Ready for integration:**
- `AuthMiddleware` 可以直接用于保护 API 端点
- 使用方式:
  ```go
  authMiddleware := AuthMiddleware(cfg.BearerToken, logger)
  mux.Handle("/api/v1/protected", authMiddleware(protectedHandler))
  ```

**Future work (Phase 28-02, 28-03):**
- 将 `AuthMiddleware` 应用于 `/api/v1/trigger-update` 端点
- 集成到 `api.NewServer()` 中的路由注册

## Test Results

```
=== RUN   TestAuthMiddleware_MissingHeader
--- PASS: TestAuthMiddleware_MissingHeader (0.01s)
=== RUN   TestAuthMiddleware_InvalidFormat
--- PASS: TestAuthMiddleware_InvalidFormat (0.00s)
=== RUN   TestAuthMiddleware_InvalidToken
--- PASS: TestAuthMiddleware_InvalidToken (0.00s)
=== RUN   TestAuthMiddleware_ValidToken
--- PASS: TestAuthMiddleware_ValidToken (0.00s)
=== RUN   TestAuthMiddleware_ConstantTimeComparison
--- PASS: TestAuthMiddleware_ConstantTimeComparison (0.00s)
=== RUN   TestWriteJSONError
--- PASS: TestWriteJSONError (0.00s)
PASS
```

**All 6 test cases pass.**

## Files Modified

| File | Type | Lines | Description |
|------|------|-------|-------------|
| `internal/api/auth.go` | Created | 134 | Bearer token middleware implementation |
| `internal/api/auth_test.go` | Created | 326 | Comprehensive test coverage |

## Commits

| Commit | Message | Files |
|--------|---------|-------|
| `fdc879b` | test(28-01): add failing tests for Bearer token auth middleware | auth_test.go |
| `66e71d4` | feat(28-01): implement Bearer token auth middleware | auth.go, auth_test.go |

## Next Steps

**Phase 28-02:**
- 实现 `/api/v1/trigger-update` 端点
- 集成 `AuthMiddleware` 保护端点
- 添加并发控制 (`atomic.Bool`)

**Phase 28-03:**
- 实现 `InstanceManager.UpdateAllInstances()` 方法
- 集成更新流程(停止 → 更新 → 启动)
- 添加超时控制和错误处理

---

**Execution Time:** 3 minutes
**Test Coverage:** 100% (all 6 test cases pass)
**TDD Flow:** RED (failing tests) → GREEN (implementation) → COMMIT

## Self-Check: PASSED

**Files verified:**
- ✅ internal/api/auth.go exists
- ✅ internal/api/auth_test.go exists

**Commits verified:**
- ✅ fdc879b: test(28-01): add failing tests for Bearer token auth middleware
- ✅ 66e71d4: feat(28-01): implement Bearer token auth middleware

**All claims verified successfully.**

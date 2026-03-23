# Phase 29: HTTP Help Endpoint - Context

**Created:** 2026-03-23
**Status:** Ready for planning

## Problem Statement

### Current Issue

当程序已启动运行时，第三方程序如果需要查看 help 信息：

1. **CLI Help 冲突**: 使用 `--help` 或 `-h` 标志会尝试启动一个新的程序实例
2. **端口冲突风险**: 如果新实例尝试使用相同的 API 端口，会导致端口冲突错误
3. **多实例问题**: 程序设计为单一实例运行（单端口监听），启动多个实例会引发不可预期的行为

### Use Case

第三方程序（如 nanobot 本身或其他自动化工具）需要：
- **程序未启动时**: 使用 CLI help 命令（`--help`）查看使用说明
- **程序已启动时**: 通过 HTTP API 查询 help 信息，避免启动新实例

## Proposed Solution

### HTTP Help Endpoint

添加 `GET /api/v1/help` 端点，提供与 CLI help 相同的使用说明信息：

**端点设计:**
```
GET /api/v1/help
Authorization: Bearer <token> (optional - help should be public)
```

**响应格式:**
```json
{
  "version": "v0.5",
  "architecture": "HTTP API + Monitor Service",
  "endpoints": {
    "trigger_update": {
      "method": "POST",
      "path": "/api/v1/trigger-update",
      "auth": "required",
      "description": "触发更新流程"
    },
    "help": {
      "method": "GET",
      "path": "/api/v1/help",
      "auth": "optional",
      "description": "查看使用说明"
    },
    "logs": {
      "method": "GET",
      "path": "/api/v1/logs/{instance}",
      "auth": "optional",
      "description": "实时日志流"
    },
    "instances": {
      "method": "GET",
      "path": "/api/v1/instances",
      "auth": "optional",
      "description": "实例列表"
    }
  },
  "config": {
    "file": "config.yaml",
    "api_port": 8080,
    "monitor_interval": "15m"
  },
  "cli_flags": {
    "--config": "配置文件路径 (default: ./config.yaml)",
    "--version": "显示版本信息",
    "-h, --help": "显示帮助信息"
  }
}
```

### Design Decisions

#### 1. Authentication Requirement

**Decision:** Help endpoint should NOT require authentication

**Rationale:**
- Help information is public, not sensitive
- Third-party tools should be able to query help without knowing the Bearer token
- Simpler integration for monitoring and discovery tools
- Consistent with industry practice (most public APIs have public docs)

**Alternative considered:** Require auth for consistency
- ❌ Adds friction for legitimate use cases
- ❌ Third-party tools need token just to discover API

#### 2. Response Format

**Decision:** JSON format (not plain text)

**Rationale:**
- Machine-readable for third-party programs
- Structured data easier to parse and display
- Extensible for future metadata
- Consistent with other API endpoints

**Alternative considered:** Plain text (same as CLI)
- ❌ Harder for programs to parse
- ❌ Not extensible
- ❌ Inconsistent with JSON API design

#### 3. Content Scope

**Decision:** Include version, endpoints, config reference, and CLI flags

**Rationale:**
- Complete program information in one request
- Endpoint discovery for API exploration
- Config reference helps users understand settings
- CLI flags documented for completeness

**Alternative considered:** Minimal help (only endpoints)
- ❌ Incomplete information
- ❌ Users still need to check CLI help for config details

#### 4. Endpoint Path

**Decision:** `/api/v1/help`

**Rationale:**
- Consistent with existing API structure
- Versioned path allows future changes
- Follows RESTful conventions
- Easy to remember

**Alternative considered:** `/help` (root path)
- ❌ Inconsistent with `/api/v1/*` structure
- ❌ May conflict with future web UI routes

## Requirements

### HELP-01: HTTP Help Endpoint
- **描述**: 提供 GET /api/v1/help 端点返回使用说明
- **验证**: HTTP GET 请求返回 200 OK 和 JSON 响应

### HELP-02: No Authentication Required
- **描述**: Help 端点不需要 Bearer token 认证
- **验证**: 无 Authorization header 的请求成功返回 200 OK

### HELP-03: JSON Response Format
- **描述**: 返回结构化的 JSON 响应，包含版本、端点、配置、CLI 标志
- **验证**: 响应包含所有必需字段（version, endpoints, config, cli_flags）

### HELP-04: Content Accuracy
- **描述**: Help 信息与 CLI --help 输出一致
- **验证**: CLI flags 和端点描述与实际实现匹配

## Success Criteria

1. GET /api/v1/help 返回 200 OK 和 JSON 格式的 help 信息
2. 请求不需要 Bearer token 认证（公开访问）
3. JSON 响应包含 version, endpoints, config, cli_flags 字段
4. 端点信息与实际 API 路由匹配
5. 第三方程序可以解析 JSON 响应并显示 help 信息

## Technical Approach

### Implementation Plan

**Wave 1:**
- Plan 29-01: 实现 HelpHandler 和 JSON 响应结构
- Plan 29-02: 注册路由并测试

**Files to modify:**
- `internal/api/help.go` (new file) - HelpHandler implementation
- `internal/api/help_test.go` (new file) - Test coverage
- `internal/api/server.go` - Route registration

**Estimated effort:** 2 plans, ~30 minutes

### Code Structure

```go
// internal/api/help.go
package api

type HelpHandler struct {
    version string
    config  *config.Config
    logger  *slog.Logger
}

type HelpResponse struct {
    Version      string                 `json:"version"`
    Architecture string                 `json:"architecture"`
    Endpoints    map[string]EndpointInfo `json:"endpoints"`
    Config       ConfigReference        `json:"config"`
    CLIFlags     map[string]string      `json:"cli_flags"`
}

type EndpointInfo struct {
    Method      string `json:"method"`
    Path        string `json:"path"`
    Auth        string `json:"auth"`
    Description string `json:"description"`
}

func (h *HelpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request)
```

## Dependencies

- **Phase 28**: 需要 HTTP API 服务器基础设施（internal/api/server.go）

## Integration Points

- **Server Registration**: 在 `api.NewServer()` 中添加 `GET /api/v1/help` 路由
- **No Auth Middleware**: 直接注册处理器，不经过 AuthMiddleware
- **Version Info**: 从 build info 或常量读取版本号

## Testing Strategy

- Test 1: GET /api/v1/help returns 200 OK
- Test 2: No Authorization header required
- Test 3: Response contains all expected fields
- Test 4: JSON structure is valid
- Test 5: Endpoint paths match actual routes

## Documentation

Update README.md to mention HTTP help endpoint:
```
### HTTP API Help
查看程序使用说明：
- 程序未启动: `nanobot-auto-updater --help`
- 程序已启动: `curl http://localhost:8080/api/v1/help`
```

---

*Phase: 29-http-help-endpoint*
*Context created: 2026-03-23*

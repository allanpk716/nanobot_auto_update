# Phase 29: HTTP Help Endpoint - Research

**Researched:** 2026-03-23
**Domain:** Go HTTP API / JSON 响应 / 无认证公开端点
**Confidence:** HIGH

## Summary

Phase 29 旨在为程序添加 HTTP help 端点，让第三方程序可以在程序运行时查询使用说明，避免使用 CLI help 命令启动新实例导致的端口冲突问题。

**核心实现路径:**
- 创建 `internal/api/help.go`，实现 `HelpHandler` 结构体和 `ServeHTTP` 方法
- 在 `internal/api/server.go` 中注册 `GET /api/v1/help` 路由（无需认证中间件）
- 返回结构化 JSON 响应，包含版本、端点列表、配置参考、CLI 标志等信息
- 复用现有的 `writeJSONError` 辅助函数处理错误响应

**技术要点:**
1. **无需认证** - 直接注册处理器，不经过 `AuthMiddleware`
2. **JSON 响应** - 定义 `HelpResponse` 结构体，使用 `encoding/json` 序列化
3. **版本信息** - 从 `main.Version` 变量读取（通过构造函数注入）
4. **端点列表** - 硬编码已实现的端点信息（`/api/v1/trigger-update`, `/api/v1/help`, `/api/v1/logs/{instance}/stream`, `/api/v1/instances`, `/api/v1/instances/status`）
5. **配置参考** - 从 `config.Config` 对象提取关键字段（API port, Monitor interval, HealthCheck interval）

**Primary recommendation:** 创建独立的 `HelpHandler`，在 `NewServer` 时注入版本号和配置对象，直接注册到 `http.ServeMux` 而不使用认证中间件，返回包含完整程序信息的 JSON 响应。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
1. **端点路径**: `/api/v1/help` (GET 方法)
2. **无需认证**: Help 端点不需要 Bearer token 认证（公开访问）
3. **JSON 响应格式**: 返回结构化 JSON，包含 `version`, `architecture`, `endpoints`, `config`, `cli_flags` 字段
4. **内容范围**: 包含版本、端点列表、配置参考、CLI 标志

### Claude's Discretion
- 响应字段的详细结构设计
- 版本号的获取方式（从 `main.Version` 还是 build info）
- 端点列表的维护方式（硬编码 vs 动态发现）
- 配置字段的选择和格式

### Deferred Ideas (OUT OF SCOPE)
- 动态 API 发现机制
- OpenAPI/Swagger 规范生成
- 交互式 API 文档 UI
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| HELP-01 | 提供 GET /api/v1/help 端点返回使用说明 | HTTP handler 模式（参考 `TriggerHandler`），`http.ServeMux` 路由注册 |
| HELP-02 | Help 端点不需要 Bearer token 认证 | 直接注册到 mux 而不使用 `AuthMiddleware`（参考 `/logs/{instance}` 和 `/api/v1/instances` 的注册方式）|
| HELP-03 | 返回结构化的 JSON 响应 | 定义 `HelpResponse` 结构体，使用 `json.Encoder` 序列化（参考 `TriggerHandler.Handle` 的 JSON 响应模式）|
| HELP-04 | Help 信息与 CLI --help 输出一致 | 从 `main.go` 的 `--help` 输出和 `README.md` 提取 CLI flags 信息 |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| net/http | Go 1.24.11 | HTTP 服务器和路由 | 标准库，项目已使用 `http.ServeMux` |
| encoding/json | Go 1.24.11 | JSON 序列化 | 标准库，所有 API 响应都使用 |
| log/slog | Go 1.24.11 | 结构化日志 | 标准库，项目统一使用 `slog` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/spf13/pflag | 1.0.10 | CLI 标志定义 | 已在 `main.go` 中使用（仅用于文档化 CLI flags）|

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| 标准 `http.ServeMux` | 第三方路由器（gorilla/mux, chi）| 标准库已满足需求，项目已使用 `http.ServeMux`，无需引入新依赖 |

**Installation:** 无需安装新依赖，使用 Go 标准库和现有依赖。

**Version verification:**
```bash
$ go version
go version go1.24.13 windows/amd64

$ grep -E "github.com/spf13/pflag" go.mod
github.com/spf13/pflag v1.0.10
```

## Architecture Patterns

### Recommended Project Structure
```
internal/api/
├── help.go          # NEW - HelpHandler 实现
├── help_test.go     # NEW - 单元测试
├── server.go        # MODIFY - 注册 help 路由
├── auth.go          # 已有 - AuthMiddleware（help 不使用）
├── trigger.go       # 已有 - 参考实现模式
└── sse.go           # 已有 - 参考无需认证的端点
```

### Pattern 1: HTTP Handler 结构体模式
**What:** 使用结构体封装 handler 依赖（logger, config, version），提供 `ServeHTTP` 或 `Handle` 方法
**When to use:** 所有需要访问配置或日志的 handler（项目统一模式）
**Example:**
```go
// 来源: internal/api/trigger.go
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
    // 处理逻辑
}
```

### Pattern 2: 无需认证的端点注册
**What:** 直接将 handler 注册到 `http.ServeMux`，不使用 `AuthMiddleware` 包装
**When to use:** 公开端点（如 help, logs, instances）
**Example:**
```go
// 来源: internal/api/server.go
// 无需认证的端点（直接注册）
mux.HandleFunc("GET /api/v1/logs/{instance}/stream", sseHandler.Handle)
mux.HandleFunc("GET /logs/{instance}", web.NewWebPageHandler(im, logger))
mux.HandleFunc("GET /api/v1/instances", web.NewInstanceListHandler(im, logger))

// 需要认证的端点（使用中间件）
mux.Handle("POST /api/v1/trigger-update",
    authMiddleware(http.HandlerFunc(triggerHandler.Handle)))
```

### Pattern 3: JSON 响应格式
**What:** 设置 `Content-Type: application/json`，使用 `json.NewEncoder(w).Encode(response)` 返回 JSON
**When to use:** 所有 API 响应
**Example:**
```go
// 来源: internal/api/trigger.go
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusOK)

response := APIUpdateResult{
    Success: !result.HasErrors(),
    // ...
}

if err := json.NewEncoder(w).Encode(response); err != nil {
    h.logger.Error("Failed to encode response", "error", err)
}
```

### Anti-Patterns to Avoid
- **不要直接返回 `config.Config` 对象**: 可能包含敏感信息（如 Bearer Token），应选择性暴露配置字段
- **不要硬编码版本号**: 应从 `main.Version` 变量读取（通过构造函数注入）
- **不要在 help 响应中包含敏感信息**: Bearer token、Pushover keys 等应标记为 `"configured"` 或 `"not_configured"`

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON 错误响应 | 自定义错误格式 | `writeJSONError` (internal/api/auth.go) | 项目已统一使用 RFC 7807 格式 |
| HTTP 路由 | 手动解析路径 | `http.ServeMux` 路径模式 | 标准库支持 `{instance}` 等路径参数 |
| 日志记录 | `fmt.Println` | `log/slog.Logger` | 项目统一使用结构化日志 |

**Key insight:** 项目已有成熟的 HTTP handler 模式和辅助函数，无需重新设计 JSON 响应格式或错误处理机制。

## Runtime State Inventory

> 本阶段不涉及 rename/refactor/migration，跳过此部分。

## Common Pitfalls

### Pitfall 1: 暴露敏感配置信息
**What goes wrong:** Help 响应包含完整的 `config.Config` 对象，泄露 Bearer Token 或 Pushover keys
**Why it happens:** 直接序列化配置对象，未过滤敏感字段
**How to avoid:** 定义专用的 `ConfigReference` 结构体，仅暴露非敏感字段（如 port, interval），敏感字段标记为 `"configured": true/false`
**Warning signs:** JSON 响应中包含 `"bearer_token": "actual-token-value"`

### Pitfall 2: 版本号硬编码
**What goes wrong:** Help 响应中的版本号固定为 `"v0.5"`，不反映实际构建版本
**Why it happens:** 未从 `main.Version` 变量读取版本号
**How to avoid:** 在 `NewServer` 时注入版本号参数，传递给 `HelpHandler`
**Warning signs:** 更新版本后 help 响应仍显示旧版本号

### Pitfall 3: 端点列表与实际不同步
**What goes wrong:** Help 响应中的端点列表缺失或过时，与实际注册的路由不匹配
**Why it happens:** 硬编码端点列表后添加新端点时忘记更新
**How to avoid:** 添加新端点时同步更新 `HelpHandler` 的端点列表（或在代码审查中检查）
**Warning signs:** 实际可用的端点未出现在 help 响应中

### Pitfall 4: 错误的 HTTP 方法处理
**What goes wrong:** Help 端点接受 POST 请求或未拒绝非 GET 方法
**Why it happens:** 未在 handler 开始时检查 `r.Method`
**How to avoid:** 参考 `TriggerHandler.Handle` 的方法验证模式：
```go
if r.Method != http.MethodGet {
    writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET method is supported")
    return
}
```
**Warning signs:** POST /api/v1/help 返回 200 OK

## Code Examples

### Help Handler 实现
```go
// internal/api/help.go
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
)

// HelpHandler handles GET /api/v1/help requests
// HELP-01: HTTP endpoint for help information
// HELP-02: No authentication required (public access)
type HelpHandler struct {
	version string
	config  *config.Config
	logger  *slog.Logger
}

// NewHelpHandler creates a new help handler
func NewHelpHandler(version string, cfg *config.Config, logger *slog.Logger) *HelpHandler {
	return &HelpHandler{
		version: version,
		config:  cfg,
		logger:  logger.With("source", "api-help"),
	}
}

// ServeHTTP handles GET /api/v1/help requests
// HELP-03: Returns JSON response with version, endpoints, config, cli_flags
func (h *HelpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// HELP-01: Only GET method is supported
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET method is supported")
		return
	}

	// Build response
	response := HelpResponse{
		Version:      h.version,
		Architecture: "HTTP API + Monitor Service",
		Endpoints:    h.getEndpoints(),
		Config:       h.getConfigReference(),
		CLIFlags:     h.getCLIFlags(),
	}

	// HELP-03: Return JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode help response", "error", err)
	}
}

// HelpResponse is the JSON response structure for help endpoint
// HELP-03: Structured JSON format
type HelpResponse struct {
	Version      string                    `json:"version"`
	Architecture string                    `json:"architecture"`
	Endpoints    map[string]EndpointInfo   `json:"endpoints"`
	Config       ConfigReference           `json:"config"`
	CLIFlags     map[string]string         `json:"cli_flags"`
}

// EndpointInfo describes an API endpoint
type EndpointInfo struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Auth        string `json:"auth"`
	Description string `json:"description"`
}

// ConfigReference contains non-sensitive config information
type ConfigReference struct {
	APIPort        int    `json:"api_port"`
	MonitorInterval string `json:"monitor_interval"`
	HealthCheckInterval string `json:"health_check_interval"`
	ConfigFile     string `json:"config_file"`
}

// getEndpoints returns the list of available API endpoints
// HELP-04: Endpoint information matches actual implementation
func (h *HelpHandler) getEndpoints() map[string]EndpointInfo {
	return map[string]EndpointInfo{
		"trigger_update": {
			Method:      "POST",
			Path:        "/api/v1/trigger-update",
			Auth:        "required",
			Description: "触发更新流程（需要 Bearer Token 认证）",
		},
		"help": {
			Method:      "GET",
			Path:        "/api/v1/help",
			Auth:        "optional",
			Description: "查看使用说明和 API 端点列表",
		},
		"logs_stream": {
			Method:      "GET",
			Path:        "/api/v1/logs/{instance}/stream",
			Auth:        "optional",
			Description: "SSE 实时日志流",
		},
		"instances": {
			Method:      "GET",
			Path:        "/api/v1/instances",
			Auth:        "optional",
			Description: "实例名称列表",
		},
		"instances_status": {
			Method:      "GET",
			Path:        "/api/v1/instances/status",
			Auth:        "optional",
			Description: "实例状态列表（名称、端口、运行状态）",
		},
		"logs_ui": {
			Method:      "GET",
			Path:        "/logs/{instance}",
			Auth:        "optional",
			Description: "Web UI 日志查看器",
		},
	}
}

// getConfigReference returns non-sensitive config information
func (h *HelpHandler) getConfigReference() ConfigReference {
	return ConfigReference{
		APIPort:        h.config.API.Port,
		MonitorInterval: h.config.Monitor.Interval.String(),
		HealthCheckInterval: h.config.HealthCheck.Interval.String(),
		ConfigFile:     "./config.yaml",
	}
}

// getCLIFlags returns CLI flag documentation
// HELP-04: CLI flags match actual implementation
func (h *HelpHandler) getCLIFlags() map[string]string {
	return map[string]string{
		"--config":   "配置文件路径 (default: ./config.yaml)",
		"--version":  "显示版本信息",
		"-h, --help": "显示帮助信息",
	}
}
```

### Server 注册（修改 internal/api/server.go）
```go
// 在 NewServer 函数中添加（参考现有的 handler 注册模式）

// Create help handler (HELP-01, HELP-02)
helpHandler := NewHelpHandler(version, cfg, logger)

// Register help endpoint (no auth required)
// HELP-02: Direct registration without auth middleware
mux.Handle("GET /api/v1/help", helpHandler)
```

### 测试示例（internal/api/help_test.go）
```go
// 来源: 参考 internal/api/trigger_test.go 的测试模式
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
)

// TestHelpHandler_Success tests HELP-01, HELP-02, HELP-03:
// Returns 200 with valid JSON response without authentication
func TestHelpHandler_Success(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.Config{
		API: config.APIConfig{
			Port:        8080,
			BearerToken: "test-token",
			Timeout:     30 * time.Second,
		},
		Monitor: config.MonitorConfig{
			Interval: 15 * time.Minute,
			Timeout:  10 * time.Second,
		},
		HealthCheck: config.HealthCheckConfig{
			Interval: 1 * time.Minute,
		},
	}

	handler := NewHelpHandler("v0.5", cfg, logger)

	// Create GET request (no Authorization header)
	req := httptest.NewRequest("GET", "/api/v1/help", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify 200 OK
	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	// Verify JSON response (HELP-03)
	var response HelpResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}

	// Verify required fields (HELP-03)
	if response.Version == "" {
		t.Error("Version field is empty")
	}
	if response.Architecture == "" {
		t.Error("Architecture field is empty")
	}
	if len(response.Endpoints) == 0 {
		t.Error("Endpoints field is empty")
	}
	if response.Config.APIPort == 0 {
		t.Error("Config.APIPort is zero")
	}
	if len(response.CLIFlags) == 0 {
		t.Error("CLIFlags field is empty")
	}

	// Verify Content-Type
	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}
}

// TestHelpHandler_MethodNotAllowed tests HELP-01:
// Returns 405 for POST request
func TestHelpHandler_MethodNotAllowed(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.Config{}
	handler := NewHelpHandler("v0.5", cfg, logger)

	// Create POST request
	req := httptest.NewRequest("POST", "/api/v1/help", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify 405 Method Not Allowed
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| N/A | HTTP Help Endpoint | Phase 29 (2026-03-23) | 避免程序运行时 CLI help 命令的端口冲突 |

**Deprecated/outdated:**
- 无（本阶段为新增功能）

## Open Questions

1. **版本号注入方式**
   - What we know: `main.Version` 变量在 `main.go` 中定义，通过 ldflags 在构建时设置
   - What's unclear: 如何将版本号传递给 `api.NewServer`（需要修改 `NewServer` 函数签名）
   - Recommendation: 在 `main.go` 中调用 `api.NewServer` 时传入 `Version` 参数：
     ```go
     // main.go
     apiServer, err := api.NewServer(&cfg.API, instanceManager, Version, logger)
     ```
     ```go
     // internal/api/server.go
     func NewServer(cfg *config.APIConfig, im *instance.InstanceManager, version string, logger *slog.Logger) (*Server, error)
     ```

2. **端点列表维护**
   - What we know: 项目当前有 6 个端点（trigger-update, help, logs/stream, instances, instances/status, logs/{instance}）
   - What's unclear: 未来添加新端点时如何确保 help 响应同步更新
   - Recommendation: 在代码审查时检查新端点是否同步更新到 `HelpHandler.getEndpoints()`，或在 `CLAUDE.md` 中添加提醒规则

## Environment Availability

> 本阶段仅涉及代码修改，无外部依赖。

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.24+ | HTTP server, JSON encoding | ✓ | 1.24.13 | — |
| net/http | HTTP routing | ✓ | 标准库 | — |
| encoding/json | JSON serialization | ✓ | 标准库 | — |

**Missing dependencies with no fallback:** 无

**Missing dependencies with fallback:** 无

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify/assert |
| Config file | 无（使用 `testing` 包） |
| Quick run command | `go test ./internal/api -run TestHelp -v` |
| Full suite command | `go test ./internal/api -v` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| HELP-01 | GET /api/v1/help returns 200 OK | unit | `go test ./internal/api -run TestHelpHandler_Success -v` | ❌ Wave 0 |
| HELP-02 | No Authorization header required | unit | `go test ./internal/api -run TestHelpHandler_Success -v` | ❌ Wave 0 |
| HELP-03 | Response contains version, endpoints, config, cli_flags | unit | `go test ./internal/api -run TestHelpHandler_Success -v` | ❌ Wave 0 |
| HELP-04 | CLI flags and endpoints match actual implementation | unit | `go test ./internal/api -run TestHelpHandler_ContentAccuracy -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/api -run TestHelp -v`
- **Per wave merge:** `go test ./internal/api -v`
- **Phase gate:** Full test suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/api/help_test.go` — covers HELP-01, HELP-02, HELP-03, HELP-04
- [ ] `internal/api/help.go` — HelpHandler implementation
- [ ] `internal/api/server.go` — Route registration for GET /api/v1/help

## Sources

### Primary (HIGH confidence)
- 项目源码: `internal/api/server.go`, `internal/api/trigger.go`, `internal/api/auth.go` — HTTP handler 模式和认证中间件
- 项目源码: `internal/web/handler.go` — 无需认证的端点注册示例
- Go 标准库文档: `net/http`, `encoding/json` — 标准库用法

### Secondary (MEDIUM confidence)
- 项目源码: `cmd/nanobot-auto-updater/main.go` — CLI flags 定义和版本号变量
- 项目源码: `internal/config/config.go` — 配置结构体定义
- README.md — CLI 使用说明和端点文档

### Tertiary (LOW confidence)
- 无

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — 使用 Go 标准库，项目已有成熟的 HTTP handler 模式
- Architecture: HIGH — 参考 `TriggerHandler` 和 `web.NewInstanceListHandler` 的实现模式，无需设计新架构
- Pitfalls: HIGH — 基于项目现有代码审查和常见的 API 安全问题

**Research date:** 2026-03-23
**Valid until:** 30 days（稳定架构，Go 标准库无重大变更）

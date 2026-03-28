# Phase 32: Query API - Research

**Researched:** 2026-03-28
**Domain:** Go HTTP API / REST query endpoint with pagination
**Confidence:** HIGH

## Summary

Phase 32 需要实现两个核心功能：(1) 从 JSONL 文件恢复历史日志到内存（启动时），(2) 提供 HTTP GET 查询端点支持分页参数。代码库已经具备所有前置组件：AuthMiddleware（Phase 28）、UpdateLogger with GetAll()（Phase 30/31）、TriggerHandler 模式（Phase 28）以及 bufio.Scanner 流式读取模式（Phase 31 CleanupOldLogs）。实现范围明确，不需要引入任何新的外部依赖。

**Primary recommendation:** 严格复用现有 AuthMiddleware 和 handler 模式，在 UpdateLogger 上添加 LoadFromFile() 方法和一个支持分页参数的 GetPage() 方法，创建新的 query.go handler 文件。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** 启动时从 JSONL 文件恢复历史日志到内存，使用 bufio.Scanner 流式读取，先清理再恢复
- **D-02:** 嵌套 JSON 响应结构：data 字段包含日志列表，meta 字段包含分页元数据
- **D-03:** 最新优先排序（descending by start_time），offset=0 返回最新日志
- **D-04:** 复用 Phase 28 的 AuthMiddleware，无需新增认证逻辑
- **D-05:** 标准 HTTP 错误响应：200 OK / 401 Unauthorized / 500 Internal Server Error

### Claude's Discretion
- 启动恢复的具体实现细节（bufio 缓冲区大小、错误恢复策略）
- 分页参数校验的具体错误消息措辞
- UpdateLogger 中添加恢复方法的签名和位置
- 查询 handler 的代码组织（独立文件 vs 与 trigger handler 同文件）
- meta 字段中是否包含 page 计算（total/limit 向上取整）

### Deferred Ideas (OUT OF SCOPE)
None - 讨论保持在阶段范围内
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| QUERY-01 | 提供 HTTP GET /api/v1/update-logs 查询接口，返回 JSON 格式更新日志列表和分页元数据 | 复用 TriggerHandler 模式创建 QueryHandler，使用 Go 标准库 encoding/json 序列化嵌套结构体 |
| QUERY-02 | Bearer Token 认证保护，复用 Phase 28 AuthMiddleware | 直接复用 auth.go 中的 AuthMiddleware 和 validateBearerToken，零改动 |
| QUERY-03 | 分页参数支持：limit（默认 20，最大 100）、offset（默认 0，最小 0）、流式读取、早期终止 | 在 UpdateLogger 上实现 GetPage() 方法，基于 GetAll() 内存切片做倒序分页切片 |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| net/http (stdlib) | Go 1.24 | HTTP server and handler | Go 标准库，项目已全面使用 |
| encoding/json (stdlib) | Go 1.24 | JSON serialization | 项目已有模式，TriggerHandler 使用相同方式 |
| bufio (stdlib) | Go 1.24 | Stream reading JSONL file | Phase 31 CleanupOldLogs 已建立此模式 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| strconv (stdlib) | Go 1.24 | Parse query parameters | 解析 limit/offset 字符串参数为整数 |
| github.com/google/uuid | v1.6.0 | UUID generation | 已在 go.mod 中，但本阶段不直接使用 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| 手动解析 query params | go-playground/validator | 引入新依赖不值得，仅 2 个参数 |
| sort.Slice 倒序 | 新建倒序 slice | sort.Slice 更简洁，但在每次查询时排序有性能开销 |

**Installation:**
无需安装新依赖。所有需要的功能都来自 Go 标准库和已有依赖。

**Version verification:**
```
Go: 1.24.13 (已验证 go version)
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── api/
│   ├── server.go       # 添加 GET /api/v1/update-logs 路由
│   ├── trigger.go      # 参考模式，不修改
│   ├── query.go        # 新建：QueryHandler + 响应结构体
│   ├── query_test.go   # 新建：查询端点测试
│   ├── auth.go         # 复用 AuthMiddleware，不修改
│   └── help.go         # 添加 update_logs 端点信息
├── updatelog/
│   ├── logger.go       # 添加 LoadFromFile() 和 GetPage() 方法
│   ├── logger_test.go  # 添加新方法的测试
│   └── types.go        # 不修改
```

### Pattern 1: Handler with Middleware Wrapping
**What:** 复用 TriggerHandler 的 handler 结构和 AuthMiddleware 包装模式
**When to use:** 所有需要认证的 API 端点
**Example:**
```go
// server.go 路由注册模式（来自现有代码）
queryHandler := NewQueryHandler(updateLogger, logger)
mux.Handle("GET /api/v1/update-logs",
    authMiddleware(http.HandlerFunc(queryHandler.Handle)))
```

### Pattern 2: 内存分页切片
**What:** 从 GetAll() 获取全部日志后做倒序切片实现分页
**When to use:** 查询端点的分页实现
**Example:**
```go
// GetPage 返回分页结果（Claude's Discretion 下的推荐签名）
func (ul *UpdateLogger) GetPage(limit, offset int) ([]UpdateLog, int) {
    ul.mu.RLock()
    defer ul.mu.RUnlock()

    total := len(ul.logs)
    if offset >= total {
        return []UpdateLog{}, total
    }

    // 计算倒序索引范围
    // slice 中索引 0 = 最旧，末尾 = 最新
    // 用户想要 offset=0 返回最新
    start := total - offset - limit
    end := total - offset
    if start < 0 {
        start = 0
    }
    if end > total {
        end = total
    }

    result := make([]UpdateLog, end-start)
    copy(result, ul.logs[start:end])
    return result, total
}
```

### Pattern 3: 文件恢复（LoadFromFile）
**What:** 启动时从 JSONL 文件流式读取历史日志到内存 slice
**When to use:** 应用启动时，在 CleanupOldLogs() 之后调用
**Example:**
```go
// LoadFromFile 从 JSONL 文件恢复历史日志到内存
func (ul *UpdateLogger) LoadFromFile() error {
    if ul.filePath == "" {
        return nil // memory-only mode
    }

    if _, err := os.Stat(ul.filePath); os.IsNotExist(err) {
        return nil // file doesn't exist yet, nothing to load
    }

    f, err := os.Open(ul.filePath)
    if err != nil {
        return fmt.Errorf("failed to open file for loading: %w", err)
    }
    defer f.Close()

    scanner := bufio.NewScanner(f)
    loaded := 0
    for scanner.Scan() {
        line := scanner.Text()
        if line == "" {
            continue
        }
        var log UpdateLog
        if err := json.Unmarshal([]byte(line), &log); err != nil {
            ul.logger.Warn("Skipping invalid log line during load", "error", err)
            continue
        }
        ul.logs = append(ul.logs, log)
        loaded++
    }

    ul.logger.Info("Loaded update logs from file", "count", loaded)
    return scanner.Err()
}
```

### Anti-Patterns to Avoid
- **在 GetPage 中对 slice 排序:** 每次 API 请求都调用 sort.Slice 是 O(n log n) 的浪费。利用 slice 本身就是按时间顺序存储（旧到新）的特性，直接通过索引计算做倒序切片。
- **在 handler 中直接访问 UpdateLogger.logs:** 应该通过 GetPage() 等方法间接访问，保持封装性和线程安全。
- **分页查询走文件 I/O:** QUERY-03 提到流式读取，但根据 D-01 决策，查询走内存读取。流式读取仅在 LoadFromFile() 启动恢复时使用。
- **offset 超出范围返回 400 错误:** 应返回 200 + 空列表，这是标准 REST 分页行为（D-03 和 REQUIREMENTS 明确说明）。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 分页参数解析和校验 | 手写复杂的参数解析框架 | strconv.Atoi + 简单 if 判断 | 只有 2 个参数，不需要框架 |
| JSON 响应序列化 | 自定义 JSON writer | encoding/json + 结构体标签 | 项目已全面使用此模式 |
| Bearer Token 认证 | 新的认证逻辑 | AuthMiddleware (auth.go) | 已有完整实现，直接复用 |
| 错误响应格式 | 自定义错误格式 | writeJSONError() | 已有 RFC 7807 格式实现 |

**Key insight:** 本阶段的核心工作是"组装"现有组件，而非构建新基础设施。

## Common Pitfalls

### Pitfall 1: GetPage 倒序索引计算错误
**What goes wrong:** 倒序切片时 start/end 索引计算越界，导致 panic 或返回错误数据
**Why it happens:** 倒序遍历时的索引映射容易出错（最新在末尾，offset=0 应返回末尾元素）
**How to avoid:** 使用 total - offset 的数学关系计算索引；边界条件处理 offset + limit > total 的情况；编写充分的单元测试覆盖边界条件
**Warning signs:** offset=0, limit=10 返回空列表；offset 接近 total 时 panic

### Pitfall 2: LoadFromFile 忘记持锁
**What goes wrong:** LoadFromFile 在无锁状态下修改 ul.logs，与其他 goroutine 并发访问冲突
**Why it happens:** 启动时看起来是单线程，但 GetAll() 可能同时被调用
**How to avoid:** LoadFromFile 在应用启动流程中、HTTP 服务器启动前调用（D-01 明确说明），但方法实现仍应获取 mu.Lock() 以防未来使用场景变化
**Warning signs:** 启动后查询结果随机丢失或重复

### Pitfall 3: limit/offset 参数类型解析失败
**What goes wrong:** 用户传入非数字字符串（如 ?limit=abc），strconv.Atoi 失败未处理
**Why it happens:** 未对 URL query 参数做充分校验
**How to avoid:** Atoi 失败时使用默认值（limit=20, offset=0），而非返回 400 错误
**Warning signs:** 非 200 状态码返回给传了非法参数的客户端

### Pitfall 4: meta.total 与实际数据不一致
**What goes wrong:** meta.total 使用了分页前的总数，但数据被过滤后不匹配
**Why it happens:** 当前无过滤功能（v0.6.x 添加），但代码结构需要预留
**How to avoid:** GetPage() 返回 total（= len(logs)）和 data 分开，handler 分别用于 meta 和 data
**Warning signs:** total=42 但 data 数组有 20 条（实际上这是正确的分页行为）

### Pitfall 5: Windows bufio.Scanner 默认缓冲区限制
**What goes wrong:** bufio.Scanner 默认 64KB 缓冲区，超长 JSON 行（含大量实例详情）可能超出
**Why it happens:** 每个 UpdateLog 包含 Instances 数组，极端情况下可能很大
**How to avoid:** 如果单条日志超过 64KB，需要用 scanner.Buffer() 增大缓冲区。Phase 31 CleanupOldLogs 使用默认值，当前场景 7 天保留 + 正常实例数量下不太可能超限
**Warning signs:** LoadFromFile 报 bufio.Scanner: token too long 错误

## Code Examples

### QueryHandler 响应结构体
```go
// query.go

// UpdateLogsResponse is the JSON response structure for GET /api/v1/update-logs
type UpdateLogsResponse struct {
    Data []updatelog.UpdateLog `json:"data"`
    Meta PaginationMeta        `json:"meta"`
}

// PaginationMeta contains pagination metadata
type PaginationMeta struct {
    Total  int `json:"total"`
    Offset int `json:"offset"`
    Limit  int `json:"limit"`
}
```

### QueryHandler 实现
```go
// query.go

type QueryHandler struct {
    updateLogger *updatelog.UpdateLogger
    logger       *slog.Logger
}

func NewQueryHandler(ul *updatelog.UpdateLogger, logger *slog.Logger) *QueryHandler {
    return &QueryHandler{
        updateLogger: ul,
        logger:       logger.With("source", "api-query"),
    }
}

func (h *QueryHandler) Handle(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed",
            "Only GET method is supported")
        return
    }

    // Parse and validate pagination parameters
    limit := 20   // default
    offset := 0   // default

    if v := r.URL.Query().Get("limit"); v != "" {
        if n, err := strconv.Atoi(v); err == nil {
            if n > 0 && n <= 100 {
                limit = n
            } else if n > 100 {
                limit = 100
            }
            // n <= 0: keep default 20
        }
    }

    if v := r.URL.Query().Get("offset"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n >= 0 {
            offset = n
        }
    }

    // Get paginated data
    logs, total := h.updateLogger.GetPage(limit, offset)

    // Build response
    response := UpdateLogsResponse{
        Data: logs,
        Meta: PaginationMeta{
            Total:  total,
            Offset: offset,
            Limit:  limit,
        },
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    if err := json.NewEncoder(w).Encode(response); err != nil {
        h.logger.Error("Failed to encode query response", "error", err)
    }
}
```

### server.go 路由注册
```go
// 在 NewServer() 中，trigger-update 路由之后添加：

// Query update logs endpoint with auth (Phase 32: QUERY-01, QUERY-02)
queryHandler := NewQueryHandler(updateLogger, logger)
mux.Handle("GET /api/v1/update-logs",
    authMiddleware(http.HandlerFunc(queryHandler.Handle)))
```

### main.go 启动恢复
```go
// 在 CleanupOldLogs() 之后添加：

// Load history from JSONL file (Phase 32: D-01)
if err := updateLogger.LoadFromFile(); err != nil {
    logger.Error("Failed to load update logs from file", "error", err)
    // Non-fatal: continue with empty logs
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| 直接在 handler 中排序 | 利用 slice 顺序特性做索引切片 | Phase 32 设计决策 | 避免每次请求 O(n log n) 排序 |
| 文件 I/O 分页查询 | 内存读取 + 启动恢复 | Phase 32 D-01 | 查询性能好，启动时一次性加载 |

**Deprecated/outdated:**
- 无。本阶段使用 Go 标准库，无版本兼容性问题。

## Open Questions

1. **GetPage() 是否需要返回错误？**
   - What we know: 内存读取不会失败，但保持一致性（如 GetAll 不返回 error）
   - What's unclear: 无
   - Recommendation: GetPage() 返回 ([]UpdateLog, int)，不返回 error，与 GetAll() 模式一致

2. **limit 超过 100 或为负数时的行为？**
   - What we know: REQUIREMENTS 规定最大 100、默认 20
   - What's unclear: 传入 0 或负数时是返回错误还是用默认值
   - Recommendation: 用默认值（宽松策略），不返回 400 错误，简化客户端使用

3. **meta 中是否包含 page 字段？**
   - What we know: CONTEXT.md D-02 定义了 meta 结构（total, offset, limit）
   - What's unclear: 是否添加 page = total/limit 向上取整（Claude's Discretion 范围）
   - Recommendation: 不添加 page 字段，保持简单。offset+limit 已足够客户端计算页码

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go runtime | Build & test | Yes | 1.24.13 | - |
| net/http | HTTP server | Yes (stdlib) | 1.24 | - |
| bufio | Stream read | Yes (stdlib) | 1.24 | - |
| encoding/json | JSON serialize | Yes (stdlib) | 1.24 | - |

**Missing dependencies with no fallback:**
None - 本阶段完全依赖 Go 标准库和已有项目依赖。

**Missing dependencies with fallback:**
None

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + stretchr/testify (v1.11.1) |
| Config file | None - Go convention |
| Quick run command | `go test ./internal/api/ ./internal/updatelog/ -v -count=1` |
| Full suite command | `go test ./... -v -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| QUERY-01 | GET /api/v1/update-logs returns JSON list with pagination meta | unit | `go test ./internal/api/ -run TestQueryHandler -v` | No - Wave 0 |
| QUERY-01 | Empty results return {data: [], meta: {total: 0}} | unit | `go test ./internal/api/ -run TestQueryHandler_Empty -v` | No - Wave 0 |
| QUERY-01 | 200 OK on successful query | unit | `go test ./internal/api/ -run TestQueryHandler_Success -v` | No - Wave 0 |
| QUERY-01 | 500 on internal error (nil updateLogger) | unit | `go test ./internal/api/ -run TestQueryHandler_NilLogger -v` | No - Wave 0 |
| QUERY-02 | Auth middleware protects endpoint | unit | `go test ./internal/api/ -run TestQueryHandler_WithAuth -v` | No - Wave 0 |
| QUERY-02 | 401 without valid token | unit | `go test ./internal/api/ -run TestQueryHandler_WithAuth -v` | No - Wave 0 |
| QUERY-03 | limit parameter defaults to 20 | unit | `go test ./internal/api/ -run TestQueryHandler_DefaultLimit -v` | No - Wave 0 |
| QUERY-03 | limit capped at 100 | unit | `go test ./internal/api/ -run TestQueryHandler_LimitMax -v` | No - Wave 0 |
| QUERY-03 | offset defaults to 0 | unit | `go test ./internal/api/ -run TestQueryHandler_DefaultOffset -v` | No - Wave 0 |
| QUERY-03 | offset beyond range returns empty list | unit | `go test ./internal/api/ -run TestQueryHandler_OffsetOutOfRange -v` | No - Wave 0 |
| QUERY-03 | GetPage returns newest first | unit | `go test ./internal/updatelog/ -run TestGetPage -v` | No - Wave 0 |
| QUERY-03 | LoadFromFile restores from JSONL | unit | `go test ./internal/updatelog/ -run TestLoadFromFile -v` | No - Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/api/ ./internal/updatelog/ -v -count=1`
- **Per wave merge:** `go test ./... -v -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/api/query.go` - QueryHandler implementation
- [ ] `internal/api/query_test.go` - Query handler unit tests
- [ ] `internal/updatelog/logger.go` - GetPage() and LoadFromFile() methods
- [ ] `internal/updatelog/logger_test.go` - GetPage() and LoadFromFile() tests

## Sources

### Primary (HIGH confidence)
- 源码阅读: `internal/updatelog/logger.go` - UpdateLogger 完整实现（GetAll、Record、CleanupOldLogs、writeToFile）
- 源码阅读: `internal/api/auth.go` - AuthMiddleware 和 writeJSONError 完整实现
- 源码阅读: `internal/api/trigger.go` - TriggerHandler handler 模式参考
- 源码阅读: `internal/api/server.go` - NewServer 路由注册模式
- 源码阅读: `internal/updatelog/types.go` - UpdateLog 和 InstanceUpdateDetail 数据结构
- 源码阅读: `cmd/nanobot-auto-updater/main.go` - 启动流程和 UpdateLogger 生命周期

### Secondary (MEDIUM confidence)
- Go 标准库文档: net/http, encoding/json, bufio, strconv - Go 1.24 stdlib
- REQUIREMENTS.md QUERY-01/02/03 - 需求定义

### Tertiary (LOW confidence)
- 无低置信度来源

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - 全部使用 Go 标准库和已有依赖，已验证 go.mod 和源码
- Architecture: HIGH - 直接复用 TriggerHandler/AuthMiddleware 模式，模式已在项目验证
- Pitfalls: HIGH - 基于代码审查发现的具体陷阱，有代码级证据支持

**Research date:** 2026-03-28
**Valid until:** 2026-04-27（Go 标准库稳定，30 天有效期）

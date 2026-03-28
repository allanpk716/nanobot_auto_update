# Phase 32: Query API - Context

**Gathered:** 2026-03-28
**Status:** Ready for planning

<domain>
## Phase Boundary

用户能够通过 HTTP GET /api/v1/update-logs 查询更新历史日志，支持分页参数和 Bearer Token 认证。此阶段专注于查询端点设计、分页实现和数据恢复，不涉及日志记录（Phase 30）、文件持久化（Phase 31）和端到端集成（Phase 33）。

**核心功能:**
- GET /api/v1/update-logs 查询端点
- Bearer Token 认证（复用 Phase 28 的 AuthMiddleware）
- 分页参数：limit（默认 20，最大 100）和 offset（默认 0，最小 0）
- 嵌套 JSON 响应结构（data + meta）
- 最新优先排序（offset=0 返回最新日志）
- 启动时从 JSONL 文件恢复历史日志到内存
- 流式读取避免内存问题

**成功标准:**
1. 用户可以通过 GET /api/v1/update-logs 查询更新日志列表
2. 查询接口使用 Bearer Token 认证保护 (复用 Phase 28 的 AuthMiddleware)
3. 查询结果包含分页元数据 (总数、当前页、每页数量)
4. 用户可以通过 limit 参数控制每页数量 (默认 20,最大 100)
5. 用户可以通过 offset 参数控制分页偏移 (默认 0,最小 0)
6. 查询使用流式读取避免内存问题,offset 超出范围时返回空列表

</domain>

<decisions>
## Implementation Decisions

### 数据源和启动恢复
- **D-01:** 启动时从 JSONL 文件恢复历史日志到内存
  - UpdateLogger 初始化时检查 JSONL 文件是否存在
  - 使用 bufio.Scanner 流式读取文件，逐行 JSON 反序列化加载到内存 slice
  - 配合 Phase 31 D-06 启动时清理：先清理再恢复，确保只加载 7 天内的记录
  - 恢复过程是启动流程的一部分，在 HTTP 服务器开始接受请求前完成
  - 查询 API 走内存读取（Phase 31 D-02），性能好

### 响应 JSON 结构
- **D-02:** 嵌套结构，data 字段包含日志列表，meta 字段包含分页元数据
  - 响应格式：
    ```json
    {
      "data": [
        {
          "id": "uuid...",
          "start_time": "2026-03-28T10:00:00Z",
          "end_time": "2026-03-28T10:01:30Z",
          "duration_ms": 90000,
          "status": "success",
          "triggered_by": "api-trigger",
          "instances": [
            {
              "name": "gateway",
              "port": 18790,
              "status": "success",
              "error_message": "",
              "stop_duration_ms": 5000,
              "start_duration_ms": 12000
            }
          ]
        }
      ],
      "meta": {
        "total": 42,
        "offset": 0,
        "limit": 20
      }
    }
    ```
  - meta.total 表示符合条件的总记录数（当前为全量总数）
  - 空结果返回 `{data: [], meta: {total: 0, offset: 0, limit: 20}}`
  - 日志返回完整 UpdateLog 结构（与 Phase 30 定义的 JSON 标签一致）

### 日志排序方向
- **D-03:** 最新优先排序（descending by start_time）
  - offset=0 返回最新的日志，offset 增大返回更旧的日志
  - 实现方式：查询时将内存 slice 倒序排列，然后按 offset/limit 切片
  - 内存 slice 中日志按记录顺序存储（旧到新），查询时反向遍历
  - 分页语义：第 N 页 = 从最新开始跳过 N*limit 条后的 limit 条记录

### 认证机制
- **D-04:** 复用 Phase 28 的 AuthMiddleware，无需新增认证逻辑
  - 查询端点用 authMiddleware 包装（与 trigger-update 相同模式）
  - 使用相同的 api_token 配置
  - 认证失败返回 RFC 7807 JSON 错误格式（已有 writeJSONError 函数）

### 错误处理
- **D-05:** 标准 HTTP 错误响应
  - 200 OK: 查询成功（即使结果为空列表）
  - 401 Unauthorized: 认证失败（复用 AuthMiddleware）
  - 500 Internal Server Error: 服务器内部错误（如文件读取失败）

### Claude's Discretion
- 启动恢复的具体实现细节（bufio 缓冲区大小、错误恢复策略）
- 分页参数校验的具体错误消息措辞
- UpdateLogger 中添加恢复方法的签名和位置
- 查询 handler 的代码组织（独立文件 vs 与 trigger handler 同文件）
- meta 字段中是否包含 page 计算（total/limit 向上取整）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 32 需求
- `.planning/REQUIREMENTS.md` § Query API — QUERY-01, QUERY-02, QUERY-03 需求
- `.planning/ROADMAP.md` § Phase 32 — Query API 阶段目标和成功标准

### 前置阶段上下文
- `.planning/phases/30-log-structure-and-recording/30-CONTEXT.md` — UpdateLog 数据结构、UpdateLogger 组件
- `.planning/phases/31-file-persistence/31-CONTEXT.md` — JSONL 文件持久化、GetAll() 内存读取模式
- `.planning/phases/28-http-api-trigger/28-CONTEXT.md` — HTTP API 认证和并发控制模式

### 代码参考
- `internal/updatelog/logger.go` — UpdateLogger 组件（Record、GetAll、CleanupOldLogs）
- `internal/updatelog/types.go` — UpdateLog 和 InstanceUpdateDetail 数据结构
- `internal/api/auth.go` — AuthMiddleware 和 validateBearerToken（复用）
- `internal/api/server.go` — NewServer() 路由注册（添加新查询路由）
- `internal/api/trigger.go` — TriggerHandler 模式（参考 handler 实现风格）

### 外部标准
- RFC 6750: Bearer Token Usage — https://tools.ietf.org/html/rfc6750
- RFC 7807: Problem Details for HTTP APIs — https://tools.ietf.org/html/rfc7807

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **internal/api/auth.go**: AuthMiddleware
  - 完整的 Bearer Token 验证实现，直接复用
  - validateBearerToken() + AuthMiddleware() 可直接包装新查询 handler
  - writeJSONError() 提供统一的 JSON 错误响应
- **internal/updatelog/logger.go**: UpdateLogger
  - GetAll() 返回 []UpdateLog 的副本，可在此基础上实现分页切片
  - CleanupOldLogs() 已使用 bufio.Scanner 流式读取模式，恢复可复用相同模式
  - 已有 sync.RWMutex 保护并发读取
- **internal/api/trigger.go**: TriggerHandler
  - 参考 handler 实现风格（方法签名、错误处理、日志记录模式）
  - nil-safe updateLogger 检查模式

### Established Patterns
- **AuthMiddleware 包装**: `mux.Handle("GET /path", authMiddleware(handler))`
- **writeJSONError 统一错误**: 所有 API 错误使用 RFC 7807 格式
- **logger.With("source", "xxx") 预注入**: 上下文感知日志
- **sync.RWMutex 读多写少**: GetAll() 使用 RLock
- **bufio.Scanner 流式读取**: CleanupOldLogs 已建立此模式

### Integration Points
- **api.NewServer() 路由注册**: 添加 `GET /api/v1/update-logs` 路由
- **UpdateLogger 扩展**: 添加 LoadFromFile() 或类似方法实现启动恢复
- **UpdateLogger.GetAll()**: 分页查询的数据源（需考虑倒序 + 切片）
- **HelpHandler.getEndpoints()**: 添加 update_logs 端点信息

</code_context>

<specifics>
## Specific Ideas

- **启动时先清理再恢复** 确保只加载 7 天内的有效记录，避免加载后立即清理的浪费
- **GetAll() 基础上做分页切片** 零拷贝优化：获取 RLock 后直接在 slice 上做倒序遍历和切片，不需要复制整个 slice
- **meta.total 使用 len(logs)** 不需要额外的计数器，直接从内存 slice 长度获取总数
- **offset 超出范围返回空列表** 符合标准 REST 分页行为，不返回错误

</specifics>

<deferred>
## Deferred Ideas

None — 讨论保持在阶段范围内

</deferred>

---

*Phase: 32-query-api*
*Context gathered: 2026-03-28*

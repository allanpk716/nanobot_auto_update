# Phase 30: Log Structure and Recording - Context

**Gathered:** 2026-03-27
**Status:** Ready for planning

<domain>
## Phase Boundary

系统能够为每次 HTTP API 触发的更新操作生成唯一标识符 (UUID v4)、记录元数据(开始/结束时间戳、总耗时、整体状态)和每个实例的更新详情(名称、端口、状态、错误消息、日志引用、耗时明细)。此阶段专注于数据模型设计和记录机制实现,不涉及文件持久化(Phase 31)和查询 API(Phase 32)。

**核心功能:**
- 为每次 trigger-update 生成 UUID v4 唯一标识符
- 记录更新操作的开始时间戳和结束时间戳 (UTC, RFC 3339)
- 计算并存储总耗时 (毫秒级精度)
- 判定并存储整体状态 (success/partial_success/failed)
- 记录每个实例的详细更新结果
- 提供 LogBuffer 索引引用实例日志
- 扩展 HTTP 响应返回 update_id

**成功标准:**
1. 每次更新操作生成唯一的 UUID v4 标识符并在 trigger-update 响应中返回
2. 系统记录更新的开始时间戳、结束时间戳和整体状态 (success/partial_success/failed)
3. 系统记录每个实例的更新详情 (名称、端口、状态、错误消息)
4. 系统计算并存储从开始到结束的总耗时 (毫秒级精度)
5. 所有时间戳使用 UTC 时区存储 (RFC 3339 格式)

</domain>

<decisions>
## Implementation Decisions

### 数据结构设计
- **独立结构 (UpdateLog)**
  - 创建独立的 UpdateLog 结构包含 ID、时间戳、状态、实例详情数组
  - 与 UpdateResult 分离,职责清晰,易于扩展
  - UpdateLog 结构:
    ```go
    type UpdateLog struct {
        ID          string                  `json:"id"`           // UUID v4
        StartTime   time.Time               `json:"start_time"`   // RFC 3339, UTC
        EndTime     time.Time               `json:"end_time"`     // RFC 3339, UTC
        Duration    int64                   `json:"duration_ms"`  // 毫秒级精度
        Status      UpdateStatus            `json:"status"`       // success/partial_success/failed
        Instances   []InstanceUpdateDetail  `json:"instances"`    // 每个实例的详情
        TriggeredBy string                  `json:"triggered_by"` // "api-trigger"
    }

    type InstanceUpdateDetail struct {
        Name          string        `json:"name"`
        Port          uint32        `json:"port"`
        Status        string        `json:"status"`        // "success" or "failed"
        ErrorMessage  string        `json:"error_message"` // 如果失败
        LogStartIndex int           `json:"log_start_index"` // LogBuffer 起始索引
        LogEndIndex   int           `json:"log_end_index"`   // LogBuffer 结束索引
        StopDuration  int64         `json:"stop_duration_ms"`  // 停止耗时 (毫秒)
        StartDuration int64         `json:"start_duration_ms"` // 启动耗时 (毫秒)
    }

    type UpdateStatus string
    const (
        StatusSuccess       UpdateStatus = "success"
        StatusPartialSuccess UpdateStatus = "partial_success"
        StatusFailed        UpdateStatus = "failed"
    )
    ```

### 时间戳格式
- **RFC 3339 字符串**
  - 使用 `time.Time` 类型存储,JSON 序列化为 RFC 3339 格式
  - 格式示例: `"2026-03-27T10:30:00.123Z"`
  - 人类可读,与 JSON 序列化兼容,Go 标准库支持良好
  - 使用 `time.Now().UTC()` 确保所有时间戳为 UTC 时区
  - 使用 `time.Format(time.RFC3339Nano)` 支持毫秒级精度

### 状态定义
- **三态分类 (success/partial_success/failed)**
  - `success`: 所有实例成功启动和更新
  - `partial_success`: 部分实例成功,部分失败
  - `failed`: 所有实例失败或 UV 更新失败
  - 判定逻辑:
    ```go
    func determineStatus(result *UpdateResult) UpdateStatus {
        if result.HasErrors() {
            if len(result.Started) > 0 || len(result.Stopped) > 0 {
                return StatusPartialSuccess
            }
            return StatusFailed
        }
        return StatusSuccess
    }
    ```

### 实例详情
- **基本信息 + 日志引用 + 耗时明细**
  - 基本信息: 实例名称、端口、更新状态、错误消息 (来自 InstanceError)
  - 日志引用: LogBuffer 索引 (start_index, end_index),Phase 33 可直接定位实例日志
  - 耗时明细: 每个实例的停止耗时、启动耗时单独统计
  - 实现方式:
    - 在 InstanceLifecycle.Stop() 前后记录时间,计算 stop_duration
    - 在 InstanceLifecycle.Start() 前后记录时间,计算 start_duration
    - 从 InstanceLifecycle.GetLogBuffer() 获取当前 size 作为 LogEndIndex
    - LogStartIndex 需要在启动前记录 (Phase 33 集成时实现)

### UUID 生成时机
- **HTTP Handler 生成**
  - 在 TriggerHandler.Handle() 开始时生成 UUID v4
  - 使用 `github.com/google/uuid` 库生成 UUID v4
  - 在调用 InstanceManager.TriggerUpdate 前就确定 ID
  - 可以在错误日志中使用 UUID 关联整个更新流程
  - 示例:
    ```go
    func (h *TriggerHandler) Handle(w http.ResponseWriter, r *http.Request) {
        updateID := uuid.New().String()
        startTime := time.Now().UTC()

        // Execute update...
        result, err := h.instanceManager.TriggerUpdate(ctx)

        // Record log...
        log := UpdateLog{
            ID:        updateID,
            StartTime: startTime,
            EndTime:   time.Now().UTC(),
            // ...
        }
    }
    ```

### HTTP 响应集成
- **扩展现有响应 (APIUpdateResult)**
  - 在现有 `APIUpdateResult` 结构上添加 `update_id` 字段
  - 保持向后兼容,客户端可选择使用新字段
  - 响应格式:
    ```json
    {
      "update_id": "550e8400-e29b-41d4-a716-446655440000",
      "success": true,
      "stopped": ["gateway", "worker"],
      "started": ["gateway", "worker"],
      "stop_failed": [],
      "start_failed": []
    }
    ```

### 记录时机
- **Handler 后记录**
  - 在 TriggerHandler.Handle() 中,更新完成后调用 UpdateLogger.Record()
  - Handler 负责记录,职责清晰
  - 记录流程:
    1. 生成 UUID 和记录开始时间
    2. 调用 InstanceManager.TriggerUpdate()
    3. 记录结束时间,计算耗时
    4. 构建 UpdateLog 结构
    5. 调用 UpdateLogger.Record(log)
    6. 返回 HTTP 响应 (包含 update_id)

### 存储位置
- **内存 slice (Phase 30)**
  - 在 UpdateLogger 组件中使用 `[]UpdateLog` slice 存储日志
  - Phase 31 实现文件持久化时再写入文件
  - 简单实现,专注于数据模型和记录机制
  - 示例:
    ```go
    type UpdateLogger struct {
        logs   []UpdateLog
        mu     sync.RWMutex
        logger *slog.Logger
    }

    func (ul *UpdateLogger) Record(log UpdateLog) error {
        ul.mu.Lock()
        defer ul.mu.Unlock()
        ul.logs = append(ul.logs, log)
        return nil
    }
    ```

### 日志引用实现
- **索引引用 (LogBuffer 索引)**
  - 存储实例名和 LogBuffer 中的日志范围 (start_index, end_index)
  - Phase 33 可通过偏移量直接定位实例日志
  - 需要在 InstanceLifecycle 中添加获取当前 size 的方法
  - 性能好,不复制日志内容
  - 实现要点:
    - LogStartIndex: 实例启动前的 LogBuffer size
    - LogEndIndex: 实例启动后的 LogBuffer size
    - 查询时通过 `logBuffer.GetRange(start, end)` 获取日志

### 耗时计算
- **开始结束差值**
  - 在 UpdateLog 中记录 StartTime 和 EndTime
  - 计算总耗时: `Duration = EndTime.Sub(StartTime).Milliseconds()`
  - 实例级别耗时在 InstanceLifecycle 中单独统计
  - 使用 `time.Since(start).Milliseconds()` 计算毫秒级耗时
  - 示例:
    ```go
    startTime := time.Now().UTC()
    // ... execute update ...
    endTime := time.Now().UTC()
    duration := endTime.Sub(startTime).Milliseconds()
    ```

### 记录失败处理
- **非阻塞记录**
  - 日志记录失败不影响更新操作本身
  - 只记录 ERROR 日志,不返回错误给客户端
  - 确保更新流程稳定性
  - UpdateLogger.Record() 返回 error,Handler 调用时忽略错误
  - 示例:
    ```go
    if err := h.updateLogger.Record(log); err != nil {
        h.logger.Error("Failed to record update log", "error", err, "update_id", log.ID)
        // Don't return error to client, update operation itself was successful
    }
    ```

### 实现范围
- **最小范围 (Phase 30 专注数据模型和记录)**
  - 只实现 UpdateLog 数据结构和 UpdateLogger 组件
  - 在 Handler 中生成 UUID 和记录日志
  - 内存存储 (slice)
  - Phase 31 再实现 JSONL 文件持久化
  - Phase 32 再实现查询 API
  - Phase 33 再集成 LogBuffer 索引

### Claude's Discretion
- 日志字段的具体命名 (duration_ms vs duration_ms)
- 错误消息的中英文选择
- UpdateLogger 组件的具体实现细节
- InstanceUpdateDetail 的 JSON 标签命名
- LogBuffer 索引的获取方式 (需要 Phase 33 扩展 API)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 30 需求
- `.planning/REQUIREMENTS.md` § Core Logging — LOG-01, LOG-02, LOG-03, LOG-04 需求
- `.planning/ROADMAP.md` § Phase 30 — Log Structure and Recording 阶段目标和成功标准

### 现有架构参考
- `.planning/phases/28-http-api-trigger/28-CONTEXT.md` — HTTP API 触发更新端点和 APIUpdateResult 响应格式
- `.planning/phases/05-cli-immediate-update/05-CONTEXT.md` — CLI 立即更新的流程和错误处理
- `.planning/phases/19-log-buffer-core/19-CONTEXT.md` — LogBuffer 环形缓冲区和 GetHistory() API

### 外部标准
- RFC 4122: UUID UR Namespace — https://tools.ietf.org/html/rfc4122 (UUID v4 规范)
- RFC 3339: Date and Time on the Internet — https://tools.ietf.org/html/rfc3339 (时间戳格式)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **internal/instance/result.go**: UpdateResult 和 InstanceError 结构
  - 可复用现有的 UpdateResult 数据作为 UpdateLog 的实例详情来源
  - InstanceError 提供错误消息和实例信息
- **internal/api/trigger.go**: TriggerHandler 和 APIUpdateResult
  - 可扩展 APIUpdateResult 添加 update_id 字段
  - TriggerHandler.Handle() 是生成 UUID 和记录日志的入口点
- **internal/logbuffer/buffer.go**: LogBuffer 和 GetHistory()
  - GetHistory() 返回日志历史,Phase 33 可扩展 GetRange(start, end) 方法
  - 当前 size 可作为日志索引范围
- **internal/instance/manager.go**: InstanceManager.TriggerUpdate()
  - 返回 UpdateResult,可从中提取实例详情
- **internal/instance/lifecycle.go**: InstanceLifecycle
  - 提供 Stop(), Start() 方法,可在调用前后记录耗时
  - 持有 LogBuffer 实例,Phase 33 可添加获取当前 size 的方法

### Established Patterns
- **time.Now().UTC()**: 确保所有时间戳为 UTC 时区
- **time.Sub().Milliseconds()**: 计算毫秒级耗时
- **UUID v4 生成**: 使用 `github.com/google/uuid` 库
- **上下文感知日志**: 使用 `logger.With("update_id", id)` 预注入字段
- **非阻塞错误处理**: 记录失败不影响主流程
- **slice 存储**: 简单的内存存储模式

### Integration Points
- **TriggerHandler.Handle()**: 生成 UUID、记录开始时间、调用 TriggerUpdate、记录日志、返回响应
- **APIUpdateResponse**: 添加 update_id 字段
- **UpdateLogger 组件**: 新建 internal/updatelog/logger.go,提供 Record() 方法
- **InstanceUpdateDetail 构建**: 从 UpdateResult 和 InstanceLifecycle 提取信息
- **LogBuffer 索引获取**: Phase 33 需要扩展 LogBuffer API

</code_context>

<specifics>
## Specific Ideas

- **独立数据结构** 确保职责分离,UpdateLog 专注于审计日志,UpdateResult 专注于更新结果
- **RFC 3339 时间戳** 提供人类可读的时间格式,便于调试和日志分析
- **三态状态分类** 清晰区分完全成功、部分成功和完全失败的场景
- **LogBuffer 索引引用** 避免复制大量日志内容,节省内存,提高性能
- **实例耗时明细** 支持性能分析,识别启动慢或停止慢的实例
- **非阻塞记录** 确保日志记录失败不影响更新操作本身,系统稳定优先
- **最小实现范围** 专注于 Phase 30 的核心目标,为后续阶段打好基础

</specifics>

<deferred>
## Deferred Ideas

None — 讨论保持在阶段范围内

</deferred>

---

*Phase: 30-log-structure-and-recording*
*Context gathered: 2026-03-27*

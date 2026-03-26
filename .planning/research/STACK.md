# Stack Research

**Domain:** 更新日志记录和查询系统 (Update Log Recording and Query System)
**Researched:** 2026-03-26
**Confidence:** HIGH (基于 Go 标准库和成熟生态实践)

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| **Go 标准库 encoding/json** | Go 1.24+ | JSON Lines 读写 | 无需额外依赖,性能足够,支持流式解码器避免内存问题。Go 1.24+ 已优化 JSON 性能。 |
| **Go 标准库 bufio** | Go 1.24+ | 文件流式读取 | 高性能缓冲读取,Scanner 自动处理行边界,支持大文件分页查询而不加载全部到内存。 |
| **Go 标准库 os** | Go 1.24+ | 文件追加写入 | `os.OpenFile` + `os.O_APPEND\|os.O_CREATE\|os.O_WRONLY` 实现原子追加,避免竞争条件。 |
| **Go 标准库 time** | Go 1.24+ | 时间戳和保留策略 | 标准时间处理,配合 `time.Since()` 实现基于时间的清理逻辑。 |
| **github.com/google/uuid** | v1.6+ | 唯一更新ID生成 | 业界标准,符合 RFC 9562,生成 UUID v4 作为更新操作唯一标识符。 |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| **os.OpenFile** | Go 1.24+ | JSONL 文件追加写入 | 每次更新操作完成时追加一行 JSON 记录。使用 `os.O_APPEND\|os.O_CREATE\|os.O_WRONLY, 0644` 确保原子追加。 |
| **bufio.Scanner** | Go 1.24+ | JSONL 流式分页读取 | 查询接口读取文件时,逐行扫描到指定 offset,避免加载整个文件到内存。对于大行(>64KB)需要增加缓冲区大小。 |
| **json.Decoder** | Go 1.24+ | JSON Lines 流式解码 | 配合 bufio.Scanner 逐行解码 JSON 对象,避免内存爆炸。比 `json.Unmarshal` 更高效。 |
| **json.Encoder** | Go 1.24+ | JSON Lines 流式编码 | 写入日志时使用 `json.NewEncoder(file).Encode(record)` 确保每行一个完整 JSON 对象。 |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| **Go 1.24+ 内置工具** | 编译、测试、性能分析 | 无需额外工具链,标准 `go test -bench` 即可验证 JSONL 性能。 |
| **stdlib log/slog** | 调试日志 | 复用现有 logger (已集成 `github.com/WQGroup/logger`),为更新日志系统添加组件标识。 |

## Installation

```bash
# 唯一新增依赖: UUID 生成库
go get github.com/google/uuid@v1.6.0

# 其他功能全部使用 Go 标准库,无需额外安装
# - encoding/json (JSON Lines 处理)
# - bufio (流式文件读取)
# - os (文件追加写入)
# - time (时间处理和保留策略)
```

## Integration with Existing Stack

### 现有栈复用

| 现有组件 | 复用方式 | 理由 |
|---------|---------|------|
| **internal/logging** | 复用现有 `./logs` 目录结构 | 统一日志管理,更新日志文件可放在 `./logs/updates.jsonl`,无需额外配置。 |
| **internal/config** | 扩展 Config 结构体 | 在现有 `config.Config` 添加 `UpdateLogConfig` 子配置,包含文件路径和保留天数。 |
| **internal/api/auth** | 复用 Bearer Token 认证 | 查询 API `/api/v1/update-logs` 使用与 `trigger-update` 相同的认证中间件。 |
| **slog.Logger** | 复用现有 logger 实例 | 更新日志系统内部错误记录到主日志,使用 `logger.With("component", "updatelog")` 标识。 |

### 集成点说明

**1. Trigger Handler 集成 (internal/api/trigger.go)**
```go
// 在 TriggerHandler.Handle 中添加日志记录调用
result, err := h.instanceManager.TriggerUpdate(ctx)
if err == nil || result != nil {
    // 记录更新日志(成功或部分失败都记录)
    h.updateLogger.Record(ctx, result, r.RemoteAddr)
}
```

**2. 文件路径策略**
- 主日志文件: `./logs/app-YYYY-MM-DD.log` (现有)
- 更新日志: `./logs/updates.jsonl` (新增,单一文件持续追加)
- 理由: 单文件便于分页查询,通过 ModTime 实现基于时间的清理。

**3. 清理策略集成**
```go
// 在应用启动时或定期调度器中执行清理
func cleanupOldLogs(logPath string, retentionDays int) error {
    // 读取文件,过滤保留最近 N 天的记录
    // 使用临时文件 + rename 确保原子性
}
```

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| **第三方 JSONL 库** (如 jsonl-go) | 标准库已足够,引入不必要依赖 | `encoding/json` + `bufio.Scanner` |
| **数据库** (SQLite/BoltDB) | JSONL 文件满足需求,部署简单,无需 schema 迁移 | JSONL 文件 + 分页读取 |
| **json.Unmarshal (全量加载)** | 大文件导致内存问题 | `json.Decoder` 流式解码 |
| **os.WriteFile** | 不支持原子追加,每次重写整个文件 | `os.OpenFile` + `os.O_APPEND` |
| **ioutil.ReadFile (已废弃)** | Go 1.16+ 废弃,性能差 | `os.Open` + `bufio.Scanner` |
| **UUID v7** | 时间排序依赖外部库,项目需求无需 DB 索引优化 | UUID v4 (`github.com/google/uuid`) |
| **第三方清理库** (如 dir-janitor) | 逻辑简单,自行实现更灵活 | 自实现清理函数 (见 ARCHITECTURE.md) |

## Stack Patterns by Variant

**If 文件大小 < 10MB (预估 7 天内):**
- 使用单文件 `updates.jsonl`,直接追加写入
- 分页查询时全量扫描到 offset (性能可接受)
- Because: 实现简单,无并发竞争问题

**If 文件大小 > 10MB (超过 7 天未清理):**
- 定期清理任务每天运行,删除 7 天前的记录
- 考虑按日期分片 (updates-2026-03-26.jsonl),每天一个文件
- Because: 单文件过大影响查询性能,分片按日期自然对齐清理逻辑

**If 并发写入压力大 (>100 req/s):**
- 当前设计: `os.O_APPEND` 已保证写入原子性 (操作系统保证)
- 无需额外锁机制 (文件系统层面的追加已经是原子的)
- Because: Go 的文件追加在 Windows/Linux 都是原子操作

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| github.com/google/uuid@v1.6.0 | Go 1.24+ | 无 CGO 依赖,纯 Go 实现,兼容 Windows。 |
| encoding/json (stdlib) | Go 1.24+ | Go 1.24 已包含性能优化,无需 json v2 (实验性)。 |
| bufio.Scanner (stdlib) | Go 1.24+ | 默认 64KB 缓冲区,对于大 JSON 行需手动调整 `bufio.Scanner.Buffer()`。 |
| os.OpenFile (stdlib) | Go 1.24+ | Windows 平台追加写入需使用 `os.O_APPEND`,已验证兼容。 |

## Performance Considerations

### JSONL 文件性能

| 场景 | 预估性能 | 优化策略 |
|------|---------|---------|
| **写入** (追加单行) | ~0.1ms/次 | 使用 `json.Encoder` 直接编码,无需序列化到内存。 |
| **查询** (1000 条,offset=500) | ~5-10ms | bufio.Scanner 逐行跳过前 500 行,性能足够。 |
| **清理** (删除旧记录) | ~50-100ms (10MB 文件) | 使用临时文件 + rename,避免阻塞读写。 |

### 内存占用

| 场景 | 内存占用 | 说明 |
|------|---------|------|
| **写入** | < 1KB | 单个 LogRecord 序列化到缓冲区。 |
| **查询** (limit=50) | < 100KB | 最多缓存 50 条记录返回给客户端。 |
| **清理** | ~文件大小 | 需要临时加载文件到内存过滤,10MB 文件可接受。 |

### 并发控制

| 操作 | 并发策略 | 说明 |
|------|---------|------|
| **写入** | 操作系统保证原子性 | `os.O_APPEND` 写入是原子的,无需应用层锁。 |
| **读取** | 无锁读取 | 文件系统允许多进程并发读。 |
| **清理** | 排他执行 | 使用 `sync.Mutex` 或调度器确保清理期间无写入。 |

## Sources

- **Go 标准库文档** — `encoding/json`, `bufio`, `os` 官方文档 (HIGH confidence)
- **[JSON Lines 格式规范](https://jsonlines.org/)** — 官方格式定义和最佳实践 (HIGH confidence)
- **[How to append consecutively to a JSON file in Go?](https://stackoverflow.com/questions/72456088)** — Stack Overflow 社区讨论 JSONL 追加写入 (MEDIUM confidence)
- **[os: document that WriteFile is not atomic](https://github.com/golang/go/issues/56173)** — Go 官方 Issue 说明原子性注意事项 (HIGH confidence)
- **[Which UUID package do you use?](https://www.reddit.com/r/golang/comments/10bg0rn/)** — Reddit 社区讨论 UUID 库选择 (MEDIUM confidence)
- **[github.com/google/uuid](https://github.com/google/uuid)** — Google UUID 库官方文档和示例 (HIGH confidence)
- **[Optimize JSON Serialization in Go with sonic](https://oneuptime.com/blog/optimize-json-serialization-in-go-with-sonic-or-easyjson)** — JSON 性能优化指南,确认标准库已足够 (MEDIUM confidence)
- **现有代码库分析** — `internal/logging`, `internal/api/trigger.go`, `internal/instance/result.go` 集成点识别 (HIGH confidence)

---
*Stack research for: 更新日志记录和查询系统*
*Researched: 2026-03-26*

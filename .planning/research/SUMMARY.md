# Project Research Summary

**Project:** Nanobot Auto-Updater v0.6 - Update Log Recording and Query System
**Domain:** HTTP API integration for persistent update logs with JSON Lines storage
**Researched:** 2026-03-26
**Confidence:** HIGH

## Executive Summary

这是一个为现有的 nanobot 自动更新器添加持久化更新日志记录和查询功能的系统。基于研究,我们推荐使用 Go 标准库实现 JSON Lines 格式的日志存储,通过文件追加写入实现高性能记录,并使用流式读取提供分页查询 API。

该系统的核心设计原则是**简单性和可靠性优先**:避免引入数据库依赖,复用现有认证中间件,使用经过验证的标准库模式。主要风险来自并发写入冲突、磁盘空间耗尽和查询性能下降,这些可以通过 mutex 保护、7 天保留策略和 bufio.Scanner 流式读取来有效缓解。

关键架构决策:在 `internal/updatelog` 包中创建独立的领域模型,在 `TriggerHandler` 中集成日志记录,通过懒加载方式从现有 LogBuffer 捕获实例输出。这确保了最小化对现有代码的修改,同时提供完整的审计追踪能力。

## Key Findings

### Recommended Stack

推荐使用 Go 1.24+ 标准库作为核心栈,唯一新增依赖是 `github.com/google/uuid@v1.6.0` 用于生成唯一更新 ID。标准库的 `encoding/json` + `bufio` + `os` 组合足以处理 JSON Lines 的读写需求,无需第三方库。

**Core technologies:**
- **Go 标准库 encoding/json** — JSON Lines 读写 — 无需额外依赖,性能足够,支持流式解码器避免内存问题
- **Go 标准库 bufio.Scanner** — 文件流式读取 — 高性能缓冲读取,支持大文件分页查询而不加载全部到内存
- **Go 标准库 os.OpenFile** — 文件追加写入 — `os.O_APPEND|os.O_CREATE|os.O_WRONLY` 实现原子追加,避免竞争条件
- **github.com/google/uuid v1.6+** — 唯一 ID 生成 — 业界标准,符合 RFC 9562,生成 UUID v4 作为更新操作唯一标识符

**复用现有组件:**
- **internal/api/auth** — Bearer Token 认证 — 复用现有认证中间件保护查询 API
- **internal/logging** — 日志目录结构 — 复用 `./logs` 目录,新增 `updates.jsonl` 文件
- **internal/logbuffer** — 实例日志捕获 — 从现有 ring buffer 懒加载捕获更新过程中的输出

### Expected Features

基于研究,更新日志系统需要实现记录、持久化和查询三大核心能力。v0.6 版本聚焦于基础功能,高级过滤和搜索能力推迟到后续版本。

**Must have (table stakes):**
- **Update execution logging** — 记录每次更新的元数据(开始/结束时间、实例列表、成功/失败状态)
- **JSON Lines file persistence** — 使用标准格式持久化到 `./logs/updates.jsonl` 文件
- **GET /api/v1/update-logs endpoint** — 提供带分页的查询 API
- **Bearer Token authentication** — 复用现有认证机制保护查询接口
- **7-day log cleanup** — 基于时间的自动清理,防止磁盘空间耗尽
- **Request ID tracking** — UUID v4 作为每次更新的唯一标识符

**Should have (competitive):**
- **Instance-level detail** — 每个实例的成功/失败状态、错误消息、stdout/stderr 引用
- **Pagination (limit/offset)** — 标准分页参数,每页默认 20 条记录
- **Filter by status** — 查询参数过滤成功/失败的更新记录(v0.6.x)
- **Duration tracking** — 计算并存储每次更新的耗时

**Defer (v2+):**
- **Full-text search** — 搜索日志内容需要索引支持,增加复杂度
- **Log export formats** — CSV/Excel 导出可以由客户端从 JSON 转换
- **Log analytics** — 统计成功率、耗时趋势等高级分析
- **Multi-file log storage** — 按日期分片日志文件(单文件在 7 天保留期内性能可接受)

### Architecture Approach

推荐在现有系统中新增 `internal/updatelog` 包,采用领域驱动设计原则,将日志记录、持久化和查询逻辑封装在独立组件中。主要集成点是 `TriggerHandler.Handle()` 方法,在更新操作完成后调用 recorder 记录结果,并通过 store 持久化到 JSONL 文件。

**Major components:**
1. **UpdateLogRecorder (internal/updatelog/recorder.go)** — 将 `instance.UpdateResult` 转换为 `UpdateLogEntry`,从 LogBuffer 提取实例日志,构建完整的审计记录
2. **UpdateLogStore (internal/updatelog/store.go)** — 负责 JSONL 文件的追加写入和分页读取,使用 `bufio.Scanner` 实现内存高效查询
3. **UpdateLogHandler (internal/api/updatelog_handler.go)** — HTTP 处理器,解析查询参数,调用 store 返回分页结果,受 AuthMiddleware 保护
4. **CleanupManager (internal/updatelog/cleanup.go)** — 实现基于时间的保留策略,在应用启动和定期调度中删除 7 天前的记录

**数据流:**
```
HTTP POST /api/v1/trigger-update
  → TriggerHandler 执行更新
  → UpdateLogRecorder.Record() 构建 UpdateLogEntry
  → UpdateLogStore.Append() 追加到 updates.jsonl
  → 返回 JSON 响应

HTTP GET /api/v1/update-logs?limit=20&offset=0
  → AuthMiddleware 验证 Bearer Token
  → UpdateLogHandler 解析参数
  → UpdateLogStore.Query() 使用 bufio.Scanner 流式读取
  → 返回 JSON 响应(logs + pagination metadata)
```

### Critical Pitfalls

研究识别了 7 个关键陷阱,其中并发写入冲突和查询性能问题是最常见的。这些陷阱可以通过正确使用同步机制和流式 I/O 来避免。

**Top pitfalls with prevention strategies:**

1. **Concurrent Write Conflicts (并发写入冲突)** — 多个 HTTP 请求并发触发更新时,多个 goroutine 同时写入 JSONL 文件导致数据损坏
   - **预防:** 使用 `sync.Mutex` 序列化文件写入,确保同一时间只有一个 goroutine 执行追加操作
   - **阶段:** Phase 1 (Log Structure)

2. **Query Performance Issues (查询性能问题)** — 使用 `os.ReadFile()` 加载整个文件导致内存爆炸和超时
   - **预防:** 使用 `bufio.Scanner` 逐行读取,实现早期终止(读取到 limit 后停止),避免全量加载
   - **阶段:** Phase 3 (Query API)

3. **Disk Space Exhaustion (磁盘空间耗尽)** — 日志文件无限增长导致系统崩溃
   - **预防:** 实现基于时间的保留策略(7 天),在应用启动和每日定时任务中执行清理
   - **阶段:** Phase 2 (File Persistence)

4. **File Descriptor Leaks (文件描述符泄漏)** — 忘记关闭文件句柄导致 "too many open files" 错误
   - **预防:** 总是使用 `defer file.Close()` 确保文件在所有代码路径(包括错误路径)中关闭
   - **阶段:** Phase 1 (Log Structure)

5. **Time Zone Handling (时区处理)** — 使用本地时间导致 DST(夏令时)切换时出现重复或缺失的时间戳
   - **预防:** 始终使用 UTC 存储时间戳,仅在显示时转换为用户时区
   - **阶段:** Phase 1 (Log Structure)

## Implications for Roadmap

基于研究的依赖关系分析和风险优先级,建议将实现分为 4 个阶段,每个阶段都有明确的交付物和验证点。

### Phase 1: Log Structure and Recording (日志结构和记录)

**Rationale:** 首先定义数据模型和记录逻辑,这是整个系统的基础。根据架构研究,UpdateLogEntry 结构决定了后续持久化和查询的实现方式。独立包设计允许并行开发和隔离测试。

**Delivers:**
- `internal/updatelog/types.go` — 定义 `UpdateLogEntry`, `InstanceUpdateResult` 结构体
- `internal/updatelog/recorder.go` — 实现 `UpdateLogRecorder.Record()` 转换逻辑
- 单元测试验证数据转换正确性

**Addresses:**
- Update execution logging (记录元数据)
- Request ID tracking (UUID 生成)
- Instance-level detail (实例级详情)
- Duration tracking (耗时计算)

**Avoids:**
- Time Zone Handling (使用 UTC 时间戳)
- Ring Buffer Data Race (从现有 LogBuffer 安全读取)

**Build tasks:**
1. 创建 `internal/updatelog` 包结构
2. 定义 JSON 序列化标签,确保与 JSONL 格式兼容
3. 实现 `generateUpdateID()` 使用 `uuid.New().String()`
4. 实现 `Record()` 方法从 `instance.UpdateResult` 提取数据
5. 编写单元测试覆盖成功、部分失败、完全失败场景

### Phase 2: File Persistence (文件持久化)

**Rationale:** 持久化层依赖于 Phase 1 定义的数据结构。根据 Stack 研究,使用标准库的 `os.OpenFile` + `os.O_APPEND` 实现原子追加,配合 `sync.Mutex` 防止并发写入冲突。

**Delivers:**
- `internal/updatelog/store.go` — 实现 `UpdateLogStore.Append()` 和文件管理
- `internal/updatelog/cleanup.go` — 实现 7 天保留策略清理逻辑
- 集成到 `internal/config/config.go` — 添加 `UpdateLogConfig` 配置项

**Addresses:**
- JSON Lines file persistence (持久化到文件)
- 7-day log cleanup (自动清理)
- Log file path configuration (配置文件路径)

**Avoids:**
- Concurrent Write Conflicts (使用 mutex 保护)
- File Descriptor Leaks (使用 defer Close)
- Disk Space Exhaustion (实现清理逻辑)

**Build tasks:**
1. 实现 `Append()` 方法使用 `os.OpenFile(os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)`
2. 添加 `sync.Mutex` 到 `UpdateLogStore` 结构体,在 `Append()` 中加锁
3. 使用 `json.NewEncoder(file).Encode(entry)` 写入 JSON 行
4. 实现 `CleanupOldLogs()` 函数读取文件、过滤旧记录、写入临时文件、rename 原子替换
5. 在应用启动时调用清理,并设置 24 小时定时器
6. 添加配置项: `retention_days: 7`, `log_path: "./logs/updates.jsonl"`

### Phase 3: Query API (查询接口)

**Rationale:** 查询 API 依赖于 Phase 2 的持久化实现。根据架构研究,使用 `bufio.Scanner` 实现流式读取,避免内存问题。此阶段还集成认证中间件复用现有安全机制。

**Delivers:**
- `internal/updatelog/store.go` — 添加 `Query(limit, offset int)` 方法
- `internal/api/updatelog_handler.go` — HTTP 处理器
- `internal/api/server.go` — 注册路由 `GET /api/v1/update-logs`

**Addresses:**
- GET /api/v1/update-logs endpoint (查询 API)
- Pagination support (limit/offset)
- Bearer Token authentication (复用现有中间件)

**Avoids:**
- Query Performance Issues (使用 bufio.Scanner 流式读取)
- Pagination Edge Cases (验证 offset 超出范围返回空数组,不是错误)

**Build tasks:**
1. 实现 `Query()` 方法使用 `bufio.NewScanner(file)` 逐行读取
2. 在循环中跳过前 `offset` 行,读取 `limit` 行后提前终止
3. 处理 JSON 解析错误时记录日志并跳过该行(容错处理)
4. 创建 `UpdateLogHandler` 解析查询参数并调用 `store.Query()`
5. 在 `server.go` 中注册路由并应用 `AuthMiddleware`
6. 返回 JSON 响应包含 `logs` 数组和 `pagination` 元数据

### Phase 4: Integration and Testing (集成和测试)

**Rationale:** 最后阶段将日志记录集成到现有 `trigger-update` 流程中。根据架构研究,集成点是 `TriggerHandler.Handle()` 方法,在更新完成后调用 recorder 和 store。此阶段还包括端到端测试验证整个系统。

**Delivers:**
- 修改 `internal/api/trigger.go` — 集成日志记录
- 端到端测试 — 触发更新 → 查询日志 → 验证结果
- 性能测试 — 1000+ 条记录的查询响应时间 < 500ms

**Addresses:**
- Integration with existing trigger-update flow (集成到现有流程)
- End-to-end validation (端到端验证)

**Avoids:**
- JSON Lines Corruption (容错读取,跳过损坏行)
- Integration Gotchas (在更新完成后记录,不阻塞主流程)

**Build tasks:**
1. 在 `TriggerHandler` 结构体中添加 `updateLogRecorder` 和 `updateLogStore` 字段
2. 在 `Handle()` 方法中,在 `TriggerUpdate()` 返回后调用 `recorder.Record()`
3. 从 `instanceManager` 获取各实例的 LogBuffer 并提取历史日志
4. 调用 `store.Append()` 持久化,失败时记录错误但不影响更新响应
5. 编写集成测试: POST /api/v1/trigger-update → GET /api/v1/update-logs
6. 性能测试: 生成 1000 条记录,验证查询响应时间和内存占用

### Phase Ordering Rationale

**依赖关系:**
- Phase 2 依赖 Phase 1: 持久化需要先定义数据结构
- Phase 3 依赖 Phase 2: 查询 API 需要可用的存储实现
- Phase 4 依赖所有前置阶段: 集成需要完整的功能链路

**风险驱动:**
- Phase 1 最早实现,避免时区处理陷阱(使用 UTC)
- Phase 2 紧随其后,防止磁盘空间耗尽(实现清理)
- Phase 3 解决查询性能问题(流式读取)
- Phase 4 最后集成,确保错误不影响主流程(非阻塞记录)

**分组原则:**
- Phase 1+2 聚焦数据层(结构定义 + 持久化)
- Phase 3 聚焦接口层(HTTP API)
- Phase 4 聚焦集成层(连接现有系统)

### Research Flags

**Phases likely needing deeper research during planning:**

- **Phase 2 (File Persistence):** 文件清理的原子性实现需要研究临时文件 + rename 模式在 Windows 平台的兼容性。虽然有标准模式,但需要验证在高并发场景下的行为。
  - 建议: 在此阶段开始前,使用 `/gsd:research-phase` 深入研究 Windows 文件锁和原子 rename 的最佳实践

- **Phase 4 (Integration):** 从 LogBuffer 提取实例日志时,需要研究现有 `internal/logbuffer` 包的 API 是否支持快照读取,避免在读取过程中数据被覆盖。
  - 建议: 在集成前,使用 `/gsd:research-phase` 分析现有 LogBuffer 实现,确定是否需要添加快照功能

**Phases with standard patterns (skip research-phase):**

- **Phase 1 (Log Structure):** 数据结构定义和转换逻辑是标准 Go 编程,无需额外研究。UUID 生成库已选定,JSON 序列化使用标准库即可。

- **Phase 3 (Query API):** HTTP handler、查询参数解析、JSON 响应都是标准 REST 模式,现有代码库已有类似实现(trigger-update handler)。

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | 基于官方 Go 标准库文档和成熟的 JSONL 社区实践。UUID 库是 Google 官方维护,广泛使用。 |
| Features | HIGH | 功能列表基于日志管理最佳实践和项目需求,参考了 Kafka(7 天保留)、PostgreSQL(audit log)等成熟系统。 |
| Architecture | HIGH | 架构设计遵循领域驱动设计原则,集成点明确,复用现有组件。基于现有代码库分析确定可行性。 |
| Pitfalls | HIGH | 陷阱研究基于 Go 社区讨论、Stack Overflow 高票答案、GitHub Issues 和官方文档,来源可靠。 |

**Overall confidence:** HIGH

所有研究均基于高质量来源(官方文档、成熟开源项目、社区共识),技术选型保守(标准库优先),架构设计符合现有系统模式。主要不确定性在于 Phase 2 的文件清理原子性和 Phase 4 的 LogBuffer 集成,已标记为需要额外研究的区域。

### Gaps to Address

**已知缺口及处理方式:**

1. **文件清理的原子性实现:**
   - **缺口:** 在 Windows 平台上,临时文件 + rename 模式是否与 Linux 一致?高并发场景下清理过程是否阻塞写入?
   - **处理:** 在 Phase 2 实现时,先在 Windows 环境下测试原子 rename 行为。如果发现问题,考虑使用排他锁(`sync.Mutex`)暂停写入期间执行清理。

2. **LogBuffer 快照读取:**
   - **缺口:** 现有 `internal/logbuffer` 包是否支持原子快照读取?在读取 5000 行历史记录时,是否有并发写入导致数据不一致的风险?
   - **处理:** 在 Phase 4 集成前,使用 Context7 查询 `internal/logbuffer` 实现。如果不支持快照,考虑添加 `GetSnapshot()` 方法或使用现有的 `GetHistory()` 方法时短暂持有读锁。

3. **JSONL 文件大小预估:**
   - **缺口:** 7 天内的日志文件会增长到多大?是否需要更激进的清理策略或按日期分片?
   - **处理:** 在 Phase 2 实现后,收集实际数据:每次更新记录的平均大小(预估 2-5KB),每天更新频率(预估 10-50 次)。如果单文件超过 10MB,考虑在 v0.6.x 中实现按日期分片。

4. **查询性能基准:**
   - **缺口:** 1000 条记录的查询响应时间能否稳定在 <500ms?bufio.Scanner 的缓冲区大小是否需要调整?
   - **处理:** 在 Phase 3 实现后,使用 `go test -bench` 建立性能基准。如果响应时间超过目标,增加 `bufio.Scanner.Buffer()` 大小(默认 64KB,可调整到 1MB)。

**开放问题(需用户输入):**

1. **日志保留策略:** 研究基于 PROJECT.md 中的 7 天要求,但如果实际使用中日志量较大,是否接受更短的保留期(3-5 天)?还是优先考虑增加存储空间?

2. **查询排序顺序:** 查询 API 默认返回 newest-first(倒序)还是 oldest-first(正序)?研究建议倒序(最近更新优先),但需要明确用户需求。

3. **失败更新的日志保留:** 如果更新失败(trigger-update 返回错误),是否仍然记录到日志文件?研究建议记录(审计目的),但需要确认是否符合用户预期。

## Sources

### Primary (HIGH confidence)

**Stack Research:**
- Go 标准库官方文档 — `encoding/json`, `bufio`, `os` (Go 1.24+)
- JSON Lines 官方规范 — https://jsonlines.org/
- github.com/google/uuid 官方文档 — Google UUID 库 v1.6+

**Architecture Research:**
- 现有代码库 — `internal/api/trigger.go`, `internal/instance/manager.go`, `internal/logbuffer/buffer.go`
- PROJECT.md — v0.6 里程碑需求定义
- Go Blog: Error Handling — 错误处理最佳实践

**Pitfalls Research:**
- Go 官方 GitHub Issues — 并发写入、文件描述符泄漏、时区处理
- Stack Overflow 高票答案 — JSONL 读取、分页实现、原子追加
- Cloudflare Blog: The complete guide to Go net/http timeouts — HTTP 超时配置

**Features Research:**
- Log Management Best Practices 2026 — LogManager, StrongDM
- Kafka 默认保留策略 — 7 天保留期
- PostgreSQL Audit Logging — 审计日志字段设计

### Secondary (MEDIUM confidence)

- Stack Overflow 社区讨论 — JSON 文件追加写入实现模式
- Reddit r/golang 讨论 — UUID 库选择社区共识
- Dev.to 技术博客 — JSONL 性能优化、流式读取模式
- Medium 技术文章 — Go 并发安全、内存管理

### Tertiary (LOW confidence)

- 个人博客文章 — 特定场景的优化技巧(需验证)
- GitHub 讨论区 — 非官方的实现建议(需测试)

---

*Research completed: 2026-03-26*
*Ready for roadmap: yes*
*Next step: Run `/gsd:roadmap` to create detailed implementation phases*

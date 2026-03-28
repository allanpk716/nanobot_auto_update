# Phase 31: File Persistence - Context

**Gathered:** 2026-03-28
**Status:** Ready for planning

<domain>
## Phase Boundary

系统能够将 Phase 30 创建的 UpdateLog 记录持久化到 JSON Lines 文件，并自动清理 7 天前的旧记录。此阶段专注于文件 I/O 实现、并发安全写入、原子性清理和 UpdateLogger 生命周期管理，不涉及查询 API（Phase 32）和端到端集成（Phase 33）。

**核心功能:**
- Record() 同步写入 JSONL 文件（每次 fsync）
- 内存 + 文件双写（内存供查询，文件供持久化）
- 启动时清理 7 天前旧日志
- 后台每日定时清理（robfig/cron）
- UpdateLogger 生命周期与 main.go 集成
- 文件 I/O 错误降级为纯内存模式

**成功标准:**
1. 更新日志以 JSON Lines 格式持久化到 ./logs/updates.jsonl 文件
2. 文件写入使用原子追加和 sync.Mutex 保护避免并发冲突
3. 应用启动时自动删除 7 天前的日志记录
4. 清理过程不阻塞正常的读写操作
5. 日志文件不存在时自动创建

</domain>

<decisions>
## Implementation Decisions

### 写入策略
- **D-01:** 同步立即写入 — 每次 Record() 调用时立即序列化 JSON 并追加写入文件
  - 数据安全性优先，更新操作频率低（分钟级），延迟影响可忽略
  - 每次写入后调用 file.Sync() 强制刷盘
  - 使用 os.OpenFile 以追加模式直接写入（不使用 bufio 缓冲）
  - 与需求"原子追加写入"一致

- **D-02:** 内存 + 文件双写
  - Record() 同时写入内存 slice 和 JSONL 文件
  - GetAll() 从内存读取（快速，供 Phase 32 查询 API 使用）
  - 文件是持久化备份，程序重启后可从文件恢复
  - 两者保持一致：内存写入成功 + 文件写入可能失败（降级）

### 文件 I/O 错误处理
- **D-03:** 记录错误 + 内存降级 — 文件写入失败时记录 ERROR 日志，继续内存存储
  - 每次 Record() 都尝试文件写入（不设置"停止写入"标志）
  - 自然重试机制：如果临时错误恢复，下次写入自动成功
  - 文件打开失败时在 Record() 内处理（检查目录是否存在，不存在则创建）
  - 不在启动时预创建文件 — 首次 Record() 时懒创建

### 生命周期集成
- **D-04:** UpdateLogger 在 main.go 中创建，传给 api.NewServer()
  - UpdateLogger 的生命周期与整个应用一致（而非绑定 HTTP 服务器）
  - main.go 的优雅关闭逻辑可以直接调用 UpdateLogger.Close()
  - api.NewServer() 接收 *UpdateLogger 参数（不再内部创建）

- **D-05:** 添加 Close() 方法关闭文件 handle
  - 同步写入每次都 fsync，不需要额外的 Flush() 方法
  - Close() 负责：关闭文件 handle、停止清理 cron 任务
  - 遵循 Go 资源管理习惯（与 dailyRotateWriter.Close() 一致）

### 清理时机
- **D-06:** 启动时清理 + 后台每日定时清理
  - 启动时立即执行一次清理（删除 7 天前记录）
  - 使用 robfig/cron 添加每日清理任务（凌晨 3 点执行，与现有 cron 更新任务错开）
  - 清理间隔 24 小时 — 与日志保留天数一致，每天检查一次足够
  - 清理过程不阻塞正常读写（使用临时文件 + rename 实现原子性）

### Claude's Discretion
- JSONL 文件具体的打开/关闭时机（每次 Record() 打开 vs 保持文件 handle）
- 清理 cron 任务的注册方式（复用 main.go 现有 cron scheduler vs 独立 scheduler）
- 内存 slice 在启动时是否从文件恢复（Phase 32 查询 API 可能需要）
- 文件 handle 的错误恢复策略（写入失败后是否重新打开文件）
- 清理任务的具体 cron 表达式（3 点 vs 其他时间）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 31 需求
- `.planning/REQUIREMENTS.md` § Storage — STORE-01, STORE-02 需求
- `.planning/ROADMAP.md` § Phase 31 — File Persistence 阶段目标和成功标准

### 现有架构参考
- `.planning/phases/30-log-structure-and-recording/30-CONTEXT.md` — UpdateLog 数据结构、UpdateLogger 组件、TriggerHandler 集成
- `.planning/phases/28-http-api-trigger/28-CONTEXT.md` — HTTP API 认证和并发控制模式
- `.planning/phases/19-log-buffer-core/19-CONTEXT.md` — 环形缓冲区的线程安全模式（sync.RWMutex + slice）

### 代码参考
- `internal/updatelog/logger.go` — 当前 UpdateLogger 内存实现
- `internal/updatelog/types.go` — UpdateLog 和 InstanceUpdateDetail 数据结构
- `internal/api/trigger.go` — TriggerHandler 调用 UpdateLogger.Record()
- `internal/api/server.go` — 当前 NewServer() 内创建 UpdateLogger 的逻辑
- `internal/logging/daily_rotate.go` — 现有文件 I/O 模式（lumberjack、sync.Mutex、日期轮转）
- `cmd/nanobot-auto-updater/main.go` — 主程序入口、优雅关闭逻辑、cron scheduler

### 外部标准
- JSON Lines 格式规范 — https://jsonlines.org/
- Windows 文件原子操作模式（临时文件 + rename）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **internal/updatelog/logger.go**: UpdateLogger 组件
  - 已有 Record() 和 GetAll() 方法框架，注释明确标注"Phase 31 adds file persistence"
  - 已有 sync.RWMutex 和 slice 存储，只需扩展文件写入逻辑
- **internal/updatelog/types.go**: UpdateLog 和 InstanceUpdateDetail
  - 所有字段已有 JSON 标签，json.Marshal() 直接可用
  - DetermineStatus() 和 BuildInstanceDetails() 辅助函数已完成
- **internal/logging/daily_rotate.go**: 现有文件 I/O 模式
  - Lumberjack 依赖已存在于 go.mod
  - sync.Mutex 文件写入模式可参考
  - 日期检查和文件轮转模式可借鉴
- **cmd/nanobot-auto-updater/main.go**: 主程序入口
  - 已有 os.MkdirAll("./logs", 0755) 创建日志目录
  - 已有 robfig/cron scheduler 实例
  - 已有优雅关闭模式（signal.NotifyContext + 10 秒超时）

### Established Patterns
- **sync.Mutex 文件保护**: daily_rotate.go 使用 Mutex 保护文件写入
- **os.MkdirAll 目录创建**: main.go 启动时创建 logs 目录
- **robfig/cron 定时任务**: 项目已使用 cron 做定时更新
- **signal.NotifyContext 优雅关闭**: main.go 已有关闭流程
- **非阻塞错误处理**: 记录失败不影响主流程
- **上下文感知日志**: 使用 logger.With() 预注入字段

### Integration Points
- **UpdateLogger 扩展**: 在 Record() 中添加文件写入逻辑
- **main.go 修改**: UpdateLogger 创建位置从 NewServer() 移到 main.go
- **api.NewServer() 修改**: 接收 *UpdateLogger 参数而非内部创建
- **main.go 优雅关闭**: 在关闭流程中调用 UpdateLogger.Close()
- **main.go cron 注册**: 添加清理任务到现有 cron scheduler
- **启动时清理**: UpdateLogger 初始化时执行一次清理

</code_context>

<specifics>
## Specific Ideas

- **同步立即写入 + fsync** 确保每次更新记录都可靠持久化，更新操作频率低（分钟级），性能影响可忽略
- **内存 + 文件双写** 让 Phase 32 查询 API 从内存快速读取，文件作为持久化备份
- **每次 Record() 重试文件写入** 自然恢复机制，临时错误（磁盘满恢复后）会在下次写入时自动修复
- **UpdateLogger 提升到 main.go** 确保生命周期与整个应用一致，不受 HTTP 服务器启停影响
- **robfig/cron 定时清理** 复用现有依赖和模式，风格统一

</specifics>

<deferred>
## Deferred Ideas

None — 讨论保持在阶段范围内

</deferred>

---

*Phase: 31-file-persistence*
*Context gathered: 2026-03-28*

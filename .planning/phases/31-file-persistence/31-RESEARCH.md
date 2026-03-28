# Phase 31: File Persistence - Research

**Researched:** 2026-03-28
**Domain:** Go file I/O, JSON Lines persistence, concurrent write safety, Windows atomic operations
**Confidence:** HIGH

## Summary

Phase 31 将 Phase 30 建立的内存 UpdateLogger 扩展为支持 JSON Lines 文件持久化。核心任务包括: 在 Record() 中添加文件追加写入逻辑、实现基于临时文件+rename 的原子性清理、集成 robfig/cron 定时清理任务、以及调整 UpdateLogger 生命周期使其在 main.go 中创建。

研究确认了几个关键技术点: (1) Go 1.24 在 Windows 上 os.Rename 已支持覆盖目标文件 (使用 MoveFileExW + MOVEFILE_REPLACE_EXISTING),临时文件+rename 模式可用; (2) google/renameio 不支持 Windows,不应使用; (3) robfig/cron/v3 当前最新稳定版为 v3.0.1,项目尚未引入此依赖; (4) bufio.Scanner 默认 64KB 行限制对当前日志记录大小足够。

**Primary recommendation:** 使用 os.OpenFile(O_APPEND|O_CREATE|O_WRONLY) 保持文件 handle 开放,配合 sync.Mutex 保护并发写入,每次写入后 fsync;清理使用同目录临时文件+os.Rename 原子替换;新引入 robfig/cron/v3@v3.0.1 实现定时清理。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** 同步立即写入 -- 每次 Record() 调用时立即序列化 JSON 并追加写入文件,每次写入后调用 file.Sync() 强制刷盘,使用 os.OpenFile 以追加模式直接写入
- **D-02:** 内存 + 文件双写 -- Record() 同时写入内存 slice 和 JSONL 文件,GetAll() 从内存读取,文件是持久化备份
- **D-03:** 记录错误 + 内存降级 -- 文件写入失败时记录 ERROR 日志,继续内存存储,每次 Record() 都尝试文件写入
- **D-04:** UpdateLogger 在 main.go 中创建,传给 api.NewServer()
- **D-05:** 添加 Close() 方法关闭文件 handle
- **D-06:** 启动时清理 + 后台每日定时清理 (robfig/cron)

### Claude's Discretion
- JSONL 文件具体的打开/关闭时机 (每次 Record() 打开 vs 保持文件 handle)
- 清理 cron 任务的注册方式 (复用 main.go 现有 cron scheduler vs 独立 scheduler)
- 内存 slice 在启动时是否从文件恢复 (Phase 32 查询 API 可能需要)
- 文件 handle 的错误恢复策略 (写入失败后是否重新打开文件)
- 清理任务的具体 cron 表达式 (3 点 vs 其他时间)

### Deferred Ideas (OUT OF SCOPE)
None
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| STORE-01 | 系统能够持久化更新日志到 JSON Lines 文件,使用原子追加和 sync.Mutex 保护,文件不存在时自动创建 | Standard Stack (os.OpenFile flags); Architecture Patterns (JSONL append writer); Code Examples (Record with file write) |
| STORE-02 | 系统能够自动清理 7 天前旧日志,启动时执行,使用临时文件+rename 原子性清理,不阻塞正常读写 | Architecture Patterns (atomic cleanup); Windows os.Rename verification; robfig/cron integration |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| os (stdlib) | Go 1.24 | File I/O operations (OpenFile, Rename, MkdirAll) | 标准库,无外部依赖,已验证 Windows 兼容性 |
| encoding/json (stdlib) | Go 1.24 | JSON serialization for UpdateLog | 标准库,types.go 已有 JSON tags |
| bufio (stdlib) | Go 1.24 | Streaming JSONL file read during cleanup | 标准库,bufio.Scanner 逐行读取内存高效 |
| sync (stdlib) | Go 1.24 | Mutex for concurrent file write protection | 项目已使用 (logger.go, daily_rotate.go) |
| robfig/cron/v3 | v3.0.1 | 定时清理任务调度 | Go 生态最广泛使用的 cron 库,稳定版本 |
| github.com/google/uuid | v1.6.0 (已有) | UUID v4 生成 | go.mod 已存在 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| os (stdlib) | Go 1.24 | os.CreateTemp 创建清理用临时文件 | 清理过程中创建同目录临时文件 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| robfig/cron/v3 | time.Ticker + goroutine | robfig/cron 提供更精确的 cron 表达式和标准 API,但需引入新依赖 |
| google/renameio | 手动 os.CreateTemp + os.Rename | renameio **不支持 Windows**,不可用 |
| bufio.Scanner | json.Decoder streaming | Scanner 更适合 JSONL (按行分割),Decoder 适合连续 JSON 对象 |

**Installation:**
```bash
go get github.com/robfig/cron/v3@v3.0.1
```

**Version verification:**
- robfig/cron/v3: v3.0.1 是最新稳定版 (通过 `go list -m -versions` 验证)
- Go 版本: 1.24.13 windows/amd64 (已验证)

## Architecture Patterns

### Recommended Project Structure
```
internal/updatelog/
  logger.go       -- UpdateLogger 扩展: 添加文件写入、清理、Close() 方法
  types.go        -- 保持不变
  logger_test.go  -- 扩展测试: 文件写入、清理、并发
cmd/nanobot-auto-updater/
  main.go         -- UpdateLogger 创建位置、cron 注册、优雅关闭集成
internal/api/
  server.go       -- NewServer() 接收 *UpdateLogger 参数
  trigger.go      -- 保持不变
```

### Pattern 1: JSONL Append Writer (文件持久化核心)
**What:** UpdateLogger 保持一个打开的文件 handle,使用 sync.Mutex 保护并发追加写入
**When to use:** Record() 方法中同时写入内存和文件
**Why keep handle open:** 更新操作频率低但需要可靠持久化,保持 handle 开放避免每次 open/close 系统调用开销,且已有 fsync 保证数据安全

```go
// UpdateLogger 结构体扩展
type UpdateLogger struct {
    logs     []UpdateLog
    mu       sync.RWMutex
    logger   *slog.Logger
    filePath string           // JSONL 文件路径
    file     *os.File         // 保持打开的文件 handle (nil 表示纯内存模式)
    fileMu   sync.Mutex       // 文件写入互斥锁 (与 RWMutex 分离,避免清理阻塞 GetAll)
}

// Record 扩展 -- 在内存写入后追加文件写入
func (ul *UpdateLogger) Record(log UpdateLog) error {
    ul.mu.Lock()
    ul.logs = append(ul.logs, log)
    ul.mu.Unlock()

    // 文件写入 (独立于内存锁,避免阻塞 GetAll)
    if err := ul.writeToFile(log); err != nil {
        ul.logger.Error("Failed to write update log to file",
            "error", err, "update_id", log.ID)
        // 降级: 内存已成功,文件失败不影响业务
    }
    return nil
}

// writeToFile -- 独立的文件写入方法
func (ul *UpdateLogger) writeToFile(log UpdateLog) error {
    ul.fileMu.Lock()
    defer ul.fileMu.Unlock()

    // 懒创建: 首次写入时打开文件
    if ul.file == nil {
        if err := ul.openFile(); err != nil {
            return err
        }
    }

    data, err := json.Marshal(log)
    if err != nil {
        return fmt.Errorf("failed to marshal update log: %w", err)
    }

    data = append(data, '\n')
    if _, err := ul.file.Write(data); err != nil {
        // 写入失败,关闭 handle,下次 Record() 会重新打开
        ul.file.Close()
        ul.file = nil
        return fmt.Errorf("failed to write to file: %w", err)
    }

    // D-01: 每次写入后 fsync
    if err := ul.file.Sync(); err != nil {
        ul.logger.Warn("fsync failed", "error", err)
    }
    return nil
}
```

### Pattern 2: Atomic Cleanup (临时文件 + rename)
**What:** 清理旧记录时,将保留的记录写入临时文件,然后原子替换原文件
**When to use:** 启动时清理 + 每日定时清理

```go
// CleanupOldLogs 删除 7 天前的日志记录 (原子性)
func (ul *UpdateLogger) CleanupOldLogs() error {
    cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour)

    ul.fileMu.Lock()
    defer ul.fileMu.Unlock()

    // 关闭当前文件 handle (清理需要独占访问)
    if ul.file != nil {
        ul.file.Close()
        ul.file = nil
    }

    // 文件不存在,无需清理
    if _, err := os.Stat(ul.filePath); os.IsNotExist(err) {
        return nil
    }

    // 打开原文件读取
    src, err := os.Open(ul.filePath)
    if err != nil {
        return fmt.Errorf("failed to open file for cleanup: %w", err)
    }
    defer src.Close()

    // 创建同目录临时文件 (保证同一文件系统,rename 才能原子)
    dir := filepath.Dir(ul.filePath)
    tmp, err := os.CreateTemp(dir, "updates-*.jsonl.tmp")
    if err != nil {
        return fmt.Errorf("failed to create temp file: %w", err)
    }
    tmpPath := tmp.Name()

    // 流式读取 + 写入保留记录
    scanner := bufio.NewScanner(src)
    kept := 0
    removed := 0
    for scanner.Scan() {
        var log UpdateLog
        line := scanner.Text()
        if err := json.Unmarshal([]byte(line), &log); err != nil {
            continue // 跳过无效行
        }
        if log.StartTime.After(cutoff) {
            tmp.WriteString(line + "\n")
            kept++
        } else {
            removed++
        }
    }
    src.Close()
    tmp.Close()

    if removed == 0 {
        // 没有需要清理的记录,删除临时文件
        os.Remove(tmpPath)
        return nil
    }

    // 原子替换: 临时文件 -> 目标文件
    // Go 1.24 Windows: os.Rename 使用 MoveFileExW + MOVEFILE_REPLACE_EXISTING,支持覆盖
    if err := os.Rename(tmpPath, ul.filePath); err != nil {
        os.Remove(tmpPath) // 清理临时文件
        return fmt.Errorf("failed to rename temp file: %w", err)
    }

    ul.logger.Info("Cleaned up old log records",
        "kept", kept, "removed", removed)
    return nil
}
```

### Pattern 3: Close() 方法
**What:** 优雅关闭文件 handle,遵循 Go 资源管理模式
**When to use:** main.go 优雅关闭流程中调用

```go
// Close 关闭文件 handle 并停止清理任务
func (ul *UpdateLogger) Close() error {
    ul.fileMu.Lock()
    defer ul.fileMu.Unlock()

    if ul.file != nil {
        err := ul.file.Close()
        ul.file = nil
        return err
    }
    return nil
}
```

### Pattern 4: main.go 集成
**What:** UpdateLogger 在 main.go 中创建和注入,生命周期与应用一致

```go
// main.go 变更:

// 1. 创建 UpdateLogger (在 API server 之前)
updateLogger := updatelog.NewUpdateLogger(logger, "./logs/updates.jsonl")

// 2. 启动时清理
if err := updateLogger.CleanupOldLogs(); err != nil {
    logger.Error("Failed to cleanup old logs", "error", err)
}

// 3. 注册 cron 清理任务
c := cron.New()
c.AddFunc("0 0 3 * * *", func() {  // 每天凌晨 3 点
    if err := updateLogger.CleanupOldLogs(); err != nil {
        logger.Error("Scheduled log cleanup failed", "error", err)
    }
})
c.Start()

// 4. 传入 NewServer (不再内部创建)
apiServer, err = api.NewServer(&cfg.API, instanceManager, cfg, Version, logger, updateLogger)

// 5. 优雅关闭
// ... 在 signal handler 中:
c.Stop()
updateLogger.Close()
```

### Anti-Patterns to Avoid
- **RWMutex 混用:** 不要用同一个 RWMutex 保护内存和文件操作。清理过程需要独占文件访问,如果复用 RWMutex 会导致 GetAll() 被长时间阻塞。使用独立的 fileMu (sync.Mutex) 保护文件操作
- **bufio.Writer 缓冲写入:** CONTEXT.md D-01 明确要求"不使用 bufio 缓冲",因为每次写入需要 fsync,缓冲层是多余的
- **每次 Record() 打开/关闭文件:** 系统调用开销大,且与保持 handle 的项目模式 (daily_rotate.go) 不一致
- **使用 google/renameio:** 该库明确声明 **不支持 Windows**,在此项目中不可用
- **清理时不关闭文件 handle:** 在 Windows 上,打开的文件不能被 rename 替换。清理前必须先 Close() 文件 handle

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cron 表达式解析和调度 | 自定义 timer/ticker + 时间判断逻辑 | robfig/cron/v3 | 成熟库,支持标准 cron 表达式,错误处理完善 |
| JSON 序列化 | 手动拼接 JSON 字符串 | encoding/json.Marshal | types.go 已有完整 JSON tags,Marshal 直接可用 |
| 文件追加写入 | 手动 seek + write | os.OpenFile(O_APPEND) | OS 级别保证追加位置原子性 (POSIX O_APPEND) |
| 流式文件读取 | ReadFile 全量加载到内存 | bufio.Scanner | 避免内存问题,支持逐行处理 |

**Key insight:** 此阶段的核心复杂度在于并发控制 (内存锁 vs 文件锁的分离) 和 Windows 平台的原子清理实现,而非 JSON 处理或文件 I/O 本身。

## Common Pitfalls

### Pitfall 1: Windows 文件锁定阻止 Rename
**What goes wrong:** 清理时尝试 os.Rename 替换文件,但文件 handle 仍然打开,导致 Windows 报错 "The process cannot access the file because it is being used by another process"
**Why it happens:** Windows 不像 POSIX 那样允许 unlink 打开中的文件。如果有任何 goroutine 持有文件 handle,os.Rename 会失败
**How to avoid:** 清理流程必须先关闭文件 handle (ul.file.Close(); ul.file = nil),在 fileMu.Lock() 保护下执行,确保没有其他写入者持有 handle
**Warning signs:** CleanupOldLogs 返回 "rename temp file" 错误,尤其是程序启动后首次清理正常但后续清理失败

### Pitfall 2: bufio.Scanner 行大小限制
**What goes wrong:** 如果单条 UpdateLog 的 JSON 序列化结果超过 64KB,Scanner 会报 "token too long" 错误
**Why it happens:** bufio.Scanner 默认 MaxScanTokenSize 为 64KB
**How to avoid:** 当前 UpdateLog 结构体大小远小于 64KB (实例数量通常 < 10),暂不需要增加 buffer。如果将来需要,使用 scanner.Buffer(make([]byte, 0), maxCapacity) 增大限制
**Warning signs:** 清理过程中 "skipping invalid line" 日志频繁出现

### Pitfall 3: 清理期间 Record() 并发写入
**What goes wrong:** 清理过程中 (文件已关闭),新的 Record() 调用尝试写入已关闭的文件
**Why it happens:** Record() 和 CleanupOldLogs() 并发执行
**How to avoid:** fileMu.Lock() 确保清理和写入互斥;清理完成后 ul.file 置 nil,下次 writeToFile() 会懒创建重新打开文件
**Warning signs:** "Failed to write update log to file: write to file" 错误紧跟清理日志

### Pitfall 4: 临时文件残留
**What goes wrong:** 清理过程中程序崩溃,临时文件留在磁盘上
**Why it happens:** os.CreateTemp 创建文件后,如果 rename 之前程序退出,临时文件不会被自动清理
**How to avoid:** (1) 清理函数中 rename 失败时显式 os.Remove(tmpPath); (2) 启动时清理可以顺便删除旧临时文件 (匹配 updates-*.jsonl.tmp 模式)
**Warning signs:** logs/ 目录中累积 .tmp 文件

### Pitfall 5: 锁粒度过粗阻塞查询
**What goes wrong:** GetAll() 使用 RWMutex,如果 Record() 在写文件时持有写锁时间过长,GetAll() 会被阻塞
**Why it happens:** 如果文件写入和内存写入使用同一个锁
**How to avoid:** 使用两个独立锁: mu (sync.RWMutex) 保护内存 slice, fileMu (sync.Mutex) 保护文件操作。Record() 流程: Lock(mu) -> 写内存 -> Unlock(mu) -> Lock(fileMu) -> 写文件 -> Unlock(fileMu)
**Warning signs:** 查询 API 响应时间与文件 I/O 耦合

### Pitfall 6: NewUpdateLogger 签名变更导致编译错误
**What goes wrong:** 添加 filePath 参数后忘记更新所有调用点
**Why it happens:** 当前 NewUpdateLogger(logger) 只有一个参数,Phase 31 需要添加 filePath
**How to avoid:** 在修改 NewUpdateLogger 签名的同时更新 server.go 和 main.go 中的调用
**Warning signs:** 编译错误 "cannot use ... as ... in argument to NewUpdateLogger"

## Code Examples

### OpenFile 懒创建模式
```go
// openFile 在首次写入时调用,确保目录和文件存在
func (ul *UpdateLogger) openFile() error {
    dir := filepath.Dir(ul.filePath)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("failed to create log directory: %w", err)
    }

    f, err := os.OpenFile(ul.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return fmt.Errorf("failed to open JSONL file: %w", err)
    }
    ul.file = f
    return nil
}
```

### bufio.Scanner 流式读取 (清理中使用)
```go
scanner := bufio.NewScanner(src)
// 默认 64KB 限制对当前日志大小足够 (每条 UpdateLog JSON 约 200-2000 bytes)
// 如果需要增大: scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
for scanner.Scan() {
    line := scanner.Text()
    if line == "" {
        continue // 跳过空行
    }
    var log UpdateLog
    if err := json.Unmarshal([]byte(line), &log); err != nil {
        // 记录跳过的无效行但不中断清理
        continue
    }
    // 处理 log...
}
if err := scanner.Err(); err != nil {
    // 处理扫描错误
}
```

### robfig/cron 基本用法
```go
import "github.com/robfig/cron/v3"

// 默认 5 字段格式: min hour day month weekday
// 每天凌晨 3 点: "0 3 * * *"
c := cron.New()
c.AddFunc("0 3 * * *", func() {
    if err := updateLogger.CleanupOldLogs(); err != nil {
        logger.Error("Scheduled log cleanup failed", "error", err)
    }
})
c.Start()
// 停止: c.Stop()
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| os.Rename 使用 MoveFile (Windows) | os.Rename 使用 MoveFileExW + MOVEFILE_REPLACE_EXISTING | Go 1.5 | Windows 上 os.Rename 可覆盖目标文件,临时文件+rename 模式可用 |
| bufio.Scanner 64KB 默认 | 相同 (仍为 64KB) | 不变 | 当前日志大小远小于限制,无需调整 |
| robfig/cron v1 (gopkg.in) | robfig/cron/v3 (Go Modules) | 2020 | v3 使用标准 Go Modules,v1 已过时 |

**Deprecated/outdated:**
- google/renameio: 明确不支持 Windows,不适合此项目
- robfig/cron v1/v2: 使用旧的 gopkg.in 导入路径,v3 才是 Go Modules 版本

## Open Questions

1. **内存恢复时机 -- Phase 32 前是否需要**
   - What we know: CONTEXT.md Claude's Discretion 中提到"内存 slice 在启动时是否从文件恢复"
   - What's unclear: Phase 32 查询 API 是否需要启动时从文件恢复数据到内存
   - Recommendation: 本阶段暂不实现文件恢复到内存,留给 Phase 32 按需实现。理由: Phase 31 的成功标准不包含查询功能,过早实现会增加复杂度

2. **robfig/cron 独立 scheduler vs 复用现有**
   - What we know: 项目当前没有 cron 依赖,main.go 没有现有 scheduler
   - What's unclear: 未来是否有更多定时任务需求
   - Recommendation: 创建独立的 cron scheduler 实例,生命周期与 UpdateLogger 绑定。如果未来有更多定时任务,可以在 main.go 统一管理

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go runtime | All code | Yes | 1.24.13 windows/amd64 | -- |
| ./logs/ directory | JSONL file storage | Yes (main.go MkdirAll) | -- | UpdateLogger.openFile() 中也做 MkdirAll |
| robfig/cron/v3 | 定时清理 | No (新依赖) | v3.0.1 | go get 安装 |
| os.Rename (Windows overwrite) | 原子清理 | Yes (Go 1.5+) | 已验证 | -- |

**Missing dependencies with no fallback:**
- robfig/cron/v3 -- 必须通过 `go get github.com/robfig/cron/v3@v3.0.1` 安装

**Missing dependencies with fallback:**
- None

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | testing (stdlib) + testify |
| Config file | None (Go convention) |
| Quick run command | `go test ./internal/updatelog/ -v -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| STORE-01 | Record() 写入 JSONL 文件 | unit | `go test ./internal/updatelog/ -run TestWriteToFile -v` | No -- Wave 0 |
| STORE-01 | 并发写入不冲突 | unit | `go test ./internal/updatelog/ -run TestConcurrentFileWrite -v` | No -- Wave 0 |
| STORE-01 | 文件不存在时自动创建 | unit | `go test ./internal/updatelog/ -run TestAutoCreateFile -v` | No -- Wave 0 |
| STORE-02 | 清理 7 天前记录 | unit | `go test ./internal/updatelog/ -run TestCleanupOldLogs -v` | No -- Wave 0 |
| STORE-02 | 清理不阻塞 GetAll() | unit | `go test ./internal/updatelog/ -run TestCleanupNoBlock -v` | No -- Wave 0 |
| STORE-02 | Close() 关闭文件 handle | unit | `go test ./internal/updatelog/ -run TestClose -v` | No -- Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/updatelog/ -v -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/updatelog/logger_test.go` -- 添加 TestWriteToFile, TestConcurrentFileWrite, TestAutoCreateFile
- [ ] `internal/updatelog/logger_test.go` -- 添加 TestCleanupOldLogs, TestCleanupNoBlock, TestClose
- [ ] Framework install: `go get github.com/robfig/cron/v3@v3.0.1` -- cron 依赖安装

## Sources

### Primary (HIGH confidence)
- Go 1.24 os.Rename doc: "If newpath already exists and is not a directory, Rename replaces it" -- 已通过 `go doc os.Rename` 验证
- Go 1.24 os.Rename Windows overwrite: 已通过实际测试验证 (tmp/test_rename.go)
- robfig/cron/v3 v3.0.1: 已通过 `go list -m -versions` 确认
- 项目现有代码: logger.go, types.go, trigger.go, server.go, daily_rotate.go, main.go -- 全部已读取分析

### Secondary (MEDIUM confidence)
- [Go Issue #10773](https://github.com/golang/go/issues/10773) -- os.Rename Windows 行为历史
- [Go Issue #8914](https://github.com/golang/go/issues/8914) -- Windows Rename 原子性讨论
- [robfig/cron GitHub](https://github.com/robfig/cron) -- v3 API 和 cron 表达式格式
- [alexwlchan.net - Atomic file in Go](https://alexwlchan.net/notes/2026/go-atomicfile/) -- 临时文件+rename 模式

### Tertiary (LOW confidence)
- [google/renameio](https://github.com/google/renameio) -- 不支持 Windows (在 README 中明确声明)
- [StackOverflow - bufio Scanner buffer limit](https://stackoverflow.com/questions/39859222/golang-how-to-overcome-scan-buffer-limit-from-bufio) -- 64KB 限制及解决方法

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- 所有库版本已验证,核心模式已在项目中存在参考实现 (daily_rotate.go)
- Architecture: HIGH -- 双锁模式 (内存 RWMutex + 文件 Mutex) 是 Go 并发编程的标准实践,已在其他项目广泛验证
- Pitfalls: HIGH -- Windows 文件锁定和 bufio.Scanner 限制通过实际搜索和测试确认
- Cron integration: MEDIUM -- robfig/cron 是成熟库,但需确认 v3.0.1 与 Go 1.24 的兼容性 (预期无问题)

**Research date:** 2026-03-28
**Valid until:** 2026-04-27 (30 days -- 稳定的标准库和成熟第三方库)

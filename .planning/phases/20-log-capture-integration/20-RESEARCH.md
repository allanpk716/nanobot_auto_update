# Phase 20: Log Capture Integration - Research

**Researched:** 2026-03-17
**Domain:** Go 进程 stdout/stderr 捕获、并发管道读取、goroutine 生命周期管理
**Confidence:** HIGH

## Summary

Phase 20 实现日志捕获功能,修改进程启动逻辑以捕获 nanobot 进程的 stdout/stderr 输出并实时写入 LogBuffer (Phase 19 已实现)。核心挑战在于:(1) 正确使用 `exec.Command` 的 `StdoutPipe`/`StderrPipe` 避免死锁;(2) 使用 goroutine 并发读取 stdout 和 stderr 管道,防止管道缓冲区满(默认 64KB)导致进程阻塞;(3) 使用 `bufio.Scanner` 逐行读取并写入 LogBuffer,确保大输出量(>10MB)时不丢失日志;(4) 使用 context 控制 goroutine 生命周期,在进程停止时自动停止捕获。

研究发现 Go 标准库 `os/exec` 的 `StdoutPipe()`/`StderrPipe()` 存在 race condition 风险 —— `cmd.Wait()` 会关闭管道,如果此时 goroutine 正在读取会导致 data race。推荐使用 `cmd.Stdout = io.Writer` 模式替代 `StdoutPipe()`,将管道输出重定向到自定义 `io.Writer` (如 `os.Pipe()` 或 `io.Pipe()`),然后在独立 goroutine 中从管道读取并写入 LogBuffer。这种方式避免了 `Wait()` 和管道读取的 race condition,且符合 Go 官方文档建议。

**Primary recommendation:** 修改 `internal/lifecycle/starter.go` 的 `StartNanobot` 函数,使用 `os.Pipe()` 创建 stdout/stderr 管道,设置 `cmd.Stdout = stdoutWriter` 和 `cmd.Stderr = stderrWriter`,启动两个 goroutine 分别从 stdoutReader/stderrReader 使用 `bufio.Scanner` 逐行读取,每行调用 `logBuffer.Write(LogEntry{Source: "stdout"/"stderr", Content: line, Timestamp: time.Now()})`。使用 context 控制 goroutine 生命周期,进程退出时 context 取消,goroutine 自动退出。

<user_constraints>

## User Constraints (from CONTEXT.md)

### Locked Decisions

**无 CONTEXT.md** - 本阶段无用户预先决策,所有技术选择由 Claude 自行研究决定。

### Claude's Discretion

所有技术决策由 Claude 基于研究结果决定:

- 选择 stdout/stderr 捕获方式 (`StdoutPipe()` vs `cmd.Stdout = io.Writer`)
- 选择管道读取模式 (`bufio.Scanner` vs `bufio.Reader` vs `io.Copy`)
- 选择并发架构 (goroutine 数量、channel 模式、context 取消策略)
- 选择错误处理策略 (管道读取失败时是否重试、日志丢失时是否记录)
- 选择缓冲区大小和读取超时配置

### Deferred Ideas (OUT OF SCOPE)

None - 当前阶段专注于核心日志捕获功能。

</user_constraints>

<phase_requirements>

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| **CAPT-01** | 系统捕获 nanobot 进程的 stdout 输出流 | 使用 `cmd.Stdout = stdoutWriter` 重定向 stdout,goroutine 从 stdoutReader 使用 `bufio.Scanner` 逐行读取并写入 LogBuffer |
| **CAPT-02** | 系统捕获 nanobot 进程的 stderr 输出流 | 使用 `cmd.Stderr = stderrWriter` 重定向 stderr,goroutine 从 stderrReader 使用 `bufio.Scanner` 逐行读取并写入 LogBuffer |
| **CAPT-03** | 系统并发读取 stdout 和 stderr 管道,防止管道缓冲区满导致死锁 | 使用两个独立 goroutine 分别读取 stdout 和 stderr,避免顺序读取导致的死锁。参考 GitHub Issue #16787 |
| **CAPT-04** | 系统在 nanobot 进程启动时自动开始捕获输出 | 在 `StartNanobot` 函数中,`cmd.Start()` 之前设置 `cmd.Stdout`/`cmd.Stderr` 并启动读取 goroutine |
| **CAPT-05** | 系统在 nanobot 进程停止时自动停止捕获输出 | 使用 `context.WithCancel` 创建可取消 context,goroutine 监听 `ctx.Done()`,进程退出时调用 `cancel()` 停止 goroutine |

</phase_requirements>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| **os/exec** (标准库) | Go 1.24+ | 进程启动和管道管理 | Go 标准库,提供 `exec.Command`、`cmd.Stdout`、`cmd.Stderr` 字段,官方推荐方式 |
| **os** (标准库) | Go 1.24+ | `os.Pipe()` 创建管道 | 标准库,创建同步管道连接 reader/writer,避免 `StdoutPipe()` 的 race condition |
| **bufio** (标准库) | Go 1.24+ | `bufio.Scanner` 逐行读取 | 标准库,提供 `Scanner.Scan()` 和 `Scanner.Text()` 方法,自动处理行边界和缓冲 |
| **context** (标准库) | Go 1.24+ | Goroutine 生命周期控制 | 标准库,使用 `context.WithCancel` 控制日志捕获 goroutine 的启动和停止 |
| **sync** (标准库) | Go 1.24+ | `sync.WaitGroup` 等待 goroutine 退出 | 标准库,进程停止时使用 `WaitGroup.Wait()` 确保日志捕获 goroutine 完全退出 |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| **time** (标准库) | Go 1.24+ | `time.Now()` 为每条日志生成时间戳 | 每行日志写入 LogBuffer 时调用 `time.Now()` |
| **log/slog** (标准库) | Go 1.24+ | 警告/错误日志 | 管道读取失败时记录 ERROR,慢订阅者丢弃日志时记录 WARN |
| **github.com/WQGroup/logger** | 项目已使用 | 项目统一日志库 | starter.go 中使用 `logger.With("component", "log-capture")` 注入上下文 |
| **internal/logbuffer** | Phase 19 实现 | LogBuffer 日志存储 | 每行日志调用 `logBuffer.Write(LogEntry{...})` 写入缓冲区 |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `cmd.Stdout = io.Writer` | `cmd.StdoutPipe()` | `StdoutPipe()` 存在 race condition (GitHub Issue #19685, #69060),`Wait()` 关闭管道时可能与读取 goroutine 冲突。`cmd.Stdout = io.Writer` 避免 race condition,官方推荐方式 |
| `os.Pipe()` + 独立 goroutine | `io.MultiWriter(stdout, buffer)` | MultiWriter 简单但无法逐行处理,且需要 `io.Pipe()` 避免 race。`os.Pipe()` + goroutine 模式更灵活,可以逐行处理并写入 LogBuffer |
| `bufio.Scanner` | `bufio.Reader.ReadLine()` | Scanner 更简单,自动处理行边界和缓冲区管理。ReadLine 需要手动管理 `isPrefix` 和缓冲区,复杂度高。Scanner 官方推荐用于逐行读取 |
| Context 取消 goroutine | Channel 关闭信号 | Context 是 Go 标准的 goroutine 生命周期管理模式,支持超时和级联取消。Channel 关闭需要额外逻辑,不如 context 简洁 |
| 独立 goroutine 读取 stdout/stderr | 单 goroutine 使用 `select` 多路复用 | 单 goroutine 无法用 `select` 同时读取两个 `io.Reader` (select 只能用于 channel)。必须使用两个 goroutine 分别读取 |

**Installation:**

无需安装外部依赖,所有库均为 Go 标准库或项目内部包。

## Architecture Patterns

### Recommended Project Structure

```
internal/lifecycle/
├── starter.go           # StartNanobot 函数 (需修改)
├── starter_test.go      # 新增测试文件
├── capture.go           # 新增:日志捕获逻辑 (captureLogs 函数)
└── capture_test.go      # 新增:捕获逻辑测试
```

### Pattern 1: os.Pipe() + Goroutine 逐行读取 (推荐模式)

**What:** 使用 `os.Pipe()` 创建 stdout/stderr 管道,设置 `cmd.Stdout = stdoutWriter`,启动独立 goroutine 从 `stdoutReader` 使用 `bufio.Scanner` 逐行读取并写入 LogBuffer

**When to use:** 需要实时捕获进程输出并逐行处理,避免 `StdoutPipe()` race condition

**Example:**

```go
// Source: 基于 Go 标准库 os/exec + os.Pipe 模式 (避免 StdoutPipe race)
package lifecycle

import (
    "bufio"
    "context"
    "io"
    "log/slog"
    "os"
    "os/exec"
    "sync"
    "time"

    "github.com/HQGroup/nanobot-auto-updater/internal/logbuffer"
)

// StartNanobotWithCapture 启动 nanobot 并捕获 stdout/stderr
// CAPT-01: 捕获 stdout
// CAPT-02: 捕获 stderr
// CAPT-03: 并发读取 stdout 和 stderr
// CAPT-04: 进程启动时自动开始捕获
// CAPT-05: 进程停止时自动停止捕获
func StartNanobotWithCapture(
    ctx context.Context,
    command string,
    port uint32,
    startupTimeout time.Duration,
    logger *slog.Logger,
    logBuffer *logbuffer.LogBuffer, // Phase 19 实现
) error {
    logger.Info("Starting nanobot with log capture", "command", command, "port", port)

    // 创建可取消的 context (CAPT-05)
    captureCtx, cancelCapture := context.WithCancel(context.Background())
    var wg sync.WaitGroup

    // 准备命令
    cmd := exec.CommandContext(ctx, "cmd", "/c", command)
    cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")

    // 创建 stdout/stderr 管道 (避免 StdoutPipe race condition)
    stdoutReader, stdoutWriter, err := os.Pipe()
    if err != nil {
        cancelCapture()
        return err
    }
    defer stdoutReader.Close()
    defer stdoutWriter.Close()

    stderrReader, stderrWriter, err := os.Pipe()
    if err != nil {
        cancelCapture()
        return err
    }
    defer stderrReader.Close()
    defer stderrWriter.Close()

    // 设置 cmd.Stdout 和 cmd.Stderr (CAPT-01, CAPT-02)
    cmd.Stdout = stdoutWriter
    cmd.Stderr = stderrWriter

    // 启动 stdout 捕获 goroutine (CAPT-03)
    wg.Add(1)
    go captureLogs(captureCtx, &wg, stdoutReader, "stdout", logBuffer, logger)

    // 启动 stderr 捕获 goroutine (CAPT-03)
    wg.Add(1)
    go captureLogs(captureCtx, &wg, stderrReader, "stderr", logBuffer, logger)

    // 启动进程 (CAPT-04)
    if err := cmd.Start(); err != nil {
        cancelCapture() // 进程启动失败,取消捕获 goroutine
        wg.Wait()       // 等待 goroutine 退出
        logger.Error("Failed to start nanobot process", "error", err)
        return err
    }

    logger.Info("Nanobot process started", "pid", cmd.Process.Pid)

    // 启动监控 goroutine: 进程退出时停止捕获 (CAPT-05)
    go func() {
        cmd.Wait() // 等待进程退出
        logger.Info("Nanobot process exited", "pid", cmd.Process.Pid)
        cancelCapture() // 取消 context,停止日志捕获 goroutine
        wg.Wait()       // 等待捕获 goroutine 退出
        logger.Debug("Log capture goroutines stopped")
    }()

    // 验证端口启动
    if err := waitForPortListening(ctx, port, startupTimeout, logger); err != nil {
        logger.Error("Nanobot startup verification failed", "port", port, "error", err)
        return err
    }

    logger.Info("Nanobot startup verified, port is listening", "port", port)
    return nil
}

// captureLogs 从管道读取日志并写入 LogBuffer
// source: "stdout" or "stderr"
func captureLogs(
    ctx context.Context,
    wg *sync.WaitGroup,
    reader io.Reader,
    source string,
    logBuffer *logbuffer.LogBuffer,
    logger *slog.Logger,
) {
    defer wg.Done()

    scanner := bufio.NewScanner(reader)
    for scanner.Scan() {
        select {
        case <-ctx.Done():
            // Context 取消,停止读取 (CAPT-05)
            logger.Debug("Log capture stopped", "source", source)
            return
        default:
            // 逐行读取并写入 LogBuffer
            line := scanner.Text()
            entry := logbuffer.LogEntry{
                Timestamp: time.Now(),
                Source:    source,
                Content:   line,
            }
            if err := logBuffer.Write(entry); err != nil {
                logger.Error("Failed to write log to buffer",
                    "source", source, "error", err)
            }
        }
    }

    // 检查扫描错误 (ERR-01)
    if err := scanner.Err(); err != nil {
        logger.Error("Log capture scanner error",
            "source", source, "error", err)
    }
}
```

### Pattern 2: 避免 StdoutPipe Race Condition

**What:** 不要使用 `cmd.StdoutPipe()`,改用 `cmd.Stdout = io.Writer` 模式

**Why:** `StdoutPipe()` 的文档明确指出:"Wait will close the pipe after seeing the command exit, so most callers need not close the pipe themselves; however, an implication is that it is incorrect to call Wait before all reads from the pipe have completed." 在 goroutine 中读取 `StdoutPipe()` 返回的管道,同时主 goroutine 调用 `cmd.Wait()`,会导致 race condition。

**Anti-pattern (错误方式):**

```go
// ❌ 错误: StdoutPipe + goroutine 导致 race condition
cmd := exec.Command("nanobot")
stdoutPipe, _ := cmd.StdoutPipe() // 返回的管道会被 Wait() 关闭

cmd.Start()

// goroutine 读取 stdout
go func() {
    io.Copy(os.Stdout, stdoutPipe) // 读取操作
}()

cmd.Wait() // Wait() 关闭 stdoutPipe,与读取操作并发 -> RACE!
```

**Correct pattern (正确方式):**

```go
// ✅ 正确: cmd.Stdout = io.Writer 避免 race
cmd := exec.Command("nanobot")

stdoutReader, stdoutWriter, _ := os.Pipe()
cmd.Stdout = stdoutWriter // 设置 io.Writer

cmd.Start()

// goroutine 从 reader 读取
go func() {
    defer stdoutReader.Close()
    scanner := bufio.NewScanner(stdoutReader)
    for scanner.Scan() {
        fmt.Println(scanner.Text())
    }
}()

cmd.Wait()
stdoutWriter.Close() // 显式关闭 writer
```

### Pattern 3: Context 取消 + WaitGroup 等待退出

**What:** 使用 `context.WithCancel` 控制 goroutine 生命周期,使用 `sync.WaitGroup` 确保进程退出时所有 goroutine 完全退出

**When to use:** 进程生命周期管理,确保无 goroutine 泄漏

**Example:**

```go
// Source: Go 标准库 context + sync 模式
captureCtx, cancelCapture := context.WithCancel(context.Background())
var wg sync.WaitGroup

// 启动 goroutine
wg.Add(1)
go func() {
    defer wg.Done()
    for {
        select {
        case <-captureCtx.Done():
            return // Context 取消,退出
        default:
            // 工作逻辑
        }
    }
}()

// 进程退出时停止 goroutine
cmd.Wait()
cancelCapture() // 取消 context
wg.Wait()       // 等待 goroutine 退出
```

### Anti-Patterns to Avoid

- **使用 `cmd.StdoutPipe()` 读取进程输出**: 存在 race condition (GitHub Issue #19685, #69060),`Wait()` 关闭管道时可能与读取 goroutine 冲突
- **顺序读取 stdout 和 stderr**: 会导致死锁,如果 stderr 管道缓冲区满(64KB)而程序正在等待 stdout 读取完成。必须使用两个 goroutine 并发读取
- **忘记关闭管道 writer**: 导致 reader 永远阻塞在 EOF,goroutine 泄漏
- **使用 `io.Copy` 而不是 `bufio.Scanner`**: `io.Copy` 会读取整个输出到内存,大输出(>10MB)会占用大量内存。Scanner 逐行读取,内存占用小
- **不使用 context 控制 goroutine 生命周期**: 进程退出后 goroutine 永远阻塞在管道读取,导致 goroutine 泄漏
- **在 goroutine 中调用 `cmd.Wait()`**: `Wait()` 只能调用一次,应该在主 goroutine 或监控 goroutine 中调用

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| **进程输出捕获** | 从零实现管道读取、缓冲区管理、行边界处理 | `bufio.Scanner` + `os.Pipe()` | Scanner 自动处理行边界和缓冲区管理,官方推荐。自实现容易出错 (缓冲区边界、EOF 处理、内存泄漏) |
| **Goroutine 生命周期管理** | 手动管理 goroutine 启动/停止、channel 信号 | `context.WithCancel` + `sync.WaitGroup` | Context 是 Go 标准的 goroutine 生命周期管理模式,WaitGroup 确保完全退出。自实现容易遗漏清理逻辑 |
| **并发管道读取** | 单 goroutine 使用 `select` 同时读取多个 `io.Reader` | 两个独立 goroutine 分别读取 stdout/stderr | `select` 只能用于 channel,不能用于 `io.Reader`。必须使用多个 goroutine 并发读取 |
| **逐行读取逻辑** | 手动实现行缓冲、分隔符检测、缓冲区扩容 | `bufio.Scanner` + `Scan()` + `Text()` | Scanner 自动处理所有细节,包括大行(>64KB)的分块读取。自实现复杂度高,容易出错 |

**Key insight:** 进程输出捕获的核心是 **避免 race condition** (使用 `cmd.Stdout = io.Writer` 而不是 `StdoutPipe()`) 和 **并发读取多个管道** (两个 goroutine 分别读取 stdout/stderr)。`bufio.Scanner` 是逐行读取的标准方式,无需手动管理缓冲区。

## Common Pitfalls

### Pitfall 1: StdoutPipe Race Condition

**What goes wrong:** 使用 `cmd.StdoutPipe()` 在 goroutine 中读取,同时主 goroutine 调用 `cmd.Wait()`,导致 data race

**Why it happens:** `Wait()` 会在进程退出后关闭管道,如果此时 goroutine 正在读取会导致并发读写冲突

**How to avoid:**
1. 使用 `cmd.Stdout = io.Writer` 替代 `StdoutPipe()`
2. 使用 `os.Pipe()` 创建管道,设置 `cmd.Stdout = stdoutWriter`
3. 在独立 goroutine 中从 `stdoutReader` 读取
4. 进程退出后显式关闭 `stdoutWriter`

**Warning signs:** `go test -race` 报告 data race 在 `os.(*File).close()` 和 `os.(*File).Read()`

### Pitfall 2: 管道缓冲区满导致死锁

**What goes wrong:** 顺序读取 stdout 和 stderr,如果 stderr 管道缓冲区满(64KB)而程序正在等待 stdout 读取完成,导致死锁

**Why it happens:** 管道有固定缓冲区大小(通常 64KB),写入端缓冲区满时会阻塞直到读取端读取数据

**How to avoid:**
1. 使用两个独立 goroutine 分别读取 stdout 和 stderr
2. 确保两个 goroutine 同时启动,不要等待一个完成再启动另一个
3. 使用 `bufio.Scanner` 逐行读取,避免缓冲区积压

**Warning signs:** 进程启动后卡住,无输出,`ps` 显示进程状态为 "S" (sleeping)

### Pitfall 3: Goroutine 泄漏

**What goes wrong:** 进程退出后,日志捕获 goroutine 仍然阻塞在 `scanner.Scan()`,导致 goroutine 永远不退出

**Why it happens:** `scanner.Scan()` 会阻塞直到读到数据或 EOF,如果 writer 端未关闭,reader 永远等待

**How to avoid:**
1. 使用 `context.WithCancel` 创建可取消 context
2. goroutine 中使用 `select { case <-ctx.Done(): return default: scanner.Scan() }`
3. 进程退出时调用 `cancel()` 取消 context
4. 使用 `sync.WaitGroup` 等待 goroutine 退出

**Warning signs:** `runtime.NumGoroutine()` 持续增长,内存占用上升

### Pitfall 4: 忘记关闭管道 Writer

**What goes wrong:** 进程退出后未关闭 `stdoutWriter`,导致 `stdoutReader` 永远阻塞在 `scanner.Scan()`,goroutine 泄漏

**Why it happens:** Reader 在读到 EOF 时才会返回,Writer 未关闭时 Reader 永远等待更多数据

**How to avoid:**
1. 使用 `defer stdoutWriter.Close()` 确保函数退出时关闭
2. 在监控 goroutine 中,`cmd.Wait()` 后立即关闭 writer
3. 使用 `defer` 确保即使 panic 也会关闭

**Warning signs:** `lsof -p <pid>` 显示管道未关闭,goroutine 数量不减

### Pitfall 5: 大输出量导致内存溢出

**What goes wrong:** 使用 `io.ReadAll` 或 `bytes.Buffer` 读取整个输出到内存,nanobot 输出 100MB+ 日志导致 OOM

**Why it happens:** `io.ReadAll` 会分配足够大的缓冲区存储整个输出,无内存限制

**How to avoid:**
1. 使用 `bufio.Scanner` 逐行读取,每行立即写入 LogBuffer
2. 不要缓存整个输出,流式处理
3. LogBuffer 使用环形缓冲区 (5000 条容量),自动覆盖旧日志

**Warning signs:** 内存占用持续增长,CPU 飙升,进程被 OOM killer 杀死

## Code Examples

### 完整实现示例 (推荐)

```go
// Source: 基于 Go 标准库 + 项目内部 logbuffer
package lifecycle

import (
    "bufio"
    "context"
    "io"
    "log/slog"
    "os"
    "os/exec"
    "sync"
    "time"

    "github.com/HQGroup/nanobot-auto-updater/internal/logbuffer"
)

// StartNanobotWithCapture 启动 nanobot 并捕获日志 (完整实现)
func StartNanobotWithCapture(
    ctx context.Context,
    command string,
    port uint32,
    startupTimeout time.Duration,
    logger *slog.Logger,
    logBuffer *logbuffer.LogBuffer,
) error {
    logger.Info("Starting nanobot with log capture", "command", command, "port", port)

    // 创建可取消的 context (CAPT-05)
    captureCtx, cancelCapture := context.WithCancel(context.Background())
    var wg sync.WaitGroup

    // 准备命令
    cmd := exec.CommandContext(ctx, "cmd", "/c", command)
    cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")
    cmd.SysProcAttr = &windows.SysProcAttr{
        HideWindow:    true,
        CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP,
    }

    // 创建 stdout 管道 (CAPT-01)
    stdoutReader, stdoutWriter, err := os.Pipe()
    if err != nil {
        cancelCapture()
        return err
    }

    // 创建 stderr 管道 (CAPT-02)
    stderrReader, stderrWriter, err := os.Pipe()
    if err != nil {
        cancelCapture()
        stdoutReader.Close()
        stdoutWriter.Close()
        return err
    }

    // 设置 cmd.Stdout 和 cmd.Stderr (避免 StdoutPipe race)
    cmd.Stdout = stdoutWriter
    cmd.Stderr = stderrWriter

    // 启动 stdout 捕获 goroutine (CAPT-03)
    wg.Add(1)
    go captureLogs(captureCtx, &wg, stdoutReader, "stdout", logBuffer, logger)

    // 启动 stderr 捕获 goroutine (CAPT-03)
    wg.Add(1)
    go captureLogs(captureCtx, &wg, stderrReader, "stderr", logBuffer, logger)

    // 启动进程 (CAPT-04)
    if err := cmd.Start(); err != nil {
        cancelCapture() // 进程启动失败,取消捕获 goroutine
        wg.Wait()       // 等待 goroutine 退出
        stdoutReader.Close()
        stdoutWriter.Close()
        stderrReader.Close()
        stderrWriter.Close()
        logger.Error("Failed to start nanobot process", "error", err)
        return err
    }

    logger.Info("Nanobot process started", "pid", cmd.Process.Pid)

    // 启动监控 goroutine: 进程退出时停止捕获 (CAPT-05)
    go func() {
        err := cmd.Wait() // 等待进程退出
        if err != nil {
            logger.Warn("Nanobot process exited with error", "pid", cmd.Process.Pid, "error", err)
        } else {
            logger.Info("Nanobot process exited", "pid", cmd.Process.Pid)
        }

        // 关闭 writer 端,触发 reader EOF
        stdoutWriter.Close()
        stderrWriter.Close()

        // 取消 context,停止日志捕获 goroutine
        cancelCapture()
        wg.Wait() // 等待捕获 goroutine 退出

        logger.Debug("Log capture goroutines stopped")
    }()

    // 释放进程句柄 (原 starter.go 逻辑)
    if err := cmd.Process.Release(); err != nil {
        logger.Warn("Failed to detach nanobot process (non-fatal)", "error", err)
    }

    // 验证端口启动
    if err := waitForPortListening(ctx, port, startupTimeout, logger); err != nil {
        logger.Error("Nanobot startup verification failed", "port", port, "error", err)
        return err
    }

    logger.Info("Nanobot startup verified, port is listening", "port", port)
    return nil
}

// captureLogs 从管道读取日志并写入 LogBuffer
func captureLogs(
    ctx context.Context,
    wg *sync.WaitGroup,
    reader io.ReadCloser,
    source string,
    logBuffer *logbuffer.LogBuffer,
    logger *slog.Logger,
) {
    defer wg.Done()
    defer reader.Close()

    scanner := bufio.NewScanner(reader)
    for {
        select {
        case <-ctx.Done():
            // Context 取消,停止读取 (CAPT-05)
            logger.Debug("Log capture stopped", "source", source)
            return
        default:
            // 非阻塞扫描
            if !scanner.Scan() {
                // EOF 或错误
                if err := scanner.Err(); err != nil {
                    // ERR-01: 记录错误但继续运行
                    logger.Error("Log capture scanner error",
                        "source", source, "error", err)
                }
                return
            }

            // 逐行读取并写入 LogBuffer
            line := scanner.Text()
            entry := logbuffer.LogEntry{
                Timestamp: time.Now(),
                Source:    source,
                Content:   line,
            }
            if err := logBuffer.Write(entry); err != nil {
                // ERR-03: 写入失败时记录错误并丢弃日志行
                logger.Error("Failed to write log to buffer",
                    "source", source, "line", line, "error", err)
            }
        }
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| **`cmd.StdoutPipe()` + goroutine 读取** | `cmd.Stdout = io.Writer` + `os.Pipe()` | Go 1.7+ (2016) | 避免 race condition,官方推荐方式。`StdoutPipe()` 文档明确警告不要与 `Wait()` 并发调用 |
| **单 goroutine 顺序读取 stdout/stderr** | 两个 goroutine 并发读取 | Go 1.0+ (2012) | 避免管道缓冲区满导致的死锁。社区最佳实践 (GitHub Issue #16787) |
| **`io.ReadAll` 缓存整个输出** | `bufio.Scanner` 流式逐行读取 | Go 1.1+ (2013) | 避免大输出量导致 OOM,内存占用恒定。Scanner 官方推荐用于逐行读取 |
| **Channel 关闭信号控制 goroutine** | `context.WithCancel` 控制 goroutine | Go 1.7+ (2016) | Context 支持超时和级联取消,Go 标准并发模式。Channel 关闭需要额外逻辑 |

**Deprecated/outdated:**
- **`cmd.StdoutPipe()`**: 存在 race condition,不要在 goroutine 中读取
- **`ioutil.ReadAll`**: 已废弃 (Go 1.16+),改用 `io.ReadAll`,但仍然不推荐用于大输出
- **无 context 的 goroutine**: 难以控制生命周期,容易泄漏

## Open Questions

1. **如何处理进程启动失败时的管道清理?**

   **What we know:**
   - 进程启动失败时,已创建的管道和 goroutine 需要正确清理
   - 需要关闭 reader/writer 端,取消 context,等待 goroutine 退出

   **What's unclear:**
   - 是否需要封装管道创建和清理逻辑到独立函数
   - 是否需要自定义错误类型区分"进程启动失败"和"端口验证失败"

   **Recommendation:**
   - 使用 `defer` 确保函数退出时清理管道
   - 创建 `cleanupPipes(stdoutReader, stdoutWriter, stderrReader, stderrWriter)` 辅助函数
   - 在错误路径调用 `cancelCapture()` + `wg.Wait()` + 清理管道

2. **是否需要支持重新连接 LogBuffer?**

   **What we know:**
   - Phase 19 LogBuffer 已实现,容量 5000 条
   - Phase 21 会将 LogBuffer 集成到 InstanceLifecycle

   **What's unclear:**
   - 如果进程重启 (Phase 21),LogBuffer 是否需要清空
   - 是否需要支持热重载 LogBuffer (不停止进程)

   **Recommendation:**
   - Phase 20 专注于日志捕获,不处理 LogBuffer 生命周期
   - LogBuffer 的创建和销毁由 Phase 21 的 InstanceManager 管理
   - Phase 21 根据需求决定是否在进程重启时清空 LogBuffer

## Validation Architecture

> workflow.nyquist_validation 未在 .planning/config.json 中显式设置为 false,因此包含此部分。

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go 1.24+ 标准测试框架 (`testing` 包) |
| Config file | 无 (Go 测试不需要配置文件) |
| Quick run command | `go test -v ./internal/lifecycle` |
| Full suite command | `go test -v -race ./internal/lifecycle` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| **CAPT-01** | 捕获 nanobot 进程的 stdout 输出流 | unit + integration | `go test -v -run TestCaptureStdout ./internal/lifecycle` | ❌ Wave 0 |
| **CAPT-02** | 捕获 nanobot 进程的 stderr 输出流 | unit + integration | `go test -v -run TestCaptureStderr ./internal/lifecycle` | ❌ Wave 0 |
| **CAPT-03** | 并发读取 stdout 和 stderr 管道,防止死锁 | unit + integration | `go test -v -run TestConcurrentPipeReading ./internal/lifecycle` | ❌ Wave 0 |
| **CAPT-04** | 进程启动时自动开始捕获输出 | unit + integration | `go test -v -run TestAutoStartCapture ./internal/lifecycle` | ❌ Wave 0 |
| **CAPT-05** | 进程停止时自动停止捕获输出 | unit + integration | `go test -v -run TestAutoStopCapture ./internal/lifecycle` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test -v ./internal/lifecycle` (快速验证)
- **Per wave merge:** `go test -v -race ./internal/lifecycle` (完整 race 检测)
- **Phase gate:** `go test -v -race -cover ./internal/lifecycle` (覆盖率 > 80%)

### Wave 0 Gaps

- [ ] `internal/lifecycle/capture.go` - 日志捕获逻辑 (captureLogs 函数)
- [ ] `internal/lifecycle/capture_test.go` - 捕获逻辑测试 (模拟进程输出)
- [ ] `internal/lifecycle/starter.go` - 修改 StartNanobot 函数支持 LogBuffer 参数
- [ ] `internal/lifecycle/starter_test.go` - StartNanobotWithCapture 集成测试

*(如果无 gaps: "None — existing test infrastructure covers all phase requirements")*

## Sources

### Primary (HIGH confidence)

- **Go 标准库文档**: os/exec, bufio, context, os - 官方文档,无过期风险
- **GitHub Issue #19685**: os/exec: data race between StdoutPipe and Wait - 官方确认的 race condition (验证时间: 2026-03-17)
- **GitHub Issue #16787**: io/ioutil hangs with too big output from os/exec stderr and stdout pipes - 管道缓冲区满导致死锁 (验证时间: 2026-03-17)
- **项目现有代码**: internal/lifecycle/starter.go, internal/logbuffer/buffer.go - 已验证实现

### Secondary (MEDIUM confidence)

- **Hack MySQL: Reading os/exec.Cmd Output Without Race Conditions**: https://hackmysql.com/rand/reading-os-exec-cmd-output-without-race-conditions/ - 避免使用 `StdoutPipe`,推荐 `cmd.Stdout = io.Writer` (验证时间: 2026-03-17)
- **Stack Overflow: Go: How to prevent pipes from blocking**: https://stackoverflow.com/questions/61783991 - 管道阻塞解决方案 (验证时间: 2026-03-17)
- **DoltHub Blog: Some Useful Patterns for Go's os/exec**: https://www.dolthub.com/blog/2022-11-28-go-os-exec-patterns/ - os/exec 最佳实践 (验证时间: 2026-03-17)

### Tertiary (LOW confidence)

- 无 - 所有核心发现均通过 Primary 或 Secondary 源验证

## Metadata

**Confidence breakdown:**
- Standard stack: **HIGH** - Go 标准库文档 + 官方 GitHub Issue 确认 race condition 和解决方案
- Architecture: **HIGH** - `os.Pipe()` + goroutine 模式是社区最佳实践,避免 `StdoutPipe` race condition
- Pitfalls: **HIGH** - 基于 Go 官方 Issue 和社区最佳实践总结,所有陷阱都有验证案例

**Research date:** 2026-03-17
**Valid until:** 2026-04-17 (30 天,Go 标准库稳定,进程管道模式不会重大变化)

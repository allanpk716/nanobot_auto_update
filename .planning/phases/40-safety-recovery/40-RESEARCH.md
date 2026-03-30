# Phase 40: Safety & Recovery - Research

**Researched:** 2026-03-30
**Domain:** Windows self-update safety: auto-restart, Pushover notification, .old cleanup/recovery
**Confidence:** HIGH

## Summary

Phase 40 是 v0.8 Self-Update 里程碑的最后一个阶段，为自更新流程添加安全层：更新后自动重启（self-spawn + 端口重试）、Pushover 通知（开始/完成两次）、启动时 .old 备份文件清理、以及异常 .exe.old 存在时的自动恢复。

本阶段全部基于已验证的代码模式和现有基础设施：(1) PoC 已验证 self-spawn 模式可行（`tmp/poc_selfupdate.go:64-74`），(2) `SelfUpdateHandler` 已有异步 goroutine + panic recovery，(3) `Notifier` interface 已在 `TriggerHandler` 中建立注入模式，(4) `Updater.Update()` 已生成 `.exe.old` 备份。本阶段的核心工作是**集成**这些现有能力，而非创建新机制。

需要修改的文件范围较小且边界清晰：`internal/api/selfupdate_handler.go`（添加 Notifier 注入 + 通知 + self-spawn 重启）、`internal/api/server.go`（传递 Notifier）、`cmd/nanobot-auto-updater/main.go`（启动时 .old 清理/恢复 + 端口重试 + 状态文件写入逻辑）。可选地，将 self-spawn 和 .old 清理逻辑提取为 `internal/lifecycle/restart.go` 以保持可测试性。

**Primary recommendation:** 按照 D-01 到 D-05 的锁定决策，在 SelfUpdateHandler goroutine 中添加 Notifier 通知和 self-spawn 重启，在 main.go 启动序列中添加 .old 状态检查和端口重试。所有模式都有项目内先例可循。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** 直接退出 -- 更新成功后在 goroutine 中执行 `cmd.Start` (复用 PoC 模式: `CREATE_NO_WINDOW`) + `os.Exit(0)`，跳过 graceful shutdown。简单快速，端口重用风险低。
- **D-02:** 复用 Notifier interface -- 通过构造函数注入 `SelfUpdateHandler`，复用 Phase 34 的 `Notifier` interface (duck typing, `Notify(title, message)` 方法)。与 `TriggerHandler` 注入模式一致。
- **D-03:** 开始 + 完成两次通知 -- 自更新开始时发送通知（含当前版本和目标版本），完成时再发一次通知（含结果：成功/失败 + 版本信息 + 错误详情）。与 Phase 34 TriggerHandler 模式一致。
- **D-04:** 状态文件标记 -- 更新成功后写入 `.update-success` 状态文件（含时间戳和新版本号）。启动时检查：
  - `.update-success` 存在 -> 上次更新成功 -> 清理 `.old` + 删除 `.update-success`
  - `.update-success` 不存在但 `.exe.old` 存在 -> 上次更新后崩溃 -> 从 `.exe.old` 恢复旧版本
- **D-05:** 重试机制 -- 新进程启动时如果 HTTP server 端口绑定失败（端口仍被旧进程占用），等待 500ms 重试，最多重试 5 次（总共 2.5s）。绑定成功后继续正常启动。

### Claude's Discretion
- 状态文件的具体命名和存放路径（建议 exe 同目录）
- 重启通知的具体消息格式和标题
- 端口重试的具体实现（循环 + time.Sleep vs ticker）
- .old 恢复的具体实现（文件复制 vs rename）
- 日志字段命名和上下文注入方式

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SAFE-01 | 更新后自动重启（self-spawn + graceful shutdown + 端口重绑重试） | PoC 已验证 self-spawn 模式 (tmp/poc_selfupdate.go:64-74)；项目已有 `internal/lifecycle/daemon.go` 的 SysProcAttr + cmd.Start + os.Exit(0) 模式；D-01 锁定直接退出策略；D-05 锁定 500ms x5 重试 |
| SAFE-02 | Pushover 通知（自更新开始/完成/失败通知） | `internal/notifier/notifier.go` 已实现 `Notify(title, message)` 方法；`TriggerHandler` 已建立 Notifier 注入模式 (trigger.go:41-42)；D-02 锁定复用此 interface；D-03 锁定两次通知 |
| SAFE-03 | .old 文件清理（启动时检查并清理旧备份文件） | `Updater.Update()` 已在 selfupdate.go:356 设置 `OldSavePath: exePath + ".old"`；D-04 锁定状态文件标记方案；启动时检测 `.update-success` 存在即清理 |
| SAFE-04 | 启动时备份验证（检测 .exe.old 异常文件存在，自动恢复旧版本） | D-04 锁定状态文件方案区分正常/异常 .old 存在；`os.Rename` 可原子恢复；恢复后 self-spawn + os.Exit(0) 重启旧版本 |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| golang.org/x/sys | v0.41.0 (existing) | Windows SysProcAttr, CREATE_NO_WINDOW | 项目已有，self-spawn 必需 |
| github.com/minio/selfupdate | v0.6.0 (existing) | .old 文件已由此库创建 | Phase 38 已集成，无需新增 |
| github.com/gregdel/pushover | v1.4.0 (existing) | Pushover 通知发送 | Notifier 已封装此库 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| encoding/json (stdlib) | Go 1.24 | 状态文件 JSON 序列化 | .update-success 文件内容 |
| os/exec (stdlib) | Go 1.24 | self-spawn 新进程 | 更新后重启 + .old 恢复后重启 |
| net (stdlib) | Go 1.24 | 端口绑定重试 | `net.Listen` 检测端口可用性 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| 直接 os.Exit(0) | graceful shutdown | D-01 锁定直接退出。graceful 需要信号传递机制、更长的停机时间窗口、更复杂的端口释放协调，收益不大 |
| .update-success 状态文件 | Windows 信号量/注册表 | 状态文件简单、可调试、跨重启持久。注册表过于重量级，信号量进程死亡后自动清除无法区分正常/异常 |

**Installation:**
```bash
# 无需安装新依赖 -- 所有库已在 go.mod 中
```

**Version verification:**
- `golang.org/x/sys`: v0.41.0 (go.mod confirmed)
- `github.com/minio/selfupdate`: v0.6.0 (go.mod confirmed)
- `github.com/gregdel/pushover`: v1.4.0 (go.mod confirmed)

## Architecture Patterns

### Recommended Changes Map
```
Files to MODIFY (no new files required):
├── internal/api/selfupdate_handler.go  # Add Notifier + self-spawn + notifications
├── internal/api/server.go              # Pass Notifier to NewSelfUpdateHandler
└── cmd/nanobot-auto-updater/main.go    # Add .old cleanup/recovery + port retry + state file

Optional extraction (Claude's discretion):
└── internal/lifecycle/restart.go       # Extract self-spawn + .old logic for testability
```

### Pattern 1: Notifier Injection into SelfUpdateHandler (follows TriggerHandler pattern)
**What:** 通过构造函数注入 Notifier interface，与 TriggerHandler 模式完全一致
**When to use:** SelfUpdateHandler 需要发送 Pushover 通知时
**Example:**
```go
// Source: internal/api/trigger.go:46 (existing pattern to follow)
// Current:
func NewSelfUpdateHandler(updater SelfUpdateChecker, version string, im UpdateMutex, logger *slog.Logger) *SelfUpdateHandler {

// After Phase 40:
func NewSelfUpdateHandler(updater SelfUpdateChecker, version string, im UpdateMutex, notif Notifier, logger *slog.Logger) *SelfUpdateHandler {
    return &SelfUpdateHandler{
        updater:         updater,
        version:         version,
        instanceManager: im,
        notifier:        notif,           // NEW: nil-safe, like TriggerHandler
        logger:          logger.With("source", "api-self-update"),
    }
}
```

### Pattern 2: Self-Spawn Restart (from PoC + lifecycle/daemon.go)
**What:** exec.Command + SysProcAttr + cmd.Start + os.Exit(0)
**When to use:** 更新成功后重启新版本，或 .old 恢复后重启旧版本
**Example:**
```go
// Source: tmp/poc_selfupdate.go:64-74 (PoC validated)
// Source: internal/lifecycle/daemon.go:60-64 (existing project code)
func restartSelf() error {
    exePath, err := os.Executable()
    if err != nil {
        return fmt.Errorf("get exe path: %w", err)
    }
    cmd := exec.Command(exePath, os.Args[1:]...)
    cmd.SysProcAttr = &syscall.SysProcAttr{
        HideWindow:    true,
        CreationFlags: windows.CREATE_NO_WINDOW,
    }
    if err := cmd.Start(); err != nil {
        return fmt.Errorf("spawn new process: %w", err)
    }
    os.Exit(0)
    return nil // unreachable
}
```

### Pattern 3: Non-Blocking Notification (from TriggerHandler)
**What:** 在 goroutine 中异步发送通知，失败仅记日志不中断流程
**When to use:** 自更新开始和完成时发送通知
**Example:**
```go
// Source: internal/api/trigger.go:77-94 (existing pattern)
if h.notifier != nil {
    go func() {
        defer func() {
            if r := recover(); r != nil {
                h.logger.Error("notification goroutine panic", "panic", r)
            }
        }()
        if err := h.notifier.Notify(title, message); err != nil {
            h.logger.Error("notification failed", "error", err)
        }
    }()
}
```

### Pattern 4: Status File for Update State Tracking
**What:** 更新成功后写入 JSON 状态文件，启动时检查以区分正常/异常
**When to use:** 启动时判断 .old 文件是正常备份还是需要恢复
**Example:**
```go
// Update success marker (written after selfupdate.Apply succeeds)
type UpdateSuccessMarker struct {
    Timestamp  string `json:"timestamp"`
    NewVersion string `json:"new_version"`
    OldVersion string `json:"old_version"`
}

// Write after successful Apply()
marker := UpdateSuccessMarker{
    Timestamp:  time.Now().Format(time.RFC3339),
    NewVersion: targetVersion,
    OldVersion: currentVersion,
}
data, _ := json.Marshal(marker)
os.WriteFile(exePath+".update-success", data, 0644)
```

### Pattern 5: Port Binding Retry (D-05)
**What:** net.Listen 失败时等待重试，给旧进程时间释放端口
**When to use:** 新版本进程启动时端口可能仍被旧进程占用
**Example:**
```go
// D-05: 500ms interval, max 5 retries (2.5s total)
func listenWithRetry(addr string, logger *slog.Logger) (net.Listener, error) {
    var lastErr error
    for i := 0; i < 5; i++ {
        listener, err := net.Listen("tcp", addr)
        if err == nil {
            return listener, nil
        }
        lastErr = err
        logger.Warn("port bind failed, retrying",
            "addr", addr,
            "attempt", i+1,
            "error", err)
        time.Sleep(500 * time.Millisecond)
    }
    return nil, fmt.Errorf("failed to bind %s after 5 retries: %w", addr, lastErr)
}
```
**Note:** 需要修改 `Server.Start()` 方法，将 `httpServer.ListenAndServe()` 改为先 `listenWithRetry()` 然后 `httpServer.Serve(listener)`。

### Anti-Patterns to Avoid
- **不要在 os.Exit(0) 前发送完成通知**: 通知是异步 goroutine，os.Exit(0) 会立即终止进程，通知可能来不及发送。完成通知应在 self-spawn 之前发送，或在新进程启动后由新进程发送（推荐在 self-spawn 前发送，因为通知是 fire-and-forget）。
- **不要用 cmd.Run() 替代 cmd.Start()**: `Run()` 会阻塞等待子进程退出，导致死锁。必须用 `Start()`。
- **不要假设端口立即释放**: Windows TCP 端口释放有延迟（旧进程退出 -> OS 释放端口），必须用重试机制。
- **不要在 main goroutine 中做 .old 恢复后的 self-spawn**: 恢复旧版本后需要 self-spawn + os.Exit(0)，这会终止当前进程。必须在 main() 的最早期执行 .old 检查。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Windows exe 替换 | 自己写 rename + rollback | `minio/selfupdate.Apply()` | 已在 Phase 38 集成 |
| Pushover 通知 | 自己写 HTTP 请求 | `notifier.Notifier.Notify()` | 已在 Phase 34 实现 |
| 进程创建标志 | 自己调 Windows API | `windows.SysProcAttr{HideWindow, CreationFlags}` | 已在 lifecycle/daemon.go 中使用 |
| 版本比较 | 自己写字符串比较 | `golang.org/x/mod/semver.Compare()` | 已在 selfupdate 包中使用 |

**Key insight:** 本阶段是纯集成阶段。所有底层能力（exe 替换、通知发送、进程创建）都已在前序 phase 实现。本阶段的工作是串联这些能力，添加编排逻辑（状态文件、通知时机、端口重试）。

## Common Pitfalls

### Pitfall 1: os.Exit(0) 截断完成通知
**What goes wrong:** 在 goroutine 中 `notifier.Notify()` 后立即 `os.Exit(0)`，但 Notify 是异步的（在子 goroutine 中），os.Exit 会立即终止整个进程，通知来不及发送。
**Why it happens:** TriggerHandler 的通知模式是 fire-and-forget goroutine。在 self-spawn 场景下，os.Exit(0) 紧跟在 goroutine 启动之后，通知 goroutine 可能还没执行。
**How to avoid:** 方案 A：完成通知改为同步发送（不用 goroutine），发送完再 self-spawn + os.Exit。方案 B：在 self-spawn 前加短暂 sleep 等待通知发出（不可靠）。**推荐方案 A**：完成通知同步发送，因为更新已成功，几毫秒的网络延迟不会影响用户体验。
**Warning signs:** 更新成功但用户没收到完成通知。

### Pitfall 2: .old 恢复后再次触发恢复（无限循环）
**What goes wrong:** 启动检测到 .exe.old 存在且无 .update-success -> 恢复旧版本 -> 重启 -> 旧版本启动时又检测到 .exe.old（因为恢复用的是 os.Rename，把 .old 移回 exe，但某些边缘情况下 .old 可能残留）-> 再次恢复 -> 无限循环。
**Why it happens:** os.Rename 成功后 .old 文件不再存在，但如果恢复失败（部分成功），可能留下 .old 文件。或者恢复逻辑写错（用 Copy 而非 Rename）。
**How to avoid:** (1) 恢复使用 `os.Rename(exePath+".old", exePath)` -- 原子操作，成功后 .old 不存在。(2) 恢复前先验证 .exe.old 文件确实存在且大小 > 0。(3) 恢复后 self-spawn + os.Exit(0)，新进程启动时 .old 已不存在，不会再次触发恢复。
**Warning signs:** 进程反复启动退出，日志中出现连续的 "restoring from .old backup" 消息。

### Pitfall 3: 端口重试掩盖真正的端口占用问题
**What goes wrong:** 端口被另一个无关进程占用（如用户手动启动了另一个实例），重试 5 次后绑定成功（因为那个进程刚好退出了），掩盖了配置问题。
**Why it happens:** 端口重试只应在 self-update 重启场景下短暂占用时生效，不应掩盖持久性端口冲突。
**How to avoid:** (1) 重试次数限制为 5 次（D-05），总共 2.5s。(2) 每次重试记录 Warn 日志，包含端口号和尝试次数。(3) 5 次重试后仍失败则 Fatal 退出，让用户知道端口冲突。
**Warning signs:** 日志中频繁出现端口重试警告。

### Pitfall 4: SelfUpdateHandler 测试中 Notifier 为 nil 导致 panic
**What goes wrong:** 现有测试 `newTestSelfUpdateHandler()` 没有传 Notifier，Phase 40 添加 Notifier 字段后所有现有测试会编译失败或运行时 nil panic。
**Why it happens:** 构造函数签名变更需要更新所有调用点。
**How to avoid:** Notifier 参数允许 nil（与 TriggerHandler 一致），使用前检查 `if h.notifier != nil`。更新 `newTestSelfUpdateHandler()` 和 `server.go:NewServer()` 中的调用。
**Warning signs:** 编译错误或测试 panic。

### Pitfall 5: 状态文件路径与 exe 路径不一致
**What goes wrong:** `os.Executable()` 在不同调用时可能返回不同路径（如 `C:\app\exe.exe` vs `\\?\C:\app\exe.exe`），导致状态文件和 .old 文件的路径不匹配。
**Why it happens:** Windows 路径 API 在某些情况下添加 `\\?\` 前缀。minio/selfupdate 内部有 `normalizeExecutablePath()` 处理此问题。
**How to avoid:** 使用 `os.Executable()` 获取路径后，统一用 `filepath.Clean()` 或 `filepath.Abs()` 规范化。或者直接用 `exePath + ".update-success"` 拼接（与 selfupdate.go 中的 `exePath + ".old"` 拼接方式一致）。
**Warning signs:** .update-success 文件已写入但启动时检测不到。

### Pitfall 6: Update goroutine 中 self-spawn 前需要写状态文件
**What goes wrong:** 按 D-04 设计，.update-success 状态文件应在 selfupdate.Apply 成功后写入。但如果写文件和 self-spawn 之间的时间窗口极短，状态文件可能来不及 flush 到磁盘。
**Why it happens:** os.WriteFile 默认不调用 fsync。在极端情况下（断电），文件可能存在但内容为空。
**How to avoid:** 写入 .update-success 后，可以用 `file.Sync()` 确保数据落盘。或者在读取时检查文件内容是否有效 JSON（空文件 != 成功标记）。
**Warning signs:** 断电重启后 .update-success 存在但为空文件，导致错误地清理 .old 而非恢复。

## Code Examples

### Example 1: SelfUpdateHandler goroutine with Notifier + Self-spawn
```go
// In HandleUpdate goroutine (after h.updater.Update succeeds)
// Source: Based on tmp/poc_selfupdate.go:64-74 + internal/api/trigger.go:77-94

// Write success marker BEFORE self-spawn (D-04)
exePath, _ := os.Executable()
marker := UpdateSuccessMarker{
    Timestamp:  time.Now().Format(time.RFC3339),
    NewVersion: releaseInfo.Version, // need access to release info
    OldVersion: h.version,
}
markerData, _ := json.Marshal(marker)
if err := os.WriteFile(exePath+".update-success", markerData, 0644); err != nil {
    h.logger.Error("failed to write update-success marker", "error", err)
    // Continue anyway -- .old recovery is a fallback
}

// Send completion notification SYNCHRONOUSLY (avoid Pitfall 1)
if h.notifier != nil {
    title := "Self-Update Complete"
    msg := fmt.Sprintf("Successfully updated from %s to %s", h.version, releaseInfo.Version)
    if err := h.notifier.Notify(title, msg); err != nil {
        h.logger.Error("completion notification failed", "error", err)
    }
}

// Self-spawn restart (D-01, Pattern from PoC)
cmd := exec.Command(exePath, os.Args[1:]...)
cmd.SysProcAttr = &syscall.SysProcAttr{
    HideWindow:    true,
    CreationFlags: windows.CREATE_NO_WINDOW,
}
if err := cmd.Start(); err != nil {
    h.logger.Error("failed to spawn new process", "error", err)
    return
}
os.Exit(0)
```

### Example 2: Startup .old Cleanup/Recovery in main.go
```go
// In main(), BEFORE config loading or at least BEFORE server start
// Source: D-04 state file pattern
func checkUpdateState(logger *slog.Logger) {
    exePath, err := os.Executable()
    if err != nil {
        return
    }

    oldPath := exePath + ".old"
    successPath := exePath + ".update-success"

    // Case 1: .update-success exists -> last update was successful
    if _, err := os.Stat(successPath); err == nil {
        logger.Info("previous update successful, cleaning up .old backup")
        os.Remove(oldPath)      // Clean up .old
        os.Remove(successPath)  // Clean up marker
        return
    }

    // Case 2: .old exists but no .update-success -> crash during update
    if _, err := os.Stat(oldPath); err == nil {
        logger.Warn("crash detected during update, restoring from .old backup",
            "old_path", oldPath, "exe_path", exePath)

        // Restore: os.Rename is atomic
        if err := os.Rename(oldPath, exePath); err != nil {
            logger.Error("failed to restore from .old backup", "error", err)
            return
        }

        logger.Info("restored from .old backup, restarting...")

        // Self-spawn the restored version
        cmd := exec.Command(exePath, os.Args[1:]...)
        cmd.SysProcAttr = &syscall.SysProcAttr{
            HideWindow:    true,
            CreationFlags: windows.CREATE_NO_WINDOW,
        }
        cmd.Start()
        os.Exit(0)
    }

    // Case 3: no .old and no marker -> normal startup
}
```

### Example 3: Port Binding Retry in Server.Start()
```go
// Modified Server.Start() for D-05
// Source: net.Listen + http.Server.Serve pattern
func (s *Server) Start() error {
    s.logger.Info("HTTP server starting", "addr", s.httpServer.Addr)

    // D-05: Retry port binding for self-update restart scenario
    var listener net.Listener
    var lastErr error
    for i := 0; i < 5; i++ {
        listener, lastErr = net.Listen("tcp", s.httpServer.Addr)
        if lastErr == nil {
            break
        }
        s.logger.Warn("port bind failed, retrying",
            "addr", s.httpServer.Addr,
            "attempt", i+1,
            "error", lastErr)
        time.Sleep(500 * time.Millisecond)
    }
    if lastErr != nil {
        return fmt.Errorf("failed to bind after 5 retries: %w", lastErr)
    }

    err := s.httpServer.Serve(listener)
    if err != nil && err != http.ErrServerClosed {
        return err
    }
    return nil
}
```

### Example 4: Start Notification in HandleUpdate
```go
// In HandleUpdate, BEFORE launching the goroutine (D-03: start notification)
// Source: Based on trigger.go:77-94 pattern

// Send start notification (D-03: start + complete two notifications)
if h.notifier != nil {
    title := "Self-Update Starting"
    msg := fmt.Sprintf("Current: %s", h.version)
    go func() {
        defer func() {
            if r := recover(); r != nil {
                h.logger.Error("start notification goroutine panic", "panic", r)
            }
        }()
        if err := h.notifier.Notify(title, msg); err != nil {
            h.logger.Error("start notification failed", "error", err)
        }
    }()
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| gracefu shutdown + port handoff | Direct os.Exit(0) + port retry | D-01 (Phase 40) | 简化重启逻辑，减少竞态条件 |
| Hidden .old files (no cleanup) | .update-success marker + cleanup | D-04 (Phase 40) | 解决 .old 文件累积问题 |
| No crash recovery | .old 检测 + auto-restore | D-04 (Phase 40) | 更新崩溃后自动恢复旧版本 |

**Deprecated/outdated:**
- None in this phase (pure integration of existing patterns)

## Open Questions

1. **SelfUpdateHandler 如何获取 target version 用于通知?**
   - What we know: 当前 `HandleUpdate` goroutine 直接调用 `h.updater.Update(h.version)`，不暴露 target version。但 `SelfUpdateChecker` interface 有 `NeedUpdate()` 方法可以获取 `ReleaseInfo`。
   - What's unclear: 是否需要在 goroutine 内部再调用 `NeedUpdate()` 获取目标版本（有缓存，不会多一次 API 调用），还是修改 `Update()` 方法返回更多信息。
   - Recommendation: 在 goroutine 内调用 `NeedUpdate()` 获取 release info。`NeedUpdate()` 有 1 小时缓存（selfupdate.go:27 cacheTTL），更新前刚刚检查过，必定命中缓存。通知中包含当前版本（`h.version`）和目标版本（`releaseInfo.Version`）。

2. **状态文件是否需要 fsync 确保持久化?**
   - What we know: `os.WriteFile` 不保证立即落盘。极端情况（断电）可能丢失。
   - What's unclear: 实际风险有多高。
   - Recommendation: 写入后调用 `file.Sync()`。这是一个低频操作（仅在更新成功后），额外延迟可接受。在读取时也验证文件内容是否为有效 JSON 作为双重保险。

3. **端口重试是否应该修改 Server.Start() 还是包装在 main.go?**
   - What we know: 当前 `Server.Start()` 直接调用 `httpServer.ListenAndServe()`。
   - Recommendation: 修改 `Server.Start()` 方法，将 `ListenAndServe()` 拆为 `net.Listen()` + `httpServer.Serve(listener)`，在 `net.Listen()` 处加重试逻辑。这样封装在 Server 内部，对 main.go 透明。

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Build + test | Yes | go1.24.11 windows/amd64 | -- |
| golang.org/x/sys | SysProcAttr | Yes | v0.41.0 (go.mod) | -- |
| github.com/minio/selfupdate | exe replacement | Yes | v0.6.0 (go.mod) | -- |
| github.com/gregdel/pushover | Pushover notifications | Yes | v1.4.0 (go.mod) | -- |
| github.com/stretchr/testify | Tests | Yes | v1.11.1 (go.mod) | -- |
| Windows OS | exe rename trick | Yes | Windows 10 Pro 10.0.19045 | -- |

**Missing dependencies with no fallback:**
- None -- all dependencies already in go.mod

**Missing dependencies with fallback:**
- None

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | none (standard go test) |
| Quick run command | `go test ./internal/api/ -run TestSelfUpdate -v -count=1 -timeout 30s` |
| Full suite command | `go test ./internal/api/ ./internal/selfupdate/ -v -count=1 -timeout 60s` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SAFE-01 | Self-spawn after update + port retry | unit (mock) | `go test ./internal/api/ -run TestSelfUpdate_Restart -v` | Wave 0: create |
| SAFE-01 | Port binding retry loop | unit | `go test ./internal/api/ -run TestServer_PortRetry -v` | Wave 0: create |
| SAFE-02 | Start notification sent | unit (mock notifier) | `go test ./internal/api/ -run TestSelfUpdate_StartNotify -v` | Wave 0: create |
| SAFE-02 | Completion notification sent (success) | unit (mock notifier) | `go test ./internal/api/ -run TestSelfUpdate_CompleteNotify -v` | Wave 0: create |
| SAFE-02 | Completion notification sent (failure) | unit (mock notifier) | `go test ./internal/api/ -run TestSelfUpdate_FailureNotify -v` | Wave 0: create |
| SAFE-03 | .old cleanup on successful startup | unit | `go test ./cmd/nanobot-auto-updater/ -run TestCleanup -v` or separate package | Wave 0: create |
| SAFE-04 | .old recovery on crash detection | unit | `go test ./cmd/nanobot-auto-updater/ -run TestRecovery -v` or separate package | Wave 0: create |
| SAFE-04 | Normal startup (no .old, no marker) | unit | Same test file, test no-op path | Wave 0: create |

### Sampling Rate
- **Per task commit:** `go test ./internal/api/ -run TestSelfUpdate -count=1 -timeout 30s`
- **Per wave merge:** `go test ./internal/api/ ./internal/selfupdate/ -v -count=1 -timeout 60s`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/api/selfupdate_handler_test.go` -- extend existing tests for Notifier + restart (mockNotifier already exists in trigger_test.go, can reuse pattern)
- [ ] `internal/api/server_test.go` -- add port retry test
- [ ] Consider extracting .old cleanup/recovery to testable function (e.g., `internal/lifecycle/update_state.go`) since main package is hard to test
- [ ] cmd/nanobot-auto-updater tests: existing tests have goroutine leak timeout issues. Recommend isolating .old logic in a separate testable package.

## Sources

### Primary (HIGH confidence)
- `internal/api/selfupdate_handler.go` -- Current handler with goroutine + panic recovery. All integration points identified.
- `internal/api/trigger.go` -- Notifier injection pattern (constructor + nil-safe + async send). Proven pattern to follow.
- `internal/notifier/notifier.go` -- Notifier interface with Notify(title, message). Already supports nil-safe (IsEnabled check).
- `tmp/poc_selfupdate.go:64-74` -- PoC-validated self-spawn pattern (cmd.Start + SysProcAttr + os.Exit(0)).
- `internal/lifecycle/daemon.go:49-92` -- Existing project self-spawn code with SysProcAttr + CREATE_NO_WINDOW.
- `internal/selfupdate/selfupdate.go:351-360` -- OldSavePath already set to `exePath + ".old"`.
- `cmd/nanobot-auto-updater/main.go:147-164` -- Current server startup flow, integration point for port retry.
- `internal/api/server.go:118-125` -- Server.Start() method, needs modification for port retry.

### Secondary (MEDIUM confidence)
- Phase 36 RESEARCH -- Self-spawn pattern analysis, Windows process creation pitfalls.
- Phase 38 RESEARCH -- minio/selfupdate behavior, .old file management.
- `.planning/research/PITFALLS.md` -- Pitfall 7: .exe.old file accumulation on Windows.
- [Go Forum: bind address already in use](https://forum.golangbridge.org/t/bind-address-already-in-use-even-after-listener-closed/1510) -- TIME_WAIT behavior on port release.

### Tertiary (LOW confidence)
- [Stack Overflow: golang errors with bind address already in use](https://stackoverflow.com/questions/29924910/golang-errors-with-bind-address-already-in-use-even-though-nothing-is-running-on) -- Port binding race condition patterns.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all libraries already in go.mod, no new dependencies
- Architecture: HIGH -- all patterns have project-internal precedents (TriggerHandler Notifier injection, PoC self-spawn, lifecycle daemon restart)
- Pitfalls: HIGH -- identified from existing code analysis and PoC validation experience

**Research date:** 2026-03-30
**Valid until:** 2026-04-30 (stable codebase, no external dependency changes expected)

# Phase 41: Startup Notification - Research

**Researched:** 2026-04-06
**Domain:** Golang async notification integration with existing Pushover + auto-start infrastructure
**Confidence:** HIGH

## Summary

Phase 41 在应用启动时自动启动所有实例完成后，发送一条聚合 Pushover 通知，汇总每个实例的启动结果（成功或失败含错误详情）。这是一个纯集成任务，所有基础设施已就绪：`notifier.Notifier`（Phase 9，含 `IsEnabled()` 优雅降级）、`instance.InstanceManager.StartAllInstances()`（Phase 24，返回 `AutoStartResult`）、异步通知发送模式（Phase 27/34，goroutine + panic recovery）、依赖注入模式（Phase 30/34）。

改动范围极小：`cmd/nanobot-auto-updater/main.go`（auto-start goroutine 完成后发送聚合通知）和可能的 `internal/notifier/notifier.go`（新增启动通知格式化方法，可选）。核心逻辑约 30-50 行新增代码。关键设计决策：复用 `AutoStartResult` 结构（已有 `Started`/`Failed`/`Skipped` 字段），在 auto-start goroutine 内部完成通知发送。

**Primary recommendation:** 完全复用 Phase 34 的异步通知模式和 Phase 27 的 panic recovery 模式。在 `main.go` 的 auto-start goroutine 中，`StartAllInstances()` 返回后立即异步发送格式化的聚合通知。不创建新组件、不引入新依赖、不新建文件。

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| STRT-01 | 多实例启动结果聚合成一条 Pushover 通知（包含每个实例的启动状态） | `AutoStartResult` 已包含 `Started`/`Failed`/`Skipped` 字段；`Failed` 中的 `InstanceError` 包含实例名、端口和错误详情；格式化方法可参考 `notifier.formatUpdateResultMessage()` |
| STRT-02 | 异步发送启动通知，不阻塞启动流程 | auto-start 本身已在 goroutine 中执行（main.go L204-225）；通知发送在该 goroutine 内部完成，不阻塞主流程；复用 Phase 34 的 goroutine + panic recovery 模式 |
| STRT-03 | Pushover 未配置时优雅降级（跳过通知） | `notifier.Notifier.Notify()` 内部已有 `IsEnabled()` 检查，未配置时记录 DEBUG 日志并返回 nil（notifier.go L93-97）；无需额外判断逻辑 |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| gregdel/pushover | v1.4.0 | Pushover API 客户端 | 项目已依赖，Notifier 内部使用 `[VERIFIED: go.mod]` |
| slog (stdlib) | Go 1.24 | 结构化日志 | 项目标准日志库 `[VERIFIED: codebase]` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| stretchr/testify | v1.11.1 | 测试断言 | 编写启动通知测试时可选使用 `[VERIFIED: go.mod]` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| 在 main.go 内联格式化 | 新建 `internal/startup` 包 | 过度抽象，仅约 30 行代码，不值得独立包 |
| 修改 `StartAllInstances` 接受 Notifier | 在调用者处发送通知 | 修改返回值签名更干净，保持关注点分离；调用者控制通知策略 |

**Installation:**
无需新增依赖。所有依赖已在 go.mod 中。

**Version verification:**
```
gregdel/pushover v1.4.0 (已在 go.mod)
Go 1.24.11 (已安装)
```

## Architecture Patterns

### Recommended Project Structure
```
cmd/nanobot-auto-updater/
└── main.go                    # [改动] auto-start goroutine 中增加通知逻辑

internal/notifier/
└── notifier.go                # [可选改动] 新增 NotifyStartupResult() 方法

internal/instance/
└── manager.go                 # [不改动] AutoStartResult 已满足需求
└── errors.go                  # [不改动] InstanceError 已含完整信息
```

### Pattern 1: 异步通知 + Panic Recovery（Phase 34 模式）
**What:** 在 goroutine 中发送通知，带 panic recovery，失败仅记录 ERROR
**When to use:** 任何非关键通知发送
**Example:**
```go
// Source: internal/api/trigger.go (Phase 34 已有模式)
go func() {
    defer func() {
        if r := recover(); r != nil {
            logger.Error("notification goroutine panic",
                "panic", r,
                "stack", string(debug.Stack()))
        }
    }()
    if err := notif.Notify(title, message); err != nil {
        logger.Error("notification failed", "error", err)
    }
}()
```

### Pattern 2: 聚合结果格式化（Phase 34 formatCompletionMessage 模式）
**What:** 将 AutoStartResult 格式化为用户友好的通知消息
**When to use:** 启动通知消息构建
**Example:**
```go
// Source: 参考内部 api/trigger.go formatCompletionMessage 和 notifier.go formatUpdateResultMessage
func formatStartupMessage(result *instance.AutoStartResult) string {
    var msg strings.Builder
    // 成功实例
    for _, name := range result.Started {
        msg.WriteString(fmt.Sprintf("  OK %s\n", name))
    }
    // 失败实例
    for _, err := range result.Failed {
        msg.WriteString(fmt.Sprintf("  FAIL %s: %v\n", err.InstanceName, err.Err))
    }
    return msg.String()
}
```

### Pattern 3: AutoStartResult 数据结构（已存在）
**What:** Phase 24 已定义的结构，包含启动汇总信息
**When to use:** 获取启动结果数据
**Example:**
```go
// Source: internal/instance/manager.go L188-194
type AutoStartResult struct {
    Started []string         `json:"started"` // 成功启动的实例名称
    Failed  []*InstanceError `json:"failed"`  // 启动失败的实例错误
    Skipped []string         `json:"skipped"` // 跳过自动启动的实例 (auto_start: false)
}
```

### Anti-Patterns to Avoid
- **阻塞主流程等待通知:** 通知发送必须是非阻塞的；auto-start goroutine 内的通知发送应在 StartAllInstances 返回后立即执行，不需要额外的 goroutine 嵌套（因为 auto-start 本身已在 goroutine 中）
- **在 InstanceManager 中注入 Notifier:** 违反关注点分离；InstanceManager 管理实例生命周期，不应关心通知逻辑；通知策略应由调用者（main.go）决定
- **创建新的通知接口:** 项目已有 `notifier.Notifier` 和 `notification.Notifier` 两个接口；直接使用 `*notifier.Notifier` 具体类型（与 Phase 34 模式一致）
- **通知包含 Skipped 实例:** Skipped 实例（auto_start=false）不应出现在启动通知中，用户已明确选择不自动启动它们

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Pushover 未配置检测 | 新增 IsEnabled() 检查逻辑 | `notifier.Notify()` 内部已有 | 内部已处理，返回 nil，记录 DEBUG `[VERIFIED: notifier.go L93-97]` |
| Panic recovery | 自定义 recovery 逻辑 | `defer func() { recover() }()` 模式 | Phase 27/34 已验证的标准模式 |
| 实例启动结果汇总 | 新建结果聚合器 | `AutoStartResult` 已有完整信息 | Phase 24 已定义，含 Started/Failed/Skipped |

**Key insight:** 这是一个最小集成任务。核心改动仅在 `main.go` 的 auto-start goroutine 中增加约 30 行通知逻辑。可选在 `notifier.go` 中增加格式化方法以提高可测试性。

## Common Pitfalls

### Pitfall 1: auto-start goroutine 中通知失败导致 panic
**What goes wrong:** 如果通知逻辑 panic 且未 recovery，整个 auto-start goroutine 崩溃
**Why it happens:** auto-start goroutine 已有 panic recovery（main.go L206-210），但通知逻辑在 recovery 范围内，如果 panic 会被外层 recovery 捕获
**How to avoid:** 通知发送本身已在 defer recover 范围内执行；但通知内部的 goroutine 需要**独立的** panic recovery（因为外层 goroutine 的 recover 无法捕获子 goroutine 的 panic）
**Warning signs:** 通知逻辑使用了 `go func()` 但没有 `defer recover()`

### Pitfall 2: STRT-03 误解 — 需要静默跳过而非 DEBUG 日志
**What goes wrong:** 需求要求"不记录错误或警告"，但当前 `notifier.Notify()` 在未配置时记录 DEBUG 日志
**Why it happens:** STRT-03 原文: "the startup notification is silently skipped with no errors or warnings logged" — DEBUG 级别不算错误或警告
**How to avoid:** 确认 DEBUG 日志级别满足"不记录错误或警告"的要求（DEBUG < INFO < WARN < ERROR）。当前行为已满足 STRT-03。无需修改 Notifier 行为。
**Warning signs:** 如果有人尝试在 startup notification 路径上增加 WARN 级别日志

### Pitfall 3: Skipped 实例出现在通知中
**What goes wrong:** 将 auto_start=false 的实例也列在启动通知中，产生误导
**Why it happens:** AutoStartResult 包含 Skipped 字段，容易直接遍历所有字段
**How to avoid:** 通知中只包含 Started 和 Failed 实例；Skipped 实例静默忽略
**Warning signs:** 格式化逻辑中引用 `result.Skipped`

### Pitfall 4: 零实例场景发送空通知
**What goes wrong:** 所有实例都设置了 auto_start=false 时，通知内容为空
**Why it happens:** 没有检查 AutoStartResult 是否有实际内容
**How to avoid:** 如果 `len(result.Started) == 0 && len(result.Failed) == 0`，跳过通知发送
**Warning signs:** 格式化后的消息为空字符串

### Pitfall 5: 通知中包含过多技术细节
**What goes wrong:** 在 Pushover 通知中包含完整的 error stack 或端口信息
**Why it happens:** InstanceError 包含 Port 和 Err 字段，容易直接全部输出
**How to avoid:** 通知应简洁：实例名 + 成功/失败状态 + 失败原因（一句话），详细信息留给日志。参考 Phase 34 D-02 的设计哲学。
**Warning signs:** 通知消息超过 500 字符

## Code Examples

### Example 1: main.go auto-start goroutine 改动（核心集成点）
```go
// Source: cmd/nanobot-auto-updater/main.go L204-225 (现有代码)
// 改动: 在 StartAllInstances() 调用后增加通知逻辑

go func() {
    defer func() {
        if r := recover(); r != nil {
            logger.Error("auto-start goroutine panic",
                "panic", r,
                "stack", string(debug.Stack()))
        }
    }()

    autoStartTimeout := 5 * time.Minute
    autoStartCtx, cancel := context.WithTimeout(context.Background(), autoStartTimeout)
    defer cancel()

    logger.Info("Starting auto-start for all instances",
        "instance_count", len(cfg.Instances),
        "timeout", autoStartTimeout)

    // Execute auto-start and get result
    result := instanceManager.StartAllInstances(autoStartCtx)

    // STRT-01, STRT-02: Send aggregated startup notification
    // STRT-03: notif.Notify() handles graceful degradation internally
    sendStartupNotification(notif, result, logger)
}()
```

### Example 2: 启动通知发送函数
```go
// Source: 新增函数，参考 Phase 34 异步通知模式
func sendStartupNotification(notif *notifier.Notifier, result *instance.AutoStartResult, logger *slog.Logger) {
    if notif == nil || result == nil {
        return
    }

    // Skip if no instances were started or failed (all skipped)
    if len(result.Started) == 0 && len(result.Failed) == 0 {
        logger.Debug("No instances to report in startup notification")
        return
    }

    title, message := formatStartupNotification(result)
    
    // STRT-02: Async send (already in goroutine, but use separate goroutine
    // for notification to keep consistent with Phase 34 pattern)
    go func() {
        defer func() {
            if r := recover(); r != nil {
                logger.Error("startup notification goroutine panic",
                    "panic", r,
                    "stack", string(debug.Stack()))
            }
        }()
        if err := notif.Notify(title, message); err != nil {
            logger.Error("Failed to send startup notification", "error", err)
        }
    }()
}
```

### Example 3: 启动通知格式化
```go
// Source: 参考 notifier.formatUpdateResultMessage 和 api.formatCompletionMessage
func formatStartupNotification(result *instance.AutoStartResult) (string, string) {
    var msg strings.Builder

    totalAttempted := len(result.Started) + len(result.Failed)

    if len(result.Failed) > 0 {
        title := "Nanobot startup partially failed"
        if len(result.Started) == 0 {
            title = "Nanobot startup failed"
        }
        msg.WriteString(fmt.Sprintf("Started: %d/%d\n", len(result.Started), totalAttempted))
        for _, name := range result.Started {
            msg.WriteString(fmt.Sprintf("  OK %s\n", name))
        }
        msg.WriteString("Failed:\n")
        for _, err := range result.Failed {
            msg.WriteString(fmt.Sprintf("  FAIL %s: %v\n", err.InstanceName, err.Err))
        }
        return title, msg.String()
    }

    return "Nanobot startup completed",
        fmt.Sprintf("All %d instances started successfully", len(result.Started))
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| pushover.NewMessage(message) | pushover.NewMessageWithTitle(message, title) | Phase 34 | 支持通知标题，更清晰的推送显示 |

**Deprecated/outdated:**
- 无已废弃的 API 使用

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | DEBUG 日志级别满足 STRT-03 "no errors or warnings logged" 的要求 | Pitfalls #2 | 如果需要完全不记录任何日志，需修改 notifier.Notify() 的行为（低风险，DEBUG 级别通常不被视为"错误或警告"） |
| A2 | auto-start goroutine 中的 panic recovery 能覆盖通知逻辑的 panic（如果不在子 goroutine 中发送） | Pitfalls #1 | 如果通知在子 goroutine 中发送需要独立 recovery |
| A3 | 通知中不需要包含 Skipped 实例 | Anti-Patterns #4 | 用户可能希望知道哪些实例被跳过了（低风险，auto_start=false 是用户主动配置的） |

**If this table is empty:** All claims in this research were verified or cited -- no user confirmation needed.

## Open Questions

1. **通知格式化方法放置位置**
   - What we know: 可选放在 main.go 内联、notifier.go 新增方法、或独立函数
   - What's unclear: 哪个位置最符合项目惯例
   - Recommendation: 放在 main.go 作为包级函数（与 auto-start 逻辑在同一文件），不需要新建文件。如需测试可导出到独立的 test 文件。参考 Phase 34 的 formatCompletionMessage 放在 trigger.go 中（与调用者同文件）。

2. **是否需要在 auto-start goroutine 内额外嵌套 goroutine 发送通知**
   - What we know: auto-start 已在 goroutine 中执行，直接调用 Notify() 不会阻塞主流程
   - What's unclear: 是否需要与 Phase 34 保持一致使用子 goroutine
   - Recommendation: 不需要额外 goroutine。auto-start 本身已在 goroutine 中，直接同步调用 Notify() 即可（即使 Pushover API 调用耗时几秒，也不阻塞主流程）。但如果团队偏好与 Phase 34 模式完全一致，可以使用子 goroutine。

## Environment Availability

> Step 2.6: Phase 依赖检查 — 无外部依赖，纯代码改动。

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.24+ | 编译 | Yes | go1.24.11 | -- |
| gregdel/pushover | Pushover API | Yes | v1.4.0 | -- |

**Missing dependencies with no fallback:** None

**Missing dependencies with fallback:** None

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) + stretchr/testify |
| Config file | none |
| Quick run command | `go test ./internal/notifier/... -v -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| STRT-01 | Aggregated notification with all instance statuses | unit | `go test ./internal/notifier/... -run TestNotifyStartupResult -v` | No -- Wave 0 |
| STRT-02 | Async notification does not block startup | unit | `go test ./cmd/nanobot-auto-updater/... -run TestStartupNotification -v` | No -- Wave 0 |
| STRT-03 | Graceful degradation when Pushover not configured | unit | `go test ./internal/notifier/... -run TestNotifyStartupResult_Disabled -v` | No -- Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/notifier/... ./internal/instance/... -v -count=1`
- **Per wave merge:** `go test ./... -count=1` (excluding lifecycle package with known test compilation error)
- **Phase gate:** Full suite green (excluding known pre-existing lifecycle/capture_test.go compilation error)

### Wave 0 Gaps
- [ ] `internal/notifier/notifier_ext_test.go` -- Add startup notification tests (TestNotifyStartupResult, TestFormatStartupMessage)
- [ ] `cmd/nanobot-auto-updater/main_test.go` -- Optionally add integration-level tests for notification wiring

## Security Domain

> No new security boundaries introduced. Phase reuses existing Notifier component with existing Pushover credentials.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | -- |
| V3 Session Management | no | -- |
| V4 Access Control | no | -- |
| V5 Input Validation | no | -- |
| V6 Cryptography | no | -- |

### Known Threat Patterns

No new threat patterns introduced. Startup notification is a one-way push notification using existing Pushover credentials.

## Sources

### Primary (HIGH confidence)
- Codebase inspection: `internal/notifier/notifier.go` -- Notify(), IsEnabled(), NotifyUpdateResult(), formatUpdateResultMessage() `[VERIFIED: direct file read]`
- Codebase inspection: `cmd/nanobot-auto-updater/main.go` -- auto-start goroutine, Notifier creation, dependency injection `[VERIFIED: direct file read]`
- Codebase inspection: `internal/instance/manager.go` -- StartAllInstances(), AutoStartResult struct `[VERIFIED: direct file read]`
- Codebase inspection: `internal/api/trigger.go` -- Phase 34 notification integration pattern `[VERIFIED: direct file read]`
- Codebase inspection: `internal/notification/manager_test.go` -- MockNotifier test pattern `[VERIFIED: direct file read]`
- go.mod -- dependency versions `[VERIFIED: direct file read]`

### Secondary (MEDIUM confidence)
- `.planning/milestones/v0.7-phases/34-update-notification-integration/34-RESEARCH.md` -- Phase 34 notification research for pattern reference `[CITED: project archive]`

### Tertiary (LOW confidence)
- None -- all findings verified against codebase

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all dependencies verified in go.mod, no new dependencies needed
- Architecture: HIGH -- follows established Phase 27/34 patterns, minimal code changes
- Pitfalls: HIGH -- based on direct codebase analysis of existing notification patterns

**Research date:** 2026-04-06
**Valid until:** 2026-05-06 (stable codebase, no external API changes expected)

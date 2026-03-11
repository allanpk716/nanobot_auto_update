# Phase 10: 主程序集成 - Research

**Researched:** 2026-03-11
**Domain:** Go 应用集成、长期运行服务、优雅关闭、Context 传播
**Confidence:** HIGH

## Summary

Phase 10 需要将已完成的 InstanceManager 集成到主程序中,实现完整的多实例更新流程。当前主程序使用 v1.0 单实例模式(lifecycle.Manager),需要重构为支持多实例调度执行。关键技术挑战包括:配置模式检测(legacy vs multi-instance)、调度器集成、Context 传播、长期运行稳定性以及资源管理。

**Primary recommendation:** 在 main.go 中添加配置模式检测逻辑,根据 len(cfg.Instances) > 0 决定使用 InstanceManager(多实例)还是 legacy lifecycle.Manager(单实例),两种模式共享相同的调度器和通知基础设施。

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| robfig/cron/v3 | 3.0.1 | Cron 调度器 | Phase 3 已采用,支持 SkipIfStillRunning 模式防止任务重叠 |
| context | 标准库 | 生命周期管理 | Go 惯用模式,支持超时、取消、值传播 |
| os/signal | 标准库 | 信号处理 | 跨平台支持 SIGINT/SIGTERM 优雅关闭 |
| log/slog | 标准库 | 结构化日志 | Phase 1 已采用,所有组件统一使用 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| spf13/pflag | 1.0.10 | CLI 参数解析 | 已用于 --update-now, --timeout, --config 等标志 |
| spf13/viper | 1.21.0 | 配置加载 | 已用于 YAML 配置加载和验证 |
| gregdel/pushover | 1.4.0 | Pushover 通知 | 已集成 NotifyUpdateResult 方法 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| robfig/cron | gocron | robfig/cron 更轻量,Phase 3 已集成,无需更换 |
| context.Background() | context.WithTimeout() | 定时任务使用 Background(),--update-now 使用 WithTimeout() (已实现) |

**Installation:**
已安装,无需新增依赖。

## Architecture Patterns

### Recommended Project Structure
```
cmd/nanobot-auto-updater/
├── main.go                # 入口点,模式检测,调度器集成
internal/
├── instance/
│   ├── manager.go         # InstanceManager 协调器 (Phase 8 已完成)
│   ├── result.go          # UpdateResult 结果聚合 (Phase 8 已完成)
│   └── lifecycle.go       # InstanceLifecycle 包装器 (Phase 7 已完成)
├── notifier/
│   └── notifier.go        # NotifyUpdateResult 扩展 (Phase 9 已完成)
├── scheduler/
│   └── scheduler.go       # 调度器 (Phase 3 已完成)
└── config/
    └── config.go          # 配置加载和验证 (Phase 6 已完成)
```

### Pattern 1: 配置模式检测
**What:** 根据配置内容动态选择单实例或多实例模式
**When to use:** main.go 启动时,加载配置后立即检测
**Example:**
```go
// Source: internal/config/config.go
func (c *Config) ValidateModeCompatibility() error {
    hasLegacyMode := c.Nanobot.Port != 0
    hasNewMode := len(c.Instances) > 0

    if hasLegacyMode && hasNewMode {
        return fmt.Errorf("配置错误: 不能同时使用 'nanobot' section 和 'instances' 数组")
    }
    return nil
}

// main.go 中使用
func main() {
    cfg, err := config.Load(*configFile)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
        os.Exit(1)
    }

    // 模式检测
    useMultiInstance := len(cfg.Instances) > 0

    if useMultiInstance {
        // 多实例模式
        manager := instance.NewInstanceManager(cfg, logger)
        // ... 调度器集成
    } else {
        // Legacy 单实例模式 (v1.0 兼容)
        lifecycleCfg := lifecycle.Config{
            Port:           cfg.Nanobot.Port,
            StartupTimeout: cfg.Nanobot.StartupTimeout,
        }
        lifecycleMgr := lifecycle.NewManager(lifecycleCfg, logger)
        // ... 现有逻辑
    }
}
```

### Pattern 2: 调度器集成 - 多实例模式
**What:** 使用 robfig/cron 调度 InstanceManager.UpdateAll()
**When to use:** 定时任务触发时,执行完整的停止→更新→启动流程
**Example:**
```go
// Source: 参考 cmd/nanobot-auto-updater/main.go L238-L261
// 多实例模式的调度器集成
err = sched.AddJob(cfg.Cron, func() {
    logger.Info("Starting scheduled multi-instance update job")

    ctx := context.Background() // 定时任务使用 Background context
    result, err := manager.UpdateAll(ctx)

    if err != nil {
        logger.Error("Scheduled multi-instance update failed",
            "error", err.Error())

        // UV 更新失败,发送通用失败通知
        if notifyErr := notif.NotifyFailure("Scheduled Multi-Instance Update", err); notifyErr != nil {
            logger.Error("Failed to send failure notification", "error", notifyErr.Error())
        }
        return
    }

    // 检查实例级别失败并发送详细通知
    if result.HasErrors() {
        logger.Error("Some instances failed during scheduled update",
            "stop_failed", len(result.StopFailed),
            "start_failed", len(result.StartFailed))

        if notifyErr := notif.NotifyUpdateResult(result); notifyErr != nil {
            logger.Error("Failed to send instance failure notification", "error", notifyErr.Error())
        }
    } else {
        logger.Info("Scheduled multi-instance update completed successfully",
            "stopped", len(result.Stopped),
            "started", len(result.Started))
    }
})
```

### Pattern 3: --update-now 模式集成
**What:** --update-now 立即执行一次多实例更新并退出
**When to use:** 用户手动触发更新或 nanobot 调用
**Example:**
```go
// Source: 参考 cmd/nanobot-auto-updater/main.go L137-L228
// 多实例 --update-now 模式
if *updateNow {
    logger.Info("Executing immediate multi-instance update", "timeout", timeout.String())

    ctx, cancel := context.WithTimeout(context.Background(), *timeout)
    defer cancel()

    result := UpdateNowResult{}

    // 执行多实例更新
    updateResult, err := manager.UpdateAll(ctx)
    if err != nil {
        // UV 更新失败 (关键错误)
        logger.Error("Multi-instance update failed", "error", err.Error())

        // 发送通用失败通知
        notif.NotifyFailure("Multi-Instance Update", err)

        result = UpdateNowResult{
            Success:  false,
            Error:    err.Error(),
            ExitCode: 1,
        }
        outputJSON(result)
        os.Exit(1)
    }

    // 检查实例级别失败并发送详细通知
    if updateResult.HasErrors() {
        logger.Error("Some instances failed during multi-instance update",
            "stop_failed", len(updateResult.StopFailed),
            "start_failed", len(updateResult.StartFailed))

        if notifyErr := notif.NotifyUpdateResult(updateResult); notifyErr != nil {
            logger.Error("Failed to send instance failure notification", "error", notifyErr.Error())
        }

        // 即使有实例失败,UV 更新成功,整体算成功
        result = UpdateNowResult{
            Success: true,
            Source:  "multi-instance",
            Message: fmt.Sprintf("Update completed with %d instance failures", len(updateResult.StopFailed)+len(updateResult.StartFailed)),
        }
    } else {
        result = UpdateNowResult{
            Success: true,
            Source:  "multi-instance",
            Message: "All instances updated successfully",
        }
    }

    logger.Info("Multi-instance update completed", "result", result)
    outputJSON(result)
    os.Exit(0)
}
```

### Pattern 4: 优雅关闭集成
**What:** 使用 os/signal 捕获 SIGINT/SIGTERM 并优雅停止调度器
**When to use:** 程序退出时,确保调度器完成当前任务
**Example:**
```go
// Source: cmd/nanobot-auto-updater/main.go L267-L282
// 已实现,无需修改
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

sched.Start()
logger.Info("Scheduler started", "cron", cfg.Cron, "pid", os.Getpid())

sig := <-sigChan
logger.Info("Shutdown signal received", "signal", sig.String())

sched.Stop() // 等待当前任务完成 (SkipIfStillRunning 确保不会重叠)
logger.Info("Application shutdown complete")
```

### Anti-Patterns to Avoid
- **在定时任务中使用 context.WithTimeout():** 定时任务应使用 context.Background(),超时由 Updater 内部控制
- **忽略 UpdateResult.HasErrors():** 即使 UV 更新成功,实例失败也需要发送通知
- **混合使用单实例和多实例模式:** 配置验证已防止此问题,但集成代码需要明确模式检测
- **在 InstanceManager 外部调用 lifecycle.Manager:** 多实例模式下,所有生命周期操作通过 InstanceManager 协调

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 定时任务调度 | 自建调度器 | robfig/cron | 已集成,支持 SkipIfStillRunning,成熟稳定 |
| 信号处理 | 自建信号捕获 | os/signal + signal.Notify | 标准库,跨平台支持 |
| Context 传播 | 全局变量 | context.Context 参数传递 | Go 惯用模式,支持取消和超时 |
| 日志注入 | 手动拼接字段 | logger.With("key", value) | slog 结构化日志,Phase 1 已标准化 |
| 模式检测 | 字符串比较 | len(cfg.Instances) > 0 | 配置验证已实现模式兼容性检查 |

**Key insight:** Phase 10 是集成阶段,不应引入新的核心逻辑,复用 Phase 6-9 已实现的组件。

## Common Pitfalls

### Pitfall 1: Context 泄漏导致定时任务无法执行
**What goes wrong:** 在定时任务中使用 context.WithTimeout(),超时后 context 取消,后续任务无法执行
**Why it happens:** 误认为定时任务需要超时控制,实际应由 Updater 内部控制
**How to avoid:** 定时任务使用 context.Background(),--update-now 使用 context.WithTimeout()
**Warning signs:** 定时任务第一次执行成功,后续任务全部超时

### Pitfall 2: 忽略实例级别失败
**What goes wrong:** 只检查 UV 更新错误,忽略 UpdateResult.HasErrors()
**Why it happens:** InstanceManager.UpdateAll() 返回 error 时仅表示 UV 更新失败,实例失败记录在 UpdateResult 中
**How to avoid:** 双层错误检查:先检查 err (UV 更新失败),再检查 result.HasErrors() (实例失败)
**Warning signs:** 实例停止/启动失败但未收到通知

### Pitfall 3: 内存泄漏 - InstanceLifecycle 未释放资源
**What goes wrong:** 长期运行后内存持续增长
**Why it happens:** InstanceLifecycle 持有 logger 引用,但 logger 本身是全局的,无需释放
**How to avoid:** 确保没有在循环中创建未关闭的资源(goroutine、channel、文件句柄)
**Warning signs:** 运行 24 小时后内存使用显著增长

### Pitfall 4: 配置模式检测错误
**What goes wrong:** 同时存在 cfg.Nanobot.Port 和 cfg.Instances 时使用错误的逻辑
**Why it happens:** 未调用 cfg.ValidateModeCompatibility() 或忽略错误
**How to avoid:** config.Load() 已包含模式验证,直接使用 len(cfg.Instances) > 0 检测模式
**Warning signs:** 程序启动时报错 "配置错误: 不能同时使用 'nanobot' section 和 'instances' 数组"

## Code Examples

### 配置模式检测和分支
```go
// Source: 基于 cmd/nanobot-auto-updater/main.go 改造
func main() {
    // ... flag 解析,配置加载

    cfg, err := config.Load(*configFile)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
        os.Exit(1)
    }

    // 模式检测 (配置验证已确保不会同时存在两种模式)
    useMultiInstance := len(cfg.Instances) > 0

    if useMultiInstance {
        logger.Info("Running in multi-instance mode", "instance_count", len(cfg.Instances))
        runMultiInstanceMode(cfg, logger, notif, sched, *updateNow, *timeout)
    } else {
        logger.Info("Running in legacy single-instance mode", "port", cfg.Nanobot.Port)
        runLegacyMode(cfg, logger, notif, sched, *updateNow, *timeout)
    }
}

func runMultiInstanceMode(cfg *config.Config, logger *slog.Logger, notif *notifier.Notifier, sched *scheduler.Scheduler, updateNow bool, timeout time.Duration) {
    manager := instance.NewInstanceManager(cfg, logger)

    if updateNow {
        executeMultiInstanceUpdateNow(manager, logger, notif, timeout)
        return
    }

    scheduleMultiInstanceUpdates(manager, logger, notif, sched, cfg.Cron)
    waitForShutdown(sched, logger)
}
```

### 多实例定时任务集成
```go
// Source: 新增代码,参考 cmd/nanobot-auto-updater/main.go L238-L261
func scheduleMultiInstanceUpdates(manager *instance.InstanceManager, logger *slog.Logger, notif *notifier.Notifier, sched *scheduler.Scheduler, cronExpr string) {
    err := sched.AddJob(cronExpr, func() {
        logger.Info("Starting scheduled multi-instance update job")

        ctx := context.Background() // 定时任务使用 Background context
        result, err := manager.UpdateAll(ctx)

        if err != nil {
            // UV 更新失败 (关键错误)
            logger.Error("Scheduled multi-instance update failed",
                "error", err.Error())

            if notifyErr := notif.NotifyFailure("Scheduled Multi-Instance Update", err); notifyErr != nil {
                logger.Error("Failed to send failure notification", "error", notifyErr.Error())
            }
            return
        }

        // 检查实例级别失败
        if result.HasErrors() {
            logger.Error("Some instances failed during scheduled update",
                "stop_failed", len(result.StopFailed),
                "start_failed", len(result.StartFailed))

            if notifyErr := notif.NotifyUpdateResult(result); notifyErr != nil {
                logger.Error("Failed to send instance failure notification", "error", notifyErr.Error())
            }
        } else {
            logger.Info("Scheduled multi-instance update completed successfully",
                "stopped", len(result.Stopped),
                "started", len(result.Started))
        }
    })

    if err != nil {
        logger.Error("Failed to register scheduled job", "error", err.Error())
        os.Exit(1)
    }
}
```

### 多实例 --update-now 集成
```go
// Source: 新增代码,参考 cmd/nanobot-auto-updater/main.go L137-L228
func executeMultiInstanceUpdateNow(manager *instance.InstanceManager, logger *slog.Logger, notif *notifier.Notifier, timeout time.Duration) {
    logger.Info("Executing immediate multi-instance update", "timeout", timeout.String())

    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    result := UpdateNowResult{}

    updateResult, err := manager.UpdateAll(ctx)
    if err != nil {
        // UV 更新失败 (关键错误)
        logger.Error("Multi-instance update failed", "error", err.Error())

        notif.NotifyFailure("Multi-Instance Update", err)

        result = UpdateNowResult{
            Success:  false,
            Error:    err.Error(),
            ExitCode: 1,
        }
        outputJSON(result)
        os.Exit(1)
    }

    // 检查实例级别失败
    if updateResult.HasErrors() {
        logger.Error("Some instances failed during multi-instance update",
            "stop_failed", len(updateResult.StopFailed),
            "start_failed", len(updateResult.StartFailed))

        if notifyErr := notif.NotifyUpdateResult(updateResult); notifyErr != nil {
            logger.Error("Failed to send instance failure notification", "error", notifyErr.Error())
        }

        // UV 更新成功,即使有实例失败也算成功
        result = UpdateNowResult{
            Success: true,
            Source:  "multi-instance",
            Message: fmt.Sprintf("Update completed with %d instance failures",
                len(updateResult.StopFailed)+len(updateResult.StartFailed)),
        }
    } else {
        result = UpdateNowResult{
            Success: true,
            Source:  "multi-instance",
            Message: "All instances updated successfully",
        }
    }

    logger.Info("Multi-instance update completed",
        "stopped", len(updateResult.Stopped),
        "started", len(updateResult.Started))

    outputJSON(result)
    os.Exit(0)
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| 单实例 lifecycle.Manager | 多实例 InstanceManager | Phase 8 | 支持同时管理多个 nanobot 实例 |
| 通用失败通知 | 详细实例失败通知 | Phase 9 | NotifyUpdateResult 列出每个实例的成功/失败状态 |
| 手动停止/启动 | 自动协调停止→更新→启动 | Phase 8 | InstanceManager.UpdateAll() 编排完整流程 |
| 配置固定模式 | 动态模式检测 | Phase 6 | 自动识别 legacy 或 multi-instance 模式 |

**Deprecated/outdated:**
- v1.0 单实例模式: 仍然支持用于向后兼容,但新功能将优先在多实例模式中实现
- --run-once 标志: Phase 5 已移除,替换为 --update-now

## Open Questions

1. **长期运行稳定性测试策略**
   - What we know: 理论上 goroutine 和资源管理正确,但缺乏 24x7 实际运行验证
   - What's unclear: 内存泄漏可能出现的场景(循环引用、未关闭 channel、goroutine 泄漏)
   - Recommendation: 建议在 Phase 10 验证阶段添加内存监控和压力测试,运行至少 48 小时观察内存使用趋势

2. **UV 更新失败后的恢复策略**
   - What we know: InstanceManager 在 UV 更新失败时跳过启动实例,避免启动旧版本
   - What's unclear: 是否需要自动重试机制(当前设计:不重试,等待下一个定时周期)
   - Recommendation: 保持当前设计,下次定时任务会自动重试,避免立即重试导致的雪崩效应

3. **多实例启动顺序依赖**
   - What we know: 当前设计串行启动所有实例,无依赖关系
   - What's unclear: 未来是否需要支持实例间依赖(如 "先启动 gateway 再启动 worker")
   - Recommendation: Phase 10 暂不实现依赖管理,保持串行启动的简单性,未来需求可在 v2.0 考虑

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (标准库) |
| Config file | 无 - 使用 go test 自动发现 |
| Quick run command | `go test -v ./cmd/nanobot-auto-updater -run TestMultiInstance` |
| Full suite command | `go test -v ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| v0.2 定时任务触发 | 定时任务自动执行停止所有→更新→启动所有流程 | integration | `go test -v ./cmd/nanobot-auto-updater -run TestScheduledMultiInstanceUpdate` | ❌ Wave 0 |
| v0.2 -run-once 模式 | 执行一次完整多实例更新流程并退出 | integration | `go test -v ./cmd/nanobot-auto-updater -run TestUpdateNowMultiInstance` | ❌ Wave 0 |
| v0.2 日志追踪 | 每个实例日志包含实例名称和操作状态 | unit | `go test -v ./internal/instance -run TestInstanceLifecycleLogging` | ✅ Phase 7 |
| v0.2 资源管理 | 长期运行无内存泄漏或句柄泄漏 | manual | 运行 `make build && ./nanobot-auto-updater.exe` 48 小时,监控内存 | ❌ Wave 0 |
| v0.2 长期稳定性 | 多次更新周期后系统正常工作 | integration | `go test -v ./cmd/nanobot-auto-updater -run TestMultiInstanceLongRunning -timeout 2h` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -v ./cmd/nanobot-auto-updater` (快速验证集成)
- **Per wave merge:** `go test -v ./...` (完整测试套件)
- **Phase gate:** `make test` + 手动内存泄漏测试通过

### Wave 0 Gaps
- [ ] `cmd/nanobot-auto-updater/main_test.go` — 添加 TestScheduledMultiInstanceUpdate (集成测试,需要 mock scheduler)
- [ ] `cmd/nanobot-auto-updater/main_test.go` — 添加 TestUpdateNowMultiInstance (集成测试,需要 mock InstanceManager)
- [ ] `cmd/nanobot-auto-updater/main_test.go` — 添加 TestMultiInstanceLongRunning (长期运行测试,模拟多次更新周期)
- [ ] `docs/test-plan.md` — 手动测试计划:48 小时运行 + 内存监控

*(如果无测试框架安装需求,添加说明:Go 标准库 testing 已内置,无需额外安装)*

## Sources

### Primary (HIGH confidence)
- internal/instance/manager.go - InstanceManager 协调器实现 (Phase 8 已完成)
- internal/instance/result.go - UpdateResult 结果聚合 (Phase 8 已完成)
- internal/notifier/notifier.go - NotifyUpdateResult 方法 (Phase 9 已完成)
- internal/config/config.go - 配置模式检测逻辑 (Phase 6 已完成)
- cmd/nanobot-auto-updater/main.go - 主程序入口 (v1.0 已实现)

### Secondary (MEDIUM confidence)
- [VictoriaMetrics Blog - Graceful Shutdown in Go: Practical Patterns](https://victoriametrics.com/blog/go-graceful-shutdown/) - 优雅关闭模式验证
- [OneUptime Blog - How to Avoid Common Goroutine Leaks in Go](https://oneuptime.com/blog/post/2026-01-07-go-goroutine-leaks/view) - goroutine 泄漏预防策略

### Tertiary (LOW confidence)
- [Harness Engineering - The Silent Leak: How One Line of Go Drained Memory](https://engineering.harness.io/the-silent-leak-how-one-line-of-go-drained-memory-across-thousands-of-goroutines-f9872f6329d1) - 实际案例,需验证适用性

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - 所有依赖已在 Phase 1-9 中验证,无需新增库
- Architecture: HIGH - 配置模式检测和分支逻辑清晰,InstanceManager 接口设计良好
- Pitfalls: MEDIUM - 长期运行稳定性需要实际测试验证,理论分析可能遗漏边界情况

**Research date:** 2026-03-11
**Valid until:** 30 天 - Go 标准库和 robfig/cron 稳定,集成模式不会快速变化

# Phase 9: 通知扩展 - Research

**Researched:** 2026-03-11
**Domain:** Go通知消息格式化、错误聚合报告、Pushover API使用
**Confidence:** HIGH

## Summary

Phase 9需要在现有通知系统基础上扩展支持多实例失败报告。当前系统使用`gregdel/pushover`库发送单条失败通知,Phase 8已经实现了`UpdateResult`结构聚合所有实例的成功/失败状态。本研究聚焦如何将`UpdateResult`转换为用户友好的通知消息,遵循"一次更新只发送一条通知"原则,避免通知风暴。

**Primary recommendation:** 在`internal/notifier`包中新增`NotifyUpdateResult()`方法,使用`strings.Builder`构建结构化消息,仅在存在失败时发送通知,消息包含失败实例详情(名称、操作、原因)和成功实例列表。

## User Constraints (from CONTEXT.md)

> Phase 9 无CONTEXT.md文件,所有决策由研究员根据需求文档制定。

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| ERROR-01 | 失败通知包含具体哪些实例失败及其失败原因 | `UpdateResult`结构提供完整失败信息,`NotifyUpdateResult()`方法格式化输出 |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| gregdel/pushover | 1.4.0 | Pushover API客户端 | 项目已集成,提供`NewMessageWithTitle()`API |
| strings | 标准库 | 消息构建 | 使用`strings.Builder`高效构建多行消息 |
| fmt | 标准库 | 错误格式化 | 使用`fmt.Sprintf()`格式化错误详情 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| log/slog | 标准库 | 结构化日志 | 通知发送成功/失败时记录INFO/ERROR日志 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| strings.Builder | strings.Join() | strings.Join()适用于简单切片连接,多行结构化消息仍需Builder |
| fmt.Sprintf() | text/template | 模板引擎过度设计,通知消息格式简单,直接格式化更清晰 |

**Installation:**
无需新增依赖,使用现有库。

## Architecture Patterns

### Recommended Notification Extension Pattern

扩展现有`internal/notifier/notifier.go`,而非创建新包:

```go
// NotifyUpdateResult 发送多实例更新结果通知
// 仅在result.HasErrors()为true时发送通知
func (n *Notifier) NotifyUpdateResult(result *instance.UpdateResult) error {
    if !result.HasErrors() {
        n.logger.Debug("All instances succeeded, skipping failure notification")
        return nil
    }

    title := "Nanobot 多实例更新失败"
    message := n.formatUpdateResultMessage(result)
    return n.Notify(title, message)
}

// formatUpdateResultMessage 构建用户友好的错误报告
func (n *Notifier) formatUpdateResultMessage(result *instance.UpdateResult) string {
    var msg strings.Builder

    // 1. 失败摘要
    failedCount := len(result.StopFailed) + len(result.StartFailed)
    msg.WriteString(fmt.Sprintf("更新失败: %d 个实例操作失败\n\n", failedCount))

    // 2. 停止失败详情
    if len(result.StopFailed) > 0 {
        msg.WriteString("停止失败的实例:\n")
        for _, err := range result.StopFailed {
            msg.WriteString(fmt.Sprintf("  ✗ %s (端口 %d)\n", err.InstanceName, err.Port))
            msg.WriteString(fmt.Sprintf("    原因: %v\n", err.Err))
        }
        msg.WriteString("\n")
    }

    // 3. 启动失败详情
    if len(result.StartFailed) > 0 {
        msg.WriteString("启动失败的实例:\n")
        for _, err := range result.StartFailed {
            msg.WriteString(fmt.Sprintf("  ✗ %s (端口 %d)\n", err.InstanceName, err.Port))
            msg.WriteString(fmt.Sprintf("    原因: %v\n", err.Err))
        }
        msg.WriteString("\n")
    }

    // 4. 成功启动的实例列表
    if len(result.Started) > 0 {
        msg.WriteString(fmt.Sprintf("成功启动的实例 (%d):\n", len(result.Started)))
        for _, name := range result.Started {
            msg.WriteString(fmt.Sprintf("  ✓ %s\n", name))
        }
    }

    return msg.String()
}
```

### Anti-Patterns to Avoid

- **反模式1: 为每个失败实例发送单独通知**
  - 为什么不好: 3个实例失败=3条通知,造成通知风暴
  - 应该做: 一条通知包含所有失败实例

- **反模式2: 所有实例都成功时仍发送通知**
  - 为什么不好: 不必要的打扰,用户会忽略通知
  - 应该做: 仅`result.HasErrors()`为true时发送

- **反模式3: 消息只包含失败实例,不显示成功实例**
  - 为什么不好: 用户无法了解整体状态,不知道哪些实例正常
  - 应该做: 包含成功启动列表,让用户了解完整情况

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 多实例消息构建 | 手动拼接字符串 | strings.Builder | Builder避免多次内存分配,性能更优 |
| Pushover消息发送 | 直接HTTP调用 | gregdel/pushover库 | 库处理认证、重试、错误解析等复杂逻辑 |
| 错误判断 | 自定义逻辑 | UpdateResult.HasErrors() | 已有方法,语义清晰 |

**Key insight:** 通知扩展本质是"格式化+条件判断",核心逻辑是消息构建和发送时机控制,无需引入复杂框架。

## Common Pitfalls

### Pitfall 1: 忘记检查HasErrors()导致发送不必要通知

**What goes wrong:** 所有实例都成功时仍发送"更新成功"通知,造成通知疲劳
**Why it happens:** 开发者可能认为"有消息总是好的",但实际用户体验是"只在出问题时通知我"
**How to avoid:** `NotifyUpdateResult()`入口立即检查`!result.HasErrors()`并返回nil
**Warning signs:** 用户反馈"通知太多,都麻木了"

### Pitfall 2: InstanceError.Err包含技术细节,用户看不懂

**What goes wrong:** 消息显示"context deadline exceeded"或"process not found",用户无法理解
**Why it happens:** `InstanceError.Err`是底层错误,直接传递给用户
**How to avoid:** 格式化时使用`%v`而非技术错误码,或考虑映射到用户友好消息
**Warning signs:** 用户问"这个错误是什么意思?"

### Pitfall 3: 消息过长超过Pushover限制

**What goes wrong:** 10个实例失败时,消息超过Pushover的1024字符限制,发送失败
**Why it happens:** 未预估最大消息长度,未做截断处理
**How to avoid:** 计算最大可能长度(例如10实例×每实例100字符=1000字符),超限时截断并添加"...详见日志"
**Warning signs:** Pushover API返回400错误或截断消息

### Pitfall 4: 忘记记录通知发送日志

**What goes wrong:** 用户说"没收到通知",但无法确认是否发送成功
**Why it happens:** `Notify()`方法已记录日志,但开发者可能绕过`Notify()`直接调用`client.SendMessage()`
**How to avoid:** 始终通过`Notify()`/`NotifyUpdateResult()`发送,不直接操作Pushover客户端
**Warning signs:** 排查通知问题时日志中找不到发送记录

## Code Examples

### 示例1: NotifyUpdateResult完整实现

```go
// Source: 基于项目现有notifier.go扩展
package notifier

import (
    "fmt"
    "strings"

    "github.com/HQGroup/nanobot-auto-updater/internal/instance"
)

// NotifyUpdateResult 发送多实例更新结果通知
// 仅在有失败实例时发送通知,避免通知风暴
func (n *Notifier) NotifyUpdateResult(result *instance.UpdateResult) error {
    // 成功时不发送通知
    if !result.HasErrors() {
        n.logger.Debug("All instances succeeded, skipping failure notification",
            "stopped", len(result.Stopped),
            "started", len(result.Started))
        return nil
    }

    title := "Nanobot 多实例更新失败"
    message := n.formatUpdateResultMessage(result)

    return n.Notify(title, message)
}

// formatUpdateResultMessage 构建结构化错误报告
func (n *Notifier) formatUpdateResultMessage(result *instance.UpdateResult) string {
    var msg strings.Builder

    // 失败摘要
    failedCount := len(result.StopFailed) + len(result.StartFailed)
    msg.WriteString(fmt.Sprintf("更新失败: %d 个实例操作失败\n\n", failedCount))

    // 停止失败
    if len(result.StopFailed) > 0 {
        msg.WriteString("停止失败的实例:\n")
        for _, err := range result.StopFailed {
            msg.WriteString(fmt.Sprintf("  ✗ %s (端口 %d)\n", err.InstanceName, err.Port))
            msg.WriteString(fmt.Sprintf("    原因: %v\n", err.Err))
        }
        msg.WriteString("\n")
    }

    // 启动失败
    if len(result.StartFailed) > 0 {
        msg.WriteString("启动失败的实例:\n")
        for _, err := range result.StartFailed {
            msg.WriteString(fmt.Sprintf("  ✗ %s (端口 %d)\n", err.InstanceName, err.Port))
            msg.WriteString(fmt.Sprintf("    原因: %v\n", err.Err))
        }
        msg.WriteString("\n")
    }

    // 成功启动列表
    if len(result.Started) > 0 {
        msg.WriteString(fmt.Sprintf("成功启动的实例 (%d):\n", len(result.Started)))
        for _, name := range result.Started {
            msg.WriteString(fmt.Sprintf("  ✓ %s\n", name))
        }
    }

    return msg.String()
}
```

### 示例2: 主程序集成(Phase 10预览)

```go
// Source: cmd/nanobot-auto-updater/main.go 未来集成代码
import (
    "github.com/HQGroup/nanobot-auto-updater/internal/instance"
    "github.com/HQGroup/nanobot-auto-updater/internal/notifier"
)

// 在定时任务或--update-now模式下:
func executeUpdate(cfg *config.Config, logger *slog.Logger, notif *notifier.Notifier) {
    // 创建InstanceManager
    manager := instance.NewInstanceManager(cfg, logger)

    // 执行更新
    result, err := manager.UpdateAll(context.Background())
    if err != nil {
        logger.Error("Update process failed", "error", err)
        // UV更新失败等严重错误,发送通用失败通知
        notif.NotifyFailure("Multi-instance Update", err)
        return
    }

    // 发送多实例结果通知(仅在失败时)
    if err := notif.NotifyUpdateResult(result); err != nil {
        logger.Error("Failed to send update result notification", "error", err)
    }

    logger.Info("Update completed",
        "stopped", len(result.Stopped),
        "started", len(result.Started),
        "stop_failed", len(result.StopFailed),
        "start_failed", len(result.StartFailed))
}
```

### 示例3: 单元测试示例

```go
// Source: 基于项目现有notifier_test.go模式
package notifier

import (
    "errors"
    "testing"

    "github.com/HQGroup/nanobot-auto-updater/internal/instance"
)

func TestNotifyUpdateResult_NoErrors(t *testing.T) {
    // 创建disabled notifier
    logger, _ := newTestLogger()
    n := New(logger)

    // 所有实例都成功
    result := &instance.UpdateResult{
        Stopped: []string{"inst1", "inst2"},
        Started: []string{"inst1", "inst2"},
    }

    // 应返回nil且不发送通知
    err := n.NotifyUpdateResult(result)
    if err != nil {
        t.Errorf("Expected nil when no errors, got: %v", err)
    }
}

func TestFormatUpdateResultMessage_WithFailures(t *testing.T) {
    logger, _ := newTestLogger()
    n := New(logger)

    result := &instance.UpdateResult{
        StopFailed: []*instance.InstanceError{
            {
                InstanceName: "failed-stop",
                Operation:    "stop",
                Port:         8080,
                Err:          errors.New("timeout"),
            },
        },
        StartFailed: []*instance.InstanceError{
            {
                InstanceName: "failed-start",
                Operation:    "start",
                Port:         8081,
                Err:          errors.New("port in use"),
            },
        },
        Started: []string{"success1"},
    }

    msg := n.formatUpdateResultMessage(result)

    // 验证消息包含关键信息
    if !strings.Contains(msg, "2 个实例操作失败") {
        t.Error("Message should mention 2 failed instances")
    }
    if !strings.Contains(msg, "failed-stop") {
        t.Error("Message should include failed instance name")
    }
    if !strings.Contains(msg, "8080") {
        t.Error("Message should include port number")
    }
    if !strings.Contains(msg, "success1") {
        t.Error("Message should include successful instance")
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| 单实例失败通知 | 多实例聚合通知 | Phase 8 (2026-03-11) | 避免通知风暴,一次更新一条通知 |
| 只报告失败 | 成功+失败都报告 | Phase 9 (本阶段) | 用户了解完整状态,不仅知道"哪里错了",还知道"哪些正常" |

**Deprecated/outdated:**
- 无,通知系统从Phase 3建立至今未变更,Phase 9是首次扩展

## Open Questions

### 1. 是否需要处理消息超长截断?

**What we know:**
- Pushover消息限制1024字符(参考官方文档)
- 单实例错误约100字符(名称+端口+原因)
- 10实例失败约1000字符,接近限制

**What's unclear:**
- 项目实际会配置多少实例? (当前需求未明确上限)
- 用户是否需要完整错误列表,还是"前N个+详见日志"?

**Recommendation:**
- Phase 9暂不实现截断,Phase 10集成时根据实际测试决定
- 如需截断,保留前5个失败实例,添加"\n...更多失败详见日志"
- 理由: 过早优化是万恶之源,先验证实际场景

### 2. InstanceError.Err的错误消息是否需要映射?

**What we know:**
- `InstanceError.Err`包含底层错误(如"context.DeadlineExceeded")
- 当前直接使用`%v`格式化,技术性较强

**What's unclear:**
- 用户是否能理解"context deadline exceeded"?
- 是否需要建立错误码→友好消息映射表?

**Recommendation:**
- Phase 9保持现状,使用`%v`输出原始错误
- 如用户反馈难以理解,在Phase 10增加错误映射
- 理由: InstanceError已在Phase 7使用中文操作名("停止实例"/"启动实例"),已部分优化可读性

## Validation Architecture

> workflow.nyquist_validation未在config.json中显式禁用,因此包含此部分

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (标准库) |
| Config file | none — 使用`t.Setenv()`管理环境变量 |
| Quick run command | `go test ./internal/notifier -v -run TestNotifyUpdateResult` |
| Full suite command | `go test ./internal/notifier -v` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| ERROR-01 | 失败通知包含实例名称、操作、原因 | unit | `go test ./internal/notifier -v -run TestFormatUpdateResultMessage` | ❌ Wave 0 |
| ERROR-01 | 所有实例成功时不发送通知 | unit | `go test ./internal/notifier -v -run TestNotifyUpdateResult_NoErrors` | ❌ Wave 0 |
| ERROR-01 | 多实例失败只发送一条通知 | unit | `go test ./internal/notifier -v -run TestNotifyUpdateResult_SingleNotification` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/notifier -v -run TestNotifyUpdateResult`
- **Per wave merge:** `go test ./internal/notifier -v` (完整包测试)
- **Phase gate:** `go test ./internal/notifier -v -cover` (覆盖率>80%)

### Wave 0 Gaps
- [ ] `internal/notifier/notifier_ext_test.go` — 测试NotifyUpdateResult和formatUpdateResultMessage
- [ ] 无需新增框架配置,复用现有测试基础设施

## Sources

### Primary (HIGH confidence)
- [gregdel/pushover GitHub](https://github.com/gregdel/pushover) - Go Pushover库源码和使用示例
- [Pushover Multi-Line Support](https://support.pushover.net/i35-sending-multi-line-notifications-through-the-api) - 官方确认使用`\n`换行
- 项目现有代码:
  - `internal/notifier/notifier.go` (已实现基础通知)
  - `internal/instance/result.go` (已实现UpdateResult)
  - `internal/instance/errors.go` (已实现InstanceError)

### Secondary (MEDIUM confidence)
- [Go strings.Builder Best Practices](https://go-cookbook.com/snippets/strings/string-building) - Builder使用场景和性能优势
- [Go Error Handling Best Practices](https://oneuptime.com/blog/post/2026-02-20-go-error-handling-patterns/view) - 错误格式化和上下文提供
- [Pushover Message Formatting](https://community.n8n.io/t/pushover-notification-formatting/18704) - 社区实践

### Tertiary (LOW confidence)
- [Notification Message Best Practices (Datadog)](https://docs.datadoghq.com/monitors/guide/notification-message-best-practices/) - 监控通知最佳实践,部分适用

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - 项目已使用gregdel/pushover和strings.Builder,无需新增依赖
- Architecture: HIGH - 扩展现有notifier包符合项目结构,UpdateResult已在Phase 8就绪
- Pitfalls: MEDIUM - 基于WebSearch和经验推断,实际用户反馈可能揭示新问题

**Research date:** 2026-03-11
**Valid until:** 2026-04-11 (Go标准库和Pushover API稳定,30天内有效)

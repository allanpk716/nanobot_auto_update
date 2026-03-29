# Phase 34: Update Notification Integration - Research

**Researched:** 2026-03-29
**Domain:** Go HTTP handler integration with Pushover notification system
**Confidence:** HIGH

## Summary

Phase 34 将现有 Notifier 组件注入 TriggerHandler，在 HTTP API 触发的更新流程中发送"更新开始"和"更新完成"两阶段 Pushover 通知。这是一个纯集成任务——不涉及新组件创建，所有基础设施已存在：Notifier（Phase 9）、TriggerHandler（Phase 28）、UpdateLogger 注入模式（Phase 30）、异步通知发送模式（Phase 27）。

改动范围极小且模式成熟：TriggerHandler 增加 `notifier *notifier.Notifier` 字段，在 Handle() 方法的两个关键点插入异步通知调用。主要改动集中在 `internal/api/trigger.go`（通知逻辑）、`internal/api/server.go`（参数传递）、`cmd/nanobot-auto-updater/main.go`（依赖注入）三个文件。

**Primary recommendation:** 完全复用 Phase 30 的 UpdateLogger 注入模式和 Phase 27 的异步通知发送模式，不创新、不抽象，最小改动完成集成。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01: 开始通知格式**
  - 标题: "Nanobot 更新开始"
  - 消息: "触发来源: api-trigger\n待更新实例数: {N}"
  - 中文内容，与现有 Pushover 通知风格一致（Phase 27 用中文）

- **D-02: 完成通知格式**
  - 标题根据状态动态生成:
    - success: "Nanobot 更新成功"
    - partial_success: "Nanobot 更新部分成功"
    - failed: "Nanobot 更新失败"
  - 消息包含汇总信息:
    - 耗时（秒）: "耗时: {X.X}s"
    - 总实例数、成功数、失败数
    - 失败实例名称列表（如有）
  - 不包含每个实例的详细错误信息（通知应简洁，详细查看日志）

- **D-03: 与 UpdateLogger 相同的注入模式**
  - TriggerHandler 增加 `notifier *notifier.Notifier` 字段
  - NewTriggerHandler 增加 `notif *notifier.Notifier` 参数
  - NewServer 增加 `notif *notifier.Notifier` 参数
  - main.go 将已创建的 Notifier 传入 NewServer
  - 与 Phase 30 注入 UpdateLogger 的模式完全一致

- **D-04: 开始通知在 UUID 生成之后、TriggerUpdate 之前**
  - 流程: 生成 UUID -> 记录开始时间 -> 发送开始通知 -> TriggerUpdate -> ...
  - 开始通知中包含 update_id 可选（但不需要，通知内容保持简洁）

- **D-05: 完成通知在 UpdateLog 记录之后、HTTP 响应之前**
  - 流程: ... -> TriggerUpdate -> 记录 UpdateLog -> 发送完成通知 -> 返回 HTTP 响应
  - 完成通知在 goroutine 中发送，不阻塞 HTTP 响应

- **D-06: 复用现有 IsEnabled() 机制**
  - Notifier.IsEnabled() 返回 false 时，Notify() 已自动跳过并记录 DEBUG
  - 无需额外判断，直接调用 Notify() 即可
  - 与 Phase 27 行为一致

- **D-07: 与 Phase 27 一致 -- 仅记录 ERROR 日志**
  - 通知发送失败记录 ERROR 日志，不影响更新流程
  - 不重试
  - HTTP 响应和 UpdateLog 记录不受影响

### Claude's Discretion
- 通知内容的具体措辞和格式细节
- goroutine 中错误处理的具体实现
- 开始通知和完成通知是否使用同一个 goroutine
- 日志字段的具体命名

### Deferred Ideas (OUT OF SCOPE)
None
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| UNOTIF-01 | 更新开始时发送 Pushover 通知，包含触发来源和实例数量 | D-01 格式已锁定；在 Handle() UUID 生成后、TriggerUpdate 前插入异步通知；使用 `h.instanceManager` 获取实例数量（需确认 TriggerUpdater 接口是否暴露实例数） |
| UNOTIF-02 | 更新完成后发送 Pushover 通知，包含三态状态、耗时和实例结果 | D-02 格式已锁定；复用 updatelog.DetermineStatus() 获取三态；从 UpdateResult 提取汇总信息；在 UpdateLog 记录后发送 |
| UNOTIF-03 | 通知发送为异步非阻塞，失败不影响更新流程和 API 响应 | Phase 27 已有成熟模式：`go func() { ... }()` + panic recovery + ERROR 日志；完成通知在 goroutine 中发送不阻塞响应 |
| UNOTIF-04 | Pushover 未配置时跳过通知，记录 DEBUG 日志 | D-06 已锁定：Notifier.Notify() 内部已有 IsEnabled() 检查，未配置时自动跳过并记录 DEBUG；无需额外逻辑 |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| gregdel/pushover | v1.4.0 | Pushover API 客户端 | 项目已依赖，Notifier 内部使用 |
| google/uuid | v1.6.0 | UUID v4 生成 | 项目已依赖，TriggerHandler 使用 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| stretchr/testify | v1.11.1 | 测试断言和 mock | 编写通知集成测试时可选使用 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| 具体 `*notifier.Notifier` 类型 | `notification.Notifier` 接口 | CONTEXT.md D-03 锁定使用具体类型（与 UpdateLogger 一致），接口在 notification 包中仅为解耦 NetworkMonitor |

**Installation:**
无需新增依赖。所有依赖已在 go.mod 中。

## Architecture Patterns

### Recommended Project Structure
```
internal/api/
  trigger.go         # 增加 notifier 字段和两处通知调用
  server.go          # NewServer 增加 notifier 参数
  trigger_test.go    # 增加通知相关测试
cmd/nanobot-auto-updater/
  main.go            # api.NewServer() 调用增加 notif 参数
```

### Pattern 1: 依赖注入（与 UpdateLogger 一致）
**What:** 通过构造函数参数注入 Notifier 到 TriggerHandler
**When to use:** Phase 30 已建立的模式，Notifier 遵循相同模式
**Example:**
```go
// trigger.go -- 构造函数增加参数
func NewTriggerHandler(im TriggerUpdater, cfg *config.APIConfig, logger *slog.Logger, ul *updatelog.UpdateLogger, notif *notifier.Notifier) *TriggerHandler {
    return &TriggerHandler{
        instanceManager: im,
        config:          cfg,
        logger:          logger.With("source", "api-trigger"),
        updateLogger:    ul,
        notifier:        notif,
    }
}

// server.go -- 透传参数
func NewServer(cfg *config.APIConfig, im *instance.InstanceManager, fullCfg *config.Config, version string, logger *slog.Logger, updateLogger *updatelog.UpdateLogger, notif *notifier.Notifier) (*Server, error) {
    // ...
    triggerHandler := NewTriggerHandler(im, cfg, logger, updateLogger, notif)
    // ...
}

// main.go -- 传入已创建的 notif
apiServer, err = api.NewServer(&cfg.API, instanceManager, cfg, Version, logger, updateLogger, notif)
```

### Pattern 2: 异步通知发送（与 Phase 27 一致）
**What:** 在 goroutine 中发送通知，包含 panic recovery
**When to use:** 通知发送可能因网络问题阻塞或 panic
**Example:**
```go
// Phase 27 已有模式（notification/manager.go sendNotification 方法）
go func() {
    defer func() {
        if r := recover(); r != nil {
            h.logger.Error("通知发送 goroutine panic",
                "panic", r,
                "stack", string(debug.Stack()))
        }
    }()

    if err := h.notifier.Notify(title, message); err != nil {
        h.logger.Error("发送更新通知失败",
            "error", err,
            "title", title)
    }
}()
```

### Pattern 3: Nil-safe 组件检查（与 UpdateLogger 一致）
**What:** 检查注入组件是否为 nil 再调用
**When to use:** TriggerHandler 中 notifier 字段可能为 nil（测试场景或未来扩展）
**Example:**
```go
if h.notifier != nil {
    // 发送通知
    go func() { ... }()
}
```
注意：根据 D-06，Notifier.Notify() 内部已有 IsEnabled() 检查，但 TriggerHandler 层仍需 nil 检查（与 updateLogger 的 nil 检查模式一致）。

### Anti-Patterns to Avoid
- **同步发送通知**: 绝不在 Handle() 的主 goroutine 中直接调用 Notify()，即使看起来很快。Pushover API 可能超时，会阻塞 HTTP 响应。
- **在开始通知中阻塞**: 开始通知也必须在 goroutine 中发送，不能同步等待。虽然 CONTEXT.md D-04 暗示开始通知在 TriggerUpdate 之前，但不应阻塞后续流程。
- **创建新的通知抽象层**: 不需要定义新的接口或包装器，直接使用 `*notifier.Notifier` 具体类型。
- **在通知中包含详细错误信息**: D-02 明确排除，通知应简洁。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Pushover 未配置检查 | 额外的 enabled 判断逻辑 | Notifier.Notify() 内置 IsEnabled() 检查 | D-06 已锁定，Notify() 未配置时自动跳过 |
| 三态状态分类 | 自定义 success/partial_success/failed 判断 | updatelog.DetermineStatus(result) | Phase 30 已有成熟逻辑，复用确保一致性 |
| Pushover API 调用 | HTTP 请求构建 | Notifier.Notify(title, message) | 已封装 pushover 库调用和错误处理 |

**Key insight:** 整个 Phase 的核心工作是在 Handle() 方法的正确位置插入两处已有的 Notify() 调用，不创建新功能。

## Common Pitfalls

### Pitfall 1: 开始通知阻塞更新流程
**What goes wrong:** 在 Handle() 主流程中同步调用 Notify()，如果 Pushover API 响应慢，会延迟 TriggerUpdate 的执行
**Why it happens:** UNOTIF-01 要求"在执行 TriggerUpdate 之前发送通知"，但"之前"指逻辑顺序，不是同步等待
**How to avoid:** 开始通知也在 goroutine 中异步发送，不等待结果。Handle() 立即继续执行 TriggerUpdate。
**Warning signs:** 测试中 Handle() 响应时间明显增加

### Pitfall 2: 忘记 nil 检查导致 panic
**What goes wrong:** TriggerHandler.notifier 为 nil 时调用 Notify() 导致 nil pointer panic
**Why it happens:** 测试中可能使用 nil notifier 创建 handler（现有 `newTestHandler` 不传 notifier）
**How to avoid:** 通知调用前检查 `h.notifier != nil`，与 updateLogger 的 nil 检查模式一致
**Warning signs:** 现有测试 panic

### Pitfall 3: 获取实例数量的方式不正确
**What goes wrong:** UNOTIF-01 要求开始通知包含"待更新实例数"，但 TriggerUpdater 接口不暴露实例数量
**Why it happens:** TriggerHandler 依赖 TriggerUpdater 接口（仅 TriggerUpdate 方法），不持有 `*instance.InstanceManager`
**How to avoid:** 需要从 TriggerUpdater 获取实例数量。选项：(A) 扩展 TriggerUpdater 接口添加 InstanceCount() 方法；(B) 单独注入实例数量；(C) 使用配置中的实例数量。推荐选项 A，最干净。或者，开始通知不包含实例数量，仅记录触发了更新。需要 Planner 决策。
**Warning signs:** 编译错误或运行时 panic

### Pitfall 4: 完成通知在错误路径上未发送
**What goes wrong:** TriggerUpdate 返回错误时（如 UV update failed），直接 return 错误响应，跳过完成通知
**Why it happens:** Handle() 方法在 err != nil 时有多个 early return 路径（ErrUpdateInProgress、DeadlineExceeded、其他错误）
**How to avoid:** 仅在正常完成路径（result 不为 nil）发送完成通知。错误路径（ErrUpdateInProgress、超时等）不发送完成通知，因为这些情况下更新并未真正执行。
**Warning signs:** 用户在更新失败时未收到通知

### Pitfall 5: 测试中通知 goroutine 时序问题
**What goes wrong:** 测试验证通知是否发送时，goroutine 可能还未执行，导致测试 flaky
**Why it happens:** 异步通知在独立 goroutine 中，测试无法确定何时完成
**How to avoid:** 测试中可使用同步方式验证（如在 mock Notifier 中记录调用），或使用 short sleep 等待 goroutine。推荐方式：mock Notifier 记录调用参数，测试验证参数正确而非执行时序。
**Warning signs:** 测试间歇性失败

## Code Examples

### 开始通知发送位置（Handle() 方法中）
```go
// trigger.go Handle() 方法
// 在现有 Step 2 (UUID + startTime) 之后、Step 3 (context) 之前插入

// 2. Generate UUID v4 and record start time (LOG-02)
updateID := uuid.New().String()
startTime := time.Now().UTC()
h.logger.Info("Update triggered", "update_id", updateID)

// --- 新增: 发送开始通知 (UNOTIF-01) ---
if h.notifier != nil {
    instanceCount := h.getInstanceCount() // 需要解决如何获取实例数量
    go func() {
        defer func() {
            if r := recover(); r != nil {
                h.logger.Error("开始通知 goroutine panic",
                    "panic", r,
                    "stack", string(debug.Stack()))
            }
        }()
        title := "Nanobot 更新开始"
        message := fmt.Sprintf("触发来源: api-trigger\n待更新实例数: %d", instanceCount)
        if err := h.notifier.Notify(title, message); err != nil {
            h.logger.Error("发送更新开始通知失败", "error", err)
        }
    }()
}
// --- 新增结束 ---

// 3. Create context with timeout from config
ctx, cancel := context.WithTimeout(r.Context(), h.config.Timeout)
defer cancel()
```

### 完成通知发送位置（Handle() 方法中）
```go
// 在现有 Step 6 (UpdateLog 记录) 之后、Step 7 (返回响应) 之前插入

// 6. Build and record UpdateLog (LOG-01, LOG-03, LOG-04)
if h.updateLogger != nil {
    // ... existing code ...
}

// --- 新增: 发送完成通知 (UNOTIF-02) ---
if h.notifier != nil {
    status := updatelog.DetermineStatus(result)
    elapsed := endTime.Sub(startTime).Seconds()
    go func() {
        defer func() {
            if r := recover(); r != nil {
                h.logger.Error("完成通知 goroutine panic",
                    "panic", r,
                    "stack", string(debug.Stack()))
            }
        }()

        title := fmt.Sprintf("Nanobot 更新%s", statusToTitle(status))
        message := formatCompletionMessage(result, status, elapsed)
        if err := h.notifier.Notify(title, message); err != nil {
            h.logger.Error("发送更新完成通知失败",
                "error", err,
                "update_id", updateID)
        }
    }()
}
// --- 新增结束 ---

// 7. Return JSON result with update_id
```

### 测试中的 Mock Notifier
```go
// trigger_test.go 中添加 mock notifier
type mockNotifier struct {
    enabled       bool
    notifyCalled  bool
    lastTitle     string
    lastMessage   string
    mu            sync.Mutex
}

func (m *mockNotifier) IsEnabled() bool { return m.enabled }

func (m *mockNotifier) Notify(title, message string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.notifyCalled = true
    m.lastTitle = title
    m.lastMessage = message
    return nil
}

func (m *mockNotifier) getCalled() bool {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.notifyCalled
}

func (m *mockNotifier) getLastNotification() (string, string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.lastTitle, m.lastMessage
}
```

注意：由于 CONTEXT.md D-03 锁定使用具体类型 `*notifier.Notifier`（而非接口），测试中无法直接 mock。两个解决方案：
1. **推荐**: 在 TriggerHandler 中对 notifier 做 nil 检查，测试用 nil 传入验证"不 panic"；集成测试验证真实通知发送
2. **替代**: 修改 NewTriggerHandler 接受一个 `NotificationSender` 接口（包含 Notify 和 IsEnabled），但这与 D-03 矛盾

**最终推荐**: 遵循 D-03 使用具体类型。测试策略：
- 单元测试：传入 nil notifier，验证不 panic + 不影响响应
- 单元测试：使用 disabled notifier（`notifier.NewWithConfig(notifier.Config{}, logger)`），验证 Notify() 被安全跳过
- 功能验证：手动或集成测试确认通知内容正确

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| NotificationManager 使用接口 | TriggerHandler 使用具体类型 | Phase 27 vs Phase 30 | 不同组件有不同模式，Phase 34 遵循 Phase 30 |
| 同步通知发送 | 异步 goroutine + panic recovery | Phase 27 | 必须遵循异步模式 |

**Deprecated/outdated:**
- 无

## Open Questions

1. **如何获取"待更新实例数"用于开始通知**
   - What we know: TriggerHandler 依赖 TriggerUpdater 接口（仅 `TriggerUpdate(ctx) (*UpdateResult, error)`），不暴露实例数量。但 TriggerHandler 持有 `*config.APIConfig`，不持有完整 Config。
   - What's unclear: 是否需要扩展 TriggerUpdater 接口，或通过其他方式获取实例数量
   - Recommendation: 三种可行方案：
     - A: TriggerHandler 增加 `instanceCount int` 字段，NewTriggerHandler 传入
     - B: TriggerUpdater 接口增加 `InstanceCount() int` 方法
     - C: 开始通知不包含实例数量（最简单，但与 UNOTIF-01 "实例数量" 不完全匹配）
     - **推荐方案 A**: 最小改动，NewServer 中从 InstanceManager 获取数量传入

## Environment Availability

Step 2.6: SKIPPED (no external dependencies identified -- Phase 34 是纯代码集成，所有组件已在项目中)

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + stretchr/testify v1.11.1 |
| Config file | none -- Go 标准 testing |
| Quick run command | `go test ./internal/api/ -count=1 -run TestTriggerHandler -v` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| UNOTIF-01 | 开始通知在 TriggerUpdate 前异步发送 | unit | `go test ./internal/api/ -count=1 -run TestTriggerHandler_StartNotification -v` | Wave 0 (需新增) |
| UNOTIF-02 | 完成通知包含三态状态、耗时和实例结果 | unit | `go test ./internal/api/ -count=1 -run TestTriggerHandler_CompletionNotification -v` | Wave 0 (需新增) |
| UNOTIF-03 | 通知失败不影响 HTTP 响应和 UpdateLog | unit | `go test ./internal/api/ -count=1 -run TestTriggerHandler_NotificationFailureNonBlocking -v` | Wave 0 (需新增) |
| UNOTIF-04 | Pushover 未配置时跳过通知无错误 | unit | `go test ./internal/api/ -count=1 -run TestTriggerHandler_DisabledNotification -v` | Wave 0 (需新增) |

### Sampling Rate
- **Per task commit:** `go test ./internal/api/ -count=1 -run TestTriggerHandler -v`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/api/trigger_test.go` -- 需增加 UNOTIF-01/02/03/04 测试用例和 mockNotifier
- [ ] 现有 `newTestHandler()` 需更新签名增加 notifier 参数（或传入 nil）
- [ ] 注意：由于使用具体类型 `*notifier.Notifier`，无法直接 mock。需要使用 nil 或 disabled notifier 测试

## Sources

### Primary (HIGH confidence)
- 源码 `internal/api/trigger.go` -- Handle() 方法完整流程和注入点
- 源码 `internal/notifier/notifier.go` -- Notifier 完整实现，Notify() 和 IsEnabled() 方法
- 源码 `internal/api/server.go` -- NewServer 构造函数和参数传递链
- 源码 `cmd/nanobot-auto-updater/main.go` -- 依赖注入入口
- 源码 `internal/updatelog/types.go` -- DetermineStatus() 三态逻辑和 UpdateLog 结构
- 源码 `internal/notification/manager.go` -- Phase 27 异步通知发送模式（goroutine + panic recovery）

### Secondary (MEDIUM confidence)
- 源码 `internal/api/trigger_test.go` -- 现有测试模式和 newTestHandler 工厂函数
- 源码 `internal/instance/manager.go` -- TriggerUpdater 接口定义和 UpdateResult 结构
- 源码 `internal/instance/result.go` -- UpdateResult 结构体和 HasErrors() 方法

### Tertiary (LOW confidence)
- 无

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- 所有依赖已在 go.mod 中，无需新增
- Architecture: HIGH -- D-03 锁定与 UpdateLogger 相同注入模式，代码已验证
- Pitfalls: HIGH -- 基于 5 个已读取的源文件分析得出
- Test patterns: MEDIUM -- 具体 Notifier 类型导致 mock 限制，需要 Wave 0 验证测试策略

**Research date:** 2026-03-29
**Valid until:** 2026-04-29 (stable -- 纯代码集成，无外部依赖变化风险)

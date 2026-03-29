# Phase 35: Notification Integration Testing - Context

**Gathered:** 2026-03-29
**Status:** Ready for planning

<domain>
## Phase Boundary

E2E 验证 Phase 34 实现的完整通知生命周期：开始通知、完成通知、非阻塞行为、优雅降级。无新功能需求，纯测试阶段，验证 UNOTIF-01 至 UNOTIF-04 全部成功标准。

**核心验证点：**
1. 开始通知在 TriggerUpdate 执行前发送，包含正确触发来源和实例数
2. 完成通知在更新完成后发送，包含正确状态、耗时、实例详情
3. Pushover 失败不影响 API 响应状态码、响应体、UpdateLog 记录
4. 未配置 Notifier 时零通知尝试，更新流程无错误

**不包含：**
- 新功能实现（Phase 34 已完成）
- 单元测试补充（已有 3 个通知单元测试在 trigger_test.go）
- 性能基准测试（通知为异步 goroutine，不阻塞主流程）

</domain>

<decisions>
## Implementation Decisions

### 通知验证策略
- **D-01: 重构 Notifier 为 interface**
  - 在 trigger.go 中定义 `Notifier` interface：`Notify(title, message string) error`
  - `TriggerHandler.notifier` 字段类型从 `*notifier.Notifier` 改为 interface
  - `notifier.Notifier` 具体结构体自然满足该 interface（duck typing）
  - 测试中注入 recording mock，记录所有 `Notify()` 调用的 title 和 message
  - 需同步修改：trigger.go（字段类型 + import）、server.go（参数类型）、main.go（无变化，具体类型自动满足 interface）

- **D-02: Recording mock 结构**
  - 创建 `recordingNotifier` mock：记录 calls slice（每次调用的 title + message）
  - 提供 `Calls() []NotifyCall` 和 `CallCount() int` 方法供断言
  - 支持 configurable 行为：正常返回 nil 或返回 error（用于失败模拟）

### E2E 测试范围
- **D-03: 4 个 E2E 测试，每个对应一个成功标准**
  - Test 1: 开始通知验证 — mock 记录到 Notify 被调用，验证 title="Nanobot 更新开始"、message 包含 "api-trigger" 和实例数
  - Test 2: 完成通知验证 — mock 记录到 2 次 Notify 调用（开始+完成），验证完成通知的 title 根据状态正确（成功/部分成功/失败）、message 包含耗时和实例详情
  - Test 3: 非阻塞验证 — mock.Notify() 返回 error，验证 HTTP 响应仍为 200、JSON body 正确、UpdateLog 正常记录
  - Test 4: 降级验证 — nil notifier，验证零 Notify 调用、HTTP 响应正常、无错误

- **D-04: 遵循 Phase 33 集成测试模式**
  - 测试文件：`internal/api/integration_test.go`（追加到现有文件）
  - 使用 `t.TempDir()` 创建临时 JSONL 文件
  - 复用 Phase 33 的 `mockTriggerUpdater` 和共享组件模式
  - 使用 `httptest.NewRequest` + `httptest.NewRecorder`

### Pushover 失败模拟
- **D-05: Mock 返回 error 模拟失败**
  - recordingNotifier 支持配置 `shouldError bool`
  - 当 shouldError=true 时，Notify() 返回 fmt.Errorf("simulated pushover failure")
  - 测试验证：HTTP 200 + 正确 JSON body + UpdateLog 文件中有记录
  - 不需要 fake HTTP server，interface mock 足够

### Claude's Discretion
- Interface 定义的具体位置（trigger.go 文件内 or 独立文件）
- recordingNotifier 的具体实现细节
- 每个 E2E 测试的内部结构（table-driven vs 独立函数）
- 辅助测试函数的命名和位置
- 断言通知内容的具体方式（精确匹配 or 包含关键词）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 需求参考
- `.planning/REQUIREMENTS.md` § UNOTIF-01, UNOTIF-02, UNOTIF-03, UNOTIF-04 — 更新通知需求定义

### Phase 34 实现参考
- `.planning/phases/34-update-notification-integration/34-CONTEXT.md` — 通知注入决策和实现细节
- `.planning/phases/34-update-notification-integration/34-01-SUMMARY.md` — Phase 34 实现总结
- `.planning/phases/34-update-notification-integration/34-VALIDATION.md` — Phase 34 验证状态

### 测试模式参考
- `.planning/phases/33-integration-and-testing/33-CONTEXT.md` — Phase 33 集成测试模式（4 个 E2E 测试结构）
- `internal/api/integration_test.go` — 现有 E2E 测试（392 行），Phase 35 测试追加到该文件
- `internal/api/trigger_test.go` — 现有单元测试（19 个），含 3 个通知相关测试

### 关键代码文件
- `internal/api/trigger.go` — TriggerHandler Handle() 方法，开始通知（71-88行）、完成通知（137-156行）、辅助函数 statusToTitle/formatCompletionMessage
- `internal/api/server.go` — NewServer() 参数传递链（需改 notifier 参数类型）
- `internal/notifier/notifier.go` — Notifier 具体实现，Notify()/IsEnabled()
- `cmd/nanobot-auto-updater/main.go` — Notifier 创建和注入

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **internal/api/integration_test.go**: 4 个现有 E2E 测试，可直接追加新测试
  - `mockTriggerUpdater` 结构体可复用
  - `newTestServer` helper 模式可复用（需增加 notifier 参数）
- **internal/api/trigger_test.go**: 3 个通知相关单元测试作为参考
  - `TestTriggerHandler_NotifierNil_NilSafe` — nil notifier 模式
  - `TestTriggerHandler_DisabledNotifier_NilSafe` — disabled notifier 模式
  - `TestTriggerHandler_NilNotifier_ErrorPaths` — error 路径覆盖
- **internal/notifier/notifier.go**: Notifier 具体结构体，Notify(title, message) 方法签名即为 interface 的目标签名

### Established Patterns
- **E2E 集成测试**: Phase 33 模式 — httptest + TempDir + shared components + JSONL file verification
- **Mock 接口**: `mockTriggerUpdater` 实现 `TriggerUpdater` interface — 相同模式应用于 Notifier interface
- **Async 测试**: 通知在 goroutine 中发送，测试需要同步等待机制（channel 或 time-based wait）
- **Nil-safe 组件**: handler 检查 nil，非阻塞错误日志

### Integration Points
- **trigger.go Notifier interface**: 新 interface 定义 + TriggerHandler.notifier 字段类型变更
- **server.go NewServer**: 参数类型从 `*notifier.Notifier` 改为 `Notifier` interface
- **integration_test.go**: 追加 4 个 E2E 测试，使用 recordingNotifier mock
- **现有测试适配**: trigger_test.go 和 server_test.go 中传入的 `*notifier.Notifier` 需适配新 interface（duck typing，无需改动传参）

</code_context>

<specifics>
## Specific Ideas

- **Interface 最小化**: 只暴露 `Notify(title, message string) error` 一个方法，IsEnabled() 不需要放入 interface（测试通过 nil 控制降级场景）
- **Recording mock 模式**: 与 mockTriggerUpdater 一致 — 简单结构体 + 字段控制行为，无框架依赖
- **Phase 33 模式延续**: 用户熟悉 4 测试 E2E 结构，保持一致性降低认知负担
- **Async 测试同步**: 可用小 sleep 或 channel 机制等待 goroutine 中的通知完成，需注意测试稳定性

</specifics>

<deferred>
## Deferred Ideas

None — 讨论保持在阶段范围内

</deferred>

---

*Phase: 35-notification-integration-testing*
*Context gathered: 2026-03-29*

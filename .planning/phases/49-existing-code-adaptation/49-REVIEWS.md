---
phase: 49
reviewers: [opencode]
reviewed_at: 2026-04-11T12:00:00Z
plans_reviewed: [49-01-PLAN.md, 49-02-PLAN.md]
---

# Cross-AI Plan Review — Phase 49

## OpenCode Review

### Plan 01: daemon.go 服务模式跳过 + defaultRestartFn SCM 重启策略 + 验证 ADPT-03

**Summary**

Plan 01 直截了当且范围明确 — 两处针对性修改，影响两个文件。守护进程跳过和 SCM 恢复重启策略是简单、防御性的更改，完全符合用户决定。ADPT-03 已经验证为现有代码。低风险，高置信度。

**Strengths**

- 完全符合用户决定 D-01/D-02/D-03/D-08 — 无偏差
- `defaultRestartFn` 中的 `os.Exit(1)` 方法很优雅：利用现有的 SCM 恢复策略（3 次重启，60 秒间隔），无需在自更新代码中添加 SCM API 调用
- 守护进程跳过在两个函数的顶部都使用了 `if IsServiceMode() { return false, nil }` — 这是最小侵入性的
- 明确声明在 `IsServiceMode` 错误时忽略（继续执行守护进程逻辑），这避免了在边缘情况下服务检测失败时破坏控制台模式
- ADPT-03 已在 `main.go:74-83` 中实现 — 确认无需额外工作

**Concerns**

- **[LOW] service mode 分支不尝试 spawn**：service mode 下 `os.Exit(1)` 在 spawn 之前触发，不存在 spawn 失败的 edge case。但应在 plan 中明确说明。
- **[LOW] IsServiceMode 检查位置**：应放在 env var check 之前，避免不必要的 syscall 开销。Plan 应指定精确插入点。
- **[LOW] SCM 恢复在 3 次失败后静默停止**：如果自更新生成损坏的二进制文件，SCM 尝试 3 次后放弃。24h 后计数器重置。自更新 goroutine 已在失败时发送通知，但值得关注。

**Suggestions**

- 明确 service mode 分支只执行 `os.Exit(1)`，不尝试 spawn
- 将 `IsServiceMode()` 检查作为 `MakeDaemon()` 和 `MakeDaemonSimple()` 中的第一行
- 在 restart 路径中添加日志行，便于运维排查

**Risk Assessment: LOW** — 两处针对性代码修改，代码库中已有确切模式。SCM 恢复策略已配置和测试。回归风险仅限于控制台模式，但明确为"不变"。

---

### Plan 02: config 热重载: viper.WatchConfig + 组件重建回调 + onReady 集成

**Summary**

Plan 02 雄心勃勃且架构良好，但包含重要的并发复杂性。将 viper 实例暴露为包级单例，并使用基于 `reflect.DeepEqual` 的部分 config diff 来重建正在运行的组件，是合理的实现方式。然而，该 plan 在组件生命周期排序（停止旧的 → 创建新的 → 启动新的）方面留下了关键的细节未解决，并且 `onReady` 回调设计引入了 `service_windows.go` 和 `main.go` 的结构更改，需要仔细的集成测试。

**Strengths**

- 清晰分离热重载：`hotreload.go` 将监视/差异逻辑与应用程序集成隔离开来
- `sync.Mutex` 用于线程安全的 config 访问是正确的方法
- `reflect.DeepEqual` 用于部分比较避免了不必要的组件重启
- 验证回退（"失败时回退到旧 config"）防止错误编辑导致 service 不可用
- 不热重载 `api.port` 和 `service` 部分是正确的
- `HotReloadCallbacks` 结构提供清晰的关注点分离

**Concerns**

- **[HIGH] 组件重启缺少 concurrency protection**：`handleConfigChange` 中的"停止旧的 → 创建新的 → 启动新的"周期与正在进行的工作没有 atomic。例如，`NetworkMonitor.Stop()` 被调用时，`NotificationManager` 可能仍在读取事件，可能 panic 或发送陈旧数据。
- **[HIGH] `onReady` 回调插入位置精确性**：`onReady` 必须在 SCM 报告 Running（第 79 行）和事件循环（第 83 行）之间调用。这是一个狭窄的窗口，plan 应明确显示精确代码位置。
- **[HIGH] `Instances` 热重载特别复杂**：`InstanceManager` 在 `AppComponents` 中被类型化为 `any`。热重载 instances 需要 type assertion 和完全替换（停止所有 → 重新创建 → 启动所有），还是部分 diff？Plan 未充分详细说明。
- **[MEDIUM] `viper.WatchConfig()` filesystem event edge cases**：Windows 上 fsnotify 在 atomic write 期间触发多个事件。应添加 debounce（如 500ms `time.AfterFunc`），避免单次编辑触发多次 reload。
- **[MEDIUM] `SelfUpdater` 没有 `Stop()` 方法**：热重载 `self_update` config 时创建新 Updater，但旧 Updater 可能被 `SelfUpdateHandler` 引用，创建 stale reference。
- **[MEDIUM] `BearerToken` 热重载需要 API server cooperation**：middleware 需要动态读取 token，而不是在 startup 时捕获。Plan 没有描述 API server 如何获取新 token。
- **[LOW] Console mode 排除不明确**：应在 `main.go` 中明确检查仅 service mode 启用 WatchConfig。

**Suggestions**

- 添加 debounce timer（500ms `time.AfterFunc`）合并快速 filesystem events
- Instances 回调明确：完全替换 `InstanceManager`（StopAll → recreate → StartAll），不做部分 diff
- BearerToken 热重载：API server auth middleware 通过 thread-safe accessor 读取 token
- onReady 回调中组件重启失败时记录错误继续运行，不让 service 崩溃
- AppShutdown 中集成 StopWatch() 调用
- 标记 hot reload 集成点便于未来维护

**Risk Assessment: HIGH** — 组件 lifecycle 管理（并发停止/启动/重建）是最大风险。Plan 正确识别了 architecture，但缺乏 concurrency safety 细节，且 Instances hot reload 和 BearerToken propagation 存在未解决复杂性。

---

## Consensus Summary

> 仅 1 个外部 reviewer（OpenCode），共识基于该 review 结果。

### Agreed Strengths
- Plan 01 低风险高置信度，完全符合用户决策
- `os.Exit(1)` 触发 SCM recovery 是优雅方案
- Plan 02 架构分离合理（hotreload.go 独立模块）
- `reflect.DeepEqual` 部分比较避免不必要的组件重启
- 验证回退策略防止 config 错误导致 service 不可用

### Agreed Concerns（按优先级）

1. **[HIGH] Plan 02 组件重建的并发安全** — stop/create/start 周期无 atomic 保护，可能与正在进行的操作冲突
2. **[HIGH] Instances 热重载复杂度** — 需要明确是完全替换还是部分 diff，`any` 类型断言的处理方式
3. **[HIGH] onReady 回调插入位置** — 需精确到行号的代码位置说明
4. **[MEDIUM] fsnotify debounce** — Windows 上文件保存可能触发多次事件，需要 debounce 机制
5. **[MEDIUM] SelfUpdater stale reference** — handler 引用的旧 Updater 不会被更新
6. **[MEDIUM] BearerToken propagation** — API middleware 需要动态读取 token 的机制

### Divergent Views

无（单一 reviewer）

---

*Review conducted: 2026-04-11*
*Reviewers: OpenCode (via GitHub Copilot)*

# Phase 40: Safety & Recovery - Context

**Gathered:** 2026-03-30
**Status:** Ready for planning

<domain>
## Phase Boundary

更新后程序自动重启新版本、发送 Pushover 通知（开始/完成）、启动时清理 .old 备份文件、启动时检测异常 .exe.old 文件存在并自动恢复旧版本。本阶段是 v0.8 Self-Update 里程碑的最后一个阶段。

本阶段不涉及：新的 HTTP API 端点（Phase 39 已完成）、自更新核心逻辑（Phase 38 已完成）、CI/CD 配置（Phase 37 已完成）。

</domain>

<decisions>
## Implementation Decisions

### 重启策略
- **D-01:** 直接退出 — 更新成功后在 goroutine 中执行 `cmd.Start` (复用 PoC 模式: `CREATE_NO_WINDOW`) + `os.Exit(0)`，跳过 graceful shutdown。简单快速，端口重用风险低。

### 通知集成
- **D-02:** 复用 Notifier interface — 通过构造函数注入 `SelfUpdateHandler`，复用 Phase 34 的 `Notifier` interface (duck typing, `Notify(title, message)` 方法)。与 `TriggerHandler` 注入模式一致。
- **D-03:** 开始 + 完成两次通知 — 自更新开始时发送通知（含当前版本和目标版本），完成时再发一次通知（含结果：成功/失败 + 版本信息 + 错误详情）。与 Phase 34 TriggerHandler 模式一致。

### .old 处理逻辑
- **D-04:** 状态文件标记 — 更新成功后写入 `.update-success` 状态文件（含时间戳和新版本号）。启动时检查：
  - `.update-success` 存在 → 上次更新成功 → 清理 `.old` + 删除 `.update-success`
  - `.update-success` 不存在但 `.exe.old` 存在 → 上次更新后崩溃 → 从 `.exe.old` 恢复旧版本

### 端口重绑策略
- **D-05:** 重试机制 — 新进程启动时如果 HTTP server 端口绑定失败（端口仍被旧进程占用），等待 500ms 重试，最多重试 5 次（总共 2.5s）。绑定成功后继续正常启动。

### Claude's Discretion
- 状态文件的具体命名和存放路径（建议 exe 同目录）
- 重启通知的具体消息格式和标题
- 端口重试的具体实现（循环 + time.Sleep vs ticker）
- .old 恢复的具体实现（文件复制 vs rename）
- 日志字段命名和上下文注入方式

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### PoC 验证代码（self-spawn 模式参考）
- `tmp/poc_selfupdate.go` — cmd.Start + os.Exit(0) 自重启模式，CREATE_NO_WINDOW 标志，OldSavePath 配置
- `tmp/poc_selfupdate_test.go` — PoC 自动化测试，.old 文件验证模式

### 现有代码架构
- `internal/selfupdate/selfupdate.go` — Updater.Update() 已实现 .old 备份（OldSavePath: exePath + ".old"）
- `internal/api/selfupdate_handler.go` — SelfUpdateHandler 异步更新 goroutine + atomic.Value 状态追踪
- `internal/api/server.go` — NewServer() 路由注册，selfUpdater 注入
- `internal/notifier/notifier.go` — Notifier struct + Notify(title, message) 方法，NewWithConfig() 构造
- `internal/api/trigger.go` — TriggerHandler 的 Notifier 注入模式（参考集成方式）
- `cmd/nanobot-auto-updater/main.go` — 启动序列、HTTP server 启动、信号处理

### 需求追踪
- `.planning/REQUIREMENTS.md` — SAFE-01 至 SAFE-04 需求定义
- `.planning/ROADMAP.md` — Phase 40 成功标准（4 条）

### 项目规范
- `CLAUDE.md` — 项目规则：使用中文回答，临时文件放 tmp/

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/notifier/notifier.go:Notifier` — Notify(title, message) 方法直接复用，已支持非阻塞发送
- `internal/api/selfupdate_handler.go:SelfUpdateHandler` — 更新 goroutine 已有 panic recovery，D-01 重启逻辑加入此 goroutine
- `tmp/poc_selfupdate.go:64-74` — self-spawn 代码片段可直接复用（cmd.Start + SysProcAttr + os.Exit(0)）
- `cmd/nanobot-auto-updater/main.go:149-164` — HTTP server 启动流程，D-05 重试逻辑加入此处

### Established Patterns
- Notifier interface 注入 (Phase 34): 构造函数参数 + 内部字段存储
- 异步 goroutine + panic recovery: SelfUpdateHandler 已建立此模式
- 上下文感知日志: `logger.With("source", "api-self-update")`
- 非阻塞通知: goroutine 发送，失败仅记日志不中断流程
- .old 备份: `selfupdate.Options{OldSavePath: exePath + ".old"}`

### Integration Points
- `internal/api/selfupdate_handler.go` — 新增 Notifier 参数 + 重启逻辑
- `internal/api/server.go:NewServer()` — 传递 Notifier 到 SelfUpdateHandler
- `cmd/nanobot-auto-updater/main.go` — 启动序列新增 .old 清理/恢复逻辑 + 端口重试
- `cmd/nanobot-auto-updater/main.go` — 新增 self-spawn 后的 .update-success 状态文件写入

</code_context>

<specifics>
## Specific Ideas

- Self-spawn 命令: `exec.Command(exePath, os.Args[1:]...)` — 保持相同的命令行参数（config 路径等）
- SysProcAttr: `&syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}` — 后台运行
- .update-success 文件内容: 时间戳 + 新版本号，JSON 格式（便于调试）
- 端口重试: 500ms 间隔，最多 5 次，每次 log.Warn 记录
- .old 恢复: `os.Rename(exePath+".old", exePath)` — 原子操作，然后 cmd.Start + os.Exit(0) 用恢复的旧版本重启

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 40-safety-recovery*
*Context gathered: 2026-03-30*

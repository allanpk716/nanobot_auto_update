# GSD Debug Knowledge Base

Resolved debug sessions. Used by `gsd-debugger` to surface known-pattern hypotheses at the start of new investigations.

---

## port-conflict-residual-process — 残留进程占用端口且启动命令缺少端口参数
- **Date:** 2026-03-24T15:30:00Z
- **Error patterns:** Port not yet available, retrying, 端口占用, 端口不可用, port conflict, residual process
- **Root cause:** 两个问题：(1) 自动启动流程缺少停止阶段导致残留进程占用端口；(2) start_command 配置缺少 --port 参数导致 nanobot 使用错误的端口
- **Fix:** (1) 在 StartAllInstances 启动前调用 StopForUpdate 清理残留进程；(2) 在 StartNanobotWithCapture 中自动检查并补充 --port 参数（如果命令中未包含）
- **Files changed:** internal/instance/manager.go, internal/lifecycle/starter.go
---

## nanobot-immediate-exit — nanobot实例启动后立即退出（context被取消导致进程被杀死）
- **Date:** 2026-03-26T18:05:00Z
- **Error patterns:** exit status 1, 立即退出, 进程被杀死, 几秒内退出, 进程启动后立即退出
- **Root cause:** main.go 第 172-173 行的 autoStartCtx 使用了 defer cancel()，导致当启动 goroutine 返回时，context 被立即取消。由于 starter.go 使用 exec.CommandContext(ctx, ...) 启动进程，当 context 被取消时，Go 运行时会杀死所有通过这个 context 启动的子进程。
- **Fix:** (1) 修改 starter.go，使用 context.Background() (detachedCtx) 而不是传入的 context 来创建命令，使进程生命周期独立于启动 context；(2) 同时使用 detachedCtx 启动 captureLogs goroutines，确保日志捕获不会被父 context 取消影响
- **Files changed:** internal/lifecycle/starter.go
---

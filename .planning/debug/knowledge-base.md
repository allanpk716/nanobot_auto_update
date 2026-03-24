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

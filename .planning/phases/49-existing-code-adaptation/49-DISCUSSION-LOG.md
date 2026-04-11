# Phase 49: Existing Code Adaptation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-11
**Phase:** 49-existing-code-adaptation
**Areas discussed:** SCM 重启策略, 配置热重载范围与方式, 守护进程适配, 工作目录适配确认

---

## SCM 重启策略

| Option | Description | Selected |
|--------|-------------|----------|
| A: net stop + net start | exec.Command shell 调用，无空窗期但有 shell 开销 | |
| B: SCM API | golang.org/x/sys/windows/svc/mgr 直接操作，最精确但最复杂 | |
| C: Exit + Recovery | os.Exit(1) 触发 SCM recovery policy 60s 后重启，最简单 | ✓ |

**User's choice:** C: Exit + Recovery
**Notes:** 接受 60 秒空窗期，因为自更新是低频操作。非零退出码确保 SCM 触发 recovery policy。

---

## 配置热重载方式

| Option | Description | Selected |
|--------|-------------|----------|
| A: viper.WatchConfig() | viper 内置 fsnotify 封装，代码量最少 | ✓ |
| B: 手动 fsnotify | 更精确控制（防抖、写入完成检测），代码量稍多 | |

**User's choice:** A: viper.WatchConfig()
**Notes:** 项目已依赖 viper 和 fsnotify，不引入新依赖。viper 封装足够满足需求。

---

## 配置热重载范围

| Option | Description | Selected |
|--------|-------------|----------|
| 全部可热重载项 | 实例、监控、Pushover、自更新、健康检查、API Token | ✓ |
| 仅基础配置 | 监控、Pushover、自更新、健康检查（不含实例） | |
| 最小范围 | 仅 Pushover、自更新 GitHub 配置 | |

**User's choice:** 全部可热重载项
**Notes:** 端口和服务配置不热重载。实例配置热重载需处理新增/删除/修改三种场景。

---

## 守护进程适配

| Option | Description | Selected |
|--------|-------------|----------|
| A: 加 IsServiceMode() 检查 | MakeDaemon/MakeDaemonSimple 开头添加服务模式检查，防御性编程 | ✓ |
| B: 不改动 | 当前未被 main.go 调用，保持代码简洁 | |

**User's choice:** A: 加 IsServiceMode() 检查
**Notes:** 代码量极少（2行），防御性编程即使未来被调用也安全。

---

## 工作目录适配

**确认：** main.go:74-83 已实现，无需讨论。Phase 49 只确认无误。

---

## Claude's Discretion

- 服务模式 restartFn 具体实现方式
- 配置热重载的组件重建策略
- viper.WatchConfig 的错误处理
- 配置重载失败时的降级策略
- 实例配置热重载的具体流程
- 热重载相关的测试策略

## Deferred Ideas

None — discussion stayed within phase scope

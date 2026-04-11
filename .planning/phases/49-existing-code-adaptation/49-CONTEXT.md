# Phase 49: Existing Code Adaptation - Context

**Gathered:** 2026-04-11
**Status:** Ready for planning

<domain>
## Phase Boundary

服务模式下所有现有功能（守护进程、自更新重启、文件路径、配置重载）正常工作，无需用户额外配置。控制台模式下所有行为与当前完全一致（无回归）。

具体交付：
1. daemon.go 在服务模式下跳过守护进程循环（ADPT-01）
2. 自更新后 restartFn 在服务模式下使用 SCM recovery policy 重启而非 self-spawn（ADPT-02）
3. 服务模式下工作目录自动设置为 exe 所在目录（ADPT-03 — 已实现）
4. 服务模式下监听 config.yaml 文件变更，自动重载配置无需重启服务（ADPT-04）
5. 控制台模式下所有行为无回归

不包括：服务注册/卸载（Phase 48 已完成）、svc.Handler 实现（Phase 47 已完成）。

</domain>

<decisions>
## Implementation Decisions

### SCM 重启策略 (ADPT-02)
- **D-01:** 服务模式下自更新后，restartFn 直接调用 `os.Exit(1)`（非零退出码），触发 SCM recovery policy 自动重启（Phase 48 D-07: 3x ServiceRestart, 60s 间隔, 24h 重置失败计数）
- **D-02:** 控制台模式下 restartFn 保持现有 self-spawn 行为不变（defaultRestartFn: exec.Command + os.Exit(0)）
- **D-03:** 服务模式检测方式：在 defaultRestartFn 内部调用 `lifecycle.IsServiceMode()` 判断当前运行模式，选择不同退出策略。无需注入不同 restartFn

### 配置热重载 (ADPT-04)
- **D-04:** 使用 `viper.WatchConfig()` 实现文件监听（viper 内置 fsnotify 封装，项目中已依赖两者，不引入新依赖）
- **D-05:** 热重载范围 — 全部可热重载项：
  - 实例配置（instances）— 新增/删除实例需启动/停止进程
  - 监控参数（monitor）— 间隔/超时需重建 NetworkMonitor
  - Pushover 配置（pushover）— Token/Key 需重建 Notifier
  - 自更新配置（self_update）— GitHub 配置需重建 SelfUpdater
  - 健康检查（health_check）— 间隔需重建 HealthMonitor
  - API Token（api.bearer_token）— 可更新认证配置
- **D-06:** 不热重载的配置项：
  - API 端口（api.port）— 变更需重启 HTTP 服务器，不在运行时处理
  - 服务配置（service）— auto_start 变更涉及服务注册，不应在运行时修改
- **D-07:** 热重载时仅重建受影响的组件，不做全量 AppShutdown/AppStartup 循环

### 守护进程适配 (ADPT-01)
- **D-08:** 在 `MakeDaemon()` 和 `MakeDaemonSimple()` 函数开头添加 `if IsServiceMode() { return false, nil }` 检查。防御性编程，即使当前未被 main.go 调用，未来调用也安全

### 工作目录适配 (ADPT-03)
- **D-09:** 已在 main.go:74-83 提前实现（服务模式下 os.Chdir 到 exe 所在目录），Phase 49 只需确认无误，无额外改动

### Claude's Discretion
- 服务模式 restartFn 的具体实现方式（在 defaultRestartFn 内检查 IsServiceMode vs 注入不同 fn）
- 配置热重载的组件重建策略（哪些组件 stop+recreate，重建顺序）
- viper.WatchConfig 的 OnConfigChange 回调中的错误处理和日志
- 配置重载失败时的降级策略（保留旧配置继续运行 vs 报错退出）
- 实例配置热重载的具体流程（新增实例启动、删除实例停止、参数变更重启）
- 热重载相关的测试策略

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 配置系统
- `internal/config/config.go` — Config 根结构体、Load() 函数、viper 初始化流程
- `internal/config/service.go` — ServiceConfig 结构体（service_name, display_name, auto_start）

### 守护进程
- `internal/lifecycle/daemon.go` — MakeDaemon/MakeDaemonSimple 现有实现（build tag: windows）
- `internal/lifecycle/servicedetect_windows.go` — IsServiceMode() 实现
- `internal/lifecycle/servicedetect.go` — 非 Windows 平台 stub

### 自更新重启
- `internal/api/selfupdate_handler.go:63-98` — restartFn 定义和 defaultRestartFn 实现
- `internal/api/selfupdate_handler.go:286` — restartFn 调用点

### 服务模式入口
- `cmd/nanobot-auto-updater/main.go` — 第 74-83 行（工作目录适配）、第 272-281 行（服务模式启动分支）
- `internal/lifecycle/service_windows.go` — ServiceHandler.Execute, RunService
- `internal/lifecycle/app.go` — AppStartup/AppShutdown, AppComponents, CreateComponentsFunc

### 服务管理
- `internal/lifecycle/servicemgr_windows.go` — RegisterService（含 recovery policy 配置）
- `internal/lifecycle/servicemgr.go` — 非 Windows 平台 stub

### 组件接口
- `internal/lifecycle/app.go:16-62` — Shutdownable, Stoppable, Startable, Closable, Cleanupable, APIServerControl, HealthMonitorControl, NotifySender 接口定义

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `viper.WatchConfig()` + `viper.OnConfigChange()` — viper 内置 fsnotify 封装，项目中 viper v1.20.1 已可用
- `fsnotify v1.9.0` — 已作为 viper 间接依赖存在于 go.mod，无需 go get
- `internal/lifecycle/servicedetect_windows.go` — IsServiceMode() 已有，daemon.go 和 defaultRestartFn 可直接调用
- `internal/lifecycle/app.go` — AppComponents 容器和组件接口已定义，热重载可复用 stop+recreate 模式
- `internal/config/config.go` Load() — viper.New() + SetDefault + ReadInConfig + Unmarshal 模式可参考

### Established Patterns
- build tags: `//go:build windows` 用于 Windows 特定代码，配套 `//go:build !windows` stub
- nil-safe 组件管理：关闭前检查组件是否为 nil
- 非阻塞关闭：组件 Stop() 方法使用 context 超时控制
- 上下文感知日志 (logger.With 预注入)
- 优雅降级 (失败不中断整体流程)
- 退出码策略：0=正常退出，1=错误退出，2=注册服务后退出

### Integration Points
- `cmd/nanobot-auto-updater/main.go:272-281` — 服务模式启动入口（RunService 调用点），可能需要在此处启动 config watcher
- `internal/lifecycle/app.go:170-290` — AppStartup 函数，热重载时可能需要调用其中的子步骤重建组件
- `internal/api/selfupdate_handler.go:85-98` — defaultRestartFn 需要添加 IsServiceMode 分支
- `internal/lifecycle/daemon.go:16-46` — MakeDaemon/MakeDaemonSimple 需要添加 IsServiceMode 检查

</code_context>

<specifics>
## Specific Ideas

- 服务模式自更新后的 60 秒空窗期（recovery policy 重启间隔）是可接受的，因为自更新是低频操作
- 配置热重载时日志应明确记录哪些配置项变更触发了哪些组件重建
- 实例配置热重载是最复杂的部分，需要仔细处理新增/删除/修改三种场景

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---
*Phase: 49-existing-code-adaptation*
*Context gathered: 2026-04-11*

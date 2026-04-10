# Roadmap: Nanobot Auto Updater

## Milestones

- **v1.0 Single Instance Auto-Update** - Phases 01-04 (shipped 2026-02-18)
- **v0.2 Multi-Instance Support** - Phases 05-18 (shipped 2026-03-16)
- **v0.4 Real-time Log Viewing** - Phases 19-23 (shipped 2026-03-20)
- **v0.5 Core Monitoring and Automation** - Phases 24-29 (shipped 2026-03-24)
- **v0.6 Update Log Recording and Query System** - Phases 30-33 (shipped 2026-03-29)
- **v0.7 Update Lifecycle Notifications** - Phases 34-35 (shipped 2026-03-29)
- **v0.8 Self-Update** - Phases 36-40 (shipped 2026-03-30)
- **v0.9 Startup Notification & Telegram Monitor** - Phases 41-43 (shipped 2026-04-06)
- **v0.10 管理界面自更新功能** - Phases 44-45 (shipped 2026-04-08)
- **v0.11 Windows 服务自启动** - Phases 46-49 (in progress)

## Phases

<details>
<summary>v1.0 Single Instance Auto-Update (Phases 01-04) - SHIPPED 2026-02-18</summary>

基础自动更新功能已交付。

</details>

<details>
<summary>v0.2 Multi-Instance Support (Phases 05-18) - SHIPPED 2026-03-16</summary>

多实例管理功能已交付。

</details>

<details>
<summary>v0.4 Real-time Log Viewing (Phases 19-23) - SHIPPED 2026-03-20</summary>

实时日志查看功能已交付。

</details>

<details>
<summary>v0.5 Core Monitoring and Automation (Phases 24-29) - SHIPPED 2026-03-24</summary>

核心监控和自动化功能已交付。

</details>

<details>
<summary>v0.6 Update Log Recording and Query System (Phases 30-33) - SHIPPED 2026-03-29</summary>

- [x] Phase 30: Log Structure and Recording (2/2 plans) - completed 2026-03-27
- [x] Phase 31: File Persistence (2/2 plans) - completed 2026-03-28
- [x] Phase 32: Query API (2/2 plans) - completed 2026-03-29
- [x] Phase 33: Integration and Testing (2/2 plans) - completed 2026-03-29

</details>

<details>
<summary>v0.7 Update Lifecycle Notifications (Phases 34-35) - SHIPPED 2026-03-29</summary>

- [x] Phase 34: Update Notification Integration (1/1 plan) - completed 2026-03-29
- [x] Phase 35: Notification Integration Testing (1/1 plan) - completed 2026-03-29

</details>

<details>
<summary>v0.8 Self-Update (Phases 36-40) - SHIPPED 2026-03-30</summary>

- [x] Phase 36: PoC Validation (1/1 plan) - completed 2026-03-29
- [x] Phase 37: CI/CD Pipeline (1/1 plan) - completed 2026-03-29
- [x] Phase 38: Self-Update Core (2/2 plans) - completed 2026-03-30
- [x] Phase 39: HTTP API Integration (2/2 plans) - completed 2026-03-30
- [x] Phase 40: Safety & Recovery (2/2 plans) - completed 2026-03-30

</details>

<details>
<summary>v0.9 Startup Notification & Telegram Monitor (Phases 41-43) - SHIPPED 2026-04-06</summary>

- [x] Phase 41: Startup Notification (2/2 plans) - completed 2026-04-06
- [x] Phase 42: Telegram Monitor Core (2/2 plans) - completed 2026-04-06
- [x] Phase 43: Telegram Monitor Integration (2/2 plans) - completed 2026-04-06

</details>

<details>
<summary>v0.10 管理界面自更新功能 (Phases 44-45) - SHIPPED 2026-04-08</summary>

- [x] Phase 44: 后端 -- 自更新进度追踪 + Web Token API (2/2 plans) - completed 2026-04-07
- [x] Phase 45: 前端 -- 自更新管理 UI (2/2 plans) - completed 2026-04-08

</details>

### v0.11 Windows 服务自启动 (In Progress)

**Milestone Goal:** 支持通过配置文件开启 Windows 服务模式，系统启动即运行，无需用户登录桌面。

- [ ] **Phase 46: Service Configuration & Mode Detection** - 配置驱动 auto_start 开关，启动时自动检测服务/控制台运行模式
- [ ] **Phase 47: Windows Service Handler** - 实现 svc.Handler 接口，处理服务生命周期和优雅关闭
- [ ] **Phase 48: Service Manager** - 服务注册/卸载/恢复策略的完整管理
- [ ] **Phase 49: Existing Code Adaptation** - 守护进程、重启机制和工作目录的服务模式适配

## Phase Details

### Phase 46: Service Configuration & Mode Detection
**Goal**: 用户通过 config.yaml 控制服务模式开关，程序启动时自动检测运行环境并选择正确模式
**Depends on**: Nothing (first phase of v0.11)
**Requirements**: MGR-01, SVC-01
**Success Criteria** (what must be TRUE):
  1. config.yaml 中 auto_start: true/false 配置项被正确加载和解析
  2. 程序在 Windows 服务上下文中启动时，svc.IsWindowsService() 返回 true，程序进入服务模式
  3. 程序在命令行直接运行时，svc.IsWindowsService() 返回 false，程序进入控制台模式（行为与当前完全一致）
**Plans**: 2 plans

Plans:
- [ ] 46-01-PLAN.md — ServiceConfig 结构体、Validate()、表格驱动测试、Config 集成 (TDD)
- [ ] 46-02-PLAN.md — svc.IsWindowsService() 检测封装、main.go 入口分支逻辑

### Phase 47: Windows Service Handler
**Goal**: Windows SCM 能通过标准服务接口启动和停止程序，服务生命周期完全可控
**Depends on**: Phase 46
**Requirements**: SVC-02, SVC-03
**Success Criteria** (what must be TRUE):
  1. Windows SCM 调用 Start 时，程序通过 svc.Handler.Execute 方法启动所有业务逻辑（实例管理、HTTP API、定时任务等）
  2. SCM 发送 Stop 控制码时，程序在 30 秒内完成资源清理并退出（关闭 HTTP 服务器、停止实例、清理 goroutine）
  3. SCM 发送 Shutdown 控制码时（系统关机），程序同样执行优雅关闭流程
  4. 服务在 Windows 服务管理器中显示为 "Running" 状态，停止后显示为 "Stopped"
**Plans**: TBD

Plans:
- [ ] 47-01: TBD
- [ ] 47-02: TBD

### Phase 48: Service Manager
**Goal**: 用户设置 auto_start: true 后，程序自动完成服务注册和恢复策略配置，无需手动操作 sc.exe
**Depends on**: Phase 47
**Requirements**: MGR-02, MGR-03, MGR-04
**Success Criteria** (what must be TRUE):
  1. auto_start: true 时，程序以管理员权限运行后自动注册为 Windows 服务（SCM CreateService）并退出
  2. auto_start: false 时，程序检测到已注册服务则自动卸载（SCM DeleteService）并退出
  3. 服务配置了 SCM 恢复策略：失败后自动重启，无需人工干预
  4. 非 Windows 平台编译时 auto_start 配置被忽略，不影响其他功能
**Plans**: TBD

Plans:
- [ ] 48-01: TBD
- [ ] 48-02: TBD

### Phase 49: Existing Code Adaptation
**Goal**: 服务模式下所有现有功能（守护进程、自更新重启、文件路径、配置重载）正常工作，无需用户额外配置
**Depends on**: Phase 48
**Requirements**: ADPT-01, ADPT-02, ADPT-03, ADPT-04
**Success Criteria** (what must be TRUE):
  1. 服务模式下 daemon.go 不进入守护进程循环（因为 SCM 负责进程生命周期管理）
  2. 自更新后 restartFn 使用 SCM 重启（net stop + net start）而非 self-spawn，确保服务状态正确
  3. 服务模式下工作目录自动设置为 exe 所在目录（非 System32），config.yaml 和日志文件路径正确解析
  4. 服务模式下监听 config.yaml 文件变更，自动重载配置（无需重启服务）
  5. 控制台模式下 daemon.go、restartFn、工作目录行为与当前完全一致（无回归）
**Plans**: TBD

Plans:
- [ ] 49-01: TBD
- [ ] 49-02: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 46 -> 47 -> 48 -> 49

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 46. Service Configuration & Mode Detection | v0.11 | 0/2 | Planning complete | - |
| 47. Windows Service Handler | v0.11 | 0/? | Not started | - |
| 48. Service Manager | v0.11 | 0/? | Not started | - |
| 49. Existing Code Adaptation | v0.11 | 0/? | Not started | - |

---
*Last updated: 2026-04-10 (Phase 46 planned — 2 plans in 2 waves)*

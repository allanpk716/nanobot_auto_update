# Roadmap: Nanobot Auto Updater

## Milestones

- ✅ **v1.0 单实例自动更新** - Phases 1-5 (shipped 2026-02-18)
- 🚧 **v0.2 多实例支持** - Phases 6-10 (in progress)

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

<details>
<summary>✅ v1.0 单实例自动更新 (Phases 1-5) - SHIPPED 2026-02-18</summary>

### Phase 1: Infrastructure
**Goal**: Application foundation with logging, configuration, and CLI
**Plans**: 4 plans

Plans:
- [x] 01-01-PLAN.md - Logging module with custom format and rotation (INFR-01, INFR-02)
- [x] 01-02-PLAN.md - Config enhancement with cron field and viper integration (INFR-03, INFR-04)
- [x] 01-03-PLAN.md - CLI entry point with pflag and integration (INFR-05, INFR-06, INFR-07, INFR-08, INFR-09)
- [x] 01-04-PLAN.md - Fix log format to exact specification (INFR-01 gap closure)
- [x] 01.1-01-PLAN.md - Config and process detection (NanobotConfig, FindPIDByPort)
- [x] 01.1-02-PLAN.md - Stopper and starter (graceful stop, hidden start, port verification)
- [x] 01.1-03-PLAN.md - Lifecycle manager and integration (orchestrator, config validation)

### Phase 2: Core Update Logic
**goal**: Nanobot can be updated from GitHub main branch with automatic fallback to stable version
**Plans**: 2 plans

Plans:
- [x] 02-01-PLAN.md - UV installation checker (UPDT-01, UPDT-02)
- [x] 02-02-PLAN.md - Core update logic with GitHub primary and PyPI fallback (UPDT-03, UPDT-04, UPDT-05, INFR-10)

### Phase 3: Scheduling and Notifications
**goal**: Updates run automatically on schedule and user is notified of failures
**Plans**: 3 plans

Plans:
- [x] 03-01-PLAN.md - Scheduler package with SkipIfStillRunning mode (SCHD-01, SCHD-03)
- [x] 03-02-PLAN.md - Notifier package with Pushover and graceful missing config (NOTF-01, NOTF-02, NOTF-03, NOTF-04)
- [x] 03-03-PLAN.md - Main.go scheduled mode integration (SCHD-01, SCHD-02, SCHD-03, NOTF-02, NOTF-03)

### Phase 4: Runtime Integration
**goal**: Program runs as a Windows background service without visible console window
**Plans**: 1 plan

Plans:
- [x] 04-01-PLAN.md - Makefile with build and build-release targets (RUN-01, RUN-02)

### Phase 5: CLI Immediate Update
**goal:** Add --update-now flag for immediate update execution with JSON output
**plans**: 1 plan

Plans:
- [x] 05-01-PLAN.md - Add --update-now flag with JSON output, --timeout flag, remove --run-once (CLI-01, CLI-02, CLI-03, CLI-04, CLI-05)

</details>

### 🚧 v0.2 多实例支持 (In Progress)

**Milestone Goal:** 支持同时管理多个 nanobot 实例的升级和启动

#### Phase 6: 配置扩展
**Goal**: 用户可以在 YAML 配置文件中定义多个 nanobot 实例
**Depends on**: Phase 5
**Requirements**: CONF-01, CONF-02, CONF-03
**Success Criteria** (what must be TRUE):
  1. 用户可以在 config.yaml 中使用 instances 数组定义多个实例,每个实例包含 name、port、start_command 字段
  2. 程序启动时自动验证实例名称唯一性,发现重复时立即报错退出并显示清晰的错误信息
  3. 程序启动时自动验证端口唯一性,发现重复时立即报错退出并显示清晰的错误信息
  4. 旧的 v1.0 配置文件(无 instances 字段)仍然可以正常加载和使用
**Plans**: 2 plans

Plans:
- [x] 06-01-PLAN.md — 扩展配置结构支持多实例,创建 InstanceConfig 和验证逻辑
- [x] 06-02-PLAN.md — 创建测试用例和测试数据,验证多实例配置正确性

#### Phase 7: 生命周期扩展
**Goal**: 为每个实例提供独立的上下文感知生命周期管理
**Depends on**: Phase 6
**Requirements**: LIFECYCLE-01 (部分), LIFECYCLE-02 (部分)
**Success Criteria** (what must be TRUE):
  1. 每个实例的所有日志消息都包含实例名称,用户可以轻松追踪哪个实例发生了什么
  2. 系统可以为特定名称的实例执行停止操作
  3. 系统可以为特定名称的实例执行启动操作
  4. 停止和启动操作复用现有的 v1.0 生命周期逻辑,无需重写底层实现
**Plans**: 1 plan

Plans:
- [x] 07-01-PLAN.md — 实现 InstanceLifecycle 包装器和 InstanceError 自定义错误类型,重构 StartNanobot 支持动态命令参数

#### Phase 8: 实例协调器
**Goal**: InstanceManager 可以协调所有实例的停止→更新→启动流程
**Depends on**: Phase 7
**Requirements**: LIFECYCLE-01, LIFECYCLE-02, LIFECYCLE-03, ERROR-02
**Success Criteria** (what must be TRUE):
  1. 用户执行更新时,系统按顺序停止所有配置的实例(串行执行)
  2. 用户执行更新时,系统在所有实例停止后执行一次 UV 更新操作(全局更新)
  3. 用户执行更新时,系统按顺序启动所有配置的实例(串行执行)
  4. 某个实例停止失败时,系统记录错误但继续停止其他实例(优雅降级)
  5. 某个实例启动失败时,系统记录错误但继续启动其他实例(优雅降级)
  6. 系统收集所有实例的操作结果(成功/失败状态和错误信息),不丢失任何失败信息
**Plans**: 1 plan

Plans:
- [x] 08-01-PLAN.md — 实现 InstanceManager 协调器,UpdateResult 和 UpdateError 错误聚合

#### Phase 9: 通知扩展
**Goal**: 失败通知包含具体哪些实例失败及其失败原因
**Depends on**: Phase 8
**Requirements**: ERROR-01
**Success Criteria** (what must be TRUE):
  1. 更新完成后,如果有实例失败,用户收到一条 Pushover 通知,列出所有失败的实例名称
  2. 通知消息包含每个失败实例的具体操作(停止失败或启动失败)和错误原因
  3. 通知消息包含成功启动的实例列表,用户可以了解整体状态
  4. 所有实例都成功时,不发送失败通知(避免不必要的打扰)
  5. 同一批次的多个实例失败只发送一条通知,避免通知风暴
**Plans**: 1 plan

Plans:
- [x] 09-01-PLAN.md — 扩展通知支持多实例失败报告

#### Phase 10: 主程序集成
**Goal**: 主程序集成 InstanceManager,完整的多实例更新流程可用
**Depends on**: Phase 9
**Requirements**: 所有 v0.2 需求的端到端验证
**Success Criteria** (what must be TRUE):
  1. 定时任务触发时,系统自动执行"停止所有→更新→启动所有"的完整流程
  2. 使用 --update-now 参数时,系统执行一次完整的多实例更新流程并退出
  3. 用户可以通过日志查看每个实例的详细操作过程和状态
  4. 多实例场景下的资源使用合理,无内存泄漏或句柄泄漏
  5. 长期运行(24x7)稳定,多次更新周期后系统仍然正常工作
**Plans**: 2 plans

Plans:
- [x] 10-01-PLAN.md — 集成 InstanceManager 到主程序,支持多实例定时更新和 --update-now 模式
- [ ] 10-02-PLAN.md — 增强多实例配置加载日志,输出每个实例的详细信息(名称、端口、启动命令)

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 1.1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Infrastructure | v1.0 | 4/4 | Complete | 2026-02-18 |
| 01.1. Nanobot Lifecycle | v1.0 | 3/3 | Complete | 2026-02-18 |
| 2. Core Update Logic | v1.0 | 2/2 | Complete | 2026-02-18 |
| 3. Scheduling and Notifications | v1.0 | 3/3 | Complete | 2026-02-18 |
| 4. Runtime Integration | v1.0 | 1/1 | Complete | 2026-02-18 |
| 5. CLI Immediate Update | v1.0 | 1/1 | Complete | 2026-02-18 |
| 6. 配置扩展 | v0.2 | 2/2 | Complete | 2026-03-10 |
| 7. 生命周期扩展 | v0.2 | 1/1 | Complete | 2026-03-11 |
| 8. 实例协调器 | v0.2 | 1/1 | Complete | 2026-03-11 |
| 9. 通知扩展 | v0.2 | 1/1 | Complete | 2026-03-11 |
| 10. 主程序集成 | 2/2 | Complete    | 2026-03-13 | - |

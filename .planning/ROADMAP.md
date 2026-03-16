# Roadmap: Nanobot Auto Updater

## Milestones

- ✅ **v1.0 单实例自动更新** - Phases 1-5 (shipped 2026-02-18)
- ✅ **v0.2 多实例支持** - Phases 6-10 (shipped 2026-03-16)

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

<details>
<summary>✅ v1.0 单实例自动更新 (Phases 1-5) - SHIPPED 2026-02-18</summary>

### Phase 1: Infrastructure
**Goal**: Application foundation with logging, configuration, and CLI
**Plans**: 7 plans (includes Phase 1.1 Nanobot Lifecycle)

### Phase 2: Core Update Logic
**goal**: Nanobot can be updated from GitHub main branch with automatic fallback to stable version
**Plans**: 2 plans

### Phase 3: Scheduling and Notifications
**goal**: Updates run automatically on schedule and user is notified of failures
**Plans**: 3 plans

### Phase 4: Runtime Integration
**goal**: Program runs as a Windows background service without visible console window
**Plans**: 1 plan

### Phase 5: CLI Immediate Update
**goal:** Add --update-now flag for immediate update execution with JSON output
**plans**: 1 plan

**详细计划**: See `.planning/milestones/v1.0-ROADMAP.md`

</details>

<details>
<summary>✅ v0.2 多实例支持 (Phases 6-10) - SHIPPED 2026-03-16</summary>

### Phase 6: 配置扩展
**Goal**: 用户可以在 YAML 配置文件中定义多个 nanobot 实例
**Plans**: 2 plans
**Completed**: 2026-03-10

### Phase 7: 生命周期扩展
**Goal**: 为每个实例提供独立的上下文感知生命周期管理
**Plans**: 1 plan
**Completed**: 2026-03-11

### Phase 8: 实例协调器
**Goal**: InstanceManager 可以协调所有实例的停止→更新→启动流程
**Plans**: 1 plan
**Completed**: 2026-03-11

### Phase 9: 通知扩展
**Goal**: 失败通知包含具体哪些实例失败及其失败原因
**Plans**: 1 plan
**Completed**: 2026-03-11

### Phase 10: 主程序集成
**Goal**: 主程序集成 InstanceManager,完整的多实例更新流程可用
**Plans**: 2 plans
**Completed**: 2026-03-13

**详细计划**: See `.planning/milestones/v0.2-ROADMAP.md`

</details>

## Progress

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
| 10. 主程序集成 | v0.2 | 2/2 | Complete | 2026-03-13 |

**Total Progress:** 10 phases, 18 plans, all complete

---

_Next milestone planning: Use `/gsd:new-milestone`_

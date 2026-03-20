# Roadmap: Nanobot Auto Updater

## Milestones

- **v1.0 MVP** - Phases 1-4 (shipped 2026-02-18)
- **v0.2 Multi-Instance** - Phases 5-18 (shipped 2026-03-16)
- **v0.4 Real-time Logs** - Phases 19-23 (shipped 2026-03-20)
- **v0.5 Core Monitoring and Automation** - Phases 24-28 (in progress)

## Phases

<details>
<summary>v1.0 MVP (Phases 1-4) - SHIPPED 2026-02-18</summary>

- [x] Phase 1: 基础配置和日志 (3 plans)
- [x] Phase 01.1: Nanobot 生命周期管理 (2 plans)
- [x] Phase 2: UV 检测和更新逻辑 (2 plans)
- [x] Phase 3: 调度和通知 (2 plans)
- [x] Phase 4: 运行时集成 (1 plan)

</details>

<details>
<summary>v0.2 Multi-Instance (Phases 5-18) - SHIPPED 2026-03-16</summary>

多实例支持里程碑，支持同时管理多个 nanobot 实例的升级和启动。包含 5 个阶段，7 个计划，8 个任务，约 5000 行 Go 代码。

</details>

<details>
<summary>v0.4 Real-time Logs (Phases 19-23) - SHIPPED 2026-03-20</summary>

实时日志查看功能，通过 SSE 流式传输和 Web UI 访问。包含 5 个阶段，11 个计划，33 个需求，约 12,000 行代码增加。

- [x] Phase 19: Log Buffer Core (2/2 plans) — completed 2026-03-17
- [x] Phase 20: Log Capture Integration (2/2 plans) — completed 2026-03-17
- [x] Phase 21: Instance Management Integration (2/2 plans) — completed 2026-03-17
- [x] Phase 22: SSE Streaming API (2/2 plans) — completed 2026-03-18
- [x] Phase 23: Web UI and Error Handling (3/3 plans) — completed 2026-03-19

**Key features:**
- Thread-safe ring buffer (5000 lines, concurrent R/W, FIFO)
- Stdout/stderr capture with os.Pipe()
- Per-instance LogBuffer with lifecycle integration
- SSE streaming API with history and heartbeat
- Embedded Web UI with instance selector and error handling

</details>

### v0.5 Core Monitoring and Automation (In Progress)

核心监控和自动化功能，补全服务基础设施。

- [ ] **Phase 24: Auto-start** - 应用启动时自动启动所有配置的实例
- [ ] **Phase 25: Instance Health Monitoring** - 定期检查实例运行状态
- [ ] **Phase 26: Network Monitoring Core** - 定期测试 Google 连通性
- [ ] **Phase 27: Network Monitoring Notifications** - 连通性变化时发送通知
- [ ] **Phase 28: HTTP API Trigger** - 通过 HTTP API 触发更新流程

## Phase Details

### Phase 24: Auto-start

**Goal:** 应用启动时自动启动所有配置的实例，无需手动干预

**Depends on:** None (独立功能，但需要 v0.2 实例管理基础设施)

**Requirements:** AUTOSTART-01, AUTOSTART-02, AUTOSTART-03, AUTOSTART-04

**Success Criteria** (what must be TRUE):
1. 用户启动应用后，所有配置的实例自动按顺序启动
2. 用户可以通过日志看到每个实例的启动状态（成功或失败）
3. 某个实例启动失败时，其他实例仍然继续启动
4. 所有实例启动完成后，用户可以在日志中看到汇总状态（成功/失败数量）

**Plans:** 2/4 plans executed

Plans:
- [ ] 24-01-PLAN.md - 配置扩展：添加 AutoStart 字段
- [ ] 24-02-PLAN.md - InstanceManager 扩展：添加 StartAllInstances 方法
- [ ] 24-03-PLAN.md - main.go 集成：异步启动实例

---

### Phase 25: Instance Health Monitoring

**Goal:** 用户可以实时了解每个实例的运行状态，无需手动检查

**Depends on:** Phase 24 (需要实例已启动)

**Requirements:** HEALTH-01, HEALTH-02, HEALTH-03, HEALTH-04

**Success Criteria** (what must be TRUE):
1. 系统定期（默认间隔）检查每个实例是否在运行（通过端口监听）
2. 实例从运行变为停止时，用户可以在 ERROR 日志中看到记录
3. 实例从停止恢复为运行时，用户可以在 INFO 日志中看到记录
4. 用户可以通过配置文件调整健康检查的间隔时间

**Plans:** TBD

---

### Phase 26: Network Monitoring Core

**Goal:** 系统定期监控网络连通性，记录 Google 可达性状态

**Depends on:** None (独立功能)

**Requirements:** MONITOR-01, MONITOR-02, MONITOR-03, MONITOR-06

**Success Criteria** (what must be TRUE):
1. 系统定期（默认间隔）向 google.com 发送 HTTP 请求测试连通性
2. 请求失败时，用户可以在 ERROR 日志中看到失败的详细信息
3. 请求成功时，用户可以在 INFO 日志中看到成功的记录
4. 用户可以通过配置文件调整监控间隔和请求超时时间

**Plans:** TBD

---

### Phase 27: Network Monitoring Notifications

**Goal:** 网络连通性状态变化时，用户收到 Pushover 通知

**Depends on:** Phase 26 (需要连通性监控基础设施)

**Requirements:** MONITOR-04, MONITOR-05

**Success Criteria** (what must be TRUE):
1. 连通性从失败恢复为成功时，用户收到 Pushover 恢复通知
2. 连通性从成功变为失败时，用户收到 Pushover 失败通知

**Plans:** TBD

---

### Phase 28: HTTP API Trigger

**Goal:** 用户可以通过 HTTP API 远程触发更新流程

**Depends on:** Phase 24 (需要实例自动启动能力)

**Requirements:** API-01, API-02, API-03, API-04, API-05, API-06

**Success Criteria** (what must be TRUE):
1. 用户发送 POST /api/v1/trigger-update 请求（带 Bearer Token）可以触发更新
2. 认证失败的请求返回 401 错误，不触发更新
3. 更新流程执行完整的停止-更新-启动过程
4. 用户收到 JSON 格式的更新结果（成功/失败、详细信息）
5. 更新过程中重复请求被拒绝，用户收到"更新进行中"的错误消息

**Plans:** TBD

---

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. 基础配置和日志 | v1.0 | 3/3 | Complete | 2026-02-18 |
| 01.1. Nanobot 生命周期管理 | v1.0 | 2/2 | Complete | 2026-02-18 |
| 2. UV 检测和更新逻辑 | v1.0 | 2/2 | Complete | 2026-02-18 |
| 3. 调度和通知 | v1.0 | 2/2 | Complete | 2026-02-18 |
| 4. 运行时集成 | v1.0 | 1/1 | Complete | 2026-02-18 |
| 5-18. Multi-Instance | v0.2 | 7/7 | Complete | 2026-03-16 |
| 19. Log Buffer Core | v0.4 | 2/2 | Complete | 2026-03-17 |
| 20. Log Capture Integration | v0.4 | 2/2 | Complete | 2026-03-17 |
| 21. Instance Management Integration | v0.4 | 2/2 | Complete | 2026-03-17 |
| 22. SSE Streaming API | v0.4 | 2/2 | Complete | 2026-03-18 |
| 23. Web UI and Error Handling | v0.4 | 3/3 | Complete | 2026-03-19 |
| 24. Auto-start | 2/4 | In Progress|  | - |
| 25. Instance Health Monitoring | v0.5 | 0/0 | Not started | - |
| 26. Network Monitoring Core | v0.5 | 0/0 | Not started | - |
| 27. Network Monitoring Notifications | v0.5 | 0/0 | Not started | - |
| 28. HTTP API Trigger | v0.5 | 0/0 | Not started | - |

---

*Last updated: 2026-03-20 after Phase 24 planning*

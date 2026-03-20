# Roadmap: Nanobot Auto Updater

## Milestones

- ✅ **v1.0 MVP** - Phases 1-4 (shipped 2026-02-18)
- ✅ **v0.2 Multi-Instance** - Phases 5-18 (shipped 2026-03-16)
- ✅ **v0.4 Real-time Logs** - Phases 19-23 (shipped 2026-03-20)
- 📋 **v0.5** - Future release (planned)

## Phases

<details>
<summary>✅ v1.0 MVP (Phases 1-4) — SHIPPED 2026-02-18</summary>

- [x] Phase 1: 基础配置和日志 (3 plans)
- [x] Phase 01.1: Nanobot 生命周期管理 (2 plans)
- [x] Phase 2: UV 检测和更新逻辑 (2 plans)
- [x] Phase 3: 调度和通知 (2 plans)
- [x] Phase 4: 运行时集成 (1 plan)

</details>

<details>
<summary>✅ v0.2 Multi-Instance (Phases 5-18) — SHIPPED 2026-03-16</summary>

多实例支持里程碑，支持同时管理多个 nanobot 实例的升级和启动。包含 5 个阶段，7 个计划，8 个任务，约 5000 行 Go 代码。

</details>

<details>
<summary>✅ v0.4 Real-time Logs (Phases 19-23) — SHIPPED 2026-03-20</summary>

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

### 📋 v0.5 (Planned)

Future milestone - requirements to be defined.

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

---

*Last updated: 2026-03-20 after v0.4 milestone completion*

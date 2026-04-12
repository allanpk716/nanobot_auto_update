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
- **v0.11 Windows 服务自启动** - Phases 46-49 (shipped 2026-04-11)

## Phases

**Phase Numbering:**
- Integer phases (50, 51, 52, 53): Planned milestone work
- Decimal phases (50.1, 50.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

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

<details>
<summary>v0.11 Windows 服务自启动 (Phases 46-49) - SHIPPED 2026-04-11</summary>

- [x] Phase 46: Service Configuration & Mode Detection (2/2 plans) - completed 2026-04-10
- [x] Phase 47: Windows Service Handler (2/2 plans) - completed 2026-04-10
- [x] Phase 48: Service Manager (2/2 plans) - completed 2026-04-11
- [x] Phase 49: Existing Code Adaptation (2/2 plans) - completed 2026-04-11

</details>

### v0.12 实例管理与配置编辑 (In Progress)

**Milestone Goal:** 在 Web 后台界面完整管理 nanobot 实例的生命周期（CRUD + 启停）和 nanobot 自身配置文件

- [x] **Phase 50: Instance Config CRUD API** - Backend API for creating, reading, updating, deleting instance configurations with validation (completed 2026-04-11)
- [x] **Phase 51: Instance Lifecycle Control API** - Backend API for start/stop operations on individual instances with auth (completed 2026-04-12)
- [ ] **Phase 52: Nanobot Config Management API** - Backend API for reading/writing nanobot config.json per instance with auto-directory setup
- [ ] **Phase 53: Instance Management UI** - Full web interface with instance cards, CRUD dialogs, lifecycle controls, and nanobot config editor

## Phase Details

### Phase 50: Instance Config CRUD API
**Goal**: Users can manage instance configurations through a validated REST API that auto-persists to config.yaml
**Depends on**: Phase 49 (existing config hot reload mechanism)
**Requirements**: IC-01, IC-02, IC-03, IC-04, IC-05, IC-06
**Success Criteria** (what must be TRUE):
  1. User can create a new instance via POST with all config fields (name, port, start_command, startup_timeout, auto_start) and it appears in config.yaml
  2. User can update an existing instance's configuration via PUT and changes are reflected in config.yaml within 500ms
  3. User can delete an instance via DELETE — running instances are stopped first, then removed from config.yaml
  4. User can copy an instance via POST — auto-updater config is cloned with new name/port and nanobot config directory is created
  5. Invalid configs are rejected with clear error messages (duplicate name, duplicate port, missing required fields, port out of range)
**Plans**: 2 plans

Plans:
- [x] 50-01-PLAN.md — SaveConfig function + InstanceConfigHandler with all 6 CRUD endpoints + route registration
- [x] 50-02-PLAN.md — Comprehensive handler tests and SaveConfig tests (TDD)

### Phase 51: Instance Lifecycle Control API
**Goal**: Users can start and stop individual instances on demand through authenticated API endpoints
**Depends on**: Phase 50 (instance config must exist to control lifecycle)
**Requirements**: LC-01, LC-02, LC-03
**Success Criteria** (what must be TRUE):
  1. User can start a stopped instance via POST /api/v1/instances/{name}/start and the instance begins listening on its configured port
  2. User can stop a running instance via POST /api/v1/instances/{name}/stop and the instance process terminates
  3. All CRUD and lifecycle endpoints return 401 Unauthorized when Bearer token is missing or incorrect (reuses existing constant-time auth)
**Plans**: 2 plans

Plans:
- [x] 51-01-PLAN.md — InstanceLifecycleHandler with HandleStart/HandleStop + route registration with auth middleware
- [x] 51-02-PLAN.md — Comprehensive handler tests for start/stop/auth/error scenarios

### Phase 52: Nanobot Config Management API
**Goal**: Users can read and write nanobot's own config.json for any instance through the API, with automatic directory and default config creation
**Depends on**: Phase 50 (instance must exist in auto-updater config to manage its nanobot config)
**Requirements**: NC-01, NC-02, NC-03, NC-04
**Success Criteria** (what must be TRUE):
  1. Creating a new instance auto-creates the nanobot config directory (e.g., ~/.nanobot-{name}/) with a minimal valid config.json
  2. User can read any instance's nanobot config.json via GET /api/v1/instances/{name}/nanobot-config and receive valid JSON
  3. User can update any instance's nanobot config.json via PUT /api/v1/instances/{name}/nanobot-config and the file is updated on disk
  4. Copying an instance clones the nanobot config.json to the new directory with port and name fields updated
**Plans**: TBD

### Phase 53: Instance Management UI
**Goal**: Users can manage all instances and nanobot configurations through a visual web interface without touching config files
**Depends on**: Phase 51, Phase 52 (all backend APIs must be available)
**Requirements**: UI-01, UI-02, UI-03, UI-04, UI-05, UI-06
**Success Criteria** (what must be TRUE):
  1. User sees an instance list page with cards showing name, port, command, running status indicator, and action buttons (start/stop/edit/delete/copy)
  2. User can create a new instance via a dialog with all config fields and a nanobot config editor, and it appears in the list immediately
  3. User can edit an existing instance's configuration via dialog and changes persist to config.yaml
  4. User can copy an instance via dialog with new name/port, and both auto-updater and nanobot configs are cloned
  5. User can delete an instance with a confirmation dialog that warns if the instance is running
  6. User can edit nanobot config via a hybrid editor with structured form for common fields (API key, model, telegram token) and raw JSON text editor
**Plans**: TBD
**UI hint**: yes

## Progress

**Execution Order:**
Phases execute in numeric order: 50 -> 51 -> 52 -> 53

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 50. Instance Config CRUD API | v0.12 | 2/2 | Complete    | 2026-04-11 |
| 51. Instance Lifecycle Control API | v0.12 | 2/2 | Complete    | 2026-04-12 |
| 52. Nanobot Config Management API | v0.12 | 0/? | Not started | - |
| 53. Instance Management UI | v0.12 | 0/? | Not started | - |

---
*Last updated: 2026-04-11 (Phase 51 planned)*

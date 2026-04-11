# Requirements: Nanobot Auto Updater — v0.12

**Defined:** 2026-04-11
**Core Value:** 自动保持 nanobot 处于最新版本，无需用户手动干预。

## v0.12 Requirements

Requirements for v0.12 实例管理与配置编辑 milestone. Each maps to roadmap phases.

### Instance Config (IC) — 实例配置 CRUD

- [ ] **IC-01**: User can create new instance via API with all config fields (name, port, start_command, startup_timeout, auto_start)
- [ ] **IC-02**: User can update existing instance configuration via API
- [ ] **IC-03**: User can delete instance via API (stops running instance first)
- [ ] **IC-04**: User can copy an instance (clones auto-updater config with new name/port + nanobot config to new directory)
- [ ] **IC-05**: All config changes auto-persist to config.yaml and trigger hot reload (reuses existing 500ms debounce mechanism)
- [ ] **IC-06**: Config validation — unique name across instances, unique port across instances, required fields non-empty, port range 1-65535

### Lifecycle Control (LC) — 生命周期控制

- [ ] **LC-01**: User can start a stopped instance via API (POST /api/v1/instances/{name}/start)
- [ ] **LC-02**: User can stop a running instance via API (POST /api/v1/instances/{name}/stop)
- [ ] **LC-03**: All new CRUD and lifecycle endpoints require Bearer token authentication (reuses existing auth mechanism)

### Nanobot Config (NC) — Nanobot 配置管理

- [ ] **NC-01**: Creating a new instance auto-creates nanobot config directory (e.g., ~/.nanobot-{name}/) and default config.json with minimal valid configuration
- [ ] **NC-02**: User can read nanobot's config.json for any instance via API (GET /api/v1/instances/{name}/nanobot-config)
- [ ] **NC-03**: User can update nanobot's config.json for any instance via API (PUT /api/v1/instances/{name}/nanobot-config)
- [ ] **NC-04**: Copy instance clones nanobot config.json to new directory with port/name updated

### Instance Management UI (UI) — 管理界面

- [ ] **UI-01**: Redesigned instance list page — full instance cards showing config details (name, port, command), running status indicator, and action buttons (start/stop/restart/edit/delete/copy)
- [ ] **UI-02**: Create instance dialog with all config fields (name, port, start_command, startup_timeout, auto_start) and nanobot config editor
- [ ] **UI-03**: Edit instance dialog — modify all auto-updater config fields, changes auto-persist to config.yaml
- [ ] **UI-04**: Copy instance dialog — clone config with new name/port, clone nanobot config to new directory
- [ ] **UI-05**: Delete instance confirmation dialog — warns if instance is running, offers to stop first
- [ ] **UI-06**: Nanobot config hybrid editor — structured form for common parameters (providers API key, model, telegram token) + full JSON text editor with syntax highlighting

## Future Requirements

Deferred to future milestones.

### Advanced Instance Management

- **AIM-01**: Instance config import/export (backup/restore)
- **AIM-02**: Batch operations (start all, stop all)
- **AIM-03**: Instance drag-and-drop reordering
- **AIM-04**: Config version history and rollback

### Enhanced Nanobot Config

- **ENC-01**: Nanobot config validation against nanobot schema
- **ENC-02**: Nanobot config template library (pre-built configs for common use cases)
- **ENC-03**: Per-provider form editors (Telegram setup wizard, etc.)

## Out of Scope

| Feature | Reason |
|---------|--------|
| Nanobot installation management | Auto-updater manages updates, not installation; nanobot installed separately |
| Multi-user authentication | Single-admin tool, Bearer token sufficient |
| Instance resource monitoring (CPU/RAM) | Out of scope for auto-updater; use system tools |
| Dark theme / UI theming | Not requested, standard light theme sufficient |
| Mobile responsive design | Desktop-first admin tool, not a public-facing app |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| IC-01 | Phase 50 | Pending |
| IC-02 | Phase 50 | Pending |
| IC-03 | Phase 50 | Pending |
| IC-04 | Phase 50 | Pending |
| IC-05 | Phase 50 | Pending |
| IC-06 | Phase 50 | Pending |
| LC-01 | Phase 51 | Pending |
| LC-02 | Phase 51 | Pending |
| LC-03 | Phase 51 | Pending |
| NC-01 | Phase 52 | Pending |
| NC-02 | Phase 52 | Pending |
| NC-03 | Phase 52 | Pending |
| NC-04 | Phase 52 | Pending |
| UI-01 | Phase 53 | Pending |
| UI-02 | Phase 53 | Pending |
| UI-03 | Phase 53 | Pending |
| UI-04 | Phase 53 | Pending |
| UI-05 | Phase 53 | Pending |
| UI-06 | Phase 53 | Pending |

**Coverage:**
- v0.12 requirements: 19 total
- Mapped to phases: 19
- Unmapped: 0

---
*Requirements defined: 2026-04-11*
*Last updated: 2026-04-11 after roadmap creation*

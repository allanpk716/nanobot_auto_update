# Roadmap: Nanobot Auto Updater

## Overview

Build a Windows background service in Go that automatically keeps the nanobot AI agent tool up-to-date. Start with infrastructure (logging, configuration, CLI), implement the core update logic with rollback support, add scheduled execution with failure notifications, then polish for production use with hidden console window and documentation.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Infrastructure** - Logging, configuration, CLI, and uv command executor (completed 2026-02-18)
- [x] **Phase 01.1: Nanobot lifecycle management** - Stop before update, start after update (INSERTED) (completed 2026-02-18)
- [ ] **Phase 2: Core Update Logic** - Dependency checking, update execution, and rollback mechanism
- [ ] **Phase 3: Scheduling and Notifications** - Cron-based scheduling and Pushover failure alerts
- [ ] **Phase 4: Runtime Integration** - Windows background execution and final integration

## Phase Details

### Phase 1: Infrastructure
**Goal**: Application foundation with logging, configuration, CLI, and safe subprocess execution
**Depends on**: Nothing (first phase)
**Requirements**: INFR-01, INFR-02, INFR-03, INFR-04, INFR-05, INFR-06, INFR-07, INFR-08, INFR-09, INFR-10
**Success Criteria** (what must be TRUE):
  1. User can run the program with `-help` flag and see usage information
  2. User can run the program with `-version` flag and see version information
  3. Logs are written to ./logs/ directory with the format "2024-01-01 12:00:00.123 - [INFO]: message"
  4. Configuration is loaded from ./config.yaml with cron field defaulting to "0 3 * * *"
  5. User can override config file path via `-config` flag and cron expression via `-cron` flag
  6. User can run a one-time update via `-run-once` flag
  7. Executed uv commands do not flash a command prompt window
**Plans:** 3 plans in 2 waves

Plans:
- [x] 01-01-PLAN.md - Logging module with custom format and rotation (INFR-01, INFR-02)
- [x] 01-02-PLAN.md - Config enhancement with cron field and viper integration (INFR-03, INFR-04)
- [x] 01-03-PLAN.md - CLI entry point with pflag and integration (INFR-05, INFR-06, INFR-07, INFR-08, INFR-09, INFR-10)

### Phase 01.1: Nanobot lifecycle management - stop before update, start after update (INSERTED)

**Goal:** Stop nanobot before update, restart after update with graceful shutdown and hidden startup
**Depends on:** Phase 1
**Requirements:** IMPL-01, IMPL-02, IMPL-03, IMPL-04
**Success Criteria** (what must be TRUE):
  1. Nanobot running status can be detected by checking if port 18790 is listening
  2. Stop command gracefully terminates nanobot with 5-second timeout, force-killing if needed
  3. Start command launches "nanobot gateway" with hidden window (no console flash)
  4. Startup is verified by checking port becomes available within configurable timeout
  5. Stop failure cancels the update; start failure logs warning but does not fail update
**Plans:** 2/3 plans executed

Plans:
- [x] 01.1-01-PLAN.md - Config and process detection (NanobotConfig, FindPIDByPort)
- [x] 01.1-02-PLAN.md - Stopper and starter (graceful stop, hidden start, port verification)
- [ ] 01.1-03-PLAN.md - Lifecycle manager and integration (orchestrator, config validation)

### Phase 2: Core Update Logic
**Goal**: Nanobot can be updated from GitHub main branch with automatic fallback to stable version
**Depends on**: Phase 1
**Requirements**: UPDT-01, UPDT-02, UPDT-03, UPDT-04, UPDT-05
**Success Criteria** (what must be TRUE):
  1. Program exits with clear error message if uv is not installed on the system
  2. User can trigger an update that installs nanobot from GitHub main branch using uv
  3. If GitHub update fails, program automatically falls back to installing nanobot-ai stable version from PyPI
  4. All update attempts are logged with detailed success/failure information
  5. Update result (success/fallback/failure) is visible in logs
**Plans**: TBD

### Phase 3: Scheduling and Notifications
**Goal**: Updates run automatically on schedule and user is notified of failures
**Depends on**: Phase 2
**Requirements**: SCHD-01, SCHD-02, SCHD-03, NOTF-01, NOTF-02, NOTF-03, NOTF-04
**Success Criteria** (what must be TRUE):
  1. Program executes updates automatically based on cron expression from configuration
  2. Default schedule runs daily at 3 AM ("0 3 * * *")
  3. Overlapping update jobs are skipped if previous job is still running
  4. User receives Pushover notification when update fails, including the failure reason
  5. Program runs without Pushover configuration (logs warning instead of failing)
**Plans**: TBD

### Phase 4: Runtime Integration
**Goal**: Program runs as a Windows background service without visible console window
**Depends on**: Phase 3
**Requirements**: RUN-01, RUN-02
**Success Criteria** (what must be TRUE):
  1. Program runs on Windows without displaying a console window
  2. User can start the program manually (not auto-started on boot)
  3. All features from previous phases work correctly in background mode
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 -> 01.1 -> 2 -> 3 -> 4

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Infrastructure | 3/3 | Complete | 2026-02-18 |
| 01.1. Nanobot Lifecycle | 3/3 | Complete    | 2026-02-18 |
| 2. Core Update Logic | 0/TBD | Not started | - |
| 3. Scheduling and Notifications | 0/TBD | Not started | - |
| 4. Runtime Integration | 0/TBD | Not started | - |

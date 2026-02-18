# Requirements: Nanobot Auto Updater

**Defined:** 2025-02-18
**Core Value:** Automatically keep nanobot at the latest version without user manual intervention

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Infrastructure

- [x] **INFR-01**: Program supports custom log format output (2024-01-01 12:00:00.123 - [INFO]: message)
- [x] **INFR-02**: Logs stored in ./logs/ directory with 24-hour rotation, keeping 7 days
- [x] **INFR-03**: Load configuration from ./config.yaml
- [x] **INFR-04**: Configuration file supports cron field (default "0 3 * * *")
- [x] **INFR-05**: Support -config flag to specify config file path
- [x] **INFR-06**: Support -cron flag to override cron expression in config
- [x] **INFR-07**: Support -run-once flag to execute one update and exit
- [x] **INFR-08**: Support -version flag to display version info
- [x] **INFR-09**: Support help flag to display usage information
- [x] **INFR-10**: Hide command window when executing uv commands (use SysProcAttr.HideWindow)

### Nanobot Lifecycle

- [x] **IMPL-01**: Detect nanobot running by port 18790
- [x] **IMPL-02**: Gracefully stop nanobot with 5-second timeout, force-kill if needed
- [x] **IMPL-03**: Start nanobot gateway with hidden window, verify port becomes available
- [x] **IMPL-04**: Stop failure cancels update; start failure logs warning only

### Core Update

- [x] **UPDT-01**: Check if uv is installed on startup
- [x] **UPDT-02**: Log error and exit if uv is not installed
- [x] **UPDT-03**: Install nanobot from GitHub main branch using uv
- [x] **UPDT-04**: Fallback to uv tool install nanobot-ai stable version if update fails
- [x] **UPDT-05**: Log detailed update process information

### Scheduling

- [ ] **SCHD-01**: Support cron expression scheduled update triggering
- [ ] **SCHD-02**: Default cron is "0 3 * * *" (daily at 3 AM)
- [ ] **SCHD-03**: Prevent job overlap execution (SkipIfStillRunning mode)

### Notifications

- [x] **NOTF-01**: Read Pushover config from environment variables (PUSHOVER_TOKEN, PUSHOVER_USER)
- [x] **NOTF-02**: Send notification via Pushover when update fails
- [x] **NOTF-03**: Notification includes failure reason
- [x] **NOTF-04**: Log warning only if Pushover config missing, don't block program

### Runtime

- [ ] **RUN-01**: Support Windows background execution, hide console window
- [ ] **RUN-02**: Program starts manually, not auto-start on boot

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Advanced Features

- **ADVT-01**: Maintenance window - only update during specified hours
- **ADVT-02**: Version pinning - update to specific version
- **ADVT-03**: Health check endpoint - HTTP health check interface
- **ADVT-04**: Update verification - checksum/signature verification

### Retry Mechanism

- **RETRY-01**: Failure retry mechanism - exponential backoff retry
- **RETRY-02**: Max retry count configuration

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| GUI interface | CLI tool, no GUI needed |
| Update history | Keep it simple, no history storage |
| Auto-start on boot | Manual start by user |
| Cross-platform | Windows only |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| INFR-01 | Phase 1 | Complete |
| INFR-02 | Phase 1 | Complete |
| INFR-03 | Phase 1 | Complete |
| INFR-04 | Phase 1 | Complete |
| INFR-05 | Phase 1 | Complete |
| INFR-06 | Phase 1 | Complete |
| INFR-07 | Phase 1 | Complete |
| INFR-08 | Phase 1 | Complete |
| INFR-09 | Phase 1 | Complete |
| INFR-10 | Phase 1 | Complete |
| IMPL-01 | Phase 01.1 | Complete |
| IMPL-02 | Phase 01.1 | Complete |
| IMPL-03 | Phase 01.1 | Complete |
| IMPL-04 | Phase 01.1 | Complete |
| UPDT-01 | Phase 2 | Complete |
| UPDT-02 | Phase 2 | Complete |
| UPDT-03 | Phase 2 | Complete |
| UPDT-04 | Phase 2 | Complete |
| UPDT-05 | Phase 2 | Complete |
| SCHD-01 | Phase 3 | Complete |
| SCHD-02 | Phase 3 | Pending |
| SCHD-03 | Phase 3 | Complete |
| NOTF-01 | Phase 3 | Complete |
| NOTF-02 | Phase 3 | Complete |
| NOTF-03 | Phase 3 | Complete |
| NOTF-04 | Phase 3 | Complete |
| RUN-01 | Phase 4 | Pending |
| RUN-02 | Phase 4 | Pending |

**Coverage:**
- v1 requirements: 28 total
- Mapped to phases: 28
- Complete: 13
- Unmapped: 0

---
*Requirements defined: 2025-02-18*
*Last updated: 2026-02-18 after plan 03-02 completion*

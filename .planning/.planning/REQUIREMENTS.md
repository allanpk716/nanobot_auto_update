# Requirements: Nanobot Auto Updater v0.2

**Defined:** 2026-03-09
**Core Value:** 自动保持 nanobot 处于最新版本，无需用户手动干预

## v0.2 Requirements

Requirements for multi-instance support. Each maps to roadmap phases.

### Instance Configuration

- [ ] **CONF-01**: User can define multiple nanobot instances in YAML configuration
- [ ] **CONF-02**: Each instance configuration includes name, port, and startup command
- [ ] **CONF-03**: System validates instance names are unique at startup
- [ ] **CONF-04**: System fails fast with clear error message when configuration validation fails

### Lifecycle Management

- [ ] **LIFECYCLE-01**: System stops all configured instances sequentially before update
- [ ] **LIFECYCLE-02**: System updates nanobot tool to latest version (GitHub main with PyPI fallback)
- [ ] **LIFECYCLE-03**: System starts all configured instances sequentially after update
- [ ] **LIFECYCLE-04**: Instance stop failure does not prevent stopping other instances
- [ ] **LIFECYCLE-05**: Instance start failure does not prevent starting other instances

### Error Handling and Notification

- [ ] **ERROR-01**: System collects startup failures from all instances
- [ ] **ERROR-02**: System sends aggregated failure notification via Pushover
- [ ] **ERROR-03**: Notification message includes all failed instance names and error reasons
- [ ] **ERROR-04**: System tracks status of each instance (running/stopped/failed)
- [ ] **ERROR-05**: All log messages include instance identifier for debugging

## v0.1 Requirements (Completed)

Already implemented in v1.0 milestone.

### Logging and Configuration (v0.1)

- ✓ Detect system UV package manager installation
- ✓ Execute update tasks according to cron expression
- ✓ Install nanobot from GitHub latest code using UV
- ✓ Fallback to UV tool install nanobot-ai stable version on update failure
- ✓ Notify user via Pushover on update failure
- ✓ Support configuration file (YAML) for runtime parameters
- ✓ Support command line parameter override
- ✓ Run in background with hidden console window
- ✓ Log to file with log rotation

## v0.3 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Single Instance Control

- **CTRL-01**: User can control specific instance by name via CLI flags
- **CTRL-02**: User can stop single instance without affecting others
- **CTRL-03**: User can start single instance without others

### Advanced Features

- **ADV-01**: Configurable retry strategy for failed instances
- **ADV-02**: Parallel instance startup for faster recovery
- **ADV-03**: Instance grouping for partial updates

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| GUI interface | CLI tool, no graphical interface needed |
| Update history | Keep simple, no history storage |
| Auto-start on boot | User manually starts |
| Cross-platform support | Windows only |
| Single instance control (v0.2) | Complexity, defer to v0.3 |
| Parallel startup (v0.2) | 2-3 instances, sequential is sufficient |
| Instance grouping (v0.2) | Not needed for current use case |
| Retry strategies (v0.2) | Adds complexity without clear benefit |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| CONF-01 | Phase 5 | Pending |
| CONF-02 | Phase 5 | Pending |
| CONF-03 | Phase 5 | Pending |
| CONF-04 | Phase 5 | Pending |
| LIFECYCLE-01 | Phase 6 | Pending |
| LIFECYCLE-02 | Phase 6 | Pending |
| LIFECYCLE-03 | Phase 6 | Pending |
| LIFECYCLE-04 | Phase 6 | Pending |
| LIFECYCLE-05 | Phase 6 | Pending |
| ERROR-01 | Phase 7 | Pending |
| ERROR-02 | Phase 7 | Pending |
| ERROR-03 | Phase 7 | Pending |
| ERROR-04 | Phase 7 | Pending |
| ERROR-05 | Phase 7 | Pending |

**Coverage:**
- v0.2 requirements: 14 total
- Mapped to phases: 14
- Unmapped: 0 ✓

---
*Requirements defined: 2026-03-09*
*Last updated: 2026-03-09 after v0.2 milestone start*

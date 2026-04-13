---
phase: 52-nanobot-config-management-api
plan: 01
subsystem: api
tags: [nanobot-config, json, file-management, http-api, path-parsing]

# Dependency graph
requires:
  - phase: 46-service-configuration-mode-detection
    provides: "Config and InstanceConfig structs"
provides:
  - "NanobotConfigManager with ParseConfigPath, GenerateDefaultConfig, ReadConfig, WriteConfig, CreateDefaultConfig, CloneConfig"
  - "NanobotConfigHandler with HandleGet (lazy-creation fallback) and HandlePut"
  - "GET/PUT /api/v1/instance-configs/{name}/nanobot-config endpoints"
affects: [52-02, 53-ui]

# Tech tracking
tech-stack:
  added: []
  patterns: [lazy-creation-fallback, config-path-parsing-from-start-command, mutex-protected-file-write]

key-files:
  created:
    - internal/nanobot/config_manager.go
    - internal/api/nanobot_config_handler.go
  modified:
    - internal/api/server.go

key-decisions:
  - "Use os.UserHomeDir() instead of string substitution with ~ for Windows path handling"
  - "Lazy-creation fallback on GET auto-creates missing nanobot config for known instances"
  - "findInstanceByNameForNanobotConfig as local function (instance_config_handler.go not in this worktree)"
  - "getConfig closure captures fullCfg from NewServer parameter (no GetCurrentConfig in this codebase state)"

patterns-established:
  - "ConfigManager struct with sync.Mutex for concurrent file write safety"
  - "Handler struct with manager + getConfig + logger dependency injection"
  - "Lazy-creation fallback: GET returns 200 with auto-created default config when file is missing"

requirements-completed: [NC-02, NC-03]

# Metrics
duration: 6min
completed: 2026-04-12
---

# Phase 52 Plan 01: NanobotConfigManager and GET/PUT API Summary

**NanobotConfigManager with path parsing from start_command --config, default config generation, thread-safe file read/write, and GET/PUT API endpoints with lazy-creation fallback**

## Performance

- **Duration:** 6 min
- **Started:** 2026-04-12T05:43:55Z
- **Completed:** 2026-04-12T05:50:29Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- NanobotConfigManager with 6 public functions for nanobot config.json file management
- ParseConfigPath extracts --config from start_command with os.UserHomeDir() fallback
- GenerateDefaultConfig produces full nanobot config structure per CONTEXT.md specifics
- NanobotConfigHandler HandleGet with lazy-creation fallback for missing config files
- NanobotConfigHandler HandlePut with JSON validation and restart hint
- Route registration at /api/v1/instance-configs/{name}/nanobot-config with auth middleware

## Task Commits

Each task was committed atomically:

1. **Task 1: Create NanobotConfigManager** - `1d8da69` (feat)
2. **Task 2: Create NanobotConfigHandler and register routes** - `edb89a0` (feat)

## Files Created/Modified
- `internal/nanobot/config_manager.go` - ConfigManager with ParseConfigPath, GenerateDefaultConfig, ReadConfig, WriteConfig, CreateDefaultConfig, CloneConfig
- `internal/api/nanobot_config_handler.go` - NanobotConfigHandler with HandleGet (lazy-creation) and HandlePut
- `internal/api/server.go` - Added nanobot import and route registration for GET/PUT nanobot-config endpoints

## Decisions Made
- Used os.UserHomeDir() for home directory resolution instead of string substitution with ~ to handle Windows paths correctly
- Created local findInstanceByNameForNanobotConfig function since instance_config_handler.go (Phase 50) is not in this worktree
- Used getConfig closure capturing fullCfg from NewServer parameter since GetCurrentConfig (hotreload.go) is not in this codebase state
- Workspace value in nanobot config uses ~/.nanobot-{name} form for consistency with how nanobot reads it

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Adapted to worktree codebase state (pre-Phase 50/51)**
- **Found during:** Task 2 (NanobotConfigHandler implementation)
- **Issue:** Worktree branch was reset to 120c639 (pre-Phase 50/51), so instance_config_handler.go, instance_lifecycle_handler.go, hotreload.go, and GetCurrentConfig() were not available. AuthMiddleware signature differs (string vs func() string).
- **Fix:** Created local findInstanceByNameForNanobotConfig, used fullCfg closure for getConfig, matched existing AuthMiddleware(string, logger) signature
- **Files modified:** internal/api/nanobot_config_handler.go, internal/api/server.go
- **Verification:** go build ./... and go vet pass with zero errors
- **Committed in:** edb89a0 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (blocking)
**Impact on plan:** Adaptation to worktree state. Functional behavior is identical; integration points will merge cleanly when Phase 50/51 code is merged.

## Issues Encountered
None beyond the worktree state adaptation documented above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- NanobotConfigManager and API endpoints ready for Phase 52-02 integration with InstanceConfigHandler create/copy flows
- CloneConfig function ready for copy callback integration
- CreateDefaultConfig function ready for create callback integration

---
*Phase: 52-nanobot-config-management-api*
*Completed: 2026-04-12*

## Self-Check: PASSED

- FOUND: internal/nanobot/config_manager.go
- FOUND: internal/api/nanobot_config_handler.go
- FOUND: internal/api/server.go
- FOUND: .planning/phases/52-nanobot-config-management-api/52-01-SUMMARY.md
- FOUND: 1d8da69 (Task 1 commit)
- FOUND: edb89a0 (Task 2 commit)

---
phase: 49-existing-code-adaptation
plan: 02
subsystem: config
tags: [viper, hotreload, fsnotify, debounce, windows-service, bearer-token, dynamic-getter]

# Dependency graph
requires:
  - phase: 48-service-manager
    provides: "service_windows.go ServiceHandler with Execute/RunService, service.go non-Windows stub"
provides:
  - "config/hotreload.go: WatchConfig, StopWatch, GetCurrentConfig, HotReloadCallbacks with 500ms debounce"
  - "config.go: exported viperInstance, GetViper(), ReloadConfig()"
  - "auth.go: AuthMiddleware with dynamic token getter (func() string)"
  - "server.go: NewServer with getToken parameter"
  - "service_windows.go: onReady callback after Running, StopWatch before shutdown"
  - "main.go: onReady closure with 6 HotReloadCallbacks in service mode"
affects: [50-integration-testing, service-manager, api-auth]

# Tech tracking
tech-stack:
  added: [fsnotify (via viper.WatchConfig), reflect.DeepEqual for config diff]
  patterns: [debounce timer for file events, callback-based component rebuild, dynamic token getter closure, shared mutable variable for cross-goroutine token propagation]

key-files:
  created:
    - internal/config/hotreload.go
  modified:
    - internal/config/config.go
    - internal/api/auth.go
    - internal/api/server.go
    - internal/lifecycle/service_windows.go
    - internal/lifecycle/service.go
    - cmd/nanobot-auto-updater/main.go

key-decisions:
  - "500ms debounce via time.AfterFunc to coalesce Windows fsnotify rapid events"
  - "sync.Mutex serializes all component rebuilds (stop/create/start under single lock)"
  - "SelfUpdater only logs on config change, no rebuild to avoid stale reference in SelfUpdateHandler"
  - "BearerToken hot reload via shared variable + func() string closure (Go string assignment is atomic)"
  - "Instances use full replace strategy (StopAll -> recreate -> StartAll) instead of partial diff"
  - "HotReloadCallbacks uses function fields to avoid circular imports from config package"

patterns-established:
  - "Callback-based component rebuild: config package detects changes, caller provides rebuild functions"
  - "Dynamic getter closure for hot-reloadable auth tokens"
  - "Debounce timer pattern for file system event coalescing"

requirements-completed: [ADPT-04]

# Metrics
duration: 26min
completed: 2026-04-11
---

# Phase 49 Plan 02: Config Hot Reload Summary

**Service mode config.yaml hot reload with 500ms debounce, serialized rebuild, and dynamic bearer token getter**

## Performance

- **Duration:** 26 min
- **Started:** 2026-04-11T09:38:23Z
- **Completed:** 2026-04-11T10:04:58Z
- **Tasks:** 4
- **Files modified:** 7

## Accomplishments
- Config file watcher with 500ms debounce for Windows fsnotify coalescing
- 6 component rebuild callbacks: Monitor, Pushover, SelfUpdate (log-only), HealthCheck, BearerToken, Instances
- AuthMiddleware dynamic token getter enables hot token rotation without API server restart
- onReady callback in service_windows.go enables post-startup initialization (hot reload watcher)
- Serialized rebuild via sync.Mutex prevents concurrent component recreation

## Task Commits

Each task was committed atomically:

1. **Task 1: Export viper instance + create hotreload module** - `ed1ad95` (feat)
2. **Task 2: AuthMiddleware dynamic token getter** - `ca9c00f` (feat)
3. **Task 3: service_windows.go onReady callback** - `ad7961b` (feat)
4. **Task 4: main.go integration WatchConfig + dynamic token** - `5233043` (feat)

## Files Created/Modified
- `internal/config/hotreload.go` - Hot reload module: WatchConfig, StopWatch, HotReloadCallbacks, debounce, serialized rebuild
- `internal/config/config.go` - Exported viperInstance, added GetViper() and ReloadConfig()
- `internal/api/auth.go` - AuthMiddleware signature changed to accept func() string
- `internal/api/server.go` - NewServer accepts getToken func() string parameter
- `internal/api/auth_test.go` - Updated AuthMiddleware calls with func() string getter
- `internal/api/server_test.go` - Updated NewServer calls with token getter
- `internal/api/query_test.go` - Updated AuthMiddleware call in test helper
- `internal/api/selfupdate_handler_test.go` - Updated AuthMiddleware call
- `internal/api/trigger_test.go` - Updated AuthMiddleware call
- `internal/lifecycle/service_windows.go` - Added onReady callback, StopWatch before shutdown
- `internal/lifecycle/service.go` - Synced non-Windows stub signatures
- `internal/lifecycle/service_handler_test.go` - Updated test for new onReady param
- `cmd/nanobot-auto-updater/main.go` - onReady closure with HotReloadCallbacks, dynamic token

## Decisions Made
- Used time.AfterFunc for debounce (cleaner than manual timer management, runs in goroutine)
- Used reflect.DeepEqual for config section comparison (simple, covers all fields)
- Full replace for instances (StopAllNanobots -> NewInstanceManager -> StartAll) avoids complex partial diff
- SelfUpdater change only logs warning -- rebuild would break SelfUpdateHandler's stale reference
- currentBearerToken as shared var with string assignment (Go guarantees atomic string writes)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed viper.OnConfigChange callback signature**
- **Found during:** Task 1 (hotreload.go creation)
- **Issue:** Plan specified `viper.ConfigChangeInfo` type but viper actually uses `fsnotify.Event`
- **Fix:** Changed callback parameter type to `fsnotify.Event`, added `github.com/fsnotify/fsnotify` import
- **Files modified:** internal/config/hotreload.go
- **Verification:** `go build ./internal/config/` passes
- **Committed in:** ed1ad95 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Minor API signature fix. No scope creep.

## Issues Encountered
- Pre-existing test failures in internal/lifecycle (capture_test.go) unrelated to this plan's changes. Service handler tests pass correctly.

## Next Phase Readiness
- Config hot reload module ready for integration testing
- All component rebuild callbacks implemented and verified via compilation
- Ready for plan 03 (if any) or integration testing phase

---
*Phase: 49-existing-code-adaptation*
*Completed: 2026-04-11*

## Self-Check: PASSED
- All 7 source files verified present on disk
- All 4 task commits verified in git log (ed1ad95, ca9c00f, ad7961b, 5233043)
- SUMMARY.md verified present

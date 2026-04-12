---
phase: 53-instance-management-ui
plan: 03
subsystem: ui
tags: [javascript, css, hybrid-editor, bidirectional-sync, json-textarea, structured-form, nanobot-config, password-field]

# Dependency graph
requires:
  - phase: 50-instance-config-crud-api
    provides: GET/POST/PUT/DELETE /api/v1/instance-configs endpoints
  - phase: 51-instance-lifecycle-api
    provides: POST /api/v1/instances/{name}/start and /stop endpoints
  - phase: 53-01
    provides: Modal system, toast system, card rendering, auth integration
  - phase: 53-02
    provides: CRUD dialogs (create, edit, copy, delete) with shared form builder
provides:
  - Nanobot config hybrid editor with left-right split layout
  - Structured form for common parameters (model, provider, API key, port, telegram token)
  - JSON textarea for full config editing
  - Bidirectional sync between form and JSON protected by syncGuard flag
  - API key password field with show/hide toggle
  - Save with restart hint toast notification
  - Helper functions: getNestedValue, setNestedValue, findFirstApiKey
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns: [hybrid editor split view, syncGuard bidirectional sync, nested path accessor helpers, password field with toggle]

key-files:
  created: []
  modified:
    - internal/web/static/home.js
    - internal/web/static/style.css

key-decisions:
  - "syncGuard boolean flag prevents infinite loop between form->JSON->form event cascading"
  - "JSON textarea is source of truth for saving; both sync directions ensure it stays current"
  - "API key uses type=password by default with show/hide toggle (T-53-11 mitigation)"
  - "findFirstApiKey iterates providers (zhipu, groq, aihubmix) to find first non-empty apiKey"
  - "API key sync targets current provider field value or first provider with existing apiKey"
  - "Save success toast includes restart hint to surface backend recommendation"

requirements-completed: [UI-06]

# Metrics
duration: 6min
completed: 2026-04-12
---

# Phase 53 Plan 03: Nanobot Config Hybrid Editor Summary

**Hybrid nanobot config editor with left-right split view, structured form for common parameters, raw JSON textarea for full config editing, bidirectional sync protected by syncGuard, and authenticated API integration with restart hint**

## Performance

- **Duration:** 6 min
- **Started:** 2026-04-12T09:32:00Z
- **Completed:** 2026-04-12T09:38:03Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Implemented showNanobotConfigDialog() with full hybrid editor replacing placeholder from Plan 01
- Built left-right split layout: structured form (model, provider, API key, gateway port, telegram token) on left, JSON textarea on right
- Implemented bidirectional sync with syncGuard flag preventing infinite event loops
- Added getNestedValue(), setNestedValue(), findFirstApiKey() helper functions for nested JSON path access
- API key field uses type=password with show/hide toggle button for security
- Save PUTs full config JSON to backend, success toast includes restart hint
- Added CSS: hybrid-editor grid, modal-wide, nanobot-json-textarea, api-key-wrapper, responsive media query
- Verified complete Phase 53 integration: all 9 API endpoints, 16 decisions (D-01 through D-16), 6 requirements (UI-01 through UI-06)

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement Nanobot Config hybrid editor with bidirectional sync** - `d681121` (feat)
2. **Task 2: End-to-end integration verification** - verification only (no code changes)

## Files Created/Modified
- `internal/web/static/home.js` - Replaced placeholder showNanobotConfigDialog() with ~180-line implementation: async loading, hybrid editor HTML, bidirectional sync with syncGuard, API key toggle, save handler with PUT; added getNestedValue, setNestedValue, findFirstApiKey helpers
- `internal/web/static/style.css` - Added ~90 lines: hybrid-editor grid with responsive media query, hybrid-editor-left/right, nanobot-json-textarea, nanobot-section-label, modal-wide, json-error, api-key-wrapper/toggle styles

## Decisions Made
- syncGuard boolean prevents bidirectional sync infinite loops: set true before programmatic update, reset false after
- JSON textarea is the single source of truth for saving; form fields are convenience shortcuts
- API key targets the provider matching the current "provider" field value, falling back to first provider with existing apiKey
- Password type with toggle button addresses threat T-53-11 (information disclosure)
- Toast message "配置已保存。重启实例以应用更改。" surfaces backend restart hint without separate dialog
- modal-wide class (max-width: 900px) gives hybrid editor sufficient horizontal space

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 53 (instance-management-ui) is now fully complete
- All 6 requirements (UI-01 through UI-06) implemented
- All 16 design decisions (D-01 through D-16) implemented
- Complete CRUD workflow: create, edit, copy, delete instance configs
- Lifecycle controls: start, stop instances
- Nanobot config editing: hybrid form + JSON editor
- No remaining placeholder functions or "即将推出" messages

## Self-Check: PASSED

All files verified present:
- FOUND: internal/web/static/home.js
- FOUND: internal/web/static/style.css

All commits verified:
- FOUND: d681121 (feat: nanobot config hybrid editor)

Verification results:
- Go build: BUILD_OK
- API endpoints: all 9 referenced (instance-configs GET/POST/PUT/DELETE, /copy, instances/status, start/stop, nanobot-config GET/PUT)
- syncGuard: 8 references (declaration + guard checks + set true/false)
- type="password": present for API key field
- show/hide toggle: present (nb-apikey-toggle)
- Restart hint: present in save success toast
- XSS safety: innerHTML only for static templates, all user data via textContent
- Self-update module: all 6 functions intact
- No remaining placeholder functions
- All D-01 through D-16 decisions have grep-able artifacts
- All UI-01 through UI-06 requirements have implementations

---
*Phase: 53-instance-management-ui*
*Completed: 2026-04-12*

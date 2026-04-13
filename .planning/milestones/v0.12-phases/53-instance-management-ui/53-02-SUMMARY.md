---
phase: 53-instance-management-ui
plan: 02
subsystem: ui
tags: [javascript, css, dialog, form-validation, crud, copy, delete, bearer-auth]

# Dependency graph
requires:
  - phase: 50-instance-config-crud-api
    provides: GET/POST/PUT/DELETE /api/v1/instance-configs endpoints
  - phase: 51-instance-lifecycle-api
    provides: POST /api/v1/instances/{name}/start and /stop endpoints
  - phase: 53-01
    provides: Modal system, toast system, card rendering, auth integration
provides:
  - Create instance dialog with two-column form, toggle switch, and inline validation
  - Edit instance dialog with pre-filled form, readonly name field, and PUT API call
  - Copy instance dialog with source config pre-fill, port=sourcePort+1 suggestion
  - Delete instance confirmation dialog with running warning and destructive action styling
  - Shared form builder (buildInstanceFormHtml) and validation (validateInstanceForm)
affects: [53-03]

# Tech tracking
tech-stack:
  added: []
  patterns: [shared form builder pattern, toggle switch component, readonly-vs-disabled distinction, destructive action confirmation pattern]

key-files:
  created: []
  modified:
    - internal/web/static/home.js
    - internal/web/static/style.css

key-decisions:
  - "Shared buildInstanceFormHtml() builder function for create/edit/copy dialogs to avoid code duplication"
  - "readonly attribute (not disabled) on edit dialog name field to ensure value included in PUT body"
  - "sourcePort + 1 simple increment for copy dialog (no frontend availability check -- server validates conflicts)"
  - "textContent for all user-provided data in delete confirmation (XSS prevention per threat model T-53-07)"
  - "Auto-fetch instance config in edit/copy/delete dialogs to show accurate details"

requirements-completed: [UI-02, UI-03, UI-04, UI-05]

# Metrics
duration: 10min
completed: 2026-04-12
---

# Phase 53 Plan 02: Instance CRUD Dialogs Summary

**Four complete CRUD dialog systems (Create, Edit, Copy, Delete) with shared form builder, inline validation, Chinese UI text, toggle switch component, and authenticated API integration**

## Performance

- **Duration:** 10 min
- **Started:** 2026-04-12T09:18:21Z
- **Completed:** 2026-04-12T09:28:15Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Implemented showCreateDialog() with two-column form grid, all 5 config fields, and auto_start toggle switch
- Implemented showEditDialog(name) with pre-fetched config, readonly name field, and PUT API integration
- Implemented showCopyDialog(name) with source config display, port=sourcePort+1 suggestion, and POST to /copy endpoint
- Implemented showDeleteDialog(name, isRunning) with confirmation dialog, running instance warning, and DELETE API call
- Built shared form infrastructure: buildInstanceFormHtml(), validateInstanceForm(), displayServerFieldErrors()
- Added comprehensive CSS: form-grid layout, toggle-switch component, btn-form-primary/cancel/danger, delete-warning, source-info-box

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement Create and Edit instance dialogs** - `94777cc` (feat)
2. **Task 2: Implement Copy and Delete instance dialogs** - `b1b62ec` (feat)

## Files Created/Modified
- `internal/web/static/home.js` - Added 620+ lines: showCreateDialog, showEditDialog, showCopyDialog, showDeleteDialog, buildInstanceFormHtml, validateInstanceForm, displayServerFieldErrors, escapeAttr; replaced 4 placeholder functions
- `internal/web/static/style.css` - Added 150+ lines: form-grid, form-group, toggle-switch, btn-form-primary/cancel/danger, delete-warning, delete-info-box, source-info-box styles

## Decisions Made
- Shared buildInstanceFormHtml() builder avoids 3x duplicated form HTML construction across create/edit/copy dialogs
- HTML readonly attribute (not disabled) on edit name field -- disabled fields are excluded from form data, readonly fields are included and visually indicate non-editability
- Copy dialog suggests sourcePort + 1 as convenience only; server-side validation handles port conflicts with auto-increment
- Delete dialog fetches instance config to show port/command details in confirmation; isRunning parameter from card render is acceptable even with 5s polling staleness since server auto-stops before deletion
- All dialog text in Chinese matching existing UI patterns from Plan 01

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- CRUD dialogs fully functional and ready for browser-based integration testing
- All four operations (create, edit, copy, delete) call correct backend API endpoints with Bearer auth
- Form validation handles both client-side and server-side (422) errors with Chinese messages
- showNanobotConfigDialog placeholder remains for Plan 03 implementation

## Self-Check: PASSED

All files verified present:
- FOUND: internal/web/static/home.js
- FOUND: internal/web/static/style.css

All commits verified:
- FOUND: 94777cc (feat: create and edit dialogs)
- FOUND: b1b62ec (feat: copy and delete dialogs)

---
*Phase: 53-instance-management-ui*
*Completed: 2026-04-12*

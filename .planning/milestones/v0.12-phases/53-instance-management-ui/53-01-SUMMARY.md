---
phase: 53-instance-management-ui
plan: 01
subsystem: ui
tags: [html, css, javascript, modal, toast, fetch-api, bearer-auth]

# Dependency graph
requires:
  - phase: 50-instance-config-crud-api
    provides: GET/POST/PUT/DELETE /api/v1/instance-configs endpoints
  - phase: 51-instance-lifecycle-api
    provides: POST /api/v1/instances/{name}/start and /stop endpoints
provides:
  - Card-based instance grid with full config details (name, port, command, timeout, auto_start)
  - Modal system (showModal/closeModal with Escape key, background click, X button)
  - Toast notification system (success/error with prepend newest-on-top, 3s auto-dismiss)
  - Authenticated API integration with graceful degradation (status-only cards when auth fails)
  - Lifecycle action buttons (start/stop) with loading state and AbortController timeout
  - Placeholder functions for CRUD dialogs (Plans 02 and 03)
affects: [53-02, 53-03]

# Tech tracking
tech-stack:
  added: []
  patterns: [Promise.allSettled dual-API fetch, AbortController timeout, textContent XSS safety, toast prepend pattern]

key-files:
  created: []
  modified:
    - internal/web/static/home.html
    - internal/web/static/home.js
    - internal/web/static/style.css

key-decisions:
  - "Promise.allSettled over Promise.all for graceful auth degradation -- status-only cards when config API fails"
  - "AbortController with 65s start / 35s stop timeout (backend timeout + 5s margin)"
  - "Toast prepend via insertBefore for newest-on-top stacking order"
  - "textContent for all user data rendering (XSS mitigation per threat model T-53-03)"
  - "Placeholder functions (showCreateDialog, showEditDialog, etc.) return toast for Plans 02/03"

patterns-established:
  - "Dual-API fetch pattern: Promise.allSettled([status, configs]) with 3-state rendering (both/configs-only/status-only)"
  - "Modal system: showModal(title, bodyHtml, footerHtml) with overlay click, X button, Escape key close"
  - "Toast system: showToast(message, type) with prepend, 3s auto-dismiss, CSS fade-out animation"
  - "Lifecycle action: handleLifecycleAction(name, action, button) with loading state, AbortController, token refresh"

requirements-completed: [UI-01]

# Metrics
duration: 6min
completed: 2026-04-12
---

# Phase 53 Plan 01: Instance List Page Redesign Summary

**Card-based instance management UI with full-config display, modal/toast components, dual-API Promise.allSettled fetch, and authenticated lifecycle controls (AbortController timeouts)**

## Performance

- **Duration:** 6 min
- **Started:** 2026-04-12T09:05:34Z
- **Completed:** 2026-04-12T09:12:31Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Transformed minimal instance list (name + port + restart button) into full management dashboard with config details, status indicators, and layered action buttons
- Built reusable modal system with three close methods (X button, background click, Escape key)
- Implemented toast notification system with newest-on-top prepend and 3-second auto-dismiss
- Integrated dual API sources (instance-configs + instances/status) via Promise.allSettled for graceful auth degradation
- Added lifecycle start/stop buttons with loading state and AbortController timeout (65s/35s)

## Task Commits

Each task was committed atomically:

1. **Task 1: Redesign instance cards, API integration, and page structure** - `259dc10` (feat)
2. **Task 2: Automated verification of redesigned instance list** - verification only (no code changes)

## Files Created/Modified
- `internal/web/static/home.html` - Added btn-new-instance button, modal-container div, toast-container div
- `internal/web/static/home.js` - Rewrote loadInstances (Promise.allSettled), createInstanceCard (full config), added handleLifecycleAction, showToast, showModal/closeModal, Escape handler; preserved self-update module
- `internal/web/static/style.css` - Added 200+ lines of new styles: modal overlay/dialog, toast animations, status dots, card action buttons, auto-start tags, command text truncation

## Decisions Made
- Used Promise.allSettled (not Promise.all) to gracefully handle auth failure -- shows status-only cards without config details
- AbortController timeouts: 65s for start (backend 60s + margin), 35s for stop (backend 30s + margin)
- Toast prepend via insertBefore ensures newest notification appears on top
- All user data rendered via textContent (not innerHTML) for XSS prevention
- Placeholder functions for CRUD/config dialogs show "即将推出" toast -- will be implemented in Plans 02 and 03

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- UI foundation ready for Plan 02: instance CRUD dialogs (create, edit, copy, delete) can build on showModal() and showToast()
- UI foundation ready for Plan 03: nanobot config editor can build on showModal() with larger dialog
- All backend API endpoints verified accessible from frontend code
- Self-update module completely preserved and functional

## Self-Check: PASSED

All files verified present:
- FOUND: internal/web/static/home.html
- FOUND: internal/web/static/home.js
- FOUND: internal/web/static/style.css
- FOUND: .planning/phases/53-instance-management-ui/53-01-SUMMARY.md

All commits verified:
- FOUND: 259dc10 (feat: UI foundation overhaul)

---
*Phase: 53-instance-management-ui*
*Completed: 2026-04-12*

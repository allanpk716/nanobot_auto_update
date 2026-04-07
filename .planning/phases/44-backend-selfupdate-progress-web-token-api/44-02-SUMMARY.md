---
phase: 44-backend-selfupdate-progress-web-token-api
plan: 02
subsystem: api
tags: [web-config, localhost-only, auth-token, bearer-token, TDD]

# Dependency graph
requires:
  - phase: 39-selfupdate-http-api
    provides: AuthMiddleware + writeJSONError + route registration pattern
  - phase: 44-01
    provides: SelfUpdateCheckResponse with progress field
provides:
  - GET /api/v1/web-config localhost-only endpoint returning auth_token
  - WebConfigHandler returning BearerToken from config
  - localhostOnly wrapper for loopback address validation
affects: [phase-45-frontend-selfupdate-ui]

# Tech tracking
tech-stack:
  added: []
  patterns: [localhostOnly middleware wrapper, net.SplitHostPort RemoteAddr parsing]

key-files:
  created:
    - internal/api/webconfig_handler.go
    - internal/api/webconfig_handler_test.go
  modified:
    - internal/api/server.go

key-decisions:
  - "localhostOnly wrapper function (not middleware) since only one endpoint needs it"
  - "Direct RemoteAddr check without X-Forwarded-For (no reverse proxy in project)"
  - "web-config endpoint skips authMiddleware to avoid chicken-and-egg problem"

requirements-completed: [API-02]

# Metrics
duration: 2min
completed: 2026-04-07
---

# Phase 44 Plan 02: Web UI Token API Summary

**GET /api/v1/web-config localhost-only endpoint returning auth_token for frontend auto-authentication**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-07T04:17:44Z
- **Completed:** 2026-04-07T04:20:16Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- WebConfigHandler returns auth_token (BearerToken from config) as JSON response
- localhostOnly wrapper restricts access to 127.0.0.1 and ::1 loopback addresses
- GET /api/v1/web-config route registered without authMiddleware (intentional)
- 7 comprehensive unit tests covering all localhost/remote/token scenarios

## Task Commits

Each task was committed atomically:

1. **Task 1: Create webconfig_handler.go + route registration in server.go** - `a3b709d` (feat)
2. **Task 2: Write comprehensive tests for webconfig handler** - `2a49890` (test)

## Files Created/Modified
- `internal/api/webconfig_handler.go` - New file: WebConfigResponse struct, NewWebConfigHandler, localhostOnly wrapper
- `internal/api/webconfig_handler_test.go` - New file: 7 tests (LocalhostToken, RemoteForbidden, IPv6Localhost, InvalidRemoteAddr, NoAuthRequired, EmptyToken, JSONContentType)
- `internal/api/server.go` - Added web-config route registration with localhostOnly wrapper (no authMiddleware)

## Decisions Made
- Used localhostOnly as a standalone wrapper function (not middleware) since only one endpoint needs this protection
- Directly checks r.RemoteAddr without trusting X-Forwarded-For headers (project has no reverse proxy)
- Skips authMiddleware for web-config to solve chicken-and-egg problem: frontend needs token before it can authenticate

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Pre-existing compilation errors in `internal/api/server_test.go` and `internal/api/sse_test.go` (NewInstanceManager argument count mismatch) block `go test ./internal/api/ -count=1`. These are out of scope - already logged in deferred-items.md. Webconfig handler tests verified by running specific test files.

## Next Phase Readiness
- GET /api/v1/web-config endpoint ready for frontend consumption
- Frontend (Phase 45) can fetch auth_token from localhost-only endpoint without prior authentication
- Combined with Phase 44-01 progress tracking, all backend APIs for self-update UI are complete

---
*Phase: 44-backend-selfupdate-progress-web-token-api*
*Completed: 2026-04-07*

## Self-Check: PASSED

All files verified present:
- FOUND: internal/api/webconfig_handler.go
- FOUND: internal/api/webconfig_handler_test.go
- FOUND: internal/api/server.go

All commits verified:
- FOUND: a3b709d feat(44-02): add web-config endpoint with localhost-only access
- FOUND: 2a49890 test(44-02): add comprehensive tests for webconfig handler

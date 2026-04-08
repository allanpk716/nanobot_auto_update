---
phase: 45-frontend-selfupdate-management-ui
fixed_at: 2026-04-08T00:00:00Z
review_path: .planning/phases/45-frontend-selfupdate-management-ui/45-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 3
skipped: 1
status: partial
---

# Phase 45: Code Review Fix Report

**Fixed at:** 2026-04-08
**Source review:** .planning/phases/45-frontend-selfupdate-management-ui/45-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 4 (1 Critical, 3 Warning)
- Fixed: 3
- Skipped: 1

## Fixed Issues

### CR-01: XSS via innerHTML with unsanitized instance.name

**Files modified:** `internal/web/static/home.js`
**Commit:** 0c974db
**Applied fix:** Replaced `innerHTML` template literal in `createInstanceCard` (lines 39-52) with safe DOM construction using `createElement` and `textContent`. Also added `encodeURIComponent` for the link href. This eliminates the stored XSS vector where a malicious instance name containing HTML/script payloads would be parsed as executable HTML. The fix follows the same safe pattern already used in the self-update module (lines 160-281).

### WR-02: download_percent value used without range validation

**Files modified:** `internal/web/static/home.js`
**Commit:** 29416f6
**Applied fix:** Added clamping logic `Math.max(0, Math.min(100, Number(progress.download_percent) || 0))` before using `download_percent` for display text and CSS width. Prevents "NaN%", negative, or >100% values from appearing in the UI.

### WR-03: Exception messages surfaced directly to user in error divs

**Files modified:** `internal/web/static/home.js`
**Commit:** 5db71ee
**Applied fix:** Replaced two instances of `'...' + e.message` error display with generic user-facing messages: "检测更新失败，请查看控制台获取详情" and "启动更新失败，请查看控制台获取详情". Detailed error information remains available via `console.error` calls that already exist in the catch blocks.

## Skipped Issues

### WR-01: Restart endpoint lacks authentication and CSRF protection

**File:** `internal/api/server.go:66`
**Reason:** Pre-existing backend code outside Phase 45 scope. The Phase 45 deliverable is the frontend self-update management UI (home.html, home.js, style.css). The restart endpoint in server.go was not introduced or modified in this phase. Adding auth middleware to this endpoint requires backend changes and client-side token handling that go beyond the frontend scope of this phase.
**Original issue:** The POST `/api/v1/instances/{name}/restart` endpoint has no authentication middleware and no CSRF protection, while the self-update endpoints correctly use `authMiddleware`.

---

_Fixed: 2026-04-08_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_

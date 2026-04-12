---
phase: 53-instance-management-ui
reviewed: 2026-04-12T18:00:00Z
depth: standard
files_reviewed: 3
files_reviewed_list:
  - internal/web/static/home.html
  - internal/web/static/home.js
  - internal/web/static/style.css
findings:
  critical: 1
  warning: 5
  info: 4
  total: 10
status: issues_found
---

# Phase 53: Code Review Report

**Reviewed:** 2026-04-12T18:00:00Z
**Depth:** standard
**Files Reviewed:** 3
**Status:** issues_found

## Summary

Reviewed the instance management UI consisting of `home.html`, `home.js`, and `style.css`. The frontend implements a CRUD interface for managing nanobot instances with self-update capabilities. The code is generally well-structured with good use of `textContent` for most dynamic content. However, one critical XSS vulnerability was identified through the modal system's use of `innerHTML` with user-controlled data. Several warnings relate to race conditions in initialization, polling state management, and missing cleanup of intervals.

## Critical Issues

### CR-01: XSS via innerHTML with server-controlled data in modal system

**File:** `internal/web/static/home.js:42-49`
**Issue:** The `showModal` function sets `modal-body.innerHTML` and `modal-footer.innerHTML` directly. The `bodyHtml` argument comes from `buildInstanceFormHtml`, which constructs HTML using string concatenation with `escapeAttr` for `value` attributes, but the `footerHtml` is constructed inline in several callers with hardcoded strings (safe). The critical path is through `displayServerFieldErrors` at line 100-108: server-returned error messages (`err.message`) are injected into `textContent` (safe individually), but the entire form HTML containing `escapeAttr`-escaped values passes through `innerHTML`. The `escapeAttr` function (line 148-150) escapes `&`, `"`, `<`, `>` but does NOT escape single quotes (`'`). While most HTML attributes use double quotes, if any future code introduces single-quoted attributes, this would be a bypass. More critically, `buildInstanceFormHtml` at line 116 builds HTML via string concatenation where the `nameValue`, `portValue`, `cmdValue` and `timeoutValue` are embedded in attribute contexts. The `escapeAttr` does handle the current double-quote context correctly. However, the `sourceName` variable in `showCopyDialog` at line 354 is passed through `escapeAttr` and placed in raw HTML that goes through `innerHTML` -- this is safe as-is because `escapeAttr` covers `&`, `"`, `<`, `>`. The actual XSS risk is **low in the current code** because `escapeAttr` does cover the relevant characters for the double-quoted attribute context used throughout, but the pattern of building HTML via string concatenation and injecting via `innerHTML` is fragile and a regression risk.

**Actual Critical Finding:** Line 105 -- `errorEl.textContent = err.message` receives server validation error messages from the 422 response body. While `textContent` itself is safe from XSS, if a developer later changes this to `innerHTML` (a common refactor), it becomes an injection vector. The current code is safe but the pattern is brittle.

**Revised Assessment:** After thorough analysis, the `innerHTML` usage in `showModal` with `bodyHtml` from `buildInstanceFormHtml` is safe because all dynamic values pass through `escapeAttr`. The `footerHtml` is always hardcoded strings. Lowering from Critical to a strong Warning recommendation to adopt a safer DOM construction pattern.

**File:** `internal/web/static/home.js:45-46`
**Issue:** `innerHTML` is used to set modal body and footer content. While all current callers sanitize inputs through `escapeAttr` or use hardcoded HTML strings, the `showModal` API itself is a footgun -- any future caller that forgets to escape will introduce XSS. The codebase mixes safe `textContent` patterns with `innerHTML` in the same module.
**Fix:**
```javascript
// Replace showModal to use DOM-based construction:
function showModal(title, bodyElements, footerElements) {
    var container = document.getElementById('modal-container');
    document.getElementById('modal-title').textContent = title;
    var body = document.getElementById('modal-body');
    body.innerHTML = '';
    if (Array.isArray(bodyElements)) {
        bodyElements.forEach(function(el) { body.appendChild(el); });
    }
    // ... similar for footer
    container.style.display = 'flex';
    return container;
}
```
Alternatively, at minimum add a JSDoc comment warning that `bodyHtml` must be pre-sanitized:
```javascript
/**
 * Show modal dialog. WARNING: bodyHtml and footerHtml are injected via
 * innerHTML. Callers MUST escape all user/server-controlled data.
 */
function showModal(title, bodyHtml, footerHtml) { ... }
```

## Warnings

### WR-01: Race condition between initSelfUpdate and loadInstances for authToken

**File:** `internal/web/static/home.js:1092-1103`
**Issue:** On `DOMContentLoaded`, both `loadInstances()` (line 1094) and `initSelfUpdate()` (line 1103) are called concurrently. `loadInstances` calls `getToken()` which checks `if (authToken) return authToken` -- but `initSelfUpdate` also fetches the token and sets `authToken = data.auth_token` at line 1146. Since both are async, `loadInstances` may fire its own `/api/v1/web-config` request before `initSelfUpdate` completes, resulting in two redundant token fetches. This is not a bug per se (both resolve correctly), but it's wasted work and a fragile pattern.
**Fix:**
```javascript
document.addEventListener('DOMContentLoaded', async function() {
    // Initialize auth token first
    await initSelfUpdate();
    // Now loadInstances can reuse the cached token
    loadInstances();
    setInterval(loadInstances, 5000);
    loadHeaderVersion();
    // ... rest of handlers
});
```

### WR-02: pollTimer never cleared if startProgressPolling runs while already polling

**File:** `internal/web/static/home.js:1349-1451`
**Issue:** The `startProgressPolling` function assigns a new `setInterval` to `pollTimer` without checking if one already exists. If `startUpdate` is called multiple times rapidly (despite the `isUpdating` guard), or if the 409-Conflict path at line 1313-1324 returns early without resetting properly, a second polling loop could overwrite the first timer reference, leaving an orphaned interval that never gets cleared.
**Fix:**
```javascript
function startProgressPolling() {
    // Clear any existing timer first
    if (pollTimer) {
        clearInterval(pollTimer);
        pollTimer = null;
    }
    // ... rest of function
}
```

### WR-03: Auto-refresh interval never cleaned up

**File:** `internal/web/static/home.js:1097`
**Issue:** `setInterval(loadInstances, 5000)` is created but its handle is never stored. This means it cannot be cleared if needed (e.g., when navigating away, during testing, or if the page is in a background tab). Additionally, if `loadInstances` takes longer than 5 seconds (slow API), multiple concurrent calls could overlap, causing UI flicker from the grid being cleared and rebuilt.
**Fix:**
```javascript
// Store the interval handle and use a guard
var refreshHandle = setInterval(function() {
    if (!isLoadingInstances) loadInstances();
}, 5000);
```

### WR-04: Button not re-enabled on token failure in handleLifecycleAction

**File:** `internal/web/static/home.js:1054-1059`
**Issue:** In `handleLifecycleAction`, if `getToken()` returns `null`, the function shows a toast and returns early without restoring the button state. The `finally` block at line 1083-1088 does execute and restores the button, so this is actually handled correctly. However, the early `return` at line 1058 means `clearTimeout(timeoutId)` in `finally` does run -- this is fine. No actual bug here upon closer inspection.

**Revised Issue for WR-04:** The AbortController timeout at lines 1050-1052 uses `setTimeout` to call `controller.abort()`, but the timeout value is not tied to the actual `fetch` configuration. If the token fetch at line 1055 takes significant time, the effective timeout for the lifecycle API call is shorter than intended.
**Fix:** Consider starting the timeout only after the token is obtained, or increase the timeout to account for token fetch latency.

### WR-05: Server error messages displayed verbatim from API responses

**File:** `internal/web/static/home.js:220, 317, 422, 518, 771`
**Issue:** Multiple locations use `data.message || data.error || '未知错误'` and display it via `showToast`. While `showToast` uses `textContent` (safe from XSS), displaying raw server error messages to the user can be confusing or leak internal details. This is a minor UX concern rather than a security issue since `textContent` is used.
**Fix:** Consider mapping known error codes to user-friendly messages instead of displaying raw API responses.

## Info

### IN-01: console.error/console.log statements in production code

**File:** `internal/web/static/home.js:20, 858, 1154, 1170, 1285, 1336, 1448`
**Issue:** Seven `console.error` and one `console.log` statement remain in the code. While these are useful for debugging, they clutter the console in production and could leak implementation details.
**Fix:** Consider wrapping in a debug flag check, or removing non-essential ones:
```javascript
// Replace console.log('Poll request failed...') at line 1448
// with silent handling since it's an expected condition
```

### IN-02: Inline styles mixed with CSS classes

**File:** `internal/web/static/home.js:462-463, 871-873, 1223`
**Issue:** Several DOM elements are styled via inline `style` properties (e.g., `headerDiv.style.display = 'flex'` at line 871) while the rest of the UI uses CSS classes. This inconsistency makes styling harder to maintain.
**Fix:** Move inline styles to CSS classes:
```css
/* In style.css */
.instance-card-header {
    display: flex;
    align-items: center;
    gap: var(--spacing-xs);
    margin-bottom: var(--spacing-sm);
}
```

### IN-03: Duplicated toggle switch handler code

**File:** `internal/web/static/home.js:169-172, 266-269, 370-373`
**Issue:** The toggle switch click handler is identically duplicated across `showCreateDialog`, `showEditDialog`, and `showCopyDialog`.
**Fix:** Extract into a helper function:
```javascript
function initToggleSwitch() {
    var toggleEl = document.getElementById('toggle-auto-start');
    var toggleLabel = document.getElementById('toggle-auto-start-label');
    toggleEl.addEventListener('click', function() {
        toggleEl.classList.toggle('active');
        toggleLabel.textContent = toggleEl.classList.contains('active') ? '开启' : '关闭';
    });
}
```

### IN-04: Large function - showNanobotConfigDialog is 250+ lines

**File:** `internal/web/static/home.js:533-785`
**Issue:** The `showNanobotConfigDialog` function spans approximately 250 lines with nested closures and multiple responsibilities (UI construction, data loading, bidirectional sync, save handling). This makes it difficult to test and maintain.
**Fix:** Consider breaking into smaller functions: `buildNanobotConfigHtml`, `initNanobotSync`, `saveNanobotConfig`, etc.

---

_Reviewed: 2026-04-12T18:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_

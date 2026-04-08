---
phase: 45-frontend-selfupdate-management-ui
reviewed: 2026-04-08T00:00:00Z
depth: standard
files_reviewed: 3
files_reviewed_list:
  - internal/web/static/home.html
  - internal/web/static/home.js
  - internal/web/static/style.css
findings:
  critical: 1
  warning: 3
  info: 2
  total: 6
status: issues_found
---

# Phase 45: Code Review Report

**Reviewed:** 2026-04-08
**Depth:** standard
**Files Reviewed:** 3
**Status:** issues_found

## Summary

Reviewed the frontend self-update management UI (HTML, JavaScript, CSS). The self-update module (lines 114-434 of home.js) uses `textContent` for all API response data, which is good XSS hygiene. However, the instance list rendering (lines 39-52) uses `innerHTML` with unsanitized `instance.name` from the API, creating a stored XSS vector if instance names contain HTML/script payloads. Additionally, the restart endpoint lacks any authentication or CSRF protection, and error messages from exceptions are surfaced directly to the UI.

The CSS file is clean with no issues. The HTML is well-structured.

## Critical Issues

### CR-01: XSS via innerHTML with unsanitized instance.name

**File:** `internal/web/static/home.js:39-52`
**Issue:** The `createInstanceCard` function builds HTML via template literals inserted with `innerHTML`. The `instance.name` value from the `/api/v1/instances/status` API is interpolated directly into the HTML without sanitization:

```javascript
card.innerHTML = `
    <a href="/logs/${instance.name}" class="instance-name">${instance.name}</a>
    ...
    <button class="btn-restart" data-instance="${instance.name}">重启实例</button>
`;
```

If an instance name in the server configuration contains HTML special characters (e.g., `<img src=x onerror=alert(1)>`), it will be parsed as HTML, enabling stored XSS. The `instance.name` originates from the Go config file (`cfg.Name`), flows through `InstanceStatus.Name`, and arrives in the JSON response. A malicious or misconfigured instance name would execute arbitrary JavaScript in the browser.

Note: The self-update module (lines 160-281) correctly uses `textContent` and `createElement` for all API data -- the same pattern should be applied here.

**Fix:** Replace the `innerHTML` template with safe DOM construction using `createElement` and `textContent`, matching the pattern already used in `checkUpdate()`:

```javascript
function createInstanceCard(instance) {
    const card = document.createElement('div');
    card.className = 'instance-card';

    const statusClass = instance.running ? 'status-running' : 'status-stopped';
    const statusText = instance.running ? '运行中' : '已停止';

    const nameLink = document.createElement('a');
    nameLink.href = '/logs/' + encodeURIComponent(instance.name);
    nameLink.className = 'instance-name';
    nameLink.textContent = instance.name;
    card.appendChild(nameLink);

    const infoDiv = document.createElement('div');
    infoDiv.className = 'instance-info';

    const portRow = document.createElement('div');
    portRow.className = 'info-row';
    const portLabel = document.createElement('span');
    portLabel.className = 'label';
    portLabel.textContent = '端口:';
    const portValue = document.createElement('span');
    portValue.className = 'value';
    portValue.textContent = instance.port;
    portRow.appendChild(portLabel);
    portRow.appendChild(portValue);
    infoDiv.appendChild(portRow);

    const statusRow = document.createElement('div');
    statusRow.className = 'info-row';
    const statusLabel = document.createElement('span');
    statusLabel.className = 'label';
    statusLabel.textContent = '状态:';
    const statusValue = document.createElement('span');
    statusValue.className = 'value ' + statusClass;
    statusValue.textContent = statusText;
    statusRow.appendChild(statusLabel);
    statusRow.appendChild(statusValue);
    infoDiv.appendChild(statusRow);
    card.appendChild(infoDiv);

    const restartBtn = document.createElement('button');
    restartBtn.className = 'btn-restart';
    restartBtn.dataset.instance = instance.name;
    restartBtn.textContent = '重启实例';
    restartBtn.addEventListener('click', function() {
        restartInstance(instance.name, restartBtn);
    });
    card.appendChild(restartBtn);

    return card;
}
```

## Warnings

### WR-01: Restart endpoint lacks authentication and CSRF protection

**File:** `internal/web/static/home.js:74` (client-side), `internal/api/server.go:66` (server-side)
**Issue:** The POST `/api/v1/instances/{name}/restart` endpoint has no authentication middleware and no CSRF protection (see `server.go` line 66 -- registered directly on mux with no middleware wrapper). This means:
1. Any website the user visits could send a POST request to restart instances (CSRF), assuming the user is on localhost.
2. Any script on the same origin can trigger instance restarts without authorization.

The self-update endpoints correctly use `authMiddleware` (server.go lines 99-103), but the restart endpoint was overlooked.

**Fix:** Add authentication middleware to the restart endpoint in `server.go`, consistent with the self-update endpoints:
```go
mux.Handle("POST /api/v1/instances/{name}/restart",
    authMiddleware(http.HandlerFunc(web.NewInstanceRestartHandler(im, logger))))
```

Then update the client-side `restartInstance` function to include the Bearer token:
```javascript
const response = await fetch(`/api/v1/instances/${instanceName}/restart`, {
    method: 'POST',
    headers: { 'Authorization': 'Bearer ' + authToken }
});
```

### WR-02: download_percent value used without range validation

**File:** `internal/web/static/home.js:399-401`
**Issue:** The `progress.download_percent` value from the API is used directly to set `textContent` and CSS `width` without validation. If the server returns an unexpected value (null, undefined, NaN, negative, or >100), the UI will display nonsensical text like "下载中 NaN%" or set an invalid CSS width.

```javascript
if (currentStatusEl) currentStatusEl.textContent = '下载中 ' + progress.download_percent + '%';
if (currentFillEl) currentFillEl.style.width = progress.download_percent + '%';
if (currentTextEl) currentTextEl.textContent = progress.download_percent + '%';
```

**Fix:** Clamp the value before use:
```javascript
const pct = Math.max(0, Math.min(100, Number(progress.download_percent) || 0));
if (currentStatusEl) currentStatusEl.textContent = '下载中 ' + pct + '%';
if (currentFillEl) currentFillEl.style.width = pct + '%';
if (currentTextEl) currentTextEl.textContent = pct + '%';
```

### WR-03: Exception messages surfaced directly to user in error divs

**File:** `internal/web/static/home.js:274` and `internal/web/static/home.js:326`
**Issue:** Error messages from caught exceptions are displayed to the user via `e.message`:
- Line 274: `errorDiv.textContent = '检测更新失败: ' + e.message;`
- Line 326: `errorDiv.textContent = '启动更新失败: ' + e.message;`

While `textContent` prevents XSS, the exception message may contain internal details (network URLs, stack traces, HTTP status codes) that should not be exposed to end users. This is an information leakage concern.

Note: The same pattern is used in `restartInstance` (line 92) with `alert()`, which is also affected.

**Fix:** Display a generic user-facing message and log the detailed error to the console (which the code already does via `console.error`):
```javascript
errorDiv.textContent = '检测更新失败，请查看控制台获取详情';
```

## Info

### IN-01: Auth token stored in global variable

**File:** `internal/web/static/home.js:115`
**Issue:** `authToken` is stored as a module-level global variable (`let authToken = '';`). While the `/api/v1/web-config` endpoint is localhost-only (server.go line 94), a global variable is accessible from the browser console or any other script on the page. This is mitigated by the localhost-only restriction on the web-config endpoint, so the practical risk is low.

**Fix:** Consider wrapping the self-update module in an IIFE to keep `authToken` out of the global scope, or use a closure pattern:
```javascript
(function() {
    let authToken = '';
    // ... rest of self-update code
})();
```

### IN-02: Style applied via inline JavaScript instead of CSS class

**File:** `internal/web/static/home.js:207` and `internal/web/static/home.js:229`
**Issue:** The `versionRow` and `dateRow` elements use inline styles (`versionRow.style.marginBottom = '4px'`) rather than CSS classes. This is inconsistent with the rest of the codebase which uses CSS classes for all styling.

**Fix:** Add a utility CSS class in `style.css` and reference it:
```css
/* In style.css */
.info-row + .info-row {
    margin-top: 4px;
}
```

---

_Reviewed: 2026-04-08_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_

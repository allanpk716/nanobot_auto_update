---
phase: 54-delete-button-protection
reviewed: 2026-04-14T00:00:00Z
depth: standard
files_reviewed: 2
files_reviewed_list:
  - internal/web/static/home.js
  - internal/web/static/style.css
findings:
  critical: 0
  warning: 1
  info: 1
  total: 2
status: issues_found
---

# Phase 54: Code Review Report

**Reviewed:** 2026-04-14
**Depth:** standard
**Files Reviewed:** 2
**Status:** issues_found

## Summary

Reviewed the delete button protection feature for running instances. The core changes are correct: the delete button is disabled via `deleteBtn.disabled = isRunning` on card creation, and CSS rules properly suppress hover effects on disabled state and apply a visual disabled style consistent with `.btn-form-danger:disabled`. One edge case was identified where `isRunning` can be `null` when status data is unavailable, which would evaluate to falsy and leave the delete button enabled -- this is actually the desired behavior since status is unknown. One info-level finding regarding debug logging that was introduced alongside the feature changes.

## Warnings

### WR-01: Debug console.log statements left in production code

**File:** `internal/web/static/home.js:813-819`
**Issue:** Six `console.log` debug statements were added to `loadInstances()` as part of this diff. These are labeled with `[DEBUG loadInstances]` and log Promise.allSettled results, rejected reasons, and value keys. While not a correctness bug, debug logging in production code leaks internal state (object keys, rejection reasons) into the browser console on every 5-second auto-refresh cycle (line 1108: `setInterval(loadInstances, 5000)`). This generates persistent console noise and exposes internal API structure.

**Fix:** Remove these debug statements before merging, or gate them behind a debug flag:
```javascript
// Remove lines 813-819:
//     console.log('[DEBUG loadInstances] statusOk=' + statusOk, 'configOk=' + configOk);
//     if (!statusOk) console.log('[DEBUG loadInstances] status rejected:', statusResult.reason);
//     if (!configOk) console.log('[DEBUG loadInstances] config rejected:', configResult.reason);
//     if (statusOk && statusResult.value) console.log('[DEBUG loadInstances] status value keys:', Object.keys(statusResult.value));
//     if (configOk && configResult.value) console.log('[DEBUG loadInstances] config value keys:', Object.keys(configResult.value));
```

## Info

### IN-01: `.btn-secondary:hover` lacks `:not(:disabled)` guard (pre-existing)

**File:** `internal/web/static/style.css:541`
**Issue:** The `.btn-secondary:hover` rule does not include `:not(:disabled)`, meaning all secondary buttons (including the delete button via class inheritance) would show hover effects even when disabled. However, the more specific `.btn-delete-danger:hover:not(:disabled)` rule overrides this for the delete button specifically. This means **edit**, **copy**, and **config** buttons in the status-only (auth-failed) fallback path (lines 858-860) would still show hover styling while disabled. This is a pre-existing inconsistency, not introduced by this change, but worth noting since this phase is touching the disabled-button pattern.

**Fix:** Consider adding `:not(:disabled)` to the base `.btn-secondary:hover` rule for consistency:
```css
.btn-secondary:hover:not(:disabled) {
    background-color: #f5f5f5;
    border-color: #2563eb;
}
```

---

_Reviewed: 2026-04-14_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_

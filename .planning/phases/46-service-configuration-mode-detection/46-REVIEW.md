---
phase: 46-service-configuration-mode-detection
reviewed: 2026-04-10T12:00:00Z
depth: standard
files_reviewed: 6
files_reviewed_list:
  - internal/config/service.go
  - internal/config/service_test.go
  - internal/config/config.go
  - internal/lifecycle/servicedetect_windows.go
  - internal/lifecycle/servicedetect.go
  - cmd/nanobot-auto-updater/main.go
findings:
  critical: 0
  warning: 1
  info: 1
  total: 2
status: issues_found
---

# Phase 46: Code Review Report

**Reviewed:** 2026-04-10T12:00:00Z
**Depth:** standard
**Files Reviewed:** 6
**Status:** issues_found

## Summary

Reviewed 6 files across the service configuration and mode detection feature. The implementation is well-structured: `ServiceConfig` validation is thorough with regex-based alphanumeric enforcement, the build-tag-based platform split for `IsServiceMode()` is clean, and `main.go` integrates service detection before config loading per the design (D-06). Test coverage for `ServiceConfig.Validate()` is solid with 11 table-driven cases covering nil, false, valid, invalid, boundary, and max-length scenarios.

One warning found regarding `len()` being used for `DisplayName` character-count validation where multi-byte Unicode would overcount. One informational note about the Phase 48 placeholder in main.go.

## Critical Issues

No critical issues found.

## Warnings

### WR-01: DisplayName length validation uses byte count instead of character count

**File:** `internal/config/service.go:43`
**Issue:** The `DisplayName` max-length check uses `len(s.DisplayName)` which counts bytes, not Unicode code points. Since `DisplayName` is a human-readable string (not restricted to ASCII like `ServiceName`), it could contain multi-byte characters (e.g., Chinese characters used in display names). For example, a 100-character CJK string would be 300 bytes, triggering the 256-byte limit when the actual character count is well under 256. The `ServiceName` field is correctly validated with `len()` because it is restricted to `[a-zA-Z0-9]+` (single-byte only), but `DisplayName` has no such restriction.

**Fix:**
```go
// Replace len(s.DisplayName) with utf8.RuneCountInString(s.DisplayName)
// Line 38:
if utf8.RuneCountInString(s.DisplayName) == 0 {
    return fmt.Errorf("service.display_name is required when auto_start is true")
}

// Line 43:
if utf8.RuneCountInString(s.DisplayName) > 256 {
    return fmt.Errorf("service.display_name must be at most 256 characters, got %d", utf8.RuneCountInString(s.DisplayName))
}
```

Note: The empty-string check on line 38 (`len(s.DisplayName) == 0`) is technically correct since an empty string is 0 in both bytes and runes. Changing it to `utf8.RuneCountInString` for consistency would be fine but is not strictly necessary.

## Info

### IN-01: Service registration exit is a Phase 48 placeholder

**File:** `cmd/nanobot-auto-updater/main.go:134-139`
**Issue:** When `auto_start` is `true` and the process is in console mode, the code logs "registering as Windows service" but then logs "Service registration will be handled by Phase 48" and exits with code 2. No actual SCM registration occurs. This is intentional per the phase plan (Phase 46 is detection/configuration only; Phase 48 implements the actual service management). The `os.Exit(2)` is safe here -- no defers or goroutines are active at this point in the code path. No action needed now, but ensure Phase 48 replaces lines 134-139 with the actual implementation.

**Fix:** No fix needed for Phase 46. Track in Phase 48 to replace the placeholder with actual SCM registration logic.

---

_Reviewed: 2026-04-10T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_

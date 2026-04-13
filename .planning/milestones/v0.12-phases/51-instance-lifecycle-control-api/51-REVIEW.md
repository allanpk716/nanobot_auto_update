---
phase: 51-instance-lifecycle-control-api
reviewed: 2026-04-12T12:00:00Z
depth: standard
files_reviewed: 4
files_reviewed_list:
  - internal/api/instance_lifecycle_handler.go
  - internal/api/instance_lifecycle_handler_test.go
  - internal/api/server.go
  - internal/instance/lifecycle_test_helper.go
findings:
  critical: 0
  warning: 1
  info: 2
  total: 3
status: issues_found
---

# Phase 51: Code Review Report

**Reviewed:** 2026-04-12T12:00:00Z
**Depth:** standard
**Files Reviewed:** 4
**Status:** issues_found

## Summary

Reviewed the instance lifecycle control API implementation (Phase 51), which adds start/stop endpoints for individual instances with update-lock coordination and bearer token authentication. The implementation is well-structured: proper locking semantics (TryLockUpdate + defer UnlockUpdate), good test coverage (success paths, error paths, auth, concurrency), and clean separation of concerns. Found one warning (inconsistent indentation in server.go that looks like a copy-paste artifact) and two informational items. No security or correctness issues.

## Warnings

### WR-01: Inconsistent indentation in server.go for Phase 50/51 blocks

**File:** `internal/api/server.go:108-123`
**Issue:** Lines 108-123 (the instance config CRUD endpoints and lifecycle control endpoints) are indented with 3 tabs instead of 2 tabs used by the surrounding code. This appears to be a copy-paste artifact from Phase 50 that was carried forward into Phase 51. The closing brace on line 106 (`}` of the `if selfUpdater != nil` block) returns to 2-tab indentation, but lines 108-123 use 3 tabs, and then line 125 drops back to 2 tabs for `// Create HTTP server`. While Go is whitespace-agnostic and this compiles and runs correctly, it is a maintenance smell -- it makes the code look like it is nested inside a block that was removed, which could confuse future readers.

**Fix:**
Remove one level of indentation from lines 108-123 so they align with the rest of the function body (2 tabs):
```go
		}  // line 106 -- end of selfUpdater block

		// Instance config CRUD endpoints (Phase 50: IC-01 through IC-06)
		instanceConfigHandler := NewInstanceConfigHandler(config.GetCurrentConfig, logger)
		mux.Handle("GET /api/v1/instance-configs", authMiddleware(http.HandlerFunc(instanceConfigHandler.HandleList)))
		// ... (remaining routes at same indent level)

		// Instance lifecycle control endpoints (Phase 51: LC-01, LC-02, LC-03)
		lifecycleHandler := NewInstanceLifecycleHandler(im, logger)
		mux.Handle("POST /api/v1/instances/{name}/start",
			authMiddleware(http.HandlerFunc(lifecycleHandler.HandleStart)))
		mux.Handle("POST /api/v1/instances/{name}/stop",
			authMiddleware(http.HandlerFunc(lifecycleHandler.HandleStop)))

		// Create HTTP server
```

## Info

### IN-01: json.NewEncoder.Encode return value ignored on success responses

**File:** `internal/api/instance_lifecycle_handler.go:69` and `internal/api/instance_lifecycle_handler.go:114`
**Issue:** The return value of `json.NewEncoder(w).Encode(...)` is not checked. If encoding fails, the response will be partially written with an incomplete JSON body and the HTTP status code will already be sent (200 OK by default). This is consistent with the pattern used elsewhere in the codebase (e.g., other handlers in this project), so it is not flagged as a warning. Just noting for awareness.

**Fix:** No immediate action required. If stricter error handling is desired, capture the error and log it:
```go
if err := json.NewEncoder(w).Encode(map[string]interface{}{...}); err != nil {
    h.logger.Error("Failed to encode response", "error", err)
}
```

### IN-02: lifecycle_test_helper.go exposes a public test-aid method

**File:** `internal/instance/lifecycle_test_helper.go:10`
**Issue:** `SetPIDForTest` is an exported method on `InstanceLifecycle` that sets the internal `pid` field. It is exported so the `api` package tests can use it. This is a common Go pattern for test helpers, and the doc comment makes the intent clear. No action needed, but documenting that this is intentionally exported for cross-package testing.

**Fix:** No fix needed. The current approach is standard Go practice.

---

_Reviewed: 2026-04-12T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_

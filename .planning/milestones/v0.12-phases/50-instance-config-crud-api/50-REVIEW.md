---
phase: 50-instance-config-crud-api
reviewed: 2026-04-11T12:00:00Z
depth: standard
files_reviewed: 6
files_reviewed_list:
  - internal/api/instance_config_handler.go
  - internal/api/instance_config_handler_test.go
  - internal/api/server.go
  - internal/config/config.go
  - internal/config/hotreload.go
  - internal/config/update_test.go
findings:
  critical: 1
  warning: 3
  info: 3
  total: 7
status: issues_found
---

# Phase 50: Code Review Report

**Reviewed:** 2026-04-11T12:00:00Z
**Depth:** standard
**Files Reviewed:** 6
**Status:** issues_found

## Summary

Reviewed 6 files implementing the Instance Config CRUD API (Phase 50), including the HTTP handler, route registration, `UpdateConfig` atomic read-modify-write with deep copy, and hot-reload integration. The code is well-structured with proper auth middleware, validation error collection, and concurrency safety via mutex.

One critical security issue was found: unbounded request body reads (`io.ReadAll`) in the copy handler without size limits, which enables denial-of-service attacks. Three warnings relate to a missing error check on `StopAllNanobots` return value, inconsistent `StartupTimeout` zero-value handling, and a defensive missing-argument validation in the copy path.

## Critical Issues

### CR-01: Unbounded Request Body Read (DoS Vector)

**File:** `internal/api/instance_config_handler.go:355`
**Issue:** `HandleCopy` uses `io.ReadAll(r.Body)` without any size limit. An attacker could send an extremely large request body to exhaust server memory, causing a denial-of-service condition. The `HandleCreate` and `HandleUpdate` handlers avoid this by using `json.NewDecoder(r.Body).Decode()` which reads incrementally, but `HandleCopy` eagerly reads the entire body into memory before parsing.
**Fix:**
```go
// Limit request body to 1MB to prevent DoS
const maxBodySize = 1 << 20 // 1MB

func (h *InstanceConfigHandler) HandleCopy(w http.ResponseWriter, r *http.Request) {
    sourceName := r.PathValue("name")

    bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
    if err != nil {
        writeJSONError(w, http.StatusBadRequest, "bad_request", "Failed to read request body")
        return
    }
    // ... rest of handler
}
```

Consider applying `http.MaxBytesReader` at the middleware level for all instance-config endpoints as a broader hardening measure.

## Warnings

### WR-01: StopAllNanobots Return Value Ignored

**File:** `internal/api/instance_config_handler.go:338`
**Issue:** `lifecycle.StopAllNanobots` returns `(int, error)` indicating how many processes were stopped and any failure, but `HandleDelete` discards both return values. If stopping nanobot processes fails, the API still returns 200 OK, and the caller has no indication that the process cleanup did not complete.
**Fix:**
```go
stopped, err := lifecycle.StopAllNanobots(ctx, 5*time.Second, h.logger)
if err != nil {
    h.logger.Error("failed to stop nanobot processes after config delete", "error", err, "stopped", stopped)
    // Continue returning success since config was deleted, but log the error prominently
}
```

### WR-02: StartupTimeout Zero-Value Silently Uses Source Duration on Update

**File:** `internal/api/instance_config_handler.go:105-108`
**Issue:** In `toInstanceConfig`, if `req.StartupTimeout == 0` (the JSON default for omitted field), the resulting `InstanceConfig.StartupTimeout` remains `0` (zero `time.Duration`). For `HandleCreate` and `HandleUpdate`, this means an omitted `startup_timeout` field produces a config with `StartupTimeout = 0`, bypassing the minimum 5-second validation in `InstanceConfig.Validate()`. The validation in `instance.go:35` only checks `ic.StartupTimeout != 0 && ic.StartupTimeout < 5*time.Second`, so zero passes validation. This may result in a zero startup timeout for the instance.

For `HandleCopy` (line 424), this is mitigated because the source instance's timeout is cloned first, and the override only applies when `req.StartupTimeout > 0`. But `HandleCreate` and `HandleUpdate` build the config entirely from the request.
**Fix:** In `toInstanceConfig`, apply a sensible default when zero is provided, or change `InstanceConfig.Validate()` to reject zero values:
```go
// Option A: Apply default in toInstanceConfig
if req.StartupTimeout > 0 {
    ic.StartupTimeout = time.Duration(req.StartupTimeout) * time.Second
} else {
    ic.StartupTimeout = 30 * time.Second // sensible default
}

// Option B: Strengthen validation in instance.go
if ic.StartupTimeout < 5*time.Second {
    return fmt.Errorf("startup_timeout must be at least 5 seconds, got: %v", ic.StartupTimeout)
}
```

### WR-03: HandleCopy Does Not Validate Empty Source Name

**File:** `internal/api/instance_config_handler.go:351-353`
**Issue:** `HandleCopy` reads `sourceName := r.PathValue("name")` but does not check if it is empty. While Go 1.22+ ServeMux routing should always populate the `{name}` pattern, defensive validation is absent. More importantly, when no body is provided (empty body test case at line 579), the auto-generated name `sourceName + "-copy"` could produce `"-copy"` if the path value were somehow empty. The `HandleGet`, `HandleUpdate`, and `HandleDelete` handlers similarly do not validate the path name. This is a minor defensive gap rather than an active bug since the ServeMux pattern matching should prevent empty values.
**Fix:** Add a guard at the top of each path-value handler:
```go
func (h *InstanceConfigHandler) HandleCopy(w http.ResponseWriter, r *http.Request) {
    sourceName := r.PathValue("name")
    if sourceName == "" {
        writeJSONError(w, http.StatusBadRequest, "bad_request", "Instance name is required")
        return
    }
    // ...
}
```

## Info

### IN-01: Duplicate Deep Copy of AutoStart in HandleCopy

**File:** `internal/api/instance_config_handler.go:431-435`
**Issue:** `HandleCopy` performs an explicit deep copy of `clonedInstance.AutoStart` (lines 432-435), but `deepCopyConfig` in `config.go:199-215` already performs the same deep copy when the config goes through the `UpdateConfig` path. Since `clonedInstance` is a value copy of `*sourceIC` (line 378), the `AutoStart` pointer is shared with the source instance's config inside the `UpdateConfig` closure. The explicit deep copy at line 432 is correct and necessary because the mutation function receives a deep copy of the config, but `clonedInstance` is captured from the outer scope and its `AutoStart` still points to the source's allocation. This is correct but worth a comment explaining why the copy is needed here despite `deepCopyConfig` existing.
**Fix:** No code change required. Consider adding a clarifying comment:
```go
// Deep copy AutoStart pointer -- clonedInstance was created from sourceIC
// which points into the config passed to the UpdateConfig closure.
// Although deepCopyConfig copies instances, this AutoStart pointer is on
// a local variable outside that deep copy's scope.
```

### IN-02: Global State Mutation in Tests (viperInstance)

**File:** `internal/config/update_test.go:54,80-81,121-122,216,264`
**Issue:** Several tests directly set `viperInstance = nil` to reset state between test runs. This mutates a package-level global that could leak between test cases if tests run in parallel. The tests use `t.Cleanup(func() { config.StopWatch() })` in some places but manually reset `viperInstance` in others. This is a test-only concern and does not affect production code.
**Fix:** No immediate fix required since Go tests run sequentially by default. If parallel tests are added in the future, use `sync.Once` or a test-local viper instance.

### IN-03: Code Duplication in Error Handling Blocks

**File:** `internal/api/instance_config_handler.go:236-244,289-301,323-330,445-458`
**Issue:** The error handling pattern (checking for `notFoundError`, then `validationError`, then generic error) is repeated verbatim across `HandleCreate`, `HandleUpdate`, `HandleDelete`, and `HandleCopy`. This is a maintainability concern -- adding a new error type requires updating all four blocks.
**Fix:** Consider extracting a helper method:
```go
func (h *InstanceConfigHandler) handleMutationError(w http.ResponseWriter, err error) bool {
    var nfErr *notFoundError
    if errors.As(err, &nfErr) {
        writeJSONError(w, http.StatusNotFound, "not_found", nfErr.Error())
        return true
    }
    var valErr *validationError
    if errors.As(err, &valErr) {
        h.writeValidationError(w, "Validation failed", valErr.details)
        return true
    }
    writeJSONError(w, http.StatusInternalServerError, "internal_error", err.Error())
    return true
}
```

---

_Reviewed: 2026-04-11T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_

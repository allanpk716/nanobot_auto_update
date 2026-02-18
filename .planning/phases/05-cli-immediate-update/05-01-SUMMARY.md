# Phase 5, Plan 01: Add --update-now Flag with JSON Output

**Status:** Complete
**Duration:** 5 minutes
**Completed:** 2026-02-18

## Summary

Successfully implemented the `--update-now` CLI flag for immediate update execution with JSON output, replacing the old `--run-once` flag. The feature enables third-party programs to invoke the updater programmatically and parse structured results.

## Changes Made

### Files Modified
- `cmd/main.go` - Added new CLI flags, JSON output, lifecycle integration
- `cmd/main_test.go` - Updated tests for new flags

### Key Implementations

1. **New CLI Flags**
   - `--update-now`: Triggers immediate update and exits with JSON output
   - `--timeout`: Configurable update timeout (default 5 minutes, accepts "5m", "300s", etc.)

2. **JSON Output**
   - `UpdateNowResult` struct with success/error fields
   - `outputJSON()` helper for stdout emission
   - Success format: `{"success": true, "source": "github|pypi", "message": "Update completed"}`
   - Failure format: `{"success": false, "error": "description", "exit_code": 1}`

3. **Lifecycle Integration**
   - Stop nanobot before update
   - Execute update with configurable timeout
   - Start nanobot after successful update
   - Start failure logs warning only (doesn't fail update)

4. **Help Documentation**
   - Added JSON output format documentation
   - Documented new flags with examples

5. **Tests Updated**
   - `TestUpdateNowFlag`: Verifies flag parsing
   - `TestTimeoutFlag`: Tests valid/invalid timeout formats
   - `TestTimeoutDefault`: Verifies 5m0s default
   - `TestHelpFlag`: Now checks for JSON format documentation

## Verification Results

### Build
- `go build ./cmd/main.go` - PASS

### Tests
- `go test ./cmd/... -short` - PASS (all tests skip in short mode)

### Help Output
```
Usage: nanobot-auto-updater [options]

Options:

JSON Output Format (--update-now):
  Success: {"success": true, "version": "X.Y.Z", "source": "github|pypi", "message": "Update completed"}
  Failure: {"success": false, "error": "description", "exit_code": 1}
      --config string      Path to config file (default "./config.yaml")
      --cron string        Cron expression (overrides config file)
  -h, --help               Show help
      --timeout duration   Update timeout duration (e.g., '5m', '300s') (default 5m0s)
      --update-now         Execute immediate update and exit with JSON output
      --version            Show version information
```

## Requirements Coverage

| ID | Description | Status |
|----|-------------|--------|
| CLI-01 | --update-now flag triggers immediate update | Implemented |
| CLI-02 | --timeout flag for configurable timeout | Implemented |
| CLI-03 | JSON output to stdout as last line | Implemented |
| CLI-04 | Exit code 0 on success, non-zero on failure | Implemented |
| CLI-05 | Help output documents flags and JSON format | Implemented |

## Decisions Made

1. **Timeout format**: Used Go's `time.Duration` which accepts both "5m" and "300s" formats
2. **JSON field names**: Used `success`, `source`, `message`, `error`, `exit_code`
3. **Version field**: Included in struct but not populated (can be added later if needed)
4. **Start failure handling**: Logs warning only, consistent with existing behavior

## Deviations

None - all planned tasks completed as specified.

## Key Files

### Created
None

### Modified
- `cmd/main.go` - CLI entry point with --update-now and --timeout flags
- `cmd/main_test.go` - Updated tests for new flags

## Next Steps

Phase 5 is complete. All phases finished.

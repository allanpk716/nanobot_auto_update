# Phase 5: CLI Immediate Update - Context

**Gathered:** 2026-02-18
**Status:** Ready for planning

<domain>
## Phase Boundary

Support command-line flag `--update-now` to trigger immediate update on startup. The feature is designed for third-party programmatic invocation with JSON output for both success and error cases. Existing `--run-once` flag will be removed and replaced by this new flag.

</domain>

<decisions>
## Implementation Decisions

### Flag Design
- Flag name: `--update-now`
- Behavior: Execute immediate update and exit (no scheduler)
- Remove existing `--run-once` flag completely
- Add `--timeout` flag for configurable update timeout (default: 5 minutes)

### Exit Behavior
- Exit immediately after update completes
- Do NOT start scheduler (scheduled mode)
- Update flow includes: check uv -> stop nanobot -> update -> start nanobot gateway

### Failure Handling
- Exit code: 0 = success, non-zero = failure
- Timeout configurable via `--timeout` flag (in seconds or duration format)

### JSON Output
- Output to stdout (last line of output)
- Include logs before JSON (for debugging)
- Output format:

**Success:**
```json
{
  "success": true,
  "version": "1.2.3",
  "source": "github",
  "message": "Update completed"
}
```

**Failure:**
```json
{
  "success": false,
  "error": "Network timeout",
  "exit_code": 1
}
```

### Help Documentation
- Update `--help` output to include:
  - `--update-now` flag description
  - `--timeout` flag description
  - JSON output format documentation for third-party consumers
- Remove `--run-once` from help

### Nanobot Lifecycle
- Maintain existing behavior: stop before update, start after update
- Start nanobot gateway after successful update

### Claude's Discretion
- Exact JSON field names (as long as they convey required info)
- Timeout format (seconds vs duration string like "5m")
- Log verbosity level during update

</decisions>

<specifics>
## Specific Ideas

- Feature designed for third-party programmatic invocation
- JSON output enables easy parsing by calling programs
- Logs + JSON format balances debugging needs with programmatic consumption

</specifics>

<deferred>
## Deferred Ideas

None - discussion stayed within phase scope.

</deferred>

---

*Phase: 05-cli-immediate-update*
*Context gathered: 2026-02-18*

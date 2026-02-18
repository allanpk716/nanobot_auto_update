# Phase 5: CLI Immediate Update - Research

**Researched:** 2026-02-18
**Domain:** Go CLI, JSON output, flag parsing
**Confidence:** HIGH

## Summary

Phase 5 adds a `--update-now` CLI flag for immediate update execution with JSON output for programmatic consumption. The feature replaces the existing `--run-once` flag and adds a configurable `--timeout` flag. The implementation leverages the existing codebase infrastructure (updater, lifecycle manager) and requires modifications to the CLI entry point, output formatting, and exit code handling.

**Primary recommendation:** Use Go's `encoding/json` for output, extend pflag configuration for new flags, and maintain clear separation between human-readable logs (stdout during execution) and machine-readable JSON (final line of output).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Flag Design
- Flag name: `--update-now`
- Behavior: Execute immediate update and exit (no scheduler)
- Remove existing `--run-once` flag completely
- Add `--timeout` flag for configurable update timeout (default: 5 minutes)

#### Exit Behavior
- Exit immediately after update completes
- Do NOT start scheduler (scheduled mode)
- Update flow includes: check uv -> stop nanobot -> update -> start nanobot gateway

#### Failure Handling
- Exit code: 0 = success, non-zero = failure
- Timeout configurable via `--timeout` flag (in seconds or duration format)

#### JSON Output
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

#### Help Documentation
- Update `--help` output to include:
  - `--update-now` flag description
  - `--timeout` flag description
  - JSON output format documentation for third-party consumers
- Remove `--run-once` from help

#### Nanobot Lifecycle
- Maintain existing behavior: stop before update, start after update
- Start nanobot gateway after successful update

### Claude's Discretion
- Exact JSON field names (as long as they convey required info)
- Timeout format (seconds vs duration string like "5m")
- Log verbosity level during update

### Deferred Ideas (OUT OF SCOPE)
None - discussion stayed within phase scope.
</user_constraints>

## Standard Stack

### Core (Already in Project)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/spf13/pflag | v1.0.5 | POSIX-style CLI flag parsing | Already used in project, drop-in replacement for standard flag |
| encoding/json | stdlib | JSON encoding/decoding | Standard library, no dependencies |
| context | stdlib | Timeout/cancellation | Already used throughout project |
| os | stdlib | Exit codes, stdout/stderr | Standard library |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| time.ParseDuration | stdlib | Parse duration strings like "5m" | For --timeout flag parsing |

## Architecture Patterns

### Current CLI Pattern (cmd/main.go)
```
1. Define flags with pflag
2. Parse flags
3. Handle --version / --help (early exit)
4. Load config
5. Override config from CLI flags
6. Initialize infrastructure (logs, uv check)
7. Execute mode (run-once vs scheduled)
```

### Recommended Pattern for --update-now
```
1. Define flags with pflag (add --update-now, --timeout; remove --run-once)
2. Parse flags
3. Handle --version / --help (early exit)
4. Load config (optional for --update-now - only need nanobot config)
5. Initialize infrastructure (logs, uv check)
6. If --update-now:
   a. Create context with timeout from --timeout flag
   b. Create lifecycle manager with config
   c. Execute: StopForUpdate -> Update -> StartAfterUpdate
   d. Format and output JSON result
   e. Exit with appropriate code
7. Else: Start scheduler (existing behavior)
```

### JSON Output Pattern
```go
// Define result struct
type UpdateResult struct {
    Success bool   `json:"success"`
    Version string `json:"version,omitempty"`
    Source  string `json:"source,omitempty"`
    Message string `json:"message,omitempty"`
    Error   string `json:"error,omitempty"`
    ExitCode int   `json:"exit_code,omitempty"`
}

// Output as final line
json.NewEncoder(os.Stdout).Encode(result)
```

### Anti-Patterns to Avoid
- **JSON mixed with logs on stderr**: Keep JSON on stdout, logs on stderr or mixed (JSON is last line of stdout)
- **Not flushing before exit**: Ensure all output is written before os.Exit()
- **Ignoring context timeout**: The update must respect the --timeout deadline

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Duration parsing | Manual string parsing | time.ParseDuration | Handles "5m", "300s", etc. |
| JSON output | Manual string formatting | encoding/json | Handles escaping, proper formatting |
| Timeout handling | Manual timers | context.WithTimeout | Integrates with existing context pattern |

## Common Pitfalls

### Pitfall 1: JSON Not Being Last Line
**What goes wrong:** Logs written to stdout mix with JSON, making parsing difficult
**Why it happens:** slog outputs to stdout by default
**How to avoid:** Either:
1. Write logs to stderr for --update-now mode, OR
2. Accept that JSON is the LAST LINE and caller reads backwards
**Warning signs:** Third-party consumers can't parse output

### Pitfall 2: Missing Version in Success JSON
**What goes wrong:** Version field not populated in JSON output
**Why it happens:** Current updater doesn't extract version from uv output
**How to avoid:** Either:
1. Parse version from `uv tool list` after install, OR
2. Use `nanobot --version` to get installed version
**Recommendation:** Claude's discretion - may need to add version detection

### Pitfall 3: Exit Code Lost in JSON
**What goes wrong:** Program exits with 0 even on failure because JSON was written successfully
**Why it happens:** os.Exit(0) called after JSON encode
**How to avoid:** Use os.Exit(result.ExitCode) or separate exit code variable

### Pitfall 4: Timeout Not Propagating
**What goes wrong:** --timeout flag parsed but not passed to update context
**Why it happens:** Context timeout hardcoded in updater (5 minutes)
**How to avoid:** Pass timeout value through to updater or create context in main with timeout

## Code Examples

### Duration Flag Parsing (pflag)
```go
// Source: pflag documentation pattern
var timeout time.Duration
pflag.DurationVar(&timeout, "timeout", 5*time.Minute, "Update timeout (e.g., '5m', '300s')")
```

### Context with Timeout
```go
// Source: Go stdlib pattern
ctx, cancel := context.WithTimeout(context.Background(), timeout)
defer cancel()
```

### JSON Output
```go
// Source: Go stdlib encoding/json
type UpdateResultJSON struct {
    Success  bool   `json:"success"`
    Version  string `json:"version,omitempty"`
    Source   string `json:"source,omitempty"`
    Message  string `json:"message,omitempty"`
    Error    string `json:"error,omitempty"`
    ExitCode int    `json:"exit_code,omitempty"`
}

result := UpdateResultJSON{
    Success: true,
    Version: "1.2.3",
    Source:  "github",
    Message: "Update completed",
}

output, _ := json.Marshal(result)
fmt.Println(string(output)) // Last line of stdout
```

### Exit Code Pattern
```go
// Determine exit code
exitCode := 0
if !result.Success {
    exitCode = result.ExitCode
    if exitCode == 0 {
        exitCode = 1 // Default failure code
    }
}

os.Exit(exitCode)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| --run-once flag | --update-now flag | Phase 5 | Clearer intent, JSON output |
| No timeout flag | --timeout flag | Phase 5 | Configurable update window |

**Deprecated/outdated:**
- `--run-once` flag: Being replaced by `--update-now` with JSON output

## Integration Points

### Existing Code to Modify
1. **cmd/main.go**: Add new flags, remove --run-once, add --update-now handler
2. **internal/updater/updater.go**: May need to accept configurable timeout or return version info

### Existing Code to Reuse
1. **internal/lifecycle/manager.go**: StopForUpdate(), StartAfterUpdate()
2. **internal/updater/updater.go**: Update() method
3. **internal/config/config.go**: NanobotConfig for lifecycle manager

### Code Flow for --update-now
```
main.go:
  -> NewManager(config.Nanobot)
  -> manager.StopForUpdate(ctx)
  -> updater.Update(ctx)
  -> manager.StartAfterUpdate(ctx)
  -> Output JSON
  -> os.Exit(code)
```

## Sources

### Primary (HIGH confidence)
- Go stdlib documentation (encoding/json, context, time) - Standard library
- github.com/spf13/pflag - Already used in project

### Secondary (MEDIUM confidence)
- Existing project code (cmd/main.go, internal/updater/updater.go) - Verified in codebase

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries already in use or stdlib
- Architecture: HIGH - Clear integration with existing code
- Pitfalls: HIGH - Common CLI patterns well understood

**Research date:** 2026-02-18
**Valid until:** 90 days - Stable patterns

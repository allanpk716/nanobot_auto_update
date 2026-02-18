# Phase 2: Core Update Logic - Research

**Researched:** 2026-02-18
**Domain:** Go command execution, uv package manager integration, Windows process management
**Confidence:** HIGH

## Summary

Phase 2 implements the core update logic that allows nanobot to be updated from GitHub's main branch using the `uv` Python package manager, with automatic fallback to the stable PyPI version if the GitHub update fails. The implementation requires understanding three key areas: (1) Go's `os/exec` package for running external commands with hidden windows, (2) `exec.LookPath` for verifying uv installation at startup, and (3) `uv tool install` commands for installing Python packages from both Git repositories and PyPI.

The primary technical approach is straightforward: use `exec.LookPath("uv")` to verify uv is installed at startup (exit with error if not), then use `exec.Command` with `windows.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}` to execute `uv tool install git+https://github.com/nanobot-ai/nanobot@main` for the primary update, falling back to `uv tool install nanobot-ai` if the first command fails. Capture command output using `CombinedOutput()` for detailed logging.

**Primary recommendation:** Create a dedicated `internal/updater` package with three components: (1) `checker.go` for uv installation verification using `exec.LookPath`, (2) `updater.go` for the update logic with GitHub primary and PyPI fallback, and (3) use the existing `windows.SysProcAttr` pattern from `lifecycle/stopper.go` and `starter.go` for hiding command windows.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| os/exec | Go stdlib | Execute external commands | Standard Go approach for subprocess management |
| golang.org/x/sys/windows | v0.41.0 | Windows-specific process attributes | Already in use for CREATE_NO_WINDOW support |
| log/slog | Go stdlib | Structured logging | Already configured in project |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| context | Go stdlib | Cancellation and timeout | All external command execution |
| bytes | Go stdlib | Buffer for command output capture | When capturing stdout/stderr |
| fmt | Go stdlib | Error formatting and messages | All error handling |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| exec.LookPath | exec.Command probe | LookPath is cleaner - no subprocess spawn, just PATH lookup |
| CombinedOutput | separate stdout/stderr | Combined is simpler for logging, no need to separate streams |
| os/exec | github.com/cli/safeexec | safeexec avoids Windows current directory security issue, but we control the environment and the command name is fixed ("uv") |

**Key insight:** The project already has the `windows.SysProcAttr` pattern established in `internal/lifecycle/stopper.go` and `starter.go`. Reuse this exact pattern for consistency.

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── updater/              # NEW: Core update logic
│   ├── checker.go        # uv installation verification
│   ├── updater.go        # Update with fallback logic
│   └── updater_test.go   # Unit tests
├── lifecycle/            # EXISTING: Nanobot lifecycle management
│   ├── manager.go
│   ├── stopper.go
│   ├── starter.go
│   └── detector.go
├── config/               # EXISTING: Configuration loading
└── logging/              # EXISTING: Custom log format
```

### Pattern 1: Command Existence Check with exec.LookPath
**What:** Verify uv is installed before attempting updates
**When to use:** At application startup (UPDT-01, UPDT-02)
**Example:**
```go
// Source: https://pkg.go.dev/os/exec - LookPath documentation
import (
    "errors"
    "fmt"
    "os/exec"
)

// CheckUvInstalled verifies uv is in PATH
func CheckUvInstalled() error {
    _, err := exec.LookPath("uv")
    if err != nil {
        if errors.Is(err, exec.ErrNotFound) {
            return fmt.Errorf("uv is not installed or not in PATH - please install uv first")
        }
        return fmt.Errorf("failed to check for uv: %w", err)
    }
    return nil
}
```

### Pattern 2: Execute Command with Hidden Window and Output Capture
**What:** Run external commands without showing console window, capture output for logging
**When to use:** All uv command executions (UPDT-03, UPDT-04, INFR-10)
**Example:**
```go
// Source: Project's internal/lifecycle/stopper.go pattern + os/exec docs
//go:build windows

import (
    "bytes"
    "context"
    "fmt"
    "os/exec"
    "time"

    "golang.org/x/sys/windows"
)

// RunHiddenCommand executes a command with hidden window and captures output
func RunHiddenCommand(ctx context.Context, name string, args ...string) (string, error) {
    cmd := exec.CommandContext(ctx, name, args...)
    cmd.SysProcAttr = &windows.SysProcAttr{
        HideWindow:    true,
        CreationFlags: windows.CREATE_NO_WINDOW,
    }

    output, err := cmd.CombinedOutput()
    if err != nil {
        return string(output), fmt.Errorf("command failed: %w", err)
    }
    return string(output), nil
}

// Example usage for update:
func UpdateFromGitHub(ctx context.Context) error {
    output, err := RunHiddenCommand(ctx, "uv", "tool", "install",
        "git+https://github.com/nanobot-ai/nanobot@main")
    if err != nil {
        return fmt.Errorf("github update failed: %w\nOutput: %s", err, output)
    }
    return nil
}
```

### Pattern 3: Update with Fallback Logic
**What:** Try GitHub main branch first, fall back to PyPI stable on failure
**When to use:** Main update operation (UPDT-03, UPDT-04)
**Example:**
```go
// Source: Requirement UPDT-03, UPDT-04
import (
    "context"
    "log/slog"
)

type UpdateResult string

const (
    UpdateSuccess   UpdateResult = "success"
    UpdateFallback  UpdateResult = "fallback"
    UpdateFailed    UpdateResult = "failed"
)

// Update executes update with GitHub primary and PyPI fallback
func Update(ctx context.Context, logger *slog.Logger) (UpdateResult, error) {
    // Primary: Install from GitHub main branch
    logger.Info("Attempting update from GitHub main branch")
    output, err := RunHiddenCommand(ctx, "uv", "tool", "install",
        "git+https://github.com/nanobot-ai/nanobot@main")
    if err == nil {
        logger.Info("Update successful from GitHub",
            "source", "github",
            "output", output)
        return UpdateSuccess, nil
    }

    logger.Warn("GitHub update failed, attempting PyPI fallback",
        "error", err.Error(),
        "github_output", output)

    // Fallback: Install stable version from PyPI
    output, err = RunHiddenCommand(ctx, "uv", "tool", "install", "nanobot-ai")
    if err == nil {
        logger.Info("Update successful from PyPI fallback",
            "source", "pypi",
            "output", output)
        return UpdateFallback, nil
    }

    logger.Error("Update failed - both GitHub and PyPI failed",
        "pypi_output", output,
        "error", err.Error())
    return UpdateFailed, fmt.Errorf("update failed: %w\nPyPI output: %s", err, output)
}
```

### Anti-Patterns to Avoid
- **Don't use exec.Command without SysProcAttr:** On Windows, this can flash a command prompt window (violates INFR-10)
- **Don't use os.StartProcess directly:** exec.Command wraps it properly and handles argument quoting
- **Don't ignore context cancellation:** Long-running uv commands should respect context for graceful shutdown
- **Don't swallow command output:** Always log CombinedOutput for debugging update failures

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Check if uv exists | Manual PATH scanning | exec.LookPath | Handles PATHEXT on Windows, returns ErrNotFound |
| Capture command output | Manual pipe handling | cmd.CombinedOutput() | Handles stdout/stderr merging, buffer management |
| Hide command window | Custom Win32 API calls | windows.SysProcAttr | Already proven in existing code |
| Structured logging | fmt.Printf | slog with custom handler | Already configured with rotation |

**Key insight:** The Go standard library and golang.org/x/sys/windows provide everything needed. No third-party packages required for this phase.

## Common Pitfalls

### Pitfall 1: Missing exec.ErrNotFound Check
**What goes wrong:** When uv is not installed, `LookPath` returns a generic error that may be confused with permission errors
**Why it happens:** The error type must be checked with `errors.Is(err, exec.ErrNotFound)` for reliable detection
**How to avoid:** Always use `errors.Is()` to check for ErrNotFound specifically
**Warning signs:** Error messages like "file not found" without clear indication that uv is missing

### Pitfall 2: Command Window Flashing
**What goes wrong:** Executing uv commands shows a brief console window popup
**Why it happens:** Both `HideWindow: true` AND `CreationFlags: windows.CREATE_NO_WINDOW` are needed for complete hiding
**How to avoid:** Use the exact SysProcAttr pattern from existing lifecycle code:
```go
cmd.SysProcAttr = &windows.SysProcAttr{
    HideWindow:    true,
    CreationFlags: windows.CREATE_NO_WINDOW,
}
```
**Warning signs:** Console windows appearing briefly during updates

### Pitfall 3: Not Logging Command Output
**What goes wrong:** When update fails, there's no diagnostic information about what went wrong
**Why it happens:** Forgetting to capture or log CombinedOutput()
**How to avoid:** Always capture output and log it with the error context:
```go
output, err := cmd.CombinedOutput()
if err != nil {
    logger.Error("Command failed", "error", err, "output", string(output))
}
```
**Warning signs:** Empty or generic error messages in logs when update fails

### Pitfall 4: Context Timeout Not Applied
**What goes wrong:** uv install commands can hang indefinitely if network is slow or GitHub is unreachable
**Why it happens:** Not using exec.CommandContext or not setting a deadline
**How to avoid:** Always use context with timeout for external commands:
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()
cmd := exec.CommandContext(ctx, "uv", ...)
```
**Warning signs:** Update process hangs without completing

### Pitfall 5: Git URL Format Incorrect
**What goes wrong:** uv tool install fails with "invalid URL" or "repository not found"
**Why it happens:** The git+https URL format must be exact: `git+https://github.com/owner/repo@branch`
**How to avoid:** Use the exact format from uv documentation:
- GitHub main: `git+https://github.com/nanobot-ai/nanobot@main`
- PyPI stable: just `nanobot-ai` (package name only)
**Warning signs:** "Invalid requirement" or "cannot find repository" errors from uv

## Code Examples

### Complete Updater Package Structure

```go
// internal/updater/checker.go
//go:build windows

package updater

import (
	"errors"
	"fmt"
	"os/exec"
)

// CheckUvInstalled verifies uv is available in PATH.
// Returns a clear error if uv is not installed.
func CheckUvInstalled() error {
	_, err := exec.LookPath("uv")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("uv is not installed or not in PATH - please install uv from https://docs.astral.sh/uv/")
		}
		return fmt.Errorf("failed to check for uv installation: %w", err)
	}
	return nil
}
```

```go
// internal/updater/updater.go
//go:build windows

package updater

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"

	"golang.org/x/sys/windows"
)

// UpdateResult represents the outcome of an update attempt
type UpdateResult string

const (
	ResultSuccess  UpdateResult = "success"  // GitHub update succeeded
	ResultFallback UpdateResult = "fallback" // PyPI fallback succeeded
	ResultFailed   UpdateResult = "failed"   // Both GitHub and PyPI failed
)

// Updater handles nanobot updates via uv
type Updater struct {
	logger       *slog.Logger
	githubURL    string
	pypiPackage  string
	updateTimeout time.Duration
}

// NewUpdater creates a new updater instance
func NewUpdater(logger *slog.Logger) *Updater {
	return &Updater{
		logger:       logger,
		githubURL:    "git+https://github.com/nanobot-ai/nanobot@main",
		pypiPackage:  "nanobot-ai",
		updateTimeout: 5 * time.Minute,
	}
}

// Update attempts to update nanobot from GitHub main, falling back to PyPI
func (u *Updater) Update(ctx context.Context) (UpdateResult, error) {
	ctx, cancel := context.WithTimeout(ctx, u.updateTimeout)
	defer cancel()

	// Primary: Try GitHub main branch
	u.logger.Info("Starting update from GitHub main branch")
	output, err := u.runCommand(ctx, "uv", "tool", "install", u.githubURL)
	if err == nil {
		u.logger.Info("Update successful from GitHub",
			"source", "github",
			"output", truncateOutput(output))
		return ResultSuccess, nil
	}

	u.logger.Warn("GitHub update failed, attempting PyPI fallback",
		"error", err.Error(),
		"github_output", truncateOutput(output))

	// Fallback: Try PyPI stable version
	output, err = u.runCommand(ctx, "uv", "tool", "install", u.pypiPackage)
	if err == nil {
		u.logger.Info("Update successful from PyPI fallback",
			"source", "pypi",
			"output", truncateOutput(output))
		return ResultFallback, nil
	}

	u.logger.Error("Update failed - both GitHub and PyPI attempts failed",
		"pypi_output", truncateOutput(output),
		"error", err.Error())
	return ResultFailed, fmt.Errorf("update failed (GitHub and PyPI): %w", err)
}

// runCommand executes a command with hidden window and returns combined output
func (u *Updater) runCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	return buf.String(), err
}

// truncateOutput limits output length for logging
func truncateOutput(s string) string {
	const maxLen = 500
	if len(s) > maxLen {
		return s[:maxLen] + "... (truncated)"
	}
	return s
}
```

### Main.go Integration Example

```go
// cmd/main.go - Add at startup after logger initialization
func main() {
	// ... existing flag and config handling ...

	// Initialize logger
	logger := logging.NewLogger("./logs")
	slog.SetDefault(logger)

	// CHECK UV INSTALLATION (UPDT-01, UPDT-02)
	logger.Info("Checking uv installation")
	if err := updater.CheckUvInstalled(); err != nil {
		logger.Error("uv installation check failed", "error", err.Error())
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	logger.Info("uv is installed and available")

	// ... rest of main ...
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| pip install | uv tool install | 2024-2025 | Faster, isolated tool environments |
| pipx | uv tool install | 2024-2025 | Unified under uv, no separate tool needed |
| Manual PATH scanning | exec.LookPath | Go 1.x | Standard library handles PATHEXT on Windows |
| syscall.SysProcAttr | golang.org/x/sys/windows | Go 1.4+ | Better Windows API support |

**Deprecated/outdated:**
- `syscall.SysProcAttr` on Windows: Use `golang.org/x/sys/windows.SysProcAttr` instead (project already uses this)
- Direct Win32 API calls for hiding windows: Use SysProcAttr instead

## Open Questions

1. **Update timeout duration**
   - What we know: uv tool install typically completes in seconds to a minute
   - What's unclear: Should timeout be configurable or hardcoded?
   - Recommendation: Start with 5-minute hardcoded timeout (generous for slow networks), make configurable in future if needed

2. **Output truncation for logs**
   - What we know: uv output can be verbose, especially on first install
   - What's unclear: What's the optimal truncation length?
   - Recommendation: Truncate to 500 characters in logs, but keep full output available for debugging if needed

3. **Retry logic for transient failures**
   - What we know: Network failures can be transient
   - What's unclear: Should the updater retry before falling back?
   - Recommendation: Keep it simple - fallback to PyPI counts as a retry mechanism. Add explicit retry in Phase 3 or later if needed.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| UPDT-01 | Check if uv is installed on startup | Pattern 1: exec.LookPath for clean existence check |
| UPDT-02 | Log error and exit if uv is not installed | Check at startup, use slog.Error + os.Exit(1) |
| UPDT-03 | Install nanobot from GitHub main branch using uv | Pattern 3: `uv tool install git+https://github.com/nanobot-ai/nanobot@main` |
| UPDT-04 | Fallback to uv tool install nanobot-ai stable version if update fails | Pattern 3: Try GitHub first, fall back to `uv tool install nanobot-ai` |
| UPDT-05 | Log detailed update process information | Pattern 2 & 3: Capture CombinedOutput, log with context at each step |
| INFR-10 | Hide command window when executing uv commands | Pattern 2: SysProcAttr with HideWindow + CREATE_NO_WINDOW |

## Sources

### Primary (HIGH confidence)
- https://pkg.go.dev/os/exec - Go standard library exec package documentation
- https://docs.astral.sh/uv/guides/tools/ - Official uv tools guide (verified 2026-02-18)
- Project source: internal/lifecycle/stopper.go - Existing CREATE_NO_WINDOW pattern
- Project source: internal/lifecycle/starter.go - Existing hidden window pattern

### Secondary (MEDIUM confidence)
- https://github.com/astral-sh/uv - uv repository and documentation
- https://pkg.go.dev/golang.org/x/sys/windows - Windows syscall package

### Tertiary (LOW confidence)
- None required - all core functionality verified through primary sources

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All using Go standard library and existing project patterns
- Architecture: HIGH - Clear separation of concerns, consistent with existing codebase
- Pitfalls: HIGH - Well-documented exec.LookPath and CREATE_NO_WINDOW behavior
- uv commands: HIGH - Official documentation verified

**Research date:** 2026-02-18
**Valid until:** 30 days - uv is stable, Go exec patterns are stable

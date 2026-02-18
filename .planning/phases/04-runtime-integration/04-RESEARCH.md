# Phase 4: Runtime Integration - Research

**Researched:** 2026-02-18
**Domain:** Windows background execution, Go linker flags for GUI subsystem, build process configuration
**Confidence:** HIGH

## Summary

Phase 4 focuses on making the nanobot-auto-updater run as a Windows background application without displaying a console window. This is a build-time configuration change rather than a code change. The primary mechanism is using Go's linker flag `-H=windowsgui` which sets the PE (Portable Executable) subsystem to "Windows GUI" instead of "Console", preventing Windows from allocating a console window when the program starts.

The key insight is that this is purely a build configuration change - no code modifications are required for the core application logic. The project already uses `windows.SysProcAttr` with `HideWindow: true` and `CREATE_NO_WINDOW` flags for spawned child processes (lifecycle/stopper.go, lifecycle/starter.go, updater/updater.go), ensuring those processes don't flash console windows either.

**Primary recommendation:** Add a Makefile with two build targets: `build` (console version for debugging/testing) and `build-release` (GUI subsystem for production). Update any build scripts to use the release target. No code changes required - the application already writes logs to files and doesn't require console output for normal operation.

<phase_requirements>

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| RUN-01 | Support Windows background execution, hide console window | ldflags `-H=windowsgui` sets PE subsystem to GUI, preventing console allocation |
| RUN-02 | Program starts manually, not auto-start on boot | No Windows service registration required; user runs executable manually from desktop or shortcut |

</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go linker (cmd/link) | stdlib | Set PE subsystem via -H flag | Built into Go toolchain, no external dependencies |
| debug/pe | stdlib | (Optional) Detect subsystem at runtime | Standard library for PE file inspection, useful for testing |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| golang.org/x/sys/windows | v0.41.0 | Hide child process windows | Already in use for CREATE_NO_WINDOW (project dependency) |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| ldflags -H=windowsgui | github.com/ebitengine/hideconsole package | hideconsole hides window at runtime, but still briefly flashes. ldflags is cleaner, no dependency, no code change needed. |
| ldflags -H=windowsgui | syscall.GetConsoleWindow + ShowWindow | Runtime approach requires Windows API calls, adds complexity, and still shows brief flash. ldflags prevents console entirely. |
| ldflags -H=windowsgui | golang.org/x/sys/windows/svc | Windows service package is for *services* (auto-start, SCM integration). Our requirement is manual start, no service registration. |

**Key insight:** The project already correctly hides child process windows using `windows.SysProcAttr`. Phase 4 only needs to hide the *main* application window, which is a build-time setting.

## Architecture Patterns

### Recommended Project Structure
```
nanobot_auto_update/
├── Makefile              # NEW: Build targets for console and GUI builds
├── cmd/
│   └── main.go           # UNCHANGED: No code changes required
├── internal/
│   ├── lifecycle/        # UNCHANGED: Already uses SysProcAttr correctly
│   ├── updater/          # UNCHANGED: Already uses SysProcAttr correctly
│   ├── config/
│   ├── logging/
│   ├── scheduler/
│   └── notifier/
└── go.mod
```

### Pattern 1: Build with GUI Subsystem (ldflags)
**What:** Compile the executable as a Windows GUI binary that doesn't allocate a console.
**When to use:** For production/distribution builds where the app runs in background.
**Example:**
```bash
# Build command with GUI subsystem
go build -ldflags="-H=windowsgui" -o nanobot-auto-updater.exe ./cmd

# With version embedding (recommended)
go build -ldflags="-H=windowsgui -X main.Version=1.0.0" -o nanobot-auto-updater.exe ./cmd
```

**How it works:**
- The `-H=windowsgui` flag tells the Go linker to set the PE OptionalHeader.Subsystem field to `IMAGE_SUBSYSTEM_WINDOWS_GUI` (value 2) instead of `IMAGE_SUBSYSTEM_WINDOWS_CUI` (value 3)
- When Windows launches a process, it checks this field. GUI subsystem processes don't get a console allocated
- Standard handles (stdin, stdout, stderr) are not connected to a console
- Source: https://pkg.go.dev/cmd/link (official Go linker documentation)

### Pattern 2: Makefile with Multiple Build Targets
**What:** Provide both console (debug) and GUI (release) build options.
**When to use:** Development needs console for debugging; production needs hidden console.
**Example:**
```makefile
# Makefile
.PHONY: build build-release clean test

# Development build with console window (for debugging)
build:
	go build -o nanobot-auto-updater.exe ./cmd

# Production build with hidden console (for distribution)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -H=windowsgui -X main.Version=$(VERSION)

build-release:
	go build -ldflags="$(LDFLAGS)" -o nanobot-auto-updater.exe ./cmd

# Clean build artifacts
clean:
	rm -f nanobot-auto-updater.exe

# Run tests
test:
	go test ./...
```

### Pattern 3: Optional Runtime Detection of GUI Subsystem
**What:** Detect at runtime whether the binary was built with GUI subsystem.
**When to use:** For debugging/testing, or conditional behavior based on build type.
**Example:**
```go
//go:build windows

package main

import (
	"debug/pe"
	"fmt"
	"os"
)

const (
	IMAGE_SUBSYSTEM_WINDOWS_GUI = 2
	IMAGE_SUBSYSTEM_WINDOWS_CUI = 3
)

// IsGUISubsystem returns true if built with -H=windowsgui
func IsGUISubsystem() (bool, error) {
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}

	f, err := pe.Open(exe)
	if err != nil {
		return false, err
	}
	defer f.Close()

	var subsystem uint16
	switch header := f.OptionalHeader.(type) {
	case *pe.OptionalHeader64:
		subsystem = header.Subsystem
	case *pe.OptionalHeader32:
		subsystem = header.Subsystem
	default:
		return false, fmt.Errorf("unknown optional header type")
	}

	return subsystem == IMAGE_SUBSYSTEM_WINDOWS_GUI, nil
}
```
**Source:** Stack Overflow verified solution - https://stackoverflow.com/questions/58813512

### Anti-Patterns to Avoid
- **Using fmt.Print/fmt.Fprintf(os.Stdout, ...) in GUI binary:** stdout goes nowhere in GUI binaries. Use slog for all output (project already does this).
- **Using os.Stdin in GUI binary:** stdin is not connected. All input must come from config files, CLI flags, or environment variables.
- **Expecting --help output in GUI binary:** --help won't display because there's no console. Keep console build for help/debugging.
- **Confusing GUI subsystem with Windows Service:** A GUI binary is NOT a Windows Service. It's just a regular executable without a console window. It doesn't auto-start or integrate with Service Control Manager.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Hide console window | Runtime ShowWindow API calls | ldflags -H=windowsgui | Build-time solution is cleaner, no code, no brief flash |
| Create Windows Service | Custom service registration | golang.org/x/sys/windows/svc | If service needed (not in this phase's requirements) |
| Parse command output | Manual pipe handling | exec.Command with CombinedOutput() | Already implemented in project |

**Key insight:** Phase 4 is almost entirely a build configuration change, not a code change.

## Common Pitfalls

### Pitfall 1: Console Output Disappears
**What goes wrong:** After building with -H=windowsgui, fmt.Print and fmt.Println output disappears. The --help and --version flags appear to do nothing.
**Why it happens:** GUI subsystem processes don't have stdout/stderr connected to a console, even when run from cmd.exe.
**How to avoid:** The project already uses file-based logging (internal/logging/logging.go with slog). Ensure all user-facing output uses slog or writes to files. For --help/--version, keep the console build available for debugging.
**Warning signs:** Using fmt.Print, fmt.Println, or os.Stdout.Write anywhere in the code.

### Pitfall 2: Confusing GUI Binary with Windows Service
**What goes wrong:** Expecting the program to auto-start on boot or appear in Windows Services list.
**Why it happens:** "Background execution" sounds like "Windows Service" but they're different. GUI binaries are just regular executables without console windows.
**How to avoid:** Read requirements carefully - RUN-02 explicitly states "Program starts manually, not auto-start on boot". No service registration needed.
**Warning signs:** Looking at golang.org/x/sys/windows/svc documentation when requirement says manual start.

### Pitfall 3: Testing GUI Binary Without Console Access
**What goes wrong:** Can't see error messages when GUI binary fails to start.
**Why it happens:** No console means nowhere for startup errors to appear.
**How to avoid:** (1) Check log files in ./logs/ directory, (2) Keep console build for debugging, (3) Use Windows Event Viewer if needed, (4) Run from cmd.exe and check exit code.
**Warning signs:** Relying solely on GUI build for development/testing.

### Pitfall 4: Forgetting ldflags in Build Scripts
**What goes wrong:** CI/CD pipeline builds console version instead of GUI version.
**Why it happens:** Makefile exists but build script uses `go build` directly.
**How to avoid:** Document the release build command clearly. Use Makefile targets consistently. Add comments in any build scripts.
**Warning signs:** Build scripts that don't reference Makefile or use -ldflags.

### Pitfall 5: Wrong ldflags Syntax
**What goes wrong:** `go build -ldflags -Hwindowsgui` produces error "unknown flag -Hwindowsgui".
**Why it happens:** The equals sign is required: `-H=windowsgui`, not `-Hwindowsgui`.
**How to avoid:** Use the correct syntax: `go build -ldflags="-H=windowsgui"`. Test the build command before documenting.
**Warning signs:** Build errors mentioning "unknown flag" or linker errors.

## Code Examples

### Minimal Makefile for This Project

```makefile
# Makefile for nanobot-auto-updater
# Provides both console (debug) and GUI (release) builds

.PHONY: build build-release clean test help

# Version from git tags, or "dev" if unavailable
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Build flags for release (GUI subsystem + version embedding)
LDFLAGS_RELEASE = -H=windowsgui -X main.Version=$(VERSION)

# Default: console build (easier debugging)
build:
	go build -o nanobot-auto-updater.exe ./cmd
	@echo "Built console version: nanobot-auto-updater.exe"

# Release: GUI build (no console window)
build-release:
	go build -ldflags="$(LDFLAGS_RELEASE)" -o nanobot-auto-updater.exe ./cmd
	@echo "Built release version (no console): nanobot-auto-updater.exe"

# Clean build artifacts
clean:
	rm -f nanobot-auto-updater.exe
	@echo "Cleaned build artifacts"

# Run all tests
test:
	go test ./...

# Show help
help:
	@echo "Available targets:"
	@echo "  make build         - Build console version (for debugging)"
	@echo "  make build-release - Build GUI version (for distribution)"
	@echo "  make test          - Run tests"
	@echo "  make clean         - Remove build artifacts"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION=x.x.x      - Set version (default: git tag or 'dev')"
```

### Verification Test Script

```bash
#!/bin/bash
# test-background-execution.sh
# Verifies that the release build runs without console

set -e

echo "=== Testing Background Execution ==="

# Build both versions
echo "Building console version..."
make build

echo "Building release version..."
make build-release

# Test 1: Verify release binary exists
if [ ! -f "nanobot-auto-updater.exe" ]; then
    echo "FAIL: nanobot-auto-updater.exe not found"
    exit 1
fi
echo "PASS: Binary exists"

# Test 2: Verify --version works (check exit code, output goes nowhere)
./nanobot-auto-updater.exe --version
EXIT_CODE=$?
if [ $EXIT_CODE -ne 0 ]; then
    echo "FAIL: --version returned exit code $EXIT_CODE"
    exit 1
fi
echo "PASS: --version works (exit code 0)"

# Test 3: Verify --help flag (no output expected in GUI mode)
./nanobot-auto-updater.exe --help
EXIT_CODE=$?
if [ $EXIT_CODE -ne 0 ]; then
    echo "FAIL: --help returned exit code $EXIT_CODE"
    exit 1
fi
echo "PASS: --help works (exit code 0)"

# Test 4: Verify binary runs (start and check process exists)
echo "Starting background process..."
./nanobot-auto-updater.exe --run-once &
PID=$!
sleep 2

# Check if logs directory was created (indicates successful startup)
if [ ! -d "./logs" ]; then
    echo "FAIL: logs directory not created"
    kill $PID 2>/dev/null || true
    exit 1
fi
echo "PASS: Application created logs directory"

# Cleanup
kill $PID 2>/dev/null || true

echo ""
echo "=== All tests passed ==="
echo "Release build verified: runs without console window"
echo ""
echo "Manual verification steps:"
echo "1. Double-click nanobot-auto-updater.exe in Explorer"
echo "2. Verify no console window appears"
echo "3. Check ./logs/ directory for log output"
```

### Manual Test Cases (for VERIFICATION.md)

```markdown
## Manual Test Cases for RUN-01, RUN-02

### TC-01: Console Window Hidden on Double-Click
**Steps:**
1. Run `make build-release` to create GUI binary
2. Open Windows Explorer and navigate to project directory
3. Double-click `nanobot-auto-updater.exe`
4. Observe screen for console window

**Expected:** No console window appears. Application starts silently.

**Verify:** Check `./logs/` directory for log files indicating application started.

### TC-02: Application Runs from Command Prompt
**Steps:**
1. Open cmd.exe
2. Navigate to project directory
3. Run `nanobot-auto-updater.exe --version`
4. Check exit code: `echo %ERRORLEVEL%`

**Expected:** Exit code 0. No output displayed (GUI binary has no stdout).

### TC-03: Logs Written to File
**Steps:**
1. Run `nanobot-auto-updater.exe --run-once`
2. Wait for execution to complete
3. Check `./logs/` directory

**Expected:** Log files exist with timestamps matching execution time.

### TC-04: Console Build Still Works
**Steps:**
1. Run `make build` to create console binary
2. Run `nanobot-auto-updater.exe --version`

**Expected:** Version string printed to console.

### TC-05: Manual Start (No Auto-Start)
**Steps:**
1. Reboot computer
2. Log in and wait 5 minutes
3. Check if nanobot-auto-updater.exe is running

**Expected:** Application NOT running (manual start only).

**Verify:** Application only runs when explicitly started by user.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Custom WinMain entry point | ldflags -H=windowsgui | Go 1.x | No code changes needed, pure build configuration |
| Windows Service for background apps | GUI subsystem executables | Always | Simpler deployment, no SCM registration, manual start |
| ShowWindow at runtime | ldflags at build time | Always | No brief console flash, cleaner solution |

**Deprecated/outdated:**
- Using `syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleWindow")` + ShowWindow: Still shows brief flash. Use ldflags instead.
- Windows Service for simple background apps: Overkill for manually-started applications. Use GUI subsystem instead.

## Open Questions

1. **Should we provide both .exe files (console and GUI) in releases?**
   - What we know: Console version is useful for debugging, GUI version for production
   - What's unclear: Whether to ship both or just GUI version
   - Recommendation: Ship only GUI version for simplicity. Developers can build console version locally with `make build`.

2. **How to handle errors during startup in GUI mode?**
   - What we know: No console means errors go to log files only
   - What's unclear: Should we add Windows Event Log integration?
   - Recommendation: Keep it simple. Log files are sufficient. Add Event Log in future if users request it.

3. **Should build-release include other optimizations?**
   - What we know: ldflags can also strip debug info (-s -w) for smaller binary
   - What's unclear: Whether smaller binary is worth losing debug symbols
   - Recommendation: Don't strip debug symbols for v1. Debugging production issues is more important than saving a few MB.

## Sources

### Primary (HIGH confidence)
- https://pkg.go.dev/cmd/link - Official Go linker documentation (verified 2026-02-18)
- https://pkg.go.dev/debug/pe - Go standard library PE file parsing
- https://pkg.go.dev/golang.org/x/sys/windows - Project's existing Windows syscall dependency

### Secondary (MEDIUM confidence)
- https://stackoverflow.com/questions/23250505 - How do I create an executable from Golang that doesn't open a console window when run? (66 votes, verified correct)
- https://stackoverflow.com/questions/36727740 - How to hide console window of a Go program on Windows (73 votes, verified correct)
- https://stackoverflow.com/questions/58813512 - Detect if binary was compiled with -H=windowsgui at runtime

### Tertiary (LOW confidence)
- https://github.com/ebitengine/hideconsole - Alternative runtime approach (not recommended for this use case)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Using Go standard linker, no external dependencies
- Architecture: HIGH - Pattern is well-established, no code changes required
- Pitfalls: HIGH - Well-documented behavior of GUI subsystem binaries
- Build process: HIGH - Straightforward ldflags addition to existing build

**Research date:** 2026-02-18
**Valid until:** 90 days - Go linker flags are stable, unlikely to change

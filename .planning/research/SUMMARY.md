# Project Research Summary

**Project:** Nanobot Auto Updater
**Domain:** Windows Background Service / CLI Tool for Auto-Updating Python Tools
**Researched:** 2025-02-18
**Confidence:** MEDIUM-HIGH

## Executive Summary

This is a Windows background service written in Go that automatically keeps the nanobot AI agent tool up-to-date using the uv package manager. The product falls into the category of system utilities/automation tools, where experts typically build using a layered architecture: CLI entry point, service wrapper for OS integration, scheduler for timed execution, and infrastructure for external integrations.

The recommended approach is to build a single-binary Go application using `kardianos/service` for Windows service integration, `robfig/cron` for scheduling, `urfave/cli` for command-line handling, and `WQGroup/logger` (project-specified) for structured logging. The architecture should follow a three-layer pattern: entry point (cmd/), application logic (internal/app/), and infrastructure (internal/executor/, internal/notify/). Key design decisions include using interfaces for testability, dependency injection over globals, and proper Windows service lifecycle handling.

The primary risks are Windows-specific: file locking preventing binary replacement during updates, Service Control Manager recovery action gotchas, command prompt window flashing during subprocess execution, and silent configuration parsing failures. These are mitigated by implementing rename-then-replace patterns for updates, proper service status reporting, using `SysProcAttr.HideWindow` for all subprocess calls, and explicit configuration validation.

## Key Findings

### Recommended Stack

Go is the required language with a mature ecosystem for Windows services and CLI tools. The stack is well-established with high-confidence choices based on official documentation and community adoption.

**Core technologies:**
- **Go 1.23+**: Primary language - single-binary deployment, cross-compilation, minimal runtime dependencies
- **kardianos/service v1.2.4**: Cross-platform service management - de facto standard for Windows services in Go, handles service callbacks
- **robfig/cron v3**: Cron-based job scheduling - industry standard, timezone-aware, thread-safe execution
- **urfave/cli v3**: Command-line parsing - zero-dependency, declarative API, built-in shell completion
- **go.yaml.in/yaml/v4**: YAML configuration - official library with strict field validation support
- **WQGroup/logger**: Structured logging - project-specified requirement, built on logrus with rotation support
- **gregdel/pushover**: Push notifications - official Go wrapper for failure alerts

### Expected Features

The product needs table-stakes features for a background automation tool plus a few differentiators for production readiness.

**Must have (table stakes):**
- Cron-based scheduling - users expect set-and-forget automation
- YAML configuration file - persistent settings without re-specifying
- Command-line arguments - override config for testing and one-off runs
- Structured logging - visibility into tool behavior
- Background/headless operation - Windows service or hidden console
- Dependency checking (uv presence) - fail fast with clear message

**Should have (differentiators):**
- Failure notifications via Pushover - proactive alerting when updates fail
- Automatic rollback to stable version - resilience when GitHub update fails
- Log rotation - prevents disk fill-up on long-running services
- Retry with exponential backoff - handles transient network issues

**Defer (v2+):**
- Maintenance windows - only update during specified hours
- Version pinning - update to specific version
- Health checks/HTTP endpoint - monitoring integration
- Update verification (checksums/signatures) - security feature

### Architecture Approach

The recommended architecture follows a three-layer pattern with clear component boundaries and dependency injection. The service wrapper pattern enables the same binary to run as a Windows service and interactively for debugging.

**Major components:**
1. **Entry Point (cmd/)** - CLI parsing via urfave/cli, wires dependencies, starts service
2. **Service Wrapper** - kardianos/service integration, bridges OS service manager to application logic
3. **Scheduler (internal/scheduler/)** - robfig/cron wrapper, manages scheduled update jobs with overlap prevention
4. **Updater (internal/app/updater.go)** - Core update logic, executes uv commands, handles rollback
5. **Configuration (internal/config/)** - YAML loading with hierarchy: defaults < file < env < CLI flags
6. **Notifier (internal/notify/)** - Interface-based notifications, Pushover implementation

### Critical Pitfalls

1. **Cannot Replace Running Binary on Windows** - Windows locks executables while running. Use rename-then-replace pattern: rename old binary to backup location, move new binary to target, clean up backup on next start.

2. **Service Control Manager Recovery Actions Don't Work** - SCM has specific conditions for recovery. Never set reset period to 0 (causes system hangs), implement application-level health checks, ensure proper SetServiceStatus calls.

3. **Cron Scheduler Job Overlap** - Jobs pile up if they take longer than interval. Implement mutex/flag to prevent concurrent execution, use SkipIfStillRunning pattern.

4. **Command Prompt Window Flashes** - Subprocesses create visible windows by default. Use `SysProcAttr{HideWindow: true, CreationFlags: syscall.CREATE_NO_WINDOW}` for all exec.Command calls.

5. **Configuration Zero-Value Bugs** - YAML/JSON parsing silently accepts invalid values. Implement explicit Validate() method on config struct, fail fast on missing required fields.

## Implications for Roadmap

Based on research, suggested phase structure:

### Phase 1: Foundation and Core Infrastructure
**Rationale:** Logging and configuration are required by all other components. These have no dependencies on custom code and must exist first.
**Delivers:** Working logger, configuration loader with validation, uv executor with window hiding
**Addresses:** Configuration zero-value bugs, command prompt window flashes
**Pitfalls avoided:** HTTP client timeout issues, subprocess window flashing

### Phase 2: Core Update Logic
**Rationale:** Implements the primary value proposition - checking for and performing updates via uv.
**Delivers:** Dependency checker, update checker, updater with rollback support
**Uses:** Executor from Phase 1, logger, configuration
**Implements:** uv tool upgrade command execution, fallback to stable version

### Phase 3: Scheduling and Notifications
**Rationale:** Enables automated operation with proactive alerting.
**Delivers:** Cron-based scheduler with overlap prevention, Pushover notifier
**Uses:** Updater from Phase 2
**Addresses:** Cron scheduler job overlap pitfall

### Phase 4: CLI and Service Integration
**Rationale:** Ties everything together with proper Windows service support.
**Delivers:** CLI commands (run, install, version), service wrapper, graceful shutdown
**Uses:** All previous components
**Addresses:** Service Control Manager recovery actions, blocking in service start

### Phase 5: Polish and Distribution
**Rationale:** Production-readiness requires documentation, examples, and release automation.
**Delivers:** Sample configuration, README, GoReleaser setup
**Depends on:** Complete application being functional

### Phase Ordering Rationale

- Phase 1 must come first because logger and config are dependencies for all other components
- Phase 2 builds on infrastructure to implement core business logic
- Phase 3 can be developed after core logic exists; notifications and scheduling are independent
- Phase 4 is last because CLI ties everything together
- Architecture research indicates clear build order: infrastructure -> application logic -> notifications/scheduling -> CLI integration

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 2:** uv package manager integration specifics - need to verify exact commands for GitHub vs PyPI installation, version detection
- **Phase 3:** Pushover rate limits and retry behavior - need to verify API constraints for notification frequency

Phases with standard patterns (skip research-phase):
- **Phase 1:** Well-documented Go patterns for logging, config, exec
- **Phase 4:** kardianos/service has extensive documentation and examples

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Based on official documentation, high GitHub stars, established ecosystem patterns |
| Features | MEDIUM | Based on industry patterns and competitor analysis; limited official docs for this niche domain |
| Architecture | HIGH | Standard Go project layout, well-documented patterns from kardianos/service and urfave/cli |
| Pitfalls | MEDIUM | WebSearch-based findings verified across multiple sources; some need production validation |

**Overall confidence:** MEDIUM-HIGH

### Gaps to Address

- **uv command specifics:** Exact commands for installing from GitHub main branch vs PyPI stable version need verification during implementation
- **nanobot version detection:** How to detect currently installed version vs GitHub latest - may require parsing uv output or nanobot --version
- **WQGroup/logger integration:** Need to verify rotation capabilities and custom formatter implementation for the specified log format
- **Windows service testing:** Must test in Session 0 context (actual service environment) to validate recovery behavior

## Sources

### Primary (HIGH confidence)
- github.com/kardianos/service - Cross-platform service management, Windows service support
- github.com/robfig/cron - Cron scheduling library documentation
- docs.astral.sh/uv/reference/cli/ - uv CLI commands reference
- pkg.go.dev/github.com/kardianos/service - Package documentation
- github.com/gregdel/pushover - Pushover Go API wrapper documentation
- github.com/WQGroup/logger - Project-specified logger

### Secondary (MEDIUM confidence)
- Chromium Updater Functional Specification - Official spec, authoritative patterns
- Microsoft Learn: "Descriptions of some best practices when you create Windows Services"
- Microsoft Learn: "Guidelines for Services"
- Better Stack: Go logging library benchmarks
- Cloud Google: Cron expression scheduling patterns
- Pushover API documentation

### Tertiary (LOW confidence)
- WebSearch findings on anti-patterns - Should verify with actual production experience
- Blog posts on Windows service recovery - General guidance, not updater-specific

---
*Research completed: 2025-02-18*
*Ready for roadmap: yes*

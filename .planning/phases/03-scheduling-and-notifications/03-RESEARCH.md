# Phase 3: Scheduling and Notifications - Research

**Researched:** 2026-02-18
**Domain:** Cron-based job scheduling and Pushover push notifications for Go
**Confidence:** HIGH

## Summary

Phase 3 implements automatic scheduled execution of updates using `robfig/cron/v3` with SkipIfStillRunning mode to prevent job overlap, and Pushover notifications for failure alerts. The implementation requires two main components: (1) a scheduler module that wraps the cron library with proper job overlap prevention using `cron.WithChain(cron.SkipIfStillRunning(logger))`, and (2) a notifier module that reads Pushover credentials from environment variables (PUSHOVER_TOKEN, PUSHOVER_USER) and sends failure notifications using the `gregdel/pushover` Go library.

The primary technical approach is straightforward: Initialize the cron scheduler with SkipIfStillRunning wrapper, add the update job with the cron expression from config, start the scheduler, and handle graceful shutdown. For notifications, read environment variables at startup, log a warning if not configured (don't fail), and call the Pushover API when updates fail, including the error message in the notification body.

**Primary recommendation:** Create two new internal packages: (1) `internal/scheduler/scheduler.go` wrapping robfig/cron with SkipIfStillRunning mode and slog integration, (2) `internal/notifier/notifier.go` with a Notifier interface, Pushover implementation using gregdel/pushover, and graceful handling of missing configuration. Integrate both in `cmd/main.go` to replace the existing "TODO: Phase 3" placeholder.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SCHD-01 | Support cron expression scheduled update triggering | Pattern 1: cron.New() with AddFunc() using cfg.Cron expression |
| SCHD-02 | Default cron is "0 3 * * *" (daily at 3 AM) | Already in config.go defaults(), cron parser validates expression |
| SCHD-03 | Prevent job overlap execution (SkipIfStillRunning mode) | Pattern 2: cron.WithChain(cron.SkipIfStillRunning(logger)) |
| NOTF-01 | Read Pushover config from environment variables (PUSHOVER_TOKEN, PUSHOVER_USER) | Pattern 3: os.Getenv() with warning log if missing |
| NOTF-02 | Send notification via Pushover when update fails | Pattern 4: gregdel/pushover SendMessage() on update error |
| NOTF-03 | Notification includes failure reason | Pattern 4: Include error message in notification body |
| NOTF-04 | Log warning only if Pushover config missing, don't block program | Pattern 3: Check env vars, log warning, continue execution |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| robfig/cron/v3 | v3.0.1 (already in go.mod) | Cron-based job scheduling | Industry standard for Go (98.3 benchmark score). Already imported in config.go for cron validation. Supports standard 5-field expressions, job wrappers like SkipIfStillRunning, and custom loggers. |
| gregdel/pushover | Latest | Pushover API client | Official Go wrapper with 154+ stars. Simple API: pushover.New(token), NewRecipient(user), SendMessage(). Supports message titles and priorities. |
| os (stdlib) | - | Environment variable access | os.Getenv() for reading PUSHOVER_TOKEN and PUSHOVER_USER |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| context | Go stdlib | Cancellation and graceful shutdown | Scheduler Stop() returns context for waiting on running jobs |
| log/slog | Go stdlib (already in use) | Cron job logging | Pass to cron.SkipIfStillRunning for skip event logging |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| robfig/cron | time.Ticker | Cron is required by specification (SCHD-01), provides human-readable schedules |
| gregdel/pushover | Direct HTTP POST | Library handles API details, error parsing, retry logic - less code to maintain |
| gregdel/pushover | nikoksr/notify | notify is overkill for single notification channel - adds abstraction we don't need |
| os.Getenv | viper.AutomaticEnv | os.Getenv is simpler for two variables, no viper configuration needed |

**Installation:**
```bash
go get github.com/gregdel/pushover@latest
```

Note: robfig/cron/v3 is already in go.mod.

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── scheduler/              # NEW: Cron scheduling
│   ├── scheduler.go        # Scheduler wrapper with SkipIfStillRunning
│   └── scheduler_test.go   # Unit tests
├── notifier/               # NEW: Push notifications
│   ├── notifier.go         # Notifier interface and Pushover implementation
│   └── notifier_test.go    # Unit tests
├── updater/                # EXISTING: Update logic
│   ├── checker.go
│   └── updater.go
├── lifecycle/              # EXISTING: Nanobot lifecycle
├── config/                 # EXISTING: Configuration
└── logging/                # EXISTING: Custom log format
```

### Pattern 1: Cron Scheduler Initialization
**What:** Create and configure cron scheduler with SkipIfStillRunning wrapper
**When to use:** Application startup for scheduled mode (SCHD-01, SCHD-03)
**Example:**
```go
// Source: Context7 /robfig/cron - SkipIfStillRunning pattern
package scheduler

import (
	"log/slog"

	"github.com/robfig/cron/v3"
)

// Scheduler wraps the cron scheduler with application-specific configuration
type Scheduler struct {
	cron   *cron.Cron
	logger *slog.Logger
}

// New creates a new scheduler with SkipIfStillRunning mode enabled
func New(logger *slog.Logger) *Scheduler {
	// Create cron-compatible logger adapter
	cronLogger := cron.VerbosePrintfLogger(
		logger.With("component", "scheduler"),
	)

	// Initialize cron with SkipIfStillRunning wrapper
	c := cron.New(
		cron.WithChain(
			cron.SkipIfStillRunning(cronLogger),
		),
		cron.WithLogger(cronLogger),
	)

	return &Scheduler{
		cron:   c,
		logger: logger,
	}
}

// AddJob registers a job function to run on the given cron schedule
func (s *Scheduler) AddJob(cronExpr string, jobFunc func()) error {
	_, err := s.cron.AddFunc(cronExpr, jobFunc)
	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}
	s.logger.Info("Scheduled job registered", "schedule", cronExpr)
	return nil
}

// Start begins the scheduler
func (s *Scheduler) Start() {
	s.logger.Info("Starting scheduler")
	s.cron.Start()
}

// Stop gracefully stops the scheduler and waits for running jobs
func (s *Scheduler) Stop() {
	s.logger.Info("Stopping scheduler, waiting for running jobs...")
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.logger.Info("Scheduler stopped")
}
```

### Pattern 2: Pushover Notifier with Graceful Missing Config
**What:** Notifier that reads config from env vars, warns if missing, sends on failure
**When to use:** Sending failure notifications (NOTF-01, NOTF-02, NOTF-03, NOTF-04)
**Example:**
```go
// Source: github.com/gregdel/pushover + Pushover API docs
package notifier

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/gregdel/pushover"
)

// Notifier sends push notifications
type Notifier struct {
	client    *pushover.Pushover
	recipient *pushover.Recipient
	logger    *slog.Logger
	enabled   bool
}

// New creates a new notifier from environment variables
// Returns a disabled notifier if env vars are missing (logs warning, doesn't fail)
func New(logger *slog.Logger) *Notifier {
	token := os.Getenv("PUSHOVER_TOKEN")
	user := os.Getenv("PUSHOVER_USER")

	if token == "" || user == "" {
		logger.Warn("Pushover notification disabled: PUSHOVER_TOKEN and/or PUSHOVER_USER not set")
		return &Notifier{enabled: false, logger: logger}
	}

	logger.Info("Pushover notifications enabled")
	return &Notifier{
		client:    pushover.New(token),
		recipient: pushover.NewRecipient(user),
		logger:    logger,
		enabled:   true,
	}
}

// Notify sends a notification with title and message
func (n *Notifier) Notify(title, message string) error {
	if !n.enabled {
		n.logger.Debug("Notification skipped (disabled)", "title", title)
		return nil
	}

	msg := pushover.NewMessageWithTitle(message, title)
	response, err := n.client.SendMessage(msg, n.recipient)
	if err != nil {
		return fmt.Errorf("failed to send pushover notification: %w", err)
	}

	n.logger.Info("Notification sent", "title", title, "response", response.ID)
	return nil
}

// NotifyFailure sends a failure notification with error details
func (n *Notifier) NotifyFailure(operation string, err error) error {
	title := fmt.Sprintf("Nanobot Update Failed: %s", operation)
	message := fmt.Sprintf("Error: %v", err)
	return n.Notify(title, message)
}
```

### Pattern 3: Main.go Integration
**What:** Wire scheduler and notifier into main application flow
**When to use:** Scheduled mode execution (non-run-once)
**Example:**
```go
// cmd/main.go - Replace existing TODO: Phase 3 section
func main() {
	// ... existing flag parsing, config loading, logger setup ...

	// Initialize notifier (logs warning if not configured)
	notif := notifier.New(logger)

	// Initialize scheduler with logger
	sched := scheduler.New(logger)

	// Create updater instance
	u := updater.NewUpdater(logger)

	// Add update job to scheduler
	err := sched.AddJob(cfg.Cron, func() {
		logger.Info("Starting scheduled update")
		result, err := u.Update(context.Background())
		if err != nil {
			logger.Error("Scheduled update failed", "result", result, "error", err)
			// Send failure notification
			if notifyErr := notif.NotifyFailure("Scheduled Update", err); notifyErr != nil {
				logger.Error("Failed to send notification", "error", notifyErr)
			}
			return
		}
		logger.Info("Scheduled update completed", "result", result)
	})
	if err != nil {
		logger.Error("Failed to schedule update job", "error", err)
		os.Exit(1)
	}

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start scheduler
	sched.Start()
	logger.Info("Scheduler started", "cron", cfg.Cron)

	// Wait for shutdown signal
	<-sigChan
	logger.Info("Shutdown signal received")

	// Graceful shutdown
	sched.Stop()
	logger.Info("Application stopped")
}
```

### Anti-Patterns to Avoid
- **Don't use cron.New() without WithChain:** Jobs will overlap if previous run hasn't completed
- **Don't fail if Pushover env vars missing:** Requirement NOTF-04 explicitly says log warning only
- **Don't use cron.WithoutSeconds():** Default parser already uses 5-field format (minute, hour, dom, month, dow)
- **Don't call sched.Stop() without waiting:** Use `ctx := sched.Stop(); <-ctx.Done()` to wait for running jobs

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Job overlap prevention | Manual mutex/flag tracking | cron.SkipIfStillRunning | Handles race conditions, provides logging |
| Cron expression parsing | Custom parser | cron.NewParser(cron.Minute\|cron.Hour\|...) | Already in use for config validation |
| Pushover HTTP API | Manual HTTP POST | gregdel/pushover | Handles authentication, error responses, retry |
| Graceful shutdown | Manual goroutine tracking | cron.Stop() returning context | Properly waits for in-flight jobs |

**Key insight:** Both robfig/cron and gregdel/pushover provide all needed functionality. No custom logic required beyond configuration wiring.

## Common Pitfalls

### Pitfall 1: SkipIfStillRunning Logger Incompatibility
**What goes wrong:** Passing slog.Logger directly to SkipIfStillRunning causes compilation error
**Why it happens:** cron.SkipIfStillRunning expects cron.Logger interface, not slog.Logger
**How to avoid:** Wrap slog with cron.VerbosePrintfLogger:
```go
cronLogger := cron.VerbosePrintfLogger(logger.With("component", "scheduler"))
c := cron.New(cron.WithChain(cron.SkipIfStillRunning(cronLogger)))
```
**Warning signs:** Compilation error: "cannot use logger (variable of type *slog.Logger) as cron.Logger value"

### Pitfall 2: Not Checking Pushover Env Vars Before Scheduler Start
**What goes wrong:** Application fails to send notifications but no early warning given
**Why it happens:** Env vars checked only when notification is sent, not at startup
**How to avoid:** Check and log warning in notifier.New() at application startup
**Warning signs:** No "Pushover notification disabled" warning in logs at startup

### Pitfall 3: Forgetting to Wait for Scheduler Stop
**What goes wrong:** Application exits while update job is still running, causing partial updates
**Why it happens:** cron.Stop() is non-blocking, returns immediately with context
**How to avoid:** Always wait for context after calling Stop():
```go
ctx := sched.Stop()
<-ctx.Done() // Wait for running jobs to complete
```
**Warning signs:** Updates interrupted on Ctrl+C, log shows "Stopping scheduler" but not "Scheduler stopped"

### Pitfall 4: Missing Error Return from AddFunc
**What goes wrong:** Invalid cron expression silently ignored, job never runs
**Why it happens:** AddFunc returns EntryID and error, error easily ignored
**How to avoid:** Always check error from AddFunc/AddJob:
```go
_, err := c.AddFunc(cfg.Cron, jobFunc)
if err != nil {
    return fmt.Errorf("invalid cron expression: %w", err)
}
```
**Warning signs:** Scheduled job never executes, no error in logs

### Pitfall 5: Notification Blocking Update Completion
**What goes wrong:** Pushover API timeout delays next scheduled update
**Why it happens:** SendMessage is synchronous and can take seconds if API is slow
**How to avoid:** Consider async notification (optional enhancement):
```go
go func() {
    if err := notif.NotifyFailure("Update", err); err != nil {
        logger.Error("Notification failed", "error", err)
    }
}()
```
**Warning signs:** Log timestamps show notification takes > 5 seconds

## Code Examples

### Complete Scheduler Package

```go
// internal/scheduler/scheduler.go
package scheduler

import (
	"fmt"
	"log/slog"

	"github.com/robfig/cron/v3"
)

// Scheduler manages cron-based job execution with overlap prevention
type Scheduler struct {
	cron   *cron.Cron
	logger *slog.Logger
}

// New creates a new scheduler with SkipIfStillRunning mode
func New(logger *slog.Logger) *Scheduler {
	// Create cron-compatible logger from slog
	cronLogger := cron.VerbosePrintfLogger(
		logger.With("component", "scheduler"),
	)

	// Initialize cron with job wrapper chain
	c := cron.New(
		cron.WithChain(
			cron.SkipIfStillRunning(cronLogger),
		),
		cron.WithLogger(cronLogger),
	)

	return &Scheduler{
		cron:   c,
		logger: logger,
	}
}

// AddJob registers a function to run on the given cron schedule
func (s *Scheduler) AddJob(cronExpr string, jobFunc func()) error {
	_, err := s.cron.AddFunc(cronExpr, jobFunc)
	if err != nil {
		return fmt.Errorf("failed to add cron job with schedule %q: %w", cronExpr, err)
	}
	s.logger.Info("Job scheduled", "schedule", cronExpr)
	return nil
}

// Start begins the scheduler
func (s *Scheduler) Start() {
	s.logger.Info("Starting scheduler")
	s.cron.Start()
}

// Stop gracefully stops the scheduler and waits for running jobs to complete
func (s *Scheduler) Stop() {
	s.logger.Info("Stopping scheduler")
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.logger.Info("Scheduler stopped, all jobs completed")
}
```

### Complete Notifier Package

```go
// internal/notifier/notifier.go
package notifier

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/gregdel/pushover"
)

// Notifier handles push notification sending via Pushover
type Notifier struct {
	client    *pushover.Pushover
	recipient *pushover.Recipient
	logger    *slog.Logger
	enabled   bool
}

// New creates a notifier from environment variables
// If PUSHOVER_TOKEN or PUSHOVER_USER are not set, returns a disabled notifier
// that logs warnings instead of failing
func New(logger *slog.Logger) *Notifier {
	token := os.Getenv("PUSHOVER_TOKEN")
	user := os.Getenv("PUSHOVER_USER")

	if token == "" || user == "" {
		logger.Warn("Pushover notifications disabled",
			"reason", "PUSHOVER_TOKEN and/or PUSHOVER_USER environment variables not set",
			"hint", "Set both variables to enable failure notifications")
		return &Notifier{
			enabled: false,
			logger:  logger,
		}
	}

	logger.Info("Pushover notifications enabled")
	return &Notifier{
		client:    pushover.New(token),
		recipient: pushover.NewRecipient(user),
		logger:    logger,
		enabled:   true,
	}
}

// IsEnabled returns whether notifications are configured
func (n *Notifier) IsEnabled() bool {
	return n.enabled
}

// Notify sends a notification with the given title and message
// Returns nil if notifications are disabled (no error)
func (n *Notifier) Notify(title, message string) error {
	if !n.enabled {
		n.logger.Debug("Notification skipped (not configured)", "title", title)
		return nil
	}

	msg := pushover.NewMessageWithTitle(message, title)
	response, err := n.client.SendMessage(msg, n.recipient)
	if err != nil {
		n.logger.Error("Failed to send notification",
			"title", title,
			"error", err)
		return fmt.Errorf("pushover notification failed: %w", err)
	}

	n.logger.Info("Notification sent successfully",
		"title", title,
		"id", response.ID)
	return nil
}

// NotifyFailure is a convenience method for sending failure notifications
func (n *Notifier) NotifyFailure(operation string, err error) error {
	title := fmt.Sprintf("Nanobot Update Failed: %s", operation)
	message := fmt.Sprintf("Operation: %s\n\nError: %v", operation, err)
	return n.Notify(title, message)
}
```

### Main.go Scheduled Mode Integration

```go
// cmd/main.go - Complete scheduled mode section
// (Replace lines 97-98: "TODO: Phase 3 - Implement scheduling")

import (
	"os/signal"
	"syscall"

	"github.com/HQGroup/nanobot-auto-updater/internal/notifier"
	"github.com/HQGroup/nanobot-auto-updater/internal/scheduler"
)

// ... in main() after run-once check:

	// Initialize notifier (logs warning if Pushover not configured)
	notif := notifier.New(logger)

	// Initialize scheduler with overlap prevention
	sched := scheduler.New(logger)

	// Create updater instance
	u := updater.NewUpdater(logger)

	// Register the update job
	err = sched.AddJob(cfg.Cron, func() {
		logger.Info("Starting scheduled update job")

		result, err := u.Update(context.Background())
		if err != nil {
			logger.Error("Scheduled update failed",
				"result", result,
				"error", err)

			// Send failure notification
			if notifyErr := notif.NotifyFailure("Scheduled Update", err); notifyErr != nil {
				logger.Error("Failed to send failure notification", "error", notifyErr)
			}
			return
		}

		logger.Info("Scheduled update completed successfully", "result", result)
	})
	if err != nil {
		logger.Error("Failed to register scheduled job", "error", err)
		os.Exit(1)
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the scheduler
	sched.Start()
	logger.Info("Scheduler started", "cron", cfg.Cron, "pid", os.Getpid())

	// Wait for shutdown signal
	sig := <-sigChan
	logger.Info("Shutdown signal received", "signal", sig)

	// Gracefully stop scheduler
	sched.Stop()
	logger.Info("Application shutdown complete")
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manual mutex for overlap prevention | cron.WithChain(SkipIfStillRunning) | robfig/cron v3 (2019) | Built-in, tested, logged |
| Global cron scheduler | Dependency-injected scheduler | Go best practices | Testable, configurable |
| Multiple notification libraries | gregdel/pushover standard | Go ecosystem maturity | Single well-maintained library |
| Blocking on shutdown | Context-based graceful shutdown | robfig/cron v3 | Clean job completion |

**Deprecated/outdated:**
- robfig/cron v1/v2: Lacks job wrappers, Go modules support - use v3
- Direct HTTP to Pushover API: More error-prone than gregdel/pushover library

## Open Questions

1. **Notification timeout handling**
   - What we know: Pushover API can be slow or timeout
   - What's unclear: Should notifications have a context timeout?
   - Recommendation: For now, use default client behavior. Add context timeout in future if API latency causes issues.

2. **Notification on first successful run**
   - What we know: Requirements only specify failure notifications
   - What's unclear: Should we notify on successful updates too?
   - Recommendation: No - only notify on failures as per NOTF-02. Success logging is sufficient.

3. **Cron expression validation at startup**
   - What we know: config.Validate() already validates cron expression
   - What's unclear: Should scheduler.AddJob also validate?
   - Recommendation: config validation is sufficient. AddJob error handling covers edge cases.

## Sources

### Primary (HIGH confidence)
- /robfig/cron (Context7) - SkipIfStillRunning, WithChain, job management, graceful shutdown
- https://pushover.net/api - Official Pushover API documentation (message parameters, error handling)
- https://github.com/gregdel/pushover - Official Go Pushover library documentation and examples
- https://pkg.go.dev/github.com/robfig/cron/v3 - Cron package reference

### Secondary (MEDIUM confidence)
- https://pkg.go.dev/github.com/gregdel/pushover - Pushover Go package documentation
- Project source: internal/config/config.go - Existing cron validation pattern
- Project source: cmd/main.go - Existing run-once mode pattern for reference

### Tertiary (LOW confidence)
- None required - all core functionality verified through primary sources

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - robfig/cron already in project, gregdel/pushover is well-documented
- Architecture: HIGH - Clear patterns from Context7 and official docs
- Pitfalls: HIGH - Well-documented cron v3 behavior, Pushover API errors documented

**Research date:** 2026-02-18
**Valid until:** 30 days - Both libraries are stable, patterns are mature

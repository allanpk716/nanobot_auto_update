package instance

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/lifecycle"
)

// InstanceLifecycle wraps lifecycle operations with instance-specific context.
// Each instance has its own logger with instance name pre-injected for traceability.
type InstanceLifecycle struct {
	config config.InstanceConfig
	logger *slog.Logger
}

// NewInstanceLifecycle creates an instance lifecycle manager with context-aware logging.
// The logger is enriched with instance name and component fields.
func NewInstanceLifecycle(cfg config.InstanceConfig, baseLogger *slog.Logger) *InstanceLifecycle {
	// Inject instance context into logger for all log messages
	instanceLogger := baseLogger.With("instance", cfg.Name).With("component", "instance-lifecycle")

	return &InstanceLifecycle{
		config: cfg,
		logger: instanceLogger,
	}
}

// StopForUpdate stops the instance before update.
// Returns nil if instance is not running (not an error).
// Returns InstanceError if stop operation fails.
func (il *InstanceLifecycle) StopForUpdate(ctx context.Context) error {
	il.logger.Info("Starting stop-before-update process")

	// Detect if instance is running
	running, pid, detectionMethod, err := lifecycle.IsNanobotRunning(il.config.Port)
	if err != nil {
		il.logger.Error("Failed to detect instance", "error", err)
		return &InstanceError{
			InstanceName: il.config.Name,
			Operation:    "stop",
			Port:         il.config.Port,
			Err:          fmt.Errorf("failed to detect instance: %w", err),
		}
	}

	if !running {
		il.logger.Info("Instance not running, nothing to stop")
		return nil
	}

	il.logger.Info("Found running instance", "pid", pid, "detection_method", detectionMethod)

	// Stop the instance using lifecycle package
	stopTimeout := 5 * time.Second // Locked decision: 5 second timeout
	if err := lifecycle.StopNanobot(ctx, pid, stopTimeout, il.logger); err != nil {
		il.logger.Error("Failed to stop instance", "pid", pid, "error", err)
		return &InstanceError{
			InstanceName: il.config.Name,
			Operation:    "stop",
			Port:         il.config.Port,
			Err:          fmt.Errorf("failed to stop instance (PID %d): %w", pid, err),
		}
	}

	il.logger.Info("Instance stopped successfully", "pid", pid)
	return nil
}

// StartAfterUpdate starts the instance after update.
// Uses instance-specific command and port configuration.
// Returns InstanceError if start operation fails.
func (il *InstanceLifecycle) StartAfterUpdate(ctx context.Context) error {
	il.logger.Info("Starting instance after update")

	// Handle default startup timeout
	startupTimeout := il.config.StartupTimeout
	if startupTimeout == 0 {
		startupTimeout = 30 * time.Second // Default: 30 seconds
		il.logger.Debug("Using default startup timeout", "timeout", startupTimeout)
	}

	// Start the instance using lifecycle package with instance-specific command and port
	if err := lifecycle.StartNanobot(ctx, il.config.StartCommand, il.config.Port, startupTimeout, il.logger); err != nil {
		il.logger.Error("Failed to start instance", "error", err)
		return &InstanceError{
			InstanceName: il.config.Name,
			Operation:    "start",
			Port:         il.config.Port,
			Err:          fmt.Errorf("failed to start instance: %w", err),
		}
	}

	il.logger.Info("Instance started successfully")
	return nil
}

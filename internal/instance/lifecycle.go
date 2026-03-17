package instance

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/lifecycle"
	"github.com/HQGroup/nanobot-auto-updater/internal/logbuffer"
)

// InstanceLifecycle wraps lifecycle operations with instance-specific context.
// Each instance has its own logger with instance name pre-injected for traceability.
// INST-01: Each instance has its own LogBuffer for log capture
type InstanceLifecycle struct {
	config    config.InstanceConfig
	logger    *slog.Logger
	logBuffer *logbuffer.LogBuffer // INST-01: LogBuffer for this instance
}

// NewInstanceLifecycle creates an instance lifecycle manager with context-aware logging.
// The logger is enriched with instance name and component fields.
// INST-01: Creates LogBuffer for this instance
func NewInstanceLifecycle(cfg config.InstanceConfig, baseLogger *slog.Logger) *InstanceLifecycle {
	// Inject instance context into logger for all log messages
	instanceLogger := baseLogger.With("instance", cfg.Name).With("component", "instance-lifecycle")

	// INST-01: Create LogBuffer for this instance
	logBuffer := logbuffer.NewLogBuffer(instanceLogger)

	return &InstanceLifecycle{
		config:    cfg,
		logger:    instanceLogger,
		logBuffer: logBuffer,
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

// GetLogBuffer returns the instance's LogBuffer.
// INST-01: Used by InstanceManager to access instance buffers
func (il *InstanceLifecycle) GetLogBuffer() *logbuffer.LogBuffer {
	return il.logBuffer
}

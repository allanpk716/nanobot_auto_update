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
// Uses PID-based process management instead of port detection.
type InstanceLifecycle struct {
	config    config.InstanceConfig
	logger    *slog.Logger
	logBuffer *logbuffer.LogBuffer // INST-01: LogBuffer for this instance
	pid       int32                // Process ID of the running instance (0 if not running)
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
// Uses PID-based process management: if pid > 0, stops the process directly by PID.
func (il *InstanceLifecycle) StopForUpdate(ctx context.Context) error {
	il.logger.Info("Starting stop-before-update process")

	// If we don't have a PID, the instance was never started
	if il.pid == 0 {
		il.logger.Info("Instance never started, nothing to stop")
		return nil
	}

	il.logger.Info("Stopping instance by PID", "pid", il.pid)

	// Stop the instance using the saved PID
	stopTimeout := 5 * time.Second // Locked decision: 5 second timeout
	if err := lifecycle.StopNanobot(ctx, il.pid, stopTimeout, il.logger); err != nil {
		il.logger.Error("Failed to stop instance", "pid", il.pid, "error", err)
		return &InstanceError{
			InstanceName: il.config.Name,
			Operation:    "stop",
			Port:         il.config.Port,
			Err:          fmt.Errorf("failed to stop instance (PID %d): %w", il.pid, err),
		}
	}

	// Clear the PID after successful stop
	il.pid = 0
	il.logger.Info("Instance stopped successfully")
	return nil
}

// StartAfterUpdate starts the instance after update.
// Uses instance-specific command and port configuration.
// Returns InstanceError if start operation fails.
// INST-05: Clears LogBuffer before starting
// INST-03: Uses StartNanobotWithCapture with instance's LogBuffer
// Saves the PID for future process management.
func (il *InstanceLifecycle) StartAfterUpdate(ctx context.Context) error {
	il.logger.Info("Starting instance after update")

	// INST-05: Clear LogBuffer on restart (fresh start)
	il.logBuffer.Clear()

	// Handle default startup timeout
	startupTimeout := il.config.StartupTimeout
	if startupTimeout == 0 {
		startupTimeout = 30 * time.Second // Default: 30 seconds
		il.logger.Debug("Using default startup timeout", "timeout", startupTimeout)
	}

	// Start the instance using lifecycle package with instance-specific command and port
	// INST-03: Use StartNanobotWithCapture with instance's LogBuffer
	pid, err := lifecycle.StartNanobotWithCapture(ctx, il.config.StartCommand, il.config.Port, startupTimeout, il.logger, il.logBuffer)
	if err != nil {
		il.logger.Error("Failed to start instance", "error", err)
		return &InstanceError{
			InstanceName: il.config.Name,
			Operation:    "start",
			Port:         il.config.Port,
			Err:          fmt.Errorf("failed to start instance: %w", err),
		}
	}

	// Save the PID for future process management
	il.pid = int32(pid)
	il.logger.Info("Instance started successfully with log capture", "pid", pid)
	return nil
}

// GetLogBuffer returns the instance's LogBuffer.
// INST-01: Used by InstanceManager to access instance buffers
func (il *InstanceLifecycle) GetLogBuffer() *logbuffer.LogBuffer {
	return il.logBuffer
}

// Name returns the instance name.
// AUTOSTART-01: Helper method for accessing instance configuration
func (il *InstanceLifecycle) Name() string {
	return il.config.Name
}

// Port returns the instance port.
// AUTOSTART-01: Helper method for accessing instance configuration
func (il *InstanceLifecycle) Port() uint32 {
	return il.config.Port
}

// ShouldAutoStart returns whether the instance should be automatically started.
// AUTOSTART-01: Delegates to InstanceConfig.ShouldAutoStart()
func (il *InstanceLifecycle) ShouldAutoStart() bool {
	return il.config.ShouldAutoStart()
}

// IsRunning checks if the instance is currently running by checking if the process exists.
// Uses PID-based process management: returns true if pid > 0 and process exists.
func (il *InstanceLifecycle) IsRunning() bool {
	if il.pid == 0 {
		return false // Never started
	}

	// Check if process exists using gopsutil
	proc, err := lifecycle.FindProcessByPID(il.pid, il.logger)
	if err != nil || proc == nil {
		return false // Process doesn't exist
	}

	// Process exists and is running
	return true
}

// GetPID returns the process ID of the instance (0 if not running).
func (il *InstanceLifecycle) GetPID() int32 {
	return il.pid
}

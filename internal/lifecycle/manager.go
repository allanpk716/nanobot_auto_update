package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Manager orchestrates nanobot lifecycle (stop before update, start after update)
type Manager struct {
	port           uint32
	startupTimeout time.Duration
	stopTimeout    time.Duration
	logger         *slog.Logger
}

// Config holds lifecycle manager configuration
type Config struct {
	Port           uint32        `yaml:"port"`
	StartupTimeout time.Duration `yaml:"startup_timeout"`
}

// NewManager creates a new lifecycle manager
func NewManager(cfg Config, logger *slog.Logger) *Manager {
	return &Manager{
		port:           cfg.Port,
		startupTimeout: cfg.StartupTimeout,
		stopTimeout:    5 * time.Second, // Locked decision: 5 second timeout
		logger:         logger,
	}
}

// StopForUpdate stops nanobot before update.
// Returns error if stop fails - this should cancel the update.
func (m *Manager) StopForUpdate(ctx context.Context) error {
	m.logger.Info("Starting stop-before-update process")

	running, pid, detectionMethod, err := IsNanobotRunning(m.port)
	if err != nil {
		m.logger.Error("Failed to detect nanobot", "error", err)
		return fmt.Errorf("failed to detect nanobot: %w", err)
	}

	if !running {
		m.logger.Info("Nanobot not running, nothing to stop")
		return nil
	}

	m.logger.Info("Found running nanobot", "pid", pid, "detection_method", detectionMethod)

	if err := StopNanobot(ctx, pid, m.stopTimeout, m.logger); err != nil {
		m.logger.Error("Failed to stop nanobot", "pid", pid, "error", err)
		return fmt.Errorf("failed to stop nanobot (PID %d): %w", pid, err)
	}

	m.logger.Info("Nanobot stopped successfully", "pid", pid)
	return nil
}

// StartAfterUpdate starts nanobot after update.
// Returns error if start fails, but update is still considered successful.
// Caller should log the error but not fail the update.
func (m *Manager) StartAfterUpdate(ctx context.Context) error {
	m.logger.Info("Starting nanobot after update")

	// Always start regardless of previous state (locked decision)
	if err := StartNanobot(ctx, m.startupTimeout, m.logger); err != nil {
		m.logger.Error("Failed to start nanobot", "error", err)
		return fmt.Errorf("failed to start nanobot (user can start manually): %w", err)
	}

	m.logger.Info("Nanobot started successfully")
	return nil
}

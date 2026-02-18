package lifecycle

import (
	"context"
	"fmt"
	"time"
)

// Manager orchestrates nanobot lifecycle (stop before update, start after update)
type Manager struct {
	port           uint32
	startupTimeout time.Duration
	stopTimeout    time.Duration
}

// Config holds lifecycle manager configuration
type Config struct {
	Port           uint32        `yaml:"port"`
	StartupTimeout time.Duration `yaml:"startup_timeout"`
}

// NewManager creates a new lifecycle manager
func NewManager(cfg Config) *Manager {
	return &Manager{
		port:           cfg.Port,
		startupTimeout: cfg.StartupTimeout,
		stopTimeout:    5 * time.Second, // Locked decision: 5 second timeout
	}
}

// StopForUpdate stops nanobot before update.
// Returns error if stop fails - this should cancel the update.
func (m *Manager) StopForUpdate(ctx context.Context) error {
	running, pid, err := IsNanobotRunning(m.port)
	if err != nil {
		return fmt.Errorf("failed to detect nanobot: %w", err)
	}

	if !running {
		// Not running, nothing to stop
		return nil
	}

	if err := StopNanobot(ctx, pid, m.stopTimeout); err != nil {
		return fmt.Errorf("failed to stop nanobot (PID %d): %w", pid, err)
	}

	return nil
}

// StartAfterUpdate starts nanobot after update.
// Returns error if start fails, but update is still considered successful.
// Caller should log the error but not fail the update.
func (m *Manager) StartAfterUpdate(ctx context.Context) error {
	// Always start regardless of previous state (locked decision)
	if err := StartNanobot(ctx, m.port, m.startupTimeout); err != nil {
		return fmt.Errorf("failed to start nanobot (user can start manually): %w", err)
	}
	return nil
}

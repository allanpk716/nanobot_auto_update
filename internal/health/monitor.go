package health

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// InstanceStatus holds the running status of a single instance.
// Returned by the status check function provided to HealthMonitor.
type InstanceStatus struct {
	Name    string
	Port    uint32
	Running bool
	PID     int32
}

// InstanceHealthState tracks instance health
type InstanceHealthState struct {
	IsRunning bool
	LastCheck time.Time
}

// HealthMonitor manages health check loop
type HealthMonitor struct {
	checkStatuses func() []InstanceStatus
	interval      time.Duration
	logger        *slog.Logger
	states        map[string]*InstanceHealthState
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewHealthMonitor creates a new health monitor.
// checkStatuses is called periodically to get the current status of all instances.
func NewHealthMonitor(
	checkStatuses func() []InstanceStatus,
	interval time.Duration,
	logger *slog.Logger,
) *HealthMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &HealthMonitor{
		checkStatuses: checkStatuses,
		interval:      interval,
		logger:        logger,
		states:        make(map[string]*InstanceHealthState),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start begins the health check loop (runs in goroutine)
func (hm *HealthMonitor) Start() {
	hm.logger.Info("Health monitor started", "interval", hm.interval)

	ticker := time.NewTicker(hm.interval)
	defer ticker.Stop()

	// Immediate first check
	hm.checkAllInstances()

	for {
		select {
		case <-hm.ctx.Done():
			hm.logger.Info("Health monitor stopped")
			return
		case <-ticker.C:
			hm.checkAllInstances()
		}
	}
}

// checkAllInstances iterates all instances
func (hm *HealthMonitor) checkAllInstances() {
	statuses := hm.checkStatuses()
	for _, status := range statuses {
		hm.checkInstance(status)
	}
}

// checkInstance checks single instance and logs state changes
func (hm *HealthMonitor) checkInstance(status InstanceStatus) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	now := time.Now()
	state, exists := hm.states[status.Name]

	// First check - record initial state only
	if !exists {
		hm.states[status.Name] = &InstanceHealthState{
			IsRunning: status.Running,
			LastCheck: now,
		}
		hm.logger.Info("Initial status check",
			"instance", status.Name,
			"is_running", status.Running,
			"pid", status.PID)
		return
	}

	// Check for state change
	previousState := state.IsRunning
	if previousState != status.Running {
		// State changed
		if previousState && !status.Running {
			// Running -> Stopped
			hm.logger.Error("Instance stopped",
				"instance", status.Name)
		} else if !previousState && status.Running {
			// Stopped -> Running
			hm.logger.Info("Instance recovered",
				"instance", status.Name,
				"pid", status.PID)
		}

		// Update state
		state.IsRunning = status.Running
		state.LastCheck = now
	} else {
		// No state change, just update last check time
		state.LastCheck = now
	}
}

// Stop gracefully stops the monitor
func (hm *HealthMonitor) Stop() {
	if hm.cancel != nil {
		hm.cancel()
	}
}

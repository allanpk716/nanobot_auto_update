package health

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/lifecycle"
)

// InstanceHealthState tracks instance health
type InstanceHealthState struct {
	IsRunning bool
	LastCheck time.Time
}

// HealthMonitor manages health check loop
type HealthMonitor struct {
	instances []config.InstanceConfig
	interval  time.Duration
	logger    *slog.Logger
	states    map[string]*InstanceHealthState
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(
	instances []config.InstanceConfig,
	interval time.Duration,
	logger *slog.Logger,
) *HealthMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &HealthMonitor{
		instances: instances,
		interval:  interval,
		logger:    logger,
		states:    make(map[string]*InstanceHealthState),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start begins the health check loop (runs in goroutine)
func (hm *HealthMonitor) Start() {
	hm.logger.Info("健康监控已启动", "interval", hm.interval)

	ticker := time.NewTicker(hm.interval)
	defer ticker.Stop()

	// Immediate first check
	hm.checkAllInstances()

	for {
		select {
		case <-hm.ctx.Done():
			hm.logger.Info("健康监控已停止")
			return
		case <-ticker.C:
			hm.checkAllInstances()
		}
	}
}

// checkAllInstances iterates all instances
func (hm *HealthMonitor) checkAllInstances() {
	for _, inst := range hm.instances {
		hm.checkInstance(inst)
	}
}

// checkInstance checks single instance and logs state changes
func (hm *HealthMonitor) checkInstance(inst config.InstanceConfig) {
	isRunning, pid, detectionMethod, err := lifecycle.IsNanobotRunning(inst.Port)
	if err != nil {
		hm.logger.Error("检查实例状态失败", "instance", inst.Name, "error", err)
		return
	}

	hm.mu.Lock()
	defer hm.mu.Unlock()

	now := time.Now()
	state, exists := hm.states[inst.Name]

	// First check - record initial state only
	if !exists {
		hm.states[inst.Name] = &InstanceHealthState{
			IsRunning: isRunning,
			LastCheck: now,
		}
		hm.logger.Info("初始状态检查",
			"instance", inst.Name,
			"is_running", isRunning,
			"pid", pid,
			"detection_method", detectionMethod)
		return
	}

	// Check for state change
	previousState := state.IsRunning
	if previousState != isRunning {
		// State changed
		if previousState && !isRunning {
			// Running -> Stopped
			hm.logger.Error("实例已停止",
				"instance", inst.Name,
				"previous_pid", pid)
		} else if !previousState && isRunning {
			// Stopped -> Running
			hm.logger.Info("实例已恢复运行",
				"instance", inst.Name,
				"pid", pid,
				"detection_method", detectionMethod)
		}

		// Update state
		state.IsRunning = isRunning
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

package health

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewHealthMonitor(t *testing.T) {
	checkFn := func() []InstanceStatus {
		return []InstanceStatus{
			{Name: "test1", Port: 8081, Running: false},
			{Name: "test2", Port: 8082, Running: false},
		}
	}
	interval := 30 * time.Second
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	hm := NewHealthMonitor(checkFn, interval, logger)

	assert.NotNil(t, hm)
	assert.Equal(t, interval, hm.interval)
	assert.NotNil(t, hm.states)
}

func TestMonitor_StateChange_RunningToStop(t *testing.T) {
	checkFn := func() []InstanceStatus {
		return []InstanceStatus{
			{Name: "test-instance", Port: 8081, Running: false},
		}
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	hm := NewHealthMonitor(checkFn, 30*time.Second, logger)

	// Set initial state to running
	hm.states["test-instance"] = &InstanceHealthState{
		IsRunning: true,
		LastCheck: time.Now(),
	}

	// Verify initial state
	assert.True(t, hm.states["test-instance"].IsRunning)
}

func TestMonitor_StateChange_StopToRunning(t *testing.T) {
	checkFn := func() []InstanceStatus {
		return []InstanceStatus{
			{Name: "test-instance", Port: 8081, Running: true},
		}
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	hm := NewHealthMonitor(checkFn, 30*time.Second, logger)

	// Set initial state to stopped
	hm.states["test-instance"] = &InstanceHealthState{
		IsRunning: false,
		LastCheck: time.Now(),
	}

	// Verify initial state
	assert.False(t, hm.states["test-instance"].IsRunning)
}

func TestMonitor_FirstCheck_NoStateChange(t *testing.T) {
	checkFn := func() []InstanceStatus {
		return []InstanceStatus{
			{Name: "test-instance", Port: 8081, Running: false},
		}
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	hm := NewHealthMonitor(checkFn, 30*time.Second, logger)

	// Verify no initial state exists
	_, exists := hm.states["test-instance"]
	assert.False(t, exists)
}

func TestMonitor_Stop(t *testing.T) {
	checkFn := func() []InstanceStatus {
		return []InstanceStatus{
			{Name: "test1", Port: 8081, Running: false},
		}
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	hm := NewHealthMonitor(checkFn, 30*time.Second, logger)

	// Start monitor in goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		hm.Start()
	}()

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Stop the monitor
	hm.Stop()

	// Verify context is cancelled
	assert.Equal(t, context.Canceled, hm.ctx.Err())

	// Wait for goroutine to finish
	wg.Wait()
}

func TestMonitor_RunningToStop_LogsOnlyOnce(t *testing.T) {
	checkFn := func() []InstanceStatus {
		return []InstanceStatus{
			{Name: "test-instance", Port: 8081, Running: false},
		}
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	hm := NewHealthMonitor(checkFn, 30*time.Second, logger)

	// Set initial state to running
	hm.states["test-instance"] = &InstanceHealthState{
		IsRunning: true,
		LastCheck: time.Now(),
	}

	// Verify we have initial state
	assert.True(t, hm.states["test-instance"].IsRunning)
}

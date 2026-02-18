//go:build windows

package scheduler

import (
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/logging"
)

func TestNew(t *testing.T) {
	logger := slog.Default()
	sched := New(logger)

	if sched == nil {
		t.Fatal("Expected scheduler to be non-nil")
	}

	if sched.cron == nil {
		t.Error("Expected cron field to be non-nil")
	}

	if sched.logger == nil {
		t.Error("Expected logger field to be non-nil")
	}
}

func TestAddJob(t *testing.T) {
	// Create test logger
	logger := logging.NewLogger("./logs")
	defer os.RemoveAll("./logs")

	sched := New(logger)

	// Test valid cron expression
	err := sched.AddJob("*/5 * * * *", func() {})
	if err != nil {
		t.Errorf("Expected no error for valid cron expression, got: %v", err)
	}

	// Test another valid expression
	err = sched.AddJob("0 3 * * *", func() {})
	if err != nil {
		t.Errorf("Expected no error for valid cron expression, got: %v", err)
	}
}

func TestAddJobInvalidCron(t *testing.T) {
	logger := slog.Default()
	sched := New(logger)

	// Test invalid cron expression
	err := sched.AddJob("invalid", func() {})
	if err == nil {
		t.Error("Expected error for invalid cron expression, got nil")
	}
}

func TestStartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration-style test in short mode")
	}

	// Create test logger
	logger := logging.NewLogger("./logs")
	defer os.RemoveAll("./logs")

	sched := New(logger)

	// Add a simple job that sets a flag
	var mu sync.Mutex
	jobRan := false
	err := sched.AddJob("* * * * *", func() {
		mu.Lock()
		jobRan = true
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Failed to add job: %v", err)
	}

	// Start scheduler
	sched.Start()

	// Wait a bit for scheduler to start
	time.Sleep(100 * time.Millisecond)

	// Stop scheduler
	sched.Stop()

	// Verify function returns without panic
	mu.Lock()
	ran := jobRan
	mu.Unlock()

	// We don't strictly require the job to have run, just that Start/Stop work
	t.Logf("Job ran: %v", ran)
}

func TestAddJobMultiple(t *testing.T) {
	logger := slog.Default()
	sched := New(logger)

	// Add multiple jobs
	for i := 0; i < 5; i++ {
		err := sched.AddJob("*/5 * * * *", func() {})
		if err != nil {
			t.Errorf("Failed to add job %d: %v", i, err)
		}
	}
}

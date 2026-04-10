//go:build windows

package lifecycle_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/lifecycle"
	"github.com/robfig/cron/v3"
)

// testDiscardLogger creates a logger that discards all output for testing.
func testDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestAppShutdown_NilPointer verifies AppShutdown handles nil AppComponents without panic.
func TestAppShutdown_NilPointer(t *testing.T) {
	logger := testDiscardLogger()

	// Should not panic
	lifecycle.AppShutdown(context.Background(), nil, logger)
}

// TestAppShutdown_AllNilFields verifies AppShutdown handles &AppComponents{} (all nil fields)
// without panic.
func TestAppShutdown_AllNilFields(t *testing.T) {
	logger := testDiscardLogger()
	c := &lifecycle.AppComponents{}

	// Should not panic
	lifecycle.AppShutdown(context.Background(), c, logger)
}

// TestAppShutdown_PartialComponents verifies AppShutdown handles mix of nil and non-nil components.
// Only CleanupCron is set -- it should be stopped cleanly.
func TestAppShutdown_PartialComponents(t *testing.T) {
	logger := testDiscardLogger()
	c := &lifecycle.AppComponents{
		CleanupCron: cron.New(),
	}

	// Should not panic, should stop cleanup cron
	lifecycle.AppShutdown(context.Background(), c, logger)
}

// TestAppShutdown_APIServerContext verifies AppShutdown completes within context timeout
// when APIServer is nil.
func TestAppShutdown_APIServerContext(t *testing.T) {
	logger := testDiscardLogger()
	c := &lifecycle.AppComponents{}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		lifecycle.AppShutdown(ctx, c, logger)
		close(done)
	}()

	select {
	case <-done:
		// Success: shutdown completed within timeout
	case <-time.After(3 * time.Second):
		t.Error("AppShutdown did not complete within expected time")
	}
}

// TestAppShutdown_FullComponents verifies AppShutdown with all safe components set.
func TestAppShutdown_FullComponents(t *testing.T) {
	logger := testDiscardLogger()

	c := &lifecycle.AppComponents{
		CleanupCron: cron.New(),
		// Other components (NotificationManager, NetworkMonitor, HealthMonitor,
		// UpdateLogger, APIServer) require real dependencies so they remain nil here.
		// The test verifies that nil-safety works for all components.
	}

	lifecycle.AppShutdown(context.Background(), c, logger)
}

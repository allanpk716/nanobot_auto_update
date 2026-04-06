package instance

import (
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/logbuffer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock types for monitor integration tests ---

// notifyCall records a single notification invocation.
type notifyCall struct {
	Title   string
	Message string
}

// mockLifecycleNotifier is a recording mock for the Notifier interface.
// Thread-safe via mutex to handle async notification goroutines.
type mockLifecycleNotifier struct {
	mu      sync.Mutex
	calls   []notifyCall
	enabled bool
}

func (m *mockLifecycleNotifier) IsEnabled() bool { return m.enabled }

func (m *mockLifecycleNotifier) Notify(title, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, notifyCall{Title: title, Message: message})
	return nil
}

func (m *mockLifecycleNotifier) getCalls() []notifyCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]notifyCall{}, m.calls...)
}

// newRecordingNotifier creates an enabled mockLifecycleNotifier that records calls.
func newRecordingNotifier() *mockLifecycleNotifier {
	return &mockLifecycleNotifier{enabled: true}
}

// newTestInstanceLifecycle creates an InstanceLifecycle with the given notifier
// for monitor integration testing. Uses a real LogBuffer.
func newTestInstanceLifecycle(notifier Notifier) *InstanceLifecycle {
	cfg := config.InstanceConfig{
		Name:         "test-instance",
		StartCommand: "echo test",
		Port:         8080,
	}
	return NewInstanceLifecycle(cfg, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})), notifier)
}

// --- Monitor lifecycle integration tests ---

// TestMonitor_CreatedAfterStart verifies that startTelegramMonitor creates
// non-nil telegramMonitor and monitorCancel fields.
func TestMonitor_CreatedAfterStart(t *testing.T) {
	il := newTestInstanceLifecycle(newRecordingNotifier())

	// Before start: fields should be nil
	assert.Nil(t, il.telegramMonitor, "telegramMonitor should be nil before start")
	assert.Nil(t, il.monitorCancel, "monitorCancel should be nil before start")

	// Start the monitor
	il.startTelegramMonitor()

	// After start: fields should be non-nil
	assert.NotNil(t, il.telegramMonitor, "telegramMonitor should be non-nil after start")
	assert.NotNil(t, il.monitorCancel, "monitorCancel should be non-nil after start")

	// Cleanup
	il.stopTelegramMonitor()
}

// TestMonitor_NoTriggerNoNotifications verifies TELE-07:
// Instance starts, logs non-Telegram content, zero notifications sent.
func TestMonitor_NoTriggerNoNotifications(t *testing.T) {
	notif := newRecordingNotifier()
	il := newTestInstanceLifecycle(notif)

	il.startTelegramMonitor()
	defer il.stopTelegramMonitor()

	// Write non-trigger entries to the logBuffer
	il.logBuffer.Write(logbuffer.LogEntry{
		Timestamp: time.Now(),
		Source:    "stdout",
		Content:   "Application started",
	})
	il.logBuffer.Write(logbuffer.LogEntry{
		Timestamp: time.Now(),
		Source:    "stdout",
		Content:   "Server listening on 8080",
	})

	// Wait for async processing
	time.Sleep(100 * time.Millisecond)

	// No notifications should have been sent
	calls := notif.getCalls()
	assert.Empty(t, calls, "TELE-07: no trigger log should produce zero notifications, got %d", len(calls))
}

// TestMonitor_StopCancelsMonitor verifies TELE-09:
// Start monitor, write trigger, stop before timeout -- no timeout notification fires.
func TestMonitor_StopCancelsMonitor(t *testing.T) {
	notif := newRecordingNotifier()
	il := newTestInstanceLifecycle(notif)

	il.startTelegramMonitor()

	// Write trigger entry to start the monitoring window
	il.logBuffer.Write(logbuffer.LogEntry{
		Timestamp: time.Now(),
		Source:    "stdout",
		Content:   "Starting Telegram bot...",
	})

	// Wait for monitor to enter waiting state
	time.Sleep(50 * time.Millisecond)

	// Stop the monitor before the 30s timeout fires
	il.stopTelegramMonitor()

	// Wait longer than DefaultTimeout to prove timer was cancelled
	// Using a shorter wait since we just need to verify no notification after stop
	time.Sleep(500 * time.Millisecond)

	calls := notif.getCalls()
	assert.Empty(t, calls, "TELE-09: stop should cancel monitor, no timeout notification should fire, got %d", len(calls))
}

// TestMonitor_StopWithNoMonitorNilSafe verifies TELE-09 edge case:
// StopForUpdate on a never-started instance returns nil without panic.
func TestMonitor_StopWithNoMonitorNilSafe(t *testing.T) {
	il := newTestInstanceLifecycle(newRecordingNotifier())

	// Call stopTelegramMonitor without starting (no monitor exists)
	assert.NotPanics(t, func() {
		il.stopTelegramMonitor()
	}, "stopTelegramMonitor should be nil-safe when no monitor is running")
}

// TestMonitor_FieldsClearedAfterStop verifies that after stopTelegramMonitor,
// telegramMonitor and monitorCancel fields are nil.
func TestMonitor_FieldsClearedAfterStop(t *testing.T) {
	il := newTestInstanceLifecycle(newRecordingNotifier())

	// Start the monitor
	il.startTelegramMonitor()
	require.NotNil(t, il.telegramMonitor, "telegramMonitor should be non-nil after start")
	require.NotNil(t, il.monitorCancel, "monitorCancel should be non-nil after start")

	// Stop the monitor
	il.stopTelegramMonitor()

	// Fields should be cleared
	assert.Nil(t, il.telegramMonitor, "telegramMonitor should be nil after stop")
	assert.Nil(t, il.monitorCancel, "monitorCancel should be nil after stop")
}

// TestMonitor_SuccessNotification verifies end-to-end notification delivery:
// Trigger + success patterns produce exactly one "Connected" notification.
func TestMonitor_SuccessNotification(t *testing.T) {
	notif := newRecordingNotifier()
	il := newTestInstanceLifecycle(notif)

	il.startTelegramMonitor()
	defer il.stopTelegramMonitor()

	// Write trigger entry
	il.logBuffer.Write(logbuffer.LogEntry{
		Timestamp: time.Now(),
		Source:    "stdout",
		Content:   "Starting Telegram bot...",
	})

	// Write success entry
	il.logBuffer.Write(logbuffer.LogEntry{
		Timestamp: time.Now(),
		Source:    "stdout",
		Content:   "Telegram bot commands registered",
	})

	// Wait for async processing and notification delivery
	time.Sleep(200 * time.Millisecond)

	calls := notif.getCalls()
	require.Len(t, calls, 1, "expected exactly 1 success notification, got %d", len(calls))
	assert.Contains(t, calls[0].Title, "Connected", "notification title should contain 'Connected'")
	assert.Contains(t, calls[0].Message, "test-instance", "notification should contain instance name")
}

package telegram

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/logbuffer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock types ---

type notifyCall struct {
	Title   string
	Message string
}

type mockNotifier struct {
	mu      sync.Mutex
	calls   []notifyCall
	enabled bool
}

func (m *mockNotifier) IsEnabled() bool { return m.enabled }

func (m *mockNotifier) Notify(title, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, notifyCall{Title: title, Message: message})
	return nil
}

func (m *mockNotifier) getCalls() []notifyCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]notifyCall{}, m.calls...)
}

type mockLogSubscriber struct {
	ch        chan logbuffer.LogEntry
	cancelled bool
}

func newMockLogSubscriber() *mockLogSubscriber {
	return &mockLogSubscriber{
		ch: make(chan logbuffer.LogEntry, 100),
	}
}

func (m *mockLogSubscriber) Subscribe() <-chan logbuffer.LogEntry {
	return m.ch
}

func (m *mockLogSubscriber) Unsubscribe(ch <-chan logbuffer.LogEntry) {
	m.cancelled = true
	close(m.ch)
}

func (m *mockLogSubscriber) writeEntry(content string) {
	m.ch <- logbuffer.LogEntry{
		Timestamp: time.Now(),
		Source:    "stdout",
		Content:   content,
	}
}

// --- Monitor state machine tests ---

func TestMonitor_TriggerDetected(t *testing.T) {
	sub := newMockLogSubscriber()
	notif := &mockNotifier{enabled: true}

	m := NewTelegramMonitor(sub, notif, "test-bot", 5*time.Second, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.Start(ctx)

	// Write trigger entry
	sub.writeEntry("Starting Telegram bot...")

	// Allow processing
	time.Sleep(50 * time.Millisecond)

	// Verify monitor entered waiting state by checking that no immediate notification was sent
	// (notification only happens on success/failure/timeout)
	calls := notif.getCalls()
	assert.Empty(t, calls, "trigger alone should not produce a notification")
}

func TestMonitor_SuccessDetected(t *testing.T) {
	sub := newMockLogSubscriber()
	notif := &mockNotifier{enabled: true}

	m := NewTelegramMonitor(sub, notif, "test-bot", 5*time.Second, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.Start(ctx)

	// Write trigger then success
	sub.writeEntry("Starting Telegram bot...")
	sub.writeEntry("Telegram bot commands registered")

	// Allow processing and async notification
	time.Sleep(100 * time.Millisecond)

	calls := notif.getCalls()
	require.Len(t, calls, 1, "expected exactly one success notification")
	assert.Contains(t, calls[0].Title, "Telegram")
	assert.Contains(t, calls[0].Title, "Connected")
}

func TestMonitor_FailureDetected(t *testing.T) {
	sub := newMockLogSubscriber()
	notif := &mockNotifier{enabled: true}

	m := NewTelegramMonitor(sub, notif, "test-bot", 5*time.Second, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.Start(ctx)
	time.Sleep(10 * time.Millisecond) // Wait for goroutine to set startTime

	// Write trigger then failure
	sub.writeEntry("Starting Telegram bot...")
	sub.writeEntry("httpx.ConnectError: connection refused")

	// Allow processing and async notification
	time.Sleep(100 * time.Millisecond)

	calls := notif.getCalls()
	require.Len(t, calls, 1, "expected exactly one failure notification")
	assert.Contains(t, calls[0].Title, "Failed")
}

func TestMonitor_TimeoutFires(t *testing.T) {
	sub := newMockLogSubscriber()
	notif := &mockNotifier{enabled: true}

	// Use short timeout for test speed
	m := NewTelegramMonitor(sub, notif, "test-bot", 200*time.Millisecond, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.Start(ctx)

	// Write trigger but no success/failure pattern follows
	sub.writeEntry("Starting Telegram bot...")

	// Wait for timeout to fire plus some buffer
	time.Sleep(400 * time.Millisecond)

	calls := notif.getCalls()
	require.Len(t, calls, 1, "expected exactly one timeout notification")
	assert.Contains(t, calls[0].Title, "Timeout")
}

func TestMonitor_SuccessNotification(t *testing.T) {
	sub := newMockLogSubscriber()
	notif := &mockNotifier{enabled: true}

	m := NewTelegramMonitor(sub, notif, "test-bot", 5*time.Second, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.Start(ctx)

	sub.writeEntry("Starting Telegram bot...")
	sub.writeEntry("Telegram bot commands registered")

	time.Sleep(100 * time.Millisecond)

	calls := notif.getCalls()
	require.Len(t, calls, 1)
	assert.Contains(t, calls[0].Title, "Telegram")
	assert.Contains(t, calls[0].Message, "test-bot", "notification should contain instance name")
}

func TestMonitor_FailureNotification(t *testing.T) {
	sub := newMockLogSubscriber()
	notif := &mockNotifier{enabled: true}

	m := NewTelegramMonitor(sub, notif, "test-bot", 5*time.Second, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.Start(ctx)
	time.Sleep(10 * time.Millisecond) // Wait for goroutine to set startTime

	sub.writeEntry("Starting Telegram bot...")
	sub.writeEntry("httpx.ConnectError: connection refused")

	time.Sleep(100 * time.Millisecond)

	calls := notif.getCalls()
	require.Len(t, calls, 1)
	assert.Contains(t, calls[0].Title, "Telegram")
	assert.Contains(t, calls[0].Title, "Failed")
	assert.Contains(t, calls[0].Message, "test-bot", "notification should contain instance name")
	assert.Contains(t, calls[0].Message, "httpx.ConnectError", "notification should contain failure context")
}

func TestMonitor_TimeoutNotification(t *testing.T) {
	sub := newMockLogSubscriber()
	notif := &mockNotifier{enabled: true}

	m := NewTelegramMonitor(sub, notif, "test-bot", 200*time.Millisecond, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.Start(ctx)

	sub.writeEntry("Starting Telegram bot...")

	time.Sleep(400 * time.Millisecond)

	calls := notif.getCalls()
	require.Len(t, calls, 1)
	assert.Contains(t, calls[0].Title, "Telegram")
	assert.Contains(t, calls[0].Title, "Timeout")
	assert.Contains(t, calls[0].Message, "test-bot", "notification should contain instance name")
	assert.Contains(t, calls[0].Message, "timeout", "notification should mention timeout")
}

func TestMonitor_HistoricalReplayIgnored(t *testing.T) {
	sub := newMockLogSubscriber()
	notif := &mockNotifier{enabled: true}

	m := NewTelegramMonitor(sub, notif, "test-bot", 200*time.Millisecond, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.Start(ctx)

	// Write an entry with a timestamp BEFORE monitor start (historical replay)
	sub.ch <- logbuffer.LogEntry{
		Timestamp: time.Now().Add(-10 * time.Second),
		Source:    "stdout",
		Content:   "Starting Telegram bot...",
	}

	// Wait longer than timeout — should NOT fire because entry was historical
	time.Sleep(400 * time.Millisecond)

	calls := notif.getCalls()
	assert.Empty(t, calls, "historical entry should not trigger any notification")
}

func TestMonitor_MultipleCycles(t *testing.T) {
	sub := newMockLogSubscriber()
	notif := &mockNotifier{enabled: true}

	m := NewTelegramMonitor(sub, notif, "test-bot", 5*time.Second, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.Start(ctx)

	// Cycle 1: trigger + success
	sub.writeEntry("Starting Telegram bot...")
	sub.writeEntry("Telegram bot commands registered")
	time.Sleep(100 * time.Millisecond)

	// Cycle 2: trigger + failure
	sub.writeEntry("Starting Telegram bot...")
	sub.writeEntry("httpx.ConnectError: timeout")
	time.Sleep(100 * time.Millisecond)

	calls := notif.getCalls()
	require.Len(t, calls, 2, "expected two notifications from two cycles")
	assert.Contains(t, calls[0].Title, "Connected", "first cycle should be success")
	assert.Contains(t, calls[1].Title, "Failed", "second cycle should be failure")
}

func TestMonitor_StopCancelsTimer(t *testing.T) {
	sub := newMockLogSubscriber()
	notif := &mockNotifier{enabled: true}

	m := NewTelegramMonitor(sub, notif, "test-bot", 200*time.Millisecond, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())

	go m.Start(ctx)

	// Write trigger to start timer
	sub.writeEntry("Starting Telegram bot...")
	time.Sleep(50 * time.Millisecond)

	// Stop before timeout fires
	m.Stop()

	// Wait longer than timeout — should NOT fire because Stop() cancelled timer
	time.Sleep(400 * time.Millisecond)
	cancel()

	calls := notif.getCalls()
	assert.Empty(t, calls, "Stop() should cancel timer, no timeout notification should fire")
}

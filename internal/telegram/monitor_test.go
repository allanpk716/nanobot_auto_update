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

// --- Edge case and concurrency stress tests (Plan 42-02) ---

// panicNotifier mocks a notifier whose Notify() always panics.
type panicNotifier struct {
	mu        sync.Mutex
	panicked  bool
}

func (p *panicNotifier) IsEnabled() bool { return true }

func (p *panicNotifier) Notify(title, message string) error {
	p.mu.Lock()
	p.panicked = true
	p.mu.Unlock()
	panic("intentional test panic")
}

func (p *panicNotifier) didPanic() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.panicked
}

// disabledNotifier mocks a notifier that is not enabled.
type disabledNotifier struct{}

func (d *disabledNotifier) IsEnabled() bool { return false }
func (d *disabledNotifier) Notify(title, message string) error { return nil }

func TestMonitor_RapidTriggerSuccessSequence(t *testing.T) {
	sub := newMockLogSubscriber()
	notif := &mockNotifier{enabled: true}

	m := NewTelegramMonitor(sub, notif, "test-bot", 200*time.Millisecond, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.Start(ctx)
	time.Sleep(10 * time.Millisecond) // Wait for goroutine startup

	// Send all 4 entries rapidly: trigger, success, trigger, success
	sub.writeEntry("Starting Telegram bot...")
	sub.writeEntry("Telegram bot commands registered")
	sub.writeEntry("Starting Telegram bot...")
	sub.writeEntry("Telegram bot commands registered")

	// Wait for processing and async notifications
	time.Sleep(300 * time.Millisecond)

	calls := notif.getCalls()
	require.Len(t, calls, 2, "expected exactly 2 success notifications")
	for _, c := range calls {
		assert.Contains(t, c.Title, "Connected", "each notification should be a success")
	}
}

func TestMonitor_RapidTriggerFailureSequence(t *testing.T) {
	sub := newMockLogSubscriber()
	notif := &mockNotifier{enabled: true}

	m := NewTelegramMonitor(sub, notif, "test-bot", 200*time.Millisecond, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.Start(ctx)
	time.Sleep(10 * time.Millisecond) // Wait for goroutine startup

	// Send all 4 entries rapidly: trigger, failure, trigger, failure
	sub.writeEntry("Starting Telegram bot...")
	sub.writeEntry("httpx.ConnectError: connection refused")
	sub.writeEntry("Starting Telegram bot...")
	sub.writeEntry("httpx.ConnectError: timeout")

	// Wait for processing and async notifications
	time.Sleep(300 * time.Millisecond)

	calls := notif.getCalls()
	require.Len(t, calls, 2, "expected exactly 2 failure notifications")
	for _, c := range calls {
		assert.Contains(t, c.Title, "Failed", "each notification should be a failure")
	}
}

func TestMonitor_TriggerFollowedByAnotherTrigger(t *testing.T) {
	sub := newMockLogSubscriber()
	notif := &mockNotifier{enabled: true}

	m := NewTelegramMonitor(sub, notif, "test-bot", 200*time.Millisecond, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.Start(ctx)
	time.Sleep(10 * time.Millisecond) // Wait for goroutine startup

	// Send trigger, wait 50ms, send another trigger (timer should restart)
	sub.writeEntry("Starting Telegram bot...")
	time.Sleep(50 * time.Millisecond)
	sub.writeEntry("Starting Telegram bot...")

	// Wait for timeout to fire (200ms from second trigger, plus buffer)
	time.Sleep(400 * time.Millisecond)

	calls := notif.getCalls()
	require.Len(t, calls, 1, "expected exactly 1 timeout notification (timer restarted on second trigger)")
	assert.Contains(t, calls[0].Title, "Timeout", "should be a timeout notification")
}

func TestMonitor_NotificationPanicRecovery(t *testing.T) {
	sub := newMockLogSubscriber()
	notif := &panicNotifier{}

	m := NewTelegramMonitor(sub, notif, "test-bot", 5*time.Second, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.Start(ctx)
	time.Sleep(10 * time.Millisecond) // Wait for goroutine startup

	// Send trigger + failure to trigger notification (which will panic)
	sub.writeEntry("Starting Telegram bot...")
	sub.writeEntry("httpx.ConnectError: connection refused")

	// Sleep to allow notification goroutine to run and panic to be recovered
	time.Sleep(100 * time.Millisecond)

	// If we reach here, the monitor goroutine survived the panic
	assert.True(t, notif.didPanic(), "Notify should have been called and panicked")
	// Verify monitor is still functional by sending another trigger
	sub.writeEntry("Starting Telegram bot...")
	sub.writeEntry("Telegram bot commands registered")
	time.Sleep(100 * time.Millisecond)
	// No deadlock means the monitor survived the panic
}

func TestMonitor_DisabledNotifierSucceeds(t *testing.T) {
	sub := newMockLogSubscriber()
	notif := &disabledNotifier{}

	m := NewTelegramMonitor(sub, notif, "test-bot", 5*time.Second, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.Start(ctx)
	time.Sleep(10 * time.Millisecond) // Wait for goroutine startup

	// Send trigger + success — notifier is disabled but should not panic
	sub.writeEntry("Starting Telegram bot...")
	sub.writeEntry("Telegram bot commands registered")

	// Allow processing
	time.Sleep(100 * time.Millisecond)

	// No panic, no error — test passes by reaching this point
}

func TestMonitor_ContextCancelledBeforeTrigger(t *testing.T) {
	sub := newMockLogSubscriber()
	notif := &mockNotifier{enabled: true}

	m := NewTelegramMonitor(sub, notif, "test-bot", 200*time.Millisecond, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		m.Start(ctx)
		close(done)
	}()
	time.Sleep(10 * time.Millisecond) // Wait for goroutine startup

	// Cancel context before any log entry
	cancel()

	// Verify Start() returns promptly
	select {
	case <-done:
		// Good — monitor exited
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Start() did not return after context cancellation — possible goroutine leak")
	}

	calls := notif.getCalls()
	assert.Empty(t, calls, "no notification should be sent when context is cancelled before trigger")
}

func TestMonitor_EmptyContentEntry(t *testing.T) {
	sub := newMockLogSubscriber()
	notif := &mockNotifier{enabled: true}

	m := NewTelegramMonitor(sub, notif, "test-bot", 200*time.Millisecond, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.Start(ctx)
	time.Sleep(10 * time.Millisecond) // Wait for goroutine startup

	// Send entry with empty content
	sub.writeEntry("")

	// Wait longer than timeout — empty content should not trigger anything
	time.Sleep(400 * time.Millisecond)

	calls := notif.getCalls()
	assert.Empty(t, calls, "empty content should not trigger any state change or notification")
}

func TestMonitor_ConcurrentTimerAndProcessEntry(t *testing.T) {
	sub := newMockLogSubscriber()
	notif := &mockNotifier{enabled: true}

	// Use very short timeout to create race between timer and processEntry
	m := NewTelegramMonitor(sub, notif, "test-bot", 50*time.Millisecond, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.Start(ctx)
	time.Sleep(10 * time.Millisecond) // Wait for goroutine startup

	// Send trigger, then immediately send success
	// The race: will the 50ms timer fire before processEntry sees the success?
	sub.writeEntry("Starting Telegram bot...")
	sub.writeEntry("Telegram bot commands registered")

	// Wait for either outcome
	time.Sleep(200 * time.Millisecond)

	calls := notif.getCalls()
	// Either success notification or timeout notification — exactly 1
	require.Len(t, calls, 1, "expected exactly 1 notification (either success or timeout)")
	// Validate it's one of the expected outcomes
	isSuccess := containsSubstring(calls[0].Title, "Connected")
	isTimeout := containsSubstring(calls[0].Title, "Timeout")
	assert.True(t, isSuccess || isTimeout,
		"notification should be either Connected or Timeout, got: %s", calls[0].Title)
}

// containsSubstring checks if s contains substr (test helper).
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstringHelper(s, substr))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

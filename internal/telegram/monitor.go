package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/logbuffer"
)

// LogSubscriber is satisfied by *logbuffer.LogBuffer via duck typing.
type LogSubscriber interface {
	Subscribe() <-chan logbuffer.LogEntry
	Unsubscribe(ch <-chan logbuffer.LogEntry)
}

// Notifier is satisfied by *notifier.Notifier via duck typing.
type Notifier interface {
	IsEnabled() bool
	Notify(title, message string) error
}

type monitorState int

const (
	stateIdle monitorState = iota
	stateWaiting
)

// TelegramMonitor watches log output for Telegram bot connection outcomes
// and sends Pushover notifications on success, failure, or timeout.
type TelegramMonitor struct {
	mu           sync.Mutex
	state        monitorState
	timer        *time.Timer
	logBuffer    LogSubscriber
	notifier     Notifier
	instanceName string
	timeout      time.Duration
	startTime    time.Time // TELE-08: filter entries before this timestamp
	logger       *slog.Logger
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewTelegramMonitor creates a new Telegram monitor instance.
func NewTelegramMonitor(logBuffer LogSubscriber, notifier Notifier, instanceName string, timeout time.Duration, logger *slog.Logger) *TelegramMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &TelegramMonitor{
		logBuffer:    logBuffer,
		notifier:     notifier,
		instanceName: instanceName,
		timeout:      timeout,
		logger:       logger.With("component", "telegram-monitor", "instance", instanceName),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start begins monitoring log output. Blocks until context is cancelled or channel is closed.
func (m *TelegramMonitor) Start(ctx context.Context) {
	ch := m.logBuffer.Subscribe()
	defer m.logBuffer.Unsubscribe(ch)

	m.startTime = time.Now() // TELE-08: only process entries after this time

	for {
		select {
		case entry, ok := <-ch:
			if !ok {
				return
			}
			m.processEntry(entry)
		case <-ctx.Done():
			m.mu.Lock()
			if m.timer != nil {
				m.timer.Stop()
			}
			m.mu.Unlock()
			return
		}
	}
}

// processEntry handles a single log entry, applying the state machine logic.
func (m *TelegramMonitor) processEntry(entry logbuffer.LogEntry) {
	// TELE-08: Ignore historical entries written before subscription
	if entry.Timestamp.Before(m.startTime) {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	switch m.state {
	case stateIdle:
		if IsTrigger(entry.Content) { // TELE-01
			m.state = stateWaiting
			m.startTimer()
			m.logger.Info("Telegram bot trigger detected, waiting for connection outcome")
		}
	case stateWaiting:
		if IsSuccess(entry.Content) { // TELE-02
			m.timer.Stop()
			m.state = stateIdle
			title := "Telegram Connected"
			message := fmt.Sprintf("Instance %s: Telegram bot connected successfully", m.instanceName)
			go m.sendNotification(title, message) // TELE-05
			m.logger.Info("Telegram bot connected successfully")
		} else if IsFailure(entry.Content) { // TELE-03
			m.timer.Stop()
			m.state = stateIdle
			title := "Telegram Connection Failed"
			message := fmt.Sprintf("Instance %s: httpx.ConnectError detected in log output", m.instanceName)
			go m.sendNotification(title, message) // TELE-06
			m.logger.Info("Telegram bot connection failed", "pattern", entry.Content)
		}
	}
}

// startTimer starts the timeout timer. Must be called with m.mu held.
func (m *TelegramMonitor) startTimer() {
	if m.timer != nil {
		m.timer.Stop()
	}
	m.timer = time.AfterFunc(m.timeout, func() {
		m.mu.Lock()
		defer m.mu.Unlock()

		if m.state != stateWaiting {
			return // Already resolved, ignore stale timeout
		}
		m.state = stateIdle
		title := "Telegram Connection Timeout"
		message := fmt.Sprintf("Instance %s: connection timeout, no response within %v", m.instanceName, m.timeout)
		go m.sendNotification(title, message) // TELE-04
		m.logger.Info("Telegram connection timed out", "timeout", m.timeout)
	})
}

// sendNotification delivers a Pushover notification with panic recovery.
// Called from a goroutine (go m.sendNotification) so does not spawn another goroutine.
func (m *TelegramMonitor) sendNotification(title, message string) {
	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("notification goroutine panic",
				"panic", r,
				"stack", string(debug.Stack()))
		}
	}()

	if !m.notifier.IsEnabled() {
		return
	}

	if err := m.notifier.Notify(title, message); err != nil {
		m.logger.Error("telegram notification failed",
			"error", err,
			"title", title)
	}
}

// Stop cancels the monitor, stopping any active timer and the context.
func (m *TelegramMonitor) Stop() {
	m.mu.Lock()
	if m.timer != nil {
		m.timer.Stop()
	}
	m.mu.Unlock()
	m.cancel()
}

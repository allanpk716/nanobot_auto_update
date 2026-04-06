package telegram

import (
	"strings"
	"time"
)

const (
	// TriggerPattern starts the 30s monitoring window (TELE-01)
	TriggerPattern = "Starting Telegram bot"

	// SuccessPattern indicates Telegram bot connected (TELE-02)
	SuccessPattern = "Telegram bot commands registered"

	// FailurePattern indicates connection error (TELE-03)
	FailurePattern = "httpx.ConnectError"

	// DefaultTimeout for Telegram connection monitoring (TELE-04)
	DefaultTimeout = 30 * time.Second
)

// IsTrigger returns true if the log line indicates Telegram bot is starting.
func IsTrigger(line string) bool {
	return strings.Contains(line, TriggerPattern)
}

// IsSuccess returns true if the log line indicates Telegram bot connected successfully.
func IsSuccess(line string) bool {
	return strings.Contains(line, SuccessPattern)
}

// IsFailure returns true if the log line indicates a connection error.
func IsFailure(line string) bool {
	return strings.Contains(line, FailurePattern)
}

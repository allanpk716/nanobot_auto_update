package notifier

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/gregdel/pushover"
)

// Notifier handles push notification sending via Pushover
type Notifier struct {
	client    *pushover.Pushover
	recipient *pushover.Recipient
	logger    *slog.Logger
	enabled   bool
}

// New creates a notifier from environment variables
// If PUSHOVER_TOKEN or PUSHOVER_USER are not set, returns a disabled notifier
// that logs warnings instead of failing
func New(logger *slog.Logger) *Notifier {
	token := os.Getenv("PUSHOVER_TOKEN")
	user := os.Getenv("PUSHOVER_USER")

	if token == "" || user == "" {
		logger.Warn("Pushover notifications disabled",
			"reason", "PUSHOVER_TOKEN and/or PUSHOVER_USER environment variables not set",
			"hint", "Set both variables to enable failure notifications")
		return &Notifier{
			enabled: false,
			logger:  logger,
		}
	}

	logger.Info("Pushover notifications enabled")
	return &Notifier{
		client:    pushover.New(token),
		recipient: pushover.NewRecipient(user),
		logger:    logger,
		enabled:   true,
	}
}

// IsEnabled returns whether notifications are configured
func (n *Notifier) IsEnabled() bool {
	return n.enabled
}

// Notify sends a notification with the given title and message
// Returns nil if notifications are disabled (no error)
func (n *Notifier) Notify(title, message string) error {
	if !n.enabled {
		n.logger.Debug("Notification skipped (not configured)", "title", title)
		return nil
	}

	msg := pushover.NewMessageWithTitle(message, title)
	response, err := n.client.SendMessage(msg, n.recipient)
	if err != nil {
		n.logger.Error("Failed to send notification",
			"title", title,
			"error", err)
		return fmt.Errorf("pushover notification failed: %w", err)
	}

	n.logger.Info("Notification sent successfully",
		"title", title,
		"id", response.ID)
	return nil
}

// NotifyFailure is a convenience method for sending failure notifications
func (n *Notifier) NotifyFailure(operation string, err error) error {
	title := fmt.Sprintf("Nanobot Update Failed: %s", operation)
	message := fmt.Sprintf("Operation: %s\n\nError: %v", operation, err)
	return n.Notify(title, message)
}

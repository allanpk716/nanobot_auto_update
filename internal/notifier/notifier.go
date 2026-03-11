package notifier

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/HQGroup/nanobot-auto-updater/internal/instance"
	"github.com/gregdel/pushover"
)

// Config holds Pushover configuration
type Config struct {
	ApiToken string
	UserKey  string
}

// Notifier handles push notification sending via Pushover
type Notifier struct {
	client    *pushover.Pushover
	recipient *pushover.Recipient
	logger    *slog.Logger
	enabled   bool
}

// New creates a notifier from environment variables (for backward compatibility)
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

	logger.Info("Pushover notifications enabled (from env)")
	return &Notifier{
		client:    pushover.New(token),
		recipient: pushover.NewRecipient(user),
		logger:    logger,
		enabled:   true,
	}
}

// NewWithConfig creates a notifier from Config struct
// Falls back to environment variables if config is empty
func NewWithConfig(cfg Config, logger *slog.Logger) *Notifier {
	token := cfg.ApiToken
	user := cfg.UserKey

	// Fallback to environment variables if config is empty
	if token == "" {
		token = os.Getenv("PUSHOVER_TOKEN")
	}
	if user == "" {
		user = os.Getenv("PUSHOVER_USER")
	}

	if token == "" || user == "" {
		logger.Warn("Pushover notifications disabled",
			"reason", "Pushover config not provided and env vars not set",
			"hint", "Set pushover.api_token and pushover.user_key in config.yaml, or set PUSHOVER_TOKEN and PUSHOVER_USER env vars")
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

// NotifySuccess is a convenience method for sending success notifications
func (n *Notifier) NotifySuccess(operation, details string) error {
	title := fmt.Sprintf("Nanobot Update Success: %s", operation)
	message := fmt.Sprintf("Operation: %s\n\n%s", operation, details)
	return n.Notify(title, message)
}

// NotifyUpdateResult sends a notification for multi-instance update results
// Only sends notification if there are errors (HasErrors() == true)
// Returns nil if all instances succeeded or if notifications are disabled
func (n *Notifier) NotifyUpdateResult(result *instance.UpdateResult) error {
	// 如果没有错误,记录 DEBUG 日志并返回 nil
	if !result.HasErrors() {
		n.logger.Debug("All instances succeeded, skipping failure notification",
			"stopped_count", len(result.Stopped),
			"started_count", len(result.Started))
		return nil
	}

	// 构建格式化消息
	message := n.formatUpdateResultMessage(result)

	// 发送通知
	return n.Notify("Nanobot 多实例更新失败", message)
}

// formatUpdateResultMessage formats UpdateResult into a user-friendly notification message
func (n *Notifier) formatUpdateResultMessage(result *instance.UpdateResult) string {
	var msg strings.Builder

	// 计算总失败数
	totalFailed := len(result.StopFailed) + len(result.StartFailed)

	// 第一部分: 失败摘要
	msg.WriteString(fmt.Sprintf("更新失败: %d 个实例操作失败\n\n", totalFailed))

	// 第二部分: 停止失败详情
	if len(result.StopFailed) > 0 {
		msg.WriteString("停止失败的实例:\n")
		for _, err := range result.StopFailed {
			msg.WriteString(fmt.Sprintf("  ✗ %s (端口 %d)\n", err.InstanceName, err.Port))
			msg.WriteString(fmt.Sprintf("    原因: %v\n", err.Err))
		}
		msg.WriteString("\n")
	}

	// 第三部分: 启动失败详情
	if len(result.StartFailed) > 0 {
		msg.WriteString("启动失败的实例:\n")
		for _, err := range result.StartFailed {
			msg.WriteString(fmt.Sprintf("  ✗ %s (端口 %d)\n", err.InstanceName, err.Port))
			msg.WriteString(fmt.Sprintf("    原因: %v\n", err.Err))
		}
		msg.WriteString("\n")
	}

	// 第四部分: 成功启动列表
	if len(result.Started) > 0 {
		msg.WriteString(fmt.Sprintf("成功启动的实例 (%d):\n", len(result.Started)))
		for _, name := range result.Started {
			msg.WriteString(fmt.Sprintf("  ✓ %s\n", name))
		}
	}

	return msg.String()
}

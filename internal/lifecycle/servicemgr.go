//go:build !windows

package lifecycle

import (
	"context"
	"log/slog"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
)

// ServiceManager is a no-op stub on non-Windows platforms (D-09).
type ServiceManager struct{}

// NewServiceManager returns a no-op ServiceManager on non-Windows platforms.
func NewServiceManager(cfg *config.Config, logger *slog.Logger) *ServiceManager {
	return &ServiceManager{}
}

// IsAdmin always returns false on non-Windows platforms.
func IsAdmin() bool {
	return false
}

// RegisterService is a no-op on non-Windows platforms.
// Logs a message indicating service registration is not supported.
func RegisterService(cfg *config.Config, logger *slog.Logger) error {
	logger.Info("Service registration is not supported on this platform, auto_start configuration ignored")
	return nil
}

// UnregisterService is a no-op on non-Windows platforms.
// Logs a message indicating service uninstallation is not supported.
func UnregisterService(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	logger.Info("Service uninstallation is not supported on this platform, auto_start configuration ignored")
	return nil
}

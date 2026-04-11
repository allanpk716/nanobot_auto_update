//go:build !windows

package lifecycle

import (
	"fmt"
	"log/slog"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
)

// ServiceHandler is a no-op stub on non-Windows platforms.
// On Windows, use the build-tagged service_windows.go implementation.
type ServiceHandler struct{}

// NewServiceHandler is not available on non-Windows platforms.
// Returns a stub handler. This should never be called in practice
// because IsServiceMode() always returns false on non-Windows.
func NewServiceHandler(
	cfg *config.Config,
	logger *slog.Logger,
	version string,
	updateLogger LogScheduler,
	notif NotifySender,
	createComponents CreateComponentsFunc,
	startInstances StartInstancesFunc,
	onReady func(*AppComponents),
) *ServiceHandler {
	return &ServiceHandler{}
}

// RunService is not supported on non-Windows platforms (D-09).
// Returns an error indicating the feature is unavailable.
func RunService(
	cfg *config.Config,
	logger *slog.Logger,
	version string,
	updateLogger LogScheduler,
	notif NotifySender,
	createComponents CreateComponentsFunc,
	startInstances StartInstancesFunc,
	onReady func(*AppComponents),
) error {
	return fmt.Errorf("service mode is not supported on this platform")
}

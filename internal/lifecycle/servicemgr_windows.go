//go:build windows

package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
)

// ServiceManager manages Windows service registration and lifecycle (D-01).
type ServiceManager struct {
	cfg    *config.Config
	logger *slog.Logger
}

// NewServiceManager creates a new ServiceManager instance.
func NewServiceManager(cfg *config.Config, logger *slog.Logger) *ServiceManager {
	return &ServiceManager{cfg: cfg, logger: logger}
}

// IsAdmin checks whether the current process is running with elevated (administrator) privileges (D-08).
// Returns true if the process token indicates elevation, false otherwise.
func IsAdmin() bool {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return false
	}
	defer token.Close()
	return token.IsElevated()
}

// RegisterService registers the current executable as a Windows service with SCM (D-01, D-02, D-03, D-04, D-07).
// If the service already exists, it returns nil (idempotent).
// Recovery policy: 3x ServiceRestart at 60s intervals, 24h reset period (D-07).
func (m *ServiceManager) RegisterService() error {
	// Defensive check: empty ServiceName (T-48-06)
	if m.cfg.Service.ServiceName == "" {
		return fmt.Errorf("registerService: service_name is empty, cannot register service")
	}

	// Connect to SCM
	scm, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("registerService: failed to connect to SCM: %w", err)
	}
	defer scm.Disconnect()

	// Check if service already exists (idempotent per D-04)
	existingSvc, err := scm.OpenService(m.cfg.Service.ServiceName)
	if err == nil {
		existingSvc.Close()
		m.logger.Info("Service already registered, skipping", "service_name", m.cfg.Service.ServiceName)
		return nil
	}

	// Get executable path for the current binary
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("registerService: failed to get executable path: %w", err)
	}

	m.logger.Info("Registering service", "exe_path", exePath, "service_name", m.cfg.Service.ServiceName)

	// Create the service (D-01: LocalSystem, D-02: Auto start)
	svcHandle, err := scm.CreateService(
		m.cfg.Service.ServiceName,
		exePath,
		mgr.Config{
			StartType:        mgr.StartAutomatic,
			ErrorControl:     mgr.ErrorNormal,
			ServiceStartName: "LocalSystem",
			DisplayName:      m.cfg.Service.DisplayName,
			Description:      "自动保持 nanobot 处于最新版本",
		},
	)
	if err != nil {
		return fmt.Errorf("registerService: failed to create service %q: %w", m.cfg.Service.ServiceName, err)
	}
	defer svcHandle.Close()

	// Configure recovery policy (D-07): 3x restart at 60s, 24h reset
	recoveryActions := []mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 60 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 60 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 60 * time.Second},
	}
	if err := svcHandle.SetRecoveryActions(recoveryActions, 86400); err != nil {
		return fmt.Errorf("registerService: failed to set recovery actions for service %q: %w", m.cfg.Service.ServiceName, err)
	}

	// Enable recovery on non-crash failures (non-critical -- log warning on failure)
	if err := svcHandle.SetRecoveryActionsOnNonCrashFailures(true); err != nil {
		m.logger.Warn("Failed to enable recovery on non-crash failures (non-critical)", "error", err)
	}

	m.logger.Info("Windows service registered successfully",
		"service_name", m.cfg.Service.ServiceName,
		"display_name", m.cfg.Service.DisplayName,
	)
	return nil
}

// UnregisterService stops and deletes the Windows service (D-05, D-06).
// Accepts context.Context for cancellable stop-wait loop (T-48-04).
// If the service does not exist, returns nil (idempotent).
func (m *ServiceManager) UnregisterService(ctx context.Context) error {
	// Connect to SCM
	scm, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("unregisterService: failed to connect to SCM: %w", err)
	}
	defer scm.Disconnect()

	// Open the service
	svcHandle, err := scm.OpenService(m.cfg.Service.ServiceName)
	if err != nil {
		m.logger.Info("Service not registered, nothing to uninstall", "service_name", m.cfg.Service.ServiceName)
		return nil
	}
	defer svcHandle.Close()

	// Try to stop the service first (D-05)
	m.logger.Info("Stopping service before uninstall", "service_name", m.cfg.Service.ServiceName)
	if _, err := svcHandle.Control(svc.Stop); err != nil {
		m.logger.Warn("Failed to send stop control (service may already be stopped)", "error", err)
	}

	// Poll for stopped state with context cancellation support (T-48-04)
	pollTicker := time.NewTicker(1 * time.Second)
	defer pollTicker.Stop()
	pollTimeout := time.After(30 * time.Second)
	for {
		select {
		case <-ctx.Done():
			m.logger.Warn("UnregisterService cancelled while waiting for service to stop", "error", ctx.Err())
			goto deleteService
		case <-pollTimeout:
			m.logger.Warn("Timed out waiting for service to stop, proceeding with delete")
			goto deleteService
		case <-pollTicker.C:
			status, err := svcHandle.Query()
			if err != nil {
				m.logger.Warn("Failed to query service status, proceeding with delete", "error", err)
				goto deleteService
			}
			if status.State == svc.Stopped {
				m.logger.Info("Service stopped successfully")
				goto deleteService
			}
		}
	}

deleteService:
	// Delete the service
	if err := svcHandle.Delete(); err != nil {
		return fmt.Errorf("unregisterService: failed to delete service %q: %w", m.cfg.Service.ServiceName, err)
	}
	m.logger.Info("Service unregistered", "service_name", m.cfg.Service.ServiceName)
	return nil
}

// RegisterService is a convenience wrapper that creates a ServiceManager and calls RegisterService.
func RegisterService(cfg *config.Config, logger *slog.Logger) error {
	sm := NewServiceManager(cfg, logger)
	return sm.RegisterService()
}

// UnregisterService is a convenience wrapper that creates a ServiceManager and calls UnregisterService.
func UnregisterService(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	sm := NewServiceManager(cfg, logger)
	return sm.UnregisterService(ctx)
}

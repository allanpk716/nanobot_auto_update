//go:build windows

package lifecycle

import (
	"context"
	"log/slog"
	"time"

	"golang.org/x/sys/windows/svc"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
)

// ServiceHandler implements svc.Handler for Windows service lifecycle (D-01).
// It manages the SCM state transitions and delegates startup/shutdown to
// AppStartup/AppShutdown from app.go.
type ServiceHandler struct {
	cfg              *config.Config
	logger           *slog.Logger
	version          string
	updateLogger     LogScheduler
	notif            NotifySender
	createComponents CreateComponentsFunc
	startInstances   StartInstancesFunc
}

// NewServiceHandler creates a new service handler (D-01).
// All parameters are passed through to AppStartup.
func NewServiceHandler(
	cfg *config.Config,
	logger *slog.Logger,
	version string,
	updateLogger LogScheduler,
	notif NotifySender,
	createComponents CreateComponentsFunc,
	startInstances StartInstancesFunc,
) *ServiceHandler {
	return &ServiceHandler{
		cfg:              cfg,
		logger:           logger,
		version:          version,
		updateLogger:     updateLogger,
		notif:            notif,
		createComponents: createComponents,
		startInstances:   startInstances,
	}
}

// Execute is called by svc.Run to run the service (D-02, D-03).
// It reads ChangeRequests from r and reports Status to s.
//
// Failure behavior:
// - If AppStartup fails: reports svc.Stopped, returns (true, 1).
//   AppStartup's internal rollback already cleaned up partial components.
// - If shutdown is triggered normally: reports svc.Stopped, returns (false, 0).
func (h *ServiceHandler) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	// D-02: Report starting state
	s <- svc.Status{State: svc.StartPending}

	// D-10: Initialize all business components via AppStartup
	// AppStartup handles rollback internally if it fails
	components, err := AppStartup(
		h.cfg, h.logger, h.version,
		h.updateLogger, h.notif,
		h.createComponents, h.startInstances,
	)
	if err != nil {
		h.logger.Error("Service startup failed", "error", err)
		// AppStartup already called AppShutdown on partial components (rollback)
		// Report Stopped to SCM and return error code
		s <- svc.Status{State: svc.Stopped}
		return true, 1 // service-specific error code
	}

	// D-02: Report running state, accept Stop and Shutdown
	s <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	h.logger.Info("Service is running")

	// Main event loop -- wait for Stop/Shutdown (D-03)
loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				s <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				h.logger.Info("Service stop requested", "cmd", c.Cmd)
				break loop
			default:
				// Ignore ParamChange, SessionChange, etc.
			}
		}
	}

	// D-02: Report stopping state
	s <- svc.Status{State: svc.StopPending}

	// D-04, D-06: Graceful shutdown with 30-second timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	AppShutdown(shutdownCtx, components, h.logger)

	// D-02: Report stopped -- svc package handles process exit (D-12)
	s <- svc.Status{State: svc.Stopped}
	return false, 0
}

// RunService runs the application as a Windows service (D-09).
// It blocks until the service handler's Execute method returns.
// The returned error comes from svc.Run -- typically non-nil only if
// the process was not launched by SCM (e.g., called from console mode).
func RunService(
	cfg *config.Config,
	logger *slog.Logger,
	version string,
	updateLogger LogScheduler,
	notif NotifySender,
	createComponents CreateComponentsFunc,
	startInstances StartInstancesFunc,
) error {
	handler := NewServiceHandler(cfg, logger, version, updateLogger, notif, createComponents, startInstances)
	return svc.Run(cfg.Service.ServiceName, handler)
}

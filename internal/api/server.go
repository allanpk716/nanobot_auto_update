package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/instance"
	"github.com/HQGroup/nanobot-auto-updater/internal/lifecycle"
	"github.com/HQGroup/nanobot-auto-updater/internal/selfupdate"
	"github.com/HQGroup/nanobot-auto-updater/internal/updatelog"
	"github.com/HQGroup/nanobot-auto-updater/internal/web"
)

// Server represents the HTTP API server
// SSE-07: WriteTimeout=0 for SSE long connections
type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

// NewServer creates a new HTTP API server
// SSE-07: Sets WriteTimeout=0 to support SSE long connections
// HELP-01, HELP-02: Added fullCfg and version parameters for help endpoint
// STORE-01, D-04: UpdateLogger created externally in main.go and injected here
func NewServer(cfg *config.APIConfig, im *instance.InstanceManager, fullCfg *config.Config, version string, logger *slog.Logger, updateLogger *updatelog.UpdateLogger, notif Notifier, selfUpdater *selfupdate.Updater, getToken func() string) (*Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("API config is nil")
	}
	if cfg.Port == 0 {
		return nil, fmt.Errorf("API port is required")
	}

	// Create SSE handler
	sseHandler := NewSSEHandler(im, logger)

	// Create help handler (HELP-01, HELP-02)
	// Note: Uses full config.Config, not just APIConfig
	helpHandler := NewHelpHandler(version, fullCfg, logger)

	// Create router
	mux := http.NewServeMux()

	// Static files handler (must be registered before catch-all routes)
	// This handles /static/* requests for CSS, JS, and other static assets
	mux.Handle("GET /static/", http.StripPrefix("/static/", web.Handler()))

	mux.HandleFunc("GET /api/v1/logs/{instance}/stream", sseHandler.Handle)

	// HELP-01, HELP-02: Help endpoint (no auth required)
	mux.Handle("GET /api/v1/help", helpHandler)

	// Web UI endpoint (Phase 23: UI-01)
	mux.HandleFunc("GET /logs/{instance}", web.NewWebPageHandler(im, logger))

	// Instance list API (Phase 23: UI-07)
	mux.HandleFunc("GET /api/v1/instances", web.NewInstanceListHandler(im, logger))

	// Instance status API (Quick task 260320-k8z: Task 1)
	mux.HandleFunc("GET /api/v1/instances/status", web.NewInstanceStatusHandler(im, logger))

	// Version API (no auth required)
	mux.HandleFunc("GET /api/v1/version", web.NewVersionHandler(version, logger))

	// Instance restart API (Quick task 260325-ovr: Task 1)
	mux.HandleFunc("POST /api/v1/instances/{name}/restart", web.NewInstanceRestartHandler(im, logger))

	// Home page endpoints (Quick task 260320-k8z: Task 2)
	mux.HandleFunc("GET /", web.NewHomePageHandler(im, logger))
	mux.HandleFunc("GET /logs", web.NewHomePageHandler(im, logger))

	// UpdateLogger is injected from main.go (D-04: created externally, not inside NewServer)
	// triggerHandler receives the logger for recording update operations

	// Trigger update endpoint with auth (Phase 28: API-01, API-02)
	instanceCount := len(im.GetInstanceNames())
	triggerHandler := NewTriggerHandler(im, cfg, logger, updateLogger, notif, instanceCount)
	authMiddleware := AuthMiddleware(getToken, logger)

	// Wrap handler with auth middleware
	// API-01: POST /api/v1/trigger-update endpoint exists
	// API-02: Bearer token authentication required
	// API-05: Auth failure returns 401 Unauthorized
	mux.Handle("POST /api/v1/trigger-update",
		authMiddleware(http.HandlerFunc(triggerHandler.Handle)))

	// Query update logs endpoint with auth (Phase 32: QUERY-01, QUERY-02)
	queryHandler := NewQueryHandler(updateLogger, logger)
	mux.Handle("GET /api/v1/update-logs",
		authMiddleware(http.HandlerFunc(queryHandler.Handle)))

	// Web-config endpoint (Phase 44: API-02) -- localhost-only, no auth required
	webConfigHandler := NewWebConfigHandler(cfg.BearerToken, logger)
	mux.HandleFunc("GET /api/v1/web-config", localhostOnly(webConfigHandler))

	// Self-update endpoints (Phase 39: API-01, API-02, API-03)
	if selfUpdater != nil {
		selfUpdateHandler := NewSelfUpdateHandler(selfUpdater, version, im, notif, logger)
		mux.Handle("GET /api/v1/self-update/check",
			authMiddleware(http.HandlerFunc(selfUpdateHandler.HandleCheck)))
		mux.Handle("POST /api/v1/self-update",
			authMiddleware(http.HandlerFunc(selfUpdateHandler.HandleUpdate)))
	}

		// Instance config CRUD endpoints (Phase 50: IC-01 through IC-06)
		// Handler receives config.GetCurrentConfig as the config reader -- no NewServer signature change needed.
		instanceConfigHandler := NewInstanceConfigHandler(config.GetCurrentConfig, logger)
		mux.Handle("GET /api/v1/instance-configs", authMiddleware(http.HandlerFunc(instanceConfigHandler.HandleList)))
		mux.Handle("POST /api/v1/instance-configs", authMiddleware(http.HandlerFunc(instanceConfigHandler.HandleCreate)))
		mux.Handle("GET /api/v1/instance-configs/{name}", authMiddleware(http.HandlerFunc(instanceConfigHandler.HandleGet)))
		mux.Handle("PUT /api/v1/instance-configs/{name}", authMiddleware(http.HandlerFunc(instanceConfigHandler.HandleUpdate)))
		mux.Handle("DELETE /api/v1/instance-configs/{name}", authMiddleware(http.HandlerFunc(instanceConfigHandler.HandleDelete)))
		mux.Handle("POST /api/v1/instance-configs/{name}/copy", authMiddleware(http.HandlerFunc(instanceConfigHandler.HandleCopy)))

	// Create HTTP server
	// SSE-07: WriteTimeout=0 to support SSE long connections
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      mux,
		WriteTimeout: 0, // SSE-07: Support SSE long connections
		ReadTimeout:  10 * time.Second,
	}

	logger.Info("HTTP server created", "port", cfg.Port, "write_timeout", "0 (unlimited)")

	return &Server{
		httpServer: httpServer,
		logger:     logger,
	}, nil
}

// Start starts the HTTP server with port binding retry (D-05).
func (s *Server) Start() error {
	s.logger.Info("HTTP server starting", "addr", s.httpServer.Addr)

	// D-05: Retry port binding for self-update restart scenario
	listener, err := lifecycle.ListenWithRetry(s.httpServer.Addr, s.logger)
	if err != nil {
		return err
	}

	err = s.httpServer.Serve(listener)
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("HTTP server shutting down")
	return s.httpServer.Shutdown(ctx)
}

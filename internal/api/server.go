package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/instance"
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
func NewServer(cfg *config.APIConfig, im *instance.InstanceManager, fullCfg *config.Config, version string, logger *slog.Logger) (*Server, error) {
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
	mux.HandleFunc("GET /api/v1/logs/{instance}/stream", sseHandler.Handle)

	// HELP-01, HELP-02: Help endpoint (no auth required)
	mux.Handle("GET /api/v1/help", helpHandler)

	// Web UI endpoint (Phase 23: UI-01)
	mux.HandleFunc("GET /logs/{instance}", web.NewWebPageHandler(im, logger))

	// Instance list API (Phase 23: UI-07)
	mux.HandleFunc("GET /api/v1/instances", web.NewInstanceListHandler(im, logger))

	// Instance status API (Quick task 260320-k8z: Task 1)
	mux.HandleFunc("GET /api/v1/instances/status", web.NewInstanceStatusHandler(im, logger))

	// Home page endpoints (Quick task 260320-k8z: Task 2)
	mux.HandleFunc("GET /", web.NewHomePageHandler(im, logger))
	mux.HandleFunc("GET /logs", web.NewHomePageHandler(im, logger))

	// Trigger update endpoint with auth (Phase 28: API-01, API-02)
	triggerHandler := NewTriggerHandler(im, cfg, logger)
	authMiddleware := AuthMiddleware(cfg.BearerToken, logger)

	// Wrap handler with auth middleware
	// API-01: POST /api/v1/trigger-update endpoint exists
	// API-02: Bearer token authentication required
	// API-05: Auth failure returns 401 Unauthorized
	mux.Handle("POST /api/v1/trigger-update",
		authMiddleware(http.HandlerFunc(triggerHandler.Handle)))

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

// Start starts the HTTP server
func (s *Server) Start() error {
	s.logger.Info("HTTP server starting", "addr", s.httpServer.Addr)
	err := s.httpServer.ListenAndServe()
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

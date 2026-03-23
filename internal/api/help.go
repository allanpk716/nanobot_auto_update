package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
)

// HelpHandler handles GET /api/v1/help requests
// HELP-01: HTTP endpoint for help information
// HELP-02: No authentication required (public access)
type HelpHandler struct {
	version string
	config  *config.Config
	logger  *slog.Logger
}

// NewHelpHandler creates a new help handler
func NewHelpHandler(version string, cfg *config.Config, logger *slog.Logger) *HelpHandler {
	return &HelpHandler{
		version: version,
		config:  cfg,
		logger:  logger.With("source", "api-help"),
	}
}

// ServeHTTP handles GET /api/v1/help requests
// STUB: Returns 501 Not Implemented - to be implemented in Plan 01
func (h *HelpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{"error": "not implemented"})
}

// HelpResponse is the JSON response structure for help endpoint
type HelpResponse struct {
	Version      string                  `json:"version"`
	Architecture string                  `json:"architecture"`
	Endpoints    map[string]EndpointInfo `json:"endpoints"`
	Config       ConfigReference         `json:"config"`
	CLIFlags     map[string]string       `json:"cli_flags"`
}

// EndpointInfo describes an API endpoint
type EndpointInfo struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Auth        string `json:"auth"`
	Description string `json:"description"`
}

// ConfigReference contains non-sensitive config information
type ConfigReference struct {
	APIPort             int    `json:"api_port"`
	MonitorInterval     string `json:"monitor_interval"`
	HealthCheckInterval string `json:"health_check_interval"`
}

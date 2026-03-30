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
// HELP-01: HTTP endpoint for help information
// HELP-02: No authentication required (public access)
// HELP-03: Returns JSON response with version, endpoints, config, cli_flags
func (h *HelpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// HELP-01: Only GET method is supported
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET method is supported")
		return
	}

	// Build response
	response := HelpResponse{
		Version:      h.version,
		Architecture: "HTTP API + Monitor Service",
		Endpoints:    h.getEndpoints(),
		Config:       h.getConfigReference(),
		CLIFlags:     h.getCLIFlags(),
	}

	// HELP-03: Return JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode help response", "error", err)
	}
}

// getEndpoints returns the list of available API endpoints
// HELP-04: Endpoint information matches actual implementation
func (h *HelpHandler) getEndpoints() map[string]EndpointInfo {
	return map[string]EndpointInfo{
		"trigger_update": {
			Method:      "POST",
			Path:        "/api/v1/trigger-update",
			Auth:        "required",
			Description: "触发更新流程（需要 Bearer Token 认证）",
		},
		"update_logs": {
			Method:      "GET",
			Path:        "/api/v1/update-logs",
			Auth:        "required",
			Description: "Query update log history (supports limit and offset parameters)",
		},
		"help": {
			Method:      "GET",
			Path:        "/api/v1/help",
			Auth:        "optional",
			Description: "查看使用说明和 API 端点列表",
		},
		"logs_stream": {
			Method:      "GET",
			Path:        "/api/v1/logs/{instance}/stream",
			Auth:        "optional",
			Description: "SSE 实时日志流",
		},
		"instances": {
			Method:      "GET",
			Path:        "/api/v1/instances",
			Auth:        "optional",
			Description: "实例名称列表",
		},
		"instances_status": {
			Method:      "GET",
			Path:        "/api/v1/instances/status",
			Auth:        "optional",
			Description: "实例状态列表（名称、端口、运行状态）",
		},
		"logs_ui": {
			Method:      "GET",
			Path:        "/logs/{instance}",
			Auth:        "optional",
			Description: "Web UI 日志查看器",
		},
		"self_update_check": {
			Method:      "GET",
			Path:        "/api/v1/self-update/check",
			Auth:        "required",
			Description: "Check self-update version information (current version, latest version, update status)",
		},
		"self_update": {
			Method:      "POST",
			Path:        "/api/v1/self-update",
			Auth:        "required",
			Description: "Trigger self-update (async, returns 202 Accepted)",
		},
	}
}

// getConfigReference returns non-sensitive config information
func (h *HelpHandler) getConfigReference() ConfigReference {
	return ConfigReference{
		APIPort:             int(h.config.API.Port),
		MonitorInterval:     h.config.Monitor.Interval.String(),
		HealthCheckInterval: h.config.HealthCheck.Interval.String(),
	}
}

// getCLIFlags returns CLI flag documentation
// HELP-04: CLI flags match actual implementation
func (h *HelpHandler) getCLIFlags() map[string]string {
	return map[string]string{
		"--config":   "配置文件路径 (default: ./config.yaml)",
		"--version":  "显示版本信息",
		"-h, --help": "显示帮助信息",
	}
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

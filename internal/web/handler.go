package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/HQGroup/nanobot-auto-updater/internal/instance"
	"github.com/HQGroup/nanobot-auto-updater/internal/lifecycle"
)

//go:embed static/*
var staticFiles embed.FS

// Handler returns an HTTP handler for serving static files
func Handler() http.Handler {
	// Create sub-filesystem to strip "static" prefix
	subFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(fmt.Sprintf("failed to create sub filesystem: %v", err))
	}

	return http.FileServer(http.FS(subFS))
}

// NewWebPageHandler creates a handler for serving the web UI page
// UI-01: Endpoint path /logs/:instance
// ERR-04: Return 404 if instance not found
func NewWebPageHandler(im *instance.InstanceManager, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract instance parameter from URL path
		instanceName := r.PathValue("instance")
		if instanceName == "" {
			http.Error(w, "Instance name required", http.StatusBadRequest)
			return
		}

		// Validate instance exists (ERR-04: return 404 if not found)
		_, err := im.GetLogBuffer(instanceName)
		if err != nil {
			logger.Warn("Instance not found", "instance", instanceName, "error", err)
			http.Error(w, fmt.Sprintf("Instance %s not found", instanceName), http.StatusNotFound)
			return
		}

		// Serve index.html from embedded filesystem
		subFS, err := fs.Sub(staticFiles, "static")
		if err != nil {
			logger.Error("Failed to create sub filesystem", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		content, err := fs.ReadFile(subFS, "index.html")
		if err != nil {
			logger.Error("Failed to read index.html", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Set content type and serve
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(content)
	}
}

// NewInstanceListHandler creates handler for GET /api/v1/instances
// UI-07: Returns list of configured instance names for selector dropdown
func NewInstanceListHandler(im *instance.InstanceManager, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		names := im.GetInstanceNames()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"instances": names,
		}); err != nil {
			logger.Error("Failed to encode instance list", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}
}

// InstanceStatus represents the status of a single instance
type InstanceStatus struct {
	Name    string `json:"name"`
	Port    uint32 `json:"port"`
	Running bool   `json:"running"`
}

// NewInstanceStatusHandler creates handler for GET /api/v1/instances/status
// Returns instance list with name, port, and running status
func NewInstanceStatusHandler(im *instance.InstanceManager, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		configs := im.GetInstanceConfigs()
		statuses := make([]InstanceStatus, 0, len(configs))

		for _, cfg := range configs {
			running, _, _, err := lifecycle.IsNanobotRunning(cfg.Port)
			if err != nil {
				logger.Warn("Failed to detect instance status",
					"instance", cfg.Name,
					"port", cfg.Port,
					"error", err)
				// On error, assume not running
				running = false
			}

			statuses = append(statuses, InstanceStatus{
				Name:    cfg.Name,
				Port:    cfg.Port,
				Running: running,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"instances": statuses,
		}); err != nil {
			logger.Error("Failed to encode instance status list", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}
}

// NewHomePageHandler creates handler for GET / and GET /logs
// Returns the home page with instance list
func NewHomePageHandler(im *instance.InstanceManager, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Serve home.html from embedded filesystem
		subFS, err := fs.Sub(staticFiles, "static")
		if err != nil {
			logger.Error("Failed to create sub filesystem", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		content, err := fs.ReadFile(subFS, "home.html")
		if err != nil {
			logger.Error("Failed to read home.html", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Set content type and serve
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(content)
	}
}

package web

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/HQGroup/nanobot-auto-updater/internal/instance"
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

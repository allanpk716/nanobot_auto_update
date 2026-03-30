package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"sync/atomic"

	"github.com/HQGroup/nanobot-auto-updater/internal/selfupdate"
)

// SelfUpdateChecker is the interface for checking and executing self-updates.
// Satisfied by *selfupdate.Updater via duck typing.
type SelfUpdateChecker interface {
	NeedUpdate(currentVersion string) (bool, *selfupdate.ReleaseInfo, error)
	Update(currentVersion string) error
}

// UpdateMutex is the interface for shared update lock operations.
// Satisfied by *instance.InstanceManager via duck typing (D-02).
type UpdateMutex interface {
	TryLockUpdate() bool
	UnlockUpdate()
	IsUpdating() bool
}

// SelfUpdateStatus represents the current state of a self-update operation.
// Stored in atomic.Value for lock-free concurrent access.
type SelfUpdateStatus struct {
	Status string // "idle" / "updating" / "updated" / "failed"
	Error  string // populated only when Status == "failed"
}

// SelfUpdateCheckResponse is the JSON response for the check endpoint.
type SelfUpdateCheckResponse struct {
	CurrentVersion   string `json:"current_version"`
	LatestVersion    string `json:"latest_version"`
	NeedsUpdate      bool   `json:"needs_update"`
	ReleaseNotes     string `json:"release_notes"`
	PublishedAt      string `json:"published_at"`
	DownloadURL      string `json:"download_url"`
	SelfUpdateStatus string `json:"self_update_status"`
	SelfUpdateError  string `json:"self_update_error,omitempty"`
}

// SelfUpdateHandler handles GET /api/v1/self-update/check and POST /api/v1/self-update requests.
type SelfUpdateHandler struct {
	updater         SelfUpdateChecker
	version         string
	instanceManager UpdateMutex
	status          atomic.Value // stores *SelfUpdateStatus
	logger          *slog.Logger
}

// NewSelfUpdateHandler creates a new SelfUpdateHandler.
func NewSelfUpdateHandler(updater SelfUpdateChecker, version string, im UpdateMutex, logger *slog.Logger) *SelfUpdateHandler {
	h := &SelfUpdateHandler{
		updater:         updater,
		version:         version,
		instanceManager: im,
		logger:          logger.With("source", "api-self-update"),
	}
	h.status.Store(&SelfUpdateStatus{Status: "idle"})
	return h
}

// HandleCheck handles GET /api/v1/self-update/check requests.
// Returns current version info, latest version info, and self-update status (API-03).
func (h *SelfUpdateHandler) HandleCheck(w http.ResponseWriter, r *http.Request) {
	needsUpdate, releaseInfo, err := h.updater.NeedUpdate(h.version)
	if err != nil {
		h.logger.Error("Failed to check for updates", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error",
			fmt.Sprintf("Failed to check for updates: %v", err))
		return
	}

	// Load current status
	currentStatus := h.status.Load().(*SelfUpdateStatus)

	response := SelfUpdateCheckResponse{
		CurrentVersion:   h.version,
		LatestVersion:    releaseInfo.Version,
		NeedsUpdate:      needsUpdate,
		ReleaseNotes:     releaseInfo.ReleaseNotes,
		PublishedAt:      releaseInfo.PublishedAt.Format("2006-01-02T15:04:05Z"),
		DownloadURL:      releaseInfo.DownloadURL,
		SelfUpdateStatus: currentStatus.Status,
		SelfUpdateError:  currentStatus.Error,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode check response", "error", err)
	}
}

// HandleUpdate handles POST /api/v1/self-update requests.
// Executes self-update asynchronously, returns 202 Accepted (D-01, D-04).
func (h *SelfUpdateHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	// Try to acquire lock first (D-02: shared with trigger-update)
	if !h.instanceManager.TryLockUpdate() {
		h.logger.Warn("Self-update rejected: update already in progress")
		writeJSONError(w, http.StatusConflict, "conflict",
			"An update is already in progress. Please try again later.")
		return
	}

	// Store status as "updating" before starting goroutine
	h.status.Store(&SelfUpdateStatus{Status: "updating"})

	// Write 202 Accepted response before starting goroutine (per RESEARCH Pitfall 2)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)

	response := map[string]string{
		"status":  "accepted",
		"message": "Self-update started",
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode update response", "error", err)
	}

	// Launch async goroutine for self-update
	go func() {
		defer h.instanceManager.UnlockUpdate()

		// Panic recovery (per RESEARCH Pitfall 1)
		defer func() {
			if r := recover(); r != nil {
				h.logger.Error("Self-update goroutine panic",
					"panic", r,
					"stack", string(debug.Stack()))
				h.status.Store(&SelfUpdateStatus{
					Status: "failed",
					Error:  fmt.Sprintf("panic: %v", r),
				})
			}
		}()

		h.logger.Info("Starting self-update", "current_version", h.version)

		err := h.updater.Update(h.version)
		if err != nil {
			h.logger.Error("Self-update failed", "error", err, "current_version", h.version)
			h.status.Store(&SelfUpdateStatus{
				Status: "failed",
				Error:  err.Error(),
			})
			return
		}

		h.logger.Info("Self-update completed successfully", "previous_version", h.version)
		h.status.Store(&SelfUpdateStatus{
			Status: "updated",
		})
	}()
}

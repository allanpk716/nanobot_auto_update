package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/lifecycle"
	"github.com/HQGroup/nanobot-auto-updater/internal/selfupdate"
	"golang.org/x/sys/windows"
)

// SelfUpdateChecker is the interface for checking and executing self-updates.
// Satisfied by *selfupdate.Updater via duck typing.
type SelfUpdateChecker interface {
	NeedUpdate(currentVersion string) (bool, *selfupdate.ReleaseInfo, error)
	Update(currentVersion string) error
	GetProgress() *selfupdate.ProgressState
	InvalidateCache()
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
	CurrentVersion   string                    `json:"current_version"`
	LatestVersion    string                    `json:"latest_version"`
	NeedsUpdate      bool                      `json:"needs_update"`
	ReleaseNotes     string                    `json:"release_notes"`
	PublishedAt      string                    `json:"published_at"`
	DownloadURL      string                    `json:"download_url"`
	SelfUpdateStatus string                    `json:"self_update_status"`
	SelfUpdateError  string                    `json:"self_update_error,omitempty"`
	Progress         *selfupdate.ProgressState `json:"progress"`
}

// SelfUpdateHandler handles GET /api/v1/self-update/check and POST /api/v1/self-update requests.
type SelfUpdateHandler struct {
	updater         SelfUpdateChecker
	version         string
	instanceManager UpdateMutex
	status          atomic.Value // stores *SelfUpdateStatus
	notifier        Notifier     // SAFE-02: Pushover notification sender
	logger          *slog.Logger
	// restartFn is called after successful update to restart the process.
	// In production this spawns a new process and calls os.Exit(0).
	// In tests this is overridden to avoid terminating the test process.
	restartFn func(exePath string)
}

// NewSelfUpdateHandler creates a new SelfUpdateHandler.
func NewSelfUpdateHandler(updater SelfUpdateChecker, version string, im UpdateMutex, notif Notifier, logger *slog.Logger) *SelfUpdateHandler {
	h := &SelfUpdateHandler{
		updater:         updater,
		version:         version,
		instanceManager: im,
		notifier:        notif,
		logger:          logger.With("source", "api-self-update"),
		restartFn:       defaultRestartFn,
	}
	h.status.Store(&SelfUpdateStatus{Status: "idle"})
	return h
}

// defaultRestartFn is the production restart implementation.
// Service mode (D-01): exits with code 1 to trigger SCM recovery policy auto-restart.
// Console mode (D-02): spawns a new process and exits with code 0 (self-spawn).
func defaultRestartFn(exePath string) {
	// ADPT-02, D-03: Check service mode to choose restart strategy
	if isSvc, _ := lifecycle.IsServiceMode(); isSvc {
		// D-01: Service mode — exit with non-zero code to trigger SCM recovery policy.
		// Phase 48 configured: 3x ServiceRestart, 60s interval, 24h reset failure count.
		slog.Info("service mode restart: exiting to trigger SCM recovery policy")
		os.Exit(1)
	}

	// D-02: Console mode — original self-spawn behavior (unchanged)
	cmd := exec.Command(exePath, os.Args[1:]...)
	cmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}
	// Log before cmd.Start since we won't be around after os.Exit
	slog.Info("self-spawn restart initiated", "exe", exePath)
	if err := cmd.Start(); err != nil {
		slog.Error("failed to spawn new process after update", "error", err)
		return
	}
	os.Exit(0)
}

// HandleCheck handles GET /api/v1/self-update/check requests.
// Returns current version info, latest version info, and self-update status (API-03).
// Invalidates cache before checking to ensure freshly published releases are detected.
func (h *SelfUpdateHandler) HandleCheck(w http.ResponseWriter, r *http.Request) {
	// Invalidate cache to ensure user-initiated checks always get fresh data from GitHub.
	// Without this, a stale cache from page-load-time auto-check could hide newly
	// published releases for up to cacheTTL (1 hour).
	h.updater.InvalidateCache()

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
		Progress:         h.updater.GetProgress(),
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

	// Send start notification (D-03: start notification)
	if h.notifier != nil {
		title := "Nanobot 自更新开始"
		message := fmt.Sprintf("当前版本: %s", h.version)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					h.logger.Error("start notification goroutine panic",
						"panic", r,
						"stack", string(debug.Stack()))
				}
			}()
			if err := h.notifier.Notify(title, message); err != nil {
				h.logger.Error("start notification failed", "error", err)
			}
		}()
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
				// Send failure notification for panic (D-03)
				if h.notifier != nil {
					title := "Nanobot 自更新失败"
					message := fmt.Sprintf("当前版本: %s\n错误: panic: %v", h.version, r)
					go func() {
						defer func() {
							if r := recover(); r != nil {
								h.logger.Error("panic notification goroutine panic",
									"panic", r,
									"stack", string(debug.Stack()))
							}
						}()
						if notifyErr := h.notifier.Notify(title, message); notifyErr != nil {
							h.logger.Error("panic notification failed", "error", notifyErr)
						}
					}()
				}
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
			// Send failure notification (D-03)
			if h.notifier != nil {
				title := "Nanobot 自更新失败"
				message := fmt.Sprintf("当前版本: %s\n错误: %s", h.version, err.Error())
				go func() {
					defer func() {
						if r := recover(); r != nil {
							h.logger.Error("failure notification goroutine panic",
								"panic", r,
								"stack", string(debug.Stack()))
						}
					}()
					if notifyErr := h.notifier.Notify(title, message); notifyErr != nil {
						h.logger.Error("failure notification failed", "error", notifyErr)
					}
				}()
			}
			return
		}

		h.logger.Info("Self-update completed successfully", "previous_version", h.version)
		h.status.Store(&SelfUpdateStatus{
			Status: "updated",
		})

		// Get target version for status file and notification (cache hit guaranteed)
		_, releaseInfo, _ := h.updater.NeedUpdate(h.version)
		var targetVersion string
		if releaseInfo != nil {
			targetVersion = releaseInfo.Version
		} else {
			targetVersion = "unknown"
		}

		// Write update success marker (D-04)
		exePath, _ := os.Executable()
		if exePath != "" {
			marker := map[string]string{
				"timestamp":   time.Now().Format(time.RFC3339),
				"new_version": targetVersion,
				"old_version": h.version,
			}
			markerData, _ := json.Marshal(marker)
			markerPath := exePath + ".update-success"
			if err := os.WriteFile(markerPath, markerData, 0644); err != nil {
				h.logger.Error("failed to write update-success marker", "error", err)
			}
		}

		// Send completion notification synchronously (D-03, avoids Pitfall 1)
		if h.notifier != nil {
			title := "Nanobot 自更新成功"
			message := fmt.Sprintf("已从 %s 更新到 %s", h.version, targetVersion)
			if err := h.notifier.Notify(title, message); err != nil {
				h.logger.Error("completion notification failed", "error", err)
			}
		}

		// Self-spawn restart (D-01: direct exit, no graceful shutdown)
		h.restartFn(exePath)
	}()
}

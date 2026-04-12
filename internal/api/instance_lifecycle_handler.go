package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/instance"
)

// InstanceLifecycleHandler handles start/stop operations for individual instances.
// LC-01, LC-02: Start and stop endpoints for instance lifecycle control.
// LC-03: Update-lock coordination prevents races with TriggerUpdate/SelfUpdate.
type InstanceLifecycleHandler struct {
	im     *instance.InstanceManager
	logger *slog.Logger
}

// NewInstanceLifecycleHandler creates a new InstanceLifecycleHandler.
func NewInstanceLifecycleHandler(im *instance.InstanceManager, logger *slog.Logger) *InstanceLifecycleHandler {
	return &InstanceLifecycleHandler{
		im:     im,
		logger: logger.With("source", "api-instance-lifecycle"),
	}
}

// HandleStart handles POST /api/v1/instances/{name}/start
// Starts a stopped instance. Returns 409 if already running or if an update is in progress.
func (h *InstanceLifecycleHandler) HandleStart(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Instance name required")
		return
	}

	// Update-lock guard: prevents races with TriggerUpdate and SelfUpdate (review HIGH-1)
	if !h.im.TryLockUpdate() {
		writeJSONError(w, http.StatusConflict, "conflict", "An update is already in progress, cannot start instance")
		return
	}
	defer h.im.UnlockUpdate()

	inst, err := h.im.GetLifecycle(name)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "not_found", fmt.Sprintf("Instance %q not found", name))
		return
	}

	if inst.IsRunning() {
		writeJSONError(w, http.StatusConflict, "conflict", fmt.Sprintf("Instance %q is already running", name))
		return
	}

	// Outer timeout > startup_timeout to let inner logic control its own deadline
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := inst.StartAfterUpdate(ctx); err != nil {
		h.logger.Error("Failed to start instance", "instance", name, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("Failed to start instance %q: %v", name, err))
		return
	}

	h.logger.Info("Instance started via API", "instance", name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": fmt.Sprintf("Instance %q started", name),
		"running": true,
	})
}

// HandleStop handles POST /api/v1/instances/{name}/stop
// Stops a running instance. Returns 409 if not running or if an update is in progress.
func (h *InstanceLifecycleHandler) HandleStop(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Instance name required")
		return
	}

	// Update-lock guard: prevents races with TriggerUpdate and SelfUpdate (review HIGH-1)
	if !h.im.TryLockUpdate() {
		writeJSONError(w, http.StatusConflict, "conflict", "An update is already in progress, cannot stop instance")
		return
	}
	defer h.im.UnlockUpdate()

	inst, err := h.im.GetLifecycle(name)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "not_found", fmt.Sprintf("Instance %q not found", name))
		return
	}

	if !inst.IsRunning() {
		writeJSONError(w, http.StatusConflict, "conflict", fmt.Sprintf("Instance %q is not running", name))
		return
	}

	// Outer timeout > inner stopTimeout (5s) to let inner logic control its own deadline
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := inst.StopForUpdate(ctx); err != nil {
		h.logger.Error("Failed to stop instance", "instance", name, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("Failed to stop instance %q: %v", name, err))
		return
	}

	h.logger.Info("Instance stopped via API", "instance", name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": fmt.Sprintf("Instance %q stopped", name),
		"running": false,
	})
}

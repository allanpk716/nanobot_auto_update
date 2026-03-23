package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/instance"
)

// TriggerHandler handles POST /api/v1/trigger-update requests
// API-01: HTTP endpoint for triggering updates
type TriggerHandler struct {
	instanceManager *instance.InstanceManager
	config          *config.APIConfig
	logger          *slog.Logger
}

// NewTriggerHandler creates a new trigger update handler
func NewTriggerHandler(im *instance.InstanceManager, cfg *config.APIConfig, logger *slog.Logger) *TriggerHandler {
	return &TriggerHandler{
		instanceManager: im,
		config:          cfg,
		logger:          logger.With("source", "api-trigger"),
	}
}

// Handle handles POST /api/v1/trigger-update requests
// API-01: HTTP endpoint for triggering updates
// API-03: Executes full stop->update->start flow
// API-04: Returns JSON formatted result
func (h *TriggerHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// 1. Validate method
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST method is supported")
		return
	}

	// 2. Create context with timeout from config
	ctx, cancel := context.WithTimeout(r.Context(), h.config.Timeout)
	defer cancel()

	// 3. Execute update
	result, err := h.instanceManager.TriggerUpdate(ctx)
	if err != nil {
		// Handle specific errors
		if errors.Is(err, instance.ErrUpdateInProgress) {
			h.logger.Warn("Update request rejected: update already in progress")
			writeJSONError(w, http.StatusConflict, "conflict", "Update already in progress")
			return
		}
		if errors.Is(err, context.DeadlineExceeded) {
			h.logger.Error("Update operation timed out", "timeout", h.config.Timeout)
			writeJSONError(w, http.StatusGatewayTimeout, "timeout",
				fmt.Sprintf("Update operation timed out after %v", h.config.Timeout))
			return
		}
		// Other errors (e.g., UV update failed)
		h.logger.Error("Update operation failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	// 4. Return JSON result (200 OK even if update had errors)
	h.logger.Info("Update completed", "success", !result.HasErrors())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Convert InstanceErrors to APIInstanceErrors for JSON serialization
	stopFailed := make([]*APIInstanceError, len(result.StopFailed))
	for i, err := range result.StopFailed {
		stopFailed[i] = convertToAPIError(err)
	}

	startFailed := make([]*APIInstanceError, len(result.StartFailed))
	for i, err := range result.StartFailed {
		startFailed[i] = convertToAPIError(err)
	}

	response := APIUpdateResult{
		Success:     !result.HasErrors(),
		Stopped:     result.Stopped,
		Started:     result.Started,
		StopFailed:  stopFailed,
		StartFailed: startFailed,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode response", "error", err)
	}
}

// APIUpdateResult is the JSON response structure for trigger-update endpoint
// API-04: JSON format response
type APIUpdateResult struct {
	Success     bool                      `json:"success"`
	Stopped     []string                  `json:"stopped,omitempty"`
	Started     []string                  `json:"started,omitempty"`
	StopFailed  []*APIInstanceError       `json:"stop_failed,omitempty"`
	StartFailed []*APIInstanceError       `json:"start_failed,omitempty"`
}

// APIInstanceError is a JSON-serializable version of instance.InstanceError
type APIInstanceError struct {
	InstanceName string `json:"instance"`
	Operation    string `json:"operation"`
	Port         uint32 `json:"port"`
	Error        string `json:"error"`
}

// convertToAPIError converts instance.InstanceError to APIInstanceError
func convertToAPIError(err *instance.InstanceError) *APIInstanceError {
	if err == nil {
		return nil
	}
	return &APIInstanceError{
		InstanceName: err.InstanceName,
		Operation:    err.Operation,
		Port:         err.Port,
		Error:        err.Err.Error(),
	}
}

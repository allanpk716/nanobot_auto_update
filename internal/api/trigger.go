package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/instance"
	"github.com/HQGroup/nanobot-auto-updater/internal/updatelog"
)

// TriggerUpdater is the interface for triggering instance updates.
// Introduced to allow mock testing without real UV update calls.
type TriggerUpdater interface {
	TriggerUpdate(ctx context.Context) (*instance.UpdateResult, error)
}

// Notifier is the interface for sending update notifications.
// Defined here to enable mock injection for testing.
// The concrete notifier.Notifier satisfies this interface via duck typing.
type Notifier interface {
	Notify(title, message string) error
}

// TriggerHandler handles POST /api/v1/trigger-update requests
// API-01: HTTP endpoint for triggering updates
type TriggerHandler struct {
	instanceManager TriggerUpdater
	config          *config.APIConfig
	logger          *slog.Logger
	updateLogger    *updatelog.UpdateLogger // LOG-01, LOG-02: Update log recorder
	notifier        Notifier                // UNOTIF-01, UNOTIF-02: Pushover notification sender
	instanceCount   int                     // UNOTIF-01: instance count for start notification
}

// NewTriggerHandler creates a new trigger update handler
func NewTriggerHandler(im TriggerUpdater, cfg *config.APIConfig, logger *slog.Logger, ul *updatelog.UpdateLogger, notif Notifier, instanceCount int) *TriggerHandler {
	return &TriggerHandler{
		instanceManager: im,
		config:          cfg,
		logger:          logger.With("source", "api-trigger"),
		updateLogger:    ul,
		notifier:        notif,
		instanceCount:   instanceCount,
	}
}

// Handle handles POST /api/v1/trigger-update requests
// API-01: HTTP endpoint for triggering updates
// API-03: Executes full stop->update->start flow
// API-04: Returns JSON formatted result
// LOG-01: Records update log with timing and status
// LOG-02: Generates UUID v4 and returns update_id
func (h *TriggerHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// 1. Validate method
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST method is supported")
		return
	}

	// 2. Generate UUID v4 and record start time (LOG-02)
	updateID := uuid.New().String()
	startTime := time.Now().UTC()
	h.logger.Info("Update triggered", "update_id", updateID)

	// Send start notification (UNOTIF-01, D-04)
	// Per D-07: async, non-blocking. Per D-06: Notifier.Notify() handles IsEnabled() internally.
	if h.notifier != nil {
		title := "Nanobot 更新开始"
		message := fmt.Sprintf("触发来源: api-trigger\n待更新实例数: %d", h.instanceCount)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					h.logger.Error("开始通知 goroutine panic",
						"panic", r,
						"stack", string(debug.Stack()))
				}
			}()
			if err := h.notifier.Notify(title, message); err != nil {
				h.logger.Error("发送更新开始通知失败",
					"error", err,
					"update_id", updateID)
			}
		}()
	}

	// 3. Create context with timeout from config
	ctx, cancel := context.WithTimeout(r.Context(), h.config.Timeout)
	defer cancel()

	// 4. Execute update
	result, err := h.instanceManager.TriggerUpdate(ctx)
	endTime := time.Now().UTC() // Record end time immediately after

	// 5. Handle specific errors
	if err != nil {
		if errors.Is(err, instance.ErrUpdateInProgress) {
			h.logger.Warn("Update request rejected: update already in progress", "update_id", updateID)
			writeJSONError(w, http.StatusConflict, "conflict", "Update already in progress")
			return
		}
		if errors.Is(err, context.DeadlineExceeded) {
			h.logger.Error("Update operation timed out", "timeout", h.config.Timeout, "update_id", updateID)
			writeJSONError(w, http.StatusGatewayTimeout, "timeout",
				fmt.Sprintf("Update operation timed out after %v", h.config.Timeout))
			return
		}
		// Other errors (e.g., UV update failed)
		h.logger.Error("Update operation failed", "error", err, "update_id", updateID)
		writeJSONError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	// 6. Build and record UpdateLog (LOG-01, LOG-03, LOG-04)
	// Non-blocking: log recording failure does not affect response
	if h.updateLogger != nil {
		updateLog := updatelog.UpdateLog{
			ID:          updateID,
			StartTime:   startTime,
			EndTime:     endTime,
			Duration:    endTime.Sub(startTime).Milliseconds(),
			Status:      updatelog.DetermineStatus(result),
			Instances:   updatelog.BuildInstanceDetails(result),
			TriggeredBy: "api-trigger",
		}
		if recordErr := h.updateLogger.Record(updateLog); recordErr != nil {
			h.logger.Error("Failed to record update log", "error", recordErr, "update_id", updateID)
			// Don't return error to client - update operation itself was successful
		}
	}

	// Send completion notification (UNOTIF-02, D-05)
	// Per D-07: async, non-blocking. Per D-06: Notifier.Notify() handles IsEnabled() internally.
	if h.notifier != nil {
		status := updatelog.DetermineStatus(result)
		elapsed := endTime.Sub(startTime).Seconds()
		go func() {
			defer func() {
				if r := recover(); r != nil {
					h.logger.Error("完成通知 goroutine panic",
						"panic", r,
						"stack", string(debug.Stack()))
				}
			}()
			title := statusToTitle(status)
			msg := formatCompletionMessage(result, status, elapsed)
			if err := h.notifier.Notify(title, msg); err != nil {
				h.logger.Error("发送更新完成通知失败",
					"error", err,
					"update_id", updateID)
			}
		}()
	}

	// 7. Return JSON result with update_id
	h.logger.Info("Update completed", "success", !result.HasErrors(), "update_id", updateID)
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
		UpdateID:    updateID, // LOG-02: Return UUID v4 in response
		Success:     !result.HasErrors(),
		Stopped:     result.Stopped,
		Started:     result.Started,
		StopFailed:  stopFailed,
		StartFailed: startFailed,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode response", "error", err, "update_id", updateID)
	}
}

// APIUpdateResult is the JSON response structure for trigger-update endpoint
// API-04: JSON format response
// LOG-02: Includes update_id field
type APIUpdateResult struct {
	UpdateID    string                    `json:"update_id"`               // LOG-02: UUID v4 identifier
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

// statusToTitle converts UpdateStatus to notification title per D-02
func statusToTitle(status updatelog.UpdateStatus) string {
	switch status {
	case updatelog.StatusSuccess:
		return "Nanobot 更新成功"
	case updatelog.StatusPartialSuccess:
		return "Nanobot 更新部分成功"
	case updatelog.StatusFailed:
		return "Nanobot 更新失败"
	default:
		return "Nanobot 更新完成"
	}
}

// formatCompletionMessage builds the completion notification message per D-02
func formatCompletionMessage(result *instance.UpdateResult, status updatelog.UpdateStatus, elapsed float64) string {
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("耗时: %.1fs\n", elapsed))
	msg.WriteString(fmt.Sprintf("总实例数: %d\n", len(result.Stopped)+len(result.StopFailed)))
	msg.WriteString(fmt.Sprintf("成功: %d\n", len(result.Started)))
	failedCount := len(result.StopFailed) + len(result.StartFailed)
	if failedCount > 0 {
		msg.WriteString(fmt.Sprintf("失败: %d\n", failedCount))
		// Per D-02: list failed instance names (no detailed error messages)
		msg.WriteString("失败实例:\n")
		failedNames := make(map[string]bool)
		for _, err := range result.StopFailed {
			if !failedNames[err.InstanceName] {
				msg.WriteString(fmt.Sprintf("  - %s\n", err.InstanceName))
				failedNames[err.InstanceName] = true
			}
		}
		for _, err := range result.StartFailed {
			if !failedNames[err.InstanceName] {
				msg.WriteString(fmt.Sprintf("  - %s\n", err.InstanceName))
				failedNames[err.InstanceName] = true
			}
		}
	}
	return msg.String()
}

package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/HQGroup/nanobot-auto-updater/internal/updatelog"
)

// UpdateLogsResponse is the JSON response structure for GET /api/v1/update-logs (D-02)
type UpdateLogsResponse struct {
	Data []updatelog.UpdateLog `json:"data"`
	Meta PaginationMeta        `json:"meta"`
}

// PaginationMeta contains pagination metadata (D-02)
type PaginationMeta struct {
	Total  int `json:"total"`
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

// QueryHandler handles GET /api/v1/update-logs requests
type QueryHandler struct {
	updateLogger *updatelog.UpdateLogger
	logger       *slog.Logger
}

func NewQueryHandler(ul *updatelog.UpdateLogger, logger *slog.Logger) *QueryHandler {
	return &QueryHandler{
		updateLogger: ul,
		logger:       logger.With("source", "api-query"),
	}
}

func (h *QueryHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// Validate method
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET method is supported")
		return
	}

	// Nil-safe check (same pattern as TriggerHandler)
	if h.updateLogger == nil {
		h.logger.Error("UpdateLogger is nil")
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Update logger not available")
		return
	}

	// Parse pagination params with defaults (QUERY-03: limit default 20 max 100, offset default 0 min 0)
	limit := 20
	offset := 0

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			if n > 0 && n <= 100 {
				limit = n
			} else if n > 100 {
				limit = 100
			}
			// n <= 0: keep default 20
		}
		// non-numeric: keep default 20
	}

	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
		// non-numeric or negative: keep default 0
	}

	// Get paginated data (GetPage from Plan 01)
	logs, total := h.updateLogger.GetPage(limit, offset)

	// Build response (D-02 nested structure)
	response := UpdateLogsResponse{
		Data: logs,
		Meta: PaginationMeta{
			Total:  total,
			Offset: offset,
			Limit:  limit,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode query response", "error", err)
	}
}

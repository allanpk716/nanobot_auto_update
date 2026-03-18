package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/instance"
	"github.com/HQGroup/nanobot-auto-updater/internal/logbuffer"
)

// SSEHandler handles Server-Sent Events connections for log streaming
// SSE-01: Provides /api/v1/logs/:instance/stream endpoint
type SSEHandler struct {
	instanceManager *instance.InstanceManager
	logger          *slog.Logger
}

// NewSSEHandler creates a new SSE handler
func NewSSEHandler(im *instance.InstanceManager, logger *slog.Logger) *SSEHandler {
	return &SSEHandler{
		instanceManager: im,
		logger:          logger.With("component", "sse-handler"),
	}
}

// Handle handles SSE log streaming requests
// SSE-01: Endpoint path /api/v1/logs/:instance/stream
// SSE-02: Uses Server-Sent Events protocol
// SSE-04: Detects client disconnect and cleanup resources
// SSE-05: Sends history logs on connection
// SSE-06: Distinguishes stdout and stderr with different event types
func (h *SSEHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// 1. Set SSE HTTP headers (SSE-02)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// 2. Check Flusher support
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// 3. Extract instance parameter from URL path (SSE-01)
	instanceName := r.PathValue("instance")
	if instanceName == "" {
		http.Error(w, "Instance name required", http.StatusBadRequest)
		return
	}

	// 4. Get LogBuffer for the instance (ERR-04: return 404 if not found)
	logBuffer, err := h.instanceManager.GetLogBuffer(instanceName)
	if err != nil {
		h.logger.Warn("Instance not found", "instance", instanceName, "error", err)
		http.Error(w, fmt.Sprintf("Instance %s not found", instanceName), http.StatusNotFound)
		return
	}

	// 5. Subscribe to log stream (SSE-05: automatically sends history logs)
	logChan := logBuffer.Subscribe()
	defer logBuffer.Unsubscribe(logChan) // SSE-04: cleanup on disconnect

	h.logger.Info("SSE client connected", "instance", instanceName)

	// 6. Send connected event
	fmt.Fprintf(w, "event: connected\ndata: {\"instance\":\"%s\"}\n\n", instanceName)
	flusher.Flush()

	// 7. Monitor client disconnect (SSE-04)
	ctx := r.Context()

	// 8. Start heartbeat ticker (SSE-03: every 30 seconds)
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	// 9. Main loop: forward logs and heartbeat
	for {
		select {
		case <-ctx.Done():
			// SSE-04: Client disconnected
			h.logger.Info("SSE client disconnected", "instance", instanceName, "reason", ctx.Err())
			return // defer automatically calls Unsubscribe

		case entry, ok := <-logChan:
			if !ok {
				// LogBuffer channel closed (instance deleted)
				h.logger.Info("LogBuffer channel closed", "instance", instanceName)
				return
			}

			// SSE-06: Send log event with proper event type
			h.writeSSEEvent(w, flusher, entry)

		case <-heartbeatTicker.C:
			// SSE-03: Send heartbeat comment
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
			h.logger.Debug("SSE heartbeat sent", "instance", instanceName)
		}
	}
}

// writeSSEEvent writes an SSE event to the response writer
// SSE-06: Distinguishes stdout and stderr with different event types
func (h *SSEHandler) writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, entry logbuffer.LogEntry) {
	// Set event type based on source (SSE-06)
	eventType := "stdout"
	if entry.Source == "stderr" {
		eventType = "stderr"
	}

	// Write SSE event (standard format)
	fmt.Fprintf(w, "event: %s\n", eventType)
	fmt.Fprintf(w, "data: %s\n\n", entry.Content) // Double newline ends the event

	// Flush immediately (don't buffer)
	flusher.Flush()
}

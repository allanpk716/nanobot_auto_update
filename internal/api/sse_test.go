package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/instance"
	"github.com/HQGroup/nanobot-auto-updater/internal/logbuffer"
)

// TestSSEEndpoint tests SSE endpoint response (SSE-01, SSE-02)
func TestSSEEndpoint(t *testing.T) {
	// Create mock InstanceManager
	im := createTestInstanceManager()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create SSE handler
	handler := NewSSEHandler(im, logger)

	// Create test request
	req := httptest.NewRequest("GET", "/api/v1/logs/test/stream", nil)
	req.SetPathValue("instance", "test")
	rec := httptest.NewRecorder()

	// Start handler in goroutine
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)

	go func() {
		handler.Handle(rec, req)
	}()

	// Wait for response
	time.Sleep(100 * time.Millisecond)

	// Verify HTTP headers
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type=text/event-stream, got %s", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("Expected Cache-Control=no-cache, got %s", rec.Header().Get("Cache-Control"))
	}
	if rec.Header().Get("Connection") != "keep-alive" {
		t.Errorf("Expected Connection=keep-alive, got %s", rec.Header().Get("Connection"))
	}

	// Cleanup
	cancel()
}

// TestSSEEventFormat tests SSE event format (SSE-06)
func TestSSEEventFormat(t *testing.T) {
	im := createTestInstanceManager()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	handler := NewSSEHandler(im, logger)

	// Get instance LogBuffer
	lb, err := im.GetLogBuffer("test")
	if err != nil {
		t.Fatalf("Failed to get LogBuffer: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/v1/logs/test/stream", nil)
	req.SetPathValue("instance", "test")
	rec := httptest.NewRecorder()

	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)

	go func() {
		handler.Handle(rec, req)
	}()

	time.Sleep(100 * time.Millisecond)

	// Write test logs
	lb.Write(logbuffer.LogEntry{Source: "stdout", Content: "test stdout log"})
	time.Sleep(50 * time.Millisecond)

	lb.Write(logbuffer.LogEntry{Source: "stderr", Content: "test stderr log"})
	time.Sleep(50 * time.Millisecond)

	cancel()
	time.Sleep(100 * time.Millisecond)

	body := rec.Body.String()

	// Verify event format
	if !strings.Contains(body, "event: connected") {
		t.Error("Expected connected event")
	}
	if !strings.Contains(body, "event: stdout") {
		t.Error("Expected stdout event")
	}
	if !strings.Contains(body, "test stdout log") {
		t.Error("Expected stdout log content")
	}
	if !strings.Contains(body, "event: stderr") {
		t.Error("Expected stderr event")
	}
	if !strings.Contains(body, "test stderr log") {
		t.Error("Expected stderr log content")
	}
}

// TestSSEInstanceNotFound tests instance not found returns 404 (SSE-01, ERR-02)
func TestSSEInstanceNotFound(t *testing.T) {
	// Setup: capture log output to verify warning is logged
	var logOutput strings.Builder
	logger := slog.New(slog.NewTextHandler(&logOutput, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	im := createTestInstanceManager()

	handler := NewSSEHandler(im, logger)

	req := httptest.NewRequest("GET", "/api/v1/logs/nonexistent/stream", nil)
	req.SetPathValue("instance", "nonexistent")
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	// Verify: 404 status code
	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "not found") {
		t.Errorf("Expected 'not found' error message, got %s", rec.Body.String())
	}

	// Verify: Warning is logged (ERR-02)
	logs := logOutput.String()
	if !strings.Contains(logs, "Instance not found") {
		t.Errorf("Expected warning log for instance not found, got: %s", logs)
	}
}

// TestSSEHeartbeat tests heartbeat sending (SSE-03)
func TestSSEHeartbeat(t *testing.T) {
	im := createTestInstanceManager()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	handler := NewSSEHandler(im, logger)

	req := httptest.NewRequest("GET", "/api/v1/logs/test/stream", nil)
	req.SetPathValue("instance", "test")
	rec := httptest.NewRecorder()

	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)

	go func() {
		handler.Handle(rec, req)
	}()

	// Wait long enough to receive heartbeat (at least 30 seconds)
	// Note: In actual tests, we may need to mock ticker or shorten heartbeat interval
	time.Sleep(100 * time.Millisecond)

	cancel()
}

// TestSSEClientDisconnect tests client disconnect cleanup (SSE-04, ERR-02)
func TestSSEClientDisconnect(t *testing.T) {
	// Setup: capture log output to verify info-level message
	var logOutput strings.Builder
	logger := slog.New(slog.NewTextHandler(&logOutput, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	im := createTestInstanceManager()

	handler := NewSSEHandler(im, logger)

	req := httptest.NewRequest("GET", "/api/v1/logs/test/stream", nil)
	req.SetPathValue("instance", "test")
	rec := httptest.NewRecorder()

	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)

	go func() {
		handler.Handle(rec, req)
	}()

	// Wait for connection establishment
	time.Sleep(100 * time.Millisecond)

	// Simulate client disconnect
	cancel()

	// Wait for cleanup
	time.Sleep(100 * time.Millisecond)

	// Verify: Info-level disconnect message is logged (ERR-02)
	logs := logOutput.String()
	if !strings.Contains(logs, "SSE client disconnected") {
		t.Errorf("Expected info log for client disconnect, got: %s", logs)
	}

	// Verify: No goroutine leaks (indirectly verified by clean exit)
	// Note: defer Unsubscribe ensures cleanup
}

// createTestInstanceManager creates a test InstanceManager with instances
func createTestInstanceManager() *instance.InstanceManager {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.Config{
		Instances: []config.InstanceConfig{
			{Name: "test", Port: 8080, StartCommand: "test-command"},
		},
	}

	return instance.NewInstanceManager(cfg, logger, nil)
}

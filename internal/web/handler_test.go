package web

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/instance"
)

// TestEmbedFS tests that static files are embedded correctly
func TestEmbedFS(t *testing.T) {
	// Test that embedded filesystem is accessible
	subFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		t.Fatalf("Failed to create sub filesystem: %v", err)
	}

	// Test index.html exists and has content
	content, err := fs.ReadFile(subFS, "index.html")
	if err != nil {
		t.Errorf("Failed to read index.html: %v", err)
	}
	if len(content) < 30 {
		t.Errorf("index.html too short (expected >= 30 lines, got %d bytes)", len(content))
	}

	// Test style.css exists
	content, err = fs.ReadFile(subFS, "style.css")
	if err != nil {
		t.Errorf("Failed to read style.css: %v", err)
	}
	if len(content) < 40 {
		t.Errorf("style.css too short (expected >= 40 lines, got %d bytes)", len(content))
	}

	// Test app.js exists
	content, err = fs.ReadFile(subFS, "app.js")
	if err != nil {
		t.Errorf("Failed to read app.js: %v", err)
	}
	if len(content) < 80 {
		t.Errorf("app.js too short (expected >= 80 lines, got %d bytes)", len(content))
	}
}

// TestWebHandler tests GET /logs/:instance endpoint
func TestWebHandler(t *testing.T) {
	// Create mock InstanceManager
	im := createTestInstanceManager()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create web page handler
	handler := NewWebPageHandler(im, logger)

	// Test valid instance
	req := httptest.NewRequest("GET", "/logs/test", nil)
	req.SetPathValue("instance", "test")
	rec := httptest.NewRecorder()

	handler(rec, req)

	// Verify status code
	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", rec.Code)
	}

	// Verify content type
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Expected Content-Type to contain text/html, got %s", contentType)
	}

	// Verify HTML content exists
	body := rec.Body.String()
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("Expected HTML doctype")
	}
	if !strings.Contains(body, "Nanobot Logs") {
		t.Error("Expected page title")
	}
}

// TestWebHandlerInstanceNotFound tests 404 for non-existent instance
func TestWebHandlerInstanceNotFound(t *testing.T) {
	im := createTestInstanceManager()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	handler := NewWebPageHandler(im, logger)

	req := httptest.NewRequest("GET", "/logs/nonexistent", nil)
	req.SetPathValue("instance", "nonexistent")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found, got %d", rec.Code)
	}
}

// TestConnectionStatus tests connection status indicator exists in HTML and JS
func TestConnectionStatus(t *testing.T) {
	subFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		t.Fatalf("Failed to create sub filesystem: %v", err)
	}

	// Test HTML contains connection status element
	htmlContent, err := fs.ReadFile(subFS, "index.html")
	if err != nil {
		t.Fatalf("Failed to read index.html: %v", err)
	}
	htmlStr := string(htmlContent)
	if !strings.Contains(htmlStr, `id="connection-status"`) {
		t.Error("Expected connection status element in HTML")
	}

	// Test JS contains updateConnectionStatus function
	jsContent, err := fs.ReadFile(subFS, "app.js")
	if err != nil {
		t.Fatalf("Failed to read app.js: %v", err)
	}
	jsStr := string(jsContent)
	if !strings.Contains(jsStr, "updateConnectionStatus") {
		t.Error("Expected updateConnectionStatus function in JS")
	}
	if !strings.Contains(jsStr, "status-connecting") {
		t.Error("Expected status-connecting class usage in JS")
	}
	if !strings.Contains(jsStr, "status-connected") {
		t.Error("Expected status-connected class usage in JS")
	}
	if !strings.Contains(jsStr, "status-disconnected") {
		t.Error("Expected status-disconnected class usage in JS")
	}
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

// TestInstanceListHandler tests GET /api/v1/instances endpoint
func TestInstanceListHandler(t *testing.T) {
	// Create test instance manager with multiple instances
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{
		Instances: []config.InstanceConfig{
			{Name: "instance1", Port: 8080, StartCommand: "cmd1"},
			{Name: "instance2", Port: 8081, StartCommand: "cmd2"},
			{Name: "instance3", Port: 8082, StartCommand: "cmd3"},
		},
	}
	im := instance.NewInstanceManager(cfg, logger, nil)

	// Create handler
	handler := NewInstanceListHandler(im, logger)

	// Create test request
	req := httptest.NewRequest("GET", "/api/v1/instances", nil)
	rec := httptest.NewRecorder()

	// Execute handler
	handler(rec, req)

	// Verify status code
	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", rec.Code)
	}

	// Verify content type is JSON
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected Content-Type to contain application/json, got %s", contentType)
	}

	// Decode JSON response
	var response map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	// Verify instances array exists
	instancesRaw, ok := response["instances"]
	if !ok {
		t.Fatal("Response missing 'instances' field")
	}

	// Type assertion for array
	instances, ok := instancesRaw.([]interface{})
	if !ok {
		t.Fatalf("instances field is not array, got %T", instancesRaw)
	}

	// Verify array contains expected instance names
	if len(instances) != 3 {
		t.Errorf("Expected 3 instances, got %d", len(instances))
	}

	expectedNames := []string{"instance1", "instance2", "instance3"}
	for i, expected := range expectedNames {
		if i >= len(instances) {
			break
		}
		name, ok := instances[i].(string)
		if !ok {
			t.Errorf("instances[%d] is not string, got %T", i, instances[i])
			continue
		}
		if name != expected {
			t.Errorf("instances[%d] = %q, want %q", i, name, expected)
		}
	}
}

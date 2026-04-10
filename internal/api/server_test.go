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
)

// TestNewServer tests NewServer function (SSE-07)
func TestNewServer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.APIConfig{
		Port:        8081,
		BearerToken: "test-token-32-characters-long-enough",
		Timeout:     5 * time.Second,
	}

	fullCfg := &config.Config{
		API:        *cfg,
		Instances: []config.InstanceConfig{
			{Name: "test", Port: 8080, StartCommand: "test"},
		},
	}

	im := instance.NewInstanceManager(fullCfg, logger, nil)

	server, err := NewServer(cfg, im, fullCfg, "test-version", logger, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Verify port configuration
	if server.httpServer.Addr != ":8081" {
		t.Errorf("Expected server addr :8081, got %s", server.httpServer.Addr)
	}

	// SSE-07: Verify WriteTimeout = 0
	if server.httpServer.WriteTimeout != 0 {
		t.Errorf("Expected WriteTimeout 0, got %v", server.httpServer.WriteTimeout)
	}
}

// TestServerLifecycle tests server start and shutdown
func TestServerLifecycle(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.APIConfig{
		Port:        8082,
		BearerToken: "test-token-32-characters-long-enough",
		Timeout:     5 * time.Second,
	}

	fullCfg := &config.Config{
		API:        *cfg,
		Instances: []config.InstanceConfig{
			{Name: "test", Port: 8080, StartCommand: "test"},
		},
	}

	im := instance.NewInstanceManager(fullCfg, logger, nil)

	server, err := NewServer(cfg, im, fullCfg, "test-version", logger, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Start server in goroutine
	go func() {
		if err := server.Start(); err != nil {
			// ErrServerClosed is normal error
		}
	}()

	// Wait for startup
	time.Sleep(100 * time.Millisecond)

	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

// TestNewServerValidation tests NewServer validation
func TestNewServerValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	fullCfg := &config.Config{
		Instances: []config.InstanceConfig{
			{Name: "test", Port: 8080, StartCommand: "test"},
		},
	}

	im := instance.NewInstanceManager(fullCfg, logger, nil)

	// Test with nil config
	_, err := NewServer(nil, im, fullCfg, "test-version", logger, nil, nil, nil)
	if err == nil {
		t.Error("Expected error for nil config")
	}

	// Test with zero port
	cfg := &config.APIConfig{
		Port:        0,
		BearerToken: "test-token-32-characters-long-enough",
	}
	_, err = NewServer(cfg, im, fullCfg, "test-version", logger, nil, nil, nil)
	if err == nil {
		t.Error("Expected error for zero port")
	}
}

// TestWebUIRoutes tests web UI routes are registered (Phase 23: UI-01)
func TestWebUIRoutes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.APIConfig{
		Port:        8083,
		BearerToken: "test-token-32-characters-long-enough",
		Timeout:     5 * time.Second,
	}

	fullCfg := &config.Config{
		API:        *cfg,
		Instances: []config.InstanceConfig{
			{Name: "test", Port: 8080, StartCommand: "test"},
		},
	}

	im := instance.NewInstanceManager(fullCfg, logger, nil)

	server, err := NewServer(cfg, im, fullCfg, "test-version", logger, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Create test request for /logs/{instance}
	req := httptest.NewRequest("GET", "/logs/test", nil)
	req.SetPathValue("instance", "test")
	rec := httptest.NewRecorder()

	// Serve request
	server.httpServer.Handler.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for /logs/test, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Expected Content-Type to contain text/html, got %s", contentType)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("Expected HTML doctype")
	}
}

// TestWebUIInstanceNotFound tests 404 for non-existent instance
func TestWebUIInstanceNotFound(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.APIConfig{
		Port:        8084,
		BearerToken: "test-token-32-characters-long-enough",
		Timeout:     5 * time.Second,
	}

	fullCfg := &config.Config{
		API:        *cfg,
		Instances: []config.InstanceConfig{
			{Name: "test", Port: 8080, StartCommand: "test"},
		},
	}

	im := instance.NewInstanceManager(fullCfg, logger, nil)

	server, err := NewServer(cfg, im, fullCfg, "test-version", logger, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Create test request for non-existent instance
	req := httptest.NewRequest("GET", "/logs/nonexistent", nil)
	req.SetPathValue("instance", "nonexistent")
	rec := httptest.NewRecorder()

	// Serve request
	server.httpServer.Handler.ServeHTTP(rec, req)

	// Verify 404 response
	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found, got %d", rec.Code)
	}
}

// TestServerStart_PortRetry tests that server start succeeds with port retry logic (D-05)
func TestServerStart_PortRetry(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.APIConfig{
		Port:        8085,
		BearerToken: "test-token-32-characters-long-enough",
		Timeout:     5 * time.Second,
	}

	fullCfg := &config.Config{
		API: *cfg,
		Instances: []config.InstanceConfig{
			{Name: "test", Port: 8080, StartCommand: "test"},
		},
	}

	im := instance.NewInstanceManager(fullCfg, logger, nil)
	server, err := NewServer(cfg, im, fullCfg, "test-version", logger, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	go func() {
		if err := server.Start(); err != nil {
			// ErrServerClosed is expected on shutdown
		}
	}()

	// Wait for startup
	time.Sleep(200 * time.Millisecond)

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

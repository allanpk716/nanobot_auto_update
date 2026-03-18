package api

import (
	"context"
	"log/slog"
	"os"
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

	im := instance.NewInstanceManager(&config.Config{
		Instances: []config.InstanceConfig{
			{Name: "test", Port: 8080, StartCommand: "test"},
		},
	}, logger)

	server, err := NewServer(cfg, im, logger)
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

	im := instance.NewInstanceManager(&config.Config{
		Instances: []config.InstanceConfig{
			{Name: "test", Port: 8080, StartCommand: "test"},
		},
	}, logger)

	server, err := NewServer(cfg, im, logger)
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

	im := instance.NewInstanceManager(&config.Config{
		Instances: []config.InstanceConfig{
			{Name: "test", Port: 8080, StartCommand: "test"},
		},
	}, logger)

	// Test with nil config
	_, err := NewServer(nil, im, logger)
	if err == nil {
		t.Error("Expected error for nil config")
	}

	// Test with zero port
	cfg := &config.APIConfig{
		Port:        0,
		BearerToken: "test-token-32-characters-long-enough",
	}
	_, err = NewServer(cfg, im, logger)
	if err == nil {
		t.Error("Expected error for zero port")
	}
}

//go:build windows

package lifecycle

import (
	"encoding/json"
	"log/slog"
	"net"
	"os"
	"testing"
)

// TestCheckUpdateStateInternal_Cleanup verifies "cleanup" returned when .update-success exists,
// and both .old and .update-success files are removed.
func TestCheckUpdateStateInternal_Cleanup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create temp directory and fake exe path
	tmpDir := t.TempDir()
	exePath := tmpDir + "\\test.exe"

	// Create .old file
	oldPath := exePath + ".old"
	if err := os.WriteFile(oldPath, []byte("old-binary"), 0644); err != nil {
		t.Fatalf("Failed to create .old file: %v", err)
	}

	// Create .update-success marker with valid JSON
	successPath := exePath + ".update-success"
	marker := map[string]string{"new_version": "v1.2.3"}
	markerData, _ := json.Marshal(marker)
	if err := os.WriteFile(successPath, markerData, 0644); err != nil {
		t.Fatalf("Failed to create .update-success file: %v", err)
	}

	// Call checkUpdateStateInternal
	result := checkUpdateStateInternal(exePath, logger)

	// Verify result
	if result != "cleanup" {
		t.Errorf("Expected 'cleanup', got '%s'", result)
	}

	// Verify .old file was removed
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("Expected .old file to be removed")
	}

	// Verify .update-success file was removed
	if _, err := os.Stat(successPath); !os.IsNotExist(err) {
		t.Error("Expected .update-success file to be removed")
	}
}

// TestCheckUpdateStateInternal_Recover verifies "recover" returned when .old exists without .update-success.
func TestCheckUpdateStateInternal_Recover(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create temp directory and fake exe path
	tmpDir := t.TempDir()
	exePath := tmpDir + "\\test.exe"

	// Create .old file only (no .update-success)
	oldPath := exePath + ".old"
	if err := os.WriteFile(oldPath, []byte("old-binary"), 0644); err != nil {
		t.Fatalf("Failed to create .old file: %v", err)
	}

	// Call checkUpdateStateInternal
	result := checkUpdateStateInternal(exePath, logger)

	// Verify result
	if result != "recover" {
		t.Errorf("Expected 'recover', got '%s'", result)
	}

	// Verify .old file still exists (not cleaned up in internal function)
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		t.Error("Expected .old file to still exist")
	}
}

// TestCheckUpdateStateInternal_Normal verifies "normal" returned when neither .old nor .update-success exist.
func TestCheckUpdateStateInternal_Normal(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create temp directory and fake exe path (no files created)
	tmpDir := t.TempDir()
	exePath := tmpDir + "\\test.exe"

	// Call checkUpdateStateInternal
	result := checkUpdateStateInternal(exePath, logger)

	// Verify result
	if result != "normal" {
		t.Errorf("Expected 'normal', got '%s'", result)
	}
}

// TestCheckUpdateStateInternal_CorruptMarker verifies "recover" returned when .update-success
// exists but contains invalid JSON (corrupt marker treated as missing).
func TestCheckUpdateStateInternal_CorruptMarker(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create temp directory and fake exe path
	tmpDir := t.TempDir()
	exePath := tmpDir + "\\test.exe"

	// Create .old file
	oldPath := exePath + ".old"
	if err := os.WriteFile(oldPath, []byte("old-binary"), 0644); err != nil {
		t.Fatalf("Failed to create .old file: %v", err)
	}

	// Create .update-success with invalid JSON
	successPath := exePath + ".update-success"
	if err := os.WriteFile(successPath, []byte("not-valid-json"), 0644); err != nil {
		t.Fatalf("Failed to create corrupt .update-success file: %v", err)
	}

	// Call checkUpdateStateInternal
	result := checkUpdateStateInternal(exePath, logger)

	// Verify result falls through to recovery check
	if result != "recover" {
		t.Errorf("Expected 'recover' (corrupt marker treated as missing), got '%s'", result)
	}
}

// TestListenWithRetry_Success verifies listener returned on first try with a free port.
func TestListenWithRetry_Success(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Find a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find free port: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// ListenWithRetry should succeed on first try
	l, err := ListenWithRetry(addr, logger)
	if err != nil {
		t.Fatalf("ListenWithRetry failed: %v", err)
	}
	defer l.Close()

	// Verify the listener works
	if l.Addr().Network() != "tcp" {
		t.Errorf("Expected tcp network, got %s", l.Addr().Network())
	}
}

// TestListenWithRetry_AfterClose verifies ListenWithRetry works on a port that was just released.
func TestListenWithRetry_AfterClose(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Find a free port, bind it, then close
	l1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find free port: %v", err)
	}
	addr := l1.Addr().String()
	l1.Close()

	// ListenWithRetry should succeed now that port is free
	l2, err := ListenWithRetry(addr, logger)
	if err != nil {
		t.Fatalf("ListenWithRetry failed after close: %v", err)
	}
	defer l2.Close()
}

// TestCheckUpdateStateInternal_EmptyOldFile verifies that an empty .old file
// (size 0) is NOT treated as needing recovery.
func TestCheckUpdateStateInternal_EmptyOldFile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create temp directory and fake exe path
	tmpDir := t.TempDir()
	exePath := tmpDir + "\\test.exe"

	// Create empty .old file
	oldPath := exePath + ".old"
	if err := os.WriteFile(oldPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty .old file: %v", err)
	}

	// Call checkUpdateStateInternal
	result := checkUpdateStateInternal(exePath, logger)

	// Empty .old should be treated as normal (no recovery needed)
	if result != "normal" {
		t.Errorf("Expected 'normal' for empty .old file, got '%s'", result)
	}
}

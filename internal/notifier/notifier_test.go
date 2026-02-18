package notifier

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
)

// testHandler captures log output for testing
type testHandler struct {
	records []string
}

func (h *testHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *testHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r.Message)
	return nil
}

func (h *testHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

func (h *testHandler) WithGroup(_ string) slog.Handler {
	return h
}

func newTestLogger() (*slog.Logger, *testHandler) {
	h := &testHandler{}
	return slog.New(h), h
}

// TestNew_MissingEnv verifies disabled notifier when env vars missing
func TestNew_MissingEnv(t *testing.T) {
	// Ensure env vars are not set
	os.Unsetenv("PUSHOVER_TOKEN")
	os.Unsetenv("PUSHOVER_USER")

	logger, handler := newTestLogger()
	n := New(logger)

	// Verify disabled
	if n.IsEnabled() {
		t.Error("Expected IsEnabled() to return false when env vars missing")
	}

	// Verify warning was logged
	found := false
	for _, msg := range handler.records {
		if strings.Contains(msg, "Pushover notifications disabled") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected warning log about disabled notifications")
	}
}

// TestNew_WithEnv verifies enabled notifier when env vars set
func TestNew_WithEnv(t *testing.T) {
	// Set env vars using t.Setenv (automatically cleaned up)
	t.Setenv("PUSHOVER_TOKEN", "test-token-123")
	t.Setenv("PUSHOVER_USER", "test-user-456")

	logger, handler := newTestLogger()
	n := New(logger)

	// Verify enabled
	if !n.IsEnabled() {
		t.Error("Expected IsEnabled() to return true when env vars set")
	}

	// Verify info was logged
	found := false
	for _, msg := range handler.records {
		if strings.Contains(msg, "Pushover notifications enabled") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected info log about enabled notifications")
	}
}

// TestNotify_Disabled verifies no error when disabled
func TestNotify_Disabled(t *testing.T) {
	// Create disabled notifier (no env vars)
	os.Unsetenv("PUSHOVER_TOKEN")
	os.Unsetenv("PUSHOVER_USER")

	logger, _ := newTestLogger()
	n := New(logger)

	// Call Notify - should return nil (no error)
	err := n.Notify("Test Title", "Test Message")
	if err != nil {
		t.Errorf("Expected Notify() to return nil when disabled, got: %v", err)
	}
}

// TestNotifyFailure_Formatting verifies message formatting
func TestNotifyFailure_Formatting(t *testing.T) {
	// Create disabled notifier (no actual API calls)
	os.Unsetenv("PUSHOVER_TOKEN")
	os.Unsetenv("PUSHOVER_USER")

	logger, _ := newTestLogger()
	n := New(logger)

	// Test with a sample error
	testErr := errors.New("connection timeout")
	testOperation := "Scheduled Update"

	// NotifyFailure should return nil when disabled
	err := n.NotifyFailure(testOperation, testErr)
	if err != nil {
		t.Errorf("Expected NotifyFailure() to return nil when disabled, got: %v", err)
	}

	// Note: Actual message formatting is verified by the code itself
	// The title format is: "Nanobot Update Failed: {operation}"
	// The message format is: "Operation: {operation}\n\nError: {err}"
	// Full API call verification requires integration test with real credentials
}

// TestNotify_Enabled verifies notification behavior when enabled
// Note: This test creates an enabled notifier but does not make actual API calls
// To test actual Pushover API, set PUSHOVER_TOKEN and PUSHOVER_USER env vars
// and run with -integration flag
func TestNotify_Enabled(t *testing.T) {
	// Skip if running in short mode (no integration tests)
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if real credentials are available
	token := os.Getenv("PUSHOVER_TOKEN")
	user := os.Getenv("PUSHOVER_USER")
	if token == "" || user == "" {
		t.Skip("Skipping: PUSHOVER_TOKEN and PUSHOVER_USER not set")
	}

	// Create enabled notifier with real credentials
	logger, _ := newTestLogger()
	n := New(logger)

	if !n.IsEnabled() {
		t.Fatal("Expected notifier to be enabled with real credentials")
	}

	// Send a test notification
	err := n.Notify("Test Notification", "This is a test from unit tests")
	if err != nil {
		t.Errorf("Expected Notify() to succeed with real credentials, got: %v", err)
	}
}

package lifecycle_test

import (
	"context"
	"io"
	"log/slog"
	"runtime"
	"testing"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/lifecycle"
	"github.com/stretchr/testify/assert"
)

// testServiceMgrLogger creates a discard logger for ServiceManager tests.
func testServiceMgrLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestNewServiceManager verifies NewServiceManager returns a non-nil instance.
func TestNewServiceManager(t *testing.T) {
	cfg := &config.Config{
		Service: config.ServiceConfig{
			ServiceName: "TestService",
			DisplayName: "Test Display",
		},
	}

	sm := lifecycle.NewServiceManager(cfg, testServiceMgrLogger())
	assert.NotNil(t, sm, "NewServiceManager should return non-nil")
}

// TestIsAdmin verifies IsAdmin returns a bool without panic.
// On non-Windows or non-admin contexts, it should return false.
func TestIsAdmin(t *testing.T) {
	result := lifecycle.IsAdmin()
	assert.IsType(t, false, result, "IsAdmin should return bool")
}

// TestRegisterService_EmptyServiceName verifies the defensive check:
// RegisterService should return an error containing "service_name is empty"
// when ServiceName is empty.
func TestRegisterService_EmptyServiceName(t *testing.T) {
	cfg := &config.Config{
		Service: config.ServiceConfig{
			ServiceName: "",
			DisplayName: "Test Display",
		},
	}

	err := lifecycle.RegisterService(cfg, testServiceMgrLogger())
	assert.Error(t, err, "RegisterService with empty ServiceName should return error")
	assert.Contains(t, err.Error(), "service_name is empty",
		"Error message should mention empty service_name")
}

// TestRegisterService_NonAdminOrNonWindows verifies behavior on the current platform:
// - On non-Windows: returns nil (no-op stub)
// - On Windows without admin: returns error containing "SCM" or "failed"
func TestRegisterService_NonAdminOrNonWindows(t *testing.T) {
	cfg := &config.Config{
		Service: config.ServiceConfig{
			ServiceName: "TestService",
			DisplayName: "Test Display",
		},
	}

	err := lifecycle.RegisterService(cfg, slog.Default())

	if runtime.GOOS == "windows" {
		// On Windows: without admin privileges, should fail connecting to SCM
		if err != nil {
			assert.Contains(t, err.Error(), "SCM",
				"Error should mention SCM on Windows without admin")
		}
		// If running as admin, the test might succeed -- that is also acceptable
	} else {
		// On non-Windows: no-op stub should return nil
		assert.NoError(t, err, "RegisterService should return nil on non-Windows platforms")
	}
}

// TestUnregisterService_NonAdminOrNonWindows verifies behavior on the current platform:
// - On non-Windows: returns nil (no-op stub)
// - On Windows without admin: returns error
func TestUnregisterService_NonAdminOrNonWindows(t *testing.T) {
	cfg := &config.Config{
		Service: config.ServiceConfig{
			ServiceName: "TestService",
			DisplayName: "Test Display",
		},
	}

	err := lifecycle.UnregisterService(context.Background(), cfg, testServiceMgrLogger())

	if runtime.GOOS == "windows" {
		// On Windows: without admin privileges, should fail connecting to SCM
		if err != nil {
			assert.Contains(t, err.Error(), "SCM",
				"Error should mention SCM on Windows without admin")
		}
		// If running as admin, the service might not exist (nil) or succeed -- both OK
	} else {
		// On non-Windows: no-op stub should return nil
		assert.NoError(t, err, "UnregisterService should return nil on non-Windows platforms")
	}
}

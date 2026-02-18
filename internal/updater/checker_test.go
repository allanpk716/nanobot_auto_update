//go:build windows

package updater

import (
	"strings"
	"testing"
)

// TestCheckUvInstalled tests that CheckUvInstalled works correctly.
// This test will pass if uv is installed on the development machine.
// If uv is not installed, we log the result but don't fail the test
// since this is an environmental dependency.
func TestCheckUvInstalled(t *testing.T) {
	err := CheckUvInstalled()
	if err != nil {
		// Log but don't fail - uv might not be installed on test machine
		t.Logf("CheckUvInstalled returned error (uv may not be installed): %v", err)
		// Still verify the error is not nil when uv is not found
		if !strings.Contains(err.Error(), "uv is not installed") {
			t.Errorf("Expected error message to contain 'uv is not installed', got: %v", err)
		}
	} else {
		t.Log("uv is installed and available")
	}
}

// TestCheckUvInstalledErrorMessage verifies the error message format
// for the ErrNotFound case.
func TestCheckUvInstalledErrorMessage(t *testing.T) {
	// We can't easily mock exec.LookPath, so we verify the error message
	// contains the expected content by checking the actual error message
	err := CheckUvInstalled()
	if err != nil {
		errMsg := err.Error()
		// Verify error message contains key information
		if !strings.Contains(errMsg, "uv is not installed") {
			t.Errorf("Error message should contain 'uv is not installed', got: %s", errMsg)
		}
		if !strings.Contains(errMsg, "https://docs.astral.sh/uv/") {
			t.Errorf("Error message should contain installation URL, got: %s", errMsg)
		}
		t.Logf("Error message format verified: %s", errMsg)
	} else {
		t.Log("uv is installed - skipping error message test")
	}
}

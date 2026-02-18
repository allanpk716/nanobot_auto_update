package main

import (
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// TestVersionFlag verifies --version exits immediately with version string
func TestVersionFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build the binary first
	buildCmd := exec.Command("go", "build", "-o", "test-updater.exe", "./cmd/main.go")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}

	// Run with --version flag
	cmd := exec.Command("./test-updater.exe", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Expected --version to exit cleanly, got error: %v", err)
	}

	// Verify output contains version string
	if !strings.Contains(string(output), "nanobot-auto-updater") {
		t.Errorf("Expected version output to contain 'nanobot-auto-updater', got: %s", string(output))
	}
}

// TestHelpFlag verifies -h/--help shows usage information
func TestHelpFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name string
		flag string
	}{
		{"short help flag", "-h"},
		{"long help flag", "--help"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("go", "run", "./cmd/main.go", tt.flag)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Expected %s to exit cleanly, got error: %v", tt.flag, err)
			}

			// Verify output contains usage information
			outputStr := string(output)
			if !strings.Contains(outputStr, "Usage:") && !strings.Contains(outputStr, "Options:") {
				t.Errorf("Expected help output to contain 'Usage:' or 'Options:', got: %s", outputStr)
			}
		})
	}
}

// TestInvalidCronFlag verifies invalid cron expression exits with error
func TestInvalidCronFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cmd := exec.Command("go", "run", "./cmd/main.go", "-cron", "invalid-cron")
	output, err := cmd.CombinedOutput()

	// Should exit with error
	if err == nil {
		t.Fatal("Expected invalid cron to exit with error")
	}

	// Output should mention invalid cron
	outputStr := string(output)
	if !strings.Contains(outputStr, "invalid") && !strings.Contains(outputStr, "cron") {
		t.Errorf("Expected error message about invalid cron, got: %s", outputStr)
	}
}

// TestRunOnceFlag verifies -run-once flag is parsed correctly
func TestRunOnceFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Note: This test will actually try to run the updater, so we can't verify full execution
	// But we can verify the flag is accepted without immediate error
	cmd := exec.Command("go", "run", "./cmd/main.go", "-run-once", "-config", "nonexistent.yaml")

	// We expect it to fail due to missing config or uv check, but not flag parsing
	output, _ := cmd.CombinedOutput()
	outputStr := string(output)

	// Should not complain about unknown flag
	if strings.Contains(outputStr, "unknown flag") || strings.Contains(outputStr, "unknown shorthand") {
		t.Errorf("Flag parsing failed: %s", outputStr)
	}
}

func init() {
	// Ensure tests run on Windows only (scheduler has build constraint)
	if runtime.GOOS != "windows" {
		panic("Tests only valid on Windows")
	}
}

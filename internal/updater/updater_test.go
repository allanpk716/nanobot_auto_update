//go:build windows

package updater

import (
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestNewUpdater(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	u := NewUpdater(logger)

	if u == nil {
		t.Fatal("NewUpdater returned nil")
	}

	if u.logger != logger {
		t.Error("Logger not set correctly")
	}

	expectedGithubURL := "git+https://github.com/nanobot-ai/nanobot@main"
	if u.githubURL != expectedGithubURL {
		t.Errorf("Expected githubURL %q, got %q", expectedGithubURL, u.githubURL)
	}

	expectedPypiPackage := "nanobot-ai"
	if u.pypiPackage != expectedPypiPackage {
		t.Errorf("Expected pypiPackage %q, got %q", expectedPypiPackage, u.pypiPackage)
	}

	if u.updateTimeout <= 0 {
		t.Errorf("Expected positive updateTimeout, got %v", u.updateTimeout)
	}
}

func TestTruncateOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "short string unchanged",
			input:    "short output",
			expected: "short output",
		},
		{
			name:     "exactly 500 chars unchanged",
			input:    strings.Repeat("a", 500),
			expected: strings.Repeat("a", 500),
		},
		{
			name:     "501 chars truncated",
			input:    strings.Repeat("a", 501),
			expected: strings.Repeat("a", 500) + "... (truncated)",
		},
		{
			name:     "very long string truncated",
			input:    strings.Repeat("b", 1000),
			expected: strings.Repeat("b", 500) + "... (truncated)",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateOutput(tt.input)
			if result != tt.expected {
				t.Errorf("truncateOutput(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestUpdateResultConstants(t *testing.T) {
	tests := []struct {
		name     string
		result   UpdateResult
		expected string
	}{
		{
			name:     "success constant",
			result:   ResultSuccess,
			expected: "success",
		},
		{
			name:     "fallback constant",
			result:   ResultFallback,
			expected: "fallback",
		},
		{
			name:     "failed constant",
			result:   ResultFailed,
			expected: "failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.result) != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, tt.result)
			}
		})
	}
}

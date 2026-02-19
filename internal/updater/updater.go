//go:build windows

package updater

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"golang.org/x/sys/windows"
)

// UpdateResult represents the outcome of an update attempt
type UpdateResult string

const (
	// ResultSuccess indicates GitHub update succeeded
	ResultSuccess UpdateResult = "success"
	// ResultFallback indicates PyPI fallback succeeded
	ResultFallback UpdateResult = "fallback"
	// ResultFailed indicates both GitHub and PyPI failed
	ResultFailed UpdateResult = "failed"
)

// Updater manages nanobot updates with GitHub primary and PyPI fallback
type Updater struct {
	logger        *slog.Logger
	githubURL     string
	pypiPackage   string
	updateTimeout time.Duration
}

// NewUpdater creates a new Updater with default settings
func NewUpdater(logger *slog.Logger) *Updater {
	return &Updater{
		logger:        logger,
		githubURL:     "git+https://github.com/nanobot-ai/nanobot@main",
		pypiPackage:   "nanobot-ai",
		updateTimeout: 5 * time.Minute,
	}
}

// runCommand executes a command with hidden window and returns combined output
func (u *Updater) runCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()
	return output.String(), err
}

// truncateOutput limits output to 500 characters for logging
func truncateOutput(s string) string {
	const maxLength = 500
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength] + "... (truncated)"
}

// Update attempts to update nanobot from GitHub main branch first,
// falling back to PyPI stable version if GitHub fails
func (u *Updater) Update(ctx context.Context) (UpdateResult, error) {
	ctx, cancel := context.WithTimeout(ctx, u.updateTimeout)
	defer cancel()

	// Primary: Try GitHub main branch
	u.logger.Info("Starting update from GitHub main branch")
	output, err := u.runCommand(ctx, "uv", "tool", "install", u.githubURL)
	if err == nil {
		u.logger.Info("Update successful from GitHub",
			"source", "github",
			"output", truncateOutput(output))
		return ResultSuccess, nil
	}

	u.logger.Warn("GitHub update failed, attempting PyPI fallback",
		"error", err.Error(),
		"github_output", truncateOutput(output))

	// Fallback: Try PyPI stable version
	output, err = u.runCommand(ctx, "uv", "tool", "install", u.pypiPackage)
	if err == nil {
		u.logger.Info("Update successful from PyPI fallback",
			"source", "pypi",
			"output", truncateOutput(output))
		return ResultFallback, nil
	}

	u.logger.Error("Update failed - both GitHub and PyPI attempts failed",
		"pypi_output", truncateOutput(output),
		"error", err.Error())
	return ResultFailed, fmt.Errorf("update failed (GitHub and PyPI): %w", err)
}

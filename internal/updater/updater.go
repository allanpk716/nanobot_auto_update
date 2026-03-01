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
		githubURL:     "git+https://github.com/HKUDS/nanobot.git",
		pypiPackage:   "nanobot-ai",
		updateTimeout: 5 * time.Minute,
	}
}

// GetUvVersion returns the installed uv version for diagnostic purposes
func GetUvVersion() string {
	cmd := exec.Command("uv", "--version")
	cmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
	output, err := cmd.Output()
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return string(bytes.TrimSpace(output))
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
// falling back to PyPI stable version if GitHub fails.
// Uses --force flag to ensure updates work even when already installed.
func (u *Updater) Update(ctx context.Context) (UpdateResult, error) {
	ctx, cancel := context.WithTimeout(ctx, u.updateTimeout)
	defer cancel()

	// Start heartbeat logging goroutine to track update progress
	heartbeatCtx, heartbeatCancel := context.WithCancel(context.Background())
	defer heartbeatCancel()
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		startTime := time.Now()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				elapsed := time.Since(startTime).Round(time.Second)
				u.logger.Info("Update in progress - heartbeat",
					"elapsed", elapsed.String(),
					"timeout", u.updateTimeout.String())
			}
		}
	}()

	// Primary: Try GitHub main branch
	u.logger.Info("Starting forced update from GitHub main branch",
		"command", "uv tool install --force "+u.githubURL,
		"timeout", u.updateTimeout.String())

	output, err := u.runCommand(ctx, "uv", "tool", "install", "--force", u.githubURL)

	// Always log command completion for debugging
	u.logger.Info("GitHub update command completed",
		"success", err == nil,
		"error", err,
		"output_length", len(output),
		"output", truncateOutput(output))

	if err == nil {
		u.logger.Info("Update successful from GitHub",
			"source", "github",
			"output", truncateOutput(output))
		return ResultSuccess, nil
	}

	u.logger.Warn("GitHub forced update failed, attempting PyPI fallback",
		"error", err.Error(),
		"github_output", truncateOutput(output))

	// Fallback: Try PyPI stable version
	u.logger.Info("Attempting PyPI fallback",
		"command", "uv tool install --force "+u.pypiPackage)

	output, err = u.runCommand(ctx, "uv", "tool", "install", "--force", u.pypiPackage)

	// Always log command completion for debugging
	u.logger.Info("PyPI fallback command completed",
		"success", err == nil,
		"error", err,
		"output_length", len(output),
		"output", truncateOutput(output))

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

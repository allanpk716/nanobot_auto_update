//go:build windows

package updater

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	repoPath      string
}

// NewUpdater creates a new Updater with default settings
func NewUpdater(logger *slog.Logger) *Updater {
	return &Updater{
		logger:        logger,
		githubURL:     "git+https://github.com/HKUDS/nanobot.git",
		pypiPackage:   "nanobot-ai",
		updateTimeout: 5 * time.Minute,
		repoPath:      "",
	}
}

// SetRepoPath sets the local git repo path for syncing after update.
// Note: This method is not thread-safe and should only be called during initialization
// before any concurrent access to the Updater instance.
func (u *Updater) SetRepoPath(path string) {
	u.repoPath = path
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

// SyncRepo pulls the latest changes from the local git repository.
// This keeps the local source code in sync with the installed version.
// If the repo doesn't exist, it will be cloned automatically.
func (u *Updater) SyncRepo(ctx context.Context) error {
	if u.repoPath == "" {
		u.logger.Info("No repo_path configured, skipping repo sync")
		return nil
	}

	// Check if repo path exists
	info, err := os.Stat(u.repoPath)
	if os.IsNotExist(err) {
		// Directory doesn't exist, clone the repo
		u.logger.Info("Repo path does not exist, cloning...", "path", u.repoPath)
		return u.cloneRepo(ctx)
	}

	if err != nil {
		u.logger.Error("Failed to check repo path", "path", u.repoPath, "error", err.Error())
		return fmt.Errorf("failed to check repo path: %w", err)
	}

	if !info.IsDir() {
		u.logger.Error("Repo path is not a directory", "path", u.repoPath)
		return fmt.Errorf("repo path is not a directory: %s", u.repoPath)
	}

	// Check if it's a git repo (has .git directory)
	gitDir := filepath.Join(u.repoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		u.logger.Warn("Directory exists but is not a git repo, attempting to clone...", "path", u.repoPath)
		return u.cloneRepo(ctx)
	}

	u.logger.Info("Syncing local git repo", "path", u.repoPath)

	// Run git pull
	cmd := exec.CommandContext(ctx, "git", "pull")
	cmd.Dir = u.repoPath
	cmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	err = cmd.Run()
	outputStr := output.String()

	if err != nil {
		u.logger.Error("Failed to sync repo",
			"path", u.repoPath,
			"error", err.Error(),
			"output", truncateOutput(outputStr))
		return fmt.Errorf("git pull failed: %w", err)
	}

	u.logger.Info("Repo sync completed",
		"path", u.repoPath,
		"output", truncateOutput(outputStr))
	return nil
}

// cloneRepo clones the nanobot repository to the configured path
func (u *Updater) cloneRepo(ctx context.Context) error {
	u.logger.Info("Cloning nanobot repository", "path", u.repoPath)

	// Validate repo path
	if u.repoPath == "" {
		return fmt.Errorf("repo path is empty")
	}

	// Clean the path and remove trailing separators
	cleanPath := filepath.Clean(u.repoPath)

	// Extract parent directory using standard library
	parentDir := filepath.Dir(cleanPath)

	// Handle edge case: if parent dir equals the path itself, we're at root
	if parentDir == cleanPath {
		return fmt.Errorf("invalid repo path: cannot clone to root directory")
	}

	// Check if target directory already exists and is not empty
	if info, err := os.Stat(u.repoPath); err == nil && info.IsDir() {
		// Directory exists, check if it's empty
		entries, err := os.ReadDir(u.repoPath)
		if err != nil {
			u.logger.Error("Failed to read target directory", "path", u.repoPath, "error", err.Error())
			return fmt.Errorf("failed to read target directory: %w", err)
		}
		if len(entries) > 0 {
			u.logger.Error("Target directory is not empty", "path", u.repoPath, "entries", len(entries))
			return fmt.Errorf("target directory is not empty: %s (contains %d items)", u.repoPath, len(entries))
		}
	}

	// Create parent directory if needed
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		u.logger.Error("Failed to create parent directory", "path", parentDir, "error", err.Error())
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Extract clean repository URL (remove git+ prefix if present)
	repoURL := u.githubURL
	if strings.HasPrefix(repoURL, "git+") {
		repoURL = strings.TrimPrefix(repoURL, "git+")
	}

	// Run git clone
	cmd := exec.CommandContext(ctx, "git", "clone", repoURL, u.repoPath)
	cmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()
	outputStr := output.String()

	if err != nil {
		u.logger.Error("Failed to clone repo",
			"url", repoURL,
			"path", u.repoPath,
			"error", err.Error(),
			"output", truncateOutput(outputStr))
		return fmt.Errorf("git clone failed: %w", err)
	}

	u.logger.Info("Repo cloned successfully",
		"url", repoURL,
		"path", u.repoPath,
		"output", truncateOutput(outputStr))
	return nil
}

//go:build windows

package lifecycle

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"time"

	"golang.org/x/sys/windows"
)

// checkUpdateStateInternal returns the action and performs cleanup if needed.
// Returns: "cleanup" (already cleaned), "recover" (.old needs restore), "normal" (nothing to do)
func checkUpdateStateInternal(exePath string, logger *slog.Logger) string {
	oldPath := exePath + ".old"
	successPath := exePath + ".update-success"

	if data, err := os.ReadFile(successPath); err == nil {
		var marker map[string]string
		if err := json.Unmarshal(data, &marker); err == nil {
			logger.Info("previous update successful, cleaning up .old backup",
				"new_version", marker["new_version"])
			os.Remove(oldPath)
			os.Remove(successPath)
			return "cleanup"
		}
	}

	if info, err := os.Stat(oldPath); err == nil && info.Size() > 0 {
		return "recover"
	}

	return "normal"
}

// CheckUpdateStateForPath checks for leftover update state at the given exe path.
// If a crash during update is detected (.old exists without .update-success),
// it restores the old version and restarts.
func CheckUpdateStateForPath(exePath string, logger *slog.Logger) {
	action := checkUpdateStateInternal(exePath, logger)
	if action == "recover" {
		oldPath := exePath + ".old"
		logger.Warn("crash detected during update, restoring from .old backup")
		if err := os.Rename(oldPath, exePath); err != nil {
			logger.Error("failed to restore from .old backup", "error", err)
			return
		}
		logger.Info("restored from .old backup, restarting")
		cmd := exec.Command(exePath, os.Args[1:]...)
		cmd.SysProcAttr = &windows.SysProcAttr{
			HideWindow:    true,
			CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
		}
		cmd.Start()
		os.Exit(0)
	}
}

// CheckUpdateState checks for leftover update state (.old cleanup/recovery).
// Must run before config loading and server startup (D-04).
func CheckUpdateState(logger *slog.Logger) {
	exePath, err := os.Executable()
	if err != nil {
		logger.Error("failed to get exe path", "error", err)
		return
	}
	CheckUpdateStateForPath(exePath, logger)
}

// ListenWithRetry attempts to bind a TCP port with retries.
// Per D-05: 500ms interval, max 5 retries (2.5s total).
// Used after self-update restart when old process may still hold the port.
func ListenWithRetry(addr string, logger *slog.Logger) (net.Listener, error) {
	var lastErr error
	for i := 0; i < 5; i++ {
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			if i > 0 {
				logger.Info("port bind succeeded after retry",
					"addr", addr,
					"attempts", i+1)
			}
			return listener, nil
		}
		lastErr = err
		logger.Warn("port bind failed, retrying",
			"addr", addr,
			"attempt", i+1,
			"error", err)
		time.Sleep(500 * time.Millisecond)
	}
	return nil, fmt.Errorf("failed to bind %s after 5 retries: %w", addr, lastErr)
}

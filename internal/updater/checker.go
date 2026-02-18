//go:build windows

package updater

import (
	"errors"
	"fmt"
	"os/exec"
)

// CheckUvInstalled verifies uv is available in PATH.
// Returns a clear error if uv is not installed.
func CheckUvInstalled() error {
	_, err := exec.LookPath("uv")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("uv is not installed or not in PATH - please install uv from https://docs.astral.sh/uv/")
		}
		return fmt.Errorf("failed to check for uv installation: %w", err)
	}
	return nil
}

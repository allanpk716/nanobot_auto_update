//go:build windows

package lifecycle

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"time"

	"golang.org/x/sys/windows"
)

// StartNanobot starts nanobot gateway in the background with hidden window.
// Returns error if startup fails or port is not listening within timeout.
func StartNanobot(ctx context.Context, port uint32, startupTimeout time.Duration) error {
	// Start nanobot gateway as background process
	cmd := exec.Command("nanobot", "gateway")
	cmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP,
	}

	// Detach from parent - don't wait for completion
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start nanobot: %w", err)
	}

	// Release the process so it continues independently
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("failed to detach nanobot process: %w", err)
	}

	// Verify startup by checking port becomes available
	if err := waitForPortListening(ctx, port, startupTimeout); err != nil {
		return fmt.Errorf("nanobot startup verification failed: %w", err)
	}

	return nil
}

// waitForPortListening polls until the port is listening or timeout
func waitForPortListening(ctx context.Context, port uint32, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	address := fmt.Sprintf("127.0.0.1:%d", port)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Try to connect to verify port is listening
			conn, err := net.DialTimeout("tcp", address, 1*time.Second)
			if err == nil {
				conn.Close()
				return nil // Port is listening
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

	return fmt.Errorf("port %d not listening after %v", port, timeout)
}

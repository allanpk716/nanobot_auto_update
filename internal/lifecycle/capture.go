//go:build windows

package lifecycle

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/logbuffer"
)

// captureLogs reads logs from reader line by line and writes to LogBuffer
// CAPT-01, CAPT-02: Captures stdout/stderr output stream
// CAPT-03: Runs in separate goroutine for concurrent pipe reading
// CAPT-05: Stops when context cancelled (process exit)
func captureLogs(
	ctx context.Context,
	reader io.Reader,
	source string, // "stdout" or "stderr"
	logBuffer *logbuffer.LogBuffer,
	logger *slog.Logger,
) {
	scanner := bufio.NewScanner(reader)
	for {
		select {
		case <-ctx.Done():
			// Context cancelled, stop reading (CAPT-05)
			logger.Debug("Log capture stopped", "source", source)
			return
		default:
			// Non-blocking scan
			if !scanner.Scan() {
				// EOF or error
				if err := scanner.Err(); err != nil {
					// ERR-01: Log error but continue running
					logger.Error("Log capture scanner error",
						"source", source, "error", err)
				}
				return
			}

			// Read line and write to LogBuffer
			line := scanner.Text()
			entry := logbuffer.LogEntry{
				Timestamp: time.Now(),
				Source:    source,
				Content:   line,
			}
			if err := logBuffer.Write(entry); err != nil {
				// ERR-03: Log error and drop the line (don't block)
				logger.Error("Failed to write log to buffer",
					"source", source, "line", line, "error", err)
			}
		}
	}
}

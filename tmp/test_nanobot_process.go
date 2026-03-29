//go:build ignore

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"time"

	"golang.org/x/sys/windows"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// 测试启动 nanobot 进程并捕获输出
	testCases := []struct {
		name    string
		command string
		port    uint32
	}{
		{
			name:    "nanobot-me",
			command: "nanobot gateway --port 18790",
			port:    18790,
		},
	}

	for _, tc := range testCases {
		fmt.Printf("=== 测试启动实例: %s ===\n", tc.name)
		fmt.Printf("命令: %s\n", tc.command)
		fmt.Printf("端口: %d\n\n", tc.port)

		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		defer cancel()

		// 启动进程并捕获输出
		err := startNanobotWithLogs(ctx, tc.command, tc.port, logger)
		if err != nil {
			fmt.Printf("❌ 启动失败: %v\n\n", err)
		} else {
			fmt.Printf("✓ 启动成功\n\n")
		}
	}
}

func startNanobotWithLogs(ctx context.Context, command string, port uint32, logger *slog.Logger) error {
	// 创建命令
	cmd := exec.CommandContext(ctx, "cmd", "/c", command)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")
	cmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow:    false, // 显示窗口以便看到输出
		CreationFlags: windows.CREATE_NEW_CONSOLE,
	}

	// 捕获 stdout 和 stderr
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// 启动进程
	logger.Info("启动进程", "command", command)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start nanobot: %w", err)
	}

	logger.Info("进程已启动", "pid", cmd.Process.Pid)

	// 读取输出（在后台 goroutine 中）
	outputDone := make(chan struct{})
	go func() {
		defer close(outputDone)

		// 读取 stdout
		stdoutBuf := make([]byte, 1024)
		for {
			n, err := stdoutPipe.Read(stdoutBuf)
			if n > 0 {
				fmt.Printf("[STDOUT] %s", string(stdoutBuf[:n]))
			}
			if err != nil {
				if err != io.EOF {
					logger.Error("读取 stdout 失败", "error", err)
				}
				break
			}
		}

		// 读取 stderr
		stderrBuf := make([]byte, 1024)
		for {
			n, err := stderrPipe.Read(stderrBuf)
			if n > 0 {
				fmt.Printf("[STDERR] %s", string(stderrBuf[:n]))
			}
			if err != nil {
				if err != io.EOF {
					logger.Error("读取 stderr 失败", "error", err)
				}
				break
			}
		}
	}()

	// 等待端口监听
	address := fmt.Sprintf("127.0.0.1:%d", port)
	logger.Info("等待端口监听", "address", address)

	startTime := time.Now()
	timeout := 30 * time.Second
	checkInterval := 500 * time.Millisecond

	for time.Since(startTime) < timeout {
		select {
		case <-ctx.Done():
			logger.Warn("上下文已取消")
			return ctx.Err()
		default:
			conn, err := net.DialTimeout("tcp", address, 1*time.Second)
			if err == nil {
				conn.Close()
				logger.Info("端口已监听", "port", port, "duration", time.Since(startTime))
				return nil
			}
			time.Sleep(checkInterval)
		}
	}

	// 超时，检查进程状态
	logger.Error("端口监听超时", "port", port, "timeout", timeout)

	// 等待进程退出
	waitErr := cmd.Wait()
	if waitErr != nil {
		logger.Error("进程已退出", "error", waitErr)
		return fmt.Errorf("port %d not listening after %v, process exited: %w", port, timeout, waitErr)
	}

	return fmt.Errorf("port %d not listening after %v", port, timeout)
}

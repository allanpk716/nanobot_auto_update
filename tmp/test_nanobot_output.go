// Test program to capture nanobot process output synchronously
// This helps debug why nanobot exits immediately when started by nanobot-auto-updater
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

func main() {
	fmt.Println("=== Test 1: Running 'nanobot gateway --port 18790' ===")

	// Create command
	cmd := exec.Command("nanobot", "gateway", "--port", "18790")
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")

	// Create pipes
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("Failed to create stdout pipe: %v\n", err)
		return
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		fmt.Printf("Failed to create stderr pipe: %v\n", err)
		return
	}

	// Start process
	fmt.Printf("Starting process...\n")
	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		fmt.Printf("Failed to start: %v\n", err)
		return
	}

	fmt.Printf("Process started with PID: %d\n", cmd.Process.Pid)

	// Wait 2 seconds like starter.go does
	fmt.Printf("Waiting 2 seconds for stabilization...\n")
	time.Sleep(2 * time.Second)

	// Check if process is still running
	fmt.Printf("Checking process status...\n")
	// Note: On Windows, we can't easily check if process is still running without using external packages

	// Now read all output synchronously (before Wait)
	fmt.Printf("\n=== Reading stdout ===\n")
	stdoutScanner := bufio.NewScanner(stdoutPipe)
	go func() {
		for stdoutScanner.Scan() {
			fmt.Printf("[STDOUT] %s\n", stdoutScanner.Text())
		}
		if err := stdoutScanner.Err(); err != nil && err != io.EOF {
			fmt.Printf("[STDOUT ERROR] %v\n", err)
		}
	}()

	fmt.Printf("\n=== Reading stderr ===\n")
	stderrScanner := bufio.NewScanner(stderrPipe)
	go func() {
		for stderrScanner.Scan() {
			fmt.Printf("[STDERR] %s\n", stderrScanner.Text())
		}
		if err := stderrScanner.Err(); err != nil && err != io.EOF {
			fmt.Printf("[STDERR ERROR] %v\n", err)
		}
	}()

	// Wait for process to complete
	fmt.Printf("\n=== Waiting for process to exit ===\n")
	err = cmd.Wait()
	elapsed := time.Since(startTime)

	if err != nil {
		fmt.Printf("\nProcess exited with error after %v: %v\n", elapsed, err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Printf("Exit code: %d\n", exitErr.ExitCode())
		}
	} else {
		fmt.Printf("\nProcess exited successfully after %v\n", elapsed)
	}

	// Give goroutines time to finish reading
	time.Sleep(100 * time.Millisecond)
}

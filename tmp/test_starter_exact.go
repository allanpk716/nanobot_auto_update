//go:build ignore

// Test program to reproduce the exact startup conditions as starter.go
// This will help identify if CREATE_NO_WINDOW or pipe handling causes the issue
package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"golang.org/x/sys/windows"
)

func main() {
	fmt.Println("=== Test: Running with starter.go exact conditions ===")
	fmt.Println("Using: CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP, HideWindow=true")

	ctx := context.Background()

	// Create command exactly like starter.go
	cmd := exec.CommandContext(ctx, "nanobot", "gateway", "--port", "18790")
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")

	// Use the EXACT same SysProcAttr as starter.go
	cmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP,
	}

	// Create pipes EXACTLY like starter.go
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		fmt.Printf("Failed to create stdout pipe: %v\n", err)
		return
	}

	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		stdoutReader.Close()
		stdoutWriter.Close()
		fmt.Printf("Failed to create stderr pipe: %v\n", err)
		return
	}

	// Set stdout and stderr
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	// Start process
	fmt.Printf("\nStarting process...\n")
	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		stdoutReader.Close()
		stdoutWriter.Close()
		stderrReader.Close()
		stderrWriter.Close()
		fmt.Printf("Failed to start: %v\n", err)
		return
	}

	pid := cmd.Process.Pid
	fmt.Printf("Process started with PID: %d\n", pid)

	// Close writer ends IMMEDIATELY like starter.go (line 90-91)
	fmt.Printf("Closing pipe writer ends (critical for preventing deadlock)...\n")
	stdoutWriter.Close()
	stderrWriter.Close()

	// Wait 2 seconds like starter.go (line 94)
	fmt.Printf("Waiting 2 seconds for stabilization...\n")
	time.Sleep(2 * time.Second)
	fmt.Printf("Wait complete\n")

	// Start log capture goroutines like starter.go (line 118-119)
	fmt.Printf("\nStarting log capture goroutines...\n")

	stdoutDone := make(chan bool)
	stderrDone := make(chan bool)

	go func() {
		defer close(stdoutDone)
		scanner := bufio.NewScanner(stdoutReader)
		for scanner.Scan() {
			fmt.Printf("[STDOUT] %s\n", scanner.Text())
		}
		if err := scanner.Err(); err != nil && err != io.EOF {
			fmt.Printf("[STDOUT ERROR] %v\n", err)
		}
		fmt.Printf("[STDOUT] Capture finished\n")
	}()

	go func() {
		defer close(stderrDone)
		scanner := bufio.NewScanner(stderrReader)
		for scanner.Scan() {
			fmt.Printf("[STDERR] %s\n", scanner.Text())
		}
		if err := scanner.Err(); err != nil && err != io.EOF {
			fmt.Printf("[STDERR ERROR] %v\n", err)
		}
		fmt.Printf("[STDERR] Capture finished\n")
	}()

	// Start monitor goroutine like starter.go (line 122-129)
	fmt.Printf("\nStarting process monitor goroutine...\n")
	waitDone := make(chan error)
	go func() {
		defer close(waitDone)
		err := cmd.Wait()
		elapsed := time.Since(startTime)
		waitDone <- err
		if err != nil {
			fmt.Printf("\n[MONITOR] Process exited with error after %v: %v\n", elapsed, err)
		} else {
			fmt.Printf("\n[MONITOR] Process exited normally after %v\n", elapsed)
		}
	}()

	// Wait for process to exit or timeout
	fmt.Printf("\nWaiting for process to exit (or 10 second timeout)...\n")

	select {
	case err := <-waitDone:
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				fmt.Printf("\n!!! Process exited with exit code: %d\n", exitErr.ExitCode())
			}
		}
	case <-time.After(10 * time.Second):
		fmt.Printf("\nProcess is still running after 10 seconds (this is GOOD!)\n")
		fmt.Printf("Killing process for cleanup...\n")
		cmd.Process.Kill()
		<-waitDone
	}

	// Close readers
	stdoutReader.Close()
	stderrReader.Close()

	// Wait for goroutines to finish
	<-stdoutDone
	<-stderrDone

	fmt.Printf("\n=== Test complete ===\n")
}

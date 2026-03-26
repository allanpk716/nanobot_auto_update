package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"golang.org/x/sys/windows"
)

func main() {
	fmt.Println("Testing nanobot startup with CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP")

	// Create pipes
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		fmt.Printf("Failed to create stdout pipe: %v\n", err)
		return
	}

	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		fmt.Printf("Failed to create stderr pipe: %v\n", err)
		stdoutReader.Close()
		stdoutWriter.Close()
		return
	}

	// Create detached context
	detachedCtx := context.Background()

	// Prepare command
	cmd := exec.CommandContext(detachedCtx, "nanobot", "gateway", "--config", "C:/Users/allan716/.nanobot-work-helper/config.json", "--port", "18792")
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")
	cmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP,
	}
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter
	cmd.Dir = "C:/Users/allan716/.nanobot-work-helper" // Set working directory

	// Start process
	start := time.Now()
	if err := cmd.Start(); err != nil {
		fmt.Printf("Failed to start nanobot: %v\n", err)
		stdoutReader.Close()
		stdoutWriter.Close()
		stderrReader.Close()
		stderrWriter.Close()
		return
	}

	pid := cmd.Process.Pid
	fmt.Printf("Nanobot process started with PID %d\n", pid)

	// Close writer ends immediately
	stdoutWriter.Close()
	stderrWriter.Close()

	// Wait 2 seconds
	time.Sleep(2 * time.Second)

	// Verify process is still running
	fmt.Printf("Checking if process is still running...\n")
	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Printf("Failed to find process: %v\n", err)
	} else {
		// Try to send signal 0 to check if process exists
		err = proc.Signal(os.Signal(nil))
		if err != nil {
			fmt.Printf("Process already exited: %v\n", err)
		} else {
			fmt.Printf("Process is still running\n")
		}
	}

	// Start log capture
	outputChan := make(chan string, 100)
	go func() {
		defer stdoutReader.Close()
		buf := make([]byte, 4096)
		for {
			n, err := stdoutReader.Read(buf)
			if err != nil {
				if err != io.EOF {
					fmt.Printf("Read error: %v\n", err)
				}
				return
			}
			if n > 0 {
				outputChan <- string(buf[:n])
			}
		}
	}()

	// Monitor process exit
	exitChan := make(chan error, 1)
	go func() {
		exitChan <- cmd.Wait()
	}()

	// Wait for exit or output
	for {
		select {
		case err := <-exitChan:
			elapsed := time.Since(start)
			if err != nil {
				fmt.Printf("\nProcess exited with error after %v: %v\n", elapsed, err)
				if exitErr, ok := err.(*exec.ExitError); ok {
					fmt.Printf("Exit code: %d\n", exitErr.ExitCode())
				}
			} else {
				fmt.Printf("\nProcess exited normally after %v\n", elapsed)
			}
			return
		case output := <-outputChan:
			fmt.Printf("\n[OUTPUT] %s\n", output)
		case <-time.After(10 * time.Second):
			fmt.Printf("\nProcess still running after 10 seconds, killing it...\n")
			cmd.Process.Kill()
			cmd.Wait()
			return
		}
	}
}

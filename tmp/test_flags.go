package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

func main() {
	// Test 1: CREATE_NO_WINDOW only
	fmt.Println("Test 1: CREATE_NO_WINDOW only")
	testFlags(windows.CREATE_NO_WINDOW, "test1")

	time.Sleep(2 * time.Second)

	// Test 2: CREATE_NEW_PROCESS_GROUP only
	fmt.Println("\nTest 2: CREATE_NEW_PROCESS_GROUP only")
	testFlags(windows.CREATE_NEW_PROCESS_GROUP, "test2")

	time.Sleep(2 * time.Second)

	// Test 3: Both flags combined
	fmt.Println("\nTest 3: CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP")
	testFlags(windows.CREATE_NO_WINDOW|windows.CREATE_NEW_PROCESS_GROUP, "test3")

	time.Sleep(5 * time.Second)
}

func testFlags(flags uint32, testName string) {
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		fmt.Printf("[%s] Failed to create stdout pipe: %v\n", testName, err)
		return
	}

	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		fmt.Printf("[%s] Failed to create stderr pipe: %v\n", testName, err)
		stdoutReader.Close()
		stdoutWriter.Close()
		return
	}

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "nanobot", "gateway", "--port", "18790")
	cmd.SysProcAttr = & windows.SysProcAttr{
		HideWindow:    true,
		CreationFlags: flags,
	}
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	start := time.Now()
	if err := cmd.Start(); err != nil {
		fmt.Printf("[%s] Failed to start: %v\n", testName, err)
		stdoutReader.Close()
		stdoutWriter.Close()
		stderrReader.Close()
		stderrWriter.Close()
		return
	}

	pid := cmd.Process.Pid
	fmt.Printf("[%s] Process started with PID %d, flags=0x%08X\n", testName, pid, flags)

	// Close writer ends
	stdoutWriter.Close()
	stderrWriter.Close()

	// Monitor process exit
	exitChan := make(chan error, 1)
	go func() {
		exitChan <- cmd.Wait()
	}()

	// Capture output
	outputChan := make(chan string, 100)
	go func() {
		defer stdoutReader.Close()
		buf := make([]byte, 4096)
		for {
			n, err := stdoutReader.Read(buf)
			if err != nil {
				if err != io.EOF {
					fmt.Printf("[%s] Read error: %v\n", testName, err)
				}
				return
			}
			if n > 0 {
				outputChan <- string(buf[:n])
			}
		}
	}()

	// Wait for exit or timeout
	select {
	case err := <-exitChan:
		elapsed := time.Since(start)
		if err != nil {
			fmt.Printf("[%s] Process exited with error after %v: %v\n", testName, elapsed, err)
			if exitErr, ok := err.(*exec.ExitError); ok {
				fmt.Printf("[%s] Exit code: %d\n", testName, exitErr.ExitCode())
			}
		} else {
			fmt.Printf("[%s] Process exited normally after %v\n", testName, elapsed)
		}
	case output := <-outputChan:
		fmt.Printf("[%s] Received output: %q\n", testName, output)
	case <-time.After(10 * time.Second):
		fmt.Printf("[%s] Process still running after 10 seconds\n", testName)
		cmd.Process.Kill()
		cmd.Wait()
	}
}

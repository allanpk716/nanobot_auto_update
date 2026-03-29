//go:build manual

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/minio/selfupdate"
	"golang.org/x/sys/windows"
)

var Version = "dev"

func main() {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] os.Executable: %v\n", err)
		os.Exit(1)
	}
	versionFile := exePath + ".version"

	// Step 1: Write current version to file (per D-04)
	if err := os.WriteFile(versionFile, []byte(Version), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "[%s] ERROR write version file: %v\n", Version, err)
		os.Exit(1)
	}
	fmt.Printf("[%s] Started, exe=%s\n", Version, exePath)

	// v1 behavior: read v2 binary -> Apply -> self-spawn -> exit
	if Version == "1.0.0" {
		// Determine v2 binary path: same directory, named poc_v2.exe
		dir := exePath[:strings.LastIndex(exePath, string(os.PathSeparator))+1]
		newBinPath := dir + "poc_v2.exe"

		newBin, err := os.Open(newBinPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[v1] ERROR open v2 binary: %v\n", err)
			os.Exit(1)
		}
		defer newBin.Close()

		// Set OldSavePath to explicitly save backup (per RESEARCH Pitfall 1 -- must be non-empty)
		oldPath := exePath + ".old"
		opts := selfupdate.Options{
			OldSavePath: oldPath,
		}

		err = selfupdate.Apply(newBin, opts)
		if err != nil {
			if rerr := selfupdate.RollbackError(err); rerr != nil {
				fmt.Fprintf(os.Stderr, "[v1] FATAL update+rollback failed: %v (rollback: %v)\n", err, rerr)
			} else {
				fmt.Fprintf(os.Stderr, "[v1] Update failed (rolled back): %v\n", err)
			}
			os.Exit(1)
		}

		fmt.Println("[v1] Update applied, spawning new version...")

		// Self-spawn restart (per RESEARCH Pattern 3, using cmd.Start NOT cmd.Run)
		cmd := exec.Command(exePath, os.Args[1:]...)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: windows.CREATE_NO_WINDOW,
		}
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "[v1] ERROR spawn new process: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// v2 (or any non-v1) behavior: update complete, report status
	fmt.Printf("[%s] Self-update complete!\n", Version)
	oldPath := exePath + ".old"
	if info, err := os.Stat(oldPath); err == nil {
		fmt.Printf("[%s] Backup file exists: %s (size: %d bytes)\n", Version, oldPath, info.Size())
	} else {
		fmt.Printf("[%s] WARNING: Backup file not found at %s: %v\n", Version, oldPath, err)
	}
}

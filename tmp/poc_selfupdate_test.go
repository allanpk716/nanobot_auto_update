//go:build manual

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestSelfUpdate(t *testing.T) {
	// Work from tmp/ directory so all artifacts are created there
	workDir := filepath.Join("..", "tmp")

	// Cleanup function for all generated artifacts
	cleanup := func() {
		// Brief pause to let v2 process release file locks before cleanup
		time.Sleep(1 * time.Second)
		artifacts := []string{
			filepath.Join(workDir, "poc_v1.exe.version"),
			filepath.Join(workDir, "poc_v1.exe.old"),
			filepath.Join(workDir, "poc_v2.exe.version"),
			filepath.Join(workDir, "poc_v2.exe"),
			filepath.Join(workDir, "poc_v1.exe"),
		}
		for _, f := range artifacts {
			os.Remove(f)
		}
	}
	defer cleanup()

	// Step 1: Build v1 and v2 with ldflags version injection
	build := func(version, output string) error {
		cmd := exec.Command("go", "build",
			"-ldflags", "-X main.Version="+version,
			"-o", filepath.Join(workDir, output),
			"poc_selfupdate.go",
		)
		cmd.Dir = workDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("Build %s output: %s", version, string(out))
		}
		return err
	}

	if err := build("1.0.0", "poc_v1.exe"); err != nil {
		t.Fatalf("Build v1 failed: %v", err)
	}
	t.Log("Built poc_v1.exe (version 1.0.0)")

	if err := build("2.0.0", "poc_v2.exe"); err != nil {
		t.Fatalf("Build v2 failed: %v", err)
	}
	t.Log("Built poc_v2.exe (version 2.0.0)")

	// Step 2: Run v1 (it will Apply v2 -> self-spawn -> exit)
	v1Path := filepath.Join(workDir, "poc_v1.exe")
	cmd := exec.Command(v1Path)
	cmd.Dir = workDir
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start v1: %v", err)
	}
	t.Logf("Started v1 (PID: %d), waiting for self-update...", cmd.Process.Pid)

	// Step 3: Poll for version file (per RESEARCH Pitfall 4: 500ms interval, 30s max)
	versionFile := filepath.Join(workDir, "poc_v1.exe.version")
	deadline := time.Now().Add(30 * time.Second)
	var versionContent string

	for time.Now().Before(deadline) {
		data, err := os.ReadFile(versionFile)
		if err == nil {
			versionContent = string(data)
			if versionContent == "2.0.0" {
				t.Log("VALID-01 PASSED: v2 started, version file contains '2.0.0'")
				break
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	if versionContent != "2.0.0" {
		// Read whatever we got for diagnostics
		data, _ := os.ReadFile(versionFile)
		t.Fatalf("VALID-01 FAILED: timeout waiting for v2. Version file content: '%s'", string(data))
	}

	// Step 4: Verify .old backup file exists (VALID-02)
	oldFile := filepath.Join(workDir, "poc_v1.exe.old")
	if info, err := os.Stat(oldFile); err != nil {
		t.Errorf("VALID-02 FAILED: .old backup file not found at %s: %v", oldFile, err)
	} else {
		t.Logf("VALID-02 PASSED: .old backup exists (%d bytes)", info.Size())
	}

	// Step 5: VALID-03 is inherently verified -- v2 wrote "2.0.0" to version file,
	// which means the self-spawn succeeded and v2 process started independently.
	t.Log("VALID-03 PASSED: self-spawn restart verified (v2 wrote version file independently)")
}

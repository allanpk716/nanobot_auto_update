//go:build windows

package lifecycle

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/windows/svc"
)

// testServiceConfig creates a minimal valid config for service handler tests.
// Empty instances = no health monitor, Port 0 = no API server.
func testServiceConfig() *config.Config {
	return &config.Config{
		Instances: []config.InstanceConfig{},
		API: config.APIConfig{Port: 0},
		Monitor: config.MonitorConfig{
			Interval: 30 * time.Second,
			Timeout:  10 * time.Second,
		},
		SelfUpdate: config.SelfUpdateConfig{
			GithubOwner: "test",
			GithubRepo:  "test",
		},
	}
}

// testServiceLogger creates a discard logger for service handler tests.
func testServiceLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// testServiceHandler creates a ServiceHandler with nil callbacks for testing.
// Nil createComponents and startInstances cause AppStartup to skip those steps,
// allowing the Execute state machine to be tested in isolation.
func testServiceHandler() *ServiceHandler {
	cfg := testServiceConfig()
	logger := testServiceLogger()
	return NewServiceHandler(cfg, logger, "test-version", nil, nil, nil, nil)
}

// readStatus reads a status from the channel with a timeout.
// Fails the test if the status is not received within 10 seconds.
func readStatus(t *testing.T, ch <-chan svc.Status, expected svc.State) {
	t.Helper()
	select {
	case s := <-ch:
		assert.Equal(t, expected, s.State, "expected status %v, got %v", expected, s.State)
	case <-time.After(10 * time.Second):
		t.Fatalf("timeout waiting for status %v", expected)
	}
}

// TestServiceHandler_Stop verifies that sending svc.Stop causes Execute to:
// 1. Report StartPending, then Running
// 2. Receive the Stop command
// 3. Report StopPending, then Stopped
// 4. Return (false, 0) indicating clean exit
func TestServiceHandler_Stop(t *testing.T) {
	handler := testServiceHandler()

	// Channels for communicating with Execute
	reqCh := make(chan svc.ChangeRequest)
	statusCh := make(chan svc.Status)

	// Run Execute in a goroutine
	done := make(chan svcReturn)
	go func() {
		svcSpecific, exitCode := handler.Execute([]string{}, reqCh, statusCh)
		done <- svcReturn{svcSpecific: svcSpecific, exitCode: exitCode}
	}()

	// Verify state transitions: StartPending -> Running
	readStatus(t, statusCh, svc.StartPending)
	readStatus(t, statusCh, svc.Running)

	// Send Stop command
	reqCh <- svc.ChangeRequest{Cmd: svc.Stop}

	// Verify state transitions: StopPending -> Stopped
	readStatus(t, statusCh, svc.StopPending)
	readStatus(t, statusCh, svc.Stopped)

	// Verify return values
	select {
	case ret := <-done:
		assert.False(t, ret.svcSpecific, "svcSpecific should be false on clean Stop")
		assert.Equal(t, uint32(0), ret.exitCode, "exitCode should be 0 on clean Stop")
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for Execute to return")
	}
}

// TestServiceHandler_Shutdown verifies that sending svc.Shutdown causes Execute to:
// Report StopPending, Stopped and return (false, 0) -- same as Stop.
func TestServiceHandler_Shutdown(t *testing.T) {
	handler := testServiceHandler()

	reqCh := make(chan svc.ChangeRequest)
	statusCh := make(chan svc.Status)

	done := make(chan svcReturn)
	go func() {
		svcSpecific, exitCode := handler.Execute([]string{}, reqCh, statusCh)
		done <- svcReturn{svcSpecific: svcSpecific, exitCode: exitCode}
	}()

	// Verify startup
	readStatus(t, statusCh, svc.StartPending)
	readStatus(t, statusCh, svc.Running)

	// Send Shutdown command
	reqCh <- svc.ChangeRequest{Cmd: svc.Shutdown}

	// Verify shutdown transitions
	readStatus(t, statusCh, svc.StopPending)
	readStatus(t, statusCh, svc.Stopped)

	// Verify return values
	select {
	case ret := <-done:
		assert.False(t, ret.svcSpecific, "svcSpecific should be false on clean Shutdown")
		assert.Equal(t, uint32(0), ret.exitCode, "exitCode should be 0 on clean Shutdown")
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for Execute to return")
	}
}

// TestServiceHandler_Interrogate verifies that sending svc.Interrogate after Running
// echoes the current status back.
func TestServiceHandler_Interrogate(t *testing.T) {
	handler := testServiceHandler()

	reqCh := make(chan svc.ChangeRequest)
	statusCh := make(chan svc.Status)

	done := make(chan svcReturn)
	go func() {
		svcSpecific, exitCode := handler.Execute([]string{}, reqCh, statusCh)
		done <- svcReturn{svcSpecific: svcSpecific, exitCode: exitCode}
	}()

	// Wait for Running state
	readStatus(t, statusCh, svc.StartPending)
	readStatus(t, statusCh, svc.Running)

	// Send Interrogate -- should echo current status
	reqCh <- svc.ChangeRequest{
		Cmd:           svc.Interrogate,
		CurrentStatus: svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown},
	}

	// Read the echoed status
	select {
	case s := <-statusCh:
		assert.Equal(t, svc.Running, s.State, "Interrogate should echo Running state")
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for Interrogate response")
	}

	// Now send Stop to clean up
	reqCh <- svc.ChangeRequest{Cmd: svc.Stop}
	readStatus(t, statusCh, svc.StopPending)
	readStatus(t, statusCh, svc.Stopped)

	select {
	case ret := <-done:
		assert.False(t, ret.svcSpecific)
		assert.Equal(t, uint32(0), ret.exitCode)
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for Execute to return")
	}
}

// svcReturn holds the return values from Execute.
type svcReturn struct {
	svcSpecific bool
	exitCode    uint32
}

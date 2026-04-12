package instance

// SetPIDForTest sets the internal PID field for testing purposes.
// This allows tests to simulate a running instance without starting a real process.
// The PID value should be set to a non-zero value to make IsRunning() potentially return true.
// NOTE: IsRunning() also checks if the process exists via gopsutil, so the test must
// either use a PID that corresponds to an existing process (e.g., the test process itself)
// or use a PID that FindProcessByPID will find. The simplest approach: use os.Getpid()
// in the test to get the current test process PID, which always exists.
func (il *InstanceLifecycle) SetPIDForTest(pid int32) {
	il.pid = pid
}

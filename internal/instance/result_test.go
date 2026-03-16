package instance

import (
	"errors"
	"strings"
	"testing"
)

// TestUpdateResultHasErrors tests HasErrors method
func TestUpdateResultHasErrors(t *testing.T) {
	tests := []struct {
		name     string
		result   UpdateResult
		expected bool
	}{
		{
			name:     "No errors",
			result:   UpdateResult{Stopped: []string{"a"}, Started: []string{"a"}},
			expected: false,
		},
		{
			name:     "Stop failed",
			result:   UpdateResult{StopFailed: []*InstanceError{{InstanceName: "a"}}},
			expected: true,
		},
		{
			name:     "Start failed",
			result:   UpdateResult{StartFailed: []*InstanceError{{InstanceName: "a"}}},
			expected: true,
		},
		{
			name:     "Both failed",
			result:   UpdateResult{StopFailed: []*InstanceError{{InstanceName: "a"}}, StartFailed: []*InstanceError{{InstanceName: "b"}}},
			expected: true,
		},
		{
			name:     "Empty result",
			result:   UpdateResult{},
			expected: false,
		},
		{
			name:     "Only stopped success",
			result:   UpdateResult{Stopped: []string{"inst1", "inst2"}},
			expected: false,
		},
		{
			name:     "Only started success",
			result:   UpdateResult{Started: []string{"inst1"}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasErrors(); got != tt.expected {
				t.Errorf("HasErrors() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestUpdateError tests UpdateError error aggregation
func TestUpdateError(t *testing.T) {
	errs := []*InstanceError{
		{InstanceName: "instance1", Operation: "stop", Port: 8080, Err: errors.New("stop failed")},
		{InstanceName: "instance2", Operation: "start", Port: 8081, Err: errors.New("start failed")},
	}

	updateErr := &UpdateError{Errors: errs}

	// Test Error() method
	errMsg := updateErr.Error()
	if errMsg == "" {
		t.Error("Error() returned empty string")
	}

	// Should contain instance names
	if !strings.Contains(errMsg, "instance1") {
		t.Errorf("Error message should contain 'instance1': %s", errMsg)
	}
	if !strings.Contains(errMsg, "instance2") {
		t.Errorf("Error message should contain 'instance2': %s", errMsg)
	}

	// Should contain failure count
	if !strings.Contains(errMsg, "2") {
		t.Errorf("Error message should contain failure count '2': %s", errMsg)
	}

	// Test Unwrap() method
	unwrapped := updateErr.Unwrap()
	if len(unwrapped) != 2 {
		t.Errorf("Unwrap() returned %d errors, expected 2", len(unwrapped))
	}

	// Verify unwrapped errors are correct
	for i, err := range unwrapped {
		if err == nil {
			t.Errorf("Unwrap()[%d] is nil", i)
		}
		var instanceErr *InstanceError
		if !errors.As(err, &instanceErr) {
			t.Errorf("Unwrap()[%d] should be *InstanceError, got %T", i, err)
		}
	}
}

// TestUpdateErrorEmpty tests UpdateError with no errors
func TestUpdateErrorEmpty(t *testing.T) {
	updateErr := &UpdateError{Errors: []*InstanceError{}}

	errMsg := updateErr.Error()
	if errMsg == "" {
		t.Error("Error() returned empty string for empty error list")
	}

	// Should indicate no error details
	if !strings.Contains(errMsg, "无错误详情") {
		t.Errorf("Error message should contain '无错误详情': %s", errMsg)
	}

	// Unwrap should return empty slice
	unwrapped := updateErr.Unwrap()
	if len(unwrapped) != 0 {
		t.Errorf("Unwrap() returned %d errors, expected 0", len(unwrapped))
	}
}

// TestUpdateErrorSingle tests UpdateError with single error
func TestUpdateErrorSingle(t *testing.T) {
	errs := []*InstanceError{
		{InstanceName: "single-instance", Operation: "stop", Port: 8080, Err: errors.New("single failure")},
	}

	updateErr := &UpdateError{Errors: errs}

	errMsg := updateErr.Error()
	if !strings.Contains(errMsg, "single-instance") {
		t.Errorf("Error message should contain 'single-instance': %s", errMsg)
	}

	// Should contain failure count
	if !strings.Contains(errMsg, "1") {
		t.Errorf("Error message should contain failure count '1': %s", errMsg)
	}

	unwrapped := updateErr.Unwrap()
	if len(unwrapped) != 1 {
		t.Errorf("Unwrap() returned %d errors, expected 1", len(unwrapped))
	}
}

// TestUpdateErrorUnwrapSupportsErrorsIs tests errors.Is support
func TestUpdateErrorUnwrapSupportsErrorsIs(t *testing.T) {
	underlyingErr := errors.New("underlying error")
	errs := []*InstanceError{
		{InstanceName: "inst1", Operation: "stop", Port: 8080, Err: underlyingErr},
	}

	updateErr := &UpdateError{Errors: errs}

	// Verify we can access underlying errors through Unwrap
	unwrapped := updateErr.Unwrap()
	if len(unwrapped) == 0 {
		t.Fatal("Unwrap() returned empty slice")
	}

	// Test errors.Is works on unwrapped errors
	instanceErr := unwrapped[0]
	if !errors.Is(instanceErr, underlyingErr) {
		t.Error("errors.Is should find underlying error through InstanceError")
	}
}

// TestUpdateErrorUnwrapSupportsErrorsAs tests errors.As support
func TestUpdateErrorUnwrapSupportsErrorsAs(t *testing.T) {
	errs := []*InstanceError{
		{InstanceName: "inst1", Operation: "stop", Port: 8080, Err: errors.New("test error")},
	}

	updateErr := &UpdateError{Errors: errs}

	// Verify we can access unwrapped errors
	unwrapped := updateErr.Unwrap()
	if len(unwrapped) == 0 {
		t.Fatal("Unwrap() returned empty slice")
	}

	// Test errors.As works on unwrapped errors
	var instanceErr *InstanceError
	if !errors.As(unwrapped[0], &instanceErr) {
		t.Error("errors.As should extract *InstanceError from unwrapped error")
	}

	if instanceErr.InstanceName != "inst1" {
		t.Errorf("InstanceError.InstanceName = %q, want %q", instanceErr.InstanceName, "inst1")
	}
}

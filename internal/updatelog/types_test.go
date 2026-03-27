package updatelog

import (
	"testing"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/instance"
)

func TestUpdateLogStruct(t *testing.T) {
	now := time.Now().UTC()
	ul := UpdateLog{
		ID:          "test-uuid",
		StartTime:   now,
		EndTime:     now.Add(5 * time.Second),
		Duration:    5000,
		Status:      StatusSuccess,
		Instances:   []InstanceUpdateDetail{},
		TriggeredBy: "api-trigger",
	}

	if ul.ID != "test-uuid" {
		t.Errorf("Expected ID 'test-uuid', got '%s'", ul.ID)
	}
	if ul.Duration != 5000 {
		t.Errorf("Expected Duration 5000, got %d", ul.Duration)
	}
	if ul.Status != StatusSuccess {
		t.Errorf("Expected Status %s, got %s", StatusSuccess, ul.Status)
	}
	if ul.TriggeredBy != "api-trigger" {
		t.Errorf("Expected TriggeredBy 'api-trigger', got '%s'", ul.TriggeredBy)
	}
	if len(ul.Instances) != 0 {
		t.Errorf("Expected empty Instances, got %d", len(ul.Instances))
	}
}

func TestInstanceUpdateDetailStruct(t *testing.T) {
	detail := InstanceUpdateDetail{
		Name:          "gateway",
		Port:          18790,
		Status:        "success",
		ErrorMessage:  "",
		LogStartIndex: 0,
		LogEndIndex:   100,
		StopDuration:  500,
		StartDuration: 3000,
	}

	if detail.Name != "gateway" {
		t.Errorf("Expected Name 'gateway', got '%s'", detail.Name)
	}
	if detail.Port != 18790 {
		t.Errorf("Expected Port 18790, got %d", detail.Port)
	}
	if detail.Status != "success" {
		t.Errorf("Expected Status 'success', got '%s'", detail.Status)
	}
	if detail.ErrorMessage != "" {
		t.Errorf("Expected empty ErrorMessage, got '%s'", detail.ErrorMessage)
	}
	if detail.LogStartIndex != 0 {
		t.Errorf("Expected LogStartIndex 0, got %d", detail.LogStartIndex)
	}
	if detail.LogEndIndex != 100 {
		t.Errorf("Expected LogEndIndex 100, got %d", detail.LogEndIndex)
	}
	if detail.StopDuration != 500 {
		t.Errorf("Expected StopDuration 500, got %d", detail.StopDuration)
	}
	if detail.StartDuration != 3000 {
		t.Errorf("Expected StartDuration 3000, got %d", detail.StartDuration)
	}
}

func TestUpdateStatusConstants(t *testing.T) {
	if StatusSuccess != UpdateStatus("success") {
		t.Errorf("Expected StatusSuccess 'success', got '%s'", StatusSuccess)
	}
	if StatusPartialSuccess != UpdateStatus("partial_success") {
		t.Errorf("Expected StatusPartialSuccess 'partial_success', got '%s'", StatusPartialSuccess)
	}
	if StatusFailed != UpdateStatus("failed") {
		t.Errorf("Expected StatusFailed 'failed', got '%s'", StatusFailed)
	}
}

func TestDetermineStatus(t *testing.T) {
	tests := []struct {
		name     string
		result   *instance.UpdateResult
		expected UpdateStatus
	}{
		{
			name: "all success",
			result: &instance.UpdateResult{
				Stopped:     []string{"gateway", "worker"},
				Started:     []string{"gateway", "worker"},
				StopFailed:  []*instance.InstanceError{},
				StartFailed: []*instance.InstanceError{},
			},
			expected: StatusSuccess,
		},
		{
			name: "partial success - some started, some failed",
			result: &instance.UpdateResult{
				Stopped:    []string{"gateway"},
				Started:    []string{"gateway"},
				StopFailed: []*instance.InstanceError{},
				StartFailed: []*instance.InstanceError{
					{InstanceName: "worker", Operation: "start", Port: 18791, Err: nil},
				},
			},
			expected: StatusPartialSuccess,
		},
		{
			name: "partial success - some stopped, some start failed",
			result: &instance.UpdateResult{
				Stopped:    []string{"gateway"},
				Started:    []string{},
				StopFailed: []*instance.InstanceError{},
				StartFailed: []*instance.InstanceError{
					{InstanceName: "worker", Operation: "start", Port: 18791, Err: nil},
				},
			},
			expected: StatusPartialSuccess,
		},
		{
			name: "all failed",
			result: &instance.UpdateResult{
				Stopped:     []string{},
				Started:     []string{},
				StopFailed: []*instance.InstanceError{
					{InstanceName: "gateway", Operation: "stop", Port: 18790, Err: nil},
				},
				StartFailed: []*instance.InstanceError{
					{InstanceName: "gateway", Operation: "start", Port: 18790, Err: nil},
				},
			},
			expected: StatusFailed,
		},
		{
			name: "all failed - only stop errors",
			result: &instance.UpdateResult{
				Stopped:     []string{},
				Started:     []string{},
				StopFailed: []*instance.InstanceError{
					{InstanceName: "gateway", Operation: "stop", Port: 18790, Err: nil},
				},
				StartFailed: []*instance.InstanceError{},
			},
			expected: StatusFailed,
		},
		{
			name: "partial success - only stop failed but some started",
			result: &instance.UpdateResult{
				Stopped:    []string{},
				Started:    []string{"worker"},
				StopFailed: []*instance.InstanceError{
					{InstanceName: "gateway", Operation: "stop", Port: 18790, Err: nil},
				},
				StartFailed: []*instance.InstanceError{},
			},
			expected: StatusPartialSuccess,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineStatus(tt.result)
			if got != tt.expected {
				t.Errorf("DetermineStatus() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestBuildInstanceDetails(t *testing.T) {
	t.Run("all success", func(t *testing.T) {
		result := &instance.UpdateResult{
			Stopped:     []string{"gateway"},
			Started:     []string{"gateway"},
			StopFailed:  []*instance.InstanceError{},
			StartFailed: []*instance.InstanceError{},
		}
		details := BuildInstanceDetails(result)
		if len(details) == 0 {
			t.Error("Expected non-empty details for successful result")
		}
		// Should have at least one success detail
		foundSuccess := false
		for _, d := range details {
			if d.Status == "success" {
				foundSuccess = true
				break
			}
		}
		if !foundSuccess {
			t.Error("Expected at least one success instance detail")
		}
	})

	t.Run("with failures", func(t *testing.T) {
		result := &instance.UpdateResult{
			Stopped: []string{"gateway"},
			Started: []string{"gateway"},
			StopFailed: []*instance.InstanceError{},
			StartFailed: []*instance.InstanceError{
				{InstanceName: "worker", Operation: "start", Port: 18791, Err: nil},
			},
		}
		details := BuildInstanceDetails(result)
		if len(details) == 0 {
			t.Error("Expected non-empty details for partial success result")
		}
		// Should have a failed detail for worker
		foundFailed := false
		for _, d := range details {
			if d.Name == "worker" && d.Status == "failed" {
				foundFailed = true
				break
			}
		}
		if !foundFailed {
			t.Error("Expected failed instance detail for 'worker'")
		}
	})
}

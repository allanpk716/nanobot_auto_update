package instance

import (
	"errors"
	"testing"
)

func TestInstanceError_Error(t *testing.T) {
	tests := []struct {
		name         string
		instanceName string
		operation   string
		port        uint32
		err         error
		want        string
	}{
		{
			name:         "stop operation with error",
			instanceName: "test-instance",
			operation:   "stop",
			port:        18790,
			err:         errors.New("process not found"),
			want:        `停止实例 "test-instance" 失败 (port=18790): process not found`,
		},
		{
			name:         "start operation with error",
			instanceName: "production-instance",
			operation:   "start",
			port:        18791,
			err:         errors.New("timeout waiting for port"),
			want:        `启动实例 "production-instance" 失败 (port=18791): timeout waiting for port`,
		},
		{
			name:         "nil underlying error",
			instanceName: "test-instance",
			operation:   "stop",
			port:        18790,
			err:         nil,
			want:        `停止实例 "test-instance" 失败 (port=18790): <nil>`,
		},
		{
			name:         "empty instance name",
			instanceName: "",
			operation:   "start",
			port:        18792,
			err:         errors.New("command failed"),
			want:        `启动实例 "" 失败 (port=18792): command failed`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ie := &InstanceError{
				InstanceName: tt.instanceName,
				Operation:    tt.operation,
				Port:         tt.port,
				Err:          tt.err,
			}
			if got := ie.Error(); got != tt.want {
				t.Errorf("InstanceError.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInstanceError_Unwrap(t *testing.T) {
	underlyingErr := errors.New("underlying error")
	ie := &InstanceError{
		InstanceName: "test-instance",
		Operation:    "stop",
		Port:         18790,
		Err:          underlyingErr,
	}

	// Test Unwrap returns underlying error
	unwrapped := ie.Unwrap()
	if unwrapped != underlyingErr {
		t.Errorf("InstanceError.Unwrap() = %v, want %v", unwrapped, underlyingErr)
	}

	// Test errors.Is works
	if !errors.Is(ie, underlyingErr) {
		t.Error("errors.Is(ie, underlyingErr) = false, want true")
	}

	// Test errors.As works
	var extracted *InstanceError
	if !errors.As(ie, &extracted) {
		t.Error("errors.As(ie, &extracted) = false, want true")
	}
	if extracted != ie {
		t.Errorf("extracted error = %v, want %v", extracted, ie)
	}
}

func TestInstanceError_operationText(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		want      string
	}{
		{
			name:      "stop operation",
			operation: "stop",
			want:      "停止实例",
		},
		{
			name:      "start operation",
			operation: "start",
			want:      "启动实例",
		},
		{
			name:      "unknown operation",
			operation: "unknown",
			want:      "未知操作实例",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ie := &InstanceError{
				InstanceName: "test",
				Operation:    tt.operation,
				Port:         18790,
				Err:          errors.New("test"),
			}
			got := ie.operationText()
			if got != tt.want {
				t.Errorf("operationText() = %q, want %q", got, tt.want)
			}
		})
	}
}

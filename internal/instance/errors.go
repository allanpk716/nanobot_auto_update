package instance

import (
	"fmt"
)

// InstanceError represents an error that occurred during instance lifecycle operation
type InstanceError struct {
	InstanceName string
	Operation    string // "stop" or "start"
	Port         uint32
	Err          error
}

// Error returns a formatted error message in Chinese
func (e *InstanceError) Error() string {
	return fmt.Sprintf(`%s "%s" 失败 (port=%d): %v`,
		e.operationText(),
		e.InstanceName,
		e.Port,
		e.Err,
	)
}

// Unwrap returns the underlying error for error chain traversal
func (e *InstanceError) Unwrap() error {
	return e.Err
}

// operationText converts operation type to Chinese text
func (e *InstanceError) operationText() string {
	switch e.Operation {
	case "stop":
		return "停止实例"
	case "start":
		return "启动实例"
	default:
		return "未知操作实例"
	}
}

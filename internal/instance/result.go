package instance

import (
	"fmt"
	"strings"
)

// UpdateResult 包含更新流程的所有结果
type UpdateResult struct {
	Stopped     []string         `json:"stopped"`      // 成功停止的实例名称
	Started     []string         `json:"started"`      // 成功启动的实例名称
	StopFailed  []*InstanceError `json:"stop_failed"`  // 停止失败的实例错误
	StartFailed []*InstanceError `json:"start_failed"` // 启动失败的实例错误
}

// HasErrors 检查是否有任何失败
func (r *UpdateResult) HasErrors() bool {
	return len(r.StopFailed) > 0 || len(r.StartFailed) > 0
}

// UpdateError 聚合所有实例错误
type UpdateError struct {
	Errors []*InstanceError
}

// Error 实现 error 接口
func (e *UpdateError) Error() string {
	if len(e.Errors) == 0 {
		return "更新失败: 无错误详情"
	}

	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("更新失败 (%d 个实例失败):\n", len(e.Errors)))
	for _, err := range e.Errors {
		msg.WriteString(fmt.Sprintf("  ✗ %s\n", err.InstanceName))
	}
	return msg.String()
}

// Unwrap 返回错误列表,支持 errors.Is/As 遍历
func (e *UpdateError) Unwrap() []error {
	errs := make([]error, len(e.Errors))
	for i, err := range e.Errors {
		errs[i] = err
	}
	return errs
}

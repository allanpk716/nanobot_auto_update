package updatelog

import (
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/instance"
)

// UpdateStatus represents the overall status of an update operation
type UpdateStatus string

const (
	StatusSuccess        UpdateStatus = "success"
	StatusPartialSuccess UpdateStatus = "partial_success"
	StatusFailed         UpdateStatus = "failed"
)

// InstanceUpdateDetail contains per-instance update result details
type InstanceUpdateDetail struct {
	Name          string `json:"name"`
	Port          uint32 `json:"port"`
	Status        string `json:"status"`            // "success" or "failed"
	ErrorMessage  string `json:"error_message"`     // non-empty if failed
	LogStartIndex int    `json:"log_start_index"`   // LogBuffer start index (Phase 33 integration)
	LogEndIndex   int    `json:"log_end_index"`     // LogBuffer end index (Phase 33 integration)
	StopDuration  int64  `json:"stop_duration_ms"`  // Stop operation duration in milliseconds
	StartDuration int64  `json:"start_duration_ms"` // Start operation duration in milliseconds
}

// UpdateLog represents a complete update operation record
type UpdateLog struct {
	ID          string                 `json:"id"`            // UUID v4
	StartTime   time.Time              `json:"start_time"`    // RFC 3339, UTC
	EndTime     time.Time              `json:"end_time"`      // RFC 3339, UTC
	Duration    int64                  `json:"duration_ms"`   // Total duration in milliseconds
	Status      UpdateStatus           `json:"status"`        // success/partial_success/failed
	Instances   []InstanceUpdateDetail `json:"instances"`     // Per-instance details
	TriggeredBy string                 `json:"triggered_by"`  // "api-trigger"
}

// DetermineStatus determines the overall update status based on UpdateResult
func DetermineStatus(result *instance.UpdateResult) UpdateStatus {
	if result.HasErrors() {
		if len(result.Started) > 0 || len(result.Stopped) > 0 {
			return StatusPartialSuccess
		}
		return StatusFailed
	}
	return StatusSuccess
}

// BuildInstanceDetails creates InstanceUpdateDetail slice from UpdateResult.
// For Phase 30, LogStartIndex and LogEndIndex are set to 0 (Phase 33 integration).
// For Phase 30, StopDuration and StartDuration are set to 0 (Phase 33 integration).
func BuildInstanceDetails(result *instance.UpdateResult) []InstanceUpdateDetail {
	details := []InstanceUpdateDetail{}

	// Track which instances have been added to avoid duplicates
	added := make(map[string]bool)

	// Add failed instances from StopFailed
	for _, err := range result.StopFailed {
		details = append(details, InstanceUpdateDetail{
			Name:         err.InstanceName,
			Port:         err.Port,
			Status:       "failed",
			ErrorMessage: err.Error(),
		})
		added[err.InstanceName] = true
	}

	// Add failed instances from StartFailed
	for _, err := range result.StartFailed {
		if added[err.InstanceName] {
			// Already added from StopFailed, skip duplicate
			continue
		}
		details = append(details, InstanceUpdateDetail{
			Name:         err.InstanceName,
			Port:         err.Port,
			Status:       "failed",
			ErrorMessage: err.Error(),
		})
		added[err.InstanceName] = true
	}

	// Add successful instances from Stopped
	for _, name := range result.Stopped {
		if !added[name] {
			details = append(details, InstanceUpdateDetail{
				Name:   name,
				Status: "success",
			})
			added[name] = true
		}
	}

	// Add successful instances from Started
	for _, name := range result.Started {
		if !added[name] {
			details = append(details, InstanceUpdateDetail{
				Name:   name,
				Status: "success",
			})
			added[name] = true
		}
	}

	return details
}

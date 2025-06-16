package dto

import (
	"time"
)

// ExecutionHistoryResponse is the DTO for API responses containing execution history details.
type ExecutionHistoryResponse struct {
	ID         uint      `json:"id"`
	JobID      uint      `json:"job_id"`
	ScheduleID uint      `json:"schedule_id"`
	Status     string    `json:"status"`
	ExecutedAt time.Time `json:"executed_at"`
	Duration   int64     `json:"duration_ms"`
	Output     string    `json:"output"`
}

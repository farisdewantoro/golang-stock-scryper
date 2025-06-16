package dto

import (
	"database/sql"
	"encoding/json"
	"time"
)

// RetryPolicyDTO represents the retry policy for a job in API requests/responses.
type RetryPolicyDTO struct {
	MaxRetries      int    `json:"max_retries"`
	BackoffStrategy string `json:"backoff_strategy"` // e.g., "exponential", "fixed"
	InitialInterval string `json:"initial_interval"` // e.g., "5s", "1m"
}

// ScheduleDTO represents a task schedule in API requests.
type ScheduleDTO struct {
	CronExpression string `json:"cron_expression"`
	IsActive       bool   `json:"is_active"`
}

// CreateJobRequest is the DTO for creating a new job.
type CreateJobRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload" swaggertype:"object"`
	RetryPolicy RetryPolicyDTO  `json:"retry_policy"`
	Timeout     int             `json:"timeout"` // in seconds
	Schedules   []ScheduleDTO   `json:"schedules"`
}

// UpdateJobRequest is the DTO for updating an existing job.
type UpdateJobRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload" swaggertype:"object"`
	RetryPolicy RetryPolicyDTO  `json:"retry_policy"`
	Timeout     int             `json:"timeout"` // in seconds
	Schedules   []ScheduleDTO   `json:"schedules"`
}

// ScheduleResponseDTO represents a task schedule in API responses.
type ScheduleResponseDTO struct {
	ID             uint         `json:"id"`
	CronExpression string       `json:"cron_expression"`
	IsActive       bool         `json:"is_active"`
	NextExecution  sql.NullTime `json:"next_execution" swaggertype:"string" format:"date-time"`
	LastExecution  sql.NullTime `json:"last_execution" swaggertype:"string" format:"date-time"`
}

// JobResponse is the DTO for API responses containing job details.
type JobResponse struct {
	ID          uint                  `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Type        string                `json:"type"`
	Payload     json.RawMessage       `json:"payload" swaggertype:"object"`
	RetryPolicy RetryPolicyDTO        `json:"retry_policy"`
	Timeout     int                   `json:"timeout"`
	Schedules   []ScheduleResponseDTO `json:"schedules"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
}

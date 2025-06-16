package dto

import (
	"database/sql"
	"time"
)

// CreateScheduleRequest defines the DTO for creating a new schedule.
type CreateScheduleRequest struct {
	JobID          uint   `json:"job_id"`
	CronExpression string `json:"cron_expression"`
	IsActive       bool   `json:"is_active"`
}

// UpdateScheduleRequest defines the DTO for updating an existing schedule.
type UpdateScheduleRequest struct {
	CronExpression string `json:"cron_expression"`
	IsActive       bool   `json:"is_active"`
}

// ScheduleResponse is the DTO for API responses containing schedule details.
type ScheduleResponse struct {
	ID             uint         `json:"id"`
	JobID          uint         `json:"job_id"`
	CronExpression string       `json:"cron_expression"`
	IsActive       bool         `json:"is_active"`
	NextExecution  sql.NullTime `json:"next_execution" swaggertype:"string" format:"date-time"`
	LastExecution  sql.NullTime `json:"last_execution" swaggertype:"string" format:"date-time"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

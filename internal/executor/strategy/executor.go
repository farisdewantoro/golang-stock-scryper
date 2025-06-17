package strategy

import (
	"context"

	"golang-stock-scryper/internal/entity"
)

const (
	SUCCESS = "success"
	FAILED  = "failed"
	SKIPPED = "skipped"
)

// JobExecutionStrategy defines the interface for different job execution strategies.
type JobExecutionStrategy interface {
	Execute(ctx context.Context, job *entity.Job) (string, error)
	GetType() entity.JobType
}

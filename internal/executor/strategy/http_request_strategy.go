package strategy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/pkg/logger"
)

// HTTPJobDetails defines the structure for HTTP job payloads.
type HTTPJobDetails struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
}

// HTTPStrategy executes HTTP-based jobs.
type HTTPStrategy struct {
	logger *logger.Logger
}

// NewHTTPStrategy creates a new HTTPStrategy.
func NewHTTPStrategy(log *logger.Logger) JobExecutionStrategy {
	return &HTTPStrategy{logger: log}
}

// GetType returns the job type this strategy handles.
func (s *HTTPStrategy) GetType() entity.JobType {
	return entity.JobTypeHTTP
}

// Execute performs the HTTP request defined in the job's payload.
func (s *HTTPStrategy) Execute(ctx context.Context, job *entity.Job) (string, error) {
	var details HTTPJobDetails
	if err := json.Unmarshal(job.Payload, &details); err != nil {
		s.logger.Error("Failed to unmarshal job payload", logger.ErrorField(err), logger.Field("job_id", job.ID))
		return "", fmt.Errorf("failed to unmarshal job payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, details.Method, details.URL, bytes.NewBuffer(details.Body))
	if err != nil {
		s.logger.Error("Failed to create HTTP request", logger.ErrorField(err), logger.Field("job_id", job.ID))
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	for key, value := range details.Headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error("Failed to execute HTTP request", logger.ErrorField(err), logger.Field("job_id", job.ID))
		return "", fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error("Failed to read response body", logger.ErrorField(err), logger.Field("job_id", job.ID))
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		err := fmt.Errorf("http request failed with status code %d: %s", resp.StatusCode, string(bodyBytes))
		s.logger.Error("HTTP request failed", logger.ErrorField(err), logger.Field("job_id", job.ID))
		return string(bodyBytes), err
	}

	s.logger.Info("HTTP job executed successfully", logger.Field("job_id", job.ID), logger.Field("status_code", resp.StatusCode))
	return string(bodyBytes), nil
}

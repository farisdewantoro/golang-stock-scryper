package http

import (
	"net/http"
	"strconv"

	"golang-stock-scryper/internal/scheduler/dto"
	"golang-stock-scryper/internal/scheduler/service"
	"golang-stock-scryper/pkg/logger"

	"github.com/labstack/echo/v4"
)

// JobHandler handles HTTP requests for jobs.
type JobHandler struct {
	jobService service.JobService
	logger     *logger.Logger
}

// NewJobHandler creates a new JobHandler.
func NewJobHandler(jobService service.JobService, logger *logger.Logger) *JobHandler {
	return &JobHandler{jobService: jobService, logger: logger}
}

// RegisterRoutes registers the job routes to the Echo group.
func (h *JobHandler) RegisterRoutes(g *echo.Group) {
	g.POST("", h.CreateJob)
	g.GET("", h.GetAllJobs)
	g.GET("/:id", h.GetJobByID)
	g.PUT("/:id", h.UpdateJob)
	g.DELETE("/:id", h.DeleteJob)
}

// CreateJob godoc
// @Summary Create a new job
// @Description Create a new job with schedules
// @Tags jobs
// @Accept  json
// @Produce  json
// @Param   job  body    dto.CreateJobRequest   true    "Job to create"
// @Success 201 {object} dto.JobResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /jobs [post]
func (h *JobHandler) CreateJob(c echo.Context) error {
	var req dto.CreateJobRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid request payload"})
	}

	// TODO: Add validation for the request payload

	jobResponse, err := h.jobService.CreateJob(c.Request().Context(), &req)
	if err != nil {
		// TODO: Differentiate between different error types (e.g., validation, db error)
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, jobResponse)
}

// GetJobByID godoc
// @Summary Get a job by ID
// @Description Get a single job by its ID
// @Tags jobs
// @Produce  json
// @Param   id  path    int true    "Job ID"
// @Success 200 {object} dto.JobResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /jobs/{id} [get]
func (h *JobHandler) GetJobByID(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid job ID"})
	}

	jobResponse, err := h.jobService.GetJobByID(c.Request().Context(), uint(id))
	if err != nil {
		// TODO: Handle not found error specifically
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, jobResponse)
}

// GetAllJobs godoc
// @Summary Get all jobs
// @Description Get all jobs
// @Tags jobs
// @Produce  json
// @Success 200 {array} dto.JobResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /jobs [get]
func (h *JobHandler) GetAllJobs(c echo.Context) error {
	jobs, err := h.jobService.GetAllJobs(c.Request().Context())
	if err != nil {
		h.logger.Error("Failed to get all jobs", logger.ErrorField(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get jobs"})
	}
	return c.JSON(http.StatusOK, jobs)
}

// DeleteJob godoc
// @Summary Delete a job
// @Description Delete a job by its ID
// @Tags jobs
// @Produce  json
// @Param   id  path    int true    "Job ID"
// @Success 204 {object} nil
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /jobs/{id} [delete]
func (h *JobHandler) DeleteJob(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid job ID"})
	}

	if err := h.jobService.DeleteJob(c.Request().Context(), uint(id)); err != nil {
		// The service layer already logs the error
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete job"})
	}

	return c.NoContent(http.StatusNoContent)
}

// UpdateJob godoc
// @Summary Update an existing job
// @Description Update an existing job with the given details
// @Tags jobs
// @Accept  json
// @Produce  json
// @Param   id  path    int true    "Job ID"
// @Param   job  body    dto.UpdateJobRequest   true    "Job to update"
// @Success 200 {object} dto.JobResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /jobs/{id} [put]
func (h *JobHandler) UpdateJob(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid job ID"})
	}

	var req dto.UpdateJobRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid request payload"})
	}

	// TODO: Add validation for the request payload

	jobResponse, err := h.jobService.UpdateJob(c.Request().Context(), uint(id), &req)
	if err != nil {
		// TODO: Differentiate between different error types (e.g., not found, validation, db error)
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, jobResponse)
}

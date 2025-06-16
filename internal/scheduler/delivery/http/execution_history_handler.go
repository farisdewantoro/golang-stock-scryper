package http

import (
	"net/http"
	"strconv"

	"golang-stock-scryper/internal/scheduler/service"
	"golang-stock-scryper/pkg/logger"

	"github.com/labstack/echo/v4"
)

// ExecutionHistoryHandler handles HTTP requests for execution history.
type ExecutionHistoryHandler struct {
	historyService service.ExecutionHistoryService
	logger         *logger.Logger
}

// NewExecutionHistoryHandler creates a new ExecutionHistoryHandler.
func NewExecutionHistoryHandler(historyService service.ExecutionHistoryService, logger *logger.Logger) *ExecutionHistoryHandler {
	return &ExecutionHistoryHandler{historyService: historyService, logger: logger}
}

// RegisterRoutes registers the execution history routes to the Echo group.
func (h *ExecutionHistoryHandler) RegisterRoutes(g *echo.Group) {
	g.GET("", h.GetAllExecutionHistories)
	g.GET("/:id", h.GetExecutionHistoryByID)
}

// RegisterJobRoutes registers the job-specific execution history routes.
func (h *ExecutionHistoryHandler) RegisterJobRoutes(g *echo.Group) {
	g.GET("/:id/executions", h.GetExecutionHistoriesByJobID)
}

// GetAllExecutionHistories godoc
// @Summary Get all execution histories
// @Description Get all execution history records
// @Tags executions
// @Produce  json
// @Success 200 {array} dto.ExecutionHistoryResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /executions [get]
func (h *ExecutionHistoryHandler) GetAllExecutionHistories(c echo.Context) error {
	histories, err := h.historyService.GetAllExecutionHistories(c.Request().Context())
	if err != nil {
		h.logger.Error("Failed to get all execution histories", logger.ErrorField(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get execution histories"})
	}
	return c.JSON(http.StatusOK, histories)
}

// GetExecutionHistoryByID godoc
// @Summary Get an execution history by ID
// @Description Get a single execution history record by its ID
// @Tags executions
// @Produce  json
// @Param   id  path    int true    "Execution History ID"
// @Success 200 {object} dto.ExecutionHistoryResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /executions/{id} [get]
func (h *ExecutionHistoryHandler) GetExecutionHistoryByID(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid history ID"})
	}

	history, err := h.historyService.GetExecutionHistoryByID(c.Request().Context(), uint(id))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, history)
}

// GetExecutionHistoriesByJobID godoc
// @Summary Get execution histories for a job
// @Description Get all execution history records for a specific job ID
// @Tags jobs
// @Produce  json
// @Param   id  path    int true    "Job ID"
// @Success 200 {array} dto.ExecutionHistoryResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /jobs/{id}/executions [get]
func (h *ExecutionHistoryHandler) GetExecutionHistoriesByJobID(c echo.Context) error {
	jobID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid job ID"})
	}

	histories, err := h.historyService.GetExecutionHistoriesByJobID(c.Request().Context(), uint(jobID))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, histories)
}

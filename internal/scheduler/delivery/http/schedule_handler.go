package http

import (
	"net/http"
	"strconv"

	"golang-stock-scryper/internal/scheduler/dto"
	"golang-stock-scryper/internal/scheduler/service"
	"golang-stock-scryper/pkg/logger"

	"github.com/labstack/echo/v4"
)

// ScheduleHandler handles HTTP requests for schedules.
type ScheduleHandler struct {
	scheduleService service.ScheduleService
	logger          *logger.Logger
}

// NewScheduleHandler creates a new ScheduleHandler.
func NewScheduleHandler(scheduleService service.ScheduleService, logger *logger.Logger) *ScheduleHandler {
	return &ScheduleHandler{scheduleService: scheduleService, logger: logger}
}

// RegisterRoutes registers the schedule routes to the Echo group.
func (h *ScheduleHandler) RegisterRoutes(g *echo.Group) {
	g.POST("", h.CreateSchedule)
	g.GET("", h.GetAllSchedules)
	g.GET("/:id", h.GetScheduleByID)
	g.PUT("/:id", h.UpdateSchedule)
	g.DELETE("/:id", h.DeleteSchedule)
}

// CreateSchedule godoc
// @Summary Create a new schedule
// @Description Create a new schedule with the given details
// @Tags schedules
// @Accept  json
// @Produce  json
// @Param   schedule  body    dto.CreateScheduleRequest   true    "Schedule to create"
// @Success 201 {object} dto.ScheduleResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /schedules [post]
func (h *ScheduleHandler) CreateSchedule(c echo.Context) error {
	var req dto.CreateScheduleRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid request payload"})
	}

	scheduleResponse, err := h.scheduleService.CreateSchedule(c.Request().Context(), &req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, scheduleResponse)
}

// GetScheduleByID godoc
// @Summary Get a schedule by its ID
// @Description Get a schedule by its ID
// @Tags schedules
// @Produce  json
// @Param   id  path    int true    "Schedule ID"
// @Success 200 {object} dto.ScheduleResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /schedules/{id} [get]
func (h *ScheduleHandler) GetScheduleByID(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid schedule ID"})
	}

	scheduleResponse, err := h.scheduleService.GetScheduleByID(c.Request().Context(), uint(id))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, scheduleResponse)
}

// GetAllSchedules godoc
// @Summary Get all schedules
// @Description Get all schedules
// @Tags schedules
// @Produce  json
// @Success 200 {array} dto.ScheduleResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /schedules [get]
func (h *ScheduleHandler) GetAllSchedules(c echo.Context) error {
	schedules, err := h.scheduleService.GetAllSchedules(c.Request().Context())
	if err != nil {
		h.logger.Error("Failed to get all schedules", logger.ErrorField(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get schedules"})
	}
	return c.JSON(http.StatusOK, schedules)
}

// UpdateSchedule godoc
// @Summary Update an existing schedule
// @Description Update an existing schedule with the given details
// @Tags schedules
// @Accept  json
// @Produce  json
// @Param   id  path    int true    "Schedule ID"
// @Param   schedule  body    dto.UpdateScheduleRequest   true    "Schedule to update"
// @Success 200 {object} dto.ScheduleResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /schedules/{id} [put]
func (h *ScheduleHandler) UpdateSchedule(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid schedule ID"})
	}

	var req dto.UpdateScheduleRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid request payload"})
	}

	scheduleResponse, err := h.scheduleService.UpdateSchedule(c.Request().Context(), uint(id), &req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, scheduleResponse)
}

// DeleteSchedule godoc
// @Summary Delete a schedule
// @Description Delete a schedule by its ID
// @Tags schedules
// @Produce  json
// @Param   id  path    int true    "Schedule ID"
// @Success 204 {object} nil
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /schedules/{id} [delete]
func (h *ScheduleHandler) DeleteSchedule(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid schedule ID"})
	}

	if err := h.scheduleService.DeleteSchedule(c.Request().Context(), uint(id)); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete schedule"})
	}

	return c.NoContent(http.StatusNoContent)
}

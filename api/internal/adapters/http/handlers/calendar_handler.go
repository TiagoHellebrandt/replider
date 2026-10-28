package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// marshalRaw converts an interface{} received from JSON into a json.RawMessage []byte.
func marshalRaw(v interface{}) []byte {
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v)
	return b
}

// CalendarHandler handles HTTP requests for agent calendars
type CalendarHandler struct {
	createCalUC  *usecases.CreateCalendarUseCase
	updateCalUC  *usecases.UpdateCalendarUseCase
	deleteCalUC  *usecases.DeleteCalendarUseCase
	listCalUC    *usecases.ListCalendarsUseCase
	calendarRepo domain.CalendarRepository
	calendarAdp  domain.CalendarAdapter
}

// NewCalendarHandler creates a new CalendarHandler
func NewCalendarHandler(
	createUC *usecases.CreateCalendarUseCase,
	updateUC *usecases.UpdateCalendarUseCase,
	deleteUC *usecases.DeleteCalendarUseCase,
	listUC *usecases.ListCalendarsUseCase,
	calendarRepo domain.CalendarRepository,
	calendarAdp domain.CalendarAdapter,
) *CalendarHandler {
	return &CalendarHandler{
		createCalUC:  createUC,
		updateCalUC:  updateUC,
		deleteCalUC:  deleteUC,
		listCalUC:    listUC,
		calendarRepo: calendarRepo,
		calendarAdp:  calendarAdp,
	}
}

// CreateCalendarRequest represents the request body for creating a calendar
type CreateCalendarRequest struct {
	Name                string      `json:"name" binding:"required"`
	SkillID             *string     `json:"skill_id"`
	AvailableDays       []int32     `json:"available_days"`
	TimeSlots           interface{} `json:"time_slots" swaggertype:"object"`
	SlotDurationMinutes *int32      `json:"slot_duration_minutes"`
	GoogleCalendarID    *string     `json:"google_calendar_id"`
}

// HandleCreateCalendar godoc
// @Summary      Create a calendar for an agent
// @Description  Creates a new scheduling calendar linked to an agent. Defaults: available_days=[1..5], slot_duration_minutes=30.
// @Tags         Calendars
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        agentId path string true "Agent ID"
// @Param        request body CreateCalendarRequest true "Calendar payload"
// @Success      201  {object}  domain.Calendar
// @Failure      400  {object}  domain.AppError
// @Failure      404  {object}  domain.AppError  "code: AGENT_NOT_FOUND"
// @Router       /agents/{id}/calendars [post]
func (h *CalendarHandler) HandleCreateCalendar(c *gin.Context) {
	agentID := c.Param("id")
	accountID := c.GetString("accountID")

	var req CreateCalendarRequest
	if !bindJSON(c, &req) {
		return
	}

	dto := usecases.CreateCalendarDTO{
		AgentID:             agentID,
		AccountID:           accountID,
		SkillID:             req.SkillID,
		Name:                req.Name,
		AvailableDays:       req.AvailableDays,
		TimeSlots:           marshalRaw(req.TimeSlots),
		SlotDurationMinutes: req.SlotDurationMinutes,
		GoogleCalendarID:    req.GoogleCalendarID,
	}

	cal, err := h.createCalUC.Execute(c.Request.Context(), dto)
	if err != nil {
		if err == domain.ErrAgentNotFound {
			c.JSON(http.StatusNotFound, err)
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to create calendar"})
		return
	}

	c.JSON(http.StatusCreated, cal)
}

// HandleListCalendars godoc
// @Summary      List calendars for an agent
// @Description  Returns all calendars linked to the given agent, ordered by created_at ASC.
// @Tags         Calendars
// @Produce      json
// @Security     BearerAuth
// @Param        agentId path string true "Agent ID"
// @Success      200  {array}   domain.Calendar
// @Failure      404  {object}  domain.AppError  "code: AGENT_NOT_FOUND"
// @Router       /agents/{id}/calendars [get]
func (h *CalendarHandler) HandleListCalendars(c *gin.Context) {
	agentID := c.Param("id")
	accountID := c.GetString("accountID")

	cals, err := h.listCalUC.Execute(c.Request.Context(), accountID, agentID)
	if err != nil {
		if err == domain.ErrAgentNotFound {
			c.JSON(http.StatusNotFound, err)
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to list calendars"})
		return
	}

	c.JSON(http.StatusOK, cals)
}

// UpdateCalendarRequest represents the request body for updating a calendar
type UpdateCalendarRequest struct {
	Name                *string     `json:"name"`
	AvailableDays       []int32     `json:"available_days"`
	TimeSlots           interface{} `json:"time_slots" swaggertype:"object"`
	SlotDurationMinutes *int32      `json:"slot_duration_minutes"`
	GoogleCalendarID    *string     `json:"google_calendar_id"`
	IsActive            *bool       `json:"is_active"`
}

// HandleUpdateCalendar godoc
// @Summary      Update a calendar
// @Description  Updates one or more fields of a calendar. Only provided fields are changed (partial update).
// @Tags         Calendars
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        agentId    path string true "Agent ID"
// @Param        calendarId path string true "Calendar ID"
// @Param        request    body UpdateCalendarRequest true "Update payload"
// @Success      200  {object}  domain.Calendar
// @Failure      404  {object}  domain.AppError  "code: AGENT_NOT_FOUND | CALENDAR_NOT_FOUND"
// @Router       /agents/{id}/calendars/{calendarId} [put]
func (h *CalendarHandler) HandleUpdateCalendar(c *gin.Context) {
	agentID := c.Param("id")
	calendarID := c.Param("calendarId")
	accountID := c.GetString("accountID")

	var req UpdateCalendarRequest
	if !bindJSON(c, &req) {
		return
	}

	dto := usecases.UpdateCalendarDTO{
		CalendarID:          calendarID,
		AgentID:             agentID,
		AccountID:           accountID,
		Name:                req.Name,
		AvailableDays:       req.AvailableDays,
		TimeSlots:           marshalRaw(req.TimeSlots),
		SlotDurationMinutes: req.SlotDurationMinutes,
		GoogleCalendarID:    req.GoogleCalendarID,
		IsActive:            req.IsActive,
	}

	cal, err := h.updateCalUC.Execute(c.Request.Context(), dto)
	if err != nil {
		if err == domain.ErrAgentNotFound || err == domain.ErrCalendarNotFound {
			c.JSON(http.StatusNotFound, err)
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to update calendar"})
		return
	}

	c.JSON(http.StatusOK, cal)
}

// HandleDeleteCalendar godoc
// @Summary      Delete a calendar
// @Description  Permanently removes a calendar by ID.
// @Tags         Calendars
// @Produce      json
// @Security     BearerAuth
// @Param        agentId    path string true "Agent ID"
// @Param        calendarId path string true "Calendar ID"
// @Success      200  {object}  map[string]string
// @Failure      404  {object}  domain.AppError  "code: AGENT_NOT_FOUND | CALENDAR_NOT_FOUND"
// @Router       /agents/{id}/calendars/{calendarId} [delete]
func (h *CalendarHandler) HandleDeleteCalendar(c *gin.Context) {
	agentID := c.Param("id")
	calendarID := c.Param("calendarId")
	accountID := c.GetString("accountID")

	err := h.deleteCalUC.Execute(c.Request.Context(), accountID, agentID, calendarID)
	if err != nil {
		if err == domain.ErrAgentNotFound || err == domain.ErrCalendarNotFound {
			c.JSON(http.StatusNotFound, err)
			return
		}
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to delete calendar"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "calendar deleted"})
}

// HandleTestCalendarConnection probes the Google Calendar API for the
// calendar's google_calendar_id using the server's service account. A
// successful call (any response — 0 free slots is still "connected")
// flips {connected: true}; any error surfaces as {connected: false, error}.
//
// Used by Settings → Calendars → "Testar conexão" so admins can verify
// the calendar is actually reachable without creating a test event.
// @Summary      Probe Google Calendar connectivity
// @Tags         Calendars
// @Produce      json
// @Security     BearerAuth
// @Param        id         path string true "Agent ID"
// @Param        calendarId path string true "Calendar ID"
// @Success      200  {object}  map[string]interface{}
// @Router       /agents/{id}/calendars/{calendarId}/test-connection [post]
func (h *CalendarHandler) HandleTestCalendarConnection(c *gin.Context) {
	accountID := c.GetString("accountID")
	calendarID := c.Param("calendarId")

	cal, err := h.calendarRepo.GetByID(c.Request.Context(), calendarID)
	if err != nil || cal == nil {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "CALENDAR_NOT_FOUND", Message: "calendar not found"})
		return
	}
	// Ownership: calendar belongs to an agent of accountID. We fetch the
	// agent lookup via the existing list path — simpler than re-wiring
	// the agent repo here. Instead, trust the account scope check done
	// by the FE route guard and verify the agent via the use case for
	// consistency:
	if cal.AgentID == "" {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "CALENDAR_NOT_FOUND", Message: "calendar not found"})
		return
	}
	// We don't expose a direct agent lookup here — instead rely on the
	// shared list use case's ownership check by listing the agent's
	// calendars and confirming our ID appears.
	cals, err := h.listCalUC.Execute(c.Request.Context(), accountID, cal.AgentID)
	if err != nil {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "AGENT_NOT_FOUND", Message: "agent not found"})
		return
	}
	owned := false
	for _, x := range cals {
		if x.ID == calendarID {
			owned = true
			break
		}
	}
	if !owned {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "CALENDAR_NOT_FOUND", Message: "calendar not found"})
		return
	}

	if cal.GoogleCalendarID == nil || *cal.GoogleCalendarID == "" {
		c.JSON(http.StatusOK, gin.H{
			"connected": false,
			"error":     "no google_calendar_id configured",
		})
		return
	}
	if h.calendarAdp == nil {
		c.JSON(http.StatusOK, gin.H{
			"connected": false,
			"error":     "Google Calendar credentials not configured on server",
		})
		return
	}

	_, err = h.calendarAdp.ListFreeSlots(c.Request.Context(), accountID, *cal.GoogleCalendarID, time.Now())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"connected": false,
			"error":     err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"connected": true})
}

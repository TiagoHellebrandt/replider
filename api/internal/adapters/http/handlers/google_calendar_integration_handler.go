package handlers

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/usecases"
)

// GoogleCalendarIntegrationHandler exposes the per-account "Conectar
// Google Calendar" flow. All routes are admin-gated except the OAuth
// callback, which arrives as an unauthenticated browser redirect and
// authenticates via the HMAC-signed state instead.
type GoogleCalendarIntegrationHandler struct {
	uc           *usecases.GoogleCalendarIntegrationUseCase
	dashboardURL string
}

func NewGoogleCalendarIntegrationHandler(uc *usecases.GoogleCalendarIntegrationUseCase, dashboardURL string) *GoogleCalendarIntegrationHandler {
	return &GoogleCalendarIntegrationHandler{uc: uc, dashboardURL: dashboardURL}
}

// HandleStatus godoc
// @Summary Google Calendar integration status
// @Tags integrations
// @Produce json
// @Success 200 {object} usecases.IntegrationStatus
// @Router /integrations/google-calendar [get]
func (h *GoogleCalendarIntegrationHandler) HandleStatus(c *gin.Context) {
	status, err := h.uc.Status(c.Request.Context(), c.GetString("accountID"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

// HandleConnect godoc
// @Summary Start the Google Calendar OAuth flow
// @Tags integrations
// @Produce json
// @Success 200 {object} map[string]string
// @Router /integrations/google-calendar/connect [post]
func (h *GoogleCalendarIntegrationHandler) HandleConnect(c *gin.Context) {
	authURL, err := h.uc.AuthURL(c.GetString("accountID"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"auth_url": authURL})
}

// HandleCallback receives Google's browser redirect. It never renders
// JSON — success or failure, the user lands back on the dashboard with
// a query flag the app turns into a snackbar.
func (h *GoogleCalendarIntegrationHandler) HandleCallback(c *gin.Context) {
	redirect := func(result string) {
		c.Redirect(http.StatusFound, h.dashboardURL+"/settings?gcal="+url.QueryEscape(result))
	}
	if errParam := c.Query("error"); errParam != "" {
		// User clicked "cancel" on the consent screen.
		redirect("denied")
		return
	}
	accountID, err := h.uc.HandleCallback(c.Request.Context(), c.Query("state"), c.Query("code"))
	if err != nil {
		slog.Error("google calendar callback failed", "account_id", accountID, "error", err)
		redirect("error")
		return
	}
	redirect("connected")
}

// HandleDisconnect godoc
// @Summary Disconnect Google Calendar
// @Tags integrations
// @Success 204
// @Router /integrations/google-calendar [delete]
func (h *GoogleCalendarIntegrationHandler) HandleDisconnect(c *gin.Context) {
	if err := h.uc.Disconnect(c.Request.Context(), c.GetString("accountID")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// HandleListCalendars godoc
// @Summary List the connected Google account's writable calendars
// @Tags integrations
// @Produce json
// @Success 200 {object} map[string]any
// @Router /integrations/google-calendar/calendars [get]
func (h *GoogleCalendarIntegrationHandler) HandleListCalendars(c *gin.Context) {
	cals, err := h.uc.ListCalendars(c.Request.Context(), c.GetString("accountID"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"calendars": cals})
}

package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// PublicConfigHandler serves the unauthenticated client bootstrap config —
// the handful of server-side switches the app must know BEFORE anyone logs
// in (US-F001). Keep it to booleans/enums that are safe to publish: this
// endpoint is world-readable.
type PublicConfigHandler struct {
	registrationEnabled bool
}

func NewPublicConfigHandler(registrationEnabled bool) *PublicConfigHandler {
	return &PublicConfigHandler{registrationEnabled: registrationEnabled}
}

// PublicConfigResponse is the payload of GET /v1/config.
type PublicConfigResponse struct {
	// RegistrationEnabled mirrors REGISTRATION_ENABLED. When false the
	// register screen hides its form and shows an "em breve" state; the
	// API refuses the signup anyway (REGISTRATION_PAUSED, 503) — this is
	// only so the client never offers a door that is closed.
	RegistrationEnabled bool `json:"registration_enabled"`
}

// HandleGetPublicConfig godoc
// @Summary      Public client configuration
// @Description  Unauthenticated bootstrap flags for the web/mobile client. Currently exposes whether public registration is open.
// @Tags         config
// @Produce      json
// @Success      200  {object}  handlers.PublicConfigResponse  "Public configuration"
// @Router       /config [get]
func (h *PublicConfigHandler) HandleGetPublicConfig(c *gin.Context) {
	c.JSON(http.StatusOK, PublicConfigResponse{RegistrationEnabled: h.registrationEnabled})
}

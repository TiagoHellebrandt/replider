package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	waadapter "github.com/replider/api/internal/adapters/channels/whatsapp"
	"github.com/replider/api/internal/core/domain"
)

// HandleSyncTemplates godoc
// @Summary      Sync WhatsApp message templates from Meta
// @Description  Pulls the channel's approved/pending message templates from
// @Description  the Meta Graph API (message_templates) and upserts them into
// @Description  the local cache, returning the resulting set with meta_status.
// @Tags         channels
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Channel ID"
// @Success      200  {array}   domain.WhatsAppTemplate
// @Failure      400  {object}  domain.AppError
// @Failure      404  {object}  domain.AppError
// @Router       /channels/{id}/templates/sync [post]
func (h *WhatsAppConfigHandler) HandleSyncTemplates(c *gin.Context) {
	if h.templateSyncUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{Code: "TEMPLATE_SYNC_UNAVAILABLE", Message: "template sync is not configured"})
		return
	}

	accountID := c.GetString("accountID")
	channelID := c.Param("id")

	templates, err := h.templateSyncUC.Execute(c.Request.Context(), accountID, channelID)
	if err != nil {
		// Domain AppErrors (channel not found, missing credentials) carry their
		// own HTTP status via respondError; Meta transport errors map through the
		// shared catalog so the client gets an actionable rate-limit / auth message.
		if appErr, ok := err.(*domain.AppError); ok {
			respondError(c, appErr)
			return
		}
		status, mapped := mapMetaErrorAction(err, waadapter.ActionListTemplates,
			"TEMPLATE_SYNC_FAILED", "failed to sync templates")
		c.JSON(status, mapped)
		return
	}

	c.JSON(http.StatusOK, templates)
}

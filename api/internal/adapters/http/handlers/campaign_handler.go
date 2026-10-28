package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// CampaignHandler exposes the broadcast/campaign endpoints. All routes are
// admin-only and scoped to the caller's account_id (resolved from JWT).
type CampaignHandler struct {
	createUC *usecases.CreateCampaignUseCase
	listUC   *usecases.ListCampaignsUseCase
	getUC    *usecases.GetCampaignUseCase
	deleteUC *usecases.DeleteCampaignUseCase
	cancelUC *usecases.CancelCampaignUseCase
	repo     domain.CampaignRepository
}

func NewCampaignHandler(
	createUC *usecases.CreateCampaignUseCase,
	listUC *usecases.ListCampaignsUseCase,
	getUC *usecases.GetCampaignUseCase,
	deleteUC *usecases.DeleteCampaignUseCase,
	cancelUC *usecases.CancelCampaignUseCase,
	repo domain.CampaignRepository,
) *CampaignHandler {
	return &CampaignHandler{
		createUC: createUC,
		listUC:   listUC,
		getUC:    getUC,
		deleteUC: deleteUC,
		cancelUC: cancelUC,
		repo:     repo,
	}
}

// recipientInputJSON carries one CSV-style recipient: phone + positional
// variable overrides for the template body.
type recipientInputJSON struct {
	Phone string   `json:"phone"`
	Vars  []string `json:"vars"`
}

type createCampaignRequest struct {
	ChannelID    string     `json:"channel_id" binding:"required"`
	Name         string     `json:"name" binding:"required"`
	TemplateName string     `json:"template_name" binding:"required"`
	TemplateLang string     `json:"template_lang"`
	TemplateVars []string   `json:"template_vars"`
	ScheduledAt  *time.Time `json:"scheduled_at"`
	// One of these must be provided. If both are set, RecipientsWithVars
	// wins (simple Recipients is ignored).
	Recipients         []string             `json:"recipients"`
	RecipientsWithVars []recipientInputJSON `json:"recipients_with_vars"`
}

// HandleCreateCampaign godoc
// @Summary  Create a broadcast campaign
// @Tags     campaigns
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body  body  createCampaignRequest  true  "Campaign payload"
// @Success  201  {object}  domain.Campaign
// @Router   /campaigns [post]
func (h *CampaignHandler) HandleCreateCampaign(c *gin.Context) {
	accountID := c.GetString("accountID")
	userID := c.GetString("userID")

	var req createCampaignRequest
	if !bindJSON(c, &req) {
		return
	}
	// Cap recipients per call to keep request payload bounded.
	total := len(req.Recipients) + len(req.RecipientsWithVars)
	if total == 0 {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code: "NO_RECIPIENTS", Message: "recipients or recipients_with_vars is required",
		})
		return
	}
	if total > 50_000 {
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code: "TOO_MANY_RECIPIENTS", Message: "max 50000 recipients per campaign",
		})
		return
	}

	// Convert JSON recipients to usecase shape.
	recsWithVars := make([]usecases.RecipientInput, len(req.RecipientsWithVars))
	for i, r := range req.RecipientsWithVars {
		recsWithVars[i] = usecases.RecipientInput{Phone: r.Phone, Vars: r.Vars}
	}

	camp, err := h.createUC.Execute(c.Request.Context(), usecases.CreateCampaignDTO{
		AccountID:          accountID,
		ChannelID:          req.ChannelID,
		Name:               req.Name,
		TemplateName:       req.TemplateName,
		TemplateLang:       req.TemplateLang,
		TemplateVars:       req.TemplateVars,
		ScheduledAt:        req.ScheduledAt,
		Recipients:         req.Recipients,
		RecipientsWithVars: recsWithVars,
		CreatedBy:          userID,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, camp)
}

// HandleListCampaigns godoc
// @Summary  List campaigns for the current account
// @Tags     campaigns
// @Produce  json
// @Security BearerAuth
// @Param    limit   query  int  false  "max items (default 50, max 200)"
// @Param    offset  query  int  false  "offset (default 0)"
// @Success  200  {array}  domain.Campaign
// @Router   /campaigns [get]
func (h *CampaignHandler) HandleListCampaigns(c *gin.Context) {
	accountID := c.GetString("accountID")
	limit, offset := pagination(c)

	out, err := h.listUC.Execute(c.Request.Context(), accountID, int32(limit), int32(offset))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// HandleGetCampaign godoc
// @Summary  Get a single campaign by id
// @Tags     campaigns
// @Produce  json
// @Security BearerAuth
// @Param    id  path  string  true  "Campaign ID"
// @Success  200  {object}  domain.Campaign
// @Router   /campaigns/{id} [get]
func (h *CampaignHandler) HandleGetCampaign(c *gin.Context) {
	accountID := c.GetString("accountID")
	camp, err := h.getUC.Execute(c.Request.Context(), accountID, c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, camp)
}

// HandleListRecipients godoc
// @Summary  List recipients of a campaign with their delivery status
// @Tags     campaigns
// @Produce  json
// @Security BearerAuth
// @Param    id      path   string  true   "Campaign ID"
// @Param    limit   query  int     false  "max items (default 200)"
// @Param    offset  query  int     false  "offset (default 0)"
// @Success  200  {array}  domain.CampaignRecipient
// @Router   /campaigns/{id}/recipients [get]
func (h *CampaignHandler) HandleListRecipients(c *gin.Context) {
	accountID := c.GetString("accountID")
	id := c.Param("id")

	// Authorize via getUC (resource ownership check).
	if _, err := h.getUC.Execute(c.Request.Context(), accountID, id); err != nil {
		respondError(c, err)
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	out, err := h.repo.ListRecipients(c.Request.Context(), id, int32(limit), int32(offset))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// HandleCancelCampaign godoc
// @Summary  Cancel a draft/scheduled/sending campaign
// @Tags     campaigns
// @Security BearerAuth
// @Param    id  path  string  true  "Campaign ID"
// @Success  204
// @Router   /campaigns/{id}/cancel [post]
func (h *CampaignHandler) HandleCancelCampaign(c *gin.Context) {
	accountID := c.GetString("accountID")
	if err := h.cancelUC.Execute(c.Request.Context(), accountID, c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// HandleDeleteCampaign godoc
// @Summary  Delete a campaign
// @Tags     campaigns
// @Security BearerAuth
// @Param    id  path  string  true  "Campaign ID"
// @Success  204
// @Router   /campaigns/{id} [delete]
func (h *CampaignHandler) HandleDeleteCampaign(c *gin.Context) {
	accountID := c.GetString("accountID")
	if err := h.deleteUC.Execute(c.Request.Context(), accountID, c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

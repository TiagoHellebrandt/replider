package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// EmailTemplateHandler exposes the template library.
type EmailTemplateHandler struct {
	repo domain.EmailTemplateRepository
}

func NewEmailTemplateHandler(repo domain.EmailTemplateRepository) *EmailTemplateHandler {
	return &EmailTemplateHandler{repo: repo}
}

// templateResponse carries the derived facts the editor needs, so the client
// does not re-implement variable extraction and get a different answer.
type templateResponse struct {
	*domain.EmailTemplate
	Variables []string `json:"variables"`
	WordCount int      `json:"word_count"`
	// TooLong flags a body past the point where a cold email stops being
	// read. A warning, not a limit — the person writing knows their market.
	TooLong bool                        `json:"too_long"`
	Stats   []domain.EmailTemplateStats `json:"stats,omitempty"`
}

func decorate(t *domain.EmailTemplate, stats []domain.EmailTemplateStats) templateResponse {
	words := domain.WordCount(t.Body)
	return templateResponse{
		EmailTemplate: t,
		Variables:     domain.TemplateVariables(t.Subject, t.Body),
		WordCount:     words,
		TooLong:       words > domain.ColdEmailMaxWords,
		Stats:         stats,
	}
}

// HandleList serves GET /v1/email-templates.
//
// @Summary      List email templates
// @Tags         email
// @Produce      json
// @Success      200 {array} templateResponse
// @Security     BearerAuth
// @Router       /email-templates [get]
func (h *EmailTemplateHandler) HandleList(c *gin.Context) {
	items, err := h.repo.List(c.Request.Context(), getAccountID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]templateResponse, 0, len(items))
	for _, t := range items {
		out = append(out, decorate(t, nil))
	}
	c.JSON(http.StatusOK, out)
}

// HandleGet serves GET /v1/email-templates/:id, with its per-version numbers.
//
// @Summary      Read one template and its performance
// @Tags         email
// @Produce      json
// @Param        id path string true "Template ID"
// @Success      200 {object} templateResponse
// @Failure      404 {object} domain.AppError
// @Security     BearerAuth
// @Router       /email-templates/{id} [get]
func (h *EmailTemplateHandler) HandleGet(c *gin.Context) {
	t, err := h.repo.Get(c.Request.Context(), c.Param("id"), getAccountID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	if t == nil {
		respondError(c, domain.ErrTemplateNotFound)
		return
	}
	stats, err := h.repo.Stats(c.Request.Context(), t.ID)
	if err != nil {
		// The template is the content; losing its numbers must not lose it.
		stats = nil
	}
	c.JSON(http.StatusOK, decorate(t, stats))
}

type emailTemplateRequest struct {
	Name     string  `json:"name" binding:"required"`
	Subject  string  `json:"subject"`
	Body     string  `json:"body"`
	FunnelID *string `json:"funnel_id"`
	UserID   *string `json:"user_id"`
}

// HandleUpsert serves PUT /v1/email-templates/:id (id "new" to create).
//
// @Summary      Create or update an email template
// @Tags         email
// @Accept       json
// @Produce      json
// @Param        id path string true "Template ID, or 'new'"
// @Param        request body emailTemplateRequest true "Template"
// @Success      200 {object} templateResponse
// @Security     BearerAuth
// @Router       /email-templates/{id} [put]
func (h *EmailTemplateHandler) HandleUpsert(c *gin.Context) {
	var req emailTemplateRequest
	if !bindJSON(c, &req) {
		return
	}
	accountID := getAccountID(c)

	id := c.Param("id")
	if id == "" || id == "new" {
		id = ulid.Make().String()
	} else {
		// Refuse to write over a template belonging to another account by
		// resolving it first: the upsert is keyed by id alone, so without
		// this a guessed id would create a row under the wrong owner.
		existing, err := h.repo.Get(c.Request.Context(), id, accountID)
		if err != nil {
			respondError(c, err)
			return
		}
		if existing == nil {
			respondError(c, domain.ErrTemplateNotFound)
			return
		}
	}

	t := &domain.EmailTemplate{
		ID:        id,
		AccountID: accountID,
		Name:      strings.TrimSpace(req.Name),
		Subject:   strings.TrimSpace(req.Subject),
		Body:      req.Body,
		FunnelID:  req.FunnelID,
		UserID:    req.UserID,
		IsActive:  true,
	}
	if err := h.repo.Upsert(c.Request.Context(), t); err != nil {
		respondError(c, err)
		return
	}
	saved, err := h.repo.Get(c.Request.Context(), id, accountID)
	if err != nil || saved == nil {
		c.JSON(http.StatusOK, decorate(t, nil))
		return
	}
	c.JSON(http.StatusOK, decorate(saved, nil))
}

// HandleDelete serves DELETE /v1/email-templates/:id.
//
// Soft: the stats reference the template, and a hard delete would erase the
// evidence for every campaign that used it.
//
// @Summary      Retire an email template
// @Tags         email
// @Param        id path string true "Template ID"
// @Success      204
// @Security     BearerAuth
// @Router       /email-templates/{id} [delete]
func (h *EmailTemplateHandler) HandleDelete(c *gin.Context) {
	if err := h.repo.Delete(c.Request.Context(), c.Param("id"), getAccountID(c)); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

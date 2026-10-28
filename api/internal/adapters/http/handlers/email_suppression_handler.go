package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// EmailSuppressionHandler exposes the do-not-send list.
//
// It exists because a silent block is worse than no block: a salesperson
// whose message simply never goes out, with no list to consult and no way to
// undo it, learns to distrust the whole tool. So the addresses are visible,
// each carries the remote server's own words, and any of them can be
// released — a suppression can be wrong (an address is fixed, a mailbox is
// emptied, a person changes their mind).
type EmailSuppressionHandler struct {
	repo domain.EmailSuppressionRepository
}

func NewEmailSuppressionHandler(repo domain.EmailSuppressionRepository) *EmailSuppressionHandler {
	return &EmailSuppressionHandler{repo: repo}
}

// HandleList serves GET /v1/email-suppressions.
//
// @Summary  List suppressed email addresses
// @Tags     email
// @Produce  json
// @Success  200 {array} domain.EmailSuppression
// @Failure  500 {object} domain.AppError
// @Router   /email-suppressions [get]
func (h *EmailSuppressionHandler) HandleList(c *gin.Context) {
	limit, offset := pagination(c)
	list, err := h.repo.List(c.Request.Context(), getAccountID(c), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	if list == nil {
		list = []*domain.EmailSuppression{}
	}
	c.JSON(http.StatusOK, list)
}

// HandleRelease serves DELETE /v1/email-suppressions/:address.
//
// Idempotent: releasing an address that is not on the list is a success, not
// a 404. The caller's intent — "this address should be writable" — is
// satisfied either way, and a 404 would only invite a retry loop.
//
// @Summary  Release a suppressed email address
// @Tags     email
// @Param    address path string true "Email address"
// @Success  204
// @Failure  400 {object} domain.AppError
// @Router   /email-suppressions/{address} [delete]
func (h *EmailSuppressionHandler) HandleRelease(c *gin.Context) {
	address := strings.TrimSpace(c.Param("address"))
	if address == "" {
		respondError(c, &domain.AppError{
			Code: "VALIDATION_ERROR", Message: "endereço é obrigatório",
		})
		return
	}
	if err := h.repo.Release(c.Request.Context(), getAccountID(c), address); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

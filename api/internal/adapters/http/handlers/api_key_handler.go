package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Machine API keys (§6.19, J1).

// APIKeyHandler manages the account's machine credentials.
type APIKeyHandler struct {
	uc *usecases.APIKeyUseCase
}

func NewAPIKeyHandler(uc *usecases.APIKeyUseCase) *APIKeyHandler {
	return &APIKeyHandler{uc: uc}
}

type createAPIKeyRequest struct {
	Name          string   `json:"name"`
	Scopes        []string `json:"scopes" binding:"required"`
	ExpiresInDays *int     `json:"expires_in_days,omitempty"`
}

// HandleCreate serves POST /v1/api-keys.
//
// @Summary     Mint a machine API key
// @Description Creates an account-level key with explicit scopes. The plaintext key is returned ONCE — only its hash is stored, so it cannot be shown again.
// @Tags        API Keys
// @Accept      json
// @Produce     json
// @Param       request body createAPIKeyRequest true "Name and scopes"
// @Success     201 {object} usecases.CreateAPIKeyResponse
// @Failure     400 {object} domain.AppError
// @Failure     403 {object} domain.AppError
// @Security    BearerAuth
// @Router      /api-keys [post]
func (h *APIKeyHandler) HandleCreate(c *gin.Context) {
	var req createAPIKeyRequest
	if !bindJSON(c, &req) {
		return
	}
	created, err := h.uc.Create(c.Request.Context(), usecases.CreateAPIKeyDTO{
		AccountID: getAccountID(c),
		// Who minted it, from the session — never from the body.
		CreatedByUserID: getUserID(c),
		Name:            req.Name,
		Scopes:          req.Scopes,
		ExpiresInDays:   req.ExpiresInDays,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	// 201 with the secret in the body, and this is the only time it exists
	// outside the caller's hands.
	c.JSON(http.StatusCreated, created)
}

// HandleList serves GET /v1/api-keys.
//
// @Summary     List machine API keys
// @Description The account's live keys — prefix, scopes and last use, never the key itself.
// @Tags        API Keys
// @Produce     json
// @Success     200 {array} domain.APIKeyInfo
// @Security    BearerAuth
// @Router      /api-keys [get]
func (h *APIKeyHandler) HandleList(c *gin.Context) {
	keys, err := h.uc.List(c.Request.Context(), getAccountID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, keys)
}

// HandleRevoke serves DELETE /v1/api-keys/:id.
//
// @Summary     Revoke a machine API key
// @Description Kills the key immediately. 404 when nothing matched — somebody revoking a leaked key needs to know it actually happened.
// @Tags        API Keys
// @Produce     json
// @Param       id path string true "Key ID"
// @Success     204
// @Failure     404 {object} domain.AppError
// @Security    BearerAuth
// @Router      /api-keys/{id} [delete]
func (h *APIKeyHandler) HandleRevoke(c *gin.Context) {
	revoked, err := h.uc.Revoke(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	if !revoked {
		// Never a silent success: a "revoked" that revoked nothing leaves
		// somebody believing a leaked credential is dead.
		c.JSON(http.StatusNotFound, domain.AppError{
			Code: "API_KEY_NOT_FOUND", Message: "chave não encontrada"})
		return
	}
	c.Status(http.StatusNoContent)
}

// HandleListScopes serves GET /v1/api-keys/scopes.
//
// @Summary     Available API key scopes
// @Description Every scope a key may be minted with, and what it grants.
// @Tags        API Keys
// @Produce     json
// @Success     200 {array} object
// @Security    BearerAuth
// @Router      /api-keys/scopes [get]
func (h *APIKeyHandler) HandleListScopes(c *gin.Context) {
	c.JSON(http.StatusOK, h.uc.Scopes())
}

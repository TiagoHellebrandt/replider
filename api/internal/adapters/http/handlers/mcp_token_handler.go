package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// MCPTokenHandler is the dashboard-facing management surface for the
// MCP integration: personal access tokens + active OAuth connections.
type MCPTokenHandler struct {
	patUC *usecases.MCPPatUseCase
}

func NewMCPTokenHandler(patUC *usecases.MCPPatUseCase) *MCPTokenHandler {
	return &MCPTokenHandler{patUC: patUC}
}

type createPATRequest struct {
	Name          string `json:"name"`
	ExpiresInDays *int   `json:"expires_in_days"`
}

// HandleCreatePAT godoc
// @Summary Create a personal access token for the MCP integration
// @Tags mcp
// @Accept json
// @Produce json
// @Success 201 {object} usecases.CreatePATResponse
// @Failure 409 {object} domain.AppError
// @Router /me/mcp-tokens [post]
func (h *MCPTokenHandler) HandleCreatePAT(c *gin.Context) {
	var req createPATRequest
	if !bindJSON(c, &req) {
		return
	}
	resp, err := h.patUC.Create(c.Request.Context(), usecases.CreatePATDTO{
		UserID:        getUserID(c),
		AccountID:     getAccountID(c),
		Name:          req.Name,
		ExpiresInDays: req.ExpiresInDays,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// HandleListPATs godoc
// @Summary List the caller's MCP personal access tokens (metadata only)
// @Tags mcp
// @Produce json
// @Success 200 {array} usecases.PATInfo
// @Router /me/mcp-tokens [get]
func (h *MCPTokenHandler) HandleListPATs(c *gin.Context) {
	infos, err := h.patUC.List(c.Request.Context(), getUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, infos)
}

// HandleRevokePAT godoc
// @Summary Revoke one of the caller's MCP personal access tokens
// @Tags mcp
// @Success 204
// @Failure 404 {object} domain.AppError
// @Router /me/mcp-tokens/{id} [delete]
func (h *MCPTokenHandler) HandleRevokePAT(c *gin.Context) {
	tokenID := c.Param("id")
	if tokenID == "" {
		respondError(c, domain.ErrPATNotFound)
		return
	}
	if err := h.patUC.Revoke(c.Request.Context(), getUserID(c), tokenID); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// HandleListConnections godoc
// @Summary List the caller's active OAuth connections (e.g. Claude.ai)
// @Tags mcp
// @Produce json
// @Success 200 {array} domain.OAuthConnection
// @Router /me/mcp-connections [get]
func (h *MCPTokenHandler) HandleListConnections(c *gin.Context) {
	conns, err := h.patUC.ListConnections(c.Request.Context(), getUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, conns)
}

// HandleRevokeConnection godoc
// @Summary Revoke every token granted to an OAuth client by the caller
// @Tags mcp
// @Success 204
// @Failure 404 {object} domain.AppError
// @Router /me/mcp-connections/{clientId} [delete]
func (h *MCPTokenHandler) HandleRevokeConnection(c *gin.Context) {
	if err := h.patUC.RevokeConnection(c.Request.Context(), getUserID(c), c.Param("clientId")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

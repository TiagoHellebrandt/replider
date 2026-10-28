package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// Objections ride the BrandKitHandler (optional setter, same file split as
// the overlay): the library is company voice — what the business says when
// the customer pushes back — and shares the brand kit's permission shape
// (admins write, salespeople read).

// SetObjectionRepo wires the library (optional; nil → 503 on the routes).
func (h *BrandKitHandler) SetObjectionRepo(r domain.ObjectionRepository) {
	h.objections = r
}

type objectionsResponse struct {
	Items []domain.AnticipatedObjection `json:"items"`
	// IsDefault says the account never saved its own library and these are
	// the vertical seed — the screen shows "padrão do setor, edite à
	// vontade" instead of pretending the user wrote them.
	IsDefault bool `json:"is_default"`
}

// HandleListObjections serves GET /v1/objections.
//
// @Summary      Read the objection library (account's own, or the vertical seed)
// @Tags         brand
// @Produce      json
// @Success      200 {object} objectionsResponse
// @Security     BearerAuth
// @Router       /objections [get]
func (h *BrandKitHandler) HandleListObjections(c *gin.Context) {
	if h.objections == nil {
		respondError(c, &domain.AppError{Code: "OBJECTIONS_UNAVAILABLE", Message: "objection library not configured"})
		return
	}
	rows, err := h.objections.List(getAccountID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	if len(rows) == 0 {
		c.JSON(http.StatusOK, objectionsResponse{Items: domain.DefaultHealthObjections(), IsDefault: true})
		return
	}
	items := make([]domain.AnticipatedObjection, 0, len(rows))
	for _, o := range rows {
		items = append(items, domain.AnticipatedObjection{Label: o.Label, Counter: o.Counter})
	}
	c.JSON(http.StatusOK, objectionsResponse{Items: items})
}

type saveObjectionsRequest struct {
	Items []struct {
		Label   string `json:"label"`
		Counter string `json:"counter"`
	} `json:"items"`
}

// HandleSaveObjections serves PUT /v1/objections (admin).
//
// Whole-list replace, like the brand kit's claim lists. Saving an empty
// list returns the account to the vertical seed — deliberate: "reset to
// default" should not need a second endpoint.
//
// @Summary      Replace the objection library
// @Tags         brand
// @Accept       json
// @Produce      json
// @Success      200 {object} objectionsResponse
// @Failure      400 {object} domain.AppError
// @Security     BearerAuth
// @Router       /objections [put]
func (h *BrandKitHandler) HandleSaveObjections(c *gin.Context) {
	if h.objections == nil {
		respondError(c, &domain.AppError{Code: "OBJECTIONS_UNAVAILABLE", Message: "objection library not configured"})
		return
	}
	var req saveObjectionsRequest
	if !bindJSON(c, &req) {
		return
	}
	items := make([]domain.Objection, 0, len(req.Items))
	for _, it := range req.Items {
		label := strings.TrimSpace(it.Label)
		if label == "" {
			continue // a pushback with no words matches nothing
		}
		items = append(items, domain.Objection{
			Label:   label,
			Counter: strings.TrimSpace(it.Counter),
		})
	}
	if err := h.objections.Replace(getAccountID(c), items); err != nil {
		respondError(c, err)
		return
	}
	h.HandleListObjections(c)
}

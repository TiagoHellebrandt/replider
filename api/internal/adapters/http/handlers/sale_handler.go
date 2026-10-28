package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Sales (§5, F1).
type SaleHandler struct {
	uc   *usecases.SaleUseCase
	repo domain.SaleRepository
	// importer backs POST /sales/import{,/preview}. Optional: a deployment
	// without it simply does not mount the import routes.
	importer *SaleImportHandler
}

func NewSaleHandler(uc *usecases.SaleUseCase, repo domain.SaleRepository) *SaleHandler {
	return &SaleHandler{uc: uc, repo: repo}
}

// SetImport wires the sales-history import. Optional setter so neither the
// constructor nor the router signature changes.
func (h *SaleHandler) SetImport(i *SaleImportHandler) { h.importer = i }

// Import returns the import handler, nil when unwired.
func (h *SaleHandler) Import() *SaleImportHandler { return h.importer }

// saleResponse adds the derived money the client would otherwise recompute
// (and get differently).
type saleResponse struct {
	*domain.Sale
	// NetCents is what the company kept: gross minus cost minus tax. THIS is
	// what a commission should be calculated on — a percentage of gross can
	// quietly exceed the whole margin.
	NetCents int64 `json:"net_cents"`
	// MarginRate is nil when nothing was sold: zero over zero is unknown.
	MarginRate *float64 `json:"margin_rate,omitempty"`
	// Underwater flags a sale that cost more than it brought in. Surfaced,
	// never refused — discounts, loss leaders and typos all look like this,
	// and only a person can tell which.
	Underwater bool `json:"underwater"`
	// NextPurchaseDue is when this customer is likely to need it again. Nil
	// without a cycle: a guess would put a real customer on a follow-up list
	// on a date nobody chose.
	NextPurchaseDue *time.Time `json:"next_purchase_due,omitempty"`
}

func decorateSale(s *domain.Sale) saleResponse {
	return saleResponse{
		Sale: s, NetCents: s.NetCents(), MarginRate: s.MarginRate(),
		Underwater: s.Underwater(), NextPurchaseDue: s.NextPurchaseDue(),
	}
}

type saleRequest struct {
	ContactID    string            `json:"contact_id"`
	LeadID       string            `json:"lead_id"`
	SellerUserID string            `json:"seller_user_id"`
	Items        []domain.SaleItem `json:"items"`
	GrossCents   int64             `json:"gross_cents"`
	CostCents    int64             `json:"cost_cents"`
	TaxCents     int64             `json:"tax_cents"`
	SoldAt       string            `json:"sold_at"`
	// CycleDays is what the repurchase engine runs on. A sale without it is
	// one nothing will ever resurface.
	CycleDays int    `json:"cycle_days"`
	Notes     string `json:"notes"`
	// Source is only honoured as "ai_extracted", and only alongside the
	// confirming user — a client cannot label a sale as something a model
	// found when no model was involved.
	Source string `json:"source"`
}

// HandleCreate serves POST /v1/sales.
//
// @Summary  Record a sale
// @Tags     sales
// @Accept   json
// @Produce  json
// @Success  200 {object} saleResponse
// @Failure  422 {object} saleInvalidResponse
// @Router   /sales [post]
func (h *SaleHandler) HandleCreate(c *gin.Context) {
	var req saleRequest
	if !bindJSON(c, &req) {
		return
	}

	sale := &domain.Sale{
		ContactID: req.ContactID, LeadID: req.LeadID,
		SellerUserID: req.SellerUserID, Items: req.Items,
		GrossCents: req.GrossCents, CostCents: req.CostCents,
		TaxCents: req.TaxCents, CycleDays: req.CycleDays, Notes: req.Notes,
		Source: domain.SaleManual,
	}
	if req.Source == string(domain.SaleAIExtracted) {
		// Recorded as extracted AND as confirmed by this person: the model
		// proposed it, they said yes, and both halves belong in the record.
		sale.Source = domain.SaleAIExtracted
		sale.ConfirmedByUserID = getUserID(c)
	}
	if req.SoldAt != "" {
		// RFC3339 from the app, a bare date from an API client backfilling
		// history — the same layouts the import reads.
		if parsed, ok := usecases.ParseSaleDate(req.SoldAt); ok {
			sale.SoldAt = parsed
		}
	}

	saved, err := h.uc.Register(c.Request.Context(), getAccountID(c), sale)
	if err != nil {
		var invalid *usecases.ErrSaleInvalid
		if errors.As(err, &invalid) {
			// 422: the request is well-formed and the sale is not, and the
			// form renders the problems together.
			c.JSON(http.StatusUnprocessableEntity, saleInvalidResponse{
				Code: "SALE_INVALID", Problems: invalid.Problems,
			})
			return
		}
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, decorateSale(saved))
}

type saleInvalidResponse struct {
	Code     string   `json:"code"`
	Problems []string `json:"problems"`
}

// HandleList serves GET /v1/sales.
//
// @Summary  List sales
// @Tags     sales
// @Produce  json
// @Success  200 {array} saleResponse
// @Router   /sales [get]
func (h *SaleHandler) HandleList(c *gin.Context) {
	limit, offset := pagination(c)
	list, err := h.uc.List(c.Request.Context(), getAccountID(c), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]saleResponse, 0, len(list))
	for _, s := range list {
		out = append(out, decorateSale(s))
	}
	c.JSON(http.StatusOK, out)
}

// HandleListByContact serves GET /v1/contacts/:id/sales — the portfolio view.
//
// @Summary  Everything one customer bought
// @Tags     sales
// @Produce  json
// @Param    id path string true "Contact ID"
// @Success  200 {array} saleResponse
// @Router   /contacts/{id}/sales [get]
func (h *SaleHandler) HandleListByContact(c *gin.Context) {
	list, err := h.repo.ListByContact(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]saleResponse, 0, len(list))
	for _, s := range list {
		out = append(out, decorateSale(s))
	}
	c.JSON(http.StatusOK, out)
}

// HandleDelete serves DELETE /v1/sales/:id.
//
// @Summary  Delete a sale
// @Tags     sales
// @Param    id path string true "Sale ID"
// @Success  204
// @Router   /sales/{id} [delete]
func (h *SaleHandler) HandleDelete(c *gin.Context) {
	if err := h.repo.Delete(c.Request.Context(), c.Param("id"), getAccountID(c)); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// HandleExtract serves POST /v1/conversations/:id/extract-sale.
//
// Reads the conversation and PROPOSES a sale. Records nothing: a model that
// files sales on its own turns a misread message into a commission somebody
// is owed.
//
// @Summary  Read a sale out of a conversation, for confirmation
// @Tags     sales
// @Produce  json
// @Param    id path string true "Conversation ID"
// @Success  200 {object} domain.SaleExtraction
// @Failure  400 {object} domain.AppError
// @Router   /conversations/{id}/extract-sale [post]
func (h *SaleHandler) HandleExtract(c *gin.Context) {
	out, err := h.uc.Extract(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// Portfolio and repurchase (§6.5, F2/F3).
//
// The customer view of sales: who bought, when they will need it again, and
// how long since anybody spoke to them.
type PortfolioHandler struct {
	repo     domain.PortfolioRepository
	saleRepo domain.SaleRepository
	// scoped answers the reorder question for ONE account. Optional: without
	// it the screen falls back to the cross-account query, which is correct
	// but pages badly for a busy deployment.
	scoped   ScopedReorderReader
	products domain.ProductRepository
	cycles   ProductCycleWriter
}

// ScopedReorderReader answers "what is about to run out" for one account.
type ScopedReorderReader interface {
	DueForReorderByAccount(ctx context.Context, accountID string, within time.Time, limit int) ([]*domain.ReorderCandidate, error)
}

// SetScopedReorders wires the per-account reorder query.
func (h *PortfolioHandler) SetScopedReorders(r ScopedReorderReader) { h.scoped = r }

// ProductCycleWriter sets a product's repurchase cycles. A narrow port so the
// handler does not depend on the whole product repository for two integers.
type ProductCycleWriter interface {
	SetCycles(ctx context.Context, id, accountID string, consumptionDays, shelfLifeDays int) error
}

func NewPortfolioHandler(repo domain.PortfolioRepository, sales domain.SaleRepository) *PortfolioHandler {
	return &PortfolioHandler{repo: repo, saleRepo: sales}
}

// SetProductCycles wires the cycle writer.
func (h *PortfolioHandler) SetProductCycles(p domain.ProductRepository, w ProductCycleWriter) {
	h.products, h.cycles = p, w
}

// portfolioResponse adds the derived numbers the card is built around.
type portfolioResponse struct {
	*domain.PortfolioCustomer
	// DaysUntilDue is the countdown. NEGATIVE when overdue, and deliberately
	// not clamped: "acabou há 12 dias" is the most actionable thing on the
	// screen, and zero would hide it.
	DaysUntilDue *int `json:"days_until_due,omitempty"`
	// DaysSinceContact is the relationship's pulse. Nil when nobody ever
	// spoke to them, which is different from "spoke long ago".
	DaysSinceContact *int `json:"days_since_contact,omitempty"`
	Urgency          int  `json:"urgency"`
}

func decoratePortfolio(c *domain.PortfolioCustomer, now time.Time) portfolioResponse {
	return portfolioResponse{
		PortfolioCustomer: c,
		DaysUntilDue:      c.DaysUntilDue(now),
		DaysSinceContact:  c.DaysSinceContact(now),
		Urgency:           c.PortfolioUrgency(now),
	}
}

// HandleList serves GET /v1/portfolio.
//
// @Summary  The account's customers (bought or imported), most urgent first
// @Tags     portfolio
// @Produce  json
// @Success  200 {array} portfolioResponse
// @Router   /portfolio [get]
func (h *PortfolioHandler) HandleList(c *gin.Context) {
	limit, offset := pagination(c)
	list, err := h.repo.List(c.Request.Context(), getAccountID(c), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	now := time.Now()
	out := make([]portfolioResponse, 0, len(list))
	for _, customer := range list {
		out = append(out, decoratePortfolio(customer, now))
	}
	c.JSON(http.StatusOK, out)
}

// portfolioDetail is one customer plus what they actually bought.
type portfolioDetail struct {
	portfolioResponse
	Sales []*domain.Sale `json:"sales"`
}

// HandleGet serves GET /v1/portfolio/:id.
//
// @Summary  One customer's history and predictions
// @Tags     portfolio
// @Produce  json
// @Param    id path string true "Contact ID"
// @Success  200 {object} portfolioDetail
// @Failure  404 {object} domain.AppError
// @Router   /portfolio/{id} [get]
func (h *PortfolioHandler) HandleGet(c *gin.Context) {
	accountID := getAccountID(c)
	customer, err := h.repo.Get(c.Request.Context(), accountID, c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	if customer == nil {
		respondError(c, &domain.AppError{
			Code: "PORTFOLIO_CUSTOMER_NOT_FOUND", Message: "cliente sem compras registradas"})
		return
	}

	detail := portfolioDetail{
		portfolioResponse: decoratePortfolio(customer, time.Now()),
		Sales:             []*domain.Sale{},
	}
	if h.saleRepo != nil {
		if sales, err := h.saleRepo.ListByContact(c.Request.Context(), accountID, customer.ContactID); err == nil {
			detail.Sales = sales
		}
	}
	c.JSON(http.StatusOK, detail)
}

type productCycleRequest struct {
	// ConsumptionCycleDays is how long a customer takes to go through one
	// order. The fallback when a sale did not record its own cycle — which
	// is most sales.
	ConsumptionCycleDays int `json:"consumption_cycle_days"`
	// ShelfLifeDays is a DIFFERENT number: gauze expires whether or not
	// anyone uses it.
	ShelfLifeDays int `json:"shelf_life_days"`
}

// maxCycleDays bounds both. Ten years: anything longer is a typo, and a cycle
// of 99999 days silently removes a product from the reorder engine forever.
const maxCycleDays = 3650

// HandleSetProductCycles serves PUT /v1/products/:id/cycles.
//
// @Summary  Set a product's repurchase and shelf-life cycles
// @Tags     portfolio
// @Accept   json
// @Param    id path string true "Product ID"
// @Success  204
// @Failure  400 {object} domain.AppError
// @Router   /products/{id}/cycles [put]
func (h *PortfolioHandler) HandleSetProductCycles(c *gin.Context) {
	if h.cycles == nil || h.products == nil {
		respondError(c, &domain.AppError{
			Code: "PRODUCT_CYCLES_UNAVAILABLE", Message: "ciclos de produto não configurados"})
		return
	}
	var req productCycleRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.ConsumptionCycleDays < 0 || req.ConsumptionCycleDays > maxCycleDays ||
		req.ShelfLifeDays < 0 || req.ShelfLifeDays > maxCycleDays {
		respondError(c, &domain.AppError{
			Code: "VALIDATION_ERROR", Message: "ciclo inválido"})
		return
	}

	accountID := getAccountID(c)
	// Ownership first: SetCycles is scoped by account, but a 404 here is a
	// clearer answer than a silent no-op update.
	product, err := h.products.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil || product == nil || product.AccountID != accountID {
		respondError(c, &domain.AppError{
			Code: "PRODUCT_NOT_FOUND", Message: "produto não encontrado"})
		return
	}

	if err := h.cycles.SetCycles(c.Request.Context(), product.ID, accountID,
		req.ConsumptionCycleDays, req.ShelfLifeDays); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// HandleUpcomingReorders serves GET /v1/portfolio/reorders.
//
// What is about to run out, across the account. Read-only: the worker is what
// puts suggestions in the approval queue, and a screen that could trigger
// sends would be a second, weaker path to the same decision.
//
// @Summary  Predicted reorders
// @Tags     portfolio
// @Produce  json
// @Success  200 {array} domain.ReorderCandidate
// @Router   /portfolio/reorders [get]
func (h *PortfolioHandler) HandleUpcomingReorders(c *gin.Context) {
	horizon := time.Now().Add(domain.ReorderLeadTimeDays * 24 * time.Hour)
	limit, _ := pagination(c)

	// Scoped in SQL. The worker's query spans accounts because it fans out;
	// filtering that page in Go would show this account nothing whenever its
	// customers ranked past the limit.
	if h.scoped == nil {
		respondError(c, &domain.AppError{
			Code: "PORTFOLIO_UNAVAILABLE", Message: "previsões de recompra indisponíveis"})
		return
	}
	out, err := h.scoped.DueForReorderByAccount(c.Request.Context(),
		getAccountID(c), horizon, limit)
	if err != nil {
		respondError(c, err)
		return
	}
	if out == nil {
		out = []*domain.ReorderCandidate{}
	}
	c.JSON(http.StatusOK, out)
}

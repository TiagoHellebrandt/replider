package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Client-facing commission rules (§6.6, G1).
//
// Admin-only, all of it. A commission rule is the pay policy of a company:
// reading somebody else's rate is reading their salary, and writing one is
// changing it.
type CommissionRuleHandler struct {
	uc *usecases.CommissionRuleUseCase
}

func NewCommissionRuleHandler(uc *usecases.CommissionRuleUseCase) *CommissionRuleHandler {
	return &CommissionRuleHandler{uc: uc}
}

// commissionRuleRequest is the structured builder's payload.
//
// Fields mirror the domain rather than accepting a formula string: the shape
// of what a rule can do is fixed, and that is what makes the simulation below
// honest instead of a second implementation of the same arithmetic.
type commissionRuleRequest struct {
	ID    string                     `json:"id"`
	Name  string                     `json:"name"`
	Scope domain.CommissionScopeKind `json:"scope"`
	// ScopeID is the seller/team/product this rule points at. Empty for the
	// global fallback.
	ScopeID    string                      `json:"scope_id"`
	Base       domain.CommissionBaseKind   `json:"base"`
	Deductions domain.CommissionDeductions `json:"deductions"`

	RatePercent float64                      `json:"rate_percent"`
	Tiers       []domain.CommissionTier      `json:"tiers"`
	Milestones  []domain.CommissionMilestone `json:"milestones"`
	Splits      []domain.CommissionSplit     `json:"splits"`
	// RepurchaseRatePercent is a POINTER on purpose: absent means "same as a
	// new sale", zero means "we pay nothing on repurchase", and collapsing
	// the two would silently stop paying somebody.
	RepurchaseRatePercent *float64 `json:"repurchase_rate_percent"`

	// Active defaults to true on create — a rule nobody asked to disable is
	// a rule they meant to use.
	Active *bool `json:"active"`
}

func (req commissionRuleRequest) toDomain() *domain.CommissionRule {
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	return &domain.CommissionRule{
		ID:                    req.ID,
		Name:                  req.Name,
		Scope:                 req.Scope,
		ScopeID:               req.ScopeID,
		Base:                  req.Base,
		Deductions:            req.Deductions,
		RatePercent:           req.RatePercent,
		Tiers:                 req.Tiers,
		Milestones:            req.Milestones,
		Splits:                req.Splits,
		RepurchaseRatePercent: req.RepurchaseRatePercent,
		Active:                active,
	}
}

// HandleList serves GET /v1/commission-rules.
//
// @Summary  List the account's commission rules
// @Tags     commissions
// @Produce  json
// @Success  200 {array} domain.CommissionRule
// @Router   /commission-rules [get]
func (h *CommissionRuleHandler) HandleList(c *gin.Context) {
	rules, err := h.uc.List(c.Request.Context(), getAccountID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, rules)
}

// HandleGet serves GET /v1/commission-rules/:id.
//
// @Summary  One commission rule
// @Tags     commissions
// @Param    id path string true "Rule ID"
// @Produce  json
// @Success  200 {object} domain.CommissionRule
// @Failure  404 {object} domain.AppError
// @Router   /commission-rules/{id} [get]
func (h *CommissionRuleHandler) HandleGet(c *gin.Context) {
	rule, err := h.uc.Get(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, rule)
}

// HandleSave serves POST /v1/commission-rules and PUT /v1/commission-rules/:id.
//
// @Summary  Create or update a commission rule
// @Tags     commissions
// @Accept   json
// @Produce  json
// @Success  200 {object} domain.CommissionRule
// @Failure  400 {object} domain.AppError
// @Failure  409 {object} domain.AppError
// @Router   /commission-rules [post]
func (h *CommissionRuleHandler) HandleSave(c *gin.Context) {
	var req commissionRuleRequest
	if !bindJSON(c, &req) {
		return
	}
	// The path wins over the body on an update: a PUT to one id carrying
	// another id in the body is either a client bug or somebody probing, and
	// honouring the body would let the second case rewrite a different rule.
	if id := c.Param("id"); id != "" {
		req.ID = id
	}

	rule, err := h.uc.Save(c.Request.Context(), getAccountID(c), req.toDomain())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, rule)
}

// HandleDelete serves DELETE /v1/commission-rules/:id.
//
// @Summary  Delete a commission rule
// @Tags     commissions
// @Param    id path string true "Rule ID"
// @Success  204
// @Router   /commission-rules/{id} [delete]
func (h *CommissionRuleHandler) HandleDelete(c *gin.Context) {
	if err := h.uc.Delete(c.Request.Context(), getAccountID(c), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// commissionSimulationRequest is a rule that has not been saved plus the sale
// to try it on.
type commissionSimulationRequest struct {
	Rule commissionRuleRequest `json:"rule"`
	// GrossCents is the typical sale. In CENTS, like every other money field
	// on the wire.
	GrossCents int64 `json:"gross_cents"`
	// MarginPercent is the typical gross margin. Zero falls back to the
	// distributor default rather than flattering the rule with 100%.
	MarginPercent float64 `json:"margin_percent"`
	// SalesPerMonth is how the ladder and the milestone get exercised: a
	// tier nobody reaches is a tier nobody can check.
	SalesPerMonth int `json:"sales_per_month"`
}

// HandleSimulate serves POST /v1/commission-rules/simulate.
//
// Runs the same engine the statement runs, on a rule that may not exist yet —
// which is the point: "com venda média de R$10k, o vendedor ganha R$X" has to
// be the number that will actually be paid, or it is worse than no answer.
//
// @Summary  Simulate a commission rule before saving it
// @Tags     commissions
// @Accept   json
// @Produce  json
// @Success  200 {object} domain.CommissionSimulation
// @Failure  400 {object} domain.AppError
// @Router   /commission-rules/simulate [post]
func (h *CommissionRuleHandler) HandleSimulate(c *gin.Context) {
	var req commissionSimulationRequest
	if !bindJSON(c, &req) {
		return
	}
	sim, err := h.uc.Simulate(c.Request.Context(), getAccountID(c), req.Rule.toDomain(),
		req.GrossCents, req.MarginPercent, req.SalesPerMonth)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, sim)
}

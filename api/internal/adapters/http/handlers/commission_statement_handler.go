package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// The monthly commission statement (§6.6, G2).
//
// Admin-only, like the rules: this is everybody's pay in one screen. The
// salesperson's own number is a different, narrower endpoint (G3).
type CommissionStatementHandler struct {
	uc *usecases.CommissionStatementUseCase
}

func NewCommissionStatementHandler(uc *usecases.CommissionStatementUseCase) *CommissionStatementHandler {
	return &CommissionStatementHandler{uc: uc}
}

// cycleParam resolves ?cycle=YYYY-MM, defaulting to the account's current
// month in the ACCOUNT's timezone.
func (h *CommissionStatementHandler) cycleParam(c *gin.Context) string {
	if cycle := strings.TrimSpace(c.Query("cycle")); cycle != "" {
		return cycle
	}
	return h.uc.CurrentCycle(c.Request.Context(), getAccountID(c))
}

type statementListResponse struct {
	Cycle string `json:"cycle"`
	// Statements is one per salesperson, most owed first — the order an admin
	// reviews in.
	Statements []*domain.CommissionStatement `json:"statements"`
	// TotalPendingCents is what closing the whole month would cost.
	TotalPendingCents int64  `json:"total_pending_cents"`
	TotalPaidCents    int64  `json:"total_paid_cents"`
	Currency          string `json:"currency"`
}

func buildStatementList(cycle string, statements []*domain.CommissionStatement) statementListResponse {
	out := statementListResponse{
		Cycle: cycle, Statements: statements,
		// Commission figures are BRL: the rules are written in reais and the
		// payment leaves a Brazilian bank.
		Currency: domain.CommissionLedgerCurrency,
	}
	if statements == nil {
		out.Statements = []*domain.CommissionStatement{}
	}
	for _, st := range statements {
		out.TotalPendingCents += st.PendingCents
		out.TotalPaidCents += st.PaidCents
	}
	return out
}

// HandleList serves GET /v1/commissions/statements.
//
// @Summary  Commission statements for a cycle
// @Tags     commissions
// @Param    cycle query string false "Cycle (YYYY-MM), defaults to the current month"
// @Param    seller_user_id query string false "One salesperson only"
// @Produce  json
// @Success  200 {object} statementListResponse
// @Failure  400 {object} domain.AppError
// @Router   /commissions/statements [get]
func (h *CommissionStatementHandler) HandleList(c *gin.Context) {
	cycle := h.cycleParam(c)
	statements, err := h.uc.List(c.Request.Context(), getAccountID(c), cycle,
		strings.TrimSpace(c.Query("seller_user_id")))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, buildStatementList(cycle, statements))
}

// HandleRecompute serves POST /v1/commissions/statements/recompute.
//
// Idempotent, and deliberately explicit rather than automatic on every sale: a
// statement that silently changes while somebody is reading it is a statement
// they stop trusting. Paid lines are never touched — a change after payment
// comes back as an adjustment.
//
// @Summary  Recompute a cycle from the sales and the rules
// @Tags     commissions
// @Param    cycle query string false "Cycle (YYYY-MM), defaults to the current month"
// @Produce  json
// @Success  200 {object} statementListResponse
// @Failure  400 {object} domain.AppError
// @Router   /commissions/statements/recompute [post]
func (h *CommissionStatementHandler) HandleRecompute(c *gin.Context) {
	cycle := h.cycleParam(c)
	statements, err := h.uc.Recompute(c.Request.Context(), getAccountID(c), cycle)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, buildStatementList(cycle, statements))
}

type adjustRequest struct {
	Cycle        string `json:"cycle"`
	SellerUserID string `json:"seller_user_id"`
	// AmountCents may be negative: a clawback is a real event, and a statement
	// that cannot express one stops matching the bank.
	AmountCents int64 `json:"amount_cents"`
	// Note is required by the use case — an adjustment nobody explained is a
	// number nobody can defend next month.
	Note string `json:"note"`
}

// HandleAdjust serves POST /v1/commissions/statements/adjust.
//
// @Summary  Add a manual adjustment to a statement
// @Tags     commissions
// @Accept   json
// @Produce  json
// @Success  200 {object} domain.CommissionLine
// @Failure  400 {object} domain.AppError
// @Router   /commissions/statements/adjust [post]
func (h *CommissionStatementHandler) HandleAdjust(c *gin.Context) {
	var req adjustRequest
	if !bindJSON(c, &req) {
		return
	}
	cycle := strings.TrimSpace(req.Cycle)
	if cycle == "" {
		cycle = h.uc.CurrentCycle(c.Request.Context(), getAccountID(c))
	}
	entry, err := h.uc.Adjust(c.Request.Context(), getAccountID(c), cycle,
		req.SellerUserID, req.AmountCents, req.Note)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, entry)
}

type markPaidRequest struct {
	Cycle        string `json:"cycle"`
	SellerUserID string `json:"seller_user_id"`
	Note         string `json:"note"`
	// AllowOpenCycle is the second confirmation for advancing commission on a
	// month that has not ended. Sales landing after the transfer will produce
	// new lines, so it must be a decision rather than a default.
	AllowOpenCycle bool `json:"allow_open_cycle"`
}

// HandleMarkPaid serves POST /v1/commissions/statements/pay.
//
// The transfer happens outside the system (PIX/TED). This records that it
// happened, which is what makes every later recomputation refuse to rewrite it.
//
// @Summary  Mark a salesperson's cycle as paid
// @Tags     commissions
// @Accept   json
// @Produce  json
// @Success  200 {object} domain.CommissionStatement
// @Failure  400 {object} domain.AppError
// @Failure  409 {object} domain.AppError
// @Router   /commissions/statements/pay [post]
func (h *CommissionStatementHandler) HandleMarkPaid(c *gin.Context) {
	var req markPaidRequest
	if !bindJSON(c, &req) {
		return
	}
	cycle := strings.TrimSpace(req.Cycle)
	if cycle == "" {
		cycle = h.uc.CurrentCycle(c.Request.Context(), getAccountID(c))
	}
	statement, err := h.uc.MarkPaid(c.Request.Context(), getAccountID(c), cycle,
		req.SellerUserID, req.Note, req.AllowOpenCycle)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, statement)
}

// HandleExportCSV serves GET /v1/commissions/statements/export.csv.
//
// CSV rather than PDF: the file's job is to be pasted into the spreadsheet the
// company already reconciles against, and every line carries the sale, the
// base and the rate so a disagreement can be settled on the row rather than on
// the total.
//
// @Summary  Export a cycle's commission lines as CSV
// @Tags     commissions
// @Param    cycle query string false "Cycle (YYYY-MM)"
// @Produce  text/csv
// @Success  200 {string} string "CSV"
// @Router   /commissions/statements/export.csv [get]
func (h *CommissionStatementHandler) HandleExportCSV(c *gin.Context) {
	cycle := h.cycleParam(c)
	statements, err := h.uc.List(c.Request.Context(), getAccountID(c), cycle,
		strings.TrimSpace(c.Query("seller_user_id")))
	if err != nil {
		respondError(c, err)
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition",
		fmt.Sprintf(`attachment; filename="comissoes-%s.csv"`, cycle))

	w := csv.NewWriter(c.Writer)
	defer w.Flush()

	// Amounts go out in reais with two decimals, not cents: the file is read by
	// people, and a spreadsheet that shows 15000 where somebody expects 150,00
	// gets "corrected" by hand.
	_ = w.Write([]string{"ciclo", "vendedor", "tipo", "venda", "papel",
		"base", "percentual", "valor", "situacao", "explicacao", "observacao"})
	for _, st := range statements {
		for _, e := range st.Entries {
			_ = w.Write(domain.CSVSafeRow([]string{
				e.Cycle,
				displayName(st.SellerName, st.SellerUserID),
				string(e.Kind),
				e.SaleID,
				e.Role,
				reaisOf(e.BaseCents),
				strconv.FormatFloat(e.RatePercent, 'f', -1, 64),
				reaisOf(e.AmountCents),
				string(e.Status),
				e.Explanation,
				e.Note,
			}))
		}
	}
}

type myCommissionResponse struct {
	Cycle string `json:"cycle"`
	// PendingCents is what this salesperson has earned in the month so far and
	// has not been paid yet — the tile on their own home screen.
	PendingCents int64  `json:"pending_cents"`
	Currency     string `json:"currency"`
}

// HandleMine serves GET /v1/commissions/mine.
//
// Dashboard roles, not admin: the one number a salesperson is entitled to see
// is their own. It carries no other person's figures and no rule details.
//
// @Summary  The caller's accrued commission this month
// @Tags     commissions
// @Produce  json
// @Success  200 {object} myCommissionResponse
// @Router   /commissions/mine [get]
func (h *CommissionStatementHandler) HandleMine(c *gin.Context) {
	pending, cycle, err := h.uc.PendingForSeller(
		c.Request.Context(), getAccountID(c), getUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, myCommissionResponse{
		Cycle: cycle, PendingCents: pending,
		Currency: domain.CommissionLedgerCurrency,
	})
}

type commissionEstimateResponse struct {
	// AmountCents is what this deal would pay the CALLER. Null when no rule
	// covers it: zero would read as "you earn nothing on this", which is a
	// different and much worse claim.
	AmountCents *int64  `json:"amount_cents"`
	RatePercent float64 `json:"rate_percent,omitempty"`
	Explanation string  `json:"explanation,omitempty"`
	Currency    string  `json:"currency"`
}

// HandleEstimate serves GET /v1/commissions/estimate.
//
// "Se fechar, você ganha ~R$850" on the lead card. Uses the rule that governs
// the CALLER, so somebody on an exception rate is never shown the house number.
//
// @Summary  What a deal of this size would pay the caller
// @Tags     commissions
// @Param    gross_cents query int true "Deal value in cents"
// @Param    margin_percent query number false "Typical gross margin (default 30)"
// @Produce  json
// @Success  200 {object} commissionEstimateResponse
// @Failure  400 {object} domain.AppError
// @Router   /commissions/estimate [get]
func (h *CommissionStatementHandler) HandleEstimate(c *gin.Context) {
	gross, _ := strconv.ParseInt(strings.TrimSpace(c.Query("gross_cents")), 10, 64)
	margin, _ := strconv.ParseFloat(strings.TrimSpace(c.Query("margin_percent")), 64)

	sim, err := h.uc.EstimateForSale(c.Request.Context(), getAccountID(c),
		getUserID(c), gross, margin)
	if err != nil {
		respondError(c, err)
		return
	}
	out := commissionEstimateResponse{Currency: domain.CommissionLedgerCurrency}
	if sim != nil {
		amount := sim.PerSaleCents
		out.AmountCents = &amount
		if sim.Calc != nil {
			out.RatePercent = sim.Calc.RatePercent
			out.Explanation = sim.Calc.Explanation
		}
	}
	c.JSON(http.StatusOK, out)
}

func displayName(name, fallback string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	return fallback
}

// reaisOf writes cents as a decimal with a comma, the separator a Brazilian
// spreadsheet expects.
func reaisOf(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return fmt.Sprintf("%s%d,%02d", sign, cents/100, cents%100)
}

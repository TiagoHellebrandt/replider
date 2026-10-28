package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Clause alerts and recovered revenue (§6.11, G5/G6).
//
// Admin-only, like the contracts themselves: these payloads carry the value, the
// penalties and what the company went back and collected.
type ContractAlertHandler struct {
	uc *usecases.ContractAlertUseCase
}

func NewContractAlertHandler(uc *usecases.ContractAlertUseCase) *ContractAlertHandler {
	return &ContractAlertHandler{uc: uc}
}

// HandleAttention serves GET /v1/contracts/:id/attention.
//
// The plain-language summary plus the red-flag list, derived from the confirmed
// fields. An unconfirmed contract returns an empty list with confirmed=false, and
// the screen has to say why — an empty list on its own looks like a clean
// contract.
//
// @Summary  Plain-language summary and red flags for one contract
// @Tags     contracts
// @Param    id path string true "Contract ID"
// @Produce  json
// @Success  200 {object} usecases.ContractAttention
// @Failure  404 {object} domain.AppError
// @Router   /contracts/{id}/attention [get]
func (h *ContractAlertHandler) HandleAttention(c *gin.Context) {
	out, err := h.uc.Attention(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type scheduleAlertRequest struct {
	// Kind is one of the derived kinds, or "custom".
	Kind string `json:"kind"`
	// DueOn and Title are read ONLY for a custom alert. For a derived kind both
	// come from the derivation: a client that could set its own date on a
	// renewal-notice alert could schedule the reminder for the day after the
	// window closes.
	DueOn  string `json:"due_on"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	// MoneyAtStakeCents is honoured only for a custom alert too.
	MoneyAtStakeCents int64 `json:"money_at_stake_cents"`
}

// HandleSchedule serves POST /v1/contracts/:id/alerts.
//
// The 1-click "agendar" behind each red flag.
//
// @Summary  Schedule a clause deadline
// @Tags     contracts
// @Accept   json
// @Produce  json
// @Success  200 {object} domain.ContractAlert
// @Failure  400 {object} domain.AppError
// @Failure  404 {object} domain.AppError
// @Router   /contracts/{id}/alerts [post]
func (h *ContractAlertHandler) HandleSchedule(c *gin.Context) {
	var req scheduleAlertRequest
	if !bindJSON(c, &req) {
		return
	}
	alert, err := h.uc.Schedule(c.Request.Context(), getAccountID(c), getUserID(c),
		usecases.ScheduleInput{
			ContractID:        c.Param("id"),
			Kind:              domain.ContractAlertKind(strings.TrimSpace(req.Kind)),
			DueOn:             parseDateOnly(req.DueOn),
			Title:             req.Title,
			Detail:            req.Detail,
			MoneyAtStakeCents: req.MoneyAtStakeCents,
		})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, alert)
}

// alertResponse adds the countdown the client would otherwise compute.
type alertResponse struct {
	*domain.ContractAlert
	// DaysUntilDue is NEGATIVE once the deadline passed — the state that costs
	// money, which must not read as zero.
	DaysUntilDue int  `json:"days_until_due"`
	Overdue      bool `json:"overdue"`
}

type dueAlertsResponse struct {
	Alerts []alertResponse `json:"alerts"`
	// TotalAtStakeCents is what the whole list is worth. The number that decides
	// whether anybody opens it.
	TotalAtStakeCents int64 `json:"total_at_stake_cents"`
	// Overdue is how many already lapsed, so the UI can lead with them.
	Overdue int `json:"overdue"`
}

// HandleDue serves GET /v1/contract-alerts.
//
// @Summary  Pending clause deadlines, soonest first
// @Tags     contracts
// @Param    horizon_days query int false "How far ahead to look (default 90)"
// @Produce  json
// @Success  200 {object} dueAlertsResponse
// @Router   /contract-alerts [get]
func (h *ContractAlertHandler) HandleDue(c *gin.Context) {
	horizon, _ := strconv.Atoi(strings.TrimSpace(c.Query("horizon_days")))
	limit, _ := pagination(c)

	alerts, err := h.uc.Due(c.Request.Context(), getAccountID(c), horizon, limit)
	if err != nil {
		respondError(c, err)
		return
	}
	now := time.Now()
	out := dueAlertsResponse{Alerts: make([]alertResponse, 0, len(alerts))}
	for _, a := range alerts {
		out.TotalAtStakeCents += a.MoneyAtStakeCents
		if a.Overdue(now) {
			out.Overdue++
		}
		out.Alerts = append(out.Alerts, alertResponse{
			ContractAlert: a, DaysUntilDue: a.DaysUntilDue(now), Overdue: a.Overdue(now),
		})
	}
	c.JSON(http.StatusOK, out)
}

type completeAlertRequest struct {
	// Dismiss closes the alert without claiming anything happened.
	Dismiss bool   `json:"dismiss"`
	Note    string `json:"note"`
	// RecoveredCents is what was actually collected — a human's figure, never the
	// computed one. The accumulated index is what was OWED; what was agreed
	// differs every time somebody negotiates.
	RecoveredCents int64  `json:"recovered_cents"`
	RecognisedOn   string `json:"recognised_on"`
}

// HandleComplete serves POST /v1/contract-alerts/:id/complete.
//
// @Summary  Close a deadline, and record what it recovered
// @Tags     contracts
// @Accept   json
// @Produce  json
// @Success  200 {object} domain.ContractAlert
// @Failure  400 {object} domain.AppError
// @Failure  404 {object} domain.AppError
// @Router   /contract-alerts/{id}/complete [post]
func (h *ContractAlertHandler) HandleComplete(c *gin.Context) {
	var req completeAlertRequest
	if !bindJSON(c, &req) {
		return
	}
	alert, err := h.uc.Complete(c.Request.Context(), getAccountID(c), getUserID(c),
		c.Param("id"), usecases.CompleteInput{
			Dismiss:        req.Dismiss,
			Note:           req.Note,
			RecoveredCents: req.RecoveredCents,
			RecognisedOn:   parseDateOnly(req.RecognisedOn),
		})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, alert)
}

type recoveryRequest struct {
	ContractID string `json:"contract_id"`
	// Kind is the deadline shape this plugged: adjustment, renewal or registry.
	Kind         string `json:"kind"`
	AmountCents  int64  `json:"amount_cents"`
	Note         string `json:"note"`
	RecognisedOn string `json:"recognised_on"`
}

// HandleRecordRecovery serves POST /v1/contract-recoveries.
//
// For revenue somebody recovered without the product prompting them. A ledger
// that only counted what the product prompted would flatter the product and
// understate the customer.
//
// @Summary  Record recovered contracted revenue
// @Tags     contracts
// @Accept   json
// @Produce  json
// @Success  200 {object} domain.ContractRecovery
// @Failure  400 {object} domain.AppError
// @Router   /contract-recoveries [post]
func (h *ContractAlertHandler) HandleRecordRecovery(c *gin.Context) {
	var req recoveryRequest
	if !bindJSON(c, &req) {
		return
	}
	rec, err := h.uc.RecordRecovery(c.Request.Context(), getAccountID(c), getUserID(c),
		req.ContractID, domain.ContractAlertKind(strings.TrimSpace(req.Kind)),
		req.AmountCents, req.Note, parseDateOnly(req.RecognisedOn))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, rec)
}

type recoveredResponse struct {
	*domain.ContractRecoverySummary
	// Recoveries is the list behind the total: a headline nobody can drill into
	// is a headline nobody believes.
	Recoveries []*domain.ContractRecovery `json:"recoveries"`
	Currency   string                     `json:"currency"`
}

// HandleRecovered serves GET /v1/contract-recoveries.
//
// The headline: how much contracted revenue this account went back and collected.
//
// @Summary  Recovered contracted revenue for a period
// @Tags     contracts
// @Param    from query string false "Start date (YYYY-MM-DD), defaults to 12 months ago"
// @Param    to query string false "End date (YYYY-MM-DD), exclusive"
// @Produce  json
// @Success  200 {object} recoveredResponse
// @Router   /contract-recoveries [get]
func (h *ContractAlertHandler) HandleRecovered(c *gin.Context) {
	now := time.Now()
	// A year by default: the leak this measures is annual (an index applied once
	// a year, a renewal once a term), and a 30-day window would report zero on an
	// account that recovered R$300k in March.
	from := time.Date(now.Year()-1, now.Month(), 1, 0, 0, 0, 0, time.UTC)
	to := now.AddDate(0, 0, 1)
	if parsed := parseDateOnly(c.Query("from")); parsed != nil {
		from = *parsed
	}
	if parsed := parseDateOnly(c.Query("to")); parsed != nil {
		to = *parsed
	}

	summary, err := h.uc.Recovered(c.Request.Context(), getAccountID(c), from, to)
	if err != nil {
		respondError(c, err)
		return
	}
	limit, _ := pagination(c)
	list, err := h.uc.RecoveryLedger(c.Request.Context(), getAccountID(c), from, to, limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, recoveredResponse{
		ContractRecoverySummary: summary, Recoveries: list,
		Currency: domain.CommissionLedgerCurrency,
	})
}

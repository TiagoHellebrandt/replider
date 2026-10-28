package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// AdminHandler groups self-serve infra health checks exposed to workspace
// admins — currently SMTP canary + WhatsApp webhook outbox triage. Kept
// separate from LGPDHandler because those endpoints target customer-data
// rights, while these target server-side plumbing.
type AdminHandler struct {
	sendSMTPTestUC     *usecases.SendSMTPTestUseCase
	waOutbox           domain.WAWebhookOutbox
	waBillingMeter     domain.WABillingMeterRepository
	accountRepo        domain.AccountRepository
	auditLog           domain.AuditLogRepository
	agentRepo          domain.AgentRepository
	billingMetrics     *usecases.BillingMetricsUseCase
	accountHealth      *usecases.AccountHealthUseCase
	npsMetrics         *usecases.NPSMetricsUseCase
	cancellationSurvey *usecases.CancellationSurveyMetricsUseCase
}

// SetCancellationSurveyMetrics enables the /admin/cancellation-survey endpoint
// (US-180). Nil-safe — endpoint returns 503 when the use case is missing.
func (h *AdminHandler) SetCancellationSurveyMetrics(uc *usecases.CancellationSurveyMetricsUseCase) {
	h.cancellationSurvey = uc
}

// SetNPSMetrics enables the /admin/nps endpoint (US-178). Nil-safe —
// endpoint returns 503 when the use case is missing.
func (h *AdminHandler) SetNPSMetrics(uc *usecases.NPSMetricsUseCase) {
	h.npsMetrics = uc
}

// SetAccountHealth enables the /admin/accounts/health endpoint (US-177).
// Nil-safe — endpoint returns 503 when the use case is missing.
func (h *AdminHandler) SetAccountHealth(uc *usecases.AccountHealthUseCase) {
	h.accountHealth = uc
}

// SetBillingMetrics enables the /admin/billing/metrics endpoint (US-071).
// Nil-safe — endpoint returns 503 when the use case is missing.
func (h *AdminHandler) SetBillingMetrics(uc *usecases.BillingMetricsUseCase) {
	h.billingMetrics = uc
}

// SetAccountRepo + SetAuditLog enable the trial-bypass admin
// endpoint. Both nil-safe — endpoint returns 503 when either is
// missing in the deployment.
func (h *AdminHandler) SetAccountRepo(r domain.AccountRepository) { h.accountRepo = r }
func (h *AdminHandler) SetAuditLog(a domain.AuditLogRepository)   { h.auditLog = a }

// SetAgentRepo enables the /admin/agents/invalid-timezones endpoint. Nil-safe —
// endpoint returns 503 when the repo is missing in the deployment.
func (h *AdminHandler) SetAgentRepo(r domain.AgentRepository) { h.agentRepo = r }

func NewAdminHandler(sendSMTPTestUC *usecases.SendSMTPTestUseCase) *AdminHandler {
	return &AdminHandler{sendSMTPTestUC: sendSMTPTestUC}
}

// SetWAWebhookOutbox enables the /admin/wa-webhook-events endpoints. The
// outbox is optional in non-prod test wiring, so the handler treats nil
// as "feature disabled" rather than panicking.
func (h *AdminHandler) SetWAWebhookOutbox(o domain.WAWebhookOutbox) {
	h.waOutbox = o
}

// SetWABillingMeter enables the /admin/wa-meter endpoint. The meter
// repo is optional (Stripe metering disabled in dev) so we treat nil
// as "feature disabled".
func (h *AdminHandler) SetWABillingMeter(m domain.WABillingMeterRepository) {
	h.waBillingMeter = m
}

// HandleSendSMTPTest sends a canary email to the authenticated admin.
// @Summary  Send SMTP test email
// @Description  Admin-only. Sends a test email to the caller's own address to verify SMTP is healthy end-to-end.
// @Tags     admin
// @Produce  json
// @Security BearerAuth
// @Success  200  {object}  map[string]string
// @Failure  401  {object}  domain.AppError
// @Failure  500  {object}  domain.AppError
// @Router   /admin/smtp/test [post]
func (h *AdminHandler) HandleSendSMTPTest(c *gin.Context) {
	accountID := c.GetString("accountID")
	userID := c.GetString("userID")
	if accountID == "" || userID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "UNAUTHORIZED", Message: "unauthorized"})
		return
	}

	if err := h.sendSMTPTestUC.Execute(c.Request.Context(), accountID, userID); err != nil {
		slog.Warn("smtp test email failed", "account_id", accountID, "error", err)
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "test email sent"})
}

// HandleListWAWebhookEvents lists outbox digests for triage.
// @Summary  List WhatsApp webhook outbox events
// @Description  Admin-only. Surfaces recent inbound webhooks so ops can
//
//	spot stuck `pending`, `failed`, or `dead` rows. Filter via the
//	optional `status` query param (`all` — default — / pending /
//	processed / skipped / dead). `limit` caps page size to 200.
//
// @Tags     admin
// @Produce  json
// @Security BearerAuth
// @Param    status  query  string  false  "all|pending|processed|skipped|dead"
// @Param    limit   query  int     false  "max 200"
// @Success  200  {array}  domain.WAWebhookEventDigest
// @Failure  401  {object}  domain.AppError
// @Failure  503  {object}  domain.AppError  "outbox not wired"
// @Router   /admin/wa-webhook-events [get]
func (h *AdminHandler) HandleListWAWebhookEvents(c *gin.Context) {
	if h.waOutbox == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code:    "WA_OUTBOX_DISABLED",
			Message: "wa webhook outbox is not configured on this deployment",
		})
		return
	}
	status := c.DefaultQuery("status", "all")
	switch status {
	case "all", "pending", "processed", "skipped", "dead", "failed":
	default:
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code:    "INVALID_STATUS",
			Message: "status must be one of all/pending/processed/skipped/failed/dead",
		})
		return
	}
	limit := 50
	if v := c.Query("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
			if limit > 200 {
				limit = 200
			}
		}
	}
	rows, err := h.waOutbox.ListByStatus(c.Request.Context(), status, limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, rows)
}

// HandleRetryWAWebhookEvent re-enqueues a single outbox row.
// @Summary  Re-enqueue a WhatsApp webhook outbox row
// @Description  Admin-only. Resets the row's status to `pending` and
//
//	clears its retry counters so the next worker tick replays it. Use
//	after fixing the root cause of a `dead` row.
//
// @Tags     admin
// @Param    id  path  string  true  "outbox row id"
// @Success  204
// @Failure  401  {object}  domain.AppError
// @Failure  503  {object}  domain.AppError
// @Router   /admin/wa-webhook-events/{id}/retry [post]
func (h *AdminHandler) HandleRetryWAWebhookEvent(c *gin.Context) {
	if h.waOutbox == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code:    "WA_OUTBOX_DISABLED",
			Message: "wa webhook outbox is not configured on this deployment",
		})
		return
	}
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "ID_REQUIRED", Message: "id required"})
		return
	}
	if err := h.waOutbox.ResetForRetry(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// HandleSetTrialBypass flips the trial-bypass flag on any account.
// Admin-only — and audited so the toggle can't be abused silently.
//
// @Summary  Toggle account trial-bypass (admin)
// @Tags     admin
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    id    path  string  true  "Account ID"
// @Param    body  body  object{bypass=boolean,reason=string}  true  "bypass flag + audit reason"
// @Success  200   {object}  map[string]bool
// @Router   /admin/accounts/{id}/trial-bypass [post]
func (h *AdminHandler) HandleSetTrialBypass(c *gin.Context) {
	if h.accountRepo == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code:    "TRIAL_BYPASS_DISABLED",
			Message: "trial bypass admin endpoint not configured",
		})
		return
	}
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "ID_REQUIRED", Message: "id required"})
		return
	}
	var req struct {
		Bypass bool   `json:"bypass"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "VALIDATION_ERROR", Message: "invalid body"})
		return
	}
	// Capture the prior flag value BEFORE the flip so the audit row records
	// the full before/after transition (US-014), not just the new state.
	// Best-effort: a read miss leaves before=nil rather than blocking.
	var before *bool
	if acc, err := h.accountRepo.GetByID(c.Request.Context(), id); err == nil && acc != nil {
		b := acc.TrialBypass
		before = &b
	}

	if err := h.accountRepo.SetTrialBypass(c.Request.Context(), id, req.Bypass, req.Reason); err != nil {
		respondError(c, err)
		return
	}
	// Audit log the flip so support / compliance can trace bypass
	// usage. Best-effort — failure here doesn't block the action.
	if h.auditLog != nil {
		actor := c.GetString("userID")
		payload, _ := json.Marshal(map[string]any{
			"before": before,
			"after":  req.Bypass,
			"reason": req.Reason,
		})
		_ = h.auditLog.Create(c.Request.Context(), &domain.AuditLog{
			ID:         ulid.Make().String(),
			AccountID:  id,
			ActorID:    &actor,
			Action:     "account.trial_bypass.set",
			EntityType: "account",
			EntityID:   id,
			Payload:    payload,
			CreatedAt:  time.Now(),
		})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "bypass": req.Bypass})
}

// HandleListWABillingMeter lists pending WhatsApp pricing rollups
// awaiting Stripe push. Used by the ops dashboard's "billing health"
// panel — sustained non-zero counts mean the Stripe push worker is
// failing or the plan's subscription_item IDs are mis-configured.
//
// @Summary  Pending WhatsApp meter rollups
// @Tags     admin
// @Produce  json
// @Security BearerAuth
// @Success  200  {array}  domain.WABillingMeterRollup
// @Router   /admin/wa-meter [get]
func (h *AdminHandler) HandleListWABillingMeter(c *gin.Context) {
	if h.waBillingMeter == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code:    "WA_METER_DISABLED",
			Message: "wa billing meter is not configured on this deployment",
		})
		return
	}
	limit := 200
	rows, err := h.waBillingMeter.ListPending(c.Request.Context(), limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows, "count": len(rows)})
}

// HandleGetBillingMetrics returns platform-wide revenue analytics —
// MRR, churn rate and LTV — derived from the billing_events ledger
// (US-071). The optional `window` query param (days, default 30, max
// 365) bounds the churn lookback.
//
// @Summary  Billing metrics (MRR / churn / LTV)
// @Description  Admin-only. Computes MRR, ARPA, churn rate and LTV from the billing events ledger.
// @Tags     admin
// @Produce  json
// @Security BearerAuth
// @Param    window  query  int  false  "churn window in days (default 30, max 365)"
// @Success  200  {object}  usecases.BillingMetricsResult
// @Failure  401  {object}  domain.AppError
// @Failure  503  {object}  domain.AppError
// @Router   /admin/billing/metrics [get]
func (h *AdminHandler) HandleGetBillingMetrics(c *gin.Context) {
	if h.billingMetrics == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code:    "BILLING_METRICS_DISABLED",
			Message: "billing metrics are not configured on this deployment",
		})
		return
	}
	window := 30
	if v := c.Query("window"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			window = parsed
			if window > 365 {
				window = 365
			}
		}
	}
	res, err := h.billingMetrics.Execute(c.Request.Context(), window)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// HandleListInvalidTimezoneAgents surfaces agents whose Timezone fails
// time.LoadLocation. AgentRunner silently falls back to UTC for these, which
// breaks booking slot math — this endpoint lets an admin spot the
// misconfiguration that the per-message WARN log only hints at.
// @Summary  List agents with invalid timezone
// @Description  Admin-only. Returns agents whose timezone string does not resolve via tzdata (AgentRunner falls back to UTC).
// @Tags     admin
// @Produce  json
// @Security BearerAuth
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  domain.AppError
// @Failure  503  {object}  domain.AppError
// @Router   /admin/agents/invalid-timezones [get]
func (h *AdminHandler) HandleListInvalidTimezoneAgents(c *gin.Context) {
	if h.agentRepo == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code:    "AGENT_REPO_DISABLED",
			Message: "agent repository is not configured on this deployment",
		})
		return
	}
	agents, err := h.agentRepo.ListAllTimezones(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	invalid := make([]*domain.AgentTimezone, 0)
	for _, a := range agents {
		if !domain.IsValidTimezone(a.Timezone) {
			invalid = append(invalid, a)
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": invalid, "count": len(invalid)})
}

// HandleListAccountHealth returns a per-account retention health score
// (US-177). The score (0-100) blends weekly bot-reply volume, whether the
// bot is active, the human-takeover ratio, and admin login recency.
// Accounts whose score dropped 30%+ week-over-week are flagged `at_risk`
// and sorted to the top so support can intervene before churn.
//
// @Summary  Per-account health scores (admin)
// @Description  Admin-only. Retention health score per account with a week-over-week drop alert.
// @Tags     admin
// @Produce  json
// @Security BearerAuth
// @Success  200  {object}  map[string]interface{}
// @Failure  401  {object}  domain.AppError
// @Failure  503  {object}  domain.AppError
// @Router   /admin/accounts/health [get]
func (h *AdminHandler) HandleListAccountHealth(c *gin.Context) {
	if h.accountHealth == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code:    "ACCOUNT_HEALTH_DISABLED",
			Message: "account health scoring is not configured on this deployment",
		})
		return
	}
	scores, err := h.accountHealth.Execute(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	atRisk := 0
	for _, s := range scores {
		if s.AtRisk {
			atRisk++
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": scores, "count": len(scores), "at_risk": atRisk})
}

// HandleGetNPSMetrics returns the aggregated in-app Net Promoter Score plus
// recent verbatim comments (US-178). The optional `window` query param
// (days, default 90 = one quarter, max 365) bounds the aggregation.
//
// @Summary  In-app NPS metrics (admin)
// @Description  Admin-only. NPS score (-100..100), promoter/passive/detractor counts and recent comments over the window.
// @Tags     admin
// @Produce  json
// @Security BearerAuth
// @Param    window  query  int  false  "window in days (default 90, max 365)"
// @Success  200  {object}  usecases.NPSMetricsResult
// @Failure  401  {object}  domain.AppError
// @Failure  503  {object}  domain.AppError
// @Router   /admin/nps [get]
func (h *AdminHandler) HandleGetNPSMetrics(c *gin.Context) {
	if h.npsMetrics == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code:    "NPS_METRICS_DISABLED",
			Message: "nps metrics are not configured on this deployment",
		})
		return
	}
	window := 90
	if v := c.Query("window"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			window = parsed
			if window > 365 {
				window = 365
			}
		}
	}
	res, err := h.npsMetrics.Execute(c.Request.Context(), window)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// HandleGetCancellationSurvey returns the churn-reason breakdown plus recent
// verbatim comments (US-180). The optional `window` query param (days,
// default 90 = one quarter, max 365) bounds the aggregation.
//
// @Summary  Cancellation reason breakdown (admin)
// @Description  Admin-only. Per-reason cancellation counts and recent comments over the window.
// @Tags     admin
// @Produce  json
// @Security BearerAuth
// @Param    window  query  int  false  "window in days (default 90, max 365)"
// @Success  200  {object}  usecases.CancellationSurveyMetricsResult
// @Failure  401  {object}  domain.AppError
// @Failure  503  {object}  domain.AppError
// @Router   /admin/cancellation-survey [get]
func (h *AdminHandler) HandleGetCancellationSurvey(c *gin.Context) {
	if h.cancellationSurvey == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code:    "CANCELLATION_SURVEY_DISABLED",
			Message: "cancellation survey metrics are not configured on this deployment",
		})
		return
	}
	window := 90
	if v := c.Query("window"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			window = parsed
			if window > 365 {
				window = 365
			}
		}
	}
	res, err := h.cancellationSurvey.Execute(c.Request.Context(), window)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

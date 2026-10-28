package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Command centre (§6.13, I2).

// The endpoints hang off MeHandler and are wired by an optional setter, so
// neither NewMeHandler nor router.Setup changes signature (the same pattern as
// me_support_handler.go and the dashboard BFF). Unwired answers 503 rather than
// panicking.

// SetCommandCenterUC wires the home command centre.
func (h *MeHandler) SetCommandCenterUC(uc *usecases.CommandCenterUseCase) {
	h.commandCenterUC = uc
}

// HandleGetCommandCenter serves GET /v1/me/command-center.
//
// @Summary     Home command centre
// @Description Four counted tiles, the mini-funnel, the impact-ranked "needs you" list (each row naming the customer) and at most one copilot suggestion. One round trip so the first screen renders at once.
// @Tags        Me
// @Produce     json
// @Success     200 {object} usecases.CommandCenterResponse
// @Failure     401 {object} domain.AppError
// @Failure     403 {object} domain.AppError
// @Security    BearerAuth
// @Router      /me/command-center [get]
func (h *MeHandler) HandleGetCommandCenter(c *gin.Context) {
	if h.commandCenterUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "COMMAND_CENTER_UNAVAILABLE", Message: "command centre not configured"})
		return
	}
	out, err := h.commandCenterUC.Execute(c.Request.Context(), actorFrom(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// dismissPendingActionRequest snoozes one row.
//
// Deliberately named "dismiss", not "complete": the actions are derived from
// live state, so the only honest thing a button can do without doing the work is
// hide the row for a while.
type dismissPendingActionRequest struct {
	Kind     string `json:"kind" binding:"required"`
	TargetID string `json:"target_id" binding:"required"`
	// SnoozeHours is optional; omitted uses the default and anything past the
	// cap is clamped, because a "snooze" long enough to outlive the deadline
	// is a deletion with better manners.
	SnoozeHours int    `json:"snooze_hours,omitempty"`
	Note        string `json:"note,omitempty"`
}

// HandleDismissPendingAction serves POST /v1/me/pending-actions/dismiss.
//
// @Summary     Snooze a pending action
// @Description Hides one row of "needs you" for this USER only, for a bounded time. There is no permanent dismissal and no "mark as done" — the row returns unless the underlying work is actually done.
// @Tags        Me
// @Accept      json
// @Produce     json
// @Param       request body dismissPendingActionRequest true "Action to snooze"
// @Success     204
// @Failure     400 {object} domain.AppError
// @Failure     401 {object} domain.AppError
// @Security    BearerAuth
// @Router      /me/pending-actions/dismiss [post]
func (h *MeHandler) HandleDismissPendingAction(c *gin.Context) {
	if h.commandCenterUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "COMMAND_CENTER_UNAVAILABLE", Message: "command centre not configured"})
		return
	}
	var req dismissPendingActionRequest
	if !bindJSON(c, &req) {
		return
	}

	dismissal := &domain.PendingActionDismissal{
		Kind:     domain.PendingActionKind(req.Kind),
		TargetID: req.TargetID,
		Note:     req.Note,
	}
	if req.SnoozeHours > 0 {
		dismissal.SnoozedUntil = time.Now().Add(time.Duration(req.SnoozeHours) * time.Hour)
	}

	if err := h.commandCenterUC.Dismiss(c.Request.Context(), actorFrom(c), dismissal); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// SetOvernightRepo wires the overnight-drafting settings and the briefing
// acknowledgement (K2). Optional: nil answers 503.
func (h *MeHandler) SetOvernightRepo(r domain.OvernightDraftRepository) {
	h.overnightRepo = r
}

// overnightSettingsRequest is the opt-in.
type overnightSettingsRequest struct {
	Enabled    bool   `json:"enabled"`
	FunnelID   string `json:"funnel_id,omitempty"`
	NightlyCap int    `json:"nightly_cap,omitempty"`
	RunHour    int    `json:"run_hour"`
}

// HandleGetOvernightSettings serves GET /v1/me/overnight-drafting.
//
// @Summary     Overnight drafting settings
// @Description Whether the AI fills the approval queue overnight, and with what nightly cap. Absent settings answer the defaults rather than 404, so the screen renders a form instead of an error.
// @Tags        Me
// @Produce     json
// @Success     200 {object} domain.OvernightDraftSettings
// @Failure     503 {object} domain.AppError
// @Security    BearerAuth
// @Router      /me/overnight-drafting [get]
func (h *MeHandler) HandleGetOvernightSettings(c *gin.Context) {
	if h.overnightRepo == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "OVERNIGHT_UNAVAILABLE", Message: "overnight drafting not configured"})
		return
	}
	settings, err := h.overnightRepo.Settings(c.Request.Context(), getAccountID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	if settings == nil {
		// No row = the default, and the default is ON (2026-08-19): the
		// worker drafts for rowless accounts every night. The screen must
		// say what production will do, so Enabled is true here too.
		settings = &domain.OvernightDraftSettings{
			Enabled:    true,
			NightlyCap: domain.DefaultNightlyDraftCap,
			RunHour:    domain.DefaultOvernightRunHour,
		}
	}
	c.JSON(http.StatusOK, settings)
}

// HandlePutOvernightSettings serves PUT /v1/me/overnight-drafting.
//
// @Summary     Configure overnight drafting
// @Description Turn the nightly drafting off or tune it. ON by default since 2026-08-19 — the promise is a queue already filled in the morning; the plan's AI-draft allowance still bounds the spend. An explicit enabled=false here is respected forever.
// @Tags        Me
// @Accept      json
// @Produce     json
// @Param       request body overnightSettingsRequest true "Settings"
// @Success     200 {object} domain.OvernightDraftSettings
// @Failure     400 {object} domain.AppError
// @Failure     503 {object} domain.AppError
// @Security    BearerAuth
// @Router      /me/overnight-drafting [put]
func (h *MeHandler) HandlePutOvernightSettings(c *gin.Context) {
	if h.overnightRepo == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "OVERNIGHT_UNAVAILABLE", Message: "overnight drafting not configured"})
		return
	}
	var req overnightSettingsRequest
	if !bindJSON(c, &req) {
		return
	}

	settings := &domain.OvernightDraftSettings{
		AccountID:  getAccountID(c),
		Enabled:    req.Enabled,
		FunnelID:   req.FunnelID,
		NightlyCap: req.NightlyCap,
		RunHour:    req.RunHour,
		// Who turned it on, from the session — never from the body.
		EnabledBy: getUserID(c),
	}
	if err := settings.Validate(); err != nil {
		respondError(c, err)
		return
	}
	if err := h.overnightRepo.SaveSettings(c.Request.Context(), settings); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

// HandleSeenBriefing serves POST /v1/me/overnight-drafting/:id/seen.
//
// @Summary     Acknowledge the morning briefing
// @Description Stops "seu SDR trabalhou de madrugada" reappearing for the rest of the day.
// @Tags        Me
// @Produce     json
// @Param       id path string true "Run ID"
// @Success     204
// @Failure     503 {object} domain.AppError
// @Security    BearerAuth
// @Router      /me/overnight-drafting/{id}/seen [post]
func (h *MeHandler) HandleSeenBriefing(c *gin.Context) {
	if h.overnightRepo == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "OVERNIGHT_UNAVAILABLE", Message: "overnight drafting not configured"})
		return
	}
	// Account-scoped in the statement, so another tenant's run id matches
	// nothing rather than relying on a check here.
	if err := h.overnightRepo.MarkSeen(c.Request.Context(),
		getAccountID(c), c.Param("id"), time.Now()); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// SetOffersOutcomeUC wires the weekly outcome line for the Ofertas screen.
func (h *MeHandler) SetOffersOutcomeUC(uc *usecases.OffersOutcomeUseCase) {
	h.offersOutcomeUC = uc
}

// HandleOffersOutcome serves GET /v1/me/offers/outcome.
//
// @Summary     This week's offer outcomes
// @Description Sent approaches, replies received and proposals created since Monday 00:00 in the account's timezone. The feedback line the approval queue was missing.
// @Tags        Me
// @Produce     json
// @Success     200 {object} usecases.OffersOutcome
// @Failure     401 {object} domain.AppError
// @Failure     503 {object} domain.AppError
// @Security    BearerAuth
// @Router      /me/offers/outcome [get]
func (h *MeHandler) HandleOffersOutcome(c *gin.Context) {
	if h.offersOutcomeUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "OFFERS_OUTCOME_UNAVAILABLE", Message: "offers outcome not configured"})
		return
	}
	out, err := h.offersOutcomeUC.Execute(c.Request.Context(), getAccountID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

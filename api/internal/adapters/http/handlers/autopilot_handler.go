package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// Progressive autopilot (§3.4).
//
// The screen this backs has one job beyond a toggle: making it plain what
// autopilot will and will not do. A feature that sends cold email unread is
// one an operator has to be able to describe out loud before they turn it on.
type AutopilotHandler struct {
	repo domain.AutopilotRepository
	// accounts resolves the account's own timezone so "sent today" means the
	// operator's today. Optional: unwired falls back to UTC.
	accounts AccountTimezoneLookup
	// enabled mirrors config.LegacyWorkers: the auto-send hook is gone
	// (2026-09-01), so without it the settings are inert and every route
	// answers 503 AUTOPILOT_UNAVAILABLE rather than storing a dead toggle.
	enabled bool
}

// AccountTimezoneLookup is the one account field a daily window needs.
type AccountTimezoneLookup interface {
	GetByID(ctx context.Context, id string) (*domain.Account, error)
}

func NewAutopilotHandler(repo domain.AutopilotRepository) *AutopilotHandler {
	return &AutopilotHandler{repo: repo, enabled: true}
}

// SetEnabled gates the whole /autopilot surface (LEGACY_WORKERS).
func (h *AutopilotHandler) SetEnabled(on bool) { h.enabled = on }

// unavailable writes the 503 and reports whether the caller must stop.
func (h *AutopilotHandler) unavailable(c *gin.Context) bool {
	if h.enabled {
		return false
	}
	respondFeatureUnavailable(c, "AUTOPILOT_UNAVAILABLE", "piloto automático desativado neste plano")
	return true
}

// SetAccounts wires the per-account timezone lookup.
func (h *AutopilotHandler) SetAccounts(a AccountTimezoneLookup) { h.accounts = a }

type autopilotResponse struct {
	*domain.Autopilot
	// EligibleToOffer is why the dashboard shows the invitation. Computed
	// here so the client cannot disagree with the server about who earned it.
	EligibleToOffer bool `json:"eligible_to_offer"`
	// StreakNeeded lets the UI say "faltam 6" instead of "not eligible",
	// which is the difference between a goal and a wall.
	StreakNeeded int `json:"streak_needed"`
	// SentToday is the cap in the operator's terms.
	SentToday int `json:"sent_today"`
	DailyCap  int `json:"effective_daily_cap"`
	MinScore  int `json:"effective_min_score"`
}

// HandleGet serves GET /v1/autopilot.
//
// @Summary  Autopilot status
// @Tags     autopilot
// @Produce  json
// @Success  200 {object} autopilotResponse
// @Router   /autopilot [get]
func (h *AutopilotHandler) HandleGet(c *gin.Context) {
	if h.unavailable(c) {
		return
	}
	accountID := getAccountID(c)
	settings, err := h.repo.Get(c.Request.Context(), accountID)
	if err != nil {
		respondError(c, err)
		return
	}
	sentToday, err := h.repo.CountSentToday(c.Request.Context(), accountID,
		h.startOfToday(c, accountID))
	if err != nil {
		// The count is informational here — the worker enforces the cap and
		// refuses its own pass when it cannot read it. Failing this whole
		// screen over a counter would hide the toggle too.
		sentToday = 0
	}

	needed := domain.AutopilotStreakToOffer - settings.UneditedStreak
	if needed < 0 {
		needed = 0
	}
	c.JSON(http.StatusOK, autopilotResponse{
		Autopilot:       settings,
		EligibleToOffer: settings.EligibleToOffer(),
		StreakNeeded:    needed,
		SentToday:       sentToday,
		DailyCap:        settings.EffectiveDailyCap(),
		MinScore:        settings.EffectiveMinScore(),
	})
}

type autopilotUpdateRequest struct {
	Enabled  *bool `json:"enabled"`
	DailyCap int   `json:"daily_cap"`
	MinScore int   `json:"min_score"`
}

// HandleUpdate serves PUT /v1/autopilot.
//
// Turning it ON requires the streak. The server checks it rather than
// trusting the client: the whole premise is that this is EARNED, and a
// direct API call must not be the way around earning it.
//
// Turning it OFF is always allowed, immediately, with no conditions. A safety
// switch that argues is not a safety switch.
//
// @Summary  Enable, disable or tune autopilot
// @Tags     autopilot
// @Accept   json
// @Produce  json
// @Success  200 {object} domain.Autopilot
// @Failure  409 {object} domain.AppError
// @Router   /autopilot [put]
func (h *AutopilotHandler) HandleUpdate(c *gin.Context) {
	if h.unavailable(c) {
		return
	}
	var req autopilotUpdateRequest
	if !bindJSON(c, &req) {
		return
	}
	accountID := getAccountID(c)
	settings, err := h.repo.Get(c.Request.Context(), accountID)
	if err != nil {
		respondError(c, err)
		return
	}

	turningOn := req.Enabled != nil && *req.Enabled && !settings.Enabled
	if turningOn && settings.UneditedStreak < domain.AutopilotStreakToOffer {
		respondError(c, &domain.AppError{
			Code: "AUTOPILOT_NOT_EARNED",
			Message: "o piloto automático libera depois de 20 aprovações seguidas " +
				"enviadas exatamente como foram sugeridas",
		})
		return
	}

	if req.Enabled != nil {
		settings.Enabled = *req.Enabled
	}
	if req.DailyCap > 0 {
		settings.DailyCap = req.DailyCap
	}
	if req.MinScore > 0 {
		settings.MinScore = req.MinScore
	}
	settings.AccountID = accountID
	if turningOn {
		now := time.Now()
		settings.EnabledAt = &now
		settings.EnabledBy = getUserID(c)
	}

	if err := h.repo.Save(c.Request.Context(), settings); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

// HandleListSends serves GET /v1/autopilot/sends.
//
// Everything autopilot sent, newest first. It is the only way an operator can
// answer "what did it do while I was away?" — and a feature that sends
// unread mail without that answer is one nobody should turn on.
//
// @Summary  What autopilot has sent
// @Tags     autopilot
// @Produce  json
// @Success  200 {array} domain.AutopilotSend
// @Router   /autopilot/sends [get]
func (h *AutopilotHandler) HandleListSends(c *gin.Context) {
	if h.unavailable(c) {
		return
	}
	limit, offset := pagination(c)
	list, err := h.repo.ListSends(c.Request.Context(), getAccountID(c), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	if list == nil {
		list = []*domain.AutopilotSend{}
	}
	c.JSON(http.StatusOK, list)
}

// HandleDismissOffer serves POST /v1/autopilot/dismiss-offer.
//
// Stamps the one time we ask, so the invitation is not nagged.
//
// @Summary  Dismiss the autopilot invitation
// @Tags     autopilot
// @Success  204
// @Router   /autopilot/dismiss-offer [post]
func (h *AutopilotHandler) HandleDismissOffer(c *gin.Context) {
	if h.unavailable(c) {
		return
	}
	if err := h.repo.MarkOffered(c.Request.Context(), getAccountID(c), time.Now()); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// startOfToday mirrors the worker's window: the ACCOUNT's own midnight, so
// the number on the screen is the same number the worker enforces.
func (h *AutopilotHandler) startOfToday(c *gin.Context, accountID string) time.Time {
	var tz string
	if h.accounts != nil {
		if account, err := h.accounts.GetByID(c.Request.Context(), accountID); err == nil && account != nil {
			tz = account.Timezone
		}
	}
	return domain.StartOfDayIn(time.Now(), tz)
}

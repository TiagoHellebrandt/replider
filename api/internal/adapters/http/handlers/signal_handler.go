package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Buying signals (§2.1).
//
// A hospital that publishes a tender has said in writing that it intends to
// buy a specific thing by a specific date. The feed exists to turn that into
// an approach while it is still open — which is why every payload carries the
// deadline, and why a closed tender never appears.
type SignalHandler struct {
	repo domain.SignalRepository
	// health is optional (SetSourceHealth): unwired, the sources route
	// answers an empty list and the screen draws no banner.
	health domain.SignalSourceHealthRepository
	// calendar is optional (SetContractCalendar): nil answers 503 rather
	// than a nil dereference mid-request.
	calendar *usecases.ContractCalendarUseCase
	// workWindow is optional (SetWorkContractWindow): without it the calendar
	// is a read-only list.
	workWindow *usecases.WorkContractWindowUseCase
	// workSignal is optional (SetWorkSignal): without it an open tender can
	// only be read or dismissed, never chased by hand.
	workSignal *usecases.SignalOfferUseCase
}

// SetWorkSignal wires the manual "I will chase this one myself" door.
func (h *SignalHandler) SetWorkSignal(uc *usecases.SignalOfferUseCase) {
	h.workSignal = uc
}

// HandleWorkSignal serves POST /v1/signals/:id/work.
//
// The door a seller had no button for. The only path from a tender to a card
// was the automatic offer, which always spends an AI draft on the account's own
// key — so somebody who wanted to work it by hand could read the notice (and
// leave the product), open a draft that might not exist, or dismiss it.
// Meanwhile an expiring CONTRACT could already be worked by hand, which made
// the asymmetry visible on the same screen.
//
// Nothing is drafted and nothing is sent: the buyer lands in the customer list
// with the tender attached, and the card asks for the one thing a public notice
// never publishes — who to call.
// @Summary     Put an open tender in the funnel, without drafting
// @Description Creates (or returns) the lead for this tender in the default funnel. No AI call, no message. Idempotent: a tender already worked returns its existing card.
// @Tags        Signals
// @Produce     json
// @Param       id path string true "Signal id"
// @Success     200 {object} domain.Lead
// @Failure     404 {object} domain.AppError
// @Failure     503 {object} domain.AppError
// @Security    BearerAuth
// @Router      /signals/{id}/work [post]
func (h *SignalHandler) HandleWorkSignal(c *gin.Context) {
	if h.workSignal == nil {
		respondError(c, domain.ErrContractCalendarUnavailable)
		return
	}
	lead, err := h.workSignal.WorkSignal(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, lead)
}

func NewSignalHandler(repo domain.SignalRepository) *SignalHandler {
	return &SignalHandler{repo: repo}
}

// SetSourceHealth wires the source-health reader (optional setter, so the
// constructor and router.Setup signatures stay untouched).
func (h *SignalHandler) SetSourceHealth(r domain.SignalSourceHealthRepository) {
	h.health = r
}

// SetWorkContractWindow wires "put this one in the funnel" (optional; nil
// answers 503).
func (h *SignalHandler) SetWorkContractWindow(uc *usecases.WorkContractWindowUseCase) {
	h.workWindow = uc
}

// HandleWorkContractWindow serves POST /v1/contract-calendar/:id/work.
//
// Turns a calendar row into a pipeline card. Nothing is drafted and nothing is
// sent: a public body published on PNCP has no e-mail and no phone, so the
// card arrives asking for the one thing it needs — the buyer's contact — which
// is exactly the work that has to happen in October for a contract that ends
// in January.
// @Summary     Work an expiring commitment
// @Description Creates (or returns) the lead for this contract's buyer in the default funnel, carrying the contract's facts. Sends nothing.
// @Tags        Signals
// @Produce     json
// @Param       id path string true "PNCP control number"
// @Success     200 {object} domain.Lead
// @Failure     404 {object} domain.AppError
// @Failure     503 {object} domain.AppError
// @Security    BearerAuth
// @Router      /contract-calendar/{id}/work [post]
func (h *SignalHandler) HandleWorkContractWindow(c *gin.Context) {
	if h.workWindow == nil {
		respondError(c, domain.ErrContractCalendarUnavailable)
		return
	}
	lead, err := h.workWindow.Execute(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, lead)
}

// SetContractCalendar wires the buying calendar (optional; nil answers 503).
func (h *SignalHandler) SetContractCalendar(uc *usecases.ContractCalendarUseCase) {
	h.calendar = uc
}

// HandleContractCalendar serves GET /v1/contract-calendar.
//
// What is about to be bought again, by whom, and when — the only read in this
// product a salesperson could not assemble by hand. A public contract carries
// a published end date, so the purchase that replaces it is a known event
// months before any tender exists.
// @Summary     Commitments about to expire in this territory
// @Description Public contracts and price registries in the account's state whose term runs out inside the window, soonest first, narrowed by the same words the radar watches. Every row links to the published record.
// @Tags        Signals
// @Produce     json
// @Param       window_days query int false "How far ahead to look (default 180, max 720)"
// @Success     200 {object} usecases.ContractCalendar
// @Failure     401 {object} domain.AppError
// @Failure     503 {object} domain.AppError
// @Security    BearerAuth
// @Router      /contract-calendar [get]
func (h *SignalHandler) HandleContractCalendar(c *gin.Context) {
	if h.calendar == nil {
		respondError(c, domain.ErrContractCalendarUnavailable)
		return
	}
	window, _ := strconv.Atoi(c.Query("window_days"))
	out, err := h.calendar.Execute(c.Request.Context(), getAccountID(c), window)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// signalResponse adds the one derived fact the card is built around.
type signalResponse struct {
	*domain.Signal
	// DaysLeft is nil when the source published no deadline. Three days is a
	// reason to write today; thirty is a reason to prepare properly, and the
	// difference is the whole value of the signal.
	DaysLeft *int `json:"days_left,omitempty"`
}

// HandleList serves GET /v1/signals.
//
// @Summary  List open buying signals
// @Tags     signals
// @Produce  json
// @Success  200 {array} signalResponse
// @Router   /signals [get]
func (h *SignalHandler) HandleList(c *gin.Context) {
	limit, offset := pagination(c)
	list, err := h.repo.List(c.Request.Context(), getAccountID(c), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	now := time.Now()
	out := make([]signalResponse, 0, len(list))
	for _, s := range list {
		out = append(out, signalResponse{Signal: s, DaysLeft: s.DaysLeft(now)})
	}
	c.JSON(http.StatusOK, out)
}

// sourceHealthResponse is what the radar needs to date its own feed, and
// nothing more. The source's error text stays server-side: a seller needs to
// know the feed is stale, not that page 20 of modality 8 returned 429.
type sourceHealthResponse struct {
	Source string `json:"source"`
	// Reachable is false when the last poll could not read the source at
	// all. The screen says so instead of presenting stored tenders as today's.
	Reachable      bool       `json:"reachable"`
	LastAttemptAt  time.Time  `json:"last_attempt_at"`
	LastSuccessAt  *time.Time `json:"last_success_at,omitempty"`
	FailedAttempts int        `json:"failed_attempts"`
}

// HandleListSources serves GET /v1/signal-sources.
//
// Exists because the radar could not tell an outage from a quiet week: it
// renders what is stored, so a portal refusing every request looked exactly
// like a portal with nothing new. The screen needs a fact it cannot derive
// from the signals themselves — a source can be perfectly healthy and publish
// nothing for days.
//
// @Summary  Health of the buying-signal sources
// @Tags     signals
// @Produce  json
// @Success  200 {array} sourceHealthResponse
// @Router   /signal-sources [get]
func (h *SignalHandler) HandleListSources(c *gin.Context) {
	out := make([]sourceHealthResponse, 0, 2)
	if h.health == nil {
		// Unwired: say nothing rather than something wrong. An empty list
		// renders no banner, which is the pre-feature behaviour.
		c.JSON(http.StatusOK, out)
		return
	}
	list, err := h.health.ListHealth(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	for _, s := range list {
		out = append(out, sourceHealthResponse{
			Source:         string(s.Source),
			Reachable:      !s.Unreachable(),
			LastAttemptAt:  s.LastAttemptAt,
			LastSuccessAt:  s.LastSuccessAt,
			FailedAttempts: s.ConsecutiveFailures,
		})
	}
	c.JSON(http.StatusOK, out)
}

// HandleDismiss serves POST /v1/signals/:id/dismiss.
//
// Recorded rather than deleted, so the next poll does not bring the same
// notice straight back.
//
// @Summary  Dismiss a buying signal
// @Tags     signals
// @Param    id path string true "Signal ID"
// @Success  204
// @Router   /signals/{id}/dismiss [post]
func (h *SignalHandler) HandleDismiss(c *gin.Context) {
	if err := h.repo.Dismiss(c.Request.Context(), c.Param("id"),
		getAccountID(c), time.Now()); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ── Watches ───────────────────────────────────────────────────────────────

type watchRequest struct {
	ID     string `json:"id,omitempty"`
	Source string `json:"source"`
	// Keywords match the tender's object. ANY match counts — a supplier of
	// monitors sells to "monitor multiparâmetro" and to "monitorização", and
	// demanding every term would find nothing.
	Keywords      []string `json:"keywords"`
	CNAEs         []string `json:"cnaes"`
	States        []string `json:"states"`
	MinValueCents int64    `json:"min_value_cents"`
	IsActive      *bool    `json:"is_active"`
}

// HandleListWatches serves GET /v1/signal-watches.
//
// @Summary  List signal watches
// @Tags     signals
// @Produce  json
// @Success  200 {array} domain.SignalWatch
// @Router   /signal-watches [get]
func (h *SignalHandler) HandleListWatches(c *gin.Context) {
	list, err := h.repo.ListWatches(c.Request.Context(), getAccountID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	if list == nil {
		list = []*domain.SignalWatch{}
	}
	c.JSON(http.StatusOK, list)
}

// HandleUpsertWatch serves PUT /v1/signal-watches/:id ("new" creates).
//
// @Summary  Create or update a signal watch
// @Tags     signals
// @Accept   json
// @Produce  json
// @Param    id path string true "Watch ID, or 'new'"
// @Success  200 {object} domain.SignalWatch
// @Failure  400 {object} domain.AppError
// @Router   /signal-watches/{id} [put]
func (h *SignalHandler) HandleUpsertWatch(c *gin.Context) {
	var req watchRequest
	if !bindJSON(c, &req) {
		return
	}
	source := domain.SignalSource(strings.ToLower(strings.TrimSpace(req.Source)))
	switch source {
	case domain.SignalPNCP, domain.SignalNewCompany, domain.SignalJobPost:
	default:
		respondError(c, &domain.AppError{
			Code: "VALIDATION_ERROR", Message: "fonte de sinal desconhecida"})
		return
	}

	id := c.Param("id")
	if id == "" || id == "new" {
		id = ulid.Make().String()
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}

	watch := &domain.SignalWatch{
		ID: id, AccountID: getAccountID(c), Source: source,
		Keywords: req.Keywords, CNAEs: req.CNAEs, States: req.States,
		MinValueCents: req.MinValueCents, IsActive: active,
	}
	// One rule, three writers: this route, the MCP tool and the seeder.
	if err := watch.NormalizeAndValidate(); err != nil {
		respondError(c, err)
		return
	}
	if err := h.repo.UpsertWatch(c.Request.Context(), watch); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, watch)
}

// HandleDeleteWatch serves DELETE /v1/signal-watches/:id.
//
// @Summary  Delete a signal watch
// @Tags     signals
// @Param    id path string true "Watch ID"
// @Success  204
// @Router   /signal-watches/{id} [delete]
func (h *SignalHandler) HandleDeleteWatch(c *gin.Context) {
	if err := h.repo.DeleteWatch(c.Request.Context(), c.Param("id"),
		getAccountID(c)); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

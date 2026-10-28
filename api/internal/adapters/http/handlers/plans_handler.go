package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// PlansHandler handles HTTP requests for Plans
type PlansHandler struct {
	planRepo    domain.PlanRepository
	accountRepo domain.AccountRepository
	bookingURL  string
}

// NewPlansHandler creates a new PlansHandler. accountRepo resolves the
// account's billing currency on authenticated requests (US-002); nil
// degrades to query-param/default filtering only.
func NewPlansHandler(planRepo domain.PlanRepository, accountRepo domain.AccountRepository) *PlansHandler {
	return &PlansHandler{planRepo: planRepo, accountRepo: accountRepo}
}

// SetBookingURL wires the calendar page a prospect books a call on
// (BOOKING_URL). Optional: empty leaves the contact channels as they were.
func (h *PlansHandler) SetBookingURL(u string) { h.bookingURL = u }

// HandleListPlans godoc
// @Summary      List available Plans
// @Description  Returns the SaaS subscription plans available for purchase, filtered by billing currency. Anonymous callers pick the currency via `?currency=` (default `brl`); authenticated callers are always pinned to their account's currency, so the dashboard never offers a plan the account cannot buy. Used by Flutter and the marketing site to populate pricing.
//
//	Plans whose `stripe_price_id` is omitted are sold under contract — the dashboard renders a "Falar com especialista" CTA for those instead of a Stripe checkout button.
//
// @Tags         plans
// @Produce      json
// @Param        currency  query  string  false  "Billing currency to list plans for (ignored when authenticated)"  Enums(brl, usd)  default(brl)
// @Success      200  {array}   domain.Plan         "List of available plans in the resolved currency"
// @Failure      400  {object}  domain.AppError     "Invalid currency"
// @Failure      500  {object}  domain.AppError     "Internal server error"
// @Router       /plans [get]
func (h *PlansHandler) HandleListPlans(c *gin.Context) {
	currency := strings.ToLower(c.Query("currency"))
	switch currency {
	case "", domain.CurrencyBRL, domain.CurrencyUSD:
	default:
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_CURRENCY", Message: "currency must be 'brl' or 'usd'"})
		return
	}

	// Authenticated requests are pinned to the account's currency —
	// the query param must not let a BRL account browse (and attempt
	// to check out) USD plans or vice versa. Currency resolution is
	// best-effort: a lookup failure falls back to the anonymous path.
	if accID := getAccountID(c); accID != "" && h.accountRepo != nil {
		if acc, err := h.accountRepo.GetByID(c.Request.Context(), accID); err == nil && acc != nil && acc.Currency != "" {
			currency = strings.ToLower(acc.Currency)
		}
	}
	if currency == "" {
		currency = domain.CurrencyBRL
	}

	plans, err := h.planRepo.ListAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to list plans"})
		return
	}

	// In-memory filter over the (cached) full listing — the plans table
	// is ~10 rows, not worth a per-currency query + cache key. Unlisted
	// plans (is_listed = false, e.g. the retired Partner plan) never
	// reach the public catalogue; accounts already on one keep working
	// via GetPlanByID.
	filtered := make([]*domain.Plan, 0, len(plans))
	for _, p := range plans {
		if !p.IsListed {
			continue
		}
		pc := strings.ToLower(p.Currency)
		if pc == "" {
			pc = domain.CurrencyBRL
		}
		if pc == currency {
			filtered = append(filtered, p)
		}
	}

	c.JSON(http.StatusOK, filtered)
}

// HandleContactChannels godoc
// @Summary      Get the contact-sales channels
// @Description  Returns the email + WhatsApp number the dashboard uses for the "Falar com especialista" CTA on contract plans (Partner / Enterprise). Public so the marketing site can also embed it.
// @Tags         plans
// @Produce      json
// @Success      200  {object}  domain.ContactSalesChannels
// @Router       /plans/contact-channels [get]
func (h *PlansHandler) HandleContactChannels(c *gin.Context) {
	ch := domain.DefaultContactSales()
	ch.BookingURL = h.bookingURL
	c.JSON(http.StatusOK, ch)
}

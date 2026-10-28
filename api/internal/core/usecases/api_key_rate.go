package usecases

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Per-plan API rate (§6.19, J1).
//
// The rate is DERIVED from the plan's own conversation allowance rather than
// stored in a column of its own. A dedicated column would be one more number to
// seed on every plan — and the failure mode of forgetting it is a plan that
// silently gets the default, which is exactly the trap migration 000184 already
// documented for the spend caps. Deriving it means a plan cannot be inconsistent
// with itself: a bigger plan is proportionally faster, always.
type PlanAPIRateResolver struct {
	accounts domain.AccountRepository
	plans    domain.PlanRepository

	mu    sync.RWMutex
	cache map[string]cachedRate
	now   func() time.Time
}

type cachedRate struct {
	rpm       int
	expiresAt time.Time
}

func NewPlanAPIRateResolver(
	accounts domain.AccountRepository, plans domain.PlanRepository,
) *PlanAPIRateResolver {
	return &PlanAPIRateResolver{
		accounts: accounts,
		plans:    plans,
		cache:    map[string]cachedRate{},
		now:      time.Now,
	}
}

// API rate bounds.
const (
	// MinAPIRequestsPerMinute is what even the smallest plan gets. Sixty is one
	// request a second — below that an ordinary sync job looks broken.
	MinAPIRequestsPerMinute = 60
	// MaxAPIRequestsPerMinute is the ceiling. Past this the honest answer is a
	// conversation about a bulk endpoint, not a faster loop.
	MaxAPIRequestsPerMinute = 600
	// apiRateCacheTTL keeps the plan lookup off the hot path. Five minutes: an
	// upgrade takes effect within one, and nobody notices.
	apiRateCacheTTL = 5 * time.Minute
)

// RequestsPerMinute answers the middleware.
//
// Returns 0 on any failure, which the middleware reads as "use the default"
// rather than as "block": a plan lookup problem must not take a paying
// customer's integration down.
func (r *PlanAPIRateResolver) RequestsPerMinute(ctx context.Context, accountID string) int {
	if r == nil || r.accounts == nil || r.plans == nil || accountID == "" {
		return 0
	}

	r.mu.RLock()
	entry, ok := r.cache[accountID]
	r.mu.RUnlock()
	if ok && entry.expiresAt.After(r.now()) {
		return entry.rpm
	}

	account, err := r.accounts.GetByID(ctx, accountID)
	if err != nil || account == nil {
		slog.WarnContext(ctx, "api rate: account lookup failed, using the default",
			"account_id", accountID, "error", err)
		return 0
	}
	plan := domain.TrialPlan()
	if account.PlanID != nil && *account.PlanID != "" {
		if p, err := r.plans.GetByID(ctx, *account.PlanID); err == nil && p != nil {
			plan = p
		}
	}

	rpm := deriveAPIRate(account.EffectiveMaxConversationsPerMonth(plan))

	r.mu.Lock()
	r.cache[accountID] = cachedRate{rpm: rpm, expiresAt: r.now().Add(apiRateCacheTTL)}
	r.mu.Unlock()
	return rpm
}

// deriveAPIRate turns a monthly conversation allowance into a per-minute API
// ceiling.
//
// One hundredth of the monthly allowance, clamped. The ratio is arbitrary in the
// way any first constant is, but the SHAPE is not: an account allowed ten
// thousand conversations a month is doing roughly ten times the business of one
// allowed a thousand, and its integration should be allowed to move roughly ten
// times as fast.
func deriveAPIRate(monthlyConversations int) int {
	if monthlyConversations <= 0 {
		// An unlimited or unset allowance gets the ceiling rather than the
		// floor: a contract plan with no stated limit is the biggest customer,
		// not the smallest.
		return MaxAPIRequestsPerMinute
	}
	rpm := monthlyConversations / 100
	if rpm < MinAPIRequestsPerMinute {
		return MinAPIRequestsPerMinute
	}
	if rpm > MaxAPIRequestsPerMinute {
		return MaxAPIRequestsPerMinute
	}
	return rpm
}

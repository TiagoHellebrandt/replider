package usecases

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// defaultAIMonthlyBudgetCents is the platform-wide ceiling applied to every
// account whose `ai_monthly_budget_cents` is NULL (BYOK cost model: "teto
// auto-definido com corte duro"). Zero = no default, i.e. a NULL column means
// no ceiling — the behaviour before AI_DEFAULT_MONTHLY_BUDGET_CENTS existed.
// main.go sets it from config once at boot; tests set it explicitly.
var defaultAIMonthlyBudgetCents atomic.Int64

// SetDefaultAIMonthlyBudgetCents wires the platform default ceiling
// (AI_DEFAULT_MONTHLY_BUDGET_CENTS). Negative values are treated as zero.
func SetDefaultAIMonthlyBudgetCents(cents int64) {
	if cents < 0 {
		cents = 0
	}
	defaultAIMonthlyBudgetCents.Store(cents)
}

// DefaultAIMonthlyBudgetCents reports the platform default ceiling (0 = none).
func DefaultAIMonthlyBudgetCents() int64 { return defaultAIMonthlyBudgetCents.Load() }

// Budget sources, reported by GET /me/ai-costs so the UI can say "seu teto"
// vs "teto padrão da plataforma".
const (
	BudgetSourceCustom  = "custom"
	BudgetSourceDefault = "default"
	BudgetSourceNone    = "none"
)

// EffectiveAIBudget resolves the ceiling that actually applies to an
// account: its own column when set (> 0), else the platform default, else
// none. Single resolver so the gate and the report cannot disagree.
func EffectiveAIBudget(account *domain.Account) (cents int64, source string) {
	if account != nil && account.AIMonthlyBudgetCents != nil && *account.AIMonthlyBudgetCents > 0 {
		return *account.AIMonthlyBudgetCents, BudgetSourceCustom
	}
	if d := DefaultAIMonthlyBudgetCents(); d > 0 {
		return d, BudgetSourceDefault
	}
	return 0, BudgetSourceNone
}

// estimatedDraftCentsFallback is the per-unit cost assumed when the account
// has no ledger history to average yet (R$0,15 — the top of the measured
// "R$0,05–0,15 por oferta" range, so the estimate errs toward allowing
// fewer units, never more).
const estimatedDraftCentsFallback int64 = 15

// The one spend gate (§6.14, K4).
//
// Every capped AI operation runs through `ensure`. One implementation rather
// than a copy of `checkBroadcastBudget` per feature, because the interesting
// decisions here — what the window is, which way a lookup failure errs, whether
// a batch stops or skips — must be the same everywhere or they are not policy,
// just four accidents.
type spendGate struct {
	accounts domain.AccountRepository
	plans    domain.PlanRepository
	counter  domain.SpendCounter
	// costs backs the CUSTOMER-set monthly budget (BYOK cost model): the
	// same ledger the invoice line reads, summed in BRL at the platform
	// rate. Optional — unwired skips the budget check.
	costs domain.AICostRepository
	now   func() time.Time
}

func newSpendGate(
	accounts domain.AccountRepository,
	plans domain.PlanRepository,
	counter domain.SpendCounter,
) *spendGate {
	return &spendGate{accounts: accounts, plans: plans, counter: counter, now: time.Now}
}

// spendAllowance is what the gate learned about one account and kind.
type spendAllowance struct {
	Cap      int
	Used     int64
	ResetsAt time.Time
}

// Remaining is what is left, floored at zero.
func (a spendAllowance) Remaining() int64 {
	left := int64(a.Cap) - a.Used
	if left < 0 {
		return 0
	}
	return left
}

// ensure refuses the operation when the account's allowance is spent.
//
// Two deliberate ways this errs:
//
//   - An UNWIRED gate (nil repos) allows. The caps are a margin control, not a
//     security boundary, and a deployment that forgot to wire one must keep
//     working rather than stop drafting for everybody.
//   - A FAILED lookup allows, loudly. Blocking a paying customer because the
//     plan table had a hiccup is a worse outcome than one uncapped draft — the
//     spend is bounded anyway by the daily token rail, and the cost ledger
//     records what actually happened either way.
//
// The one thing it never does is allow silently: every degradation logs.
func (g *spendGate) ensure(ctx context.Context, accountID string, kind domain.SpendKind) error {
	allowance, err := g.allowance(ctx, accountID, kind)
	if err != nil {
		// Already logged in allowance(). Errs open, on purpose.
		return nil
	}
	if allowance == nil {
		return nil
	}
	if allowance.Cap <= 0 {
		return domain.ErrSpendNotAllowed(kind)
	}
	if allowance.Used >= int64(allowance.Cap) {
		return domain.ErrSpendExceeded(kind, int(allowance.Used), allowance.Cap, allowance.ResetsAt)
	}
	return g.ensureBudget(ctx, accountID)
}

// ensureBudget enforces the account's OWN ceiling (ai_monthly_budget_cents):
// the customer said "no more than R$ X of AI this month", and crossing it
// refuses every further paid generation until the cycle resets. Same
// err-open posture as the plan caps — a ledger hiccup must not stop the
// product — but a ceiling actually crossed is a hard no.
func (g *spendGate) ensureBudget(ctx context.Context, accountID string) error {
	if g == nil || g.costs == nil || g.accounts == nil {
		return nil
	}
	account, err := g.accounts.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return nil
	}
	return g.ensureBudgetFor(ctx, account)
}

// ensureBudgetFor is ensureBudget for a caller that already holds the
// account row (saves the lookup on the hot inbound path).
func (g *spendGate) ensureBudgetFor(ctx context.Context, account *domain.Account) error {
	if g == nil || g.costs == nil || account == nil {
		return nil
	}
	budget, _ := EffectiveAIBudget(account)
	if budget <= 0 {
		return nil
	}
	from, resetsAt := domain.SpendWindow(g.now(), account.Timezone)
	sum, err := g.costs.SumByAccount(ctx, account.ID, from, g.now())
	if err != nil || sum == nil {
		slog.WarnContext(ctx, "spend gate: budget sum failed; allowing", "account_id", account.ID, "error", err)
		return nil
	}
	if sum.TotalBRLCents >= budget {
		return domain.ErrAIBudgetExceeded(sum.TotalBRLCents, budget, resetsAt)
	}
	return nil
}

// ensureAny is the budget-only check for AI callers that have no per-kind
// plan cap (agent_reply, copilot, brief, review, offer_match, enrichment):
// the customer's ceiling (or the platform default) still applies to them.
// Nil-safe: an unwired gate allows, same err-open posture as `ensure`.
func (g *spendGate) ensureAny(ctx context.Context, accountID string) error {
	return g.ensureBudget(ctx, accountID)
}

// ensureAnyFor is ensureAny with the account already in hand.
func (g *spendGate) ensureAnyFor(ctx context.Context, account *domain.Account) error {
	return g.ensureBudgetFor(ctx, account)
}

// budgetRoom is the batch counterpart of ensureBudget: how many more units of
// `kind` the ceiling still affords. `applies` is false when there is no
// ceiling or the lookup degraded (err-open). The per-unit estimate is the
// account's OWN ledger average over the window (cycle spend ÷ units of this
// kind so far); with no history it falls back to estimatedDraftCentsFallback.
// A ceiling already crossed returns (0, true, ErrAIBudgetExceeded).
func (g *spendGate) budgetRoom(ctx context.Context, accountID string, usedOfKind int64) (room int, applies bool, err error) {
	if g == nil || g.costs == nil || g.accounts == nil {
		return 0, false, nil
	}
	account, lookupErr := g.accounts.GetByID(ctx, accountID)
	if lookupErr != nil || account == nil {
		return 0, false, nil
	}
	budget, _ := EffectiveAIBudget(account)
	if budget <= 0 {
		return 0, false, nil
	}
	from, resetsAt := domain.SpendWindow(g.now(), account.Timezone)
	sum, sumErr := g.costs.SumByAccount(ctx, accountID, from, g.now())
	if sumErr != nil || sum == nil {
		slog.WarnContext(ctx, "spend gate: budget sum failed; allowing batch", "account_id", accountID, "error", sumErr)
		return 0, false, nil
	}
	remainingCents := budget - sum.TotalBRLCents
	if remainingCents <= 0 {
		return 0, true, domain.ErrAIBudgetExceeded(sum.TotalBRLCents, budget, resetsAt)
	}
	perUnit := estimatedDraftCentsFallback
	if usedOfKind > 0 && sum.TotalBRLCents > 0 {
		perUnit = sum.TotalBRLCents / usedOfKind
	}
	if perUnit <= 0 {
		perUnit = 1
	}
	room = int(remainingCents / perUnit)
	if room < 1 {
		// Something is still affordable; a batch that gets 0 with money
		// left would read as "budget exhausted", which it is not.
		room = 1
	}
	return room, true, nil
}

// SetCostLedger wires the budget check (optional).
func (g *spendGate) SetCostLedger(costs domain.AICostRepository) { g.costs = costs }

// budgetGateFromMeter builds a budget-only gate for an AI caller that has no
// per-kind plan cap. The meter main.go wires already holds the cost ledger,
// so every `SetAIMeter` lights the customer's ceiling up with no extra
// wiring. Nil when there is nothing to check against (nil gate = allows).
func budgetGateFromMeter(accounts domain.AccountRepository, m *AIMeter) *spendGate {
	if m == nil || m.repo == nil || accounts == nil {
		return nil
	}
	g := newSpendGate(accounts, nil, nil)
	g.SetCostLedger(m.repo)
	return g
}

// allowance resolves the cap and the usage. Returns (nil, nil) when the gate is
// not wired.
func (g *spendGate) allowance(ctx context.Context, accountID string, kind domain.SpendKind) (*spendAllowance, error) {
	if g == nil || g.accounts == nil || g.plans == nil || g.counter == nil {
		return nil, nil
	}

	account, err := g.accounts.GetByID(ctx, accountID)
	if err != nil || account == nil {
		slog.WarnContext(ctx, "spend cap: account lookup failed, allowing",
			"kind", kind, "account_id", accountID, "error", err)
		return nil, errSpendLookup
	}

	plan := domain.TrialPlan()
	if account.PlanID != nil && *account.PlanID != "" {
		p, err := g.plans.GetByID(ctx, *account.PlanID)
		if err != nil || p == nil {
			slog.WarnContext(ctx, "spend cap: plan lookup failed, allowing",
				"kind", kind, "account_id", accountID, "error", err)
			return nil, errSpendLookup
		}
		plan = p
	}

	tz := domain.FallbackTimezone
	if account.Timezone != "" {
		tz = account.Timezone
	}
	from, resetsAt := domain.SpendWindow(g.now(), tz)

	used, err := g.counter.CountOperations(ctx, accountID, domain.SpendOperations(kind), from)
	if err != nil {
		slog.WarnContext(ctx, "spend cap: usage count failed, allowing",
			"kind", kind, "account_id", accountID, "error", err)
		return nil, errSpendLookup
	}

	return &spendAllowance{
		Cap:      domain.SpendCapFor(kind, account, plan),
		Used:     used,
		ResetsAt: resetsAt,
	}, nil
}

// errSpendLookup marks a degradation, never surfaced to a caller: `ensure`
// swallows it and allows.
var errSpendLookup = &domain.AppError{Code: "SPEND_CAP_LOOKUP_FAILED"}

// remainingFor is the batch path's version of the check: how many more of this
// kind may run right now.
//
// A batch must not check once and then spend a hundred units — nor abort the
// whole call on the unit that crosses the line, which would waste the work
// already done. It takes what fits and REPORTS what it skipped, because a batch
// that silently returns 40 of 100 reads as "only 40 were possible".
//
// An unwired or degraded gate returns the requested count unchanged: same
// err-open rule as `ensure`.
func (g *spendGate) remainingFor(ctx context.Context, accountID string, kind domain.SpendKind, wanted int) (allowed int, err error) {
	allowance, lookupErr := g.allowance(ctx, accountID, kind)
	if lookupErr != nil || allowance == nil {
		return wanted, nil
	}
	if allowance.Cap <= 0 {
		return 0, domain.ErrSpendNotAllowed(kind)
	}
	remaining := allowance.Remaining()
	if remaining == 0 {
		return 0, domain.ErrSpendExceeded(kind, int(allowance.Used), allowance.Cap, allowance.ResetsAt)
	}
	if int64(wanted) > remaining {
		wanted = int(remaining)
	}
	// The customer's ceiling bounds the batch too. Before this, only the
	// single-draft path (`ensure`) ever looked at the budget, so an overnight
	// batch could blow through it in one night.
	room, applies, err := g.budgetRoom(ctx, accountID, allowance.Used)
	if err != nil {
		return 0, err
	}
	if applies && room < wanted {
		return room, nil
	}
	return wanted, nil
}

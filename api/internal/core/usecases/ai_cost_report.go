package usecases

import (
	"context"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// The AI cost report (BYOK cost model): what this account has actually
// spent this cycle, what a month looks like at its own pace, and where its
// self-set ceiling sits. The projection comes from the account's OWN ledger
// — never a platform average dressed up as a promise; a brand-new account
// honestly gets "sem histórico ainda".

// AICostReport is the answer for the billing screen and the sales call.
type AICostReport struct {
	// Cycle is the current account-month spend (the number the budget is
	// compared against).
	Cycle *domain.AICostSummary `json:"cycle"`
	// Last30d is the observation window the projection is built on.
	Last30d *domain.AICostSummary `json:"last_30d"`
	// ProjectedMonthlyBRLCents extrapolates the last 30 days to a month.
	// Zero when there is no history — rendered as "—", never as "R$ 0 de
	// custo garantido".
	ProjectedMonthlyBRLCents int64 `json:"projected_monthly_brl_cents"`
	// BudgetCents is the ceiling in force (nil = none): the customer's own
	// or the platform default — BudgetSource says which.
	BudgetCents *int64 `json:"budget_cents,omitempty"`
	// BudgetSource is "custom" (accounts.ai_monthly_budget_cents), "default"
	// (AI_DEFAULT_MONTHLY_BUDGET_CENTS) or "none".
	BudgetSource string `json:"budget_source"`
	// BudgetUsedPct is cycle spend over the ceiling, 0–100+ (may exceed
	// 100 when the ceiling was lowered mid-cycle).
	BudgetUsedPct *int `json:"budget_used_pct,omitempty"`
	// ResetsAt is when the cycle window rolls.
	ResetsAt time.Time `json:"resets_at"`
}

type AICostReportUseCase struct {
	accounts domain.AccountRepository
	costs    domain.AICostRepository
	budget   aiBudgetWriter
	now      func() time.Time
}

func NewAICostReportUseCase(accounts domain.AccountRepository, costs domain.AICostRepository) *AICostReportUseCase {
	return &AICostReportUseCase{accounts: accounts, costs: costs, now: time.Now}
}

// WithClock injects a fake clock for tests.
func (uc *AICostReportUseCase) WithClock(now func() time.Time) *AICostReportUseCase {
	uc.now = now
	return uc
}

func (uc *AICostReportUseCase) Execute(ctx context.Context, accountID string) (*AICostReport, error) {
	account, err := uc.accounts.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, domain.ErrAccountNotFound
	}
	now := uc.now()
	from, resetsAt := domain.SpendWindow(now, account.Timezone)

	cycle, err := uc.costs.SumByAccount(ctx, accountID, from, now)
	if err != nil {
		return nil, err
	}
	last30, err := uc.costs.SumByAccount(ctx, accountID, now.AddDate(0, 0, -30), now)
	if err != nil {
		return nil, err
	}

	report := &AICostReport{Cycle: cycle, Last30d: last30, ResetsAt: resetsAt}
	if last30 != nil && last30.TotalBRLCents > 0 {
		// 30 observed days ≈ one month; the point is an order of magnitude
		// the owner can sanity-check, not an invoice.
		report.ProjectedMonthlyBRLCents = last30.TotalBRLCents
	}
	// The ceiling that actually gates spend: the account's own when set,
	// else the platform default (AI_DEFAULT_MONTHLY_BUDGET_CENTS).
	budget, source := EffectiveAIBudget(account)
	report.BudgetSource = source
	if budget > 0 {
		report.BudgetCents = &budget
		if cycle != nil {
			pct := int(cycle.TotalBRLCents * 100 / budget)
			report.BudgetUsedPct = &pct
		}
	}
	return report, nil
}

// aiBudgetWriter is the narrow port for persisting the ceiling. Satisfied
// by *postgres.AccountRepository.
type aiBudgetWriter interface {
	SetAIMonthlyBudget(ctx context.Context, accountID string, cents *int64) error
}

// SetBudgetWriter wires the ceiling persistence (optional).
func (uc *AICostReportUseCase) SetBudgetWriter(w aiBudgetWriter) { uc.budget = w }

// SetBudget writes the customer's ceiling: nil or <=0 clears it.
func (uc *AICostReportUseCase) SetBudget(ctx context.Context, accountID string, cents *int64) error {
	if uc.budget == nil {
		return &domain.AppError{Code: "AI_BUDGET_UNAVAILABLE", Message: "budget storage not configured"}
	}
	if cents != nil && *cents <= 0 {
		cents = nil
	}
	return uc.budget.SetAIMonthlyBudget(ctx, accountID, cents)
}

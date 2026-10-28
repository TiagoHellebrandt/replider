package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

// The spend gate (K4).
//
// The interesting decisions are not the comparison, they are the failure modes:
// which way an unwired or broken gate errs, and what a batch does with the unit
// that crosses the line.

type fakeSpendCounter struct {
	count int64
	err   error
	// gotOps and gotSince record what was actually counted — the whole point of
	// the window test.
	gotOps   []string
	gotSince time.Time
}

func (f *fakeSpendCounter) CountOperations(_ context.Context, _ string, ops []string, since time.Time) (int64, error) {
	f.gotOps, f.gotSince = ops, since
	return f.count, f.err
}

func spendPlan(cap int) *domain.Plan {
	priceID := "price_selfserve"
	return &domain.Plan{
		ID:                  "plan-1",
		Name:                "Equipe",
		StripePriceID:       &priceID,
		MaxAIDraftsPerMonth: cap,
	}
}

func newGate(t *testing.T, counter domain.SpendCounter, plan *domain.Plan, accountErr error) *spendGate {
	t.Helper()
	planID := "plan-1"

	accounts := &MockAccountRepo{}
	if accountErr != nil {
		accounts.On("GetByID", mock.Anything, "acc").
			Return((*domain.Account)(nil), accountErr)
	} else {
		accounts.On("GetByID", mock.Anything, "acc").Return(&domain.Account{
			ID: "acc", PlanID: &planID, Timezone: "America/Sao_Paulo",
		}, nil)
	}

	plans := &MockPlanRepo{}
	plans.On("GetByID", mock.Anything, "plan-1").Return(plan, nil)

	gate := newSpendGate(accounts, plans, counter)
	gate.now = func() time.Time { return time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC) }
	return gate
}

func TestSpendGate_AllowsUnderTheCap(t *testing.T) {
	gate := newGate(t, &fakeSpendCounter{count: 999}, spendPlan(1000), nil)

	if err := gate.ensure(context.Background(), "acc", domain.SpendAIDraft); err != nil {
		t.Fatalf("999 of 1000 must pass: %v", err)
	}
}

func TestSpendGate_RefusesAtTheCapNotAfterIt(t *testing.T) {
	gate := newGate(t, &fakeSpendCounter{count: 1000}, spendPlan(1000), nil)

	err := gate.ensure(context.Background(), "acc", domain.SpendAIDraft)

	// 1000 used of 1000 means the NEXT one is over. Allowing it would sell 1001.
	var appErr *domain.AppError
	if !errors.As(err, &appErr) ||
		appErr.Code != domain.SpendExceededCode(domain.SpendAIDraft) {
		t.Fatalf("err = %v, want the exceeded code", err)
	}
}

func TestSpendGate_AZeroCapIsAnUpgradeConversationNotAnEmptyBudget(t *testing.T) {
	gate := newGate(t, &fakeSpendCounter{}, spendPlan(0), nil)

	err := gate.ensure(context.Background(), "acc", domain.SpendAIDraft)

	var appErr *domain.AppError
	if !errors.As(err, &appErr) ||
		appErr.Code != domain.SpendNotAllowedCode(domain.SpendAIDraft) {
		t.Fatalf("err = %v, want the not-allowed code — 'your plan does not "+
			"include this' is a different conversation from 'you used it up'", err)
	}
}

func TestSpendGate_CountsTheRightOperationsOverTheAccountsMonth(t *testing.T) {
	counter := &fakeSpendCounter{}
	gate := newGate(t, counter, spendPlan(10), nil)

	_ = gate.ensure(context.Background(), "acc", domain.SpendDocumentExtraction)

	// Every document-read route counts as one kind: the two contract paths
	// and the product-brochure read share the allowance.
	if len(counter.gotOps) != 3 {
		t.Fatalf("counted operations = %v, want the three document-read routes", counter.gotOps)
	}
	// And the window is the ACCOUNT's month: São Paulo's 1 July, which is
	// 03:00 UTC on the 1st.
	if counter.gotSince.Day() != 1 || counter.gotSince.Month() != time.July {
		t.Fatalf("window starts %v, want the account's 1 July", counter.gotSince)
	}
	if counter.gotSince.Location().String() != "America/Sao_Paulo" {
		t.Fatalf("window location = %v, want the account's", counter.gotSince.Location())
	}
}

func TestSpendGate_UnwiredAllows(t *testing.T) {
	// A margin control, not a security boundary: a deployment that forgot to
	// wire it must keep drafting rather than stop for everybody.
	var gate *spendGate
	if err := gate.ensure(context.Background(), "acc", domain.SpendAIDraft); err != nil {
		t.Fatalf("a nil gate must allow: %v", err)
	}

	partial := newSpendGate(nil, nil, nil)
	if err := partial.ensure(context.Background(), "acc", domain.SpendAIDraft); err != nil {
		t.Fatalf("an unwired gate must allow: %v", err)
	}
}

func TestSpendGate_ABrokenLookupAllowsRatherThanBlockingAPayingCustomer(t *testing.T) {
	cases := map[string]*spendGate{
		"account lookup fails": newGate(t, &fakeSpendCounter{}, spendPlan(10),
			errors.New("connection reset")),
		"usage count fails": newGate(t,
			&fakeSpendCounter{err: errors.New("statement timeout")}, spendPlan(10), nil),
	}
	for name, gate := range cases {
		t.Run(name, func(t *testing.T) {
			// Blocking a paying customer over a database hiccup is worse than
			// one uncapped draft: the spend is still bounded by the daily token
			// rail, and the cost ledger records what happened either way.
			if err := gate.ensure(context.Background(), "acc", domain.SpendAIDraft); err != nil {
				t.Fatalf("want allow, got %v", err)
			}
		})
	}
}

func TestSpendGate_BatchTakesWhatFits(t *testing.T) {
	// 990 of 1000 used, 25 wanted: ten fit.
	gate := newGate(t, &fakeSpendCounter{count: 990}, spendPlan(1000), nil)

	allowed, err := gate.remainingFor(context.Background(), "acc", domain.SpendAIDraft, 25)
	if err != nil {
		t.Fatalf("remainingFor: %v", err)
	}

	// Not 25 (which would oversell), and not 0 (which would waste the ten the
	// customer paid for).
	if allowed != 10 {
		t.Fatalf("allowed = %d, want 10", allowed)
	}
}

func TestSpendGate_BatchWithRoomToSpareTakesEverythingAsked(t *testing.T) {
	gate := newGate(t, &fakeSpendCounter{count: 5}, spendPlan(1000), nil)

	allowed, err := gate.remainingFor(context.Background(), "acc", domain.SpendAIDraft, 25)
	if err != nil {
		t.Fatalf("remainingFor: %v", err)
	}
	if allowed != 25 {
		t.Fatalf("allowed = %d, want the full 25", allowed)
	}
}

func TestSpendGate_BatchWithNothingLeftRefusesTheWholeCall(t *testing.T) {
	gate := newGate(t, &fakeSpendCounter{count: 1000}, spendPlan(1000), nil)

	allowed, err := gate.remainingFor(context.Background(), "acc", domain.SpendAIDraft, 25)

	// Zero allowed is a refusal with a reason, not a batch that quietly drafts
	// nothing and returns success.
	if err == nil {
		t.Fatal("want a refusal explaining why nothing was drafted")
	}
	if allowed != 0 {
		t.Fatalf("allowed = %d, want 0", allowed)
	}
}

func TestSpendGate_BatchOnABrokenLookupTakesWhatWasAsked(t *testing.T) {
	gate := newGate(t, &fakeSpendCounter{err: errors.New("timeout")}, spendPlan(1000), nil)

	allowed, err := gate.remainingFor(context.Background(), "acc", domain.SpendAIDraft, 25)

	// Same err-open rule as the single check.
	if err != nil {
		t.Fatalf("want allow, got %v", err)
	}
	if allowed != 25 {
		t.Fatalf("allowed = %d, want the requested 25", allowed)
	}
}

func TestSpendAllowance_RemainingNeverGoesNegative(t *testing.T) {
	// An over-consumed allowance (a cap lowered mid-cycle, a race) must report
	// zero left rather than a negative number a batch would treat as a size.
	a := spendAllowance{Cap: 10, Used: 25}

	if got := a.Remaining(); got != 0 {
		t.Fatalf("remaining = %d, want 0", got)
	}
}

// ---- customer-set monthly budget (BYOK cost model) ----

type fakeCostLedger struct {
	brlCents int64
	err      error
}

func (f fakeCostLedger) Record(context.Context, *domain.AICostEntry) error { return nil }
func (f fakeCostLedger) SumByAccount(context.Context, string, time.Time, time.Time) (*domain.AICostSummary, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &domain.AICostSummary{TotalBRLCents: f.brlCents}, nil
}
func (f fakeCostLedger) CountOperations(context.Context, string, []string, time.Time) (int64, error) {
	return 0, nil
}

func budgetGate(t *testing.T, spentCents int64, budgetCents *int64, ledgerErr error) *spendGate {
	t.Helper()
	planID := "plan-1"
	accounts := &MockAccountRepo{}
	accounts.On("GetByID", mock.Anything, "acc").Return(&domain.Account{
		ID: "acc", PlanID: &planID, Timezone: "America/Sao_Paulo",
		AIMonthlyBudgetCents: budgetCents,
	}, nil)
	plans := &MockPlanRepo{}
	plans.On("GetByID", mock.Anything, "plan-1").Return(spendPlan(1000), nil)

	gate := newSpendGate(accounts, plans, &fakeSpendCounter{count: 0})
	gate.SetCostLedger(fakeCostLedger{brlCents: spentCents, err: ledgerErr})
	gate.now = func() time.Time { return time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC) }
	return gate
}

func TestSpendGate_BudgetUnderTheCeilingAllows(t *testing.T) {
	budget := int64(10_000) // R$ 100
	gate := budgetGate(t, 5_000, &budget, nil)
	assert.NoError(t, gate.ensure(context.Background(), "acc", domain.SpendAIDraft))
}

func TestSpendGate_BudgetCrossedRefusesHard(t *testing.T) {
	budget := int64(10_000)
	gate := budgetGate(t, 10_000, &budget, nil)
	err := gate.ensure(context.Background(), "acc", domain.SpendAIDraft)
	require.Error(t, err)
	appErr := &domain.AppError{}
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "AI_BUDGET_EXCEEDED", appErr.Code)
	assert.Contains(t, appErr.Message, "R$ 100.00")
}

func TestSpendGate_NoBudgetSetSkipsTheCheck(t *testing.T) {
	gate := budgetGate(t, 999_999, nil, nil)
	assert.NoError(t, gate.ensure(context.Background(), "acc", domain.SpendAIDraft))
}

func TestSpendGate_BudgetLedgerErrorErrsOpen(t *testing.T) {
	budget := int64(10_000)
	gate := budgetGate(t, 0, &budget, errors.New("db down"))
	assert.NoError(t, gate.ensure(context.Background(), "acc", domain.SpendAIDraft),
		"a ledger hiccup must not stop the product; the daily token rail still bounds the spend")
}

package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

// The "auto-defined ceiling" (BYOK cost model): a NULL budget column used to
// mean no ceiling at all, and the batch path never looked at the budget.

func withDefaultBudget(t *testing.T, cents int64) {
	t.Helper()
	prev := DefaultAIMonthlyBudgetCents()
	SetDefaultAIMonthlyBudgetCents(cents)
	t.Cleanup(func() { SetDefaultAIMonthlyBudgetCents(prev) })
}

func TestSpendGate_DefaultBudgetAppliesWhenTheColumnIsNull(t *testing.T) {
	withDefaultBudget(t, 5_000) // R$ 50
	gate := budgetGate(t, 5_000, nil, nil)

	err := gate.ensure(context.Background(), "acc", domain.SpendAIDraft)

	require.Error(t, err, "a NULL column falls back to the platform default, not to 'no ceiling'")
	appErr := &domain.AppError{}
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "AI_BUDGET_EXCEEDED", appErr.Code)
}

func TestSpendGate_CustomBudgetWinsOverTheDefault(t *testing.T) {
	withDefaultBudget(t, 1_000)
	custom := int64(50_000)
	gate := budgetGate(t, 5_000, &custom, nil)

	assert.NoError(t, gate.ensure(context.Background(), "acc", domain.SpendAIDraft))
}

func TestSpendGate_NoDefaultAndNoColumnMeansNoCeiling(t *testing.T) {
	withDefaultBudget(t, 0)
	gate := budgetGate(t, 999_999, nil, nil)

	assert.NoError(t, gate.ensure(context.Background(), "acc", domain.SpendAIDraft))
}

func TestEffectiveAIBudget_ReportsTheSource(t *testing.T) {
	withDefaultBudget(t, 5_000)
	custom := int64(20_000)

	cents, source := EffectiveAIBudget(&domain.Account{AIMonthlyBudgetCents: &custom})
	assert.Equal(t, int64(20_000), cents)
	assert.Equal(t, BudgetSourceCustom, source)

	cents, source = EffectiveAIBudget(&domain.Account{})
	assert.Equal(t, int64(5_000), cents)
	assert.Equal(t, BudgetSourceDefault, source)

	withDefaultBudget(t, 0)
	cents, source = EffectiveAIBudget(&domain.Account{})
	assert.Equal(t, int64(0), cents)
	assert.Equal(t, BudgetSourceNone, source)
}

func TestSpendGate_EnsureAnyOnAnUnwiredGateAllows(t *testing.T) {
	var gate *spendGate
	assert.NoError(t, gate.ensureAny(context.Background(), "acc"))
	assert.NoError(t, gate.ensureAnyFor(context.Background(), &domain.Account{ID: "acc"}))
}

func TestSpendGate_EnsureAnyRefusesPastTheCeiling(t *testing.T) {
	budget := int64(1_000)
	gate := budgetGate(t, 1_000, &budget, nil)

	err := gate.ensureAny(context.Background(), "acc")

	appErr := &domain.AppError{}
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "AI_BUDGET_EXCEEDED", appErr.Code)
}

// batchBudgetGate is budgetGate with a real per-kind counter so remainingFor
// has both the plan allowance and the ledger average to reason with.
func batchBudgetGate(t *testing.T, spentCents int64, budgetCents *int64, usedOfKind int64, planCap int) *spendGate {
	t.Helper()
	planID := "plan-1"
	accounts := &MockAccountRepo{}
	accounts.On("GetByID", mock.Anything, "acc").Return(&domain.Account{
		ID: "acc", PlanID: &planID, Timezone: "America/Sao_Paulo",
		AIMonthlyBudgetCents: budgetCents,
	}, nil)
	plans := &MockPlanRepo{}
	plans.On("GetByID", mock.Anything, "plan-1").Return(spendPlan(planCap), nil)

	gate := newSpendGate(accounts, plans, &fakeSpendCounter{count: usedOfKind})
	gate.SetCostLedger(fakeCostLedger{brlCents: spentCents})
	gate.now = func() time.Time { return time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC) }
	return gate
}

func TestSpendGate_BatchIsCappedByWhatTheBudgetStillAffords(t *testing.T) {
	withDefaultBudget(t, 0)
	// R$ 10 spent over 100 drafts → R$ 0,10 each; R$ 12 ceiling leaves R$ 2 → 20 more.
	budget := int64(1_200)
	gate := batchBudgetGate(t, 1_000, &budget, 100, 1000)

	allowed, err := gate.remainingFor(context.Background(), "acc", domain.SpendAIDraft, 500)

	require.NoError(t, err)
	assert.Equal(t, 20, allowed, "the plan allows 900 more; the budget only 20")
}

func TestSpendGate_BatchWithNoHistoryUsesTheFallbackEstimate(t *testing.T) {
	withDefaultBudget(t, 0)
	// Nothing spent yet: R$ 3 ceiling ÷ R$ 0,15 fallback → 20.
	budget := int64(300)
	gate := batchBudgetGate(t, 0, &budget, 0, 1000)

	allowed, err := gate.remainingFor(context.Background(), "acc", domain.SpendAIDraft, 500)

	require.NoError(t, err)
	assert.Equal(t, 20, allowed)
}

func TestSpendGate_BatchNeverReturnsZeroWhileMoneyIsLeft(t *testing.T) {
	withDefaultBudget(t, 0)
	// R$ 10 over 10 drafts = R$ 1 each; 50 cents left is less than one, floor 1.
	budget := int64(1_050)
	gate := batchBudgetGate(t, 1_000, &budget, 10, 1000)

	allowed, err := gate.remainingFor(context.Background(), "acc", domain.SpendAIDraft, 500)

	require.NoError(t, err)
	assert.Equal(t, 1, allowed)
}

func TestSpendGate_BatchRefusesWhenTheBudgetIsAlreadyCrossed(t *testing.T) {
	withDefaultBudget(t, 0)
	budget := int64(1_000)
	gate := batchBudgetGate(t, 1_000, &budget, 10, 1000)

	_, err := gate.remainingFor(context.Background(), "acc", domain.SpendAIDraft, 5)

	appErr := &domain.AppError{}
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "AI_BUDGET_EXCEEDED", appErr.Code)
}

func TestSpendGate_BatchUsesTheDefaultBudgetWhenTheColumnIsNull(t *testing.T) {
	withDefaultBudget(t, 5_000)
	gate := batchBudgetGate(t, 4_985, nil, 100, 1000) // 15 cents left of R$ 50, ~50c/draft

	allowed, err := gate.remainingFor(context.Background(), "acc", domain.SpendAIDraft, 40)

	require.NoError(t, err)
	assert.Equal(t, 1, allowed)
}

func TestSpendGate_BatchWithoutACeilingKeepsThePlanAllowance(t *testing.T) {
	withDefaultBudget(t, 0)
	gate := batchBudgetGate(t, 999_999, nil, 100, 1000)

	allowed, err := gate.remainingFor(context.Background(), "acc", domain.SpendAIDraft, 40)

	require.NoError(t, err)
	assert.Equal(t, 40, allowed)
}

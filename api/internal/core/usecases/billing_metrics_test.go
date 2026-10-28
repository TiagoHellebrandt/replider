package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockBillingEventRepo struct{ mock.Mock }

func (m *mockBillingEventRepo) Create(ctx context.Context, e *domain.BillingEvent) error {
	return m.Called(ctx, e).Error(0)
}

func (m *mockBillingEventRepo) Aggregate(ctx context.Context, since time.Time) (*domain.BillingMetricsAggregate, error) {
	args := m.Called(ctx, since)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.BillingMetricsAggregate), args.Error(1)
}

func TestBillingMetrics_DerivesARPAChurnLTV(t *testing.T) {
	repo := new(mockBillingEventRepo)
	// 10 active accounts paying R$100/mo (10000 cents) each = 100000 MRR.
	// 2 churned in window → cohort 12, churn rate 2/12 ≈ 0.1667.
	repo.On("Aggregate", mock.Anything, mock.Anything).Return(&domain.BillingMetricsAggregate{
		MRRBRLCents:     100000,
		ActiveAccounts:  10,
		ChurnedInWindow: 2,
	}, nil)

	uc := NewBillingMetricsUseCase(repo)
	res, err := uc.Execute(context.Background(), 30)
	require.NoError(t, err)

	assert.Equal(t, 30, res.WindowDays)
	assert.Equal(t, int64(100000), res.MRRCents)
	assert.Equal(t, 10, res.ActiveAccounts)
	assert.Equal(t, int64(10000), res.ARPACents) // 100000 / 10
	assert.InDelta(t, 0.16667, res.ChurnRate, 0.001)
	// LTV = ARPA / churn = 10000 / 0.16667 = 60000
	assert.Equal(t, int64(60000), res.LTVCents)
}

func TestBillingMetrics_NoChurnYieldsZeroLTV(t *testing.T) {
	repo := new(mockBillingEventRepo)
	repo.On("Aggregate", mock.Anything, mock.Anything).Return(&domain.BillingMetricsAggregate{
		MRRBRLCents:     50000,
		ActiveAccounts:  5,
		ChurnedInWindow: 0,
	}, nil)

	uc := NewBillingMetricsUseCase(repo)
	res, err := uc.Execute(context.Background(), 30)
	require.NoError(t, err)

	assert.Equal(t, int64(10000), res.ARPACents)
	assert.Equal(t, float64(0), res.ChurnRate)
	// Churn rate 0 ⇒ lifetime unbounded; we report 0 rather than infinity.
	assert.Equal(t, int64(0), res.LTVCents)
}

func TestBillingMetrics_EmptyLedgerIsAllZero(t *testing.T) {
	repo := new(mockBillingEventRepo)
	repo.On("Aggregate", mock.Anything, mock.Anything).Return(&domain.BillingMetricsAggregate{}, nil)

	uc := NewBillingMetricsUseCase(repo)
	res, err := uc.Execute(context.Background(), 0) // 0 → default 30
	require.NoError(t, err)

	assert.Equal(t, 30, res.WindowDays)
	assert.Equal(t, int64(0), res.MRRCents)
	assert.Equal(t, int64(0), res.ARPACents)
	assert.Equal(t, float64(0), res.ChurnRate)
	assert.Equal(t, int64(0), res.LTVCents)
}

func TestBillingMetrics_WindowBoundsChurnCutoff(t *testing.T) {
	repo := new(mockBillingEventRepo)
	fixed := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	want := fixed.AddDate(0, 0, -7)
	repo.On("Aggregate", mock.Anything, want).Return(&domain.BillingMetricsAggregate{}, nil)

	uc := NewBillingMetricsUseCase(repo)
	uc.now = func() time.Time { return fixed }

	res, err := uc.Execute(context.Background(), 7)
	require.NoError(t, err)
	assert.Equal(t, 7, res.WindowDays)
	repo.AssertExpectations(t)
}

// US-018 review fixes: the ledger reports BRL and USD MRR separately and
// the use case consolidates by CONVERTING the USD slice — never by adding
// raw cents of different currencies (the pre-fix SQL summed a $199 USD
// account and a R$199 BRL account as 39800 "BRL" cents).
func TestBillingMetrics_ConsolidatesUSDIntoBRLAtRate(t *testing.T) {
	repo := new(mockBillingEventRepo)
	// 1 BRL account paying R$1000 + 1 USD account paying $199.
	repo.On("Aggregate", mock.Anything, mock.Anything).Return(&domain.BillingMetricsAggregate{
		MRRBRLCents:    100000,
		MRRUSDCents:    19900,
		ActiveAccounts: 2,
	}, nil)

	uc := NewBillingMetricsUseCase(repo)
	res, err := uc.Execute(context.Background(), 30)
	require.NoError(t, err)

	// Default rate 550 (R$5.50/USD): $199 → 19900 * 550 / 100 = 109450.
	assert.Equal(t, int64(100000), res.MRRBRLCents)
	assert.Equal(t, int64(19900), res.MRRUSDCents)
	assert.Equal(t, int64(DefaultUSDBRLRateCents), res.USDBRLRateCents)
	assert.Equal(t, int64(209450), res.MRRCents)
	// ARPA off the consolidated BRL figure, not the raw mixed sum.
	assert.Equal(t, int64(104725), res.ARPACents)
}

func TestBillingMetrics_UsesOwnerManagedRate(t *testing.T) {
	repo := new(mockBillingEventRepo)
	repo.On("Aggregate", mock.Anything, mock.Anything).Return(&domain.BillingMetricsAggregate{
		MRRUSDCents:    10000, // $100
		ActiveAccounts: 1,
	}, nil)

	settings := newMockProviderCostRepo()
	settings.settings[domain.SettingUSDBRLRateCents] = "600"

	uc := NewBillingMetricsUseCase(repo)
	uc.SetPlatformSettings(settings)
	res, err := uc.Execute(context.Background(), 30)
	require.NoError(t, err)

	assert.Equal(t, int64(600), res.USDBRLRateCents)
	assert.Equal(t, int64(60000), res.MRRCents) // $100 → R$600
	assert.Equal(t, int64(0), res.MRRBRLCents)
	assert.Equal(t, int64(10000), res.MRRUSDCents)
}

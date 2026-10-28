package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

type mockStatementRepo struct {
	entries     []domain.SellerStatementLine
	entriesErr  error
	cycles      []string
	cyclesErr   error
	settings    *domain.SellerSettings
	settingsErr error
	pixErr      error

	gotSeller string
	gotCycle  string
	gotPix    *string
	pixCalled bool
}

func (m *mockStatementRepo) ListSellerStatement(_ context.Context, sellerUserID, cycle string) ([]domain.SellerStatementLine, error) {
	m.gotSeller, m.gotCycle = sellerUserID, cycle
	return m.entries, m.entriesErr
}

func (m *mockStatementRepo) ListSellerCycles(context.Context, string) ([]string, error) {
	return m.cycles, m.cyclesErr
}

func (m *mockStatementRepo) GetSellerSettings(context.Context, string) (*domain.SellerSettings, error) {
	return m.settings, m.settingsErr
}

func (m *mockStatementRepo) UpsertSellerPixKey(_ context.Context, _ string, pixKey *string) error {
	m.pixCalled, m.gotPix = true, pixKey
	return m.pixErr
}

func statementUC(repo *mockStatementRepo, now time.Time) *SellerStatementUseCase {
	uc := NewSellerStatementUseCase(repo)
	uc.now = func() time.Time { return now }
	return uc
}

func ptrI64(v int64) *int64 { return &v }

func TestSellerStatement_DefaultsToLastClosedCycleWithTotals(t *testing.T) {
	pix := "vendedor@pix.com"
	repo := &mockStatementRepo{
		entries: []domain.SellerStatementLine{
			{
				AccountID: "acc-1", AccountName: "Padaria",
				Reason: domain.CommissionReasonStandard, RateApplied: 50,
				AmountCents: 10000, Status: domain.CommissionStatusPending,
				RevenueCents: ptrI64(29900), StripeFeeCents: ptrI64(2392),
				TokenCostCents: ptrI64(1353), TaxCents: ptrI64(1794),
				InfraCents: ptrI64(500), NetProfitCents: ptrI64(20000),
			},
			{
				AccountID: "acc-1", AccountName: "Padaria",
				Reason:      domain.CommissionReasonBonusNPS,
				AmountCents: 10000, Status: domain.CommissionStatusPaid,
			},
		},
		cycles:   []string{"2026-06", "2026-05"},
		settings: &domain.SellerSettings{UserID: "seller-1", PixKey: &pix},
	}

	out, err := statementUC(repo, time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)).
		Execute(context.Background(), "seller-1", "")

	assert.NoError(t, err)
	assert.Equal(t, "2026-06", out.Cycle)
	assert.False(t, out.IsOpen)
	assert.Equal(t, "seller-1", repo.gotSeller)
	assert.Equal(t, "2026-06", repo.gotCycle)
	assert.Len(t, out.Entries, 2)
	// Breakdown survives to the payload — transparency requirement.
	assert.Equal(t, int64(29900), *out.Entries[0].RevenueCents)
	assert.Equal(t, int64(20000), *out.Entries[0].NetProfitCents)
	assert.Equal(t, int64(20000), out.TotalCents)
	assert.Equal(t, int64(10000), out.PendingCents)
	assert.Equal(t, int64(10000), out.PaidCents)
	assert.Equal(t, "vendedor@pix.com", *out.PixKey)
	// Selector: earned cycles ∪ current open ∪ last closed, newest first.
	assert.Equal(t, []string{"2026-07", "2026-06", "2026-05"}, out.Cycles)
}

func TestSellerStatement_OpenCycleIsPreview(t *testing.T) {
	repo := &mockStatementRepo{cycles: []string{"2026-06"}}

	out, err := statementUC(repo, time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)).
		Execute(context.Background(), "seller-1", "2026-07")

	assert.NoError(t, err)
	assert.True(t, out.IsOpen, "current month must be flagged as an open preview")
	assert.Equal(t, "2026-07", out.Cycle)

	closed, err := statementUC(repo, time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)).
		Execute(context.Background(), "seller-1", "2026-05")
	assert.NoError(t, err)
	assert.False(t, closed.IsOpen)
}

func TestSellerStatement_EmptyCycleAndNoSettings(t *testing.T) {
	repo := &mockStatementRepo{}

	out, err := statementUC(repo, time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)).
		Execute(context.Background(), "seller-1", "2026-01")

	assert.NoError(t, err)
	assert.Empty(t, out.Entries)
	assert.Zero(t, out.TotalCents)
	assert.Zero(t, out.PendingCents)
	assert.Nil(t, out.PixKey)
	// Requested + current + last closed still offered.
	assert.Equal(t, []string{"2026-07", "2026-06", "2026-01"}, out.Cycles)
}

func TestSellerStatement_InvalidCycle(t *testing.T) {
	repo := &mockStatementRepo{}
	uc := statementUC(repo, time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC))

	for _, cycle := range []string{"2026", "2026-13", "junho", "2026-06-01"} {
		_, err := uc.Execute(context.Background(), "seller-1", cycle)
		var appErr *domain.AppError
		assert.ErrorAs(t, err, &appErr, cycle)
	}
}

func TestSellerStatement_PropagatesErrors(t *testing.T) {
	boom := errors.New("boom")
	now := time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)

	_, err := statementUC(&mockStatementRepo{entriesErr: boom}, now).
		Execute(context.Background(), "s", "")
	assert.ErrorIs(t, err, boom)

	_, err = statementUC(&mockStatementRepo{cyclesErr: boom}, now).
		Execute(context.Background(), "s", "")
	assert.ErrorIs(t, err, boom)

	_, err = statementUC(&mockStatementRepo{settingsErr: boom}, now).
		Execute(context.Background(), "s", "")
	assert.ErrorIs(t, err, boom)
}

func TestSellerStatement_UpdatePixKey(t *testing.T) {
	now := time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)

	repo := &mockStatementRepo{}
	err := statementUC(repo, now).UpdatePixKey(context.Background(), "seller-1", "  chave@pix.br  ")
	assert.NoError(t, err)
	assert.True(t, repo.pixCalled)
	assert.Equal(t, "chave@pix.br", *repo.gotPix)

	// Empty clears the key (stored NULL).
	repo = &mockStatementRepo{}
	err = statementUC(repo, now).UpdatePixKey(context.Background(), "seller-1", "   ")
	assert.NoError(t, err)
	assert.True(t, repo.pixCalled)
	assert.Nil(t, repo.gotPix)

	// Too long is rejected before touching the repo.
	repo = &mockStatementRepo{}
	err = statementUC(repo, now).UpdatePixKey(context.Background(), "seller-1", strings.Repeat("x", 141))
	var appErr *domain.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, "SELLER_INVALID_PIX_KEY", appErr.Code)
	assert.False(t, repo.pixCalled)
}

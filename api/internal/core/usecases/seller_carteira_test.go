package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

type mockCarteiraRepo struct {
	accounts     []domain.SellerCarteiraAccount
	accountsErr  error
	pending      int64
	pendingErr   error
	gotSeller    string
	gotCycle     string
	gotPendingID string
}

func (m *mockCarteiraRepo) ListCarteiraAccounts(_ context.Context, sellerUserID, cycle string) ([]domain.SellerCarteiraAccount, error) {
	m.gotSeller, m.gotCycle = sellerUserID, cycle
	return m.accounts, m.accountsErr
}

func (m *mockCarteiraRepo) SumPendingCommission(_ context.Context, sellerUserID string) (int64, error) {
	m.gotPendingID = sellerUserID
	return m.pending, m.pendingErr
}

type mockRefInfo struct {
	info *SellerRefInfo
	err  error
}

func (m *mockRefInfo) Execute(context.Context, string) (*SellerRefInfo, error) {
	return m.info, m.err
}

func carteiraUC(repo *mockCarteiraRepo, ref *mockRefInfo, now time.Time) *SellerCarteiraUseCase {
	uc := NewSellerCarteiraUseCase(repo, ref)
	uc.now = func() time.Time { return now }
	return uc
}

func TestSellerCarteira_BundlesHeaderAndAccounts(t *testing.T) {
	lead := domain.LeadSourceSellerRef
	profit := int64(20000)
	commission := int64(10000)
	repo := &mockCarteiraRepo{
		accounts: []domain.SellerCarteiraAccount{{
			AccountID:                "acc-1",
			Name:                     "Padaria",
			PlanName:                 "Core",
			MRRCents:                 9900,
			Currency:                 "brl",
			LeadSource:               &lead,
			SubscriptionStatus:       domain.SubscriptionStatusActive,
			LastCycleNetProfitCents:  &profit,
			LastCycleCommissionCents: &commission,
		}},
		pending: 12345,
	}
	ref := &mockRefInfo{info: &SellerRefInfo{Code: "ABC123", ShareURL: "https://replider.com/register?ref=ABC123"}}

	out, err := carteiraUC(repo, ref, time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)).
		Execute(context.Background(), "seller-1")

	assert.NoError(t, err)
	assert.Equal(t, "ABC123", out.RefCode)
	assert.Equal(t, "https://replider.com/register?ref=ABC123", out.ShareURL)
	assert.Equal(t, 1, out.TotalAccounts)
	assert.Equal(t, int64(12345), out.PendingCommissionCents)
	assert.Equal(t, "2026-06", out.LastClosedCycle)
	assert.Len(t, out.Accounts, 1)
	assert.Equal(t, "Padaria", out.Accounts[0].Name)
	// The repo was queried for the caller and the last CLOSED cycle.
	assert.Equal(t, "seller-1", repo.gotSeller)
	assert.Equal(t, "seller-1", repo.gotPendingID)
	assert.Equal(t, "2026-06", repo.gotCycle)
}

func TestSellerCarteira_LastClosedCycleCrossesYear(t *testing.T) {
	repo := &mockCarteiraRepo{}
	ref := &mockRefInfo{info: &SellerRefInfo{Code: "X", ShareURL: "u"}}

	out, err := carteiraUC(repo, ref, time.Date(2027, 1, 15, 3, 0, 0, 0, time.UTC)).
		Execute(context.Background(), "seller-1")

	assert.NoError(t, err)
	assert.Equal(t, "2026-12", out.LastClosedCycle)
}

func TestSellerCarteira_EmptyCarteira(t *testing.T) {
	repo := &mockCarteiraRepo{accounts: []domain.SellerCarteiraAccount{}}
	ref := &mockRefInfo{info: &SellerRefInfo{Code: "X", ShareURL: "u"}}

	out, err := carteiraUC(repo, ref, time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)).
		Execute(context.Background(), "seller-1")

	assert.NoError(t, err)
	assert.Equal(t, 0, out.TotalAccounts)
	assert.Empty(t, out.Accounts)
	assert.Equal(t, int64(0), out.PendingCommissionCents)
}

func TestSellerCarteira_PropagatesErrors(t *testing.T) {
	boom := errors.New("boom")

	_, err := carteiraUC(&mockCarteiraRepo{}, &mockRefInfo{err: boom},
		time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)).Execute(context.Background(), "s")
	assert.ErrorIs(t, err, boom)

	_, err = carteiraUC(&mockCarteiraRepo{accountsErr: boom},
		&mockRefInfo{info: &SellerRefInfo{}},
		time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)).Execute(context.Background(), "s")
	assert.ErrorIs(t, err, boom)

	_, err = carteiraUC(&mockCarteiraRepo{pendingErr: boom},
		&mockRefInfo{info: &SellerRefInfo{}},
		time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)).Execute(context.Background(), "s")
	assert.ErrorIs(t, err, boom)
}

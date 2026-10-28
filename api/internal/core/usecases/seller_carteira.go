package usecases

import (
	"context"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Narrow local interfaces (mock only what the use case touches).
type sellerCarteiraRepo interface {
	ListCarteiraAccounts(ctx context.Context, sellerUserID, cycle string) ([]domain.SellerCarteiraAccount, error)
	SumPendingCommission(ctx context.Context, sellerUserID string) (int64, error)
}

type sellerRefInfoProvider interface {
	Execute(ctx context.Context, userID string) (*SellerRefInfo, error)
}

// SellerCarteiraUseCase assembles the seller panel's carteira view
// (US-010): the personal ?ref= link, header totals and each linked
// account with the last CLOSED cycle's profit + commission. Routes are
// gated by middleware.RequirePlatformAccount, so only platform-account
// users reach it.
type SellerCarteiraUseCase struct {
	repo    sellerCarteiraRepo
	refInfo sellerRefInfoProvider
	now     func() time.Time
}

func NewSellerCarteiraUseCase(repo sellerCarteiraRepo, refInfo sellerRefInfoProvider) *SellerCarteiraUseCase {
	return &SellerCarteiraUseCase{repo: repo, refInfo: refInfo, now: time.Now}
}

// SellerCarteira is the GET /seller/carteira payload.
type SellerCarteira struct {
	RefCode                string `json:"ref_code"`
	ShareURL               string `json:"share_url"`
	TotalAccounts          int    `json:"total_accounts"`
	PendingCommissionCents int64  `json:"pending_commission_cents"`
	// LastClosedCycle (YYYY-MM) is the month before the current one —
	// the newest cycle the recompute can have fully settled.
	LastClosedCycle string                         `json:"last_closed_cycle"`
	Accounts        []domain.SellerCarteiraAccount `json:"accounts"`
	// CommissionCurrency: the currency every profit/commission figure is
	// in (always BRL by ledger construction — MRR is the only per-account
	// currency field). The app formats from this, never a hardcode.
	CommissionCurrency string `json:"commission_currency"`
}

func (uc *SellerCarteiraUseCase) Execute(ctx context.Context, userID string) (*SellerCarteira, error) {
	ref, err := uc.refInfo.Execute(ctx, userID)
	if err != nil {
		return nil, err
	}

	nowT := uc.now().UTC()
	cycle := time.Date(nowT.Year(), nowT.Month(), 1, 0, 0, 0, 0, time.UTC).
		AddDate(0, -1, 0).Format("2006-01")

	accounts, err := uc.repo.ListCarteiraAccounts(ctx, userID, cycle)
	if err != nil {
		return nil, err
	}
	pending, err := uc.repo.SumPendingCommission(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &SellerCarteira{
		RefCode:                ref.Code,
		ShareURL:               ref.ShareURL,
		TotalAccounts:          len(accounts),
		PendingCommissionCents: pending,
		LastClosedCycle:        cycle,
		Accounts:               accounts,
		CommissionCurrency:     domain.CommissionLedgerCurrency,
	}, nil
}

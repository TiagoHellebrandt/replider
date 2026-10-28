package usecases

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Narrow local interface (mock only what the use case touches).
type sellerStatementRepo interface {
	ListSellerStatement(ctx context.Context, sellerUserID, cycle string) ([]domain.SellerStatementLine, error)
	ListSellerCycles(ctx context.Context, sellerUserID string) ([]string, error)
	GetSellerSettings(ctx context.Context, userID string) (*domain.SellerSettings, error)
	UpsertSellerPixKey(ctx context.Context, userID string, pixKey *string) error
}

const maxPixKeyLen = 140

// SellerStatementUseCase serves the seller's own commission statement
// (US-011): the full per-account breakdown — revenue − Stripe fee − AI
// cost − tax − infra = net profit × rate (with the reason) = commission
// — never just a total (transparency is a founder requirement). It also
// lets the seller edit their own PIX key. Routes are gated by
// middleware.RequirePlatformAccount.
type SellerStatementUseCase struct {
	repo sellerStatementRepo
	now  func() time.Time
}

func NewSellerStatementUseCase(repo sellerStatementRepo) *SellerStatementUseCase {
	return &SellerStatementUseCase{repo: repo, now: time.Now}
}

// SellerStatement is the GET /seller/commissions payload.
type SellerStatement struct {
	// Cycle being shown (YYYY-MM); defaults to the last closed one.
	Cycle string `json:"cycle"`
	// IsOpen: the cycle hasn't closed yet (current or future month) —
	// every number is a partial preview ("prévia"), not a settled
	// statement.
	IsOpen bool `json:"is_open"`
	// Cycles the selector offers: every cycle with ledger entries plus
	// the current (open) and last closed ones, newest first.
	Cycles []string `json:"cycles"`
	// PixKey is the seller's payout key (nil = not set yet).
	PixKey  *string                      `json:"pix_key,omitempty"`
	Entries []domain.SellerStatementLine `json:"entries"`
	// Cycle totals across every entry.
	TotalCents   int64 `json:"total_cents"`
	PendingCents int64 `json:"pending_cents"`
	PaidCents    int64 `json:"paid_cents"`
	// CommissionCurrency: currency of every amount/breakdown figure
	// (always BRL by ledger construction) — the app formats from this.
	CommissionCurrency string `json:"commission_currency"`
}

// Execute assembles the statement for one cycle; empty cycle defaults
// to the last closed one (the month before now).
func (uc *SellerStatementUseCase) Execute(ctx context.Context, userID, cycle string) (*SellerStatement, error) {
	nowT := uc.now().UTC()
	currentCycle := nowT.Format("2006-01")
	lastClosed := time.Date(nowT.Year(), nowT.Month(), 1, 0, 0, 0, 0, time.UTC).
		AddDate(0, -1, 0).Format("2006-01")

	if cycle == "" {
		cycle = lastClosed
	}
	if _, _, err := parseCycle(cycle); err != nil {
		return nil, err
	}

	entries, err := uc.repo.ListSellerStatement(ctx, userID, cycle)
	if err != nil {
		return nil, err
	}
	earned, err := uc.repo.ListSellerCycles(ctx, userID)
	if err != nil {
		return nil, err
	}
	settings, err := uc.repo.GetSellerSettings(ctx, userID)
	if err != nil {
		return nil, err
	}

	out := &SellerStatement{
		Cycle:              cycle,
		IsOpen:             cycle >= currentCycle,
		Cycles:             mergeCycles(earned, currentCycle, lastClosed, cycle),
		Entries:            entries,
		CommissionCurrency: domain.CommissionLedgerCurrency,
	}
	if settings != nil {
		out.PixKey = settings.PixKey
	}
	for _, e := range entries {
		out.TotalCents += e.AmountCents
		if e.Status == domain.CommissionStatusPaid {
			out.PaidCents += e.AmountCents
		} else {
			out.PendingCents += e.AmountCents
		}
	}
	return out, nil
}

// UpdatePixKey stores the seller's own payout key; empty clears it.
func (uc *SellerStatementUseCase) UpdatePixKey(ctx context.Context, userID, pixKey string) error {
	pixKey = strings.TrimSpace(pixKey)
	if len(pixKey) > maxPixKeyLen {
		return &domain.AppError{Code: "SELLER_INVALID_PIX_KEY", Message: "pix key too long (max 140 chars)"}
	}
	var val *string
	if pixKey != "" {
		val = &pixKey
	}
	if err := uc.repo.UpsertSellerPixKey(ctx, userID, val); err != nil {
		return err
	}
	slog.Info("seller pix key updated", "seller_user_id", userID, "cleared", val == nil)
	return nil
}

// mergeCycles unions the earned cycles with the ones the selector must
// always offer, newest first.
func mergeCycles(earned []string, extras ...string) []string {
	seen := map[string]bool{}
	all := append(append([]string{}, earned...), extras...)
	out := []string{}
	for _, c := range all {
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out
}

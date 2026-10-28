package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// mockMonthlyProfitRepo implements domain.MonthlyProfitRepository in
// memory, mirroring the ON CONFLICT upsert semantics (one row per
// account+cycle, original id survives).
type mockMonthlyProfitRepo struct {
	rows        map[string]*domain.AccountMonthlyProfit // key: accountID|cycle
	upsertCalls int
	tokenCost   map[string]domain.ProfitTokenCost
	targets     []domain.ProfitComputeTarget
}

func newMockMonthlyProfitRepo() *mockMonthlyProfitRepo {
	return &mockMonthlyProfitRepo{
		rows:      map[string]*domain.AccountMonthlyProfit{},
		tokenCost: map[string]domain.ProfitTokenCost{},
	}
}

func (m *mockMonthlyProfitRepo) Upsert(_ context.Context, p *domain.AccountMonthlyProfit) error {
	m.upsertCalls++
	key := p.AccountID + "|" + p.Cycle
	if existing, ok := m.rows[key]; ok {
		p.ID = existing.ID // ON CONFLICT keeps the original id
	}
	p.ComputedAt = time.Now()
	cp := *p
	m.rows[key] = &cp
	return nil
}

func (m *mockMonthlyProfitRepo) ListByCycle(_ context.Context, cycle string) ([]*domain.AccountMonthlyProfit, error) {
	out := []*domain.AccountMonthlyProfit{}
	for _, p := range m.rows {
		if p.Cycle == cycle {
			p.IsNegative = p.NetProfitCents < 0
			out = append(out, p)
		}
	}
	return out, nil
}

func (m *mockMonthlyProfitRepo) SumTokenCost(_ context.Context, accountID string, _, _ time.Time) (domain.ProfitTokenCost, error) {
	return m.tokenCost[accountID], nil
}

func (m *mockMonthlyProfitRepo) ListComputeTargets(_ context.Context) ([]domain.ProfitComputeTarget, error) {
	return m.targets, nil
}

type mockProfitAccounts struct {
	accounts map[string]*domain.Account
}

func (m *mockProfitAccounts) GetByID(_ context.Context, id string) (*domain.Account, error) {
	if a, ok := m.accounts[id]; ok {
		return a, nil
	}
	return nil, errors.New("account not found")
}

type mockInvoiceLister struct {
	invoices map[string][]domain.StripePaidInvoice
	lastFrom time.Time
	lastTo   time.Time
	calls    int
}

func (m *mockInvoiceLister) ListPaidInvoicesInRange(customerID string, from, to time.Time) ([]domain.StripePaidInvoice, error) {
	m.calls++
	m.lastFrom, m.lastTo = from, to
	return m.invoices[customerID], nil
}

func newProfitUC(repo *mockMonthlyProfitRepo, costs *mockProviderCostRepo, accounts *mockProfitAccounts, invoices *mockInvoiceLister) *ComputeMonthlyProfitUseCase {
	if accounts == nil {
		accounts = &mockProfitAccounts{accounts: map[string]*domain.Account{}}
	}
	var lister PaidInvoiceLister
	if invoices != nil {
		lister = invoices
	}
	return NewComputeMonthlyProfitUseCase(repo, costs, accounts, lister, nil)
}

func TestRecomputeCycle_InvalidCycle(t *testing.T) {
	uc := newProfitUC(newMockMonthlyProfitRepo(), newMockProviderCostRepo(), nil, nil)
	for _, cycle := range []string{"", "2026-7", "julho", "2026-13", "2026-07-01"} {
		_, err := uc.RecomputeCycle(context.Background(), "owner@x.com", cycle, "")
		var appErr *domain.AppError
		if !errors.As(err, &appErr) || appErr.Code != "PROFIT_INVALID_CYCLE" {
			t.Fatalf("cycle %q: expected PROFIT_INVALID_CYCLE, got %v", cycle, err)
		}
	}
}

// Account without any invoice: revenue 0, net = -(token cost + infra)
// — negative net is allowed and flagged.
func TestRecomputeCycle_NoInvoice_NetIsNegativeCosts(t *testing.T) {
	repo := newMockMonthlyProfitRepo()
	costs := newMockProviderCostRepo()
	// $1.55 of stamped per-message cost; at the default rate 550 →
	// 155 USD cents → 852.5 → 853 BRL cents.
	repo.tokenCost["acc1"] = domain.ProfitTokenCost{CostUSD: 1.55}
	repo.targets = []domain.ProfitComputeTarget{{AccountID: "acc1"}} // no Stripe customer

	uc := newProfitUC(repo, costs, nil, &mockInvoiceLister{})
	out, err := uc.RecomputeCycle(context.Background(), "owner@x.com", "2026-07", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 row, got %d", len(out))
	}
	p := out[0]
	if p.RevenueCents != 0 || p.StripeFeeCents != 0 {
		t.Fatalf("expected zero revenue/fee, got %d/%d", p.RevenueCents, p.StripeFeeCents)
	}
	if p.TokenCostCents != 853 {
		t.Fatalf("expected token cost 853, got %d", p.TokenCostCents)
	}
	if p.InfraCents != 500 {
		t.Fatalf("expected default infra 500, got %d", p.InfraCents)
	}
	if p.TaxCents != 0 {
		t.Fatalf("expected zero tax on zero revenue, got %d", p.TaxCents)
	}
	if p.NetProfitCents != -1353 || !p.IsNegative {
		t.Fatalf("expected net -1353 negative, got %d (negative=%v)", p.NetProfitCents, p.IsNegative)
	}
}

func TestRecomputeCycle_WithInvoices_RealFeeTaxAndUSDConversion(t *testing.T) {
	repo := newMockMonthlyProfitRepo()
	costs := newMockProviderCostRepo()
	costs.settings[domain.SettingProfitTaxBps] = "600" // 6%
	repo.targets = []domain.ProfitComputeTarget{{AccountID: "acc1", StripeCustomerID: strPtr("cus_1")}}
	lister := &mockInvoiceLister{invoices: map[string][]domain.StripePaidInvoice{
		"cus_1": {
			{ID: "in_1", AmountPaidCents: 9900, Currency: "brl", FeeCents: 350},
			// USD invoice converts at the manual rate (default 550):
			// 3900 * 550 / 100 = 21450 BRL cents. Fee already in BRL.
			{ID: "in_2", AmountPaidCents: 3900, Currency: "usd", FeeCents: 200},
		},
	}}

	uc := newProfitUC(repo, costs, nil, lister)
	out, err := uc.RecomputeCycle(context.Background(), "owner@x.com", "2026-07", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := out[0]
	if p.RevenueCents != 31350 {
		t.Fatalf("expected revenue 31350 (9900 + 21450), got %d", p.RevenueCents)
	}
	if p.StripeFeeCents != 550 {
		t.Fatalf("expected real fee sum 550, got %d", p.StripeFeeCents)
	}
	if p.TaxCents != 1881 { // 31350 * 600 / 10000
		t.Fatalf("expected tax 1881, got %d", p.TaxCents)
	}
	if p.NetProfitCents != 31350-550-1881-500 {
		t.Fatalf("unexpected net %d", p.NetProfitCents)
	}
	if p.IsNegative {
		t.Fatal("net is positive, IsNegative must be false")
	}
	// The invoice window is the cycle's month boundaries in the
	// platform business timezone (America/Cuiaba, UTC-4) — NOT UTC
	// (US-008 review fix): July 1st 00:00 Cuiabá = July 1st 04:00 UTC.
	wantFrom := time.Date(2026, 7, 1, 4, 0, 0, 0, time.UTC)
	if !lister.lastFrom.Equal(wantFrom) || !lister.lastTo.Equal(wantFrom.AddDate(0, 1, 0)) {
		t.Fatalf("unexpected invoice window %v..%v", lister.lastFrom, lister.lastTo)
	}
}

// US-008 review fix: cycle boundaries live in America/Cuiaba, matching
// the SLA signal window — an invoice paid late on the last local
// evening of June must not leak into July's cycle.
func TestParseCycle_CuiabaBoundaries(t *testing.T) {
	from, to, err := parseCycle("2026-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := from.UTC(); !got.Equal(time.Date(2026, 6, 1, 4, 0, 0, 0, time.UTC)) {
		t.Fatalf("cycle start must be June 1st 00:00 Cuiabá (04:00 UTC), got %v", got)
	}
	if got := to.UTC(); !got.Equal(time.Date(2026, 7, 1, 4, 0, 0, 0, time.UTC)) {
		t.Fatalf("cycle end must be July 1st 00:00 Cuiabá (04:00 UTC), got %v", got)
	}
	// 2026-06-30 23:30 Cuiabá (= 2026-07-01 03:30 UTC) is still June.
	lastEvening := time.Date(2026, 7, 1, 3, 30, 0, 0, time.UTC)
	if !lastEvening.Before(to) || lastEvening.Before(from) {
		t.Fatalf("last local evening of June must fall inside the June cycle")
	}
}

// US-008 review fix: an invoice in a currency the platform can't price
// (neither BRL nor USD) must be EXCLUDED with a log — not summed as if
// its cents were raw BRL.
func TestRecomputeCycle_UnsupportedInvoiceCurrencyExcluded(t *testing.T) {
	repo := newMockMonthlyProfitRepo()
	costs := newMockProviderCostRepo()
	repo.targets = []domain.ProfitComputeTarget{{AccountID: "acc1", StripeCustomerID: strPtr("cus_1")}}
	lister := &mockInvoiceLister{invoices: map[string][]domain.StripePaidInvoice{
		"cus_1": {
			{ID: "in_brl", AmountPaidCents: 9900, Currency: "brl", FeeCents: 350, FeeCurrency: "brl"},
			// EUR cents summed as raw BRL would fabricate revenue.
			{ID: "in_eur", AmountPaidCents: 5000, Currency: "eur", FeeCents: 120, FeeCurrency: "eur"},
		},
	}}

	uc := newProfitUC(repo, costs, nil, lister)
	out, err := uc.RecomputeCycle(context.Background(), "owner@x.com", "2026-07", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := out[0]
	if p.RevenueCents != 9900 {
		t.Fatalf("EUR invoice must be excluded from revenue: want 9900, got %d", p.RevenueCents)
	}
	if p.StripeFeeCents != 350 {
		t.Fatalf("EUR invoice's fee must be excluded too: want 350, got %d", p.StripeFeeCents)
	}
}

// US-008 review fix: the fee is denominated in the balance_transaction's
// settlement currency — a USD fee must convert at the manual rate, not
// be summed as raw BRL cents.
func TestRecomputeCycle_USDFeeConvertedAtRate(t *testing.T) {
	repo := newMockMonthlyProfitRepo()
	costs := newMockProviderCostRepo()
	repo.targets = []domain.ProfitComputeTarget{{AccountID: "acc1", StripeCustomerID: strPtr("cus_1")}}
	lister := &mockInvoiceLister{invoices: map[string][]domain.StripePaidInvoice{
		"cus_1": {
			// USD invoice settled in USD: fee 200 USD cents at the
			// default rate 550 → 1100 BRL cents (not 200).
			{ID: "in_usd", AmountPaidCents: 3900, Currency: "usd", FeeCents: 200, FeeCurrency: "usd"},
		},
	}}

	uc := newProfitUC(repo, costs, nil, lister)
	out, err := uc.RecomputeCycle(context.Background(), "owner@x.com", "2026-07", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := out[0]
	if p.RevenueCents != 21450 { // 3900 * 550 / 100
		t.Fatalf("expected revenue 21450, got %d", p.RevenueCents)
	}
	if p.StripeFeeCents != 1100 {
		t.Fatalf("USD fee must convert at the rate: want 1100, got %d", p.StripeFeeCents)
	}
	// A fee with no balance_transaction (empty currency) keeps the
	// legacy BRL assumption — zero fee, nothing to convert.
	if p.NetProfitCents != 21450-1100-500 {
		t.Fatalf("unexpected net %d", p.NetProfitCents)
	}
}

// US-008 review fix: token cost rounds ONCE, at the end. Rounding the
// USD sum to whole USD cents first and converting after inflated
// sub-cent costs by the whole exchange rate: $0.005 at rate 550 is
// round(2.75) = 3 BRL cents — the old double-round made it 6.
func TestRecomputeCycle_TokenCostSingleRounding(t *testing.T) {
	repo := newMockMonthlyProfitRepo()
	costs := newMockProviderCostRepo()
	repo.tokenCost["acc1"] = domain.ProfitTokenCost{CostUSD: 0.005}
	repo.targets = []domain.ProfitComputeTarget{{AccountID: "acc1"}}

	uc := newProfitUC(repo, costs, nil, &mockInvoiceLister{})
	out, err := uc.RecomputeCycle(context.Background(), "owner@x.com", "2026-07", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[0].TokenCostCents != 3 {
		t.Fatalf("expected single-rounded token cost 3, got %d", out[0].TokenCostCents)
	}
}

// Recompute of the same cycle overwrites the row — never a second one.
func TestRecomputeCycle_IsIdempotent(t *testing.T) {
	repo := newMockMonthlyProfitRepo()
	costs := newMockProviderCostRepo()
	repo.targets = []domain.ProfitComputeTarget{{AccountID: "acc1"}}

	uc := newProfitUC(repo, costs, nil, &mockInvoiceLister{})
	first, err := uc.RecomputeCycle(context.Background(), "owner@x.com", "2026-07", "")
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	second, err := uc.RecomputeCycle(context.Background(), "owner@x.com", "2026-07", "")
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if repo.upsertCalls != 2 {
		t.Fatalf("expected 2 upserts, got %d", repo.upsertCalls)
	}
	if len(repo.rows) != 1 {
		t.Fatalf("recompute duplicated the row: %d rows", len(repo.rows))
	}
	if first[0].ID != second[0].ID {
		t.Fatal("recompute must keep the original row id")
	}
}

// Message rows carrying tokens but no stamped provider cost
// (pre-stamp history) are priced at zero — never an error, never
// invented cost.
func TestRecomputeCycle_UnstampedTokensCostZero(t *testing.T) {
	repo := newMockMonthlyProfitRepo()
	costs := newMockProviderCostRepo()
	repo.tokenCost["acc1"] = domain.ProfitTokenCost{
		CostUSD: 0, UnpricedInputTokens: 5_000_000, UnpricedOutputTokens: 1_000_000,
	}
	repo.targets = []domain.ProfitComputeTarget{{AccountID: "acc1"}}

	uc := newProfitUC(repo, costs, nil, &mockInvoiceLister{})
	out, err := uc.RecomputeCycle(context.Background(), "owner@x.com", "2026-07", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[0].TokenCostCents != 0 {
		t.Fatalf("unstamped tokens must cost zero, got %d", out[0].TokenCostCents)
	}
}

// Refunds and refund-backed credit notes net out of revenue: revenue =
// paid − refunded, per invoice, clamped at zero — including USD
// invoices (converted after netting). The old computation summed
// AmountPaid untouched, inflating the commission base.
func TestRecomputeCycle_RefundsSubtractedFromRevenue(t *testing.T) {
	repo := newMockMonthlyProfitRepo()
	costs := newMockProviderCostRepo()
	repo.targets = []domain.ProfitComputeTarget{{AccountID: "acc1", StripeCustomerID: strPtr("cus_1")}}
	lister := &mockInvoiceLister{invoices: map[string][]domain.StripePaidInvoice{
		"cus_1": {
			// Partially refunded BRL invoice: 9900 − 2000 = 7900.
			{ID: "in_1", AmountPaidCents: 9900, RefundedCents: 2000, Currency: "brl", FeeCents: 350},
			// Fully refunded → contributes zero.
			{ID: "in_2", AmountPaidCents: 5000, RefundedCents: 5000, Currency: "brl", FeeCents: 180},
			// Over-refund edge clamps at zero, never negative revenue.
			{ID: "in_3", AmountPaidCents: 1000, RefundedCents: 1001, Currency: "brl", FeeCents: 40},
			// USD nets BEFORE the manual-rate conversion:
			// (3900 − 900) * 550 / 100 = 16500 BRL cents.
			{ID: "in_4", AmountPaidCents: 3900, RefundedCents: 900, Currency: "usd", FeeCents: 200},
		},
	}}

	uc := newProfitUC(repo, costs, nil, lister)
	out, err := uc.RecomputeCycle(context.Background(), "owner@x.com", "2026-07", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := out[0]
	if p.RevenueCents != 7900+0+0+16500 {
		t.Fatalf("expected refund-netted revenue 24400, got %d", p.RevenueCents)
	}
}

// Narrowing to one account resolves it through the account repo (the
// account need not be seller-linked yet).
func TestRecomputeCycle_SingleAccount(t *testing.T) {
	repo := newMockMonthlyProfitRepo()
	costs := newMockProviderCostRepo()
	accounts := &mockProfitAccounts{accounts: map[string]*domain.Account{
		"acc9": {ID: "acc9", StripeCustomerID: strPtr("cus_9")},
	}}
	lister := &mockInvoiceLister{invoices: map[string][]domain.StripePaidInvoice{
		"cus_9": {{ID: "in_9", AmountPaidCents: 5000, Currency: "brl", FeeCents: 100}},
	}}

	uc := newProfitUC(repo, costs, accounts, lister)
	out, err := uc.RecomputeCycle(context.Background(), "owner@x.com", "2026-07", "acc9")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0].AccountID != "acc9" || out[0].RevenueCents != 5000 {
		t.Fatalf("unexpected result: %+v", out)
	}

	if _, err := uc.RecomputeCycle(context.Background(), "owner@x.com", "2026-07", "missing"); err == nil {
		t.Fatal("expected error for unknown account")
	}
}

func TestRecomputeCycle_AllSellerLinkedTargets(t *testing.T) {
	repo := newMockMonthlyProfitRepo()
	costs := newMockProviderCostRepo()
	repo.targets = []domain.ProfitComputeTarget{
		{AccountID: "acc1"},
		{AccountID: "acc2", StripeCustomerID: strPtr("cus_2")},
	}

	uc := newProfitUC(repo, costs, nil, &mockInvoiceLister{})
	out, err := uc.RecomputeCycle(context.Background(), "owner@x.com", "2026-07", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 || len(repo.rows) != 2 {
		t.Fatalf("expected 2 computed rows, got %d (stored %d)", len(out), len(repo.rows))
	}
}

func TestSetProfitSettings(t *testing.T) {
	costs := newMockProviderCostRepo()
	uc := newProfitUC(newMockMonthlyProfitRepo(), costs, nil, nil)

	if err := uc.SetProfitSettings(context.Background(), "owner@x.com", 600, 700); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if costs.settings[domain.SettingProfitTaxBps] != "600" || costs.settings[domain.SettingInfraCostCents] != "700" {
		t.Fatalf("settings not persisted: %+v", costs.settings)
	}

	for _, tc := range []struct{ tax, infra int }{{-1, 500}, {5001, 500}, {0, -1}, {0, 100001}} {
		if err := uc.SetProfitSettings(context.Background(), "owner@x.com", tc.tax, tc.infra); err == nil {
			t.Fatalf("expected bounds error for tax=%d infra=%d", tc.tax, tc.infra)
		}
	}
}

func TestListProfitCycle_BundlesSettings(t *testing.T) {
	repo := newMockMonthlyProfitRepo()
	costs := newMockProviderCostRepo()
	costs.settings[domain.SettingProfitTaxBps] = "600"
	costs.settings[domain.SettingInfraCostCents] = "750"
	costs.settings[domain.SettingUSDBRLRateCents] = "520"
	repo.rows["acc1|2026-07"] = &domain.AccountMonthlyProfit{
		ID: "row1", AccountID: "acc1", Cycle: "2026-07", NetProfitCents: -10,
	}

	uc := newProfitUC(repo, costs, nil, nil)
	out, err := uc.List(context.Background(), "2026-07")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TaxBps != 600 || out.InfraCostCents != 750 || out.USDBRLRateCents != 520 {
		t.Fatalf("unexpected settings bundle: %+v", out)
	}
	if len(out.Items) != 1 || !out.Items[0].IsNegative {
		t.Fatalf("unexpected items: %+v", out.Items)
	}

	if _, err := uc.List(context.Background(), "not-a-cycle"); err == nil {
		t.Fatal("expected invalid cycle error")
	}
}

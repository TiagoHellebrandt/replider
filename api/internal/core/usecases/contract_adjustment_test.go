package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// What a contract's price is owed (§6.11, G5/G6).
//
// The gate under test: a suggested price only exists when every month of the
// series is present. A number built on a gap is quietly low, and quietly low is
// what gets quoted to a customer who then holds the correct figure.

type fakeIndexRepo struct {
	values []domain.IndexValue
	latest *time.Time
	upsert []domain.IndexValue
}

func (f *fakeIndexRepo) Range(_ context.Context, code domain.IndexCode, from, to time.Time) ([]domain.IndexValue, error) {
	var out []domain.IndexValue
	for _, v := range f.values {
		if v.Code != code {
			continue
		}
		m := domain.FirstOfMonth(v.ReferenceMonth)
		if m.Before(domain.FirstOfMonth(from)) || m.After(domain.FirstOfMonth(to)) {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

func (f *fakeIndexRepo) Upsert(_ context.Context, values []domain.IndexValue) error {
	f.upsert = append(f.upsert, values...)
	return nil
}

func (f *fakeIndexRepo) LatestMonth(_ context.Context, _ domain.IndexCode) (*time.Time, error) {
	return f.latest, nil
}

type fakeIndexFetcher struct {
	from   time.Time
	values []domain.IndexValue
	err    error
}

func (f *fakeIndexFetcher) Fetch(_ context.Context, code domain.IndexCode, from time.Time) ([]domain.IndexValue, error) {
	f.from = from
	if f.err != nil {
		return nil, f.err
	}
	out := make([]domain.IndexValue, 0, len(f.values))
	for _, v := range f.values {
		v.Code = code
		out = append(out, v)
	}
	return out, nil
}

func monthsOf(code domain.IndexCode, first time.Time, rates ...float64) []domain.IndexValue {
	out := make([]domain.IndexValue, 0, len(rates))
	for i, rate := range rates {
		out = append(out, domain.IndexValue{
			Code: code, ReferenceMonth: first.AddDate(0, i, 0), MonthlyRate: rate,
		})
	}
	return out
}

func adjustmentUC(values []domain.IndexValue, now time.Time) *ContractAdjustmentUseCase {
	uc := NewContractAdjustmentUseCase(newFakeContractRepo(), &fakeIndexRepo{values: values})
	uc.SetClock(func() time.Time { return now })
	return uc
}

func TestContractAdjustment_PricesTheRenegotiation(t *testing.T) {
	march := 3
	now := time.Date(2026, time.April, 10, 0, 0, 0, 0, time.UTC)
	start := time.Date(2025, time.February, 1, 0, 0, 0, 0, time.UTC)

	// March 2025 through March 2026. The adjustment was due in March and it is
	// now April, so the claim runs to the last CLOSED month — an overdue increase
	// keeps growing while nobody acts, and counting only to the missed date would
	// understate what is owed.
	values := monthsOf(domain.IndexIPCA,
		time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC),
		0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5)

	contract := &domain.Contract{
		StartsOn: &start, AdjustmentMonth: &march, AdjustmentIndex: "IPCA",
		ValueCents: 10000000, // R$ 100.000
	}

	adj := adjustmentUC(values, now).For(context.Background(), contract)
	if adj == nil {
		t.Fatal("a contract with an index and a month is owed an adjustment")
	}
	if adj.DueOn.Format("2006-01") != "2026-03" {
		t.Fatalf("due = %v, want 2026-03", adj.DueOn)
	}
	// April, looking at a March date that never happened.
	if !adj.Overdue {
		t.Fatal("a passed adjustment date with no raise recorded is overdue — " +
			"that is the state costing money every month it stays true")
	}
	if adj.Accumulation == nil || !adj.Accumulation.Complete {
		t.Fatalf("accumulation incomplete: %+v", adj.Accumulation)
	}
	// 1.005^13 = 6.70%
	if adj.Accumulation.Months != 13 {
		t.Fatalf("months = %d, want 13", adj.Accumulation.Months)
	}
	if adj.Accumulation.Percent < 6.69 || adj.Accumulation.Percent > 6.71 {
		t.Fatalf("percent = %.4f, want ~6.70", adj.Accumulation.Percent)
	}
	if adj.SuggestedValueCents == nil || adj.GainCents == nil {
		t.Fatal("a complete series produces a suggested price")
	}
	// R$100.000 → R$106.700, so R$6.700 on the table. That number is the entire
	// pitch of the feature.
	if *adj.SuggestedValueCents != 10670000 {
		t.Fatalf("suggested = %d, want 10670000", *adj.SuggestedValueCents)
	}
	if *adj.GainCents != 670000 {
		t.Fatalf("gain = %d, want 670000", *adj.GainCents)
	}
}

func TestContractAdjustment_NoNumberWithoutTheWholeSeries(t *testing.T) {
	march := 3
	now := time.Date(2026, time.April, 10, 0, 0, 0, 0, time.UTC)
	start := time.Date(2025, time.February, 1, 0, 0, 0, 0, time.UTC)
	contract := &domain.Contract{
		StartsOn: &start, AdjustmentMonth: &march, AdjustmentIndex: "IPCA",
		ValueCents: 10000000,
	}

	// Ten of the thirteen months cached.
	partial := monthsOf(domain.IndexIPCA,
		time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC),
		0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5)

	adj := adjustmentUC(partial, now).For(context.Background(), contract)
	if adj.Accumulation == nil || adj.Accumulation.Complete {
		t.Fatalf("a gap must be reported: %+v", adj.Accumulation)
	}
	if adj.SuggestedValueCents != nil || adj.GainCents != nil {
		t.Fatal("no suggested price from a partial series")
	}
	// The date still fires: the reminder is a fact even when the percentage is not.
	if adj.DueOn.IsZero() || !adj.Overdue {
		t.Fatalf("the alert still stands: %+v", adj)
	}
}

func TestContractAdjustment_UnknownIndexStillAlertsWithoutAPercentage(t *testing.T) {
	june := 6
	now := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	start := time.Date(2025, time.June, 1, 0, 0, 0, 0, time.UTC)

	// "Tabela própria do fornecedor" is not an index anybody can look up.
	contract := &domain.Contract{
		StartsOn: &start, AdjustmentMonth: &june,
		AdjustmentIndex: "tabela própria", ValueCents: 10000000,
	}
	adj := adjustmentUC(nil, now).For(context.Background(), contract)
	if adj == nil || adj.Accumulation != nil {
		t.Fatalf("an unrecognised index carries no accumulation: %+v", adj)
	}
	if adj.SuggestedValueCents != nil {
		t.Fatal("and no suggested price")
	}
	if adj.DueOn.Format("2006-01") != "2026-06" {
		t.Fatalf("the date is still a fact: %v", adj.DueOn)
	}

	// No stated month at all: nothing is owed and nothing is invented.
	if got := adjustmentUC(nil, now).For(context.Background(),
		&domain.Contract{StartsOn: &start, AdjustmentIndex: "IPCA"}); got != nil {
		t.Fatalf("a contract with no adjustment month gets no adjustment: %+v", got)
	}
}

// An overdue adjustment keeps accumulating: the money owed grows every month
// nobody acts, and stopping the count at the missed date would understate it.
func TestContractAdjustment_OverdueKeepsAccumulating(t *testing.T) {
	march := 3
	// Six months past the missed March adjustment.
	now := time.Date(2026, time.September, 15, 0, 0, 0, 0, time.UTC)
	start := time.Date(2025, time.February, 1, 0, 0, 0, 0, time.UTC)

	// March 2025 → August 2026 is 18 months of series.
	values := monthsOf(domain.IndexIPCA,
		time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC),
		0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5,
		0.5, 0.5, 0.5, 0.5, 0.5, 0.5)

	adj := adjustmentUC(values, now).For(context.Background(), &domain.Contract{
		StartsOn: &start, AdjustmentMonth: &march, AdjustmentIndex: "IPCA",
		ValueCents: 10000000,
	})
	if adj.Accumulation == nil || !adj.Accumulation.Complete {
		t.Fatalf("accumulation incomplete: %+v", adj.Accumulation)
	}
	// Counted to August 2026 (the last closed month), not to February 2026.
	if adj.Accumulation.Months != 18 {
		t.Fatalf("months = %d, want 18 — an overdue adjustment counts to the "+
			"latest closed month, not to the date it was missed",
			adj.Accumulation.Months)
	}
}

func TestRefreshIndices_ResumesFromTheCacheAndOverlapsOneMonth(t *testing.T) {
	now := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)
	latest := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	repo := &fakeIndexRepo{latest: &latest}
	fetcher := &fakeIndexFetcher{values: monthsOf(domain.IndexIPCA, latest, 0.3, 0.4)}

	uc := NewContractAdjustmentUseCase(newFakeContractRepo(), repo)
	uc.SetClock(func() time.Time { return now })
	uc.SetFetcher(fetcher)

	if err := uc.RefreshIndices(context.Background(), []domain.IndexCode{domain.IndexIPCA}); err != nil {
		t.Fatalf("RefreshIndices: %v", err)
	}
	// One month of overlap: BCB revises the most recent figure once, and
	// re-reading it is how the revision lands.
	if got := fetcher.from.Format("2006-01"); got != "2026-05" {
		t.Fatalf("fetched from %s, want 2026-05 (one month before the cache head)", got)
	}
	if len(repo.upsert) != 2 {
		t.Fatalf("stored %d months, want 2", len(repo.upsert))
	}

	// With no fetcher the refresh says so rather than silently doing nothing.
	bare := NewContractAdjustmentUseCase(newFakeContractRepo(), repo)
	if err := bare.RefreshIndices(context.Background(), []domain.IndexCode{domain.IndexIPCA}); err == nil {
		t.Fatal("an unwired fetcher must report itself")
	}
}

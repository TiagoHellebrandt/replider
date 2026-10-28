package usecases

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// What a contract's price is owed (§6.11, G5/G6).
//
// This is the number that pays for the product: a supply contract whose index
// was never applied loses the difference every month, and the loss is permanent
// because the base it compounds from never resets. R$6M of contracted revenue
// with half the contracts frozen for a year is ~R$135k gone and not coming back.
//
// The rule the whole file obeys: never present a percentage built on an
// incomplete series. A number that is quietly low gets quoted to a customer, and
// then the customer is the one holding the correct figure.

// IndexFetcher reads a published series. The BCB adapter is the production
// implementation; tests substitute canned months.
type IndexFetcher interface {
	Fetch(ctx context.Context, code domain.IndexCode, from time.Time) ([]domain.IndexValue, error)
}

type ContractAdjustmentUseCase struct {
	contracts domain.ContractRepository
	indices   domain.EconomicIndexRepository
	fetcher   IndexFetcher

	now func() time.Time
}

func NewContractAdjustmentUseCase(
	contracts domain.ContractRepository, indices domain.EconomicIndexRepository,
) *ContractAdjustmentUseCase {
	return &ContractAdjustmentUseCase{
		contracts: contracts, indices: indices, now: time.Now,
	}
}

// SetFetcher wires the live series. Without it the accumulation still works off
// whatever months are cached — and reports itself incomplete rather than
// pretending the gap is zero.
func (uc *ContractAdjustmentUseCase) SetFetcher(f IndexFetcher) { uc.fetcher = f }

func (uc *ContractAdjustmentUseCase) SetClock(now func() time.Time) { uc.now = now }

// For computes the adjustment one contract is owed.
//
// Returns nil when the contract states no adjustment month or no index: a price
// that nobody agreed to raise does not get a suggestion to raise it.
func (uc *ContractAdjustmentUseCase) For(
	ctx context.Context, contract *domain.Contract,
) *domain.ContractAdjustment {
	if contract == nil {
		return nil
	}
	due := contract.NextAdjustmentDate(uc.now())
	if due == nil {
		return nil
	}
	code := domain.NormalizeIndexCode(contract.AdjustmentIndex)

	base := contract.AdjustmentBaseDate()
	adj := &domain.ContractAdjustment{
		BaseDate:          base,
		DueOn:             *due,
		Overdue:           due.Before(domain.FirstOfMonth(uc.now())),
		CurrentValueCents: contract.ValueCents,
	}
	if code == "" {
		// The contract names a month but not a recognisable index (or names one
		// we do not read). The alert still fires — the date is a fact — and
		// carries no percentage, which is the honest half of it.
		return adj
	}

	// The accumulation covers the month AFTER the base date through the month
	// BEFORE the adjustment takes effect: the index published for March is
	// what a March adjustment applies, and February's figure is the last one
	// available when it happens.
	from := domain.FirstOfMonth(base).AddDate(0, 1, 0)
	to := domain.FirstOfMonth(*due).AddDate(0, -1, 0)
	// An overdue adjustment accumulates to the latest CLOSED month instead of a
	// date in the past: the money owed keeps growing while nobody acts, and
	// stopping the count at the missed date would understate it.
	if latestClosed := domain.FirstOfMonth(uc.now()).AddDate(0, -1, 0); to.Before(latestClosed) && adj.Overdue {
		to = latestClosed
	}

	values, err := uc.indices.Range(ctx, code, from, to)
	if err != nil {
		slog.ErrorContext(ctx, "contract adjustment: index range read failed",
			"index", code, "error", err)
		return adj
	}
	adj.Accumulation = domain.AccumulateIndex(code, values, from, to)

	// The suggested price exists only when every month is present. That gate is
	// the difference between a renegotiation and a mistake.
	if percent := adj.Accumulation.UsablePercent(); percent != nil && contract.ValueCents > 0 {
		suggested := int64(math.Round(float64(contract.ValueCents) * (1 + *percent/100)))
		gain := suggested - contract.ValueCents
		adj.SuggestedValueCents = &suggested
		adj.GainCents = &gain
	}
	return adj
}

// RefreshIndices pulls the months this deployment is missing.
//
// Resumes from the newest cached month rather than re-reading the series from
// 1994 on every tick: the older months never change, and a public API deserves
// the same restraint as a paid one.
func (uc *ContractAdjustmentUseCase) RefreshIndices(ctx context.Context, codes []domain.IndexCode) error {
	if uc.fetcher == nil {
		return &domain.AppError{
			Code:    "INDEX_FETCHER_UNAVAILABLE",
			Message: "fonte de índices não configurada",
		}
	}
	for _, code := range codes {
		from := time.Date(uc.now().Year()-6, 1, 1, 0, 0, 0, 0, time.UTC)
		// Six years back on a cold cache: long enough to cover a contract signed
		// five years ago that was never adjusted, which is exactly the customer
		// with the most money on the table.
		if latest, err := uc.indices.LatestMonth(ctx, code); err == nil && latest != nil {
			// One month of overlap, deliberately: BCB revises the most recent
			// figure once, and re-reading it is how the revision lands.
			from = latest.AddDate(0, -1, 0)
		}

		values, err := uc.fetcher.Fetch(ctx, code, from)
		if err != nil {
			slog.WarnContext(ctx, "contract adjustment: index fetch failed",
				"index", code, "error", err)
			continue
		}
		if len(values) == 0 {
			continue
		}
		if err := uc.indices.Upsert(ctx, values); err != nil {
			slog.ErrorContext(ctx, "contract adjustment: index upsert failed",
				"index", code, "error", err)
		}
	}
	return nil
}

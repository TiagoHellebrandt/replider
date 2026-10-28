package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// ContractSweepWorker fills the buying calendar.
//
// A public contract carries a published end date, so the purchase that
// replaces it is a known event three to six months before any tender exists.
// That is the only way in this market to arrive while the specification is
// still being written rather than after — by the time an edital is published
// it has usually been drafted around whoever supplied the reference quote.
//
// Until now the contracts table was filled ONLY as a side effect of drafting:
// one buyer at a time, when a lead happened to match a CNES establishment
// carrying a CNPJ. Production had zero rows. This walks the territory on
// purpose.
//
// The sweep is BY BUYER, and that is forced by the source: PNCP's /contratos
// endpoint silently ignores its own `uf` parameter (verified against the live
// API on 2026-09-11 — uf=MT returned contracts from SC, SP, PA and CE), so a
// date walk would mean reading the entire country, 234.769 contracts in forty
// days. `cnpjOrgao` does filter, and CNES already gives every hospital's CNPJ.
type ContractSweepWorker struct {
	contracts domain.PublicContractRepository
	fetcher   contractsForBuyerFetcher
	accounts  sweepTerritoryLister
	cities    sweepCityResolver

	tick time.Duration
	// perTick bounds one pass. The point is to be a good citizen of a free
	// government API, not to finish in one night: a territory refreshes over
	// days and nobody is waiting on the last hospital.
	perTick int
	// pace is the wait between buyers.
	//
	// Two seconds, measured: at 700ms the first live pass took `status 429` on
	// every buyer in Cuiabá, one after another. The portal answers a
	// single-buyer query quickly — the rate limit is what bites, and it bit in
	// September too.
	pace time.Duration
	// giveUpAfter ends a pass once the source has refused this many buyers in
	// a row.
	//
	// A worker that keeps knocking after being told to wait is how a free
	// government API stops answering this IP at all. The territory refreshes
	// over days; losing one pass costs nothing, and losing access costs the
	// whole feature.
	giveUpAfter int
	// staleAfter is how long a buyer's contracts stay fresh. Contracts are
	// signed, not streamed: a fortnight is plenty.
	staleAfter time.Duration

	now   func() time.Time
	sleep func(ctx context.Context, d time.Duration) bool
}

// waitOrStop is the pause between buyers. A field-swappable primitive so tests
// run instantly instead of pacing through a real batch.
func waitOrStop(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// contractsForBuyerFetcher is the one PNCP call this needs — the same method
// the dossier already uses, so there is one place that talks to /contratos.
type contractsForBuyerFetcher interface {
	ContractsForBuyer(ctx context.Context, cnpj string) ([]domain.PublicContract, error)
}

// sweepTerritoryLister yields the territories worth sweeping. One row per
// distinct (uf, cities) an account actually sells into — sweeping is shared
// work, because a contract in Mato Grosso is the same contract for everybody
// who sells there.
type sweepTerritoryLister interface {
	ListTerritories(ctx context.Context) ([]domain.Territory, error)
}

// sweepCityResolver turns city names into the IBGE codes the establishment
// table is keyed by.
type sweepCityResolver interface {
	CityCodesByName(uf string, names []string) ([]string, error)
}

func NewContractSweepWorker(
	contracts domain.PublicContractRepository,
	fetcher contractsForBuyerFetcher,
	accounts sweepTerritoryLister,
) *ContractSweepWorker {
	return &ContractSweepWorker{
		contracts:   contracts,
		fetcher:     fetcher,
		accounts:    accounts,
		tick:        6 * time.Hour,
		perTick:     120,
		pace:        2 * time.Second,
		giveUpAfter: 5,
		staleAfter:  14 * 24 * time.Hour,
		now:         time.Now,
		sleep:       waitOrStop,
	}
}

// SetCityResolver narrows the sweep to an account's cities. Without it the
// whole state is swept, which is correct but slower to become useful.
func (w *ContractSweepWorker) SetCityResolver(r sweepCityResolver) { w.cities = r }

// Start runs the sweep on its own cadence.
func (w *ContractSweepWorker) Start(ctx context.Context) {
	// A pass on boot, then on the tick. A deploy should not cost a day of
	// calendar freshness.
	w.RunOnce(ctx)
	t := time.NewTicker(w.tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.RunOnce(ctx)
		}
	}
}

// RunOnce sweeps one bounded batch of buyers.
func (w *ContractSweepWorker) RunOnce(ctx context.Context) {
	if w.contracts == nil || w.fetcher == nil || w.accounts == nil {
		return
	}
	territories, err := w.accounts.ListTerritories(ctx)
	if err != nil {
		slog.WarnContext(ctx, "contract sweep: territory read failed", "error", err)
		return
	}
	if len(territories) == 0 {
		return
	}

	states, codes := w.scope(territories)
	if len(states) == 0 {
		return
	}

	targets, err := w.contracts.BuyersNeedingScan(
		ctx, states, codes, w.now().Add(-w.staleAfter), w.perTick)
	if err != nil {
		slog.WarnContext(ctx, "contract sweep: buyer list failed", "error", err)
		return
	}
	if len(targets) == 0 {
		return
	}

	var asked, stored, failed, inARow int
	for i, t := range targets {
		if ctx.Err() != nil {
			return
		}
		if i > 0 && !w.sleep(ctx, w.pace) {
			return
		}
		asked++
		found, err := w.fetcher.ContractsForBuyer(ctx, t.CNPJ)
		if err != nil {
			failed++
			inARow++
			// No scan stamp on failure: a buyer we could not read is a buyer
			// still owed a read, and stamping it would hide them for a
			// fortnight behind one bad afternoon.
			slog.WarnContext(ctx, "contract sweep: buyer read failed",
				"cnpj", t.CNPJ, "buyer", t.Name, "error", err)
			// The source is refusing, not this one buyer. Walking the
			// remaining hundred to collect a hundred more refusals is how an
			// IP gets banned — and the fetcher has already backed off and
			// retried before giving up, so this is the second no.
			if inARow >= w.giveUpAfter {
				slog.WarnContext(ctx, "contract sweep: source refusing, ending pass",
					"consecutive_failures", inARow, "asked", asked)
				return
			}
			continue
		}
		inARow = 0
		if len(found) > 0 {
			if err := w.contracts.Save(ctx, found); err != nil {
				failed++
				slog.WarnContext(ctx, "contract sweep: save failed",
					"cnpj", t.CNPJ, "error", err)
				continue
			}
			stored += len(found)
		}
		// Stamped even when empty: "this buyer publishes nothing" is an
		// answer worth remembering for a fortnight, and re-asking every tick
		// would spend the whole budget on the silent ones.
		if err := w.contracts.MarkScanned(ctx, t.CNPJ, len(found)); err != nil {
			slog.WarnContext(ctx, "contract sweep: scan stamp failed",
				"cnpj", t.CNPJ, "error", err)
		}
	}

	slog.InfoContext(ctx, "contract sweep: pass complete",
		"buyers_asked", asked, "contracts_stored", stored, "failed", failed,
		"states", states)
}

// scope collapses every account's territory into one set of states and one set
// of city codes. Shared on purpose: the contracts of a hospital in Cuiabá are
// the same public record for every supplier who sells there, so two accounts
// in the same state cost one sweep, not two.
func (w *ContractSweepWorker) scope(territories []domain.Territory) ([]string, []string) {
	seenUF := map[string]bool{}
	seenCity := map[string]bool{}
	states := make([]string, 0, len(territories))
	codes := make([]string, 0)
	// An account that sells to a WHOLE state must not be narrowed by another
	// account's city list, so one state-wide territory drops the city narrow
	// for that state entirely.
	wholeState := false

	for _, t := range territories {
		uf := domain.NormalizeUF(t.UF)
		if uf == "" {
			continue
		}
		if !seenUF[uf] {
			seenUF[uf] = true
			states = append(states, uf)
		}
		if len(t.Cities) == 0 || w.cities == nil {
			wholeState = true
			continue
		}
		found, err := w.cities.CityCodesByName(uf, t.Cities)
		if err != nil || len(found) == 0 {
			// A territory whose cities we cannot resolve is a territory we
			// sweep whole rather than skip.
			wholeState = true
			continue
		}
		for _, c := range found {
			if !seenCity[c] {
				seenCity[c] = true
				codes = append(codes, c)
			}
		}
	}
	if wholeState {
		return states, nil
	}
	return states, codes
}

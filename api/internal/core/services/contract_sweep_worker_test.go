package services

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

type sweepRepo struct {
	targets []domain.ContractScanTarget
	listErr error

	saved   []domain.PublicContract
	stamped []string
	saveErr error

	gotStates []string
	gotCities []string
}

func (r *sweepRepo) ListByBuyer(context.Context, string, int) ([]domain.PublicContract, error) {
	return nil, nil
}
func (r *sweepRepo) Save(_ context.Context, items []domain.PublicContract) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.saved = append(r.saved, items...)
	return nil
}
func (r *sweepRepo) LastScan(context.Context, string) (time.Time, error) { return time.Time{}, nil }
func (r *sweepRepo) MarkScanned(_ context.Context, cnpj string, _ int) error {
	r.stamped = append(r.stamped, cnpj)
	return nil
}
func (r *sweepRepo) ListExpiring(context.Context, domain.ContractCalendarFilter) ([]domain.PublicContract, error) {
	return nil, nil
}
func (r *sweepRepo) BuyersNeedingScan(
	_ context.Context, states, cities []string, _ time.Time, _ int,
) ([]domain.ContractScanTarget, error) {
	r.gotStates, r.gotCities = states, cities
	return r.targets, r.listErr
}

type sweepFetcher struct {
	byCNPJ map[string][]domain.PublicContract
	fail   map[string]bool
	asked  []string
}

func (f *sweepFetcher) ContractsForBuyer(_ context.Context, cnpj string) ([]domain.PublicContract, error) {
	f.asked = append(f.asked, cnpj)
	if f.fail[cnpj] {
		return nil, errors.New("pncp down")
	}
	return f.byCNPJ[cnpj], nil
}

type sweepTerritories struct {
	list []domain.Territory
	err  error
}

func (s sweepTerritories) ListTerritories(context.Context) ([]domain.Territory, error) {
	return s.list, s.err
}

type sweepCities struct{ codes map[string][]string }

func (s sweepCities) CityCodesByName(uf string, _ []string) ([]string, error) {
	return s.codes[uf], nil
}

func newSweep(repo *sweepRepo, f *sweepFetcher, terr sweepTerritoryLister) *ContractSweepWorker {
	w := NewContractSweepWorker(repo, f, terr)
	w.pace = 0
	w.sleep = func(context.Context, time.Duration) bool { return true }
	return w
}

// The calendar only exists if somebody fills it. Production held zero contract
// rows for months because the table was written ONLY as a side effect of
// drafting — one buyer, if a lead happened to match a CNES establishment
// carrying a CNPJ.
func TestContractSweep_StoresWhatItReadsAndStampsEveryBuyer(t *testing.T) {
	expires := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	repo := &sweepRepo{targets: []domain.ContractScanTarget{
		{CNPJ: "03507415002864", Name: "Hospital A", UF: "MT"},
		{CNPJ: "18457226000181", Name: "Hospital B", UF: "MT"},
	}}
	fetch := &sweepFetcher{byCNPJ: map[string][]domain.PublicContract{
		"03507415002864": {{
			PNCPID: "c1", BuyerCNPJ: "03507415002864",
			Object: "monitor multiparametrico", ExpiresAt: &expires,
		}},
		// B publishes nothing. A real and common answer.
	}}
	w := newSweep(repo, fetch, sweepTerritories{list: []domain.Territory{{UF: "MT"}}})

	w.RunOnce(context.Background())

	assert.Equal(t, []string{"03507415002864", "18457226000181"}, fetch.asked)
	require.Len(t, repo.saved, 1)
	assert.Equal(t, "c1", repo.saved[0].PNCPID)
	// BOTH stamped, including the silent one: "this buyer publishes nothing"
	// is an answer worth remembering, or the budget goes on re-asking them.
	assert.ElementsMatch(t, []string{"03507415002864", "18457226000181"}, repo.stamped)
}

// A buyer we could not read is a buyer still owed a read. Stamping on failure
// would hide them for a fortnight behind one bad afternoon at the portal —
// and the portal has bad afternoons, which is how this product learned about
// rate limits in the first place.
func TestContractSweep_AFailedReadIsNotStamped(t *testing.T) {
	repo := &sweepRepo{targets: []domain.ContractScanTarget{
		{CNPJ: "03507415002864", UF: "MT"},
		{CNPJ: "18457226000181", UF: "MT"},
	}}
	fetch := &sweepFetcher{
		fail:   map[string]bool{"03507415002864": true},
		byCNPJ: map[string][]domain.PublicContract{"18457226000181": {{PNCPID: "c2", BuyerCNPJ: "18457226000181"}}},
	}
	w := newSweep(repo, fetch, sweepTerritories{list: []domain.Territory{{UF: "MT"}}})

	w.RunOnce(context.Background())

	assert.Equal(t, []string{"18457226000181"}, repo.stamped)
	// And one failure does not end the pass: the next buyer is still read.
	require.Len(t, repo.saved, 1)
}

// Sweeping is shared work. A hospital's public contracts are the same public
// record for every supplier in that state, so two accounts in Mato Grosso must
// cost one sweep and not two.
func TestContractSweep_CollapsesTerritoriesIntoOneScope(t *testing.T) {
	repo := &sweepRepo{}
	w := newSweep(repo, &sweepFetcher{}, sweepTerritories{list: []domain.Territory{
		{UF: "MT", Cities: []string{"Cuiabá"}},
		{UF: "mt", Cities: []string{"Sinop"}},
		{UF: "GO", Cities: []string{"Goiânia"}},
		{UF: "XX"}, // not a state; must not widen the sweep
	}})
	w.SetCityResolver(sweepCities{codes: map[string][]string{
		"MT": {"5103403"}, "GO": {"5208707"},
	}})

	w.RunOnce(context.Background())

	assert.ElementsMatch(t, []string{"MT", "GO"}, repo.gotStates)
	assert.NotContains(t, repo.gotStates, "XX")
}

// An account selling to a WHOLE state must not be narrowed by another
// account's city list — that would silently stop sweeping most of its
// territory for the benefit of somebody else's.
func TestContractSweep_OneStateWideTerritoryDropsTheCityNarrow(t *testing.T) {
	repo := &sweepRepo{}
	w := newSweep(repo, &sweepFetcher{}, sweepTerritories{list: []domain.Territory{
		{UF: "MT", Cities: []string{"Cuiabá"}},
		{UF: "MT"}, // sells to the whole state
	}})
	w.SetCityResolver(sweepCities{codes: map[string][]string{"MT": {"5103403"}}})

	w.RunOnce(context.Background())

	assert.Equal(t, []string{"MT"}, repo.gotStates)
	assert.Empty(t, repo.gotCities, "the whole state, not one town in it")
}

// No territory anywhere means no sweep. Spending a free government API's
// patience on nobody's behalf is how a shared resource gets taken away.
func TestContractSweep_NoTerritoryAsksNothing(t *testing.T) {
	repo := &sweepRepo{}
	fetch := &sweepFetcher{}
	w := newSweep(repo, fetch, sweepTerritories{list: nil})

	w.RunOnce(context.Background())

	assert.Empty(t, fetch.asked)
}

// The source refusing is not one buyer failing.
//
// The first live pass took `status 429` on every buyer in Cuiabá, one after
// another, and the worker walked the whole batch collecting refusals. A worker
// that keeps knocking after being told to wait is how a free government API
// stops answering this IP at all — and by then the fetcher has already backed
// off and retried, so each of these is the second no.
func TestContractSweep_StopsWhenTheSourceIsRefusing(t *testing.T) {
	targets := make([]domain.ContractScanTarget, 0, 40)
	fail := map[string]bool{}
	for i := 0; i < 40; i++ {
		cnpj := fmt.Sprintf("0350741500%04d", i)
		targets = append(targets, domain.ContractScanTarget{CNPJ: cnpj, UF: "MT"})
		fail[cnpj] = true
	}
	repo := &sweepRepo{targets: targets}
	fetch := &sweepFetcher{fail: fail}
	w := newSweep(repo, fetch, sweepTerritories{list: []domain.Territory{{UF: "MT"}}})

	w.RunOnce(context.Background())

	assert.Len(t, fetch.asked, w.giveUpAfter,
		"the pass ends at the give-up threshold instead of walking forty refusals")
	assert.Empty(t, repo.stamped, "a buyer we could not read is still owed a read")
}

// A run that recovers must not end early. The counter is CONSECUTIVE
// failures — an account with one flaky buyer among thirty is a normal pass,
// not an outage.
func TestContractSweep_AnIsolatedFailureDoesNotEndThePass(t *testing.T) {
	targets := make([]domain.ContractScanTarget, 0, 12)
	byCNPJ := map[string][]domain.PublicContract{}
	for i := 0; i < 12; i++ {
		cnpj := fmt.Sprintf("0350741500%04d", i)
		targets = append(targets, domain.ContractScanTarget{CNPJ: cnpj, UF: "MT"})
		byCNPJ[cnpj] = []domain.PublicContract{{PNCPID: cnpj + "-c", BuyerCNPJ: cnpj}}
	}
	// Four scattered failures, never five in a row.
	fail := map[string]bool{
		targets[2].CNPJ: true, targets[5].CNPJ: true,
		targets[6].CNPJ: true, targets[9].CNPJ: true,
	}
	repo := &sweepRepo{targets: targets}
	fetch := &sweepFetcher{byCNPJ: byCNPJ, fail: fail}
	w := newSweep(repo, fetch, sweepTerritories{list: []domain.Territory{{UF: "MT"}}})

	w.RunOnce(context.Background())

	assert.Len(t, fetch.asked, 12, "every buyer in the batch was still asked about")
	assert.Len(t, repo.stamped, 8, "only the ones that answered are stamped")
}

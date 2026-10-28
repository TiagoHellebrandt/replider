package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCNESStore struct {
	hasBeds        map[string]bool
	hasHabs        map[string]bool
	hasServs       map[string]bool
	habRows        []domain.CNESHabilitationRow
	servRows       []domain.CNESServiceRow
	identityFilled []domain.CNESEquipmentRow
	bedRows        []domain.CNESBedRow
	bedLoads       []string
	bedErr         error
	runs           map[string]bool // uf+comp
	replaced       []string        // "uf comp rows"
	pruned         []string
	missing        []string
	upserted       []string
	territoryUFs   []string
}

func (f *fakeCNESStore) HasRun(uf, comp string) (bool, error) { return f.runs[uf+comp], nil }
func (f *fakeCNESStore) ReplaceCompetencia(uf, comp string, rows []domain.CNESEquipmentRow) error {
	f.replaced = append(f.replaced, uf+" "+comp)
	f.runs[uf+comp] = true
	return nil
}
func (f *fakeCNESStore) PruneOldCompetencias(uf string, keep int) error {
	f.pruned = append(f.pruned, uf)
	return nil
}
func (f *fakeCNESStore) MissingEstablishments(uf string, _ time.Time, _ int) ([]string, error) {
	return f.missing, nil
}
func (f *fakeCNESStore) UpsertEstablishment(e *domain.CNESEstablishment) error {
	f.upserted = append(f.upserted, e.CNES)
	return nil
}
func (f *fakeCNESStore) TerritoryUFs() ([]string, error) { return f.territoryUFs, nil }

func (f *fakeCNESStore) ReplaceBedsCompetencia(uf, comp string, rows []domain.CNESBedRow) error {
	f.bedRows = append(f.bedRows, rows...)
	f.bedLoads = append(f.bedLoads, uf+comp)
	return f.bedErr
}

func (f *fakeCNESStore) PruneOldBedCompetencias(string, int) error { return nil }

func (f *fakeCNESStore) HasBeds(uf, comp string) (bool, error) {
	return f.hasBeds[uf+comp], nil
}

func (f *fakeCNESStore) HasHabilitations(uf, comp string) (bool, error) {
	return f.hasHabs[uf+comp], nil
}

func (f *fakeCNESStore) HasServices(uf, comp string) (bool, error) {
	return f.hasServs[uf+comp], nil
}

func (f *fakeCNESStore) ReplaceHabilitationsCompetencia(uf, comp string, rows []domain.CNESHabilitationRow) error {
	f.habRows = append(f.habRows, rows...)
	return nil
}

func (f *fakeCNESStore) ReplaceServicesCompetencia(uf, comp string, rows []domain.CNESServiceRow) error {
	f.servRows = append(f.servRows, rows...)
	return nil
}

func (f *fakeCNESStore) PruneOldHabilitations(string, int) error { return nil }
func (f *fakeCNESStore) PruneOldServices(string, int) error      { return nil }

func (f *fakeCNESStore) FillEstablishmentIdentity(rows []domain.CNESEquipmentRow) (int, error) {
	f.identityFilled = append(f.identityFilled, rows...)
	return len(rows), nil
}

type fakeEquipFetcher struct {
	published map[string][]domain.CNESEquipmentRow // uf+comp
	beds      map[string][]domain.CNESBedRow       // uf+comp
	habs      map[string][]domain.CNESHabilitationRow
	servs     map[string][]domain.CNESServiceRow
	bedErr    error
	calls     []string
	bedCalls  []string
}

func (f *fakeEquipFetcher) FetchHabilitationRows(uf, comp string) ([]domain.CNESHabilitationRow, error) {
	rows, ok := f.habs[uf+comp]
	if !ok {
		return nil, domain.ErrCNESCompetenciaNotPublished
	}
	return rows, nil
}

func (f *fakeEquipFetcher) FetchServiceRows(uf, comp string) ([]domain.CNESServiceRow, error) {
	rows, ok := f.servs[uf+comp]
	if !ok {
		return nil, domain.ErrCNESCompetenciaNotPublished
	}
	return rows, nil
}

func (f *fakeEquipFetcher) FetchBedRows(uf, comp string) ([]domain.CNESBedRow, error) {
	f.bedCalls = append(f.bedCalls, uf+" "+comp)
	if f.bedErr != nil {
		return nil, f.bedErr
	}
	rows, ok := f.beds[uf+comp]
	if !ok {
		return nil, domain.ErrCNESCompetenciaNotPublished
	}
	return rows, nil
}

func (f *fakeEquipFetcher) FetchEquipmentRows(uf, comp string) ([]domain.CNESEquipmentRow, error) {
	f.calls = append(f.calls, uf+" "+comp)
	rows, ok := f.published[uf+comp]
	if !ok {
		return nil, domain.ErrCNESCompetenciaNotPublished
	}
	return rows, nil
}

type fakeEstFetcher struct{ fetched []string }

func (f *fakeEstFetcher) FetchEstablishment(_ context.Context, id string) (*domain.CNESEstablishment, error) {
	f.fetched = append(f.fetched, id)
	return &domain.CNESEstablishment{CNES: id, TradeName: "X"}, nil
}

func fixedNow() time.Time { return time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC) }

func TestCNESParkWorker_WalksBackToThePublishedMonth(t *testing.T) {
	store := &fakeCNESStore{runs: map[string]bool{}, territoryUFs: []string{"MT"}}
	// July not out; June is — DATASUS's ~45-day lag on a mid-August tick.
	fetcher := &fakeEquipFetcher{published: map[string][]domain.CNESEquipmentRow{
		"MT202606": {{CNES: "1", EquipCode: "0111", Competencia: "202606"}},
	}}
	w := NewCNESParkWorker(store, fetcher, nil).WithClock(fixedNow)

	w.tick(context.Background())

	assert.Equal(t, []string{"MT 202607", "MT 202606"}, fetcher.calls,
		"tries last month first, walks back on not-published")
	assert.Equal(t, []string{"MT 202606"}, store.replaced)
	assert.Equal(t, []string{"MT"}, store.pruned)
}

// Re-ticks are free — once the month is loaded WHOLE.
//
// The stamp used to be the only question asked, which meant it also answered
// for work added to this loader after it was written. The bed census shipped
// into exactly that gap. A fully loaded month still costs nothing; a
// partially loaded one now finishes itself (see
// TestCNESParkWorker_AStampedMonthStillGetsItsCensus).
func TestCNESParkWorker_IngestedMonthShortCircuits(t *testing.T) {
	store := &fakeCNESStore{
		runs:         map[string]bool{"MT202607": true},
		hasBeds:      map[string]bool{"MT202607": true},
		hasHabs:      map[string]bool{"MT202607": true},
		hasServs:     map[string]bool{"MT202607": true},
		territoryUFs: []string{"MT"},
	}
	fetcher := &fakeEquipFetcher{published: map[string][]domain.CNESEquipmentRow{}}
	w := NewCNESParkWorker(store, fetcher, nil).WithClock(fixedNow)

	w.tick(context.Background())

	assert.Empty(t, fetcher.calls, "a month loaded whole means no download at all")
	assert.Empty(t, fetcher.bedCalls)
	assert.Empty(t, store.replaced)
}

func TestCNESParkWorker_NoTerritoryNoWork(t *testing.T) {
	store := &fakeCNESStore{runs: map[string]bool{}}
	fetcher := &fakeEquipFetcher{}
	w := NewCNESParkWorker(store, fetcher, nil).WithClock(fixedNow)

	w.tick(context.Background())

	assert.Empty(t, fetcher.calls)
}

func TestCNESParkWorker_DefaultBatchFitsTheTickAtThePacing(t *testing.T) {
	w := NewCNESParkWorker(&fakeCNESStore{runs: map[string]bool{}}, &fakeEquipFetcher{}, nil)

	// 1500 × 300 ms = 7.5 min per UF; all 27 UFs ≈ 3.4 h, inside a 24 h tick.
	assert.Equal(t, 1500, w.establishmentsPerRun)
	perUF := time.Duration(w.establishmentsPerRun) * establishmentPacing
	assert.Less(t, 27*perUF, w.pollInterval, "the whole country's batch must finish inside one tick")
}

func TestCNESParkWorker_FillsTheEstablishmentCache(t *testing.T) {
	store := &fakeCNESStore{
		runs:         map[string]bool{"MT202607": true},
		territoryUFs: []string{"MT"},
		missing:      []string{"2655411", "9635890"},
	}
	est := &fakeEstFetcher{}
	w := NewCNESParkWorker(store, &fakeEquipFetcher{}, est).WithClock(fixedNow)
	// The politeness throttle would make this test sleep; the cap of the
	// batch is what matters here, not the pacing.
	w.establishmentsPerRun = 2

	w.tick(context.Background())

	assert.Equal(t, []string{"2655411", "9635890"}, est.fetched)
	assert.Equal(t, []string{"2655411", "9635890"}, store.upserted)
}

// The bed census rides the equipment load's month, and never costs it.
//
// The park is what the radar stands on. A census file that is missing, late
// or malformed has to leave the equipment month committed and simply stay at
// the previous census — which the reader already labels with its own
// competencia.
func TestCNESParkWorker_LoadsTheBedCensusForTheSameMonth(t *testing.T) {
	store := &fakeCNESStore{runs: map[string]bool{}, territoryUFs: []string{"MT"}}
	fetcher := &fakeEquipFetcher{
		published: map[string][]domain.CNESEquipmentRow{
			"MT202607": {{CNES: "2396866", EquipCode: "0111", Competencia: "202607"}},
		},
		beds: map[string][]domain.CNESBedRow{
			"MT202607": {{CNES: "2396866", BedType: "3", BedCode: "81",
				Competencia: "202607", UF: "MT", QtExist: 20}},
		},
	}
	w := NewCNESParkWorker(store, fetcher, nil).WithClock(fixedNow)

	w.tick(context.Background())

	if assert.Len(t, store.bedRows, 1) {
		assert.Equal(t, "81", store.bedRows[0].BedCode)
	}
	assert.Equal(t, []string{"MT202607"}, store.bedLoads,
		"the census must load under the month the equipment load committed")
}

func TestCNESParkWorker_ACensusFailureKeepsTheEquipmentMonth(t *testing.T) {
	store := &fakeCNESStore{runs: map[string]bool{}, territoryUFs: []string{"MT"}}
	fetcher := &fakeEquipFetcher{
		published: map[string][]domain.CNESEquipmentRow{
			"MT202607": {{CNES: "2396866", EquipCode: "0111", Competencia: "202607"}},
		},
		bedErr: errors.New("ftp down"),
	}
	w := NewCNESParkWorker(store, fetcher, nil).WithClock(fixedNow)

	w.tick(context.Background())

	assert.Len(t, store.replaced, 1, "the equipment month must survive a census failure")
	assert.Empty(t, store.bedLoads)
}

// THE BUG THIS SHIPPED WITH, caught in production minutes after the deploy.
//
// The equipment ingest stamp was written before the census existed, so on
// every month already stamped `HasRun` said "done" and the tick returned
// before the census could load. The feature was green in CI, deployed, and
// sat with an empty table — nothing would have loaded until DATASUS published
// a NEW month, up to thirty days later.
//
// A stamp answers for the work it was written about. Whether the census
// exists is a fact about the census.
func TestCNESParkWorker_AStampedMonthStillGetsItsCensus(t *testing.T) {
	store := &fakeCNESStore{
		runs:         map[string]bool{"MT202607": true}, // equipment already in
		hasBeds:      map[string]bool{},                 // census is not
		territoryUFs: []string{"MT"},
	}
	fetcher := &fakeEquipFetcher{
		published: map[string][]domain.CNESEquipmentRow{
			"MT202607": {{CNES: "2396866", EquipCode: "0111", Competencia: "202607",
				EntityCNPJ: "57252971000146"}},
		},
		beds: map[string][]domain.CNESBedRow{
			"MT202607": {{CNES: "2396866", BedType: "3", BedCode: "81",
				Competencia: "202607", UF: "MT", QtExist: 20}},
		},
	}
	w := NewCNESParkWorker(store, fetcher, nil).WithClock(fixedNow)

	w.tick(context.Background())

	require.Len(t, store.bedRows, 1, "a stamped month must still get its census")
	assert.Equal(t, "81", store.bedRows[0].BedCode)
	require.Len(t, store.identityFilled, 1, "and its administrative identity")
	assert.Empty(t, store.replaced,
		"the equipment month is already loaded and must not be rewritten")
}

// Once the census is in, a re-tick costs nothing — the catch-up is for a
// month whose stamp predates these columns, not a second ingest path.
func TestCNESParkWorker_ACensusAlreadyLoadedIsNotRefetched(t *testing.T) {
	store := &fakeCNESStore{
		runs:         map[string]bool{"MT202607": true},
		hasBeds:      map[string]bool{"MT202607": true},
		hasHabs:      map[string]bool{"MT202607": true},
		hasServs:     map[string]bool{"MT202607": true},
		territoryUFs: []string{"MT"},
	}
	fetcher := &fakeEquipFetcher{}
	w := NewCNESParkWorker(store, fetcher, nil).WithClock(fixedNow)

	w.tick(context.Background())

	assert.Empty(t, fetcher.bedCalls)
	assert.Empty(t, fetcher.calls)
}

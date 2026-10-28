package services

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// CNESParkWorker keeps the installed-park tables fresh: for every UF a
// customer territory names, it loads the newest published EQ{UF}{AAMM}.dbc
// from the DATASUS FTP (equipment per establishment), then fills the
// establishment cache from the ministry's open-data API, throttled.
//
// DATASUS publishes monthly with ~45 days of lag, so the worker ticks
// every 24h but almost every tick is a cheap no-op: the (uf, competencia)
// ingest stamp short-circuits before any download. Everything is
// best-effort — a DATASUS outage logs and waits for the next tick, it
// never touches the product pipeline.
type CNESParkWorker struct {
	repo    cnesParkStore
	fetcher equipmentFetcher
	openAPI establishmentFetcher

	pollInterval time.Duration
	// keepCompetencias is how many months stay in cnes_equipment per UF
	// (2 = current + previous, enough for gained-equipment deltas).
	keepCompetencias int
	// establishmentsPerRun caps open-data fetches per tick so a fresh UF
	// (thousands of unknown CNES ids) fills over a few days instead of
	// hammering the API in one go. The math at the default pacing
	// (establishmentPacing = 300ms → ~3 req/s): 1500 × 0.3 s = 450 s =
	// 7.5 min per UF per tick, ×27 UFs worst case ≈ 3.4 h — far inside the
	// 24 h tick, and the repo hands the batch biggest-park-first, so the
	// Radar's top rows get a name on day one.
	establishmentsPerRun int
	// now is injectable for tests.
	now func() time.Time

	running atomic.Bool
}

// cnesParkStore is the slice of domain.CNESParkRepository the worker uses.
type cnesParkStore interface {
	HasRun(uf, competencia string) (bool, error)
	ReplaceCompetencia(uf, competencia string, rows []domain.CNESEquipmentRow) error
	PruneOldCompetencias(uf string, keep int) error
	ReplaceBedsCompetencia(uf, competencia string, rows []domain.CNESBedRow) error
	FillEstablishmentIdentity(rows []domain.CNESEquipmentRow) (int, error)
	HasBeds(uf, competencia string) (bool, error)
	ReplaceHabilitationsCompetencia(uf, competencia string, rows []domain.CNESHabilitationRow) error
	PruneOldHabilitations(uf string, keep int) error
	HasHabilitations(uf, competencia string) (bool, error)
	ReplaceServicesCompetencia(uf, competencia string, rows []domain.CNESServiceRow) error
	PruneOldServices(uf string, keep int) error
	HasServices(uf, competencia string) (bool, error)
	PruneOldBedCompetencias(uf string, keep int) error
	MissingEstablishments(uf string, staleBefore time.Time, limit int) ([]string, error)
	UpsertEstablishment(e *domain.CNESEstablishment) error
	TerritoryUFs() ([]string, error)
}

// defaultEstablishmentsPerRun and establishmentPacing bound the open-data
// top-up per tick: see the establishmentsPerRun field for the arithmetic.
const (
	defaultEstablishmentsPerRun = 1500
	establishmentPacing         = 300 * time.Millisecond
)

type equipmentFetcher interface {
	FetchEquipmentRows(uf, competencia string) ([]domain.CNESEquipmentRow, error)
	FetchBedRows(uf, competencia string) ([]domain.CNESBedRow, error)
	FetchHabilitationRows(uf, competencia string) ([]domain.CNESHabilitationRow, error)
	FetchServiceRows(uf, competencia string) ([]domain.CNESServiceRow, error)
}

type establishmentFetcher interface {
	FetchEstablishment(ctx context.Context, cnesID string) (*domain.CNESEstablishment, error)
}

func NewCNESParkWorker(repo cnesParkStore, fetcher equipmentFetcher, openAPI establishmentFetcher) *CNESParkWorker {
	return &CNESParkWorker{
		repo:                 repo,
		fetcher:              fetcher,
		openAPI:              openAPI,
		pollInterval:         24 * time.Hour,
		keepCompetencias:     2,
		establishmentsPerRun: defaultEstablishmentsPerRun,
		now:                  time.Now,
	}
}

// WithPollInterval overrides the tick cadence (tests use ms-scale).
func (w *CNESParkWorker) WithPollInterval(d time.Duration) *CNESParkWorker {
	w.pollInterval = d
	return w
}

// WithClock injects a fake clock for tests.
func (w *CNESParkWorker) WithClock(now func() time.Time) *CNESParkWorker {
	w.now = now
	return w
}

// Start runs the loop until ctx is cancelled. Once-per-process.
func (w *CNESParkWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("cnes park worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("cnes park worker started", "poll_interval", w.pollInterval)

	w.tick(ctx)
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("cnes park worker stopped")
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

// tick ingests every territory UF that has an unseen competencia, then
// tops up the establishment cache.
func (w *CNESParkWorker) tick(ctx context.Context) {
	ufs, err := w.repo.TerritoryUFs()
	if err != nil {
		slog.Error("cnes park: territory listing failed", "error", err)
		return
	}
	for _, uf := range ufs {
		if ctx.Err() != nil {
			return
		}
		w.ingestUF(uf)
		w.refreshEstablishments(ctx, uf)
	}
}

// ingestUF walks back from last month until it finds a published
// competencia; the ingest stamp makes re-ticks free.
func (w *CNESParkWorker) ingestUF(uf string) {
	for back := 1; back <= 4; back++ {
		comp := w.now().AddDate(0, -back, 0).Format("200601")
		done, err := w.repo.HasRun(uf, comp)
		if err != nil {
			slog.Error("cnes park: run check failed", "uf", uf, "error", err)
			return
		}
		if done {
			// The equipment month is in — but the ingest stamp was written
			// before the census and the identity fill existed, so on every
			// month already stamped it says "done" for work it predates.
			// That is how the census shipped green and sat empty in
			// production: nothing would have loaded until DATASUS published
			// a NEW month, up to thirty days later.
			w.catchUpOnLoadedMonth(uf, comp)
			return
		}
		rows, err := w.fetcher.FetchEquipmentRows(uf, comp)
		if errors.Is(err, domain.ErrCNESCompetenciaNotPublished) {
			continue // month not out yet, try the previous one
		}
		if err != nil {
			slog.Error("cnes park: download failed", "uf", uf, "competencia", comp, "error", err)
			return
		}
		if err := w.repo.ReplaceCompetencia(uf, comp, rows); err != nil {
			slog.Error("cnes park: load failed", "uf", uf, "competencia", comp, "error", err)
			return
		}
		if err := w.repo.PruneOldCompetencias(uf, w.keepCompetencias); err != nil {
			slog.Warn("cnes park: prune failed", "uf", uf, "error", err)
		}
		w.fillIdentity(uf, comp, rows)
		beds, habs, servs := w.loadCompanionFamilies(uf, comp)
		slog.Info("cnes park: competencia loaded",
			"uf", uf, "competencia", comp, "rows", len(rows),
			"beds", beds, "habilitations", habs, "services", servs)
		return
	}
	slog.Warn("cnes park: no published competencia in the last 4 months", "uf", uf)
}

// catchUpOnLoadedMonth does the work the ingest stamp would otherwise hide.
//
// A month already stamped for equipment returns early, which is right for the
// equipment itself and wrong for anything added to this loader afterwards.
// The census is the test: whether it exists for this month is a fact about
// the census, not about the stamp.
func (w *CNESParkWorker) catchUpOnLoadedMonth(uf, comp string) {
	owed, err := w.owedFamilies(uf, comp)
	if err != nil {
		slog.Warn("cnes park: companion check failed", "uf", uf, "error", err)
		return
	}
	if !owed {
		return // nothing owed for this month
	}
	slog.Info("cnes park: catching up on a month loaded before these families existed",
		"uf", uf, "competencia", comp)
	// LOGGED, unlike the first version. The catch-up called loadBeds and threw
	// the count away, so a successful census left no trace at all and the only
	// way to know it had worked was to query the table. A silent success is a
	// thing somebody has to go and check.
	beds, habs, servs := w.loadCompanionFamilies(uf, comp)
	slog.Info("cnes park: catch-up loaded",
		"uf", uf, "competencia", comp,
		"beds", beds, "habilitations", habs, "services", servs)
	// The identity rides the equipment file, so catching it up costs one
	// download. Once per UF per month, and only while there is a month whose
	// stamp predates these columns.
	if rows, err := w.fetcher.FetchEquipmentRows(uf, comp); err == nil {
		w.fillIdentity(uf, comp, rows)
	} else if !errors.Is(err, domain.ErrCNESCompetenciaNotPublished) {
		slog.Warn("cnes park: identity catch-up download failed",
			"uf", uf, "competencia", comp, "error", err)
	}
}

// owedFamilies reports whether any family that ships beside the equipment is
// missing for this month.
//
// Each family answers for itself. The equipment ingest stamp was written
// before any of them existed, so it must never be the thing asked — that is
// the bug the census shipped with.
func (w *CNESParkWorker) owedFamilies(uf, comp string) (bool, error) {
	for _, has := range []func(string, string) (bool, error){
		w.repo.HasBeds, w.repo.HasHabilitations, w.repo.HasServices,
	} {
		got, err := has(uf, comp)
		if err != nil {
			return false, err
		}
		if !got {
			return true, nil
		}
	}
	return false, nil
}

// loadCompanionFamilies brings in the census, the habilitations and the
// specialized services for the month the equipment load committed.
//
// All three are best-effort and all three run: one family being late,
// missing or malformed must not cost the others, and none of them may cost
// the equipment month the radar stands on.
func (w *CNESParkWorker) loadCompanionFamilies(uf, comp string) (beds, habs, servs int) {
	return w.loadBeds(uf, comp), w.loadHabilitations(uf, comp), w.loadServices(uf, comp)
}

// loadHabilitations brings in capability the Ministry funds.
func (w *CNESParkWorker) loadHabilitations(uf, comp string) int {
	rows, err := w.fetcher.FetchHabilitationRows(uf, comp)
	if errors.Is(err, domain.ErrCNESCompetenciaNotPublished) {
		return 0
	}
	if err != nil {
		slog.Warn("cnes habilitations: download failed", "uf", uf, "competencia", comp, "error", err)
		return 0
	}
	if err := w.repo.ReplaceHabilitationsCompetencia(uf, comp, rows); err != nil {
		slog.Warn("cnes habilitations: load failed", "uf", uf, "competencia", comp, "error", err)
		return 0
	}
	if err := w.repo.PruneOldHabilitations(uf, w.keepCompetencias); err != nil {
		slog.Warn("cnes habilitations: prune failed", "uf", uf, "error", err)
	}
	return len(rows)
}

// loadServices brings in what each establishment says it performs.
func (w *CNESParkWorker) loadServices(uf, comp string) int {
	rows, err := w.fetcher.FetchServiceRows(uf, comp)
	if errors.Is(err, domain.ErrCNESCompetenciaNotPublished) {
		return 0
	}
	if err != nil {
		slog.Warn("cnes services: download failed", "uf", uf, "competencia", comp, "error", err)
		return 0
	}
	if err := w.repo.ReplaceServicesCompetencia(uf, comp, rows); err != nil {
		slog.Warn("cnes services: load failed", "uf", uf, "competencia", comp, "error", err)
		return 0
	}
	if err := w.repo.PruneOldServices(uf, w.keepCompetencias); err != nil {
		slog.Warn("cnes services: prune failed", "uf", uf, "error", err)
	}
	return len(rows)
}

// fillIdentity folds the administrative columns of the equipment file into
// the establishment cache. Best-effort: it accelerates what the open-data
// top-up would fetch anyway, so a failure costs latency, never the month.
func (w *CNESParkWorker) fillIdentity(uf, comp string, rows []domain.CNESEquipmentRow) {
	n, err := w.repo.FillEstablishmentIdentity(rows)
	if err != nil {
		slog.Warn("cnes park: identity fill failed", "uf", uf, "error", err)
		return
	}
	if n > 0 {
		slog.Info("cnes park: identity filled from the equipment file",
			"uf", uf, "competencia", comp, "establishments", n)
	}
}

// loadBeds brings in the bed census for the month the equipment load just
// committed. Reports how many rows landed.
//
// BEST-EFFORT, and deliberately after the equipment commit: the park is what
// the radar is built on, and a bed file that is missing, late or malformed
// must not cost the account its equipment month. A failure is logged and the
// next tick retries — the census simply stays at the previous month, which
// the reader already labels.
func (w *CNESParkWorker) loadBeds(uf, comp string) int {
	rows, err := w.fetcher.FetchBedRows(uf, comp)
	if errors.Is(err, domain.ErrCNESCompetenciaNotPublished) {
		slog.Info("cnes beds: census not published for this month",
			"uf", uf, "competencia", comp)
		return 0
	}
	if err != nil {
		slog.Warn("cnes beds: download failed", "uf", uf, "competencia", comp, "error", err)
		return 0
	}
	if err := w.repo.ReplaceBedsCompetencia(uf, comp, rows); err != nil {
		slog.Warn("cnes beds: load failed", "uf", uf, "competencia", comp, "error", err)
		return 0
	}
	if err := w.repo.PruneOldBedCompetencias(uf, w.keepCompetencias); err != nil {
		slog.Warn("cnes beds: prune failed", "uf", uf, "error", err)
	}
	return len(rows)
}

// refreshEstablishments fills the who-is-this cache for CNES ids seen in
// the equipment data, bounded per tick and politely throttled.
func (w *CNESParkWorker) refreshEstablishments(ctx context.Context, uf string) {
	if w.openAPI == nil {
		return
	}
	staleBefore := w.now().AddDate(0, -3, 0)
	ids, err := w.repo.MissingEstablishments(uf, staleBefore, w.establishmentsPerRun)
	if err != nil {
		slog.Error("cnes park: missing-establishment scan failed", "uf", uf, "error", err)
		return
	}
	fetched := 0
	for _, id := range ids {
		if ctx.Err() != nil {
			return
		}
		e, err := w.openAPI.FetchEstablishment(ctx, id)
		if err != nil {
			// One transient API error ends the batch (the rest retries
			// next tick) rather than burning the whole cap on a bad day.
			slog.Warn("cnes park: establishment fetch failed", "cnes", id, "error", err)
			return
		}
		if e == nil {
			// Retired id: cache a stub so it is not re-fetched every tick.
			e = &domain.CNESEstablishment{CNES: id}
		}
		if err := w.repo.UpsertEstablishment(e); err != nil {
			slog.Error("cnes park: establishment upsert failed", "cnes", id, "error", err)
			return
		}
		fetched++
		// ~3 requests/second keeps us a polite consumer of a public API.
		select {
		case <-ctx.Done():
			return
		case <-time.After(establishmentPacing):
		}
	}
	if fetched > 0 {
		slog.Info("cnes park: establishments refreshed", "uf", uf, "count", fetched)
	}
}

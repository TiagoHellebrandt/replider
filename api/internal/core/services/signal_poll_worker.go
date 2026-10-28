package services

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// SignalPollWorker pulls public buying signals and fans them out (§2.1).
//
// One fetch per source per tick, then matched in memory against every
// account's watches. The alternative — querying the source per account —
// would hit an external API once per tenant, which is slow, rude, and the
// fastest way to get an IP rate limited off a free government service.
type SignalPollWorker struct {
	repo     domain.SignalRepository
	fetchers []domain.SignalFetcher
	// offerer, when wired, turns each freshly stored signal into a lead +
	// draft in the approval queue (tese-agente-autonomo). Optional and
	// best-effort; bounded per account per run.
	offerer SignalOfferer
	// watchSeeder, when wired, creates the derived default watch for
	// accounts that have products + territory but never configured one —
	// the autonomous path must not depend on a settings screen.
	watchSeeder WatchSeeder
	// sourceHealth, when wired, records whether each source answered, so the
	// radar can say "we could not read PNCP" instead of silently serving
	// last week's tenders as today's.
	sourceHealth domain.SignalSourceHealthRepository
	interval     time.Duration
	// lookback is how far back each run reads for a watch that has already
	// produced signals.
	//
	// Deliberately wider than the interval: a run that failed, a deploy, or
	// a clock skew must not open a hole in the feed, and re-reading a tender
	// costs nothing because the upsert deduplicates on the source's own id.
	lookback time.Duration
	// backfillLookback is the window for a watch whose account has NEVER
	// stored a signal (see lookbackFor). A tender published last week and
	// closing next week is live work today; a fresh watch that only reads
	// the last two days would wait for the next one to show anything.
	backfillLookback time.Duration
	now              func() time.Time
	// runLog stamps completed passes. The boot pass still FETCHES (a fresh
	// watch must not wait six hours), but its auto-offer step is skipped
	// when the last pass is younger than bootOfferGrace — otherwise every
	// restart drafted up to maxAutoOffersPerAccountPerRun per account.
	// Optional — see WithRunLog.
	runLog domain.WorkerRunRepository
}

// signalPollWorkerName keys the worker_runs stamp.
const signalPollWorkerName = "signal_poll"

// bootOfferGrace is how recent the last pass must be for the boot pass to
// skip auto-offers.
const bootOfferGrace = time.Hour

// WithRunLog wires the worker_runs stamp.
func (w *SignalPollWorker) WithRunLog(r domain.WorkerRunRepository) *SignalPollWorker {
	w.runLog = r
	return w
}

func NewSignalPollWorker(repo domain.SignalRepository, fetchers ...domain.SignalFetcher) *SignalPollWorker {
	return &SignalPollWorker{
		repo:             repo,
		fetchers:         fetchers,
		interval:         6 * time.Hour,
		lookback:         48 * time.Hour,
		backfillLookback: 30 * 24 * time.Hour,
		now:              time.Now,
	}
}

// WithInterval and WithClock exist for tests, which must not wait six hours.
func (w *SignalPollWorker) WithInterval(d time.Duration) *SignalPollWorker {
	w.interval = d
	return w
}

func (w *SignalPollWorker) WithClock(now func() time.Time) *SignalPollWorker {
	w.now = now
	return w
}

// SignalOfferer is the seam to the offer pipeline. Satisfied by
// usecases.SignalOfferUseCase (an interface here because services must not
// import usecases).
type SignalOfferer interface {
	OfferFromSignal(ctx context.Context, s *domain.Signal) error
}

// SetOfferer wires the auto-offer step (optional).
func (w *SignalPollWorker) SetOfferer(o SignalOfferer) { w.offerer = o }

// WatchSeeder creates derived default watches before a poll.
type WatchSeeder interface {
	EnsureDefaultWatches(ctx context.Context)
}

// SetWatchSeeder wires the derived-watch step (optional).
func (w *SignalPollWorker) SetWatchSeeder(ws WatchSeeder) { w.watchSeeder = ws }

// SetSourceHealth wires the per-source outcome stamp (optional).
//
// Without it the worker behaves exactly as before and the screen simply has
// nothing to show — a missing stamp must never be read as "down", or an
// instance deployed before the wiring would tell every customer the radar is
// broken.
func (w *SignalPollWorker) SetSourceHealth(r domain.SignalSourceHealthRepository) {
	w.sourceHealth = r
}

// maxAutoOffersPerAccountPerRun bounds the burst: a fresh watch against a
// 48h lookback can match dozens of tenders, and each auto-offer spends a
// model call. The rest stay in the signals feed for a human to pick.
const maxAutoOffersPerAccountPerRun = 10

// Start runs the loop until the context is cancelled.
func (w *SignalPollWorker) Start(ctx context.Context) {
	if w.repo == nil || len(w.fetchers) == 0 {
		return
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// A first pass immediately: a freshly deployed instance whose customer
	// just configured a watch should not wait six hours to prove it works.
	// Its auto-offer step, though, only runs when the last stamped pass is
	// older than bootOfferGrace — a deploy loop must not draft ten offers
	// per account per restart. Unwired log = the pre-stamp behaviour
	// (offers on every boot), so a deployment that forgot the wiring is
	// loud in the ledger rather than silently idle.
	w.run(ctx, w.bootOffersAllowed(ctx))

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.RunOnce(ctx)
		}
	}
}

// bootOffersAllowed reports whether the boot pass may auto-offer.
func (w *SignalPollWorker) bootOffersAllowed(ctx context.Context) bool {
	if w.runLog == nil {
		return true
	}
	return bootPassDue(ctx, w.runLog, signalPollWorkerName, bootOfferGrace, w.now())
}

// RunOnce does one full pass. Exported so a test — and a future manual
// "check now" button — can drive it without a ticker.
func (w *SignalPollWorker) RunOnce(ctx context.Context) {
	w.run(ctx, true)
}

// run does one pass; withOffers=false fetches and stores but never drafts.
func (w *SignalPollWorker) run(ctx context.Context, withOffers bool) {
	if w.watchSeeder != nil {
		w.watchSeeder.EnsureDefaultWatches(ctx)
	}
	watches, err := w.repo.ListActiveWatches(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "signal poll: could not read watches", "error", err)
		return
	}
	if len(watches) == 0 {
		// Nobody is watching. Fetching anyway would spend a request on a
		// result no account would receive.
		return
	}

	fresh := w.freshAccounts(ctx, watches)
	offerer := w.offerer
	if !withOffers {
		w.offerer = nil
	}
	for _, fetcher := range w.fetchers {
		w.runFetcher(ctx, fetcher, watches, fresh)
	}
	w.offerer = offerer
	stampRun(ctx, w.runLog, signalPollWorkerName, w.now())
}

// freshAccounts says, per account with a watch, whether it has never stored
// a signal. Read once per run: the count is per account, not per watch, and
// an account with three watches must not pay three queries.
//
// A count that fails is read as "not fresh": the steady window is the safe
// default, and the next tick asks again.
func (w *SignalPollWorker) freshAccounts(ctx context.Context, watches []*domain.SignalWatch) map[string]bool {
	fresh := map[string]bool{}
	for _, watch := range watches {
		if _, seen := fresh[watch.AccountID]; seen {
			continue
		}
		n, err := w.repo.CountByAccount(ctx, watch.AccountID)
		if err != nil {
			slog.WarnContext(ctx, "signal poll: could not count an account's signals",
				"account_id", watch.AccountID, "error", err)
			n = 1
		}
		fresh[watch.AccountID] = n == 0
	}
	return fresh
}

// lookbackFor is the backfill rule: an account whose watches have NEVER
// matched anything reads 30 days back this run, so its first feed shows the
// tenders still open rather than only those published since yesterday;
// after the first stored signal it reads the steady 48h like everyone else.
// Storing is what flips it, so a watch that genuinely matches nothing keeps
// backfilling — which is correct, because there is nothing to duplicate.
func (w *SignalPollWorker) lookbackFor(watch *domain.SignalWatch, fresh map[string]bool) time.Duration {
	if fresh[watch.AccountID] {
		return w.backfillLookback
	}
	return w.lookback
}

func (w *SignalPollWorker) runFetcher(
	ctx context.Context,
	fetcher domain.SignalFetcher,
	watches []*domain.SignalWatch,
	fresh map[string]bool,
) {
	source := fetcher.Source()
	// Nobody watching THIS source: skip the fetch entirely. The watches that
	// are interested split by window — one fetch per window, not per
	// account, so the free government API is still hit a fixed number of
	// times per tick regardless of how many tenants watch it.
	byWindow := map[time.Duration][]*domain.SignalWatch{}
	for _, watch := range watches {
		if watch.Source == source {
			d := w.lookbackFor(watch, fresh)
			byWindow[d] = append(byWindow[d], watch)
		}
	}
	if len(byWindow) == 0 {
		return
	}

	offersByAccount := map[string]int{}
	// asked/failed/partial decide the health stamp below. A pass can run two
	// windows (steady + backfill): reaching the source in EITHER is reaching
	// it, so only "asked and every window refused" is an outage — but a
	// window that came back INCOMPLETE is not a clean read either, and the
	// stamp used to record it as one.
	var asked, failed, partial int
	var lastErr error
	for _, lookback := range []time.Duration{w.lookback, w.backfillLookback} {
		interested := byWindow[lookback]
		if len(interested) == 0 {
			continue
		}
		since := w.now().Add(-lookback)
		ufs := watchUFs(interested)
		if len(ufs) == 0 {
			// Nothing in this window names a state. Fetching would fall
			// back to the national walk — the most expensive request this
			// worker can make — on behalf of watches that can never match
			// anything anyway.
			slog.WarnContext(ctx, "signal poll: no scoped watch, skipping fetch",
				"source", string(source), "watches", len(interested))
			continue
		}
		asked++
		found, err := fetcher.Fetch(ctx, since, ufs)
		switch {
		case err == nil:
		case errors.Is(err, domain.ErrSignalSourcePartial):
			// Some of it was read. The rows are kept — they are real
			// tenders — and the pass is recorded as incomplete, because a
			// feed missing part of its source must not present itself as
			// today's full picture.
			partial++
			lastErr = err
			slog.WarnContext(ctx, "signal poll: source read only in part",
				"source", string(source), "lookback", lookback.String(), "error", err)
		default:
			// A public source being down is an ordinary Tuesday. Logged and
			// retried next tick; never fatal to the other sources.
			failed++
			lastErr = err
			slog.WarnContext(ctx, "signal poll: source unavailable",
				"source", string(source), "lookback", lookback.String(), "error", err)
			continue
		}
		w.fanOut(ctx, source, found, interested, offersByAccount)
	}
	if asked > 0 {
		w.recordHealth(ctx, source, failed == asked || partial > 0, lastErr)
	}
}

// recordHealth stamps how this source's pass went. `incomplete` covers both
// "nothing came back" and "some of it came back": either way this pass did
// not read the source, and last_success_at must not move.
//
// Best-effort on purpose: a health table that cannot be written must not stop
// signals from being collected. The failure is logged, and the screen keeps
// showing the previous stamp — stale, but never inverted.
func (w *SignalPollWorker) recordHealth(
	ctx context.Context, source domain.SignalSource, incomplete bool, cause error,
) {
	if w.sourceHealth == nil {
		return
	}
	var outcome error
	if incomplete {
		outcome = cause
		if outcome == nil {
			outcome = errors.New("source unreachable")
		}
	}
	if err := w.sourceHealth.RecordAttempt(ctx, source, w.now(), outcome); err != nil {
		slog.WarnContext(ctx, "signal poll: could not stamp source health",
			"source", string(source), "error", err)
	}
}

// watchUFs is the union of the watches' States, as the fetcher's scope.
//
// An unscoped watch used to mean "anywhere", and ONE of them in the group
// made the whole fetch national on everyone else's behalf. That is how a
// single dogfood row for the word "luva" turned every tick into a 120-request
// walk of the national feed and earned the 429s that emptied the radar. Such
// a watch can no longer be saved (SignalWatch.NormalizeAndValidate) and the
// rows that existed are gone (migration 000234); this skips any that survive
// rather than letting one of them speak for the group again.
func watchUFs(watches []*domain.SignalWatch) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, watch := range watches {
		for _, uf := range domain.NormalizeUFs(watch.States) {
			if _, dup := seen[uf]; dup {
				continue
			}
			seen[uf] = struct{}{}
			out = append(out, uf)
		}
	}
	sort.Strings(out)
	return out
}

// fanOut matches one fetch's result against the watches it was made for and
// stores one row per (account, signal).
func (w *SignalPollWorker) fanOut(
	ctx context.Context,
	source domain.SignalSource,
	found []*domain.Signal,
	interested []*domain.SignalWatch,
	offersByAccount map[string]int,
) {
	var stored int
	for _, signal := range found {
		for _, watch := range interested {
			if !watch.Matches(signal) {
				continue
			}
			// One row per (account, signal): the same tender legitimately
			// interests several accounts, and each needs its own dismissal
			// and lead link.
			row := *signal
			row.ID = ulid.Make().String()
			row.AccountID = watch.AccountID
			if err := w.repo.Upsert(ctx, &row); err != nil {
				slog.WarnContext(ctx, "signal poll: could not store a signal",
					"source", string(source), "external_id", signal.ExternalID, "error", err)
				continue
			}
			stored++
			// Auto-offer: the committed row (Upsert read back id/lead/
			// dismissal) says whether this tender is fresh work. Bounded so
			// a first run cannot burn a day's draft budget in one tick.
			if w.offerer != nil && row.LeadID == "" && row.DismissedAt == nil &&
				offersByAccount[row.AccountID] < maxAutoOffersPerAccountPerRun {
				if err := w.offerer.OfferFromSignal(ctx, &row); err != nil {
					slog.WarnContext(ctx, "signal poll: auto-offer failed",
						"signal_id", row.ID, "error", err)
				} else {
					offersByAccount[row.AccountID]++
				}
			}
		}
	}
	if stored > 0 {
		slog.InfoContext(ctx, "signal poll: stored new buying signals",
			"source", string(source), "count", stored, "watches", len(interested))
	}
}

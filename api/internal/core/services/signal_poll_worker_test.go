package services

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The worker fetches each source ONCE and fans out. Querying per account
// would hit a free government API once per tenant — slow, rude, and the
// fastest way to get rate limited off it.

type fakeSignalRepo struct {
	domain.SignalRepository
	watches   []*domain.SignalWatch
	watchErr  error
	stored    []*domain.Signal
	storeErr  error
	upsertHit int
	// onUpsert lets a test shape the committed row the way the real repo's
	// RETURNING does (e.g. an already-linked lead on a republished tender).
	onUpsert func(*domain.Signal)
	// counts is what CountByAccount answers per account; absent = 0, i.e. a
	// fresh account that gets the backfill window.
	counts   map[string]int
	countErr error
}

func (f *fakeSignalRepo) ListActiveWatches(context.Context) ([]*domain.SignalWatch, error) {
	return f.watches, f.watchErr
}

func (f *fakeSignalRepo) CountByAccount(_ context.Context, accountID string) (int, error) {
	return f.counts[accountID], f.countErr
}

func (f *fakeSignalRepo) Upsert(_ context.Context, s *domain.Signal) error {
	f.upsertHit++
	if f.storeErr != nil {
		return f.storeErr
	}
	if f.onUpsert != nil {
		f.onUpsert(s)
	}
	f.stored = append(f.stored, s)
	return nil
}

type fakeFetcher struct {
	source  domain.SignalSource
	signals []*domain.Signal
	err     error
	calls   int
	// sinces and ufs record each call's window and scope, in order.
	sinces []time.Time
	ufs    [][]string
}

func (f *fakeFetcher) Source() domain.SignalSource { return f.source }

func (f *fakeFetcher) Fetch(_ context.Context, since time.Time, ufs []string) ([]*domain.Signal, error) {
	f.calls++
	f.sinces = append(f.sinces, since)
	f.ufs = append(f.ufs, ufs)
	return f.signals, f.err
}

func monitorTender() *domain.Signal {
	return &domain.Signal{
		Source: domain.SignalPNCP, ExternalID: "pncp-1",
		Title: "Aquisição de monitores multiparâmetro", State: "SP",
		PublishedAt: time.Now(),
	}
}

// pncpWatch is the shape production can actually store: keywords AND a state.
// An unscoped watch is refused on save (domain.SignalWatch
// .NormalizeAndValidate) and skipped by the poll, so a fixture without a UF
// would be testing a row that cannot exist. SP matches monitorTender().
func pncpWatch(accountID string, keywords ...string) *domain.SignalWatch {
	return &domain.SignalWatch{
		ID: "w-" + accountID, AccountID: accountID,
		Source: domain.SignalPNCP, Keywords: keywords,
		States: []string{"SP"}, IsActive: true,
	}
}

func TestSignalPoll_StoresOneRowPerInterestedAccount(t *testing.T) {
	repo := &fakeSignalRepo{watches: []*domain.SignalWatch{
		pncpWatch("acc-1", "monitor"),
		pncpWatch("acc-2", "monitor"),
		pncpWatch("acc-3", "tomógrafo"),
	}}
	fetcher := &fakeFetcher{source: domain.SignalPNCP, signals: []*domain.Signal{monitorTender()}}

	NewSignalPollWorker(repo, fetcher).RunOnce(context.Background())

	// The same tender legitimately interests several accounts, and each
	// needs its own dismissal and lead link — so one row each, not one row
	// shared.
	require.Len(t, repo.stored, 2)
	assert.Equal(t, 1, fetcher.calls, "one fetch, not one per account")
	assert.NotEqual(t, repo.stored[0].ID, repo.stored[1].ID)
}

// Fetching for nobody spends a request on a result no account receives.
func TestSignalPoll_NoWatchesMeansNoFetch(t *testing.T) {
	repo := &fakeSignalRepo{}
	fetcher := &fakeFetcher{source: domain.SignalPNCP, signals: []*domain.Signal{monitorTender()}}

	NewSignalPollWorker(repo, fetcher).RunOnce(context.Background())

	assert.Zero(t, fetcher.calls)
}

// Nobody watching THIS source: skip it, even when other sources have
// watchers.
func TestSignalPoll_SkipsSourcesNobodyWatches(t *testing.T) {
	repo := &fakeSignalRepo{watches: []*domain.SignalWatch{pncpWatch("acc-1", "monitor")}}
	pncp := &fakeFetcher{source: domain.SignalPNCP, signals: []*domain.Signal{monitorTender()}}
	jobs := &fakeFetcher{source: domain.SignalJobPost}

	NewSignalPollWorker(repo, pncp, jobs).RunOnce(context.Background())

	assert.Equal(t, 1, pncp.calls)
	assert.Zero(t, jobs.calls)
}

// A public source being down is an ordinary Tuesday. It must not take the
// other sources with it.
func TestSignalPoll_OneSourceFailingDoesNotStopTheOthers(t *testing.T) {
	repo := &fakeSignalRepo{watches: []*domain.SignalWatch{
		pncpWatch("acc-1", "monitor"),
		{ID: "w2", AccountID: "acc-1", Source: domain.SignalJobPost,
			Keywords: []string{"enfermeiro"}, States: []string{"SP"}, IsActive: true},
	}}
	broken := &fakeFetcher{source: domain.SignalPNCP, err: errors.New("502")}
	working := &fakeFetcher{source: domain.SignalJobPost, signals: []*domain.Signal{{
		Source: domain.SignalJobPost, ExternalID: "job-1",
		Title: "Vaga: enfermeiro UTI", State: "SP", PublishedAt: time.Now(),
	}}}

	NewSignalPollWorker(repo, broken, working).RunOnce(context.Background())

	assert.Len(t, repo.stored, 1)
	assert.Equal(t, "job-1", repo.stored[0].ExternalID)
}

// A watch that matches nothing stores nothing — the filter is the point.
func TestSignalPoll_NonMatchingWatchStoresNothing(t *testing.T) {
	repo := &fakeSignalRepo{watches: []*domain.SignalWatch{pncpWatch("acc-1", "tomógrafo")}}
	fetcher := &fakeFetcher{source: domain.SignalPNCP, signals: []*domain.Signal{monitorTender()}}

	NewSignalPollWorker(repo, fetcher).RunOnce(context.Background())

	assert.Empty(t, repo.stored)
}

// A failed write for one account must not skip the next.
func TestSignalPoll_AStoreFailureDoesNotAbortTheRun(t *testing.T) {
	repo := &fakeSignalRepo{
		watches:  []*domain.SignalWatch{pncpWatch("acc-1", "monitor"), pncpWatch("acc-2", "monitor")},
		storeErr: errors.New("db down"),
	}
	fetcher := &fakeFetcher{source: domain.SignalPNCP, signals: []*domain.Signal{monitorTender()}}

	NewSignalPollWorker(repo, fetcher).RunOnce(context.Background())

	assert.Equal(t, 2, repo.upsertHit, "both accounts attempted")
}

// A watch read that fails means we do not know who wants what. Fetching
// anyway and storing for nobody would just spend the request.
func TestSignalPoll_WatchReadFailureSkipsTheRun(t *testing.T) {
	repo := &fakeSignalRepo{watchErr: errors.New("db down")}
	fetcher := &fakeFetcher{source: domain.SignalPNCP, signals: []*domain.Signal{monitorTender()}}

	NewSignalPollWorker(repo, fetcher).RunOnce(context.Background())

	assert.Zero(t, fetcher.calls)
}

// The lookback is wider than the interval on purpose: a failed run, a deploy
// or a clock skew must not open a hole in the feed.
func TestSignalPoll_LooksFurtherBackThanItsInterval(t *testing.T) {
	repo := &fakeSignalRepo{watches: []*domain.SignalWatch{pncpWatch("acc-1")}}
	var askedFor time.Time
	fetcher := &recordingFetcher{onFetch: func(since time.Time) { askedFor = since }}

	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	w := NewSignalPollWorker(repo, fetcher).WithClock(func() time.Time { return now })
	w.RunOnce(context.Background())

	assert.True(t, askedFor.Before(now.Add(-w.interval)),
		"lookback %v must exceed the interval %v", now.Sub(askedFor), w.interval)
}

type recordingFetcher struct{ onFetch func(time.Time) }

func (r *recordingFetcher) Source() domain.SignalSource { return domain.SignalPNCP }

func (r *recordingFetcher) Fetch(_ context.Context, since time.Time, _ []string) ([]*domain.Signal, error) {
	r.onFetch(since)
	return nil, nil
}

// ---- backfill rule: 30 days for a watch that never produced a signal ----

func fixedClock() (func() time.Time, time.Time) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return now }, now
}

// A fresh watch must see the tenders still OPEN, not only the ones published
// since yesterday: a hospital that published last week and closes next week
// is live work today.
func TestSignalPoll_FreshWatchBackfillsThirtyDays(t *testing.T) {
	repo := &fakeSignalRepo{watches: []*domain.SignalWatch{pncpWatch("acc-1", "monitor")}}
	fetcher := &fakeFetcher{source: domain.SignalPNCP}
	clock, now := fixedClock()

	NewSignalPollWorker(repo, fetcher).WithClock(clock).RunOnce(context.Background())

	require.Len(t, fetcher.sinces, 1)
	assert.Equal(t, now.Add(-30*24*time.Hour), fetcher.sinces[0])
}

// Once an account has stored anything, the steady window is enough — the
// upsert dedupes, but a 30-day read every six hours is thirty times the
// traffic for nothing.
func TestSignalPoll_WarmWatchKeepsTheSteadyLookback(t *testing.T) {
	repo := &fakeSignalRepo{
		watches: []*domain.SignalWatch{pncpWatch("acc-1", "monitor")},
		counts:  map[string]int{"acc-1": 7},
	}
	fetcher := &fakeFetcher{source: domain.SignalPNCP}
	clock, now := fixedClock()

	NewSignalPollWorker(repo, fetcher).WithClock(clock).RunOnce(context.Background())

	require.Len(t, fetcher.sinces, 1)
	assert.Equal(t, now.Add(-48*time.Hour), fetcher.sinces[0])
}

// Fresh and warm watches on the same source: one fetch per WINDOW (two),
// never one per account, and each group is matched only against its own
// result.
func TestSignalPoll_FreshAndWarmWatchesFetchSeparately(t *testing.T) {
	repo := &fakeSignalRepo{
		watches: []*domain.SignalWatch{pncpWatch("warm", "monitor"), pncpWatch("fresh", "monitor"), pncpWatch("fresh-2", "monitor")},
		counts:  map[string]int{"warm": 3},
	}
	fetcher := &fakeFetcher{source: domain.SignalPNCP, signals: []*domain.Signal{monitorTender()}}
	clock, now := fixedClock()

	NewSignalPollWorker(repo, fetcher).WithClock(clock).RunOnce(context.Background())

	require.Equal(t, 2, fetcher.calls, "one fetch per window, not per account")
	assert.ElementsMatch(t,
		[]time.Time{now.Add(-48 * time.Hour), now.Add(-30 * 24 * time.Hour)}, fetcher.sinces)
	assert.Len(t, repo.stored, 3, "every account still gets its row")
}

// A count that fails must not turn every tick into a 30-day read.
func TestSignalPoll_CountFailureFallsBackToTheSteadyLookback(t *testing.T) {
	repo := &fakeSignalRepo{
		watches:  []*domain.SignalWatch{pncpWatch("acc-1", "monitor")},
		countErr: errors.New("db down"),
	}
	fetcher := &fakeFetcher{source: domain.SignalPNCP}
	clock, now := fixedClock()

	NewSignalPollWorker(repo, fetcher).WithClock(clock).RunOnce(context.Background())

	require.Len(t, fetcher.sinces, 1)
	assert.Equal(t, now.Add(-48*time.Hour), fetcher.sinces[0])
}

// ---- scope: the fetch carries the watches' states ----

func stateWatch(accountID string, states ...string) *domain.SignalWatch {
	w := pncpWatch(accountID, "monitor")
	w.States = states
	return w
}

// PNCP pages nationally with no ordering guarantee, so the fetch must ask
// for the states the watches want — the union, normalised and deduped.
func TestSignalPoll_FetchesTheWatchesStates(t *testing.T) {
	repo := &fakeSignalRepo{watches: []*domain.SignalWatch{
		stateWatch("acc-1", "mt"), stateWatch("acc-2", "SP", "MT "),
	}}
	fetcher := &fakeFetcher{source: domain.SignalPNCP}

	NewSignalPollWorker(repo, fetcher).RunOnce(context.Background())

	require.Len(t, fetcher.ufs, 1)
	assert.Equal(t, []string{"MT", "SP"}, fetcher.ufs[0])
}

// One unscoped watch used to speak for the whole group: it made the fetch
// national on every other account's behalf. That is what a single dogfood row
// for the word "luva" did to production — 3 modalities x up to 40 pages of
// the national feed per tick, and the 429s that emptied the radar.
//
// Such a row can no longer be saved, and any that survives is now ignored:
// the scope is the states of the watches that named one.
func TestSignalPoll_AnUnscopedWatchDoesNotWidenTheOthers(t *testing.T) {
	unscoped := pncpWatch("acc-1", "luva")
	unscoped.States = nil
	repo := &fakeSignalRepo{watches: []*domain.SignalWatch{
		unscoped, stateWatch("acc-2", "MT"),
	}}
	fetcher := &fakeFetcher{source: domain.SignalPNCP}

	NewSignalPollWorker(repo, fetcher).RunOnce(context.Background())

	require.Len(t, fetcher.ufs, 1)
	assert.Equal(t, []string{"MT"}, fetcher.ufs[0],
		"the unscoped watch must not drag the fetch across the country")
}

// And when NOTHING in the window names a state, the walk is skipped outright.
// Falling back to the national feed would spend the most expensive request
// this worker can make on watches that can never match anything.
func TestSignalPoll_NoScopedWatchSkipsTheFetch(t *testing.T) {
	unscoped := pncpWatch("acc-1", "luva")
	unscoped.States = nil
	repo := &fakeSignalRepo{watches: []*domain.SignalWatch{unscoped}}
	fetcher := &fakeFetcher{source: domain.SignalPNCP}

	NewSignalPollWorker(repo, fetcher).RunOnce(context.Background())

	assert.Zero(t, fetcher.calls, "no state, no request")
}

// A watch with no keywords must NOT match everything: with the auto-offer
// behind it, "every tender in the state" is a draft budget burnt on office
// supplies.
func TestSignalPoll_EmptyKeywordWatchStoresNothing(t *testing.T) {
	repo := &fakeSignalRepo{watches: []*domain.SignalWatch{pncpWatch("acc-1")}}
	fetcher := &fakeFetcher{source: domain.SignalPNCP, signals: []*domain.Signal{monitorTender()}}

	NewSignalPollWorker(repo, fetcher).RunOnce(context.Background())

	assert.Equal(t, 1, fetcher.calls, "the source is still read for the other watches")
	assert.Empty(t, repo.stored)
}

// ---- auto-offer hook ----

type fakeOfferer struct {
	offered []string // signal ids
	err     error
}

func (f *fakeOfferer) OfferFromSignal(_ context.Context, s *domain.Signal) error {
	if f.err != nil {
		return f.err
	}
	f.offered = append(f.offered, s.AccountID+":"+s.ExternalID)
	return nil
}

func TestSignalPoll_FreshSignalBecomesAnOffer(t *testing.T) {
	repo := &fakeSignalRepo{watches: []*domain.SignalWatch{
		pncpWatch("acc1", "monitor"),
	}}
	fetcher := &fakeFetcher{source: domain.SignalPNCP,
		signals: []*domain.Signal{monitorTender()}}
	offerer := &fakeOfferer{}
	w := NewSignalPollWorker(repo, fetcher)
	w.SetOfferer(offerer)

	w.RunOnce(context.Background())

	assert.Equal(t, []string{"acc1:pncp-1"}, offerer.offered)
}

func TestSignalPoll_WorkedSignalIsNotReOffered(t *testing.T) {
	// The Upsert reads back the committed row; a tender already linked to a
	// lead (a republished amendment) must not mint a second lead.
	repo := &fakeSignalRepo{watches: []*domain.SignalWatch{
		pncpWatch("acc1", "monitor"),
	}}
	repo.onUpsert = func(s *domain.Signal) { s.LeadID = "existing-lead" }
	fetcher := &fakeFetcher{source: domain.SignalPNCP,
		signals: []*domain.Signal{monitorTender()}}
	offerer := &fakeOfferer{}
	w := NewSignalPollWorker(repo, fetcher)
	w.SetOfferer(offerer)

	w.RunOnce(context.Background())

	assert.Empty(t, offerer.offered)
}

func TestSignalPoll_OfferBurstIsBounded(t *testing.T) {
	repo := &fakeSignalRepo{watches: []*domain.SignalWatch{
		pncpWatch("acc1", "monitor"),
	}}
	var many []*domain.Signal
	for i := 0; i < maxAutoOffersPerAccountPerRun+5; i++ {
		s := monitorTender()
		s.ExternalID = fmt.Sprintf("pncp-%d", i)
		many = append(many, s)
	}
	fetcher := &fakeFetcher{source: domain.SignalPNCP, signals: many}
	offerer := &fakeOfferer{}
	w := NewSignalPollWorker(repo, fetcher)
	w.SetOfferer(offerer)

	w.RunOnce(context.Background())

	assert.Len(t, offerer.offered, maxAutoOffersPerAccountPerRun,
		"a fresh watch against the lookback must not burn the draft budget in one tick")
	assert.Equal(t, maxAutoOffersPerAccountPerRun+5, repo.upsertHit,
		"every signal is still STORED — only the auto-draft is capped")
}

type fakeWatchSeeder struct{ calls int }

func (f *fakeWatchSeeder) EnsureDefaultWatches(context.Context) { f.calls++ }

func TestSignalPoll_SeederRunsBeforeTheWatchRead(t *testing.T) {
	repo := &fakeSignalRepo{}
	w := NewSignalPollWorker(repo, &fakeFetcher{source: domain.SignalPNCP})
	seeder := &fakeWatchSeeder{}
	w.SetWatchSeeder(seeder)

	w.RunOnce(context.Background())

	assert.Equal(t, 1, seeder.calls)
}

// ---- source health ----

type fakeSourceHealth struct {
	attempts []healthAttempt
	err      error
}

type healthAttempt struct {
	source domain.SignalSource
	failed bool
}

func (f *fakeSourceHealth) RecordAttempt(
	_ context.Context, source domain.SignalSource, _ time.Time, cause error,
) error {
	f.attempts = append(f.attempts, healthAttempt{source: source, failed: cause != nil})
	return f.err
}

func (f *fakeSourceHealth) ListHealth(context.Context) ([]*domain.SignalSourceHealth, error) {
	return nil, nil
}

// The screen could not tell an outage from a quiet week, because nothing
// recorded the difference. Now the pass does.
func TestSignalPoll_RecordsThatTheSourceAnswered(t *testing.T) {
	repo := &fakeSignalRepo{watches: []*domain.SignalWatch{pncpWatch("acc-1", "monitor")}}
	fetcher := &fakeFetcher{source: domain.SignalPNCP, signals: []*domain.Signal{monitorTender()}}
	health := &fakeSourceHealth{}
	w := NewSignalPollWorker(repo, fetcher)
	w.SetSourceHealth(health)

	w.RunOnce(context.Background())

	require.Len(t, health.attempts, 1)
	assert.Equal(t, domain.SignalPNCP, health.attempts[0].source)
	assert.False(t, health.attempts[0].failed)
}

func TestSignalPoll_RecordsThatTheSourceRefused(t *testing.T) {
	repo := &fakeSignalRepo{watches: []*domain.SignalWatch{pncpWatch("acc-1", "monitor")}}
	fetcher := &fakeFetcher{source: domain.SignalPNCP, err: errors.New("pncp: source unreachable")}
	health := &fakeSourceHealth{}
	w := NewSignalPollWorker(repo, fetcher)
	w.SetSourceHealth(health)

	w.RunOnce(context.Background())

	require.Len(t, health.attempts, 1)
	assert.True(t, health.attempts[0].failed)
}

// A source nobody watches is never asked, so there is nothing to report. A
// stamp here would age into "down" for a source that was simply not needed.
func TestSignalPoll_UnwatchedSourceIsNotStamped(t *testing.T) {
	repo := &fakeSignalRepo{watches: []*domain.SignalWatch{pncpWatch("acc-1", "monitor")}}
	other := &fakeFetcher{source: domain.SignalJobPost}
	health := &fakeSourceHealth{}
	w := NewSignalPollWorker(repo, other)
	w.SetSourceHealth(health)

	w.RunOnce(context.Background())

	assert.Empty(t, health.attempts)
}

// A health table that cannot be written must not stop signals being
// collected: the stamp is a report about the work, not the work.
func TestSignalPoll_StampFailureDoesNotLoseTheSignals(t *testing.T) {
	repo := &fakeSignalRepo{watches: []*domain.SignalWatch{pncpWatch("acc-1", "monitor")}}
	fetcher := &fakeFetcher{source: domain.SignalPNCP, signals: []*domain.Signal{monitorTender()}}
	w := NewSignalPollWorker(repo, fetcher)
	w.SetSourceHealth(&fakeSourceHealth{err: errors.New("table gone")})

	w.RunOnce(context.Background())

	assert.Len(t, repo.stored, 1)
}

// A pass that read PART of the source is not a pass that read the source.
//
// PNCP walks three modalities per state and any of them can refuse. On
// 2026-09-17 modalities 6 and 4 for MT returned nothing at all, modality 8
// kept 50, and the poll stamped a clean success — so the radar showed one
// tender and the screen had no way to know that two thirds of the source had
// gone unread. The rows are still kept; the pass is recorded as incomplete.
func TestSignalPoll_APartialReadIsNotASuccess(t *testing.T) {
	repo := &fakeSignalRepo{watches: []*domain.SignalWatch{pncpWatch("acc-1", "monitor")}}
	fetcher := &fakeFetcher{
		source:  domain.SignalPNCP,
		signals: []*domain.Signal{monitorTender()},
		err: fmt.Errorf("pncp: 2 of 3 walks incomplete: %w",
			domain.ErrSignalSourcePartial),
	}
	health := &fakeSourceHealth{}
	w := NewSignalPollWorker(repo, fetcher)
	w.SetSourceHealth(health)

	w.RunOnce(context.Background())

	require.Len(t, health.attempts, 1)
	assert.True(t, health.attempts[0].failed,
		"an incomplete read must not move last_success_at")
	assert.Len(t, repo.stored, 1,
		"the tenders that DID come back are real and must be kept")
}

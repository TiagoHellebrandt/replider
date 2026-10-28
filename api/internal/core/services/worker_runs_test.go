package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

// worker_runs: the boot-pass policy for the slow tickers, and the signal
// poller's "no offers on a quick restart" guard.

type fakeRunLog struct {
	mu      sync.Mutex
	last    map[string]time.Time
	lastErr error
	stamps  []string
}

func newFakeRunLog() *fakeRunLog { return &fakeRunLog{last: map[string]time.Time{}} }

func (f *fakeRunLog) LastRun(_ context.Context, name string) (*time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lastErr != nil {
		return nil, f.lastErr
	}
	at, ok := f.last[name]
	if !ok {
		return nil, nil
	}
	return &at, nil
}

func (f *fakeRunLog) Stamp(_ context.Context, name string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.last[name] = at
	f.stamps = append(f.stamps, name)
	return nil
}

var _ domain.WorkerRunRepository = (*fakeRunLog)(nil)

func TestWorkerRuns_BootPassIsDueWhenNeverRunOrStale(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	log := newFakeRunLog()

	assert.True(t, bootPassDue(context.Background(), log, "w", 24*time.Hour, now), "never ran → run now")

	log.last["w"] = now.Add(-25 * time.Hour)
	assert.True(t, bootPassDue(context.Background(), log, "w", 24*time.Hour, now), "older than the interval → run now")

	log.last["w"] = now.Add(-2 * time.Hour)
	assert.False(t, bootPassDue(context.Background(), log, "w", 24*time.Hour, now), "fresh → wait for the ticker")
}

func TestWorkerRuns_UnwiredOrBrokenLogNeverRunsOnBoot(t *testing.T) {
	now := time.Now()
	assert.False(t, bootPassDue(context.Background(), nil, "w", time.Hour, now),
		"no log = the historical behaviour (no boot pass)")

	log := newFakeRunLog()
	log.lastErr = errors.New("db down")
	assert.False(t, bootPassDue(context.Background(), log, "w", time.Hour, now),
		"a lookup failure must not guess 'run now' for a worker that drafts messages")
}

func TestWorkerRuns_StampIsNilSafe(t *testing.T) {
	stampRun(context.Background(), nil, "w", time.Now())
	log := newFakeRunLog()
	stampRun(context.Background(), log, "w", time.Now())
	assert.Equal(t, []string{"w"}, log.stamps)
}

func TestSignalPoll_RunOnceStampsTheRunLog(t *testing.T) {
	repo := &fakeSignalRepo{watches: []*domain.SignalWatch{pncpWatch("acc1", "monitor")}}
	fetcher := &fakeFetcher{source: domain.SignalPNCP, signals: []*domain.Signal{monitorTender()}}
	log := newFakeRunLog()
	w := NewSignalPollWorker(repo, fetcher).WithRunLog(log)

	w.RunOnce(context.Background())

	assert.Equal(t, []string{signalPollWorkerName}, log.stamps)
}

func TestSignalPoll_BootPassSkipsOffersAfterARecentRun(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	repo := &fakeSignalRepo{watches: []*domain.SignalWatch{pncpWatch("acc1", "monitor")}}
	fetcher := &fakeFetcher{source: domain.SignalPNCP, signals: []*domain.Signal{monitorTender()}}
	offerer := &fakeOfferer{}
	log := newFakeRunLog()
	log.last[signalPollWorkerName] = now.Add(-10 * time.Minute) // a deploy ten minutes ago
	w := NewSignalPollWorker(repo, fetcher).WithRunLog(log).WithClock(func() time.Time { return now })
	w.SetOfferer(offerer)

	w.run(context.Background(), w.bootOffersAllowed(context.Background()))

	assert.Equal(t, 1, repo.upsertHit, "the boot pass still FETCHES and stores")
	assert.Empty(t, offerer.offered, "but drafts nothing: the last pass was ten minutes ago")
	require.NotNil(t, w.offerer, "the offerer is restored for the ticker passes")
}

func TestSignalPoll_BootPassOffersWhenTheLastRunIsOld(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	repo := &fakeSignalRepo{watches: []*domain.SignalWatch{pncpWatch("acc1", "monitor")}}
	fetcher := &fakeFetcher{source: domain.SignalPNCP, signals: []*domain.Signal{monitorTender()}}
	offerer := &fakeOfferer{}
	log := newFakeRunLog()
	log.last[signalPollWorkerName] = now.Add(-7 * time.Hour)
	w := NewSignalPollWorker(repo, fetcher).WithRunLog(log).WithClock(func() time.Time { return now })
	w.SetOfferer(offerer)

	w.run(context.Background(), w.bootOffersAllowed(context.Background()))

	assert.Equal(t, []string{"acc1:pncp-1"}, offerer.offered)
}

func TestSignalPoll_UnwiredRunLogKeepsOffersOnBoot(t *testing.T) {
	w := NewSignalPollWorker(&fakeSignalRepo{}, &fakeFetcher{source: domain.SignalPNCP})
	assert.True(t, w.bootOffersAllowed(context.Background()),
		"a deployment that forgot the wiring is loud in the ledger, not silently idle")
}

func TestRelationshipWorker_BootPassRunsOnlyWhenStale(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	log := newFakeRunLog()
	log.last[relationshipWorkerName] = now.Add(-30 * time.Minute)
	assert.False(t, bootPassDue(context.Background(), log, relationshipWorkerName, 24*time.Hour, now))
	log.last[relationshipWorkerName] = now.Add(-36 * time.Hour)
	assert.True(t, bootPassDue(context.Background(), log, relationshipWorkerName, 24*time.Hour, now),
		"a 24h ticker under daily deploys never fires; the stamp is what makes the boot pass honest")
}

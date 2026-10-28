package services

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeOverageLister struct {
	ids []string
	err error
}

func (f *fakeOverageLister) ListAccountIDsWithActiveSubscription(ctx context.Context) ([]string, error) {
	return f.ids, f.err
}

type fakeOverageReporter struct {
	mu     sync.Mutex
	seen   map[string]int
	failOn string
	calls  atomic.Int32
}

func (f *fakeOverageReporter) Execute(ctx context.Context, accountID string) (int, error) {
	f.calls.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.seen == nil {
		f.seen = map[string]int{}
	}
	f.seen[accountID]++
	if accountID == f.failOn {
		return 0, assert.AnError
	}
	return 1, nil
}

// TestOverageReportWorker_ReportsEachAccount verifies the worker sweeps
// every listed account on each tick (US-168).
func TestOverageReportWorker_ReportsEachAccount(t *testing.T) {
	lister := &fakeOverageLister{ids: []string{"acc_1", "acc_2", "acc_3"}}
	reporter := &fakeOverageReporter{}
	w := NewOverageReportWorker(lister, reporter).WithPollInterval(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)

	assert.Eventually(t, func() bool {
		return reporter.calls.Load() >= 3
	}, 2*time.Second, 5*time.Millisecond)

	cancel()
	reporter.mu.Lock()
	defer reporter.mu.Unlock()
	assert.GreaterOrEqual(t, reporter.seen["acc_1"], 1)
	assert.GreaterOrEqual(t, reporter.seen["acc_2"], 1)
	assert.GreaterOrEqual(t, reporter.seen["acc_3"], 1)
}

// TestOverageReportWorker_OneAccountFailureDoesNotStopSweep confirms a
// single account's reporting error doesn't abort the rest of the tick.
func TestOverageReportWorker_OneAccountFailureDoesNotStopSweep(t *testing.T) {
	lister := &fakeOverageLister{ids: []string{"acc_1", "boom", "acc_3"}}
	reporter := &fakeOverageReporter{failOn: "boom"}
	w := NewOverageReportWorker(lister, reporter).WithPollInterval(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)

	assert.Eventually(t, func() bool {
		reporter.mu.Lock()
		defer reporter.mu.Unlock()
		return reporter.seen["acc_3"] >= 1
	}, 2*time.Second, 5*time.Millisecond)

	cancel()
}

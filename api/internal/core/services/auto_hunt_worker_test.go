package services

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type countingHunter struct{ runs atomic.Int32 }

func (c *countingHunter) Run(context.Context) []AutoHuntSummary {
	c.runs.Add(1)
	return []AutoHuntSummary{{AccountID: "acc1", Created: 3, Found: 9}}
}

// A 24h ticker with no boot tick means a process restarted daily never hunts
// at all — and this one restarts on every deploy. Five deploys in an afternoon
// would have left the sweep looking wired and doing nothing, which is exactly
// the failure this feature exists to end.
func TestAutoHuntWorker_HuntsOnBootNotOnlyAfterADay(t *testing.T) {
	hunter := &countingHunter{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go NewAutoHuntWorker(hunter).WithInterval(time.Hour).Start(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if hunter.runs.Load() >= 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the worker waited for the ticker before hunting once")
}

// Unwired is a no-op with a log line, never a panic and never a busy loop.
func TestAutoHuntWorker_UnwiredReturns(t *testing.T) {
	done := make(chan struct{})
	go func() {
		NewAutoHuntWorker(nil).Start(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("an unwired worker did not return")
	}
}

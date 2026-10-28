package services

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeRescanner struct {
	calls   atomic.Int32
	settled int
	err     error
}

func (f *fakeRescanner) Rescan(context.Context, int32) (int, error) {
	f.calls.Add(1)
	return f.settled, f.err
}

// A deploy restarts the API and clamd together, and the daemon takes minutes
// to load its signature database. Sweeping immediately would re-record every
// pending file as 'failed' against a scanner that is merely still booting.
func TestRescanWorker_DoesNotSweepOnStart(t *testing.T) {
	f := &fakeRescanner{}
	w := NewAttachmentRescanWorker(f).WithPollInterval(time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)
	time.Sleep(60 * time.Millisecond)
	cancel()

	assert.Zero(t, f.calls.Load(), "the first sweep must wait for one full interval")
}

func TestRescanWorker_SweepsOnTick(t *testing.T) {
	f := &fakeRescanner{settled: 3}
	w := NewAttachmentRescanWorker(f).WithPollInterval(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)
	time.Sleep(120 * time.Millisecond)
	cancel()

	assert.GreaterOrEqual(t, f.calls.Load(), int32(2))
}

// A failing sweep must not kill the loop: clamd being down is exactly the
// condition this worker exists to recover from.
func TestRescanWorker_SurvivesAFailingSweep(t *testing.T) {
	f := &fakeRescanner{err: errors.New("clamd down")}
	w := NewAttachmentRescanWorker(f).WithPollInterval(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)
	time.Sleep(120 * time.Millisecond)
	cancel()

	assert.GreaterOrEqual(t, f.calls.Load(), int32(2), "the loop must keep trying")
}

func TestRescanWorker_StartsOncePerProcess(t *testing.T) {
	f := &fakeRescanner{}
	w := NewAttachmentRescanWorker(f).WithPollInterval(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)
	time.Sleep(10 * time.Millisecond)
	w.Start(ctx) // returns immediately, does not start a second loop
	time.Sleep(60 * time.Millisecond)
	cancel()

	assert.LessOrEqual(t, f.calls.Load(), int32(4),
		"a second loop would roughly double the sweep count")
}

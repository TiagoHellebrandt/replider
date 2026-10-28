package services

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeWeeklyDigestSender struct {
	calls atomic.Int32
}

func (f *fakeWeeklyDigestSender) Execute(ctx context.Context) (int, error) {
	f.calls.Add(1)
	return 0, nil
}

// monday/tuesday are fixed reference dates with the expected weekday.
var (
	aMonday  = time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC) // 2026-06-01 is a Monday
	aTuesday = time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC)
)

// TestWeeklyDigestWorker_FiresOnSendDay verifies the worker runs the send
// on its configured day (US-176 "Monday email").
func TestWeeklyDigestWorker_FiresOnSendDay(t *testing.T) {
	sender := &fakeWeeklyDigestSender{}
	w := NewWeeklyDigestWorker(sender).
		WithPollInterval(20 * time.Millisecond).
		WithClock(func() time.Time { return aMonday })

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)

	assert.Eventually(t, func() bool {
		return sender.calls.Load() >= 2 // immediate tick + at least one periodic
	}, 2*time.Second, 5*time.Millisecond)

	cancel()
}

// TestWeeklyDigestWorker_SkipsOffDay verifies the worker does NOT send on
// any day other than its send day.
func TestWeeklyDigestWorker_SkipsOffDay(t *testing.T) {
	sender := &fakeWeeklyDigestSender{}
	w := NewWeeklyDigestWorker(sender).
		WithPollInterval(20 * time.Millisecond).
		WithClock(func() time.Time { return aTuesday })

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)

	// Give it several tick windows; it must never fire on the off day.
	time.Sleep(150 * time.Millisecond)
	cancel()

	assert.Equal(t, int32(0), sender.calls.Load(), "worker should not send off its send day")
}

func TestWeeklyDigestWorker_SanityWeekdays(t *testing.T) {
	assert.Equal(t, time.Monday, aMonday.Weekday())
	assert.Equal(t, time.Tuesday, aTuesday.Weekday())
}

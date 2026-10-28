package services

import (
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_TripsAfterFiveFailuresInWindow(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	cb := NewCircuitBreaker().withClock(func() time.Time { return now })

	boom := errors.New("provider down")
	// 4 failures: still closed.
	for i := 0; i < 4; i++ {
		cb.Record(boom)
	}
	if cb.State() != CircuitClosed {
		t.Fatalf("after 4 failures want closed, got %s", cb.State())
	}
	// 5th failure within the 30s window trips it.
	cb.Record(boom)
	if cb.State() != CircuitOpen {
		t.Fatalf("after 5 failures want open, got %s", cb.State())
	}
	if err := cb.Allow(); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("open breaker must reject Allow, got %v", err)
	}
}

func TestCircuitBreaker_FailuresOutsideWindowDoNotTrip(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	cb := NewCircuitBreaker().withClock(func() time.Time { return now })
	boom := errors.New("blip")

	for i := 0; i < 4; i++ {
		cb.Record(boom)
	}
	// Jump past the 30s window, then more failures: counter restarts.
	now = now.Add(31 * time.Second)
	for i := 0; i < 4; i++ {
		cb.Record(boom)
	}
	if cb.State() != CircuitClosed {
		t.Fatalf("scattered failures must not trip, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenThenCloseOnSuccess(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	cb := NewCircuitBreaker().withClock(func() time.Time { return now })
	boom := errors.New("down")

	for i := 0; i < 5; i++ {
		cb.Record(boom)
	}
	if cb.State() != CircuitOpen {
		t.Fatalf("want open, got %s", cb.State())
	}
	// Before the 60s timeout it stays open.
	now = now.Add(59 * time.Second)
	if err := cb.Allow(); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("still within timeout, want open, got %v", err)
	}
	// After the timeout it half-opens and admits a probe.
	now = now.Add(2 * time.Second)
	if cb.State() != CircuitHalfOpen {
		t.Fatalf("want half-open after timeout, got %s", cb.State())
	}
	if err := cb.Allow(); err != nil {
		t.Fatalf("half-open must admit a probe, got %v", err)
	}
	// A successful probe closes it.
	cb.Record(nil)
	if cb.State() != CircuitClosed {
		t.Fatalf("success probe must close, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenReopensOnFailure(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	cb := NewCircuitBreaker().withClock(func() time.Time { return now })
	boom := errors.New("down")

	for i := 0; i < 5; i++ {
		cb.Record(boom)
	}
	now = now.Add(61 * time.Second)
	if cb.State() != CircuitHalfOpen {
		t.Fatalf("want half-open, got %s", cb.State())
	}
	// Probe fails → straight back to open for another full timeout.
	cb.Record(boom)
	if cb.State() != CircuitOpen {
		t.Fatalf("failed probe must reopen, got %s", cb.State())
	}
}

func TestCircuitBreaker_ExecuteRejectsWhenOpen(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	cb := NewCircuitBreaker().withClock(func() time.Time { return now })
	boom := errors.New("down")
	for i := 0; i < 5; i++ {
		cb.Record(boom)
	}

	ran := false
	err := cb.Execute(func() error { ran = true; return nil })
	if ran {
		t.Fatal("Execute must not call fn while open")
	}
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("want ErrCircuitOpen, got %v", err)
	}
}

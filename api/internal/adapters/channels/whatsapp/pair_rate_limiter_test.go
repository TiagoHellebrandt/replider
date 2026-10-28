package whatsapp

import (
	"testing"
	"time"
)

func TestPairRateLimiter_BurstThenStarve(t *testing.T) {
	l := NewPairRateLimiter(3, 1*time.Second)

	// First 3 allowed (burst capacity).
	for i := 0; i < 3; i++ {
		if !l.Allow("ch-1", "5511999") {
			t.Fatalf("attempt %d should be allowed within burst", i)
		}
	}
	// 4th immediately should be denied.
	if l.Allow("ch-1", "5511999") {
		t.Error("expected 4th send to be rate-limited")
	}
}

func TestPairRateLimiter_RefillRecovers(t *testing.T) {
	l := NewPairRateLimiter(2, 50*time.Millisecond)

	if !l.Allow("ch-1", "p") {
		t.Fatal("first allow")
	}
	if !l.Allow("ch-1", "p") {
		t.Fatal("second allow (burst)")
	}
	if l.Allow("ch-1", "p") {
		t.Fatal("third should be denied")
	}
	time.Sleep(120 * time.Millisecond) // > 2 refills
	if !l.Allow("ch-1", "p") {
		t.Error("expected refill to restore at least one token")
	}
}

func TestPairRateLimiter_IsolatesPairs(t *testing.T) {
	l := NewPairRateLimiter(1, 1*time.Second)

	if !l.Allow("ch-1", "alice") {
		t.Fatal("alice first allow")
	}
	if l.Allow("ch-1", "alice") {
		t.Fatal("alice second should be denied")
	}
	// Different recipient is unaffected.
	if !l.Allow("ch-1", "bob") {
		t.Error("bob should NOT inherit alice's denial")
	}
	// Different channel is unaffected.
	if !l.Allow("ch-2", "alice") {
		t.Error("alice on ch-2 should NOT inherit ch-1 denial")
	}
}

func TestPairRateLimiter_NilSafe(t *testing.T) {
	var l *PairRateLimiter
	if !l.Allow("ch", "r") {
		t.Error("nil limiter should permit all sends (no-op)")
	}
}

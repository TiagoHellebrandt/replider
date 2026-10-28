package whatsapp

import (
	"sync"
	"time"
)

// PairRateLimiter caps the rate at which a single (channel, recipient)
// pair can receive outbound messages. Meta returns code 131056 ("pair
// rate limit") when this is exceeded; the cooldown is silent on the
// recipient side and burns the customer's quota window.
//
// We use a per-pair token bucket — each pair gets a small burst budget
// (default 6 messages) refilled at a steady rate (default 1/sec). The
// limiter is in-memory and not shared across replicas: campaign
// pacing must NOT rely on this alone for global throttling, just as a
// per-instance safety net so a single replica with a runaway loop
// doesn't bomb one customer.
type PairRateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*pairBucket
	capacity int
	refill   time.Duration
	idleTTL  time.Duration
}

type pairBucket struct {
	tokens     float64
	lastRefill time.Time
	lastSeen   time.Time
}

// NewPairRateLimiter constructs a limiter with the given capacity
// (max messages in a burst) and refill interval (time per token).
// Defaults that match WhatsApp's documented per-pair limits without
// pushing too close: capacity=6, refill=1s.
func NewPairRateLimiter(capacity int, refill time.Duration) *PairRateLimiter {
	if capacity <= 0 {
		capacity = 6
	}
	if refill <= 0 {
		refill = 1 * time.Second
	}
	return &PairRateLimiter{
		buckets:  make(map[string]*pairBucket),
		capacity: capacity,
		refill:   refill,
		idleTTL:  10 * time.Minute,
	}
}

// Allow reports whether the (channelID, recipient) pair has a token
// available. Returns false when the bucket is empty; the caller
// should defer the send (queue, sleep, or return a typed
// rate-limited error to the user).
func (l *PairRateLimiter) Allow(channelID, recipient string) bool {
	if l == nil {
		return true
	}
	key := channelID + "|" + recipient
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, ok := l.buckets[key]
	if !ok {
		// First send — start with a full bucket minus the one we're
		// about to consume. New customers get the full burst budget.
		l.buckets[key] = &pairBucket{
			tokens:     float64(l.capacity - 1),
			lastRefill: now,
			lastSeen:   now,
		}
		l.maybeGC(now)
		return true
	}

	// Refill since last touch.
	elapsed := now.Sub(b.lastRefill)
	added := elapsed.Seconds() / l.refill.Seconds()
	if added > 0 {
		b.tokens += added
		if b.tokens > float64(l.capacity) {
			b.tokens = float64(l.capacity)
		}
		b.lastRefill = now
	}
	b.lastSeen = now

	if b.tokens >= 1 {
		b.tokens -= 1
		return true
	}
	return false
}

// maybeGC drops buckets that haven't been touched for idleTTL so the
// map doesn't grow unbounded across millions of one-shot recipients.
// Called opportunistically inside Allow under the lock — no separate
// goroutine.
func (l *PairRateLimiter) maybeGC(now time.Time) {
	// Cheap probabilistic GC: only sweep when the map is past 1024
	// entries to amortize across calls.
	if len(l.buckets) < 1024 {
		return
	}
	cutoff := now.Add(-l.idleTTL)
	for k, b := range l.buckets {
		if b.lastSeen.Before(cutoff) {
			delete(l.buckets, k)
		}
	}
}

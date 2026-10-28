package services

import (
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen is returned by Allow/Execute when the breaker is open.
var ErrCircuitOpen = errors.New("circuit breaker open")

// CircuitBreakerState is the breaker's current mode.
type CircuitBreakerState int

const (
	// CircuitClosed: calls flow through normally.
	CircuitClosed CircuitBreakerState = iota
	// CircuitOpen: calls are rejected fast (the dependency looks down).
	CircuitOpen
	// CircuitHalfOpen: a single probe is allowed to test recovery.
	CircuitHalfOpen
)

func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "closed"
	}
}

// CircuitBreaker guards a flaky dependency (the LLM provider, US-026). It
// trips OPEN after maxFailures errors within failureWindow, rejecting calls
// fast for openTimeout, then allows a single HALF-OPEN probe: success closes
// it, another failure re-opens it. This stops a provider outage from turning
// every inbound message into a slow, doomed upstream call.
type CircuitBreaker struct {
	maxFailures   int
	failureWindow time.Duration
	openTimeout   time.Duration

	now func() time.Time // injectable clock for tests

	mu          sync.Mutex
	state       CircuitBreakerState
	failures    int
	windowStart time.Time
	openedAt    time.Time
}

// NewCircuitBreaker builds a breaker with the US-026 defaults: trip after 5
// failures in 30s, stay open 60s before probing.
func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:   5,
		failureWindow: 30 * time.Second,
		openTimeout:   60 * time.Second,
		now:           time.Now,
	}
}

// withClock overrides the clock (tests only).
func (c *CircuitBreaker) withClock(now func() time.Time) *CircuitBreaker {
	c.now = now
	return c
}

// State returns the current state, applying any pending open→half-open
// transition based on the clock.
func (c *CircuitBreaker) State() CircuitBreakerState {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.maybeHalfOpen()
	return c.state
}

// Allow reports whether a call may proceed. It returns ErrCircuitOpen when
// the breaker is open (caller should serve a fallback instead of calling the
// dependency).
func (c *CircuitBreaker) Allow() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.maybeHalfOpen()
	if c.state == CircuitOpen {
		return ErrCircuitOpen
	}
	return nil
}

// Record feeds a call's outcome back into the breaker.
func (c *CircuitBreaker) Record(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err == nil {
		// A successful probe (or any success) closes and resets the breaker.
		c.state = CircuitClosed
		c.failures = 0
		return
	}

	now := c.now()
	switch c.state {
	case CircuitHalfOpen:
		// Probe failed — back to open for another timeout.
		c.trip(now)
	default:
		// Closed: count failures within the rolling window.
		if c.failures == 0 || now.Sub(c.windowStart) > c.failureWindow {
			c.windowStart = now
			c.failures = 0
		}
		c.failures++
		if c.failures >= c.maxFailures {
			c.trip(now)
		}
	}
}

// Execute runs fn behind the breaker: rejects fast when open, otherwise runs
// fn and records the outcome.
func (c *CircuitBreaker) Execute(fn func() error) error {
	if err := c.Allow(); err != nil {
		return err
	}
	err := fn()
	c.Record(err)
	return err
}

// trip moves the breaker to open. Caller holds the lock.
func (c *CircuitBreaker) trip(now time.Time) {
	c.state = CircuitOpen
	c.openedAt = now
	c.failures = 0
}

// maybeHalfOpen promotes open→half-open once the timeout elapses. Caller
// holds the lock.
func (c *CircuitBreaker) maybeHalfOpen() {
	if c.state == CircuitOpen && c.now().Sub(c.openedAt) >= c.openTimeout {
		c.state = CircuitHalfOpen
	}
}

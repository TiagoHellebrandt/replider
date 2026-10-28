package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/replider/api/internal/adapters/httpsafe"
	"github.com/replider/api/internal/core/domain"
	"golang.org/x/time/rate"
)

// Dispatcher tests deliver to httptest servers on 127.0.0.1, which the
// US-F011 SSRF guard blocks under the production policy. Opt into the dev
// policy for this package — the guard's own behaviour is pinned in
// internal/adapters/httpsafe.
func TestMain(m *testing.M) {
	httpsafe.SetAllowPrivateTargets(true)
	os.Exit(m.Run())
}

// inMemoryWebhookRepo is the smallest impl that satisfies the
// AccountWebhookRepository contract for dispatcher tests. Records
// success / failure / disable calls so tests can assert state
// transitions without a real DB.
type inMemoryWebhookRepo struct {
	mu        sync.Mutex
	hooks     []*domain.AccountWebhook
	successes []string
	failures  []string
	disabled  []string
	failMsgs  map[string]string
}

func newInMemoryWebhookRepo(seed ...*domain.AccountWebhook) *inMemoryWebhookRepo {
	return &inMemoryWebhookRepo{hooks: seed, failMsgs: map[string]string{}}
}

func (r *inMemoryWebhookRepo) Create(_ context.Context, w *domain.AccountWebhook) (*domain.AccountWebhook, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hooks = append(r.hooks, w)
	return w, nil
}
func (r *inMemoryWebhookRepo) GetByID(_ context.Context, accountID, id string) (*domain.AccountWebhook, error) {
	for _, w := range r.hooks {
		if w.ID == id && w.AccountID == accountID {
			return w, nil
		}
	}
	return nil, nil
}
func (r *inMemoryWebhookRepo) ListByAccount(_ context.Context, accountID string) ([]*domain.AccountWebhook, error) {
	out := []*domain.AccountWebhook{}
	for _, w := range r.hooks {
		if w.AccountID == accountID {
			out = append(out, w)
		}
	}
	return out, nil
}
func (r *inMemoryWebhookRepo) ListActiveForEvent(_ context.Context, accountID, event string) ([]*domain.AccountWebhook, error) {
	out := []*domain.AccountWebhook{}
	for _, w := range r.hooks {
		if w.AccountID != accountID || !w.IsActive {
			continue
		}
		for _, ev := range w.Events {
			if ev == event {
				out = append(out, w)
				break
			}
		}
	}
	return out, nil
}
func (r *inMemoryWebhookRepo) Update(_ context.Context, w *domain.AccountWebhook) (*domain.AccountWebhook, error) {
	return w, nil
}
func (r *inMemoryWebhookRepo) Delete(_ context.Context, _ string, _ string) error { return nil }
func (r *inMemoryWebhookRepo) RecordSuccess(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.successes = append(r.successes, id)
	return nil
}
func (r *inMemoryWebhookRepo) RecordFailure(_ context.Context, id string, msg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failures = append(r.failures, id)
	r.failMsgs[id] = msg
	return nil
}
func (r *inMemoryWebhookRepo) Disable(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.disabled = append(r.disabled, id)
	return nil
}

// TestDispatcher_SignsAndDeliversOnce asserts that an Emit() call:
//   - reaches the subscriber's URL exactly once
//   - signs the body with HMAC-SHA256(secret) and ships it in
//     X-Replider-Signature
//   - flips the row to success state
func TestDispatcher_SignsAndDeliversOnce(t *testing.T) {
	const secret = "shhh"
	var calls atomic.Int32
	var capturedBody []byte
	var capturedSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		capturedBody, _ = io.ReadAll(r.Body)
		capturedSig = r.Header.Get("X-Replider-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	repo := newInMemoryWebhookRepo(&domain.AccountWebhook{
		ID: "w-1", AccountID: "acc-1", URL: srv.URL, Secret: secret,
		Events: []string{"message.received"}, IsActive: true,
	})
	d := NewOutboundWebhookDispatcher(repo)
	d.Emit("acc-1", "message.received", map[string]any{"hi": "there"})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && calls.Load() == 0 {
		time.Sleep(20 * time.Millisecond)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected exactly 1 delivery, got %d", calls.Load())
	}

	// Signature: prefix + hex(HMAC-SHA256(body, secret)).
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(capturedBody)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if capturedSig != want {
		t.Errorf("signature mismatch:\n  want %s\n  got  %s", want, capturedSig)
	}

	// Wait for the success record to land — RecordSuccess runs after
	// the HTTP response is consumed, so it can race the assertion.
	for i := 0; i < 100 && len(repo.successes) == 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if len(repo.successes) != 1 || repo.successes[0] != "w-1" {
		t.Errorf("expected success record for w-1, got %v", repo.successes)
	}
}

// TestDispatcher_SkipsOnDifferentEvent asserts subscribers only fire
// for events they listed. A message.sent emit must NOT reach a
// webhook that only subscribed to message.received.
func TestDispatcher_SkipsOnDifferentEvent(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
	}))
	defer srv.Close()

	repo := newInMemoryWebhookRepo(&domain.AccountWebhook{
		ID: "w-2", AccountID: "acc-1", URL: srv.URL, Secret: "x",
		Events: []string{"message.received"}, IsActive: true,
	})
	d := NewOutboundWebhookDispatcher(repo)
	d.Emit("acc-1", "message.sent", nil)
	time.Sleep(150 * time.Millisecond)

	if calls.Load() != 0 {
		t.Errorf("dispatcher fired the wrong event; got %d calls", calls.Load())
	}
}

// TestDispatcher_AutoDisablesOnRepeatedFailure asserts that after
// maxFailures consecutive failures, the dispatcher flips is_active=false
// so a permanently-broken endpoint stops burning workers.
func TestDispatcher_AutoDisablesOnRepeatedFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	hook := &domain.AccountWebhook{
		ID: "w-3", AccountID: "acc-1", URL: srv.URL, Secret: "x",
		Events: []string{"message.received"}, IsActive: true,
		// Pre-loaded 9 failures: the next failure tips us over the
		// default threshold of 10.
		FailureCount: 9,
	}
	repo := newInMemoryWebhookRepo(hook)
	d := NewOutboundWebhookDispatcher(repo)
	// Skip real backoff waits — a 500 endpoint now triggers the full
	// retry ladder, and we only care that the threshold breach disables.
	d.sleep = func(context.Context, time.Duration) error { return nil }
	d.Emit("acc-1", "message.received", nil)

	for i := 0; i < 100 && len(repo.disabled) == 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if len(repo.disabled) != 1 || repo.disabled[0] != "w-3" {
		t.Errorf("expected disable for w-3 after threshold breach; got %v", repo.disabled)
	}
}

// TestDispatcher_RateLimitConfig pins the per-subscription throttle
// contract: 1 dispatch/second sustained with a burst queue of 10, one
// shared bucket per webhook ID.
func TestDispatcher_RateLimitConfig(t *testing.T) {
	d := NewOutboundWebhookDispatcher(newInMemoryWebhookRepo())

	lim := d.limiterFor("w-1")
	if lim.Limit() != rate.Limit(1) {
		t.Errorf("dispatch rate: want 1/s, got %v", lim.Limit())
	}
	if lim.Burst() != 10 {
		t.Errorf("dispatch burst: want 10, got %d", lim.Burst())
	}
	// Same subscription must share the bucket, otherwise the rate resets
	// on every Emit and the throttle is meaningless.
	if d.limiterFor("w-1") != lim {
		t.Error("expected the same limiter instance for the same subscription")
	}
	// Different subscriptions are throttled independently.
	if d.limiterFor("w-2") == lim {
		t.Error("expected distinct limiters per subscription")
	}
}

// TestDispatcher_RateLimitThrottles proves the bucket actually paces
// delivery: once the burst is drained, further dispatches block until
// the bucket refills. Uses a tightened bucket (burst 1, 100/s) so the
// throttle is observable in milliseconds instead of seconds.
func TestDispatcher_RateLimitThrottles(t *testing.T) {
	d := NewOutboundWebhookDispatcher(newInMemoryWebhookRepo())
	d.rateBurst = 1
	d.rateLimit = rate.Limit(100) // one token every 10ms

	ctx := context.Background()
	start := time.Now()
	// 1 token in the burst, then 3 refills @ 10ms each ≈ 30ms total.
	for i := 0; i < 4; i++ {
		if err := d.waitRate(ctx, "w-throttle"); err != nil {
			t.Fatalf("waitRate returned error: %v", err)
		}
	}
	elapsed := time.Since(start)
	if elapsed < 25*time.Millisecond {
		t.Errorf("expected throttling to delay ~30ms after burst, got %v", elapsed)
	}
}

// TestDispatcher_RateLimitDropsWhenBudgetExceeded asserts that an event
// is dropped (not delivered, not piling up goroutines) when the bucket
// can't grant a token before the delivery context expires.
func TestDispatcher_RateLimitDropsWhenBudgetExceeded(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	repo := newInMemoryWebhookRepo(&domain.AccountWebhook{
		ID: "w-drop", AccountID: "acc-1", URL: srv.URL, Secret: "x",
		Events: []string{"message.received"}, IsActive: true,
	})
	d := NewOutboundWebhookDispatcher(repo)
	// Empty the bucket up front and make refill far slower than the
	// already-expired context can wait for.
	d.rateBurst = 1
	d.rateLimit = rate.Limit(0.001)              // ~1 token / 1000s
	d.limiterFor("w-drop").AllowN(time.Now(), 1) // drain the single burst token

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if err := d.waitRate(ctx, "w-drop"); err == nil {
		t.Fatal("expected waitRate to fail once the bucket is starved past the deadline")
	}
}

// TestDispatcher_RetrySchedule pins the exponential backoff contract:
// a permanently-failing endpoint is retried following the
// [1s,5s,30s,5m,1h] ladder, capped at 5 total attempts, then
// dead-lettered. The fake sleep records the delays instead of waiting.
func TestDispatcher_RetrySchedule(t *testing.T) {
	// The canonical schedule is the audit-mandated ladder.
	want := []time.Duration{
		1 * time.Second, 5 * time.Second, 30 * time.Second,
		5 * time.Minute, 1 * time.Hour,
	}
	if len(defaultRetrySchedule) != len(want) {
		t.Fatalf("schedule length: want %d, got %d", len(want), len(defaultRetrySchedule))
	}
	for i := range want {
		if defaultRetrySchedule[i] != want[i] {
			t.Fatalf("schedule[%d]: want %v, got %v", i, want[i], defaultRetrySchedule[i])
		}
	}

	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	hook := &domain.AccountWebhook{
		ID: "w-4", AccountID: "acc-1", URL: srv.URL, Secret: "x",
		Events: []string{"message.received"}, IsActive: true,
	}
	repo := newInMemoryWebhookRepo(hook)
	d := NewOutboundWebhookDispatcher(repo)

	var slept []time.Duration
	var smu sync.Mutex
	d.sleep = func(_ context.Context, dur time.Duration) error {
		smu.Lock()
		slept = append(slept, dur)
		smu.Unlock()
		return nil
	}

	done := make(chan struct{})
	d.afterDeliver = func() { close(done) }
	d.Emit("acc-1", "message.received", nil)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("delivery did not complete in time")
	}

	// 5 attempts total → 4 backoff sleeps consumed from the ladder.
	if attempts.Load() != 5 {
		t.Errorf("expected 5 delivery attempts, got %d", attempts.Load())
	}
	smu.Lock()
	gotSlept := append([]time.Duration(nil), slept...)
	smu.Unlock()
	wantSlept := want[:4]
	if len(gotSlept) != len(wantSlept) {
		t.Fatalf("backoff count: want %d, got %d (%v)", len(wantSlept), len(gotSlept), gotSlept)
	}
	for i := range wantSlept {
		if gotSlept[i] != wantSlept[i] {
			t.Errorf("backoff[%d]: want %v, got %v", i, wantSlept[i], gotSlept[i])
		}
	}

	// Dead-letter: the terminal failure is recorded with the marker.
	smu2 := &repo.mu
	smu2.Lock()
	deadLettered := false
	for _, m := range repo.failMsgs {
		if len(m) >= len("dead-letter") && m[:len("dead-letter")] == "dead-letter" {
			deadLettered = true
		}
	}
	smu2.Unlock()
	if !deadLettered {
		t.Error("expected a dead-letter failure to be recorded")
	}
}

// TestDispatcher_EmitToEndpoint_SignsAndDelivers asserts the funnel-webhook
// path (US-016) signs the body with the funnel's secret and POSTs to the funnel
// URL — no account_webhooks repo row involved. It must do so with NO repo
// bookkeeping (the dispatcher's repo is nil here, proving the path never touches
// it).
func TestDispatcher_EmitToEndpoint_SignsAndDelivers(t *testing.T) {
	const secret = "funnel-secret"
	var calls atomic.Int32
	var capturedBody []byte
	var capturedSig, capturedEvent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		capturedBody, _ = io.ReadAll(r.Body)
		capturedSig = r.Header.Get("X-Replider-Signature")
		capturedEvent = r.Header.Get("X-Replider-Event")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// nil repo: the funnel path must never call repo methods.
	d := NewOutboundWebhookDispatcher(nil)
	done := make(chan struct{})
	d.afterDeliver = func() { close(done) }
	d.EmitToEndpoint("funnel:fun_1", "acc_1", srv.URL, secret, EventLeadQualified,
		map[string]any{"lead_id": "lead_1"})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("delivery did not complete")
	}

	if calls.Load() != 1 {
		t.Fatalf("expected exactly 1 delivery, got %d", calls.Load())
	}
	if capturedEvent != EventLeadQualified {
		t.Errorf("event header: want %s, got %s", EventLeadQualified, capturedEvent)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(capturedBody)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if capturedSig != want {
		t.Errorf("signature mismatch:\n  want %s\n  got  %s", want, capturedSig)
	}
}

func TestDispatcher_DeliverFunnelTest_ReturnsStatusAndBody(t *testing.T) {
	const secret = "funnel-test-secret"
	var capturedBody []byte
	var capturedSig, capturedEvent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		capturedSig = r.Header.Get("X-Replider-Signature")
		capturedEvent = r.Header.Get("X-Replider-Event")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"received":true}`))
	}))
	defer srv.Close()

	// nil repo: the synchronous test ping must never touch repo bookkeeping.
	d := NewOutboundWebhookDispatcher(nil)
	status, body, err := d.DeliverFunnelTest(context.Background(), "acc_1", srv.URL, secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusAccepted {
		t.Errorf("status: want %d, got %d", http.StatusAccepted, status)
	}
	if body != `{"received":true}` {
		t.Errorf("body: want %q, got %q", `{"received":true}`, body)
	}
	if capturedEvent != "webhook.test" {
		t.Errorf("event header: want webhook.test, got %s", capturedEvent)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(capturedBody)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if capturedSig != want {
		t.Errorf("signature mismatch:\n  want %s\n  got  %s", want, capturedSig)
	}
}

func TestDispatcher_DeliverFunnelTest_TransportError(t *testing.T) {
	// An unroutable URL: Do() returns a transport error, surfaced to the caller.
	d := NewOutboundWebhookDispatcher(nil)
	_, _, err := d.DeliverFunnelTest(context.Background(), "acc_1", "http://127.0.0.1:0/nope", "s")
	if err == nil {
		t.Fatal("expected a transport error, got nil")
	}
}

package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/replider/api/internal/adapters/httpsafe"
	"github.com/replider/api/internal/core/domain"
	"golang.org/x/time/rate"
)

// outboundWebhookDeadLetterTotal counts per-endpoint (funnel, US-016) webhook
// deliveries dropped after exhausting every retry. Labelled by `event` only —
// a fixed, low-cardinality enum (lead.qualified, lead.buy_intent, …) — never by
// account/funnel id (unbounded + PII). Ops alerts on a non-zero rate to catch a
// customer whose webhook endpoint went down, which otherwise only lived in a
// WARN log. The matching ERROR log carries the account_id/funnel for triage.
var outboundWebhookDeadLetterTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "outbound_webhook_dead_letter_total",
		Help: "Per-endpoint outbound webhook deliveries dead-lettered after exhausting retries, labelled by event.",
	},
	[]string{"event"},
)

// Webhook event names. Stable contract — third parties subscribe to
// these strings, so renaming is a breaking change. Add new events
// freely; never repurpose existing ones.
const (
	EventMessageReceived       = "message.received"
	EventMessageSent           = "message.sent"
	EventConversationCreated   = "conversation.created"
	EventConversationAssigned  = "conversation.assigned"
	EventConversationClosed    = "conversation.closed"
	EventQualityRatingChanged  = "whatsapp.quality_changed"
	EventTemplateStatusChanged = "whatsapp.template_status_changed"
	// EventLeadQualified fires when a sales-funnel lead completes its
	// required-metadata gate (US-016). Delivered to the FUNNEL's webhook
	// (EmitToEndpoint), not an account_webhooks subscription.
	EventLeadQualified = "lead.qualified"
	// EventLeadBuyIntent fires when a sales-funnel lead signals purchase
	// intent (US-016). Also funnel-scoped via EmitToEndpoint.
	EventLeadBuyIntent = "lead.buy_intent"
)

// WebhookHTTPClient is the minimal interface the dispatcher uses to
// reach external endpoints. Tests inject a fake; production wires the
// shared http.Client with sane timeouts.
type WebhookHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// defaultRetrySchedule is the exponential backoff applied between
// delivery attempts. A failed POST is retried after each successive
// delay; once the schedule (capped by maxAttempts) is exhausted the
// event is dead-lettered. The widening gaps keep a flapping consumer
// from being hammered while still giving a slow/restarting endpoint up
// to an hour to recover. Stable contract — tests assert this exact
// sequence.
var defaultRetrySchedule = []time.Duration{
	1 * time.Second,
	5 * time.Second,
	30 * time.Second,
	5 * time.Minute,
	1 * time.Hour,
}

// defaultMaxAttempts caps total delivery tries (initial + retries) so a
// permanently-broken endpoint can't loop forever. With 5 attempts the
// dispatcher consumes the first four backoff delays before
// dead-lettering; the trailing 1h entry is headroom for a future bump.
const defaultMaxAttempts = 5

// Per-subscription dispatch throttle. A tenant can subscribe a webhook
// and then trigger a flood of platform events (e.g. by spamming inbound
// messages), turning Replider into an amplifier that hammers the target
// URL. A token bucket per subscription caps sustained delivery at
// defaultDispatchRate/sec while still absorbing a defaultDispatchBurst
// spike, so legitimate bursts pass but a sustained flood is paced.
const (
	defaultDispatchRate  rate.Limit = 1  // 1 dispatch/second sustained
	defaultDispatchBurst int        = 10 // burst queue of 10
)

// OutboundWebhookDispatcher fans out platform events to tenant-managed
// HTTPS endpoints (Zapier, n8n, Make, custom CRMs). Each delivery is:
//
//  1. Looked up via repo.ListActiveForEvent (account+event index hits
//     the partial index, so a tenant with one webhook per event sees
//     no scan beyond their own row).
//  2. Signed with HMAC-SHA256 using the per-webhook secret so the
//     consumer can verify authenticity without a shared global key.
//  3. POSTed with a 10s timeout. Non-2xx and timeouts bump
//     failure_count; 10 consecutive failures auto-disable the row.
//  4. Fire-and-forget: the caller's request goroutine never blocks on
//     a slow consumer. Detached context lives 30s so the original
//     request completing doesn't cut delivery short.
//
// Two design choices to call out:
//   - No retry queue (yet). For the SMB-grade volumes Replider sees,
//     a single attempt with auto-disable on streak failures is enough.
//     A persistent retry table is the next layer when consumers
//     legitimately need at-least-once delivery.
//   - The dispatcher is a thin service, not a worker. It runs in the
//     caller's goroutine pool, not as a background loop, so events
//     fire as soon as they happen. Latency-sensitive use cases want
//     this; bulk replay would want a queue.
type OutboundWebhookDispatcher struct {
	repo            domain.AccountWebhookRepository
	httpClient      WebhookHTTPClient
	maxFailures     int
	deliveryTimeout time.Duration
	retrySchedule   []time.Duration
	maxAttempts     int
	// sleep waits d or returns early if ctx is cancelled. Injectable so
	// tests can assert the backoff schedule without burning wall-clock
	// on the real 5m/1h delays.
	sleep func(ctx context.Context, d time.Duration) error
	// afterDeliver, when set, is called after each subscription's
	// delivery finishes. Test-only seam for synchronizing on the
	// detached goroutine; nil in production.
	afterDeliver func()

	// Per-subscription token buckets. Keyed by webhook ID so each
	// subscription is throttled independently — one noisy webhook can't
	// starve another tenant's. rateLimit/rateBurst seed every new
	// bucket; overridable in tests to assert throttling without burning
	// wall-clock.
	rateLimit  rate.Limit
	rateBurst  int
	limitersMu sync.Mutex
	limiters   map[string]*rate.Limiter
}

func NewOutboundWebhookDispatcher(repo domain.AccountWebhookRepository) *OutboundWebhookDispatcher {
	d := &OutboundWebhookDispatcher{
		repo: repo,
		// SSRF-guarded client (US-F011): the destination is customer-chosen,
		// so the dialer re-checks the resolved IP and cross-host redirects
		// are refused. Default rather than main.go-only wiring — a
		// dispatcher built anywhere must be safe.
		httpClient:      httpsafe.Client(10 * time.Second),
		maxFailures:     10,
		deliveryTimeout: 30 * time.Second,
		retrySchedule:   defaultRetrySchedule,
		maxAttempts:     defaultMaxAttempts,
		sleep:           ctxSleep,
		rateLimit:       defaultDispatchRate,
		rateBurst:       defaultDispatchBurst,
		limiters:        map[string]*rate.Limiter{},
	}
	return d
}

// limiterFor returns the token bucket for a subscription, creating it on
// first use. The same subscription always gets the same bucket so the
// rate is shared across every Emit for that webhook.
func (d *OutboundWebhookDispatcher) limiterFor(subID string) *rate.Limiter {
	d.limitersMu.Lock()
	defer d.limitersMu.Unlock()
	lim, ok := d.limiters[subID]
	if !ok {
		lim = rate.NewLimiter(d.rateLimit, d.rateBurst)
		d.limiters[subID] = lim
	}
	return lim
}

// waitRate blocks until the subscription's bucket has a token or ctx is
// cancelled. Returns ctx.Err() if the wait would outlive the delivery
// budget — the event is then dropped rather than queueing unboundedly.
func (d *OutboundWebhookDispatcher) waitRate(ctx context.Context, subID string) error {
	return d.limiterFor(subID).Wait(ctx)
}

// ctxSleep blocks for d unless ctx is cancelled first, in which case it
// returns the context error so the retry loop can bail out cleanly.
func ctxSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// WithHTTPClient swaps the underlying client. Tests pass an httptest
// server's client to assert delivery; production sticks to the
// 10s-timeout default.
func (d *OutboundWebhookDispatcher) WithHTTPClient(c WebhookHTTPClient) *OutboundWebhookDispatcher {
	d.httpClient = c
	return d
}

// Emit fans out the event to every active subscription on the account.
// Returns immediately and runs deliveries on a detached goroutine —
// the inbound webhook handler must never block on a slow tenant
// endpoint, otherwise Meta gives up and retries the original webhook.
func (d *OutboundWebhookDispatcher) Emit(accountID, event string, payload any) {
	if d == nil || accountID == "" || event == "" {
		return
	}
	go d.run(accountID, event, payload)
}

func (d *OutboundWebhookDispatcher) run(accountID, event string, payload any) {
	// The retry budget must outlive the whole backoff schedule, not just
	// a single POST — otherwise the detached context would cancel mid-way
	// through the [1s,5s,30s,5m,1h] ladder. Budget = sum of the delays we
	// can actually consume + one delivery timeout per attempt.
	ctx, cancel := context.WithTimeout(context.Background(), d.retryBudget())
	defer cancel()

	subs, err := d.repo.ListActiveForEvent(ctx, accountID, event)
	if err != nil {
		slog.Warn("outbound webhook: list subscriptions failed",
			"account_id", accountID, "event", event, "error", err)
		return
	}
	if len(subs) == 0 {
		return
	}
	envelope := map[string]any{
		"id":         ulid.Make().String(),
		"event":      event,
		"account_id": accountID,
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"data":       payload,
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		slog.Warn("outbound webhook: marshal envelope failed",
			"account_id", accountID, "event", event, "error", err)
		return
	}

	for _, sub := range subs {
		// Throttle per subscription before spending a delivery attempt.
		// A sustained event flood is paced to rateLimit/sec (burst
		// rateBurst); if the wait would outlive the delivery budget the
		// bucket is empty for too long, so we drop this event rather than
		// pile up goroutines blocked on a hostile fan-out.
		if err := d.waitRate(ctx, sub.ID); err != nil {
			slog.Warn("outbound webhook: rate-limited, dropping event",
				"webhook_id", sub.ID, "account_id", accountID, "event", event, "error", err)
			if d.afterDeliver != nil {
				d.afterDeliver()
			}
			continue
		}
		d.deliver(ctx, sub, event, body)
		if d.afterDeliver != nil {
			d.afterDeliver()
		}
	}
}

// EmitToEndpoint fans a single event at an explicit URL+secret that is NOT an
// account_webhooks subscription row — used for per-funnel webhooks (US-016),
// where the URL/secret live on the sales_funnels row. It reuses the same HMAC
// signing, retry/backoff ladder, dead-letter logging and per-key throttle as
// Emit, but performs no repo bookkeeping (there's no failure_count/auto-disable
// for a funnel). rateKey scopes the throttle bucket (pass a funnel-stable key);
// accountID rides the envelope for consumer routing. Fire-and-forget.
func (d *OutboundWebhookDispatcher) EmitToEndpoint(rateKey, accountID, url, secret, event string, payload any) {
	if d == nil || url == "" || event == "" {
		return
	}
	go d.runEndpoint(rateKey, accountID, url, secret, event, payload)
}

func (d *OutboundWebhookDispatcher) runEndpoint(rateKey, accountID, url, secret, event string, payload any) {
	ctx, cancel := context.WithTimeout(context.Background(), d.retryBudget())
	defer cancel()

	envelope := map[string]any{
		"id":         ulid.Make().String(),
		"event":      event,
		"account_id": accountID,
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"data":       payload,
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		slog.Warn("funnel webhook: marshal envelope failed",
			"account_id", accountID, "event", event, "error", err)
		return
	}
	if err := d.waitRate(ctx, rateKey); err != nil {
		slog.Warn("funnel webhook: rate-limited, dropping event",
			"rate_key", rateKey, "account_id", accountID, "event", event, "error", err)
		if d.afterDeliver != nil {
			d.afterDeliver()
		}
		return
	}
	if delivered, lastMsg := d.deliverNoRepo(ctx, url, secret, event, body); !delivered {
		// Dead-letter: surface it loudly. A funnel has no failure_count/
		// auto-disable row, so an ERROR log (with the funnel rate_key +
		// account for triage) plus a metric are the only signal a customer's
		// webhook endpoint is down.
		outboundWebhookDeadLetterTotal.WithLabelValues(event).Inc()
		slog.Error("funnel webhook dead-letter (max attempts exhausted)",
			"rate_key", rateKey, "account_id", accountID, "event", event,
			"url", url, "attempts", d.maxAttempts, "last_error", lastMsg)
	}
	if d.afterDeliver != nil {
		d.afterDeliver()
	}
}

// deliverNoRepo runs the same attempt + backoff ladder as deliver but without
// any repo side effects — for endpoints that aren't account_webhooks rows
// (per-funnel webhooks, US-016). A 2xx short-circuits to success (true); once
// every attempt is spent (or the context dies) it returns false + the last
// error so the caller can dead-letter (log ERROR + metric).
func (d *OutboundWebhookDispatcher) deliverNoRepo(ctx context.Context, url, secret, event string, body []byte) (bool, string) {
	sub := &domain.AccountWebhook{URL: url, Secret: secret}
	var lastMsg string
	for attempt := 0; attempt < d.maxAttempts; attempt++ {
		ok, msg := d.attemptDeliver(ctx, sub, event, body)
		if ok {
			return true, ""
		}
		lastMsg = msg
		// No backoff after the final attempt — that path dead-letters.
		if attempt == d.maxAttempts-1 || attempt >= len(d.retrySchedule) {
			break
		}
		if err := d.sleep(ctx, d.retrySchedule[attempt]); err != nil {
			lastMsg = fmt.Sprintf("backoff aborted: %v (last error: %s)", err, msg)
			break
		}
	}
	return false, lastMsg
}

// retryBudget is the upper bound on how long a single subscription's
// delivery (initial attempt + all retries) may take, used to size the
// detached context so it spans the backoff schedule rather than cutting
// it short.
func (d *OutboundWebhookDispatcher) retryBudget() time.Duration {
	total := time.Duration(d.maxAttempts) * d.deliveryTimeout
	for i := 0; i < d.maxAttempts-1 && i < len(d.retrySchedule); i++ {
		total += d.retrySchedule[i]
	}
	return total
}

// deliver attempts delivery up to maxAttempts times, backing off along
// retrySchedule between failures. A 2xx short-circuits to success; once
// all attempts are spent (or the context dies) the event is
// dead-lettered. failure_count is bumped on every failed attempt so the
// existing auto-disable threshold still trips for chronically-broken
// endpoints.
func (d *OutboundWebhookDispatcher) deliver(ctx context.Context, sub *domain.AccountWebhook, event string, body []byte) {
	var lastMsg string
	for attempt := 0; attempt < d.maxAttempts; attempt++ {
		ok, msg := d.attemptDeliver(ctx, sub, event, body)
		if ok {
			_ = d.repo.RecordSuccess(ctx, sub.ID)
			return
		}
		lastMsg = msg
		d.markFailure(ctx, sub, msg)

		// No backoff after the final attempt — that path dead-letters.
		if attempt == d.maxAttempts-1 || attempt >= len(d.retrySchedule) {
			break
		}
		if err := d.sleep(ctx, d.retrySchedule[attempt]); err != nil {
			lastMsg = fmt.Sprintf("backoff aborted: %v (last error: %s)", err, msg)
			break
		}
	}
	d.deadLetter(ctx, sub, lastMsg)
}

// attemptDeliver performs one POST and reports whether it succeeded plus
// a human-readable failure reason. It records no repo side effects —
// the caller owns success/failure/dead-letter bookkeeping.
func (d *OutboundWebhookDispatcher) attemptDeliver(ctx context.Context, sub *domain.AccountWebhook, event string, body []byte) (bool, string) {
	// Re-check on EVERY attempt, not once per delivery (US-F011): the host
	// may resolve public now and internal on the retry. The dialer would
	// catch it anyway; failing here yields a readable last_error instead of
	// a generic dial failure.
	if err := httpsafe.ValidateURL(sub.URL); err != nil {
		return false, "blocked target: " + err.Error()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.URL, bytes.NewReader(body))
	if err != nil {
		return false, "build request: " + err.Error()
	}
	sig := hmacSign(body, sub.Secret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Replider-Event", event)
	req.Header.Set("X-Replider-Signature", "sha256="+sig)
	// User-Agent helps consumers route logs; matches the Meta pattern
	// of "Service/Version (purpose)".
	req.Header.Set("User-Agent", "Replider-Webhook/1.0 (outbound-events)")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return false, fmt.Sprintf("transport: %v", err)
	}
	defer resp.Body.Close()
	// Drain the body so the connection can return to the pool. We
	// don't care about consumer responses — a 200 is success, anything
	// else is failure, and we don't want to spam logs with response
	// content we can't act on.
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, ""
	}
	return false, fmt.Sprintf("status %d", resp.StatusCode)
}

// deadLetter is the terminal failure path: every attempt was exhausted.
// There's no persistent dead-letter table yet (SMB volumes don't need
// at-least-once replay), so we record a clearly-marked failure and log
// loudly so the event can be reconstructed from logs if a consumer
// disputes a missed delivery.
func (d *OutboundWebhookDispatcher) deadLetter(ctx context.Context, sub *domain.AccountWebhook, lastMsg string) {
	slog.Warn("outbound webhook dead-letter (max attempts exhausted)",
		"webhook_id", sub.ID, "account_id", sub.AccountID, "url", sub.URL,
		"attempts", d.maxAttempts, "last_error", lastMsg)
	_ = d.repo.RecordFailure(ctx, sub.ID, "dead-letter after "+fmt.Sprint(d.maxAttempts)+" attempts: "+lastMsg)
}

func (d *OutboundWebhookDispatcher) markFailure(ctx context.Context, sub *domain.AccountWebhook, msg string) {
	_ = d.repo.RecordFailure(ctx, sub.ID, msg)
	// Mirror the SQL increment on the in-memory struct so consecutive
	// retry attempts within this loop see a growing count (the row is
	// re-read fresh only on the next Emit). Disable fires exactly once,
	// on the attempt that crosses the threshold, so a multi-attempt
	// retry ladder doesn't spam Disable.
	sub.FailureCount++
	if sub.FailureCount == d.maxFailures {
		slog.Warn("outbound webhook auto-disabled (too many failures)",
			"webhook_id", sub.ID, "account_id", sub.AccountID, "url", sub.URL)
		_ = d.repo.Disable(ctx, sub.ID)
	}
}

// DeliverTest fires a synthetic `webhook.test` event at the given
// subscription synchronously so the operator can confirm their
// endpoint is reachable + signature valid before turning the row on
// for real traffic. Returns the HTTP status the consumer responded
// with so the dashboard can surface it. Skips RecordSuccess /
// RecordFailure side effects to keep the failure_count clean — the
// test is a manual ping, not real delivery.
func (d *OutboundWebhookDispatcher) DeliverTest(ctx context.Context, sub *domain.AccountWebhook) (int, error) {
	envelope := map[string]any{
		"id":         ulid.Make().String(),
		"event":      "webhook.test",
		"account_id": sub.AccountID,
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"data": map[string]any{
			"webhook_id":  sub.ID,
			"description": "Synthetic test event — your endpoint is reachable.",
		},
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.URL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	sig := hmacSign(body, sub.Secret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Replider-Event", "webhook.test")
	req.Header.Set("X-Replider-Signature", "sha256="+sig)
	req.Header.Set("User-Agent", "Replider-Webhook/1.0 (test-ping)")
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

// funnelTestBodyLimit caps how many bytes of the consumer's response body the
// funnel webhook test echoes back to the dashboard. The test surfaces the
// consumer's reply so the operator can debug their endpoint, but an unbounded
// read would let a hostile/misconfigured consumer stream us an arbitrarily
// large body — 8 KiB is plenty to show an error message or JSON ack.
const funnelTestBodyLimit = 8 << 10

// funnelTestTimeout bounds the synchronous admin ping (US-F011 AC: 5s).
// Async deliveries keep the dispatcher's longer per-attempt budget — they
// retry, so a slow-but-working consumer shouldn't be cut off.
const funnelTestTimeout = 5 * time.Second

// DeliverFunnelTest synchronously POSTs a synthetic `webhook.test` event to an
// explicit funnel URL+secret (US-017) and returns the consumer's HTTP status
// plus its (truncated) response body so the dashboard can confirm the webhook
// config end-to-end. Unlike EmitToEndpoint this is blocking, single-attempt and
// performs no retry/backoff/dead-letter — it's a manual ping behind an admin
// request, not real event delivery, so the operator wants the raw outcome now.
// The payload is signed with the same HMAC-SHA256 scheme as live deliveries so
// the consumer's signature verification is exercised too.
func (d *OutboundWebhookDispatcher) DeliverFunnelTest(ctx context.Context, accountID, url, secret string) (int, string, error) {
	// A funnel row created before US-F011 (or via a path that skipped the
	// write-time check) can still hold an internal URL — refuse it here too,
	// and bound the synchronous ping so an admin request can't hang on a
	// tarpit endpoint.
	if err := httpsafe.ValidateURL(url); err != nil {
		return 0, "", err
	}
	ctx, cancel := context.WithTimeout(ctx, funnelTestTimeout)
	defer cancel()

	envelope := map[string]any{
		"id":         ulid.Make().String(),
		"event":      "webhook.test",
		"account_id": accountID,
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"data": map[string]any{
			"description": "Synthetic funnel webhook test — your endpoint is reachable.",
		},
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return 0, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	sig := hmacSign(body, secret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Replider-Event", "webhook.test")
	req.Header.Set("X-Replider-Signature", "sha256="+sig)
	req.Header.Set("User-Agent", "Replider-Webhook/1.0 (funnel-test-ping)")
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, funnelTestBodyLimit))
	return resp.StatusCode, string(respBody), nil
}

// hmacSign returns the hex-encoded HMAC-SHA256 of body keyed by secret.
// Consumers verify by recomputing the same value and constant-time
// comparing to the X-Replider-Signature header.
func hmacSign(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

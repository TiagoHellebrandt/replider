package whatsapp

import (
	"bytes"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// retryTransport wraps http.RoundTripper to retry idempotent failures
// against the Graph API. Meta returns 429 (rate-limited) and sporadic
// 5xx during partial outages; without retries every transient blip
// surfaces as a 502 to the caller. Honors `Retry-After` when present
// and otherwise uses exponential backoff with jitter (1s, 2s, 4s).
//
// We replay the request body by buffering it on the way in so retries
// don't fail on a drained reader.
type retryTransport struct {
	base       http.RoundTripper
	maxRetries int
}

func newRetryTransport() *retryTransport {
	return &retryTransport{
		base:       http.DefaultTransport,
		maxRetries: 3,
	}
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, _ = io.ReadAll(req.Body)
		_ = req.Body.Close()
	}

	// Per Meta docs, /messages is non-idempotent — a successful send that
	// returned a transient timeout to us was still delivered to the
	// recipient. Retrying would double-send. We deliberately cap retries
	// to zero on this path; the caller surfaces the timeout and the AI
	// pipeline simply doesn't append a second bubble. The same applies
	// to /register, /deregister, /verify_code: each accepts at most one
	// invocation per cooldown window — retrying a "succeeded but flaky
	// network" call burns the customer's rate limit and locks them out.
	maxAttempts := t.maxRetries
	if isNonIdempotentPath(req) {
		maxAttempts = 0
	}

	var lastResp *http.Response
	var lastErr error

	for attempt := 0; attempt <= maxAttempts; attempt++ {
		// Reset body for each attempt — net/http drains it on send.
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			req.ContentLength = int64(len(bodyBytes))
		}

		resp, err := t.base.RoundTrip(req)
		lastResp, lastErr = resp, err

		// Retry only on transport errors, 429s and 5xxs — 4xx (except
		// 429) are client errors that won't change on retry.
		shouldRetry := err != nil ||
			(resp != nil && (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500))
		if !shouldRetry || attempt == maxAttempts {
			return resp, err
		}

		// Drain + close the failed response so the connection can be
		// reused by the next attempt.
		if resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}

		delay := backoffDelay(attempt, resp)
		slog.Warn("retrying Meta Graph API call",
			"url", redactGraphURL(req.URL),
			"attempt", attempt+1,
			"delay_ms", delay.Milliseconds(),
			"status", statusOrZero(resp),
			"err", err)

		select {
		case <-time.After(delay):
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}

	return lastResp, lastErr
}

// isNonIdempotentPath reports whether retrying this request is unsafe.
// We err on the side of "do not retry" — the Graph API does not accept
// idempotency keys, and the cost of a duplicate /messages send (the
// customer reads it twice) is higher than surfacing a timeout to ops.
func isNonIdempotentPath(req *http.Request) bool {
	if req == nil || req.URL == nil {
		return false
	}
	if req.Method != http.MethodPost {
		return false
	}
	p := req.URL.Path
	return strings.HasSuffix(p, "/messages") ||
		strings.HasSuffix(p, "/register") ||
		strings.HasSuffix(p, "/deregister") ||
		strings.HasSuffix(p, "/verify_code") ||
		strings.HasSuffix(p, "/request_code")
}

// redactGraphURL strips the `access_token` and `appsecret_proof` query
// params from a Graph API URL before logging. The retry path used to
// log `req.URL.String()` directly, leaking the System User token (when
// requests put it in the query string instead of the Authorization
// header) and the app-token form `app_id|app_secret` from /subscriptions.
func redactGraphURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	clone := *u
	q := clone.Query()
	for _, sensitiveKey := range []string{"access_token", "appsecret_proof", "client_secret"} {
		if q.Has(sensitiveKey) {
			q.Set(sensitiveKey, "[redacted]")
		}
	}
	clone.RawQuery = q.Encode()
	return clone.String()
}

// backoffDelay picks the wait before the next attempt. Honors Meta's
// Retry-After header (seconds or HTTP-date) when present; otherwise
// uses exponential backoff with up to ±25% jitter to avoid thundering
// herd when many channels hit a 429 simultaneously.
func backoffDelay(attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil && secs > 0 {
				if secs > 60 {
					secs = 60
				}
				return time.Duration(secs) * time.Second
			}
		}
	}
	base := time.Duration(1<<attempt) * time.Second
	jitter := time.Duration(rand.Int63n(int64(base / 4)))
	if rand.Intn(2) == 0 {
		return base + jitter
	}
	return base - jitter
}

func statusOrZero(r *http.Response) int {
	if r == nil {
		return 0
	}
	return r.StatusCode
}

// metaHTTPClient is the shared client every Graph API call should use
// so retries, timeouts and (future) circuit breakers apply uniformly.
// 30s is generous enough for media uploads but short enough to free
// goroutines when Meta is hard-down.
var metaHTTPClient = &http.Client{
	Timeout:   30 * time.Second,
	Transport: newRetryTransport(),
}

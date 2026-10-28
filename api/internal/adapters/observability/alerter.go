package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// FailureAlerter rolls individual failures into a threshold-based alert.
// The idea: a single AI timeout isn't worth waking anyone up, but 5
// failures in 60s means the provider is down or our config is wrong and
// we need to know NOW. Without this the only signal is an admin reading
// slog output, which nobody does.
//
// Posts a simple JSON payload to a Slack/Discord-compatible incoming
// webhook. Any service that accepts {"text": "..."} works.
type FailureAlerter struct {
	webhookURL string
	window     time.Duration
	threshold  int
	cooldown   time.Duration

	mu        sync.Mutex
	failures  map[string][]time.Time // component -> recent failure timestamps
	lastFired map[string]time.Time   // component -> last time we alerted
}

// NewFailureAlerter configures the roll-up. webhookURL empty = no-op
// alerter (safe for tests and local dev). Defaults: 5 failures in 60s
// trigger an alert; cooldown 10min prevents spam from the same incident.
func NewFailureAlerter(webhookURL string) *FailureAlerter {
	return &FailureAlerter{
		webhookURL: webhookURL,
		window:     60 * time.Second,
		threshold:  5,
		cooldown:   10 * time.Minute,
		failures:   make(map[string][]time.Time),
		lastFired:  make(map[string]time.Time),
	}
}

// Record registers a failure for the given component ("ai_provider",
// "whatsapp_send", etc.). Fires an alert when the threshold is crossed
// and cooldown is satisfied. Always returns fast — the HTTP post to the
// webhook runs async so call sites never slow down because of alerting.
func (a *FailureAlerter) Record(component, message string) {
	if a == nil || a.webhookURL == "" {
		return
	}
	now := time.Now()

	a.mu.Lock()
	defer a.mu.Unlock()

	// Drop samples outside the window before counting.
	kept := a.failures[component][:0]
	for _, t := range a.failures[component] {
		if now.Sub(t) < a.window {
			kept = append(kept, t)
		}
	}
	kept = append(kept, now)
	a.failures[component] = kept

	if len(kept) < a.threshold {
		return
	}
	if now.Sub(a.lastFired[component]) < a.cooldown {
		return
	}
	a.lastFired[component] = now
	// Reset counter — we've fired, don't re-alert on the same burst.
	a.failures[component] = nil

	go a.fire(component, message, len(kept))
}

func (a *FailureAlerter) fire(component, message string, count int) {
	text := fmt.Sprintf(":rotating_light: *Replider alert* — %d failures in %s\ncomponent: `%s`\nlast error: %s",
		count, a.window, component, message)
	payload, _ := json.Marshal(map[string]string{"text": text})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.webhookURL, bytes.NewReader(payload))
	if err != nil {
		slog.Warn("alerter: build request failed", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Warn("alerter: webhook post failed", "error", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		slog.Warn("alerter: webhook returned non-2xx", "status", resp.StatusCode)
	}
}

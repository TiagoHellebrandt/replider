package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	waadapter "github.com/replider/api/internal/adapters/channels/whatsapp"
	"github.com/replider/api/internal/core/domain"
)

// FailureReporter receives ops-grade alerts (token revoked, exchange
// refused) so the worker can surface problems beyond slog. Local
// mirror of the usecases-level interface to avoid an import cycle —
// callers wire the same alerter into both.
type FailureReporter interface {
	Record(component, message string)
}

// WATokenRefreshWorker rotates WhatsApp Cloud API access tokens before
// they expire. Embedded Signup hands us a 60d user-access token; left
// alone it dies silently after that window and the customer's channel
// goes dark. The worker:
//
//  1. Lists all active WhatsApp channels.
//  2. Calls /debug_token to learn each token's expires_at.
//  3. If <14d remain (configurable), exchanges it for a fresh 60d token
//     via the fb_exchange_token grant.
//  4. Persists the rotated token on the channel.
//  5. Reports failures (revoked tokens, exchange errors) to the alerter
//     so ops can intervene before the customer notices.
//
// Permanent System User tokens (no expires_at) are skipped — they don't
// rot and don't accept the exchange grant.
type WATokenRefreshWorker struct {
	channelRepo      domain.ChannelRepository
	appID            string
	appSecret        string
	alerter          FailureReporter
	pollInterval     time.Duration
	refreshThreshold time.Duration
	running          atomic.Bool
}

// NewWATokenRefreshWorker constructs the worker. Default cadence is 12h
// (token windows are days, no need to thrash) and refreshes anything
// inside the last 14 days of its window.
func NewWATokenRefreshWorker(
	channelRepo domain.ChannelRepository,
	appID, appSecret string,
) *WATokenRefreshWorker {
	return &WATokenRefreshWorker{
		channelRepo:      channelRepo,
		appID:            appID,
		appSecret:        appSecret,
		pollInterval:     12 * time.Hour,
		refreshThreshold: 14 * 24 * time.Hour,
	}
}

// SetAlerter wires a failure reporter so token rotation problems
// bubble up to the same alerts pipeline used by the AI failure path.
// Without it, problems only land in slog.
func (w *WATokenRefreshWorker) SetAlerter(a FailureReporter) {
	w.alerter = a
}

// WithPollInterval is for tests / staging — production keeps the 12h
// default to avoid hammering /debug_token.
func (w *WATokenRefreshWorker) WithPollInterval(d time.Duration) *WATokenRefreshWorker {
	w.pollInterval = d
	return w
}

// WithRefreshThreshold lets tests trigger refresh on tokens that still
// have most of their 60d window. Production keeps 14d.
func (w *WATokenRefreshWorker) WithRefreshThreshold(d time.Duration) *WATokenRefreshWorker {
	w.refreshThreshold = d
	return w
}

// Start runs the poll loop until ctx is cancelled. Safe to call once;
// repeat calls are no-ops to prevent double-start during hot reload.
func (w *WATokenRefreshWorker) Start(ctx context.Context) {
	if w.appID == "" || w.appSecret == "" {
		slog.Warn("WhatsApp token refresh worker disabled: missing app credentials")
		return
	}
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("wa token refresh worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)

	slog.Info("wa token refresh worker started",
		"poll_interval", w.pollInterval,
		"refresh_threshold", w.refreshThreshold)

	// One immediate sweep — covers the case where the API restarted with
	// channels that were already <14d from expiry.
	w.tick(ctx)

	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("wa token refresh worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

// waTokenRefreshConcurrency caps simultaneous Graph API calls per tick
// so a slow Meta upstream on one channel doesn't block all subsequent
// rotations. Set conservatively — Meta's per-app rate limit is the
// other constraint, and bursting too aggressively risks 429ing the
// whole tick.
const waTokenRefreshConcurrency = 8

func (w *WATokenRefreshWorker) tick(ctx context.Context) {
	channels, err := w.channelRepo.ListAllByProvider(ctx, "whatsapp")
	if err != nil {
		slog.Warn("wa token refresh: list channels failed", "error", err)
		return
	}
	if len(channels) == 0 {
		return
	}

	// Bounded parallelism: a 30s Meta timeout × 100 channels would
	// otherwise serialize to 50 minutes per tick. We cap concurrency
	// with a buffered semaphore and per-channel timeout so one slow
	// channel can't starve the rest.
	sem := make(chan struct{}, waTokenRefreshConcurrency)
	var wg sync.WaitGroup
	for _, ch := range channels {
		if ctx.Err() != nil {
			return
		}
		ch := ch
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			perChannelCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
			defer cancel()
			w.processChannel(perChannelCtx, ch)
		}()
	}
	wg.Wait()
}

// processChannel inspects one WhatsApp channel and rotates its token if
// the expiry window is closing. All branches log enough detail for ops
// to follow up; failures (revoked, exchange refused) trip the alerter.
func (w *WATokenRefreshWorker) processChannel(ctx context.Context, ch *domain.Channel) {
	cfg := decodeWAConfig(ch.Config)
	token := cfg["access_token"]
	if token == "" {
		// Channel never finished signup or token was masked out; nothing
		// to refresh — operator must rerun Embedded Signup.
		return
	}

	info, err := waadapter.DebugToken(ctx, w.appID, w.appSecret, token)
	if err != nil {
		slog.Warn("wa token refresh: debug_token failed",
			"channel_id", ch.ID, "error", err)
		w.alert("debug_token failed for channel " + ch.ID + ": " + err.Error())
		return
	}
	if !info.IsValid {
		// Permanent loss — token was revoked (admin rotation, user
		// password change, app uninstall). Refresh can't recover this.
		// Alert + leave the value in place so the dashboard's
		// reconnect CTA fires when the next webhook hits a 401.
		slog.Warn("wa token refresh: token invalid, requires reconnect",
			"channel_id", ch.ID)
		w.alert("WhatsApp token invalid for channel " + ch.ID + " — tenant must reconnect via Embedded Signup")
		return
	}

	// Permanent System User tokens have no expires_at. Skip — these
	// don't accept fb_exchange_token and we'd just spam errors.
	if info.ExpiresAt.IsZero() {
		return
	}
	remaining := time.Until(info.ExpiresAt)
	if remaining > w.refreshThreshold {
		return
	}

	slog.Info("wa token refresh: rotating token",
		"channel_id", ch.ID,
		"expires_at", info.ExpiresAt,
		"remaining", remaining)

	newToken, err := waadapter.ExchangeForLongLivedToken(ctx, w.appID, w.appSecret, token)
	if err != nil {
		slog.Warn("wa token refresh: exchange failed",
			"channel_id", ch.ID, "error", err)
		w.alert("WhatsApp token exchange failed for channel " + ch.ID + ": " + err.Error())
		return
	}
	// Partial JSONB update via jsonb_set so we don't read-modify-write
	// the whole credentials blob — concurrent admin writes (e.g. the
	// /channels/.../whatsapp/register handler updating
	// registration_status while this worker is mid-flight) would
	// otherwise be silently dropped because we'd persist the cfg map
	// we read seconds earlier.
	if err := w.channelRepo.UpdateCredentialField(ctx, ch.ID, "access_token", newToken); err != nil {
		slog.Warn("wa token refresh: persist token failed",
			"channel_id", ch.ID, "error", err)
		w.alert("WhatsApp token rotated but persist failed for channel " + ch.ID + ": " + err.Error())
		return
	}
	slog.Info("wa token refreshed",
		"channel_id", ch.ID,
		"old_expires_at", info.ExpiresAt,
		"window_extended_by_days", 60)
}

// decodeWAConfig pulls the credential map out of a channel.Config blob
// regardless of whether the upstream code stored it as a `[]byte` (the
// common case) or already-decoded `map[string]any`.
func decodeWAConfig(cfg interface{}) map[string]string {
	out := map[string]string{}
	switch v := cfg.(type) {
	case []byte:
		_ = json.Unmarshal(v, &out)
	case map[string]interface{}:
		for k, val := range v {
			if s, ok := val.(string); ok {
				out[k] = s
			}
		}
	case map[string]string:
		for k, val := range v {
			out[k] = val
		}
	}
	return out
}

func (w *WATokenRefreshWorker) alert(msg string) {
	if w.alerter == nil {
		return
	}
	w.alerter.Record("whatsapp_token_refresh", msg)
}

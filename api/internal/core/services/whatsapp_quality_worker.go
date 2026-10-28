package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// WAChannelStore is the slim port the quality worker needs from the
// channel repository — list every WhatsApp channel and merge a few
// status keys into its credentials JSONB. Satisfied by the concrete
// postgres ChannelRepository.
type WAChannelStore interface {
	ListAllByProvider(ctx context.Context, provider string) ([]*domain.Channel, error)
	MergeCredentials(ctx context.Context, channelID string, fields map[string]string) error
}

// WAPhoneProber fetches Meta's quality_rating + messaging_limit_tier for
// a phone number. Implemented in main.go as a thin wrapper over
// whatsapp.GetPhoneNumberStatus (kept out of core/ so this package
// doesn't import the channel adapter). accessToken may be empty — the
// wrapper falls back to the app/system token.
type WAPhoneProber interface {
	PhoneStatus(ctx context.Context, phoneNumberID, accessToken string) (rating, tier string, err error)
}

// WhatsAppQualityWorker periodically refreshes every WhatsApp number's
// Meta quality rating (GREEN/YELLOW/RED) and messaging-limit tier into
// the channel's credentials JSONB, so the owner panel can flag a number
// at risk of being banned across ALL accounts without an N-call live
// fan-out on every list render. A drop to RED stamps quality_degraded_at
// so we can tell a freshly-bad number from a chronically-bad one.
type WhatsAppQualityWorker struct {
	store        WAChannelStore
	prober       WAPhoneProber
	pollInterval time.Duration
	running      atomic.Bool
}

func NewWhatsAppQualityWorker(store WAChannelStore, prober WAPhoneProber) *WhatsAppQualityWorker {
	return &WhatsAppQualityWorker{
		store:        store,
		prober:       prober,
		pollInterval: 30 * time.Minute,
	}
}

// WithPollInterval overrides the tick cadence (tests use ms-scale).
func (w *WhatsAppQualityWorker) WithPollInterval(d time.Duration) *WhatsAppQualityWorker {
	w.pollInterval = d
	return w
}

// Start runs the refresh loop until ctx is cancelled. Once-per-process.
func (w *WhatsAppQualityWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("whatsapp quality worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("whatsapp quality worker started", "poll_interval", w.pollInterval)

	w.tick(ctx)
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("whatsapp quality worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *WhatsAppQualityWorker) tick(ctx context.Context) {
	channels, err := w.store.ListAllByProvider(ctx, "whatsapp")
	if err != nil {
		slog.Warn("whatsapp quality worker: failed to list channels", "error", err)
		return
	}
	var probed, skipped, failed int
	for _, ch := range channels {
		if ctx.Err() != nil {
			return
		}
		if !ch.IsActive {
			skipped++
			continue
		}
		creds := parseChannelCreds(ch.Config)
		phoneID := creds["phone_number_id"]
		if phoneID == "" {
			skipped++
			continue
		}
		rating, tier, err := w.prober.PhoneStatus(ctx, phoneID, creds["access_token"])
		if err != nil {
			// One number's probe failure (token expired, rate limit) must
			// not stop the sweep — log and move on.
			failed++
			continue
		}
		if rating == "" {
			skipped++
			continue
		}
		fields := map[string]string{
			"quality_rating":       rating,
			"messaging_limit_tier": tier,
			"quality_checked_at":   time.Now().UTC().Format(time.RFC3339),
		}
		// Stamp the moment a number first turns RED so the dashboard can
		// distinguish a fresh degradation from a long-standing one.
		if rating == "RED" && creds["quality_rating"] != "RED" {
			fields["quality_degraded_at"] = time.Now().UTC().Format(time.RFC3339)
		}
		if err := w.store.MergeCredentials(ctx, ch.ID, fields); err != nil {
			failed++
			continue
		}
		probed++
	}
	if len(channels) > 0 {
		slog.Info("whatsapp quality worker: tick complete",
			"channels", len(channels), "probed", probed,
			"skipped", skipped, "failed", failed)
	}
}

// parseChannelCreds reads a channel's credentials JSONB (stored as raw
// bytes on Config) into a flat string map. Non-string values and parse
// errors yield an empty map so the caller just skips the channel.
func parseChannelCreds(config interface{}) map[string]string {
	out := map[string]string{}
	var raw []byte
	switch v := config.(type) {
	case []byte:
		raw = v
	case json.RawMessage:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return out
	}
	if len(raw) == 0 {
		return out
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return out
	}
	for k, v := range m {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

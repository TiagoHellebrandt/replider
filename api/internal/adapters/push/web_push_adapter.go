// Package push holds outbound push-notification adapters. Today: Web Push
// (W3C + VAPID) via SherClockHolmes/webpush-go. Future: FCM (Android) and
// APNs (iOS) adapters plug into the same domain.PushSender port.
package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/replider/api/internal/core/domain"
)

// WebPushAdapter sends W3C Web Push notifications signed with a VAPID keypair.
// When the keypair is absent the adapter becomes a no-op that logs instead
// of sending — the same stub-mode used by SMTPAdapter in dev.
type WebPushAdapter struct {
	publicKey  string
	privateKey string
	subject    string // e.g. "mailto:ops@replider.com"
	httpClient *http.Client
}

func NewWebPushAdapter(publicKey, privateKey, subject string) *WebPushAdapter {
	if subject == "" {
		subject = "mailto:noreply@replider.com"
	}
	return &WebPushAdapter{
		publicKey:  publicKey,
		privateKey: privateKey,
		subject:    subject,
		// Bounded timeout — a slow/hung push endpoint must not stall
		// the outbound notification fan-out indefinitely.
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// VAPIDPublicKey returns the URL-safe base64 public key the client embeds
// in PushManager.subscribe({applicationServerKey}).
func (a *WebPushAdapter) VAPIDPublicKey() string { return a.publicKey }

func (a *WebPushAdapter) configured() bool {
	return a.publicKey != "" && a.privateKey != ""
}

// Send ships payload to a single subscription. Returns domain.ErrPushGone
// on 404/410 so the usecase can disable the dead endpoint.
func (a *WebPushAdapter) Send(ctx context.Context, sub *domain.PushSubscription, payload domain.PushPayload) error {
	if !a.configured() {
		// Warn (not Info) so production deployments missing the VAPID
		// secrets surface in the log monitor instead of silently
		// dropping every push. Set WEBPUSH_VAPID_PUBLIC_KEY +
		// WEBPUSH_VAPID_PRIVATE_KEY in the production-api environment
		// secrets to enable delivery.
		slog.Warn("webpush stub send — VAPID keys not configured",
			"endpoint", sub.Endpoint, "title", payload.Title)
		return nil
	}

	// Mobile platforms ignore Web Push; they need FCM/APNs adapters.
	if sub.Platform != "web" {
		slog.Debug("webpush skip non-web subscription", "platform", sub.Platform, "id", sub.ID)
		return nil
	}

	if sub.P256dhKey == nil || sub.AuthKey == nil {
		return fmt.Errorf("web push subscription missing keys: %s", sub.ID)
	}

	s := &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: *sub.P256dhKey,
			Auth:   *sub.AuthKey,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal push payload: %w", err)
	}

	opts := &webpush.Options{
		Subscriber:      a.subject,
		VAPIDPublicKey:  a.publicKey,
		VAPIDPrivateKey: a.privateKey,
		TTL:             60, // seconds — notifications stale fast; do not hoard
		HTTPClient:      a.httpClient,
	}

	resp, err := webpush.SendNotificationWithContext(ctx, body, s, opts)
	if err != nil {
		return fmt.Errorf("webpush deliver: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound {
		return domain.ErrPushGone
	}
	// 401/403 from the push service almost always means the VAPID
	// keypair on the server no longer matches the applicationServerKey
	// the browser stored when subscribing — typically because the
	// VAPID secrets were rotated server-side without re-subscribing
	// every browser. Treat those as Gone so the dead row is disabled
	// and the user can re-enable from Settings to mint a fresh
	// subscription bound to the current key.
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		buf, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		slog.Warn("webpush auth rejected — VAPID key likely rotated, disabling subscription",
			"status", resp.StatusCode, "endpoint", sub.Endpoint, "body", bytes.TrimSpace(buf))
		return domain.ErrPushGone
	}
	if resp.StatusCode >= 400 {
		buf, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("webpush status %d: %s", resp.StatusCode, bytes.TrimSpace(buf))
	}
	return nil
}

var _ domain.PushSender = (*WebPushAdapter)(nil)

package usecases

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// PushNotificationUseCase owns push-subscription CRUD plus the outbound
// fan-out used by the inbound pipeline to poke attendants when a new
// customer message lands. The "fan-out" is intentionally best-effort: a
// push failure never blocks the core message flow.
type PushNotificationUseCase struct {
	repo   domain.PushSubscriptionRepository
	sender domain.PushSender
}

func NewPushNotificationUseCase(repo domain.PushSubscriptionRepository, sender domain.PushSender) *PushNotificationUseCase {
	return &PushNotificationUseCase{repo: repo, sender: sender}
}

// SubscribeDTO is what the client POSTs to /v1/push/subscriptions.
type SubscribeDTO struct {
	Platform    string // "web", "ios", "android"
	Endpoint    string
	P256dh      string // web only
	Auth        string // web only
	DeviceToken string // mobile only
	UserAgent   string
}

// Subscribe upserts a push subscription owned by the calling user.
func (uc *PushNotificationUseCase) Subscribe(ctx context.Context, accountID, userID string, dto SubscribeDTO) (*domain.PushSubscription, error) {
	platform := strings.ToLower(strings.TrimSpace(dto.Platform))
	if platform == "" {
		platform = "web"
	}
	if platform != "web" && platform != "ios" && platform != "android" {
		return nil, &domain.AppError{Code: "PUSH_INVALID_PLATFORM", Message: "platform must be web, ios or android"}
	}
	if strings.TrimSpace(dto.Endpoint) == "" {
		return nil, &domain.AppError{Code: "PUSH_INVALID_ENDPOINT", Message: "endpoint is required"}
	}
	if platform == "web" && (dto.P256dh == "" || dto.Auth == "") {
		return nil, &domain.AppError{Code: "PUSH_MISSING_KEYS", Message: "web subscriptions require p256dh and auth keys"}
	}

	sub := &domain.PushSubscription{
		ID:        ulid.Make().String(),
		UserID:    userID,
		AccountID: accountID,
		Platform:  platform,
		Endpoint:  dto.Endpoint,
		Enabled:   true,
	}
	if dto.P256dh != "" {
		v := dto.P256dh
		sub.P256dhKey = &v
	}
	if dto.Auth != "" {
		v := dto.Auth
		sub.AuthKey = &v
	}
	if dto.DeviceToken != "" {
		v := dto.DeviceToken
		sub.DeviceToken = &v
	}
	if dto.UserAgent != "" {
		v := dto.UserAgent
		sub.UserAgent = &v
	}

	if err := uc.repo.Upsert(ctx, sub); err != nil {
		return nil, err
	}
	return sub, nil
}

// Unsubscribe removes the row by (userID, endpoint). Idempotent.
func (uc *PushNotificationUseCase) Unsubscribe(ctx context.Context, userID, endpoint string) error {
	if endpoint == "" {
		return &domain.AppError{Code: "PUSH_INVALID_ENDPOINT", Message: "endpoint is required"}
	}
	return uc.repo.DeleteByEndpoint(ctx, userID, endpoint)
}

// List returns the caller's active subscriptions — useful so the Flutter
// client can decide whether the current browser is already subscribed.
func (uc *PushNotificationUseCase) List(ctx context.Context, userID string) ([]*domain.PushSubscription, error) {
	return uc.repo.ListByUserID(ctx, userID)
}

// VAPIDPublicKey is exposed at /v1/push/vapid-public-key so the web client
// can embed it in PushManager.subscribe(). Empty string when push is
// disabled (keys unconfigured).
func (uc *PushNotificationUseCase) VAPIDPublicKey() string {
	if uc.sender == nil {
		return ""
	}
	return uc.sender.VAPIDPublicKey()
}

// NotifyAccount fans a payload out to every enabled subscription under the
// account, optionally excluding a user id (typically the actor). Runs
// sequentially — accounts have O(team size × devices) rows, a handful of
// sends each. Dead endpoints (410/404) are disabled in-line.
//
// Intended to be invoked from a goroutine so it never blocks the caller's
// request path.
// HasSubscribers reports whether anyone on the account can receive a push
// (US-M010). Callers use it to decide whether email is a FALLBACK or a
// duplicate — a lookup failure answers false, so the digest falls back to
// email rather than vanishing.
func (uc *PushNotificationUseCase) HasSubscribers(ctx context.Context, accountID string) bool {
	if uc.sender == nil || uc.repo == nil {
		return false
	}
	subs, err := uc.repo.ListByAccountID(ctx, accountID)
	if err != nil {
		slog.WarnContext(ctx, "push: subscriber check failed", "account_id", accountID, "error", err)
		return false
	}
	return len(subs) > 0
}

func (uc *PushNotificationUseCase) NotifyAccount(ctx context.Context, accountID, excludeUserID string, payload domain.PushPayload) {
	if uc.sender == nil {
		return
	}
	subs, err := uc.repo.ListByAccountID(ctx, accountID)
	if err != nil {
		slog.Error("push: list subscriptions failed", "account_id", accountID, "error", err)
		return
	}
	payload.AccountID = accountID
	uc.fanOut(ctx, subs, excludeUserID, payload)
}

// NotifyUser fans a payload out to every enabled subscription owned by ONE
// user, regardless of account — used by the internal CS channel (US-004) to
// poke the thread owner and only them. Same best-effort semantics as
// NotifyAccount. Payload.AccountID is stamped per subscription (a user has
// exactly one account today, but the row carries it anyway).
func (uc *PushNotificationUseCase) NotifyUser(ctx context.Context, userID string, payload domain.PushPayload) {
	if uc.sender == nil {
		return
	}
	subs, err := uc.repo.ListByUserID(ctx, userID)
	if err != nil {
		slog.Error("push: list user subscriptions failed", "user_id", userID, "error", err)
		return
	}
	if len(subs) > 0 && payload.AccountID == "" {
		payload.AccountID = subs[0].AccountID
	}
	uc.fanOut(ctx, subs, "", payload)
}

// fanOut delivers one payload to a subscription set, disabling dead
// endpoints (410/404) in-line. Sequential on purpose — the sets are
// O(devices per user/team), a handful of sends each.
func (uc *PushNotificationUseCase) fanOut(ctx context.Context, subs []*domain.PushSubscription, excludeUserID string, payload domain.PushPayload) {
	for _, sub := range subs {
		if excludeUserID != "" && sub.UserID == excludeUserID {
			continue
		}
		if err := uc.sender.Send(ctx, sub, payload); err != nil {
			if errors.Is(err, domain.ErrPushGone) || isAppErr(err, "PUSH_GONE") {
				if derr := uc.repo.DisableByEndpoint(ctx, sub.Endpoint); derr != nil {
					slog.Warn("push: failed to disable dead endpoint", "id", sub.ID, "error", derr)
				}
				continue
			}
			slog.Warn("push: send failed", "id", sub.ID, "user_id", sub.UserID, "error", err)
			continue
		}
		_ = uc.repo.Touch(ctx, sub.ID)
	}
}

// NotifySupportMessage fires the internal-CS-thread push (US-004): a
// platform-side message (seller reply, takeover send, AI reply) pokes the
// THREAD OWNER's devices with a deep link to the support widget. Runs in a
// detached goroutine with panic recovery — push must never block or crash
// the send path. Satisfies the SupportPusher ports in usecases and main.go.
//
// Localisation follows the inbound-push policy: Body is the raw message
// text; Title stays empty so the service worker renders a locale-aware
// "Replider" title keyed by Event="support_message".
func (uc *PushNotificationUseCase) NotifySupportMessage(userID, conversationID string, msg *domain.Message) {
	if uc == nil || msg == nil || userID == "" {
		return
	}
	var body string
	if msg.Content != nil {
		body = *msg.Content
	}
	payload := domain.PushPayload{
		Body:           body, // empty → SW renders localised media-type fallback
		MediaType:      msg.Type,
		Tag:            "support:" + conversationID,
		ConversationID: conversationID,
		Event:          "support_message",
		ClickURL:       "/support",
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("support push panic", "recover", r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		uc.NotifyUser(ctx, userID, payload)
	}()
}

// PushTestResult holds the outcome of a single test-push delivery so the
// diagnostic endpoint can surface per-subscription status to the caller.
// EndpointHost is a trimmed form of the W3C endpoint (host only) — full
// URLs include user-identifying tokens we don't want bouncing through
// the dashboard logs.
type PushTestResult struct {
	SubscriptionID string `json:"subscription_id"`
	Platform       string `json:"platform"`
	EndpointHost   string `json:"endpoint_host"`
	UserAgent      string `json:"user_agent,omitempty"`
	OK             bool   `json:"ok"`
	Disabled       bool   `json:"disabled"` // true when we just disabled the row (Gone)
	Error          string `json:"error,omitempty"`
}

// SendTestToUser fires a synthetic push payload at every enabled
// subscription owned by the calling user and returns per-endpoint
// outcomes so the dashboard "test notifications" button can show why
// pushes aren't arriving (no rows, gone, VAPID mismatch, etc.). Runs
// inline (not goroutine) because the caller is waiting on the result.
func (uc *PushNotificationUseCase) SendTestToUser(ctx context.Context, userID string) ([]PushTestResult, error) {
	if uc.sender == nil {
		return []PushTestResult{}, nil
	}
	subs, err := uc.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	results := make([]PushTestResult, 0, len(subs))
	// Title + Body left empty on purpose — the service-worker maps
	// Event="test" to a locale-aware "Replider — test" / "Notificação
	// de teste" pair so a single API doesn't have to know whether the
	// admin's browser is in English, Portuguese, or anything else.
	payload := domain.PushPayload{
		Tag:      "replider:test",
		Event:    "test",
		ClickURL: "/",
	}
	for _, sub := range subs {
		host := sub.Endpoint
		if u, perr := urlHost(sub.Endpoint); perr == nil {
			host = u
		}
		ua := ""
		if sub.UserAgent != nil {
			ua = *sub.UserAgent
		}
		entry := PushTestResult{
			SubscriptionID: sub.ID,
			Platform:       sub.Platform,
			EndpointHost:   host,
			UserAgent:      ua,
		}
		if err := uc.sender.Send(ctx, sub, payload); err != nil {
			entry.Error = err.Error()
			if errors.Is(err, domain.ErrPushGone) || isAppErr(err, "PUSH_GONE") {
				entry.Disabled = true
				if derr := uc.repo.DisableByEndpoint(ctx, sub.Endpoint); derr != nil {
					slog.Warn("push: failed to disable dead endpoint", "id", sub.ID, "error", derr)
				}
			}
			results = append(results, entry)
			continue
		}
		_ = uc.repo.Touch(ctx, sub.ID)
		entry.OK = true
		results = append(results, entry)
	}
	return results, nil
}

// urlHost trims an endpoint URL down to scheme://host so log output and
// API responses don't echo the user-identifying push-service token. The
// raw error stays on the server side for ops debugging.
func urlHost(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return raw, nil
	}
	return u.Scheme + "://" + u.Host, nil
}

func isAppErr(err error, code string) bool {
	var app *domain.AppError
	if errors.As(err, &app) {
		return app.Code == code
	}
	return false
}

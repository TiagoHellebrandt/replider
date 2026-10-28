package domain

import (
	"context"
	"time"
)

// PushSubscription holds a single push endpoint owned by a dashboard user.
// Web subscriptions use Endpoint + P256dhKey + AuthKey (W3C Push API).
// Mobile (future FCM/APNs) uses DeviceToken; Endpoint is filled with the
// provider's topic URL for parity.
type PushSubscription struct {
	ID          string
	UserID      string
	AccountID   string
	Platform    string // "web", "ios", "android"
	Endpoint    string
	P256dhKey   *string
	AuthKey     *string
	DeviceToken *string
	UserAgent   *string
	Enabled     bool
	LastUsedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// PushPayload is the JSON body shipped to the client (web SW or mobile app).
// Kept intentionally flat — stringly-typed fields survive JSON/round-trips on
// every platform.
type PushPayload struct {
	Title string `json:"title"`
	// Body carries the literal message text — not localised on the
	// server because the only locale the API can know is the account's
	// default, and a single account may have attendants in different
	// languages. When empty (e.g. media-only inbound), the client's SW
	// renders a localised fallback keyed by [MediaType].
	Body string `json:"body"`
	// MediaType is "image" / "audio" / "voice" / "video" / "document" /
	// empty. The service-worker uses it to render a translated body
	// like "📷 Image" or "🎙️ Áudio" without baking PT-BR strings into
	// the API payload. Always set on inbound notifications, even when
	// Body is non-empty, so the SW can append a media badge if it
	// wants to.
	MediaType      string `json:"media_type,omitempty"`
	Icon           string `json:"icon,omitempty"`
	Badge          string `json:"badge,omitempty"`
	Tag            string `json:"tag,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	AccountID      string `json:"account_id,omitempty"`
	ClickURL       string `json:"click_url,omitempty"`
	Event          string `json:"event,omitempty"` // e.g. "new_message"
}

// PushSubscriptionRepository persists push endpoints per user.
type PushSubscriptionRepository interface {
	Upsert(ctx context.Context, sub *PushSubscription) error
	ListByAccountID(ctx context.Context, accountID string) ([]*PushSubscription, error)
	ListByUserID(ctx context.Context, userID string) ([]*PushSubscription, error)
	DeleteByEndpoint(ctx context.Context, userID, endpoint string) error
	DisableByEndpoint(ctx context.Context, endpoint string) error
	Touch(ctx context.Context, id string) error
}

// PushSender ships a payload to a single subscription. Returns ErrPushGone
// when the endpoint is permanently dead (HTTP 404/410) so the caller can
// disable it. Transient errors (network, 5xx) bubble up as regular errors.
type PushSender interface {
	Send(ctx context.Context, sub *PushSubscription, payload PushPayload) error
	VAPIDPublicKey() string
}

// ErrPushGone signals a dead subscription (410/404 from the push service).
// Callers should disable the subscription row.
var ErrPushGone = &AppError{Code: "PUSH_GONE", Message: "push subscription gone"}

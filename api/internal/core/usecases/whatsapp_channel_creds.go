package usecases

import (
	"context"
	"encoding/json"

	"github.com/replider/api/internal/core/domain"
)

// The two helpers the WhatsApp template sync needs to reach a channel's Graph
// credentials.
//
// They used to live in whatsapp_flow_usecases.go and outlived it: Flows was
// removed (nothing in the app ever called its ten routes) while the template
// sync is in use. Kept in their own file rather than folded into
// whatsapp_template_usecases.go because a second WhatsApp use case will want
// them and hunting them inside an unrelated file is how they get duplicated.

// WAChannelResolver locates a WhatsApp channel by id.
//
// Use cases depend on this one-method interface rather than on
// domain.ChannelRepository so the wiring stays thin — and so a test fake stubs
// one method instead of the full repository.
type WAChannelResolver interface {
	GetByID(ctx context.Context, id string) (*domain.Channel, error)
}

// extractWAChannelCreds parses the WhatsApp credentials (waba_id,
// phone_number_id, access_token) out of channel.Config.
//
// Mirrors the inline pattern in WhatsAppConfigHandler.resolveWAChannel, kept
// here so the use case layer does not import HTTP helpers. The Config field is
// an `any` that arrives as bytes, RawMessage or a decoded map depending on the
// path, hence the switch.
func extractWAChannelCreds(ch *domain.Channel) (wabaID, phoneNumberID, accessToken string, err error) {
	if ch == nil {
		return "", "", "", &domain.AppError{Code: "CHANNEL_NOT_FOUND", Message: "channel not found"}
	}
	if ch.Type != "whatsapp" {
		return "", "", "", &domain.AppError{Code: "INVALID_CHANNEL_TYPE", Message: "only whatsapp channels expose templates"}
	}
	cfg := map[string]string{}
	switch v := ch.Config.(type) {
	case []byte:
		_ = json.Unmarshal(v, &cfg)
	case json.RawMessage:
		_ = json.Unmarshal([]byte(v), &cfg)
	case map[string]interface{}:
		for k, val := range v {
			if s, ok := val.(string); ok {
				cfg[k] = s
			}
		}
	default:
		raw, _ := json.Marshal(v)
		_ = json.Unmarshal(raw, &cfg)
	}
	wabaID = cfg["waba_id"]
	phoneNumberID = cfg["phone_number_id"]
	accessToken = cfg["access_token"]
	if wabaID == "" || accessToken == "" {
		return "", "", "", &domain.AppError{Code: "CHANNEL_MISSING_CONFIG", Message: "channel missing waba_id or access_token"}
	}
	return wabaID, phoneNumberID, accessToken, nil
}

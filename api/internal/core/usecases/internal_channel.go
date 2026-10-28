package usecases

import (
	"context"
	"encoding/json"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// ──────────────────────────────── EnsureInternalChannelUseCase ────────────────────────────────

// EnsureInternalChannelUseCase creates the platform account's single
// "internal" CS channel if it doesn't exist yet. Called once at startup
// when PLATFORM_ACCOUNT_ID is configured; idempotent across restarts
// (list-then-create — the API boots as a single process, so no
// concurrent-create race to guard). Goes straight to the repo,
// deliberately bypassing CreateChannelUseCase: internal channels are not
// customer-creatable and must not count against the plan channel cap.
type EnsureInternalChannelUseCase struct {
	channelRepo domain.ChannelRepository
}

func NewEnsureInternalChannelUseCase(channelRepo domain.ChannelRepository) *EnsureInternalChannelUseCase {
	return &EnsureInternalChannelUseCase{channelRepo: channelRepo}
}

// Execute returns the existing internal channel or the newly created one.
// A blank platformAccountID (feature not enabled) returns (nil, nil).
func (uc *EnsureInternalChannelUseCase) Execute(ctx context.Context, platformAccountID string) (*domain.Channel, error) {
	if platformAccountID == "" {
		return nil, nil
	}

	existing, err := uc.channelRepo.ListByAccountID(ctx, platformAccountID)
	if err != nil {
		return nil, err
	}
	for _, ch := range existing {
		if ch.Type == domain.ChannelTypeInternal {
			return ch, nil
		}
	}

	ch := &domain.Channel{
		ID:        ulid.Make().String(),
		AccountID: platformAccountID,
		Type:      domain.ChannelTypeInternal,
		Name:      "Suporte Replider",
		// No external webhook — the token only satisfies the NOT NULL
		// column and keeps the shared /webhooks/{token} route unroutable
		// to this channel type (HandleWebhook rejects it anyway).
		WebhookToken: ulid.Make().String(),
		Config:       json.RawMessage(`{}`),
		IsActive:     true,
	}
	if err := uc.channelRepo.Create(ctx, ch); err != nil {
		return nil, err
	}
	return ch, nil
}

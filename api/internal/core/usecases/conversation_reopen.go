package usecases

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// reopenIfClosed is what SaveInbound asks after FindByContact came back
// empty: is there a RESOLVED thread for this sender on this channel? If so
// it is reopened and returned, so the customer's reply lands on the
// conversation that holds the history instead of on a fresh one.
//
// Before this, "Resolver" put the thread in `closed`, FindByContact skips
// `closed`, and the next reply opened a second conversation with no
// context — the attendant answered a message whose question they could
// not see.
//
// The repository reaches this through the optional
// domain.ConversationReopener port; a repository without it (every
// testify mock of ConversationRepository) keeps the old behaviour, which
// is why the port is not on the main interface. Returns (nil, nil) when
// there is nothing to reopen.
func (uc *InboundWebhookUseCase) reopenIfClosed(ctx context.Context, channelID, contactID string) (*domain.Conversation, error) {
	reopener, ok := uc.convRepo.(domain.ConversationReopener)
	if !ok {
		return nil, nil
	}
	latest, err := reopener.FindLatestByContact(ctx, channelID, contactID)
	if err != nil {
		return nil, fmt.Errorf("error looking up closed conversation: %w", err)
	}
	if latest == nil || latest.Status != domain.ConversationClosed {
		return nil, nil
	}
	reopened, err := reopener.Reopen(ctx, latest.ID, latest.AccountID)
	if err != nil {
		if err == domain.ErrConversationAlreadyOpen {
			// A parallel inbound opened a thread between FindByContact and
			// here; the caller's race-recovery path re-reads it.
			return nil, nil
		}
		return nil, fmt.Errorf("failed to reopen conversation: %w", err)
	}
	if !reopened {
		return nil, nil
	}
	now := time.Now()
	latest.Status = domain.ConversationOpen
	latest.ResolvedAt = nil
	latest.WaitingSince = &now
	slog.Info("reopened resolved conversation on customer reply",
		"conversation_id", latest.ID, "channel_id", channelID)
	return latest, nil
}

package usecases

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// The three soldering points the 2026-09-01 analysis found missing between an
// approved approach and the rest of the product:
//
//  1. The channel: a funnel with no channel_id used to refuse the send
//     (LEAD_FUNNEL_NO_CHANNEL) — and in production 7 of 7 default funnels had
//     none. resolveSendChannel falls back to the account's active email channel.
//  2. Atomicity: two tabs (or the schedule worker and a person) could both read
//     pending_approval, both send, and the hospital got the offer twice.
//     claimForSend is a compare-and-set on the status.
//  3. The record: the sent offer was never written as a message, so the timeline
//     showed the reply but not the offer, a bounce could not find what bounced,
//     and the template was never credited. recordOutbound writes it.

// leadSendClaimer is the optional atomic claim the postgres lead repository
// implements. Type-asserted rather than added to domain.LeadRepository so the
// dozens of test mocks keep compiling; a repository without it falls back to
// the read-then-write check that existed before (documented, not silent).
type leadSendClaimer interface {
	ClaimForSend(ctx context.Context, leadID string) (bool, error)
}

// SetOutboundRecorder wires the conversation + message repositories so an
// approved send is persisted as an outbound message on the contact's
// conversation (creating the conversation when this is the first touch).
// Optional: without it the send still goes out — but nothing downstream
// (timeline, bounce matching, template credit) can see it, which is the bug
// this fixes, so main.go wires it.
func (uc *ApproachUseCase) SetOutboundRecorder(convRepo domain.ConversationRepository, msgRepo domain.MessageRepository) {
	uc.convRepo = convRepo
	uc.msgRepo = msgRepo
}

// resolveSendChannel answers "which channel does this lead's approach go
// through". The funnel's own channel when it has one; otherwise the account's
// active email channel (see ActiveEmailChannel for why that is the right
// default). A nil answer means the account has no way to send at all.
func (uc *ApproachUseCase) resolveSendChannel(ctx context.Context, accountID string, funnel *domain.SalesFunnel) (*domain.Channel, error) {
	if funnel != nil && funnel.ChannelID != nil && *funnel.ChannelID != "" {
		channel, err := uc.channelRepo.GetByID(ctx, *funnel.ChannelID)
		if err != nil {
			return nil, err
		}
		if channel == nil || channel.AccountID != accountID {
			return nil, domain.ErrChannelNotFound
		}
		return channel, nil
	}
	channel, err := ActiveEmailChannel(ctx, uc.channelRepo, accountID)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, domain.ErrLeadFunnelNoChannel
	}
	return channel, nil
}

// claimForSend moves the lead pending_approval → contacted BEFORE the provider
// call, atomically. A second approver (or the schedule worker racing a person)
// finds no pending row and gets LEAD_NOT_PENDING_APPROVAL instead of sending a
// second email. On a send failure the caller releases the claim so the draft
// returns to the queue.
//
// The trade-off is deliberate: a crash between the claim and the send leaves a
// lead marked contacted that was not — visible, and fixable by hand — whereas
// the old order could write to a hospital twice, which is not.
func (uc *ApproachUseCase) claimForSend(ctx context.Context, lead *domain.Lead) error {
	claimer, ok := uc.leadRepo.(leadSendClaimer)
	if !ok {
		// Non-atomic fallback (unit tests, unwired repos): the status was
		// already checked by the caller.
		return nil
	}
	claimed, err := claimer.ClaimForSend(ctx, lead.ID)
	if err != nil {
		return err
	}
	if !claimed {
		return domain.ErrLeadNotPendingApproval
	}
	return nil
}

// releaseClaim puts the lead back in the queue after a failed send. Best
// effort: the send already failed, and a second error here must not mask it.
func (uc *ApproachUseCase) releaseClaim(ctx context.Context, lead *domain.Lead) {
	if _, ok := uc.leadRepo.(leadSendClaimer); !ok {
		return
	}
	if err := uc.leadRepo.UpdateStatus(ctx, lead.ID, domain.LeadStatusPendingApproval); err != nil {
		slog.WarnContext(ctx, "approach: send failed and the lead could not be returned to the queue",
			"lead_id", lead.ID, "error", err)
	}
}

// recordOutbound persists the approved send as an outbound message on the
// contact's conversation, creating the conversation on a first touch.
//
// `to` is the address exactly as the channel wrote it (a lowercased email, or
// bare digits on WhatsApp) — the same key SaveInbound uses for
// conversations.contact_id, so the hospital's reply lands on THIS row.
// externalID is the provider's id (our own Message-ID on email, the wamid on
// WhatsApp): it is what a bounce report and a reply's In-Reply-To point at.
//
// Best effort: the message already went out, and a bookkeeping failure must
// surface as a log line, never as a failed approval.
func (uc *ApproachUseCase) recordOutbound(
	ctx context.Context,
	accountID string,
	channel *domain.Channel,
	to string,
	lead *domain.Lead,
	sent *domain.Message,
	externalID string,
	actorUserID string,
) {
	if uc.convRepo == nil || uc.msgRepo == nil || channel == nil || lead == nil || sent == nil {
		return
	}
	to = strings.TrimSpace(to)
	if channel.Type != "email" {
		to = strings.TrimPrefix(to, "+")
	} else {
		to = strings.ToLower(to)
	}
	if to == "" {
		return
	}

	conv, err := uc.convRepo.FindByContact(ctx, channel.ID, to)
	if err != nil {
		slog.WarnContext(ctx, "approach: outbound not recorded — conversation lookup failed",
			"lead_id", lead.ID, "error", err)
		return
	}
	now := uc.now()
	if conv == nil {
		var contactRef *string
		if contact := uc.leadContact(ctx, accountID, lead); contact != nil {
			id := contact.ID
			contactRef = &id
		}
		conv = &domain.Conversation{
			ID:           ulid.Make().String(),
			AccountID:    accountID,
			ChannelID:    channel.ID,
			AgentID:      channel.AgentID,
			ContactID:    to,
			ContactRefID: contactRef,
			Status:       domain.ConversationOpen,
			// An outbound-first thread is a human's: the bot must never pick
			// up a hospital's reply to an offer a person approved.
			IsBotPaused: true,
		}
		if err := uc.convRepo.Create(ctx, conv); err != nil {
			if !errors.Is(err, domain.ErrConversationAlreadyOpen) {
				slog.WarnContext(ctx, "approach: outbound not recorded — conversation create failed",
					"lead_id", lead.ID, "error", err)
				return
			}
			existing, ferr := uc.convRepo.FindByContact(ctx, channel.ID, to)
			if ferr != nil || existing == nil {
				return
			}
			conv = existing
		}
	}

	sender := strings.TrimSpace(actorUserID)
	if sender == "" && lead.OwnerUserID != nil {
		sender = *lead.OwnerUserID
	}
	if sender == "" {
		// messages.sender semantics: anything that is not 'bot'/'customer'
		// reads as a human. An approval IS a human decision even when the
		// approver's id did not travel (scheduled replay).
		sender = "approver"
	}
	convID := conv.ID
	content := ""
	if sent.Content != nil {
		content = *sent.Content
	}
	row := &domain.Message{
		ID:             ulid.Make().String(),
		ChannelID:      channel.ID,
		ConversationID: &convID,
		From:           sender,
		Type:           "text",
		Content:        &content,
		Direction:      "outbound",
		Metadata:       sent.Metadata,
		Timestamp:      now,
	}
	if id := strings.TrimSpace(externalID); id != "" {
		row.ExternalMessageID = &id
	}
	if _, err := uc.msgRepo.Create(ctx, row); err != nil {
		slog.WarnContext(ctx, "approach: outbound not recorded — message create failed",
			"lead_id", lead.ID, "conversation_id", convID, "error", err)
	}
}

// approachSender resolves who approved, for the message row. Empty when the
// caller did not say (MCP without an actor, scheduled replay).
func approachSender(in ApproachSendInput) string { return strings.TrimSpace(in.ActorUserID) }

// ensure the clock is never nil for callers constructed outside NewApproachUseCase.
func (uc *ApproachUseCase) clockNow() time.Time {
	if uc.now == nil {
		return time.Now()
	}
	return uc.now()
}

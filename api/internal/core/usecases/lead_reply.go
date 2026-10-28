package usecases

import (
	"context"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// Answering the customer from the lead.
//
// The timeline (LeadTimelineUseCase) let a vendedor READ the reply to their
// proposal. Answering it lived only under /conversations/*, which is
// admin/attendant — so she could see "yes, send the quote" and had no place to
// say anything back. This is that place: ONE lead, gated by the lead's own
// ownership rule (MutableForActor — hers or unowned), and the send itself goes
// through the same human-message path the inbox uses, so threading, delivery
// failure marking and the dashboard broadcast are one implementation.
//
// It is NOT a second outbound-approach path: it only works on a thread that
// already exists, i.e. after the first approach was approved and sent, or the
// customer wrote in. A lead nobody has exchanged a message with answers
// LEAD_NO_CONVERSATION.
type LeadReplyUseCase struct {
	leads  *LeadUseCase
	convs  leadReplyConversations
	sender humanReplier
}

// leadReplyConversations is the slice of domain.ConversationRepository this
// needs: which thread the contact is in, and its current state.
type leadReplyConversations interface {
	FindByAccountContact(ctx context.Context, accountID, contactID string) (*domain.Conversation, error)
	GetByID(ctx context.Context, id, accountID string) (*domain.Conversation, error)
}

// humanReplier is the slice of ConversationTakeoverUseCase this needs. A
// narrow port so the use case is testable without the whole takeover wiring
// and never depends on that file's internals.
type humanReplier interface {
	Assume(ctx context.Context, conversationID, accountID, userID string) (*AssumeResult, error)
	SendHumanMessage(ctx context.Context, dto SendHumanMessageDTO) (*domain.Message, error)
}

func NewLeadReplyUseCase(leads *LeadUseCase, convs leadReplyConversations, sender humanReplier) *LeadReplyUseCase {
	return &LeadReplyUseCase{leads: leads, convs: convs, sender: sender}
}

// LeadReplyDTO is what the composer sends.
type LeadReplyDTO struct {
	LeadID string
	Text   string
	// Subject rides only on e-mail threads (empty → the server's "Re:" rule).
	Subject string
	// UserName is the sender label the customer and the timeline see.
	UserName string
}

// Execute sends text to the lead's contact on the thread they already share.
//
// The bot is paused on that thread first (Assume) when it is not already: the
// human-send path refuses a live-bot thread so a person and the AI never both
// answer, and a vendedor replying IS the person taking the thread. Idempotent
// on an already-paused thread, so a colleague who assumed it keeps it — Assume
// is only called when nobody has.
func (uc *LeadReplyUseCase) Execute(ctx context.Context, actor domain.Actor, dto LeadReplyDTO) (*domain.Message, error) {
	text := strings.TrimSpace(dto.Text)
	if text == "" {
		return nil, domain.ErrLeadReplyTextRequired
	}
	lead, err := uc.leads.MutableForActor(ctx, actor, dto.LeadID)
	if err != nil {
		return nil, err
	}
	if lead.ContactID == "" {
		return nil, domain.ErrLeadNoConversation
	}
	found, err := uc.convs.FindByAccountContact(ctx, actor.AccountID, lead.ContactID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, domain.ErrLeadNoConversation
	}
	// The finder answers with the id only; the paused flag needs the row.
	conv, err := uc.convs.GetByID(ctx, found.ID, actor.AccountID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, domain.ErrLeadNoConversation
	}
	if !conv.IsBotPaused {
		if _, err := uc.sender.Assume(ctx, conv.ID, actor.AccountID, actor.UserID); err != nil {
			return nil, err
		}
	}
	return uc.sender.SendHumanMessage(ctx, SendHumanMessageDTO{
		ConversationID: conv.ID,
		AccountID:      actor.AccountID,
		UserID:         actor.UserID,
		UserName:       dto.UserName,
		Content:        text,
		Subject:        dto.Subject,
	})
}

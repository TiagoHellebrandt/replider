package usecases

import (
	"context"
	"log/slog"

	"github.com/replider/api/internal/core/domain"
)

// Inbox triage (§6.10, camada 3).
//
// Runs after an inbound message is persisted and files its conversation into
// a category. Everything it needs is already known by then — the risk verdict
// from camada 1/2, the lead's pipeline stage, whether this contact has ever
// bought — so the common cases cost one write and no model call.
type InboxTriageUseCase struct {
	writer InboxCategoryWriter

	// All optional. Each missing dependency only removes the signal it
	// carries; the classifier degrades to a coarser answer rather than
	// refusing to file the conversation at all.
	leadRepo   domain.LeadRepository
	funnelRepo domain.SalesFunnelRepository
	saleRepo   InboxSaleLookup
}

// InboxCategoryWriter stamps the verdict. A narrow port because
// ConversationRepository has dozens of test mocks.
type InboxCategoryWriter interface {
	SetInboxCategory(ctx context.Context, conversationID, category string, importance int) error
}

// InboxSaleLookup answers the single question that separates a customer from
// a prospect: has this contact ever bought?
type InboxSaleLookup interface {
	HasSale(ctx context.Context, accountID, contactID string) (bool, error)
}

func NewInboxTriageUseCase(w InboxCategoryWriter) *InboxTriageUseCase {
	return &InboxTriageUseCase{writer: w}
}

func (uc *InboxTriageUseCase) SetSalesLead(f domain.SalesFunnelRepository, l domain.LeadRepository) {
	uc.funnelRepo, uc.leadRepo = f, l
}

func (uc *InboxTriageUseCase) SetSaleLookup(s InboxSaleLookup) { uc.saleRepo = s }

// TriageInput is what the caller already has in hand after saving a message.
type TriageInput struct {
	AccountID      string
	ConversationID string
	ContactID      string
	ChannelID      string
	Subject        string
	Body           string
	// Quarantined is the risk engine's verdict for this message.
	Quarantined bool
	// HasPaymentInstruction says the body carries bank details, a boleto
	// line or a PIX key.
	HasPaymentInstruction bool
	ListUnsubscribe       bool
}

// Classify files one conversation.
//
// Best effort in every direction: this runs after the message is already
// persisted and delivered, so a failure here costs a sort order, never a
// message.
func (uc *InboxTriageUseCase) Classify(ctx context.Context, in TriageInput) {
	if uc.writer == nil || in.ConversationID == "" {
		return
	}

	signals := domain.InboxSignals{
		Quarantined:           in.Quarantined,
		HasPaymentInstruction: in.HasPaymentInstruction,
		ListUnsubscribe:       in.ListUnsubscribe,
		Subject:               in.Subject,
		Body:                  in.Body,
		// This only ever runs on inbound. Our own outbound does not make a
		// lead hot, and saying otherwise would put every cold email we sent
		// at the top of our own inbox.
		LastWasInbound: true,
	}
	signals.LeadStatus = uc.leadStatus(ctx, in)
	signals.HasSale = uc.hasSale(ctx, in)

	category, _ := domain.ClassifyInbox(signals)
	// The `ok=false` case deliberately still writes "other". A conversation
	// with no category at all sorts at zero and disappears under everything
	// — worse than a coarse label. An LLM pass can refine it later.
	importance := domain.InboxImportance(category, signals)

	if err := uc.writer.SetInboxCategory(ctx, in.ConversationID, string(category), importance); err != nil {
		slog.WarnContext(ctx, "could not file the conversation in the inbox",
			"conversation_id", in.ConversationID, "error", err)
	}
}

func (uc *InboxTriageUseCase) leadStatus(ctx context.Context, in TriageInput) domain.LeadStatus {
	if uc.funnelRepo == nil || uc.leadRepo == nil || in.ChannelID == "" {
		return ""
	}
	funnel, err := uc.funnelRepo.GetByChannelID(ctx, in.ChannelID)
	if err != nil || funnel == nil {
		return ""
	}
	lead, err := uc.leadRepo.GetByFunnelAndContact(ctx, funnel.ID, in.ContactID)
	if err != nil || lead == nil {
		return ""
	}
	return lead.Status
}

func (uc *InboxTriageUseCase) hasSale(ctx context.Context, in TriageInput) bool {
	if uc.saleRepo == nil {
		return false
	}
	// A lookup failure means "unknown", and unknown must not read as "never
	// bought" loudly — it only costs this conversation the customer label,
	// which the next inbound will fix.
	has, err := uc.saleRepo.HasSale(ctx, in.AccountID, in.ContactID)
	if err != nil {
		return false
	}
	return has
}

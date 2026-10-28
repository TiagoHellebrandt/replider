package usecases

import (
	"context"
	"log/slog"
	"strings"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// What happens when a message does not arrive (§6.8.4).
//
// Three things, in descending order of how much they matter:
//
//  1. STOP WRITING to a permanently dead address. This is the one that
//     protects the customer's sending domain — repeated sends to addresses
//     that do not exist is precisely what mailbox providers throttle for.
//  2. Count it against the template, so a list full of dead addresses shows
//     up as a bounce rate instead of as a mysteriously low reply rate.
//  3. Move the lead out of the pipeline. A lead whose only address bounces
//     is not "aguardando resposta"; nobody is ever going to answer.
//
// Everything here is best effort and nothing returns an error to the
// provider: a webhook that 500s gets retried forever, and a retry cannot fix
// a missing lead row.
type EmailBounceUseCase struct {
	suppressions domain.EmailSuppressionRepository
	msgRepo      domain.MessageRepository

	// lastOutbound answers "what did we last send to this address". A narrow
	// port because paging the ordinary message list cannot do it: that query
	// is ORDER BY created_at ASC, so its first page is the OLDEST messages
	// in the thread and a bounce is always about the newest.
	lastOutbound LastOutboundLookup

	// Optional.
	templateStats   domain.EmailTemplateRepository
	salesFunnelRepo domain.SalesFunnelRepository
	leadRepo        domain.LeadRepository
	// contactRepo turns the bounced ADDRESS into the contact the lead is
	// keyed on. Without it the lead lookup runs on the raw address, which
	// matches only rows written before the identity migration.
	contactRepo domain.ContactRepository
}

// LastOutboundLookup finds the most recent outbound message to a contact.
type LastOutboundLookup interface {
	LastOutboundToContact(ctx context.Context, channelID, contactID string) (*domain.Message, error)
}

func NewEmailBounceUseCase(
	suppressions domain.EmailSuppressionRepository,
	msgRepo domain.MessageRepository,
	lastOutbound LastOutboundLookup,
) *EmailBounceUseCase {
	return &EmailBounceUseCase{
		suppressions: suppressions, msgRepo: msgRepo, lastOutbound: lastOutbound,
	}
}

// SetTemplateStats wires the per-version counters.
func (uc *EmailBounceUseCase) SetTemplateStats(r domain.EmailTemplateRepository) {
	uc.templateStats = r
}

// SetSalesLead wires the pipeline so a dead address stops occupying a stage.
func (uc *EmailBounceUseCase) SetSalesLead(f domain.SalesFunnelRepository, l domain.LeadRepository) {
	uc.salesFunnelRepo, uc.leadRepo = f, l
}

// SetContactRepo wires the address → contact resolution the lead lookup
// needs (leads.contact_id is a contact ULID, not an email).
func (uc *EmailBounceUseCase) SetContactRepo(r domain.ContactRepository) {
	uc.contactRepo = r
}

// Record handles one delivery failure.
func (uc *EmailBounceUseCase) Record(ctx context.Context, channel *domain.Channel, b domain.EmailBounce) {
	if channel == nil || b.Recipient == "" {
		return
	}
	log := slog.With("channel_id", channel.ID, "kind", string(b.Kind))

	// A SOFT bounce is a bad day, not a bad address: a full mailbox is
	// emptied, a greylisted server accepts the retry. Suppressing on one
	// would silently delete a real customer from the list.
	if b.Kind.Permanent() {
		uc.suppress(ctx, channel.AccountID, b, log)
		uc.markLeadLost(ctx, channel, b.Recipient, log)
	}

	// Both of the remaining jobs need the message that failed, and finding
	// it is the expensive part — resolve once.
	msg := uc.failedMessage(ctx, channel, b)
	uc.creditTemplate(ctx, msg, b, log)
	uc.markUndelivered(ctx, msg, b, log)
}

// markUndelivered flips the sent message to "não entregue" in the thread.
//
// Reuses the existing failed_at/failed_reason stamp rather than inventing a
// bounce marker message: the dashboard already renders that state, and a
// salesperson staring at a thread waiting for a reply needs the bubble they
// are looking at to say it never arrived — not a new row below it.
func (uc *EmailBounceUseCase) markUndelivered(ctx context.Context, msg *domain.Message, b domain.EmailBounce, log *slog.Logger) {
	if uc.msgRepo == nil || msg == nil {
		return
	}
	reason := strings.TrimSpace(b.Diagnostic)
	if reason == "" {
		// Never blank: an empty tooltip on a red marker is worse than a
		// generic one, because it reads as a UI bug rather than an answer.
		reason = string(b.Kind) + " bounce"
	}
	if err := uc.msgRepo.MarkSendFailed(ctx, msg.ID, reason); err != nil {
		log.WarnContext(ctx, "could not mark the bounced message as undelivered",
			"message_id", msg.ID, "error", err)
	}
}

func (uc *EmailBounceUseCase) suppress(ctx context.Context, accountID string, b domain.EmailBounce, log *slog.Logger) {
	if uc.suppressions == nil {
		return
	}
	reason := domain.SuppressionHardBounce
	if b.Kind == domain.BounceComplaint {
		reason = domain.SuppressionComplaint
	}
	err := uc.suppressions.Suppress(ctx, &domain.EmailSuppression{
		ID:         ulid.Make().String(),
		AccountID:  accountID,
		Address:    strings.ToLower(strings.TrimSpace(b.Recipient)),
		Reason:     reason,
		Diagnostic: b.Diagnostic,
	})
	if err != nil {
		// Loud: this is the one failure that lets the send keep happening.
		log.ErrorContext(ctx, "could not suppress a bounced address — sends to it will continue",
			"error", err)
		return
	}
	log.InfoContext(ctx, "address suppressed after a permanent failure",
		"reason", string(reason))
}

// creditTemplate counts the bounce against the template that was sent.
//
// Finding WHICH message failed is the hard part, and the two paths are
// deliberately different in confidence:
//
//   - The report named the Message-ID. Exact.
//   - It did not, which is the common case. Fall back to the most recent
//     outbound message to that recipient on this channel — a bounce follows
//     its send by seconds to hours, so the latest one is the right one
//     essentially always, and being wrong costs one miscounted bounce.
func (uc *EmailBounceUseCase) creditTemplate(ctx context.Context, msg *domain.Message, b domain.EmailBounce, log *slog.Logger) {
	if uc.templateStats == nil || msg == nil {
		return
	}
	templateID, version := templateFromMetadata(msg.Metadata)
	if templateID == "" {
		return
	}
	if err := uc.templateStats.RecordBounce(ctx, templateID, version); err != nil {
		log.WarnContext(ctx, "could not count the bounce against its template",
			"template_id", templateID, "error", err)
	}
}

func (uc *EmailBounceUseCase) failedMessage(ctx context.Context, channel *domain.Channel, b domain.EmailBounce) *domain.Message {
	if id := strings.TrimSpace(b.OriginalMessageID); id != "" && uc.msgRepo != nil {
		if msg, err := uc.msgRepo.GetByExternalID(ctx, channel.ID, id); err == nil && msg != nil {
			return msg
		}
	}
	if uc.lastOutbound == nil {
		return nil
	}
	msg, err := uc.lastOutbound.LastOutboundToContact(ctx, channel.ID, b.Recipient)
	if err != nil {
		return nil
	}
	return msg
}

// markLeadLost takes a dead address out of the pipeline.
//
// Only for a permanent failure, and never over a lead that already reached a
// terminal state: a customer who bought and whose address later died is
// still won.
func (uc *EmailBounceUseCase) markLeadLost(ctx context.Context, channel *domain.Channel, recipient string, log *slog.Logger) {
	if uc.salesFunnelRepo == nil || uc.leadRepo == nil {
		return
	}
	funnel, err := funnelForChannel(ctx, uc.salesFunnelRepo, channel)
	if err != nil || funnel == nil {
		return
	}
	// The lead is keyed by the CONTACT, so the address has to become one
	// first. The raw address is kept as a second try for rows written before
	// the identity migration, when contact_id still held the email.
	var lead *domain.Lead
	if uc.contactRepo != nil {
		address := strings.ToLower(strings.TrimSpace(recipient))
		if contact, err := uc.contactRepo.GetByExternalID(ctx, channel.AccountID, address); err == nil && contact != nil {
			lead, err = uc.leadRepo.GetByFunnelAndContact(ctx, funnel.ID, contact.ID)
			if err != nil {
				log.WarnContext(ctx, "bounce: lead lookup failed", "funnel_id", funnel.ID, "error", err)
				return
			}
		}
	}
	if lead == nil {
		lead, err = uc.leadRepo.GetByFunnelAndContact(ctx, funnel.ID, recipient)
		if err != nil || lead == nil {
			return
		}
	}
	if lead.Status == domain.LeadStatusLost || lead.Status == domain.LeadStatusWon {
		return
	}
	if err := uc.leadRepo.UpdateStatus(ctx, lead.ID, domain.LeadStatusLost); err != nil {
		log.WarnContext(ctx, "bounce: could not move the lead out of the pipeline",
			"lead_id", lead.ID, "error", err)
	}
}

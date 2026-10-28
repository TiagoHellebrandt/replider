package usecases

import (
	"context"
	"strings"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
)

// Opening a renewal in the pipeline (§6.11, G6).
//
// The worker knows about dates and money; this knows about funnels, contacts and
// phone numbers. Keeping them apart is what lets the worker fire the DEADLINE
// even on an account with no funnel configured — a contract about to renew itself
// is worth saying out loud whether or not there is a pipeline to put it in.

// ContractRenewalLeadOpener turns a contract deadline into a lead on the funnel.
type ContractRenewalLeadOpener struct {
	leads    *LeadUseCase
	contacts domain.ContactRepository
	funnels  domain.SalesFunnelRepository
}

func NewContractRenewalLeadOpener(
	leads *LeadUseCase, contacts domain.ContactRepository, funnels domain.SalesFunnelRepository,
) *ContractRenewalLeadOpener {
	return &ContractRenewalLeadOpener{leads: leads, contacts: contacts, funnels: funnels}
}

// OpenRenewalLead files the renewal and returns the lead id.
//
// Refuses rather than improvises. Three things can be missing — the contact, the
// contact's phone, an active funnel — and each one produces an error the worker
// logs and moves on from, leaving the ALERT in place. A lead with no counterpart,
// or on a funnel nobody uses, is a row somebody deletes and stops trusting.
func (o *ContractRenewalLeadOpener) OpenRenewalLead(
	ctx context.Context, in services.RenewalLeadRequest,
) (string, error) {
	if o.leads == nil || o.contacts == nil || o.funnels == nil {
		return "", &domain.AppError{
			Code:    "RENEWAL_LEAD_UNAVAILABLE",
			Message: "criação de lead de renovação não configurada",
		}
	}

	contact, err := o.contacts.GetByID(ctx, in.ContactID, in.AccountID)
	if err != nil {
		return "", err
	}
	if contact == nil {
		return "", &domain.AppError{
			Code: "RENEWAL_LEAD_NO_CONTACT", Message: "contrato sem cliente vinculado"}
	}
	// The lead is keyed by phone, the way every other lead in this product is.
	// A contact with only an email cannot become one yet, and saying so beats
	// inventing a placeholder number that the approach flow would then try to
	// message.
	phone := strings.TrimSpace(contact.ExternalID)
	if contact.Phone != nil && strings.TrimSpace(*contact.Phone) != "" {
		phone = strings.TrimSpace(*contact.Phone)
	}
	if _, ok := domain.NormalizeTypedPhone(phone); !ok {
		return "", &domain.AppError{
			Code:    "RENEWAL_LEAD_NO_PHONE",
			Message: "o cliente deste contrato não tem telefone para abrir o lead",
		}
	}

	funnel, err := o.renewalFunnel(ctx, in.AccountID)
	if err != nil {
		return "", err
	}

	name := ""
	if contact.Name != nil {
		name = strings.TrimSpace(*contact.Name)
	}
	if name == "" {
		name = in.Title
	}

	lead, _, err := o.leads.Create(ctx, CreateLeadDTO{
		AccountID: in.AccountID,
		FunnelID:  funnel.ID,
		Name:      name,
		Phone:     phone,
		// The briefing goes in the metadata ONLY when the funnel declares a field
		// for it: unknown keys are rejected by design, and losing the lead over a
		// briefing would trade the whole feature for a note.
		Metadata: o.briefingMetadata(funnel, in.Context),
	})
	if err != nil {
		return "", err
	}
	return lead.ID, nil
}

// renewalFunnel picks where the renewal lands.
//
// The account's first ACTIVE funnel. Not a dedicated "renewals" funnel created
// on the fly: a funnel is a sales process somebody designed, and inventing one
// means the renewal shows up in a pipeline nobody looks at.
func (o *ContractRenewalLeadOpener) renewalFunnel(
	ctx context.Context, accountID string,
) (*domain.SalesFunnel, error) {
	funnels, err := o.funnels.ListByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	for _, f := range funnels {
		if f != nil && f.Active {
			return f, nil
		}
	}
	return nil, &domain.AppError{
		Code:    "RENEWAL_LEAD_NO_FUNNEL",
		Message: "nenhum funil ativo para receber o lead de renovação",
	}
}

// briefingMetadata puts the briefing on the lead when the funnel has somewhere
// to hold it.
func (o *ContractRenewalLeadOpener) briefingMetadata(
	funnel *domain.SalesFunnel, briefing string,
) map[string]string {
	briefing = strings.TrimSpace(briefing)
	if briefing == "" || funnel == nil {
		return nil
	}
	// Any of the keys a real funnel plausibly declares for free-text context.
	// Checked against the DECLARED schema rather than guessed, because an
	// undeclared key makes Create reject the whole lead — and losing the renewal
	// over a note would trade the feature for its footnote.
	fields, err := funnel.MetadataFields()
	if err != nil {
		return nil
	}
	for _, key := range []string{"contexto", "observacoes", "observações", "notes", "briefing"} {
		for _, field := range fields {
			if strings.EqualFold(field.Key, key) {
				return map[string]string{field.Key: briefing}
			}
		}
	}
	return nil
}

var _ services.RenewalLeadOpener = (*ContractRenewalLeadOpener)(nil)

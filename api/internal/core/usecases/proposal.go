package usecases

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/replider/api/internal/core/domain"
)

// ProposalDraftWriter is how a proposal reaches the approval queue.
//
// Narrow on purpose: satisfied by ApproachUseCase.SaveDraft, which already owns
// the queue's rules (which statuses may be drafted onto, the status transition,
// the ownership check). A proposal that wrote `leads.approach_draft` itself would
// be a second implementation of those rules.
type ProposalDraftWriter interface {
	SaveDraft(ctx context.Context, accountID, leadID, text string) (*domain.Lead, error)
}

// ProposalUseCase composes priced offers.
//
// It deliberately cannot send anything. The body it renders goes into the lead's
// approach draft, so the human approval and the delivery both run through the
// paths that already carry the opt-out check, the spend budget and the case
// record — see the note on domain.Proposal.
type ProposalUseCase struct {
	proposals domain.ProposalRepository
	products  domain.ProductRepository
	leads     domain.LeadRepository
	drafts    ProposalDraftWriter
}

func NewProposalUseCase(
	proposals domain.ProposalRepository,
	products domain.ProductRepository,
	leads domain.LeadRepository,
) *ProposalUseCase {
	return &ProposalUseCase{proposals: proposals, products: products, leads: leads}
}

// SetDraftWriter wires the approval queue. Optional-setter rather than a
// constructor argument so main.go's existing chain and every test call site stay
// as they are; unwired, a proposal can still be built and priced — it just
// cannot be pushed for approval.
func (uc *ProposalUseCase) SetDraftWriter(w ProposalDraftWriter) { uc.drafts = w }

// CreateProposalInput is one offer to one lead.
type CreateProposalInput struct {
	LeadID string                `json:"lead_id"`
	Lines  []domain.ProposalLine `json:"lines"`
	// Body is the salesperson's own words. Optional: the rendered item block is
	// appended to whatever is here, so an empty body still produces a readable
	// offer rather than an empty message.
	Body string `json:"body"`
}

// Create prices the lines against the catalogue and stores the offer as a draft.
func (uc *ProposalUseCase) Create(ctx context.Context, accountID string, in CreateProposalInput) (*domain.Proposal, error) {
	lead, err := uc.ownedLead(ctx, accountID, in.LeadID)
	if err != nil {
		return nil, err
	}

	catalogue, err := uc.catalogue(ctx, accountID)
	if err != nil {
		return nil, err
	}

	items, currency, err := domain.BuildItems(in.Lines, catalogue)
	if err != nil {
		return nil, err
	}

	p := &domain.Proposal{
		ID:        ulid.Make().String(),
		AccountID: accountID,
		LeadID:    lead.ID,
		Status:    domain.ProposalDraft,
		Currency:  currency,
		Items:     items,
		Body:      strings.TrimSpace(in.Body),
	}
	if err := uc.proposals.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Get returns one offer with its lines.
func (uc *ProposalUseCase) Get(ctx context.Context, accountID, id string) (*domain.Proposal, error) {
	return uc.proposals.GetByID(ctx, accountID, id)
}

// ListForLead is the history: what this customer has already been offered.
func (uc *ProposalUseCase) ListForLead(ctx context.Context, accountID, leadID string) ([]*domain.Proposal, error) {
	if _, err := uc.ownedLead(ctx, accountID, leadID); err != nil {
		return nil, err
	}
	return uc.proposals.ListByLead(ctx, accountID, leadID)
}

// ReplaceLines re-prices the offer from the catalogue as it stands NOW.
//
// Editing an offer is making a new offer: the customer has not seen this one yet,
// so today's catalogue is the honest source. What must not change is a line
// already sent, which the repository refuses.
func (uc *ProposalUseCase) ReplaceLines(ctx context.Context, accountID, id string, lines []domain.ProposalLine) (*domain.Proposal, error) {
	if _, err := uc.proposals.GetByID(ctx, accountID, id); err != nil {
		return nil, err
	}

	catalogue, err := uc.catalogue(ctx, accountID)
	if err != nil {
		return nil, err
	}
	items, currency, err := domain.BuildItems(lines, catalogue)
	if err != nil {
		return nil, err
	}

	if err := uc.proposals.ReplaceItems(ctx, accountID, id, items, currency); err != nil {
		return nil, err
	}
	return uc.proposals.GetByID(ctx, accountID, id)
}

// SetBody stores the salesperson's text.
func (uc *ProposalUseCase) SetBody(ctx context.Context, accountID, id, body string) (*domain.Proposal, error) {
	if err := uc.proposals.UpdateBody(ctx, accountID, id, strings.TrimSpace(body)); err != nil {
		return nil, err
	}
	return uc.proposals.GetByID(ctx, accountID, id)
}

// SendForApproval renders the offer and puts it in the approval queue.
//
// Nothing leaves here. The queue is where a person reads it, edits it and
// decides — the same gate every cold approach passes through.
func (uc *ProposalUseCase) SendForApproval(ctx context.Context, accountID, id string) (*domain.Proposal, error) {
	p, err := uc.proposals.GetByID(ctx, accountID, id)
	if err != nil {
		return nil, err
	}
	if !p.Editable() {
		return nil, domain.ErrProposalNotEditable
	}
	if uc.drafts == nil {
		return nil, domain.ErrProposalDraftUnavailable
	}
	if _, err := uc.drafts.SaveDraft(ctx, accountID, p.LeadID, RenderProposal(p)); err != nil {
		return nil, err
	}
	return p, nil
}

// MarkSent stamps the offer as delivered. Called after the approach carrying it
// actually went out — never as a shortcut for sending.
func (uc *ProposalUseCase) MarkSent(ctx context.Context, accountID, id string, at time.Time) error {
	return uc.proposals.MarkSent(ctx, accountID, id, at)
}

func (uc *ProposalUseCase) ownedLead(ctx context.Context, accountID, leadID string) (*domain.Lead, error) {
	lead, err := uc.leads.GetByID(ctx, leadID)
	if err != nil {
		return nil, err
	}
	// A nil lead and a lead belonging to somebody else get the SAME answer. Two
	// different answers would make this endpoint an oracle for whether an id
	// exists in another account.
	if lead == nil || lead.AccountID != accountID {
		return nil, domain.ErrLeadNotFound
	}
	return lead, nil
}

func (uc *ProposalUseCase) catalogue(ctx context.Context, accountID string) (map[string]*domain.Product, error) {
	products, err := uc.products.ListByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	// Keyed by id and scoped to the account by the query: a product id from
	// another account is simply absent, so BuildItems reports it as not found
	// rather than pricing somebody else's catalogue into this offer.
	out := make(map[string]*domain.Product, len(products))
	for _, p := range products {
		out[p.ID] = p
	}
	return out, nil
}

// RenderProposal turns an offer into the text a customer reads.
//
// Deterministic, not generated: the numbers in a proposal are the one part a
// model must never paraphrase. The salesperson's words go on top, the priced
// lines below, and the total last where a reader looks for it.
func RenderProposal(p *domain.Proposal) string {
	var b strings.Builder

	if body := strings.TrimSpace(p.Body); body != "" {
		b.WriteString(body)
		b.WriteString("\n\n")
	}

	for _, item := range p.Items {
		fmt.Fprintf(&b, "• %d × %s — %s\n",
			item.Quantity, item.Name,
			domain.FormatMoneyCents(item.TotalCents(), p.Currency),
		)
	}

	// Only worth a line of its own when there is more than one to add up.
	if len(p.Items) > 1 {
		fmt.Fprintf(&b, "\nTotal: %s",
			domain.FormatMoneyCents(p.TotalCents(), p.Currency))
	}

	return strings.TrimRight(b.String(), "\n")
}

// ProposalCustomerNamer answers "who is this proposal for", in the words a
// buyer would recognise.
//
// Its own type rather than a method on ProposalUseCase: printing a document
// needs the customer's NAME, and the proposal use case deliberately does not
// hold contacts (the contact is reachable through the lead, and storing it
// twice invites the two to disagree).
type ProposalCustomerNamer struct {
	leads    domain.LeadRepository
	contacts domain.ContactRepository
}

func NewProposalCustomerNamer(leads domain.LeadRepository, contacts domain.ContactRepository) *ProposalCustomerNamer {
	return &ProposalCustomerNamer{leads: leads, contacts: contacts}
}

// NameForProposal returns the company first, the person second, and an empty
// string when neither is known.
//
// Empty is a real answer: a document addressed to "Cliente" reads worse than
// one addressed to nobody, because the salesperson will not notice the
// placeholder before forwarding it.
func (n *ProposalCustomerNamer) NameForProposal(ctx context.Context, accountID, leadID string) string {
	if n == nil || n.leads == nil || n.contacts == nil || leadID == "" {
		return ""
	}
	lead, err := n.leads.GetByID(ctx, leadID)
	if err != nil || lead == nil || lead.AccountID != accountID || lead.ContactID == "" {
		return ""
	}
	contact, err := n.contacts.GetByID(ctx, lead.ContactID, accountID)
	if err != nil || contact == nil {
		// The legacy shape, where the lead's ContactID IS the external id.
		contact, err = n.contacts.GetByExternalID(ctx, accountID, lead.ContactID)
		if err != nil || contact == nil {
			return ""
		}
	}
	if contact.CompanyName != nil && strings.TrimSpace(*contact.CompanyName) != "" {
		return strings.TrimSpace(*contact.CompanyName)
	}
	if contact.Name != nil {
		return strings.TrimSpace(*contact.Name)
	}
	return ""
}

package usecases

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// A proposal is money and it addresses a real customer. These pin the two
// properties that matter most: it prices only from the caller's own catalogue,
// and it cannot send anything by itself.

type fakeProposalRepo struct {
	created  *domain.Proposal
	stored   map[string]*domain.Proposal
	replaced []domain.ProposalItem
	currency string
	body     *string
	sentAt   *time.Time
}

func newFakeProposalRepo() *fakeProposalRepo {
	return &fakeProposalRepo{stored: map[string]*domain.Proposal{}}
}

func (f *fakeProposalRepo) Create(_ context.Context, p *domain.Proposal) error {
	f.created = p
	f.stored[p.ID] = p
	return nil
}

func (f *fakeProposalRepo) GetByID(_ context.Context, accountID, id string) (*domain.Proposal, error) {
	p, ok := f.stored[id]
	if !ok || p.AccountID != accountID {
		return nil, domain.ErrProposalNotFound
	}
	return p, nil
}

func (f *fakeProposalRepo) ListByLead(_ context.Context, accountID, leadID string) ([]*domain.Proposal, error) {
	var out []*domain.Proposal
	for _, p := range f.stored {
		if p.AccountID == accountID && p.LeadID == leadID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeProposalRepo) ReplaceItems(_ context.Context, accountID, id string, items []domain.ProposalItem, currency string) error {
	p, ok := f.stored[id]
	if !ok || p.AccountID != accountID {
		return domain.ErrProposalNotFound
	}
	if !p.Editable() {
		return domain.ErrProposalNotEditable
	}
	f.replaced, f.currency = items, currency
	p.Items, p.Currency = items, currency
	return nil
}

func (f *fakeProposalRepo) UpdateBody(_ context.Context, accountID, id, body string) error {
	p, ok := f.stored[id]
	if !ok || p.AccountID != accountID {
		return domain.ErrProposalNotFound
	}
	if !p.Editable() {
		return domain.ErrProposalNotEditable
	}
	f.body = &body
	p.Body = body
	return nil
}

func (f *fakeProposalRepo) MarkSent(_ context.Context, _, id string, at time.Time) error {
	f.sentAt = &at
	if p, ok := f.stored[id]; ok {
		p.Status = domain.ProposalSent
		p.SentAt = &at
	}
	return nil
}

type fakeProductRepo struct {
	byAccount map[string][]*domain.Product
}

func (f *fakeProductRepo) Create(context.Context, *domain.Product) error { return nil }
func (f *fakeProductRepo) GetByID(context.Context, string) (*domain.Product, error) {
	return nil, domain.ErrProductNotFound
}
func (f *fakeProductRepo) ListByAccountID(_ context.Context, accountID string) ([]*domain.Product, error) {
	return f.byAccount[accountID], nil
}
func (f *fakeProductRepo) Update(context.Context, *domain.Product) error { return nil }
func (f *fakeProductRepo) Delete(context.Context, string) error          { return nil }

type fakeDraftWriter struct {
	calls int
	text  string
	lead  string
}

func (f *fakeDraftWriter) SaveDraft(_ context.Context, _, leadID, text string) (*domain.Lead, error) {
	f.calls++
	f.lead, f.text = leadID, text
	return &domain.Lead{ID: leadID}, nil
}

// leadReader is the narrow slice of domain.LeadRepository a proposal touches.
// The package's testify mockLeadRepo would work too, but a two-line stub keeps
// each test's fixture visible at the call site instead of behind an .On(...).
type leadRepoStub struct {
	domain.LeadRepository
	lead *domain.Lead
}

func (s *leadRepoStub) GetByID(context.Context, string) (*domain.Lead, error) {
	return s.lead, nil
}

const (
	acct  = "acc-1"
	other = "acc-2"
)

func gauze() *domain.Product {
	return &domain.Product{ID: "p1", AccountID: acct, Name: "Gaze estéril", PriceCents: 4990, Currency: "BRL"}
}

func glove() *domain.Product {
	return &domain.Product{ID: "p2", AccountID: acct, Name: "Luva cirúrgica", PriceCents: 12550, Currency: "BRL"}
}

func setup(t *testing.T, products ...*domain.Product) (*ProposalUseCase, *fakeProposalRepo, *fakeDraftWriter) {
	t.Helper()
	repo := newFakeProposalRepo()
	uc := NewProposalUseCase(
		repo,
		&fakeProductRepo{byAccount: map[string][]*domain.Product{acct: products}},
		&leadRepoStub{lead: &domain.Lead{ID: "lead-1", AccountID: acct}},
	)
	writer := &fakeDraftWriter{}
	uc.SetDraftWriter(writer)
	return uc, repo, writer
}

func TestProposalCreate_PricesFromTheCatalogue(t *testing.T) {
	uc, repo, _ := setup(t, gauze(), glove())

	p, err := uc.Create(context.Background(), acct, CreateProposalInput{
		LeadID: "lead-1",
		Lines: []domain.ProposalLine{
			{ProductID: "p1", Quantity: 3},
			{ProductID: "p2", Quantity: 2},
		},
		Body: "  Segue a proposta que conversamos.  ",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.Currency != "BRL" {
		t.Fatalf("currency = %q", p.Currency)
	}
	if got := p.TotalCents(); got != 40070 {
		t.Fatalf("total = %d, want 40070", got)
	}
	if p.Body != "Segue a proposta que conversamos." {
		t.Fatalf("body was not trimmed: %q", p.Body)
	}
	if p.Status != domain.ProposalDraft {
		t.Fatalf("status = %q, want draft", p.Status)
	}
	if repo.created == nil {
		t.Fatal("nothing was persisted")
	}
}

// The catalogue read is account-scoped, so another account's product id is simply
// absent — the offer cannot be priced from somebody else's catalogue.
func TestProposalCreate_RefusesForeignProduct(t *testing.T) {
	repo := newFakeProposalRepo()
	uc := NewProposalUseCase(
		repo,
		&fakeProductRepo{byAccount: map[string][]*domain.Product{
			other: {{ID: "p9", AccountID: other, Name: "Outro", PriceCents: 100, Currency: "BRL"}},
		}},
		&leadRepoStub{lead: &domain.Lead{ID: "lead-1", AccountID: acct}},
	)

	_, err := uc.Create(context.Background(), acct, CreateProposalInput{
		LeadID: "lead-1",
		Lines:  []domain.ProposalLine{{ProductID: "p9", Quantity: 1}},
	})
	if err != domain.ErrProductNotFound {
		t.Fatalf("err = %v, want ErrProductNotFound", err)
	}
	if repo.created != nil {
		t.Fatal("a foreign product produced a stored proposal")
	}
}

// A lead in another account and a lead that does not exist must answer the same,
// or the endpoint reports whether an id exists elsewhere.
func TestProposalCreate_ForeignLeadLooksMissing(t *testing.T) {
	repo := newFakeProposalRepo()
	uc := NewProposalUseCase(
		repo,
		&fakeProductRepo{byAccount: map[string][]*domain.Product{acct: {gauze()}}},
		&leadRepoStub{lead: &domain.Lead{ID: "lead-1", AccountID: other}},
	)

	_, err := uc.Create(context.Background(), acct, CreateProposalInput{
		LeadID: "lead-1",
		Lines:  []domain.ProposalLine{{ProductID: "p1", Quantity: 1}},
	})
	if err != domain.ErrLeadNotFound {
		t.Fatalf("err = %v, want ErrLeadNotFound", err)
	}
}

func TestProposalCreate_RefusesMixedCurrency(t *testing.T) {
	usdSwab := &domain.Product{ID: "p3", AccountID: acct, Name: "Swab", PriceCents: 500, Currency: "USD"}
	uc, repo, _ := setup(t, gauze(), usdSwab)

	_, err := uc.Create(context.Background(), acct, CreateProposalInput{
		LeadID: "lead-1",
		Lines: []domain.ProposalLine{
			{ProductID: "p1", Quantity: 1},
			{ProductID: "p3", Quantity: 1},
		},
	})
	if err != domain.ErrProposalMixedCurrency {
		t.Fatalf("err = %v, want ErrProposalMixedCurrency", err)
	}
	if repo.created != nil {
		t.Fatal("a mixed-currency offer was stored")
	}
}

// The whole point of the object: approval goes through the queue that already
// carries the opt-out check and the budget. Nothing is delivered here.
func TestProposalSendForApproval_WritesTheDraftAndSendsNothing(t *testing.T) {
	uc, _, writer := setup(t, gauze(), glove())

	p, err := uc.Create(context.Background(), acct, CreateProposalInput{
		LeadID: "lead-1",
		Lines: []domain.ProposalLine{
			{ProductID: "p1", Quantity: 3},
			{ProductID: "p2", Quantity: 2},
		},
		Body: "Segue a proposta.",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := uc.SendForApproval(context.Background(), acct, p.ID); err != nil {
		t.Fatalf("SendForApproval: %v", err)
	}

	if writer.calls != 1 || writer.lead != "lead-1" {
		t.Fatalf("draft writer got %d calls for lead %q", writer.calls, writer.lead)
	}
	for _, want := range []string{
		"Segue a proposta.",
		"3 × Gaze estéril — 149,70 BRL",
		"2 × Luva cirúrgica — 251,00 BRL",
		"Total: 400,70 BRL",
	} {
		if !strings.Contains(writer.text, want) {
			t.Fatalf("draft is missing %q:\n%s", want, writer.text)
		}
	}
	// It stays a draft: approval is somebody else's decision.
	if p.Status != domain.ProposalDraft {
		t.Fatalf("status = %q, want draft", p.Status)
	}
	if p.SentAt != nil {
		t.Fatal("SendForApproval stamped a send")
	}
}

// Unwired queue must fail loudly. Reporting success would leave the salesperson
// waiting for an approval that never appears.
func TestProposalSendForApproval_RefusesWithoutTheQueue(t *testing.T) {
	repo := newFakeProposalRepo()
	uc := NewProposalUseCase(
		repo,
		&fakeProductRepo{byAccount: map[string][]*domain.Product{acct: {gauze()}}},
		&leadRepoStub{lead: &domain.Lead{ID: "lead-1", AccountID: acct}},
	)

	p, err := uc.Create(context.Background(), acct, CreateProposalInput{
		LeadID: "lead-1",
		Lines:  []domain.ProposalLine{{ProductID: "p1", Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := uc.SendForApproval(context.Background(), acct, p.ID); err != domain.ErrProposalDraftUnavailable {
		t.Fatalf("err = %v, want ErrProposalDraftUnavailable", err)
	}
}

func TestProposalSendForApproval_RefusesAnAlreadySentOffer(t *testing.T) {
	uc, repo, writer := setup(t, gauze())

	p, err := uc.Create(context.Background(), acct, CreateProposalInput{
		LeadID: "lead-1",
		Lines:  []domain.ProposalLine{{ProductID: "p1", Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := uc.MarkSent(context.Background(), acct, p.ID, time.Now()); err != nil {
		t.Fatalf("MarkSent: %v", err)
	}
	if repo.sentAt == nil {
		t.Fatal("MarkSent did not stamp")
	}

	if _, err := uc.SendForApproval(context.Background(), acct, p.ID); err != domain.ErrProposalNotEditable {
		t.Fatalf("err = %v, want ErrProposalNotEditable", err)
	}
	if writer.calls != 0 {
		t.Fatal("a sent offer was re-queued")
	}
}

// A single line needs no total: repeating the same number twice reads as two
// different numbers that happen to match.
func TestRenderProposal_SingleLineHasNoTotalLine(t *testing.T) {
	text := RenderProposal(&domain.Proposal{
		Currency: "BRL",
		Items:    []domain.ProposalItem{{Name: "Gaze", Quantity: 2, UnitPriceCents: 4990}},
	})
	if strings.Contains(text, "Total") {
		t.Fatalf("single line got a total:\n%s", text)
	}
	if !strings.Contains(text, "2 × Gaze — 99,80 BRL") {
		t.Fatalf("line is wrong:\n%s", text)
	}
}

// An offer with no words is still a readable offer.
func TestRenderProposal_EmptyBodyStillRenders(t *testing.T) {
	text := RenderProposal(&domain.Proposal{
		Currency: "USD",
		Items:    []domain.ProposalItem{{Name: "Swab", Quantity: 1, UnitPriceCents: 123456}},
	})
	if strings.HasPrefix(text, "\n") {
		t.Fatalf("leading blank line:\n%q", text)
	}
	if !strings.Contains(text, "1,234.56 USD") {
		t.Fatalf("USD separators are wrong:\n%s", text)
	}
}

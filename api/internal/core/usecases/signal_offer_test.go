package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakeLeadLinker struct {
	linked [][3]string
	// stored is what Get answers: the manual path is handed an id and has to
	// read the row the automatic path already holds.
	stored map[string]*domain.Signal
}

func (f *fakeLeadLinker) LinkLead(_ context.Context, id, accountID, leadID string) error {
	f.linked = append(f.linked, [3]string{id, accountID, leadID})
	return nil
}

func (f *fakeLeadLinker) Get(_ context.Context, id, accountID string) (*domain.Signal, error) {
	s := f.stored[id]
	if s == nil || s.AccountID != accountID {
		return nil, nil
	}
	return s, nil
}

type fakeSignalDrafter struct {
	drafted []string
	err     error
}

func (f *fakeSignalDrafter) Draft(_ context.Context, _ string, leadID string) (*domain.Lead, error) {
	f.drafted = append(f.drafted, leadID)
	return nil, f.err
}

func tenderSignal() *domain.Signal {
	closes := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	return &domain.Signal{
		ID: "sig1", AccountID: "acc1", Source: domain.SignalPNCP,
		ExternalID: "pncp-123", Organization: "HOSPITAL MUNICIPAL DE CUIABA",
		CNPJ: "03507571000105", Title: "Pregão 100/2026 — ventiladores pulmonares",
		URL: "https://pncp.gov.br/x", ValueCents: 25_000_000, State: "MT",
		ClosesAt: &closes,
	}
}

func signalOfferHarness(t *testing.T, drafter *fakeSignalDrafter) (*SignalOfferUseCase, *fakeLeadLinker, *mockLeadRepo, *MockContactRepo) {
	t.Helper()
	linker := &fakeLeadLinker{}
	leadRepo := new(mockLeadRepo)
	contactRepo := new(MockContactRepo)
	// The resolver asks before it creates; these fixtures know nobody.
	contactRepo.On("GetByExternalID", mock.Anything, "acc1", mock.Anything).Return(nil, nil).Maybe()
	funnelRepo := new(MockSalesFunnelRepo)
	funnelRepo.On("ListByAccountID", mock.Anything, "acc1").Return(
		[]*domain.SalesFunnel{
			{ID: "f-other", AccountID: "acc1"},
			{ID: "f-default", AccountID: "acc1", IsDefault: true},
		}, nil)
	uc := NewSignalOfferUseCase(linker, contactRepo, funnelRepo, leadRepo, drafter)
	return uc, linker, leadRepo, contactRepo
}

func TestSignalOffer_TenderBecomesDraftedLead(t *testing.T) {
	drafter := &fakeSignalDrafter{}
	uc, linker, leadRepo, contactRepo := signalOfferHarness(t, drafter)

	contactRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(c *domain.Contact) bool {
		// CNPJ is the dedup key; the org name travels.
		return c.ExternalID == "03507571000105" && c.Name != nil && *c.Name == "HOSPITAL MUNICIPAL DE CUIABA"
	})).Return(&domain.Contact{ID: "c1", AccountID: "acc1", ExternalID: "03507571000105"}, nil)

	var savedLead *domain.Lead
	leadRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(l *domain.Lead) bool {
		savedLead = l
		return l.FunnelID == "f-default" && l.Source == "pncp_signal" && l.ContactID == "c1"
	})).Return(nil)

	require.NoError(t, uc.OfferFromSignal(context.Background(), tenderSignal()))

	require.NotNil(t, savedLead)
	var enrich map[string]any
	require.NoError(t, json.Unmarshal(savedLead.Enrichment, &enrich))
	assert.Equal(t, "pncp", enrich["source"])
	assert.Contains(t, enrich["edital"], "ventiladores")
	// Grouped in the Brazilian convention, through the shared formatter. The
	// old "%.2f" rendering handed the model "R$ 250000.00" — a figure a
	// Brazilian reader parses as two hundred and fifty, off by a thousand.
	assert.Equal(t, "250.000,00 BRL", enrich["edital_valor_estimado"])
	assert.Equal(t, "2026-09-14", enrich["edital_prazo"])

	assert.Equal(t, []string{savedLead.ID}, drafter.drafted, "the draft rides the normal pipeline")
	require.Len(t, linker.linked, 1)
	assert.Equal(t, [3]string{"sig1", "acc1", savedLead.ID}, linker.linked[0])
}

func TestSignalOffer_AlreadyWorkedIsSkipped(t *testing.T) {
	drafter := &fakeSignalDrafter{}
	uc, linker, _, _ := signalOfferHarness(t, drafter)

	s := tenderSignal()
	s.LeadID = "existing-lead"
	require.NoError(t, uc.OfferFromSignal(context.Background(), s))

	now := time.Now()
	s2 := tenderSignal()
	s2.DismissedAt = &now
	require.NoError(t, uc.OfferFromSignal(context.Background(), s2))

	assert.Empty(t, drafter.drafted, "worked or dismissed tenders spend nothing")
	assert.Empty(t, linker.linked)
}

func TestSignalOffer_DraftRefusalStillLinks(t *testing.T) {
	// A spend-cap refusal keeps the lead (undraftered) AND links the signal:
	// re-offering it every poll would mint duplicate leads.
	drafter := &fakeSignalDrafter{err: errors.New("AI_DRAFTS_BUDGET_EXCEEDED")}
	uc, linker, leadRepo, contactRepo := signalOfferHarness(t, drafter)

	contactRepo.On("Upsert", mock.Anything, mock.Anything).Return(&domain.Contact{ID: "c1"}, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	require.NoError(t, uc.OfferFromSignal(context.Background(), tenderSignal()))
	assert.Len(t, linker.linked, 1)
}

func TestSignalOffer_NoCNPJKeysOnSourceID(t *testing.T) {
	drafter := &fakeSignalDrafter{}
	uc, _, leadRepo, contactRepo := signalOfferHarness(t, drafter)

	contactRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(c *domain.Contact) bool {
		return c.ExternalID == "pncp:pncp-123"
	})).Return(&domain.Contact{ID: "c1"}, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	s := tenderSignal()
	s.CNPJ = ""
	require.NoError(t, uc.OfferFromSignal(context.Background(), s))
	contactRepo.AssertExpectations(t)
}

// ---- derived watch keywords ----

func TestDeriveWatchKeywords(t *testing.T) {
	products := []*domain.Product{
		{Name: "Ventilador Pulmonar VX"},
		{Name: "Bomba de infusão com kit adulto"},
		{Name: "Monitor multiparâmetro hospitalar"},
		{Name: "Luva"}, // 4 runes: too short to be a keyword
	}
	kws := deriveWatchKeywords(products)

	assert.Contains(t, kws, "ventilador")
	assert.Contains(t, kws, "pulmonar")
	assert.Contains(t, kws, "bomba")
	assert.Contains(t, kws, "infusão")
	assert.Contains(t, kws, "monitor")
	assert.Contains(t, kws, "multiparâmetro")
	assert.NotContains(t, kws, "adulto", "stopword")
	assert.NotContains(t, kws, "hospitalar", "stopword: matches every tender")
	assert.NotContains(t, kws, "luva", "too short")
	assert.NotContains(t, kws, "kit", "stopword and short")
	assert.LessOrEqual(t, len(kws), maxDerivedKeywords)
}

type fakeWatchCandidates struct{ list []domain.SignalWatchCandidate }

func (f fakeWatchCandidates) AccountsNeedingDefaultWatch(context.Context) ([]domain.SignalWatchCandidate, error) {
	return f.list, nil
}

type fakeWatchUpserter struct{ saved []*domain.SignalWatch }

func (f *fakeWatchUpserter) UpsertWatch(_ context.Context, w *domain.SignalWatch) error {
	f.saved = append(f.saved, w)
	return nil
}

type fakeProductLister struct{ byAccount map[string][]*domain.Product }

func (f fakeProductLister) Create(context.Context, *domain.Product) error { return nil }
func (f fakeProductLister) GetByID(context.Context, string) (*domain.Product, error) {
	return nil, nil
}
func (f fakeProductLister) ListByAccountID(_ context.Context, accountID string) ([]*domain.Product, error) {
	return f.byAccount[accountID], nil
}
func (f fakeProductLister) Update(context.Context, *domain.Product) error { return nil }
func (f fakeProductLister) Delete(context.Context, string) error          { return nil }

func TestSignalWatchSeeder_DerivesFromCatalogue(t *testing.T) {
	up := &fakeWatchUpserter{}
	seeder := NewSignalWatchSeeder(
		fakeWatchCandidates{list: []domain.SignalWatchCandidate{{AccountID: "acc1", UF: "mt"}}},
		fakeProductLister{byAccount: map[string][]*domain.Product{
			"acc1": {{Name: "Ventilador Pulmonar"}},
		}},
		up,
	)

	seeder.EnsureDefaultWatches(context.Background())

	require.Len(t, up.saved, 1)
	w := up.saved[0]
	assert.Equal(t, domain.SignalPNCP, w.Source)
	assert.True(t, w.IsActive)
	assert.Equal(t, []string{"MT"}, w.States)
	assert.Contains(t, w.Keywords, "ventilador")
}

func TestSignalWatchSeeder_EmptyCatalogueSeedsNothing(t *testing.T) {
	up := &fakeWatchUpserter{}
	seeder := NewSignalWatchSeeder(
		fakeWatchCandidates{list: []domain.SignalWatchCandidate{{AccountID: "acc1", UF: "MT"}}},
		fakeProductLister{},
		up,
	)
	seeder.EnsureDefaultWatches(context.Background())
	assert.Empty(t, up.saved, "a watch with no keywords would match everything")
}

// The manual door, and its whole point: the seller gets the card and the
// account's AI key is never touched.
//
// Until this existed the only path from a tender to a lead was the automatic
// offer, which always drafts. A seller who wanted to chase one by hand could
// read the notice (leaving the product, recording nothing) or dismiss it —
// while an expiring CONTRACT beside it on the same screen could be worked by
// hand.
func TestWorkSignal_PlacesTheTenderWithoutSpendingADraft(t *testing.T) {
	drafter := &fakeSignalDrafter{}
	uc, linker, leadRepo, contactRepo := signalOfferHarness(t, drafter)
	sig := tenderSignal()
	linker.stored = map[string]*domain.Signal{sig.ID: sig}
	contactRepo.On("Upsert", mock.Anything, mock.Anything).
		Return(&domain.Contact{ID: "c1", AccountID: "acc1", ExternalID: "03507571000105"}, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	lead, err := uc.WorkSignal(context.Background(), "acc1", "sig1")

	require.NoError(t, err)
	require.NotNil(t, lead)
	assert.Equal(t, "f-default", lead.FunnelID)
	assert.Equal(t, domain.LeadStatusProspected, lead.Status)
	assert.Equal(t, "pncp_signal", lead.Source)
	// The tender's facts travel with the card, or the seller opens a lead
	// that has forgotten why it exists.
	assert.Contains(t, string(lead.Enrichment), "pncp.gov.br")
	// Marked worked, so the nightly poll does not offer it again.
	require.Len(t, linker.linked, 1)
	assert.Equal(t, "sig1", linker.linked[0][0])
	// THE assertion.
	assert.Empty(t, drafter.drafted, "the manual path spent an AI draft")
}

// Pressing twice is navigation, not a second card for the same hospital.
func TestWorkSignal_IsIdempotent(t *testing.T) {
	drafter := &fakeSignalDrafter{}
	uc, linker, leadRepo, _ := signalOfferHarness(t, drafter)
	sig := tenderSignal()
	sig.LeadID = "lead-existente"
	linker.stored = map[string]*domain.Signal{sig.ID: sig}
	leadRepo.On("GetByID", mock.Anything, "lead-existente").
		Return(&domain.Lead{ID: "lead-existente", AccountID: "acc1", FunnelID: "f-default"}, nil)

	lead, err := uc.WorkSignal(context.Background(), "acc1", "sig1")

	require.NoError(t, err)
	assert.Equal(t, "lead-existente", lead.ID)
	leadRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
	assert.Empty(t, linker.linked, "a second link was written")
}

// A signal id from another account answers exactly like one that does not
// exist. Two different answers would make the endpoint a way to probe ids.
func TestWorkSignal_AnotherAccountsSignalIsSimplyNotFound(t *testing.T) {
	drafter := &fakeSignalDrafter{}
	uc, linker, _, _ := signalOfferHarness(t, drafter)
	sig := tenderSignal()
	linker.stored = map[string]*domain.Signal{sig.ID: sig}

	_, err := uc.WorkSignal(context.Background(), "outra-conta", "sig1")
	require.ErrorIs(t, err, domain.ErrSignalNotFound)

	_, err = uc.WorkSignal(context.Background(), "acc1", "nao-existe")
	require.ErrorIs(t, err, domain.ErrSignalNotFound)
}

// fakeSignalPark is the health registry as this flow needs it.
type fakeSignalPark struct {
	byCNPJ map[string]*domain.CNESEstablishment
	park   []domain.CNESParkItem
}

func (f *fakeSignalPark) FindEstablishmentByCNPJ(cnpj string) (*domain.CNESEstablishment, error) {
	return f.byCNPJ[cnpj], nil
}

func (f *fakeSignalPark) ParkByCNES(string) ([]domain.CNESParkItem, string, error) {
	return f.park, "202608", nil
}

// The two public sources answer different halves of one question — the notice
// says what they are buying, the registry says what they already run — and
// they were never joined. Production on 2026-09-17 held 60 leads carrying a
// park and 12 carrying a tender, and not one carrying both, so no draft could
// ever say "vocês publicaram o edital X e já operam N destes aparelhos".
func TestSignalOffer_CrossesTheTenderWithWhatTheBuyerAlreadyRuns(t *testing.T) {
	drafter := &fakeSignalDrafter{}
	uc, _, leadRepo, contactRepo := signalOfferHarness(t, drafter)
	phone := "(65) 3617-7876"
	email := "ca.hpsmc@cuiaba.mt.gov.br"
	uc.SetParkReader(&fakeSignalPark{
		byCNPJ: map[string]*domain.CNESEstablishment{
			"03507571000105": {
				CNES: "2495015", TradeName: "HOSPITAL E PRONTO SOCORRO MUNICIPAL DE CUIABA",
				Address: "Cuiabá", Phone: &phone, Email: &email,
			},
		},
		park: []domain.CNESParkItem{{Name: "Ventilador", QtExist: 12}},
	})

	var savedContact *domain.Contact
	contactRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(c *domain.Contact) bool {
		savedContact = c
		return true
	})).Return(&domain.Contact{ID: "c1", AccountID: "acc1"}, nil)
	var savedLead *domain.Lead
	leadRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(l *domain.Lead) bool {
		savedLead = l
		return true
	})).Return(nil)

	require.NoError(t, uc.OfferFromSignal(context.Background(), tenderSignal()))

	// The card carries a way to reach the buyer — a public notice publishes
	// neither a phone nor an address, and the registry publishes both.
	require.NotNil(t, savedContact)
	require.NotNil(t, savedContact.Email)
	assert.Equal(t, email, *savedContact.Email)
	require.NotNil(t, savedContact.Phone)

	// And the lead carries both halves at once.
	require.NotNil(t, savedLead)
	var enrich map[string]any
	require.NoError(t, json.Unmarshal(savedLead.Enrichment, &enrich))
	assert.Contains(t, enrich["edital"], "ventiladores")
	assert.Equal(t, "12x Ventilador", enrich["installed_park"])
	assert.Equal(t, "2495015", enrich["cnes"])
}

// A CNPJ the registry does not name — most tender buyers are city halls, not
// establishments — leaves the tender exactly as it was. No guessing.
func TestSignalOffer_ABuyerTheRegistryDoesNotKnowIsUnchanged(t *testing.T) {
	drafter := &fakeSignalDrafter{}
	uc, _, leadRepo, contactRepo := signalOfferHarness(t, drafter)
	uc.SetParkReader(&fakeSignalPark{byCNPJ: map[string]*domain.CNESEstablishment{}})

	contactRepo.On("Upsert", mock.Anything, mock.Anything).
		Return(&domain.Contact{ID: "c1", AccountID: "acc1"}, nil)
	var savedLead *domain.Lead
	leadRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(l *domain.Lead) bool {
		savedLead = l
		return true
	})).Return(nil)

	require.NoError(t, uc.OfferFromSignal(context.Background(), tenderSignal()))

	require.NotNil(t, savedLead)
	var enrich map[string]any
	require.NoError(t, json.Unmarshal(savedLead.Enrichment, &enrich))
	_, hasPark := enrich["installed_park"]
	assert.False(t, hasPark, "invented a park for a buyer the registry never named")
}

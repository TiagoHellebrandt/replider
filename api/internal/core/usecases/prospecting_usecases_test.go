package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/adapters/prospecting"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// echoContactRepo overrides only Upsert to echo back the contact it was given
// (with id == external_id, as the use case sets it) so the lead can key off a
// stable contact id. Other methods are inherited from the testify mock and are
// never called in these tests.
type echoContactRepo struct{ *MockContactRepo }

func (e *echoContactRepo) Upsert(ctx context.Context, c *domain.Contact) (*domain.Contact, error) {
	if c.ID == "" {
		c.ID = c.ExternalID
	}
	return c, nil
}

// fakeProspectingSearchRepo records the audit rows the use case writes.
type fakeProspectingSearchRepo struct{ created []*domain.ProspectingSearch }

func (f *fakeProspectingSearchRepo) Create(ctx context.Context, s *domain.ProspectingSearch) error {
	f.created = append(f.created, s)
	return nil
}

// fakeProspectingProvider returns canned candidates (or an error) and counts
// which search method was invoked.
type fakeProspectingProvider struct {
	candidates  []prospecting.Candidate
	err         error
	textCalls   int
	nearbyCalls int
}

func (f *fakeProspectingProvider) TextSearch(ctx context.Context, p prospecting.TextSearchParams) ([]prospecting.Candidate, error) {
	f.textCalls++
	return f.candidates, f.err
}

func (f *fakeProspectingProvider) NearbySearch(ctx context.Context, p prospecting.NearbySearchParams) ([]prospecting.Candidate, error) {
	f.nearbyCalls++
	return f.candidates, f.err
}

func threeCandidates() []prospecting.Candidate {
	return []prospecting.Candidate{
		{PlaceID: "place-1", Name: "Padaria A", Phone: "+55 11 90000-0001", Address: "Rua A", Website: "https://a.com"},
		{PlaceID: "place-2", Name: "Padaria B", Phone: "+551190000002"},
		// No phone — keyed by place_id instead.
		{PlaceID: "place-3", Name: "Padaria C"},
	}
}

func newProspectingUC(provider prospectingProvider, leadRepo *mockLeadRepo, funnelRepo *MockSalesFunnelRepo, searchRepo domain.ProspectingSearchRepository) *ProspectingUseCase {
	contactRepo := &echoContactRepo{MockContactRepo: new(MockContactRepo)}
	// The resolver asks before it creates; these fixtures know nobody.
	contactRepo.MockContactRepo.On("GetByExternalID", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil).Maybe()
	// nil account/plan repos → recipientCap falls back to the hard ceiling,
	// keeping the lead-creation count tests independent of plan wiring.
	return NewProspectingUseCase(funnelRepo, contactRepo, leadRepo, searchRepo, nil, nil, provider)
}

func TestProspectingUseCase_CreatesLeads(t *testing.T) {
	funnelRepo := new(MockSalesFunnelRepo)
	leadRepo := new(mockLeadRepo)
	searchRepo := &fakeProspectingSearchRepo{}
	provider := &fakeProspectingProvider{candidates: threeCandidates()}
	uc := newProspectingUC(provider, leadRepo, funnelRepo, searchRepo)

	funnelRepo.On("GetByID", mock.Anything, "fn-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", mock.Anything).Return(nil, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	res, err := uc.Execute(context.Background(), ProspectingSearchDTO{
		AccountID: "acc-1", FunnelID: "fn-1", Query: "padarias em sao paulo",
	})
	assert.NoError(t, err)
	assert.Equal(t, 3, res.ResultsCount)
	assert.Equal(t, 3, res.LeadsCreated)
	assert.Len(t, res.LeadIDs, 3)
	assert.Equal(t, 1, provider.textCalls)
	assert.Equal(t, 0, provider.nearbyCalls)
	// All three leads were prospected with source google_places.
	leadRepo.AssertNumberOfCalls(t, "Upsert", 3)
	// The search was recorded with leads_created=3.
	assert.Len(t, searchRepo.created, 1)
	assert.Equal(t, 3, searchRepo.created[0].LeadsCreated)
}

// "Ache 5 clínicas em São Paulo" importava 56 leads: o número pedido não
// tinha para onde ir e o limite era só o do plano. Cada página da Places é
// cobrada, e o funil de quem pediu cinco amanhecia com dezenas.
func TestProspectingUseCase_HonoursRequestedMax(t *testing.T) {
	funnelRepo := new(MockSalesFunnelRepo)
	leadRepo := new(mockLeadRepo)
	searchRepo := &fakeProspectingSearchRepo{}
	provider := &fakeProspectingProvider{candidates: threeCandidates()}
	uc := newProspectingUC(provider, leadRepo, funnelRepo, searchRepo)

	funnelRepo.On("GetByID", mock.Anything, "fn-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", mock.Anything).Return(nil, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	res, err := uc.Execute(context.Background(), ProspectingSearchDTO{
		AccountID: "acc-1", FunnelID: "fn-1", Query: "padarias em sao paulo",
		MaxResults: 2,
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, res.LeadsCreated, "pediu 2, entram 2")
	leadRepo.AssertNumberOfCalls(t, "Upsert", 2)
}

// Um número no chat não compra franquia: o pedido só aperta o teto, nunca
// o afrouxa.
func TestProspectingUseCase_RequestedMaxNeverRaisesThePlanCap(t *testing.T) {
	funnelRepo := new(MockSalesFunnelRepo)
	leadRepo := new(mockLeadRepo)
	searchRepo := &fakeProspectingSearchRepo{}
	provider := &fakeProspectingProvider{candidates: threeCandidates()}
	uc := newProspectingUC(provider, leadRepo, funnelRepo, searchRepo)

	funnelRepo.On("GetByID", mock.Anything, "fn-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", mock.Anything).Return(nil, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	res, err := uc.Execute(context.Background(), ProspectingSearchDTO{
		AccountID: "acc-1", FunnelID: "fn-1", Query: "padarias em sao paulo",
		MaxResults: 10_000,
	})
	assert.NoError(t, err)
	assert.Equal(t, 3, res.LeadsCreated, "o provedor só tinha 3; o pedido não inventa candidatos")
}

func TestProspectingUseCase_Idempotent(t *testing.T) {
	funnelRepo := new(MockSalesFunnelRepo)
	leadRepo := new(mockLeadRepo)
	searchRepo := &fakeProspectingSearchRepo{}
	provider := &fakeProspectingProvider{candidates: threeCandidates()}
	uc := newProspectingUC(provider, leadRepo, funnelRepo, searchRepo)

	funnelRepo.On("GetByID", mock.Anything, "fn-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil)
	// Every contact already has a lead on this funnel — re-running the search
	// must not create duplicates.
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", mock.Anything).
		Return(&domain.Lead{ID: "existing", FunnelID: "fn-1"}, nil)

	res, err := uc.Execute(context.Background(), ProspectingSearchDTO{
		AccountID: "acc-1", FunnelID: "fn-1", Query: "padarias em sao paulo",
	})
	assert.NoError(t, err)
	assert.Equal(t, 3, res.ResultsCount)
	assert.Equal(t, 0, res.LeadsCreated)
	assert.Equal(t, 3, res.SkippedDuplicate)
	leadRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}

func TestProspectingUseCase_NearbyWhenNoQuery(t *testing.T) {
	funnelRepo := new(MockSalesFunnelRepo)
	leadRepo := new(mockLeadRepo)
	provider := &fakeProspectingProvider{candidates: threeCandidates()}
	uc := newProspectingUC(provider, leadRepo, funnelRepo, &fakeProspectingSearchRepo{})

	funnelRepo.On("GetByID", mock.Anything, "fn-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", mock.Anything).Return(nil, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	res, err := uc.Execute(context.Background(), ProspectingSearchDTO{
		AccountID: "acc-1", FunnelID: "fn-1", Latitude: -23.5, Longitude: -46.6, RadiusMeters: 1000,
	})
	assert.NoError(t, err)
	assert.Equal(t, 3, res.LeadsCreated)
	assert.Equal(t, 1, provider.nearbyCalls)
	assert.Equal(t, 0, provider.textCalls)
}

func TestProspectingUseCase_OwnershipDenied(t *testing.T) {
	funnelRepo := new(MockSalesFunnelRepo)
	leadRepo := new(mockLeadRepo)
	provider := &fakeProspectingProvider{candidates: threeCandidates()}
	uc := newProspectingUC(provider, leadRepo, funnelRepo, &fakeProspectingSearchRepo{})

	funnelRepo.On("GetByID", mock.Anything, "fn-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "other-acc"}, nil)

	_, err := uc.Execute(context.Background(), ProspectingSearchDTO{
		AccountID: "acc-1", FunnelID: "fn-1", Query: "x",
	})
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
	assert.Equal(t, 0, provider.textCalls)
}

func TestProspectingUseCase_InputRequired(t *testing.T) {
	funnelRepo := new(MockSalesFunnelRepo)
	leadRepo := new(mockLeadRepo)
	provider := &fakeProspectingProvider{}
	uc := newProspectingUC(provider, leadRepo, funnelRepo, &fakeProspectingSearchRepo{})

	funnelRepo.On("GetByID", mock.Anything, "fn-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil)

	_, err := uc.Execute(context.Background(), ProspectingSearchDTO{
		AccountID: "acc-1", FunnelID: "fn-1",
	})
	assert.ErrorIs(t, err, domain.ErrProspectingInputRequired)
}

func TestProspectingUseCase_NotConfigured(t *testing.T) {
	funnelRepo := new(MockSalesFunnelRepo)
	leadRepo := new(mockLeadRepo)
	provider := &fakeProspectingProvider{err: prospecting.ErrAPIKeyMissing}
	uc := newProspectingUC(provider, leadRepo, funnelRepo, &fakeProspectingSearchRepo{})

	funnelRepo.On("GetByID", mock.Anything, "fn-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil)

	_, err := uc.Execute(context.Background(), ProspectingSearchDTO{
		AccountID: "acc-1", FunnelID: "fn-1", Query: "x",
	})
	assert.ErrorIs(t, err, domain.ErrProspectingNotConfigured)
}

package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Fakes for the three narrow ports. Plain structs, not mocks: each records
// what it was asked and answers what the test set.

type fakeLeadMaker struct {
	in  *ContactsToFunnelInput
	out *ContactsToFunnelResult
	err error
}

func (f *fakeLeadMaker) CreateFromContacts(_ context.Context, in ContactsToFunnelInput) (*ContactsToFunnelResult, error) {
	f.in = &in
	return f.out, f.err
}

type fakeDrafter struct {
	leadID string
	out    *domain.Lead
	err    error
}

func (f *fakeDrafter) Draft(_ context.Context, _ string, leadID string) (*domain.Lead, error) {
	f.leadID = leadID
	return f.out, f.err
}

type fakeProspector struct {
	dto *ProspectingSearchDTO
	out *ProspectingResult
	err error
}

func (f *fakeProspector) Execute(_ context.Context, dto ProspectingSearchDTO) (*ProspectingResult, error) {
	f.dto = &dto
	return f.out, f.err
}

func firstOfferFixture() (*MockAccountRepo, *MockSalesFunnelRepo, *MockContactRepo, *mockLeadRepo, *fakeLeadMaker, *fakeDrafter, *FirstOfferUseCase) {
	accountRepo := new(MockAccountRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	contactRepo := new(MockContactRepo)
	leadRepo := new(mockLeadRepo)
	maker := &fakeLeadMaker{}
	drafter := &fakeDrafter{}
	uc := NewFirstOfferUseCase(accountRepo, funnelRepo, contactRepo, leadRepo, maker, drafter)
	funnelRepo.On("EnsureDefault", mock.Anything, "acc-1").Return(&domain.SalesFunnel{ID: "f-def", AccountID: "acc-1", IsDefault: true}, nil)
	return accountRepo, funnelRepo, contactRepo, leadRepo, maker, drafter, uc
}

// The happy path: she imported customers, so the offer is for one of them.
func TestFirstOffer_WritesToACustomerSheBrought(t *testing.T) {
	_, _, contactRepo, leadRepo, maker, drafter, uc := firstOfferFixture()
	leadRepo.On("ListByFunnel", mock.Anything, "f-def", domain.LeadStatusPendingApproval, int32(1), int32(0)).Return([]*domain.Lead{}, nil)
	leadRepo.On("ListByFunnel", mock.Anything, "f-def", domain.LeadStatus(""), int32(1), int32(0)).Return([]*domain.Lead{}, nil)
	contactRepo.On("ListByAccountID", mock.Anything, "acc-1", "", 25, 0).Return([]*domain.Contact{{ID: "c-1"}}, nil)
	maker.out = &ContactsToFunnelResult{Created: 1, LeadIDs: []string{"lead-1"}}
	drafter.out = &domain.Lead{ID: "lead-1", Source: "existing_customer", ApproachDraft: "Olá, Santa Rosa…", Status: domain.LeadStatusPendingApproval}

	res, err := uc.Execute(context.Background(), "acc-1", "u-1")

	assert.NoError(t, err)
	assert.Equal(t, FirstOfferReady, res.Status)
	assert.Equal(t, FirstOfferFromCustomer, res.Source)
	assert.Equal(t, "Olá, Santa Rosa…", res.Draft)
	assert.Equal(t, "lead-1", drafter.leadID)
	// The lead lands in the DEFAULT funnel, owned by the person onboarding.
	assert.Equal(t, "f-def", maker.in.FunnelID)
	assert.Equal(t, "u-1", maker.in.OwnerUserID)
	assert.Equal(t, []string{"c-1"}, maker.in.ContactIDs)
}

// A reload must not make a second lead and spend a second draft.
func TestFirstOffer_ReturnsTheSameOfferOnReentry(t *testing.T) {
	_, _, contactRepo, leadRepo, maker, drafter, uc := firstOfferFixture()
	leadRepo.On("ListByFunnel", mock.Anything, "f-def", domain.LeadStatusPendingApproval, int32(1), int32(0)).
		Return([]*domain.Lead{{ID: "lead-1", Source: "existing_customer", ApproachDraft: "já escrita"}}, nil)

	res, err := uc.Execute(context.Background(), "acc-1", "u-1")

	assert.NoError(t, err)
	assert.Equal(t, FirstOfferReady, res.Status)
	assert.Equal(t, "já escrita", res.Draft)
	assert.Nil(t, maker.in, "no second lead")
	assert.Empty(t, drafter.leadID, "no second draft")
	contactRepo.AssertNotCalled(t, "ListByAccountID", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// No customers, but a territory: one business is found there.
func TestFirstOffer_FallsBackToABusinessInHerTerritory(t *testing.T) {
	accountRepo, _, contactRepo, leadRepo, maker, drafter, uc := firstOfferFixture()
	prospector := &fakeProspector{out: &ProspectingResult{LeadsCreated: 1, LeadIDs: []string{"lead-p"}}}
	uc.SetProspector(prospector)
	leadRepo.On("ListByFunnel", mock.Anything, "f-def", mock.Anything, int32(1), int32(0)).Return([]*domain.Lead{}, nil)
	contactRepo.On("ListByAccountID", mock.Anything, "acc-1", "", 25, 0).Return([]*domain.Contact{}, nil)
	accountRepo.On("GetByID", mock.Anything, "acc-1").Return(&domain.Account{
		ID: "acc-1", Territory: &domain.Territory{UF: "MT", Cities: []string{"Cuiabá", "Várzea Grande"}},
	}, nil)
	drafter.out = &domain.Lead{ID: "lead-p", Source: "google_places", ApproachDraft: "Bom dia…"}

	res, err := uc.Execute(context.Background(), "acc-1", "u-1")

	assert.NoError(t, err)
	assert.Equal(t, FirstOfferReady, res.Status)
	assert.Equal(t, FirstOfferFromProspect, res.Source)
	assert.Nil(t, maker.in)
	// The search is for hospitals in the first city she named, one result.
	assert.Equal(t, "hospital Cuiabá MT", prospector.dto.Query)
	assert.Equal(t, 1, prospector.dto.MaxResults)
	assert.Equal(t, "f-def", prospector.dto.FunnelID)
}

// Nothing to write to and nowhere to look: not an error, an honest answer.
func TestFirstOffer_SaysSoWhenThereIsNothingToOffer(t *testing.T) {
	accountRepo, _, contactRepo, leadRepo, _, drafter, uc := firstOfferFixture()
	leadRepo.On("ListByFunnel", mock.Anything, "f-def", mock.Anything, int32(1), int32(0)).Return([]*domain.Lead{}, nil)
	contactRepo.On("ListByAccountID", mock.Anything, "acc-1", "", 25, 0).Return([]*domain.Contact{}, nil)
	accountRepo.On("GetByID", mock.Anything, "acc-1").Return(&domain.Account{ID: "acc-1"}, nil)

	res, err := uc.Execute(context.Background(), "acc-1", "u-1")

	assert.NoError(t, err)
	assert.Equal(t, FirstOfferNothingToOffer, res.Status)
	assert.Equal(t, "no_customers_no_territory", res.Reason)
	assert.Empty(t, drafter.leadID)
}

// A search that cannot run (no key) is not her fault: the wizard still
// finishes, with the reason.
func TestFirstOffer_ASearchThatCannotRunIsNotAnError(t *testing.T) {
	accountRepo, _, contactRepo, leadRepo, _, _, uc := firstOfferFixture()
	// A transient refusal (rate limit) reads as "nobody found" — try later.
	uc.SetProspector(&fakeProspector{err: &domain.AppError{Code: "PROSPECTING_RATE_LIMITED", Message: "slow down"}})
	leadRepo.On("ListByFunnel", mock.Anything, "f-def", mock.Anything, int32(1), int32(0)).Return([]*domain.Lead{}, nil)
	contactRepo.On("ListByAccountID", mock.Anything, "acc-1", "", 25, 0).Return([]*domain.Contact{}, nil)
	accountRepo.On("GetByID", mock.Anything, "acc-1").Return(&domain.Account{
		ID: "acc-1", Territory: &domain.Territory{UF: "MT"},
	}, nil)

	res, err := uc.Execute(context.Background(), "acc-1", "u-1")

	assert.NoError(t, err)
	assert.Equal(t, FirstOfferNothingToOffer, res.Status)
	assert.Equal(t, FirstOfferReasonNoBusinessFound, res.Reason)
}

// The drafter's refusals (opt-out, spend cap) surface as they are — the
// wizard shows the same message the queue would.
func TestFirstOffer_ADraftRefusalSurfaces(t *testing.T) {
	_, _, contactRepo, leadRepo, maker, drafter, uc := firstOfferFixture()
	leadRepo.On("ListByFunnel", mock.Anything, "f-def", mock.Anything, int32(1), int32(0)).Return([]*domain.Lead{}, nil)
	contactRepo.On("ListByAccountID", mock.Anything, "acc-1", "", 25, 0).Return([]*domain.Contact{{ID: "c-1"}}, nil)
	maker.out = &ContactsToFunnelResult{LeadIDs: []string{"lead-1"}}
	drafter.err = domain.ErrContactOptedOut

	res, err := uc.Execute(context.Background(), "acc-1", "u-1")

	assert.Nil(t, res)
	assert.ErrorIs(t, err, domain.ErrContactOptedOut)
}

// Among the customers she brought, the first one WITH an email is offered
// first: the offer goes out by email, and a phone-only pick would draft fine
// and then fail at approval with "no address to write to".
func TestFirstOffer_PrefersACustomerWithAnEmail(t *testing.T) {
	_, _, contactRepo, leadRepo, maker, drafter, uc := firstOfferFixture()
	leadRepo.On("ListByFunnel", mock.Anything, "f-def", mock.Anything, int32(1), int32(0)).Return([]*domain.Lead{}, nil)
	email := "compras@hospital.com.br"
	contactRepo.On("ListByAccountID", mock.Anything, "acc-1", "", 25, 0).Return([]*domain.Contact{
		{ID: "c-phone"},
		{ID: "c-mail", Email: &email},
	}, nil)
	maker.out = &ContactsToFunnelResult{Created: 1, LeadIDs: []string{"lead-mail"}}
	drafter.out = &domain.Lead{ID: "lead-mail", Source: "existing_customer", ApproachDraft: "Olá…", Status: domain.LeadStatusPendingApproval}

	res, err := uc.Execute(context.Background(), "acc-1", "u-1")

	assert.NoError(t, err)
	assert.Equal(t, FirstOfferFromCustomer, res.Source)
	assert.Equal(t, []string{"c-mail"}, maker.in.ContactIDs)
}

// A platform with no Places key is an operator gap, and the wizard says so:
// "no business found" would send her looking for a hospital that was never
// searched for.
func TestFirstOffer_AnUnconfiguredProspectorSaysSo(t *testing.T) {
	accountRepo, _, contactRepo, leadRepo, _, _, uc := firstOfferFixture()
	uc.SetProspector(&fakeProspector{err: domain.ErrProspectingNotConfigured})
	leadRepo.On("ListByFunnel", mock.Anything, "f-def", mock.Anything, int32(1), int32(0)).Return([]*domain.Lead{}, nil)
	contactRepo.On("ListByAccountID", mock.Anything, "acc-1", "", 25, 0).Return([]*domain.Contact{}, nil)
	accountRepo.On("GetByID", mock.Anything, "acc-1").Return(&domain.Account{
		ID: "acc-1", Territory: &domain.Territory{UF: "MT"},
	}, nil)

	res, err := uc.Execute(context.Background(), "acc-1", "u-1")

	assert.NoError(t, err)
	assert.Equal(t, FirstOfferNothingToOffer, res.Status)
	assert.Equal(t, FirstOfferReasonProspectingNotConfigured, res.Reason)
}

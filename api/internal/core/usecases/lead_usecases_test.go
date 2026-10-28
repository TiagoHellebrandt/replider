package usecases

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockLeadRepo is a testify mock for domain.LeadRepository scoped to the
// lead use-case tests (the services package has its own MockLeadRepo).
type mockLeadRepo struct{ mock.Mock }

func (m *mockLeadRepo) Upsert(ctx context.Context, lead *domain.Lead) error {
	return m.Called(ctx, lead).Error(0)
}

func (m *mockLeadRepo) GetByFunnelAndContact(ctx context.Context, funnelID, contactID string) (*domain.Lead, error) {
	args := m.Called(ctx, funnelID, contactID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Lead), args.Error(1)
}

func (m *mockLeadRepo) GetByID(ctx context.Context, id string) (*domain.Lead, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Lead), args.Error(1)
}

func (m *mockLeadRepo) ListForApprovalQueue(ctx context.Context, f domain.ApprovalQueueFilter, limit, offset int32) ([]*domain.Lead, error) {
	args := m.Called(ctx, f, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Lead), args.Error(1)
}

func (m *mockLeadRepo) CountApprovalQueueByFunnel(ctx context.Context, f domain.ApprovalQueueFilter) ([]domain.QueueFunnelCount, error) {
	args := m.Called(ctx, f)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.QueueFunnelCount), args.Error(1)
}

func (m *mockLeadRepo) CountByStatus(ctx context.Context, accountID string) (map[domain.LeadStatus]int64, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[domain.LeadStatus]int64), args.Error(1)
}

func (m *mockLeadRepo) CountByStatusForOwner(ctx context.Context, accountID, ownerUserID string) (map[domain.LeadStatus]int64, error) {
	args := m.Called(ctx, accountID, ownerUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[domain.LeadStatus]int64), args.Error(1)
}

func (m *mockLeadRepo) CountUnowned(ctx context.Context, accountID string) (int64, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockLeadRepo) ListByFunnel(ctx context.Context, funnelID string, status domain.LeadStatus, limit, offset int32) ([]*domain.Lead, error) {
	args := m.Called(ctx, funnelID, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Lead), args.Error(1)
}

func (m *mockLeadRepo) UpdateMetadata(ctx context.Context, id string, collectedMetadata, enrichment json.RawMessage) error {
	return m.Called(ctx, id, collectedMetadata, enrichment).Error(0)
}

func (m *mockLeadRepo) UpdateStatus(ctx context.Context, id string, status domain.LeadStatus) error {
	return m.Called(ctx, id, status).Error(0)
}

func (m *mockLeadRepo) MarkBuyIntent(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

func (m *mockLeadRepo) MarkQualified(ctx context.Context, id string) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func TestLeadUseCase_Get_OwnershipDenied(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	uc := NewLeadUseCase(leadRepo, funnelRepo)

	leadRepo.On("GetByID", mock.Anything, "lead-1").
		Return(&domain.Lead{ID: "lead-1", AccountID: "other-acc"}, nil)

	_, err := uc.Get(context.Background(), "acc-1", "lead-1")
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
	leadRepo.AssertExpectations(t)
}

func TestLeadUseCase_ListByFunnel_OwnershipDenied(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	uc := NewLeadUseCase(leadRepo, funnelRepo)

	funnelRepo.On("GetByID", mock.Anything, "fn-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "other-acc"}, nil)

	_, err := uc.ListByFunnel(context.Background(), "acc-1", "fn-1", "", 50, 0)
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
	// ListByFunnel must never reach the lead repo when the funnel isn't owned.
	leadRepo.AssertNotCalled(t, "ListByFunnel", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	funnelRepo.AssertExpectations(t)
}

// The owner UPDATE filters on the LEAD's account_id, not the user's. Without this
// check, a ULID from another tenant was accepted and the account ended up with a
// lead whose owner sits outside it — invisible in the team panel and accruing
// commission for somebody who should not be there.
func TestLeadUseCase_AssignOwner_RejectsUserFromAnotherAccount(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	userRepo := new(MockUserRepo)
	uc := NewLeadUseCase(leadRepo, funnelRepo)
	uc.SetUserRepo(userRepo)

	leadRepo.On("GetByID", mock.Anything, "lead-1").
		Return(&domain.Lead{ID: "lead-1", AccountID: "acc-1"}, nil)
	userRepo.On("GetByID", mock.Anything, "user-de-fora").
		Return(&domain.User{ID: "user-de-fora", AccountID: "outra-acc", IsActive: true}, nil)

	actor := domain.Actor{AccountID: "acc-1", Role: domain.RoleGestor}
	owner := "user-de-fora"
	_, err := uc.AssignOwner(context.Background(), actor, "lead-1", &owner)

	assert.ErrorIs(t, err, domain.ErrLeadOwnerNotInAccount)
	leadRepo.AssertNotCalled(t, "AssignOwner",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A user who does not exist gets the SAME answer as one who exists in another
// account: telling the two apart would confirm, to anyone sweeping ULIDs, which
// ones exist somewhere.
func TestLeadUseCase_AssignOwner_UnknownUserAnswersTheSame(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	userRepo := new(MockUserRepo)
	uc := NewLeadUseCase(leadRepo, funnelRepo)
	uc.SetUserRepo(userRepo)

	leadRepo.On("GetByID", mock.Anything, "lead-1").
		Return(&domain.Lead{ID: "lead-1", AccountID: "acc-1"}, nil)
	userRepo.On("GetByID", mock.Anything, "ninguem").Return(nil, nil)

	actor := domain.Actor{AccountID: "acc-1", Role: domain.RoleGestor}
	owner := "ninguem"
	_, err := uc.AssignOwner(context.Background(), actor, "lead-1", &owner)

	assert.ErrorIs(t, err, domain.ErrLeadOwnerNotInAccount)
}

// Unassigning has no destination to check — and it is the only way to take a lead
// from somebody.
func TestLeadUseCase_AssignOwner_ClearingNeedsNoUserLookup(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	userRepo := new(MockUserRepo)
	uc := NewLeadUseCase(leadRepo, funnelRepo)
	uc.SetUserRepo(userRepo)

	leadRepo.On("GetByID", mock.Anything, "lead-1").
		Return(&domain.Lead{ID: "lead-1", AccountID: "acc-1"}, nil)
	leadRepo.On("AssignOwner", mock.Anything, "acc-1", "lead-1", (*string)(nil)).
		Return(&domain.Lead{ID: "lead-1", AccountID: "acc-1"}, nil)

	actor := domain.Actor{AccountID: "acc-1", Role: domain.RoleGestor}
	_, err := uc.AssignOwner(context.Background(), actor, "lead-1", nil)

	assert.NoError(t, err)
	userRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

// With the check unwired, it REFUSES. That is the opposite of the other optionals
// on this type, and deliberately so: the others degrade by losing a nicety, this
// one would lose the only check that separates accounts.
func TestLeadUseCase_AssignOwner_RefusesWhenTheCheckIsUnwired(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	uc := NewLeadUseCase(leadRepo, funnelRepo) // sem SetUserRepo

	leadRepo.On("GetByID", mock.Anything, "lead-1").
		Return(&domain.Lead{ID: "lead-1", AccountID: "acc-1"}, nil)

	actor := domain.Actor{AccountID: "acc-1", Role: domain.RoleGestor}
	owner := "user-1"
	_, err := uc.AssignOwner(context.Background(), actor, "lead-1", &owner)

	assert.ErrorIs(t, err, domain.ErrLeadAssignUnavailable)
	leadRepo.AssertNotCalled(t, "AssignOwner",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A salesperson who could mark leads private would take the pipeline out of the
// sight of whoever answers for it.
// Whoever clicked is looking at a screen that may be seconds stale. Undoing 60
// assignments because the 61st no longer exists is worse than reporting the ones
// left out.
func TestLeadUseCase_AssignOwnerBulk_SkipsMissingInsteadOfAborting(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	userRepo := new(MockUserRepo)
	uc := NewLeadUseCase(leadRepo, funnelRepo)
	uc.SetUserRepo(userRepo)

	userRepo.On("GetByID", mock.Anything, "u1").
		Return(&domain.User{ID: "u1", AccountID: "acc-1", IsActive: true}, nil)
	owner := "u1"
	for _, id := range []string{"a", "c"} {
		leadRepo.On("AssignOwner", mock.Anything, "acc-1", id, &owner).
			Return(&domain.Lead{ID: id, AccountID: "acc-1"}, nil)
	}
	// "b" sumiu do funil entre a contagem e o clique.
	leadRepo.On("AssignOwner", mock.Anything, "acc-1", "b", &owner).
		Return(nil, nil)

	actor := domain.Actor{AccountID: "acc-1", Role: domain.RoleGestor}
	res, err := uc.AssignOwnerBulk(context.Background(), actor,
		[]string{"a", "b", "c"}, &owner)

	assert.NoError(t, err)
	assert.Equal(t, 2, res.Assigned)
	assert.Equal(t, []string{"b"}, res.Skipped)
}

// The owner check runs ONCE, before any write: per lead it would be the same
// answer repeated N times, and an error midway would leave half assigned.
func TestLeadUseCase_AssignOwnerBulk_ChecksTheOwnerOnceBeforeWriting(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	userRepo := new(MockUserRepo)
	uc := NewLeadUseCase(leadRepo, funnelRepo)
	uc.SetUserRepo(userRepo)

	userRepo.On("GetByID", mock.Anything, "de-fora").
		Return(&domain.User{ID: "de-fora", AccountID: "outra", IsActive: true}, nil).Once()

	actor := domain.Actor{AccountID: "acc-1", Role: domain.RoleGestor}
	owner := "de-fora"
	_, err := uc.AssignOwnerBulk(context.Background(), actor,
		[]string{"a", "b", "c"}, &owner)

	assert.ErrorIs(t, err, domain.ErrLeadOwnerNotInAccount)
	leadRepo.AssertNotCalled(t, "AssignOwner",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	userRepo.AssertNumberOfCalls(t, "GetByID", 1)
}

// The same screen can send a repeated id; assigning twice does no harm, but it
// would count two in the report.
func TestLeadUseCase_AssignOwnerBulk_CountsEachLeadOnce(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	userRepo := new(MockUserRepo)
	uc := NewLeadUseCase(leadRepo, funnelRepo)
	uc.SetUserRepo(userRepo)

	userRepo.On("GetByID", mock.Anything, "u1").
		Return(&domain.User{ID: "u1", AccountID: "acc-1", IsActive: true}, nil)
	owner := "u1"
	leadRepo.On("AssignOwner", mock.Anything, "acc-1", "a", &owner).
		Return(&domain.Lead{ID: "a", AccountID: "acc-1"}, nil).Once()

	actor := domain.Actor{AccountID: "acc-1", Role: domain.RoleGestor}
	res, err := uc.AssignOwnerBulk(context.Background(), actor,
		[]string{"a", "a", ""}, &owner)

	assert.NoError(t, err)
	assert.Equal(t, 1, res.Assigned)
	leadRepo.AssertNumberOfCalls(t, "AssignOwner", 1)
}

// A successful "assigned 0" hides the empty list the client sent by mistake.
func TestLeadUseCase_AssignOwnerBulk_RefusesAnEmptyList(t *testing.T) {
	uc := NewLeadUseCase(new(mockLeadRepo), new(MockSalesFunnelRepo))
	actor := domain.Actor{AccountID: "acc-1", Role: domain.RoleGestor}

	_, err := uc.AssignOwnerBulk(context.Background(), actor, nil, nil)

	assert.ErrorIs(t, err, domain.ErrBulkAssignEmpty)
}

// Redistributing touches commission: a mistake across 200 is already an afternoon
// of repair.
func TestLeadUseCase_AssignOwnerBulk_CapsTheBlastRadius(t *testing.T) {
	uc := NewLeadUseCase(new(mockLeadRepo), new(MockSalesFunnelRepo))
	actor := domain.Actor{AccountID: "acc-1", Role: domain.RoleGestor}

	ids := make([]string, maxBulkAssign+1)
	for i := range ids {
		ids[i] = "lead"
	}
	_, err := uc.AssignOwnerBulk(context.Background(), actor, ids, nil)

	assert.ErrorIs(t, err, domain.ErrBulkAssignTooMany)
}

func TestLeadUseCase_AssignOwnerBulk_VendedorRefused(t *testing.T) {
	uc := NewLeadUseCase(new(mockLeadRepo), new(MockSalesFunnelRepo))
	actor := domain.Actor{AccountID: "acc-1", Role: domain.RoleVendedor}

	_, err := uc.AssignOwnerBulk(context.Background(), actor, []string{"a"}, nil)

	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestLeadUseCase_SetVisibility_VendedorRefused(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	uc := NewLeadUseCase(leadRepo, funnelRepo)

	actor := domain.Actor{AccountID: "acc-1", Role: domain.RoleVendedor}
	_, err := uc.SetVisibility(context.Background(), actor, "lead-1",
		domain.VisibilityPrivate)

	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestLeadUseCase_SetVisibility_RejectsGarbage(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	uc := NewLeadUseCase(leadRepo, funnelRepo)

	actor := domain.Actor{AccountID: "acc-1", Role: domain.RoleGestor}
	_, err := uc.SetVisibility(context.Background(), actor, "lead-1", "secreto")

	assert.ErrorIs(t, err, domain.ErrInvalidLeadVisibility)
	leadRepo.AssertNotCalled(t, "SetVisibility",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestLeadUseCase_UpdateMetadataField_Human(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	uc := NewLeadUseCase(leadRepo, funnelRepo)

	leadRepo.On("GetByID", mock.Anything, "lead-1").
		Return(&domain.Lead{
			ID:                "lead-1",
			AccountID:         "acc-1",
			FunnelID:          "fn-1",
			CollectedMetadata: json.RawMessage(`{}`),
			Enrichment:        json.RawMessage(`{}`),
		}, nil)
	funnelRepo.On("GetByID", mock.Anything, "fn-1").
		Return(&domain.SalesFunnel{
			ID:             "fn-1",
			AccountID:      "acc-1",
			MetadataSchema: json.RawMessage(`[{"key":"empresa","label":"Empresa","type":"text","required":true}]`),
		}, nil)

	var savedBlob json.RawMessage
	leadRepo.On("UpdateMetadata", mock.Anything, "lead-1", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { savedBlob = args.Get(2).(json.RawMessage) }).
		Return(nil)

	lead, err := uc.UpdateMetadataField(context.Background(), "acc-1", "lead-1", "empresa", "Acme SA")
	assert.NoError(t, err)
	assert.NotNil(t, lead)

	collected := map[string]services.CollectedField{}
	assert.NoError(t, json.Unmarshal(savedBlob, &collected))
	assert.Equal(t, "Acme SA", collected["empresa"].Value)
	assert.Equal(t, services.CollectedSourceHuman, collected["empresa"].Source)
	leadRepo.AssertExpectations(t)
}

func TestLeadUseCase_UpdateMetadataField_UnknownKeyRejected(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	uc := NewLeadUseCase(leadRepo, funnelRepo)

	leadRepo.On("GetByID", mock.Anything, "lead-1").
		Return(&domain.Lead{ID: "lead-1", AccountID: "acc-1", FunnelID: "fn-1"}, nil)
	funnelRepo.On("GetByID", mock.Anything, "fn-1").
		Return(&domain.SalesFunnel{
			ID:             "fn-1",
			AccountID:      "acc-1",
			MetadataSchema: json.RawMessage(`[{"key":"empresa","label":"Empresa","type":"text"}]`),
		}, nil)

	_, err := uc.UpdateMetadataField(context.Background(), "acc-1", "lead-1", "cargo", "CEO")
	assert.ErrorIs(t, err, domain.ErrLeadFieldNotInSchema)
	leadRepo.AssertNotCalled(t, "UpdateMetadata", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestLeadUseCase_UpdateStatus_InvalidRejected(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	uc := NewLeadUseCase(leadRepo, funnelRepo)

	_, err := uc.UpdateStatus(context.Background(), "acc-1", "lead-1", domain.LeadStatus("bogus"))
	assert.ErrorIs(t, err, domain.ErrInvalidLeadStatus)
	leadRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

// inboundLeadHook covers SaveInbound's markLeadResponded hook (US-015).
func newInboundHookUC(funnelRepo domain.SalesFunnelRepository, leadRepo domain.LeadRepository) *InboundWebhookUseCase {
	uc := NewInboundWebhookUseCase(nil, nil, nil, nil, nil)
	uc.SetSalesLead(funnelRepo, leadRepo)
	return uc
}

func TestInbound_MarkLeadResponded_CreatesWhenMissing(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	uc := newInboundHookUC(funnelRepo, leadRepo)

	ch := &domain.Channel{ID: "ch-1", AccountID: "acc-1"}
	funnelRepo.On("GetByChannelID", mock.Anything, "ch-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil)
	// The lead is keyed by the RESOLVED contact's ULID; the raw sender is
	// only consulted for legacy rows. Neither matches → create with the FK.
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", "ct-1").Return(nil, nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", "5511999").Return(nil, nil)
	leadRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(l *domain.Lead) bool {
		return l.FunnelID == "fn-1" && l.ContactID == "ct-1" &&
			l.Status == domain.LeadStatusResponded && l.Source == "inbound"
	})).Return(nil)

	uc.markLeadResponded(context.Background(), ch, &domain.Contact{ID: "ct-1"}, "5511999", InboundMessageDTO{})
	leadRepo.AssertExpectations(t)
}

// The FK-violation class: a sender the identity resolver could not place has
// no contacts row, so there is nothing valid to point leads.contact_id at.
// The hook must refuse to create — forcing the raw sender through the FK is
// the write that WARNed on every email reply.
func TestInbound_MarkLeadResponded_RefusesToCreateWithoutContact(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	uc := newInboundHookUC(funnelRepo, leadRepo)

	ch := &domain.Channel{ID: "ch-1", AccountID: "acc-1"}
	funnelRepo.On("GetByChannelID", mock.Anything, "ch-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", "compras@hospital.com.br").
		Return(nil, nil)

	uc.markLeadResponded(context.Background(), ch, nil, "compras@hospital.com.br", InboundMessageDTO{})
	leadRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
	leadRepo.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything)
}

// A pre-migration lead carries the sender verbatim in contact_id. It must
// still advance — matching legacy rows is allowed, only creating with them is
// not.
func TestInbound_MarkLeadResponded_LegacyRawSenderLeadStillAdvances(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	uc := newInboundHookUC(funnelRepo, leadRepo)

	ch := &domain.Channel{ID: "ch-1", AccountID: "acc-1"}
	funnelRepo.On("GetByChannelID", mock.Anything, "ch-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", "ct-1").Return(nil, nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", "5511999").
		Return(&domain.Lead{ID: "lead-legacy", Status: domain.LeadStatusContacted}, nil)
	leadRepo.On("UpdateStatus", mock.Anything, "lead-legacy", domain.LeadStatusResponded).Return(nil)

	uc.markLeadResponded(context.Background(), ch, &domain.Contact{ID: "ct-1"}, "5511999", InboundMessageDTO{})
	leadRepo.AssertExpectations(t)
}

// --- markLeadLost: the opt-out mirror of the same disease (Item 3) ----------

func TestInbound_MarkLeadLost_KeysByTheResolvedContact(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	uc := newInboundHookUC(funnelRepo, leadRepo)

	ch := &domain.Channel{ID: "ch-1", AccountID: "acc-1"}
	funnelRepo.On("GetByChannelID", mock.Anything, "ch-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", "ct-1").
		Return(&domain.Lead{ID: "lead-1", Status: domain.LeadStatusContacted}, nil)
	leadRepo.On("UpdateStatus", mock.Anything, "lead-1", domain.LeadStatusLost).Return(nil)

	uc.markLeadLost(context.Background(), ch, &domain.Contact{ID: "ct-1"}, "contato@clinica.com.br")
	leadRepo.AssertExpectations(t)
}

// No resolved contact and no legacy row: nothing to mark, and nothing is
// guessed — the WARN is the trace.
func TestInbound_MarkLeadLost_UnresolvedSenderMarksNothing(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	uc := newInboundHookUC(funnelRepo, leadRepo)

	ch := &domain.Channel{ID: "ch-1", AccountID: "acc-1"}
	funnelRepo.On("GetByChannelID", mock.Anything, "ch-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", "contato@clinica.com.br").
		Return(nil, nil)

	uc.markLeadLost(context.Background(), ch, nil, "contato@clinica.com.br")
	leadRepo.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything)
	leadRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}

// A pre-migration lead keyed by the raw sender still goes lost — matching
// legacy rows is allowed, only creating with them is not.
func TestInbound_MarkLeadLost_LegacyRawSenderLeadStillGoesLost(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	uc := newInboundHookUC(funnelRepo, leadRepo)

	ch := &domain.Channel{ID: "ch-1", AccountID: "acc-1"}
	funnelRepo.On("GetByChannelID", mock.Anything, "ch-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", "ct-1").Return(nil, nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", "5511999").
		Return(&domain.Lead{ID: "lead-legacy", Status: domain.LeadStatusContacted}, nil)
	leadRepo.On("UpdateStatus", mock.Anything, "lead-legacy", domain.LeadStatusLost).Return(nil)

	uc.markLeadLost(context.Background(), ch, &domain.Contact{ID: "ct-1"}, "5511999")
	leadRepo.AssertExpectations(t)
}

// A deal already settled is history: an opt-out must not rewrite a win.
func TestInbound_MarkLeadLost_LeavesWonAlone(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	uc := newInboundHookUC(funnelRepo, leadRepo)

	ch := &domain.Channel{ID: "ch-1", AccountID: "acc-1"}
	funnelRepo.On("GetByChannelID", mock.Anything, "ch-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", "ct-1").
		Return(&domain.Lead{ID: "lead-1", Status: domain.LeadStatusWon}, nil)

	uc.markLeadLost(context.Background(), ch, &domain.Contact{ID: "ct-1"}, "5511999")
	leadRepo.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything)
}

func TestInbound_MarkLeadResponded_BumpsContacted(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	uc := newInboundHookUC(funnelRepo, leadRepo)

	ch := &domain.Channel{ID: "ch-1", AccountID: "acc-1"}
	funnelRepo.On("GetByChannelID", mock.Anything, "ch-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", "ct-1").
		Return(&domain.Lead{ID: "lead-1", Status: domain.LeadStatusContacted}, nil)
	leadRepo.On("UpdateStatus", mock.Anything, "lead-1", domain.LeadStatusResponded).Return(nil)

	uc.markLeadResponded(context.Background(), ch, &domain.Contact{ID: "ct-1"}, "5511999", InboundMessageDTO{})
	leadRepo.AssertExpectations(t)
}

func TestInbound_MarkLeadResponded_LeavesQualified(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	uc := newInboundHookUC(funnelRepo, leadRepo)

	ch := &domain.Channel{ID: "ch-1", AccountID: "acc-1"}
	funnelRepo.On("GetByChannelID", mock.Anything, "ch-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", "ct-1").
		Return(&domain.Lead{ID: "lead-1", Status: domain.LeadStatusQualified}, nil)

	uc.markLeadResponded(context.Background(), ch, &domain.Contact{ID: "ct-1"}, "5511999", InboundMessageDTO{})
	// A qualified lead must not be dragged back to responded.
	leadRepo.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything)
	leadRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}

// US-F009: ownership-aware reads/writes.
func (m *mockLeadRepo) SetVisibility(ctx context.Context, accountID, id string, v domain.ResourceVisibility) (*domain.Lead, error) {
	args := m.Called(ctx, accountID, id, v)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Lead), args.Error(1)
}

func (m *mockLeadRepo) AssignOwner(ctx context.Context, accountID, id string, ownerUserID *string) (*domain.Lead, error) {
	args := m.Called(ctx, accountID, id, ownerUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Lead), args.Error(1)
}

func (m *mockLeadRepo) ListByFunnelForOwner(ctx context.Context, accountID, funnelID, ownerUserID string, status domain.LeadStatus, limit, offset int32) ([]*domain.Lead, error) {
	args := m.Called(ctx, accountID, funnelID, ownerUserID, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Lead), args.Error(1)
}

// --- The approval queue's filter and counts --------------------------------

// The queue is the operator's whole job, so the filter has to travel INTO the
// query. These pin what reaches the repository; the SQL itself is pinned by
// e2e/kanban_board_test.go.
func TestApprovalQueue_PassesTheFilterDownAndTotalsTheWholeQueue(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	uc := NewLeadUseCase(leadRepo, new(MockSalesFunnelRepo))

	want := domain.ApprovalQueueFilter{
		AccountID:   "acc-1",
		Status:      domain.LeadStatusPendingApproval,
		FunnelID:    "fn-2",
		NewestFirst: true,
	}
	leadRepo.On("ListForApprovalQueue", mock.Anything, want, int32(50), int32(0)).
		Return([]*domain.Lead{{ID: "lead-1", AccountID: "acc-1"}}, nil)
	// Two funnels in the backlog, only one of them selected: the total is the
	// sum of BOTH, because the number the operator needs is how much work is
	// left, not how much of it is currently on screen.
	leadRepo.On("CountApprovalQueueByFunnel", mock.Anything, want).
		Return([]domain.QueueFunnelCount{
			{FunnelID: "fn-2", FunnelName: "Clínicas", Total: 12},
			{FunnelID: "fn-9", FunnelName: "Academias", Total: 30},
		}, nil)

	actor := domain.Actor{AccountID: "acc-1", UserID: "u-1", Role: domain.RoleGestor}
	page, err := uc.ApprovalQueue(context.Background(), actor,
		ApprovalQueueOptions{FunnelID: "fn-2", NewestFirst: true}, 50, 0)

	require.NoError(t, err)
	assert.Len(t, page.Items, 1)
	assert.Equal(t, int64(42), page.Total)
	assert.Len(t, page.ByFunnel, 2)
	leadRepo.AssertExpectations(t)
}

// A gestor reads the account; a vendedor's own id narrows the read. The counts
// carry the SAME scope, so a chip can never offer a draft the list would then
// refuse to hand over.
func TestApprovalQueue_ScopesCountsLikeTheList(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	uc := NewLeadUseCase(leadRepo, new(MockSalesFunnelRepo))

	scoped := domain.ApprovalQueueFilter{
		AccountID:   "acc-1",
		OwnerUserID: "u-vend",
		Status:      domain.LeadStatusPendingApproval,
	}
	leadRepo.On("ListForApprovalQueue", mock.Anything, scoped, int32(50), int32(0)).
		Return([]*domain.Lead{}, nil)
	leadRepo.On("CountApprovalQueueByFunnel", mock.Anything, scoped).
		Return([]domain.QueueFunnelCount{}, nil)

	actor := domain.Actor{AccountID: "acc-1", UserID: "u-vend", Role: domain.RoleVendedor}
	_, err := uc.ApprovalQueue(context.Background(), actor, ApprovalQueueOptions{}, 50, 0)

	require.NoError(t, err)
	leadRepo.AssertExpectations(t)
}

// The counts are the screen's navigation, not its content. Losing them must not
// lose the drafts: those were read successfully and somebody is waiting on them.
func TestApprovalQueue_AFailedCountStillReturnsTheDrafts(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	uc := NewLeadUseCase(leadRepo, new(MockSalesFunnelRepo))

	leadRepo.On("ListForApprovalQueue", mock.Anything, mock.Anything, int32(50), int32(0)).
		Return([]*domain.Lead{{ID: "lead-1"}, {ID: "lead-2"}}, nil)
	leadRepo.On("CountApprovalQueueByFunnel", mock.Anything, mock.Anything).
		Return(nil, assert.AnError)

	actor := domain.Actor{AccountID: "acc-1", Role: domain.RoleGestor}
	page, err := uc.ApprovalQueue(context.Background(), actor, ApprovalQueueOptions{}, 50, 0)

	require.NoError(t, err)
	assert.Len(t, page.Items, 2)
	// Nothing is claimed rather than something wrong being claimed: a total of
	// 0 next to two visible cards is a screen arguing with itself.
	assert.Zero(t, page.Total)
	assert.Empty(t, page.ByFunnel)
}

// An attendant has no business approving outbound sales, and must not be able
// to count it either.
func TestApprovalQueue_AnAttendantIsRefusedBeforeAnyRead(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	uc := NewLeadUseCase(leadRepo, new(MockSalesFunnelRepo))

	actor := domain.Actor{AccountID: "acc-1", Role: domain.RoleAttendant}
	_, err := uc.ApprovalQueue(context.Background(), actor, ApprovalQueueOptions{}, 50, 0)

	assert.ErrorIs(t, err, domain.ErrForbidden)
	leadRepo.AssertNotCalled(t, "ListForApprovalQueue",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	leadRepo.AssertNotCalled(t, "CountApprovalQueueByFunnel", mock.Anything, mock.Anything)
}

// The regression from the 2026-09-09 audit: a connected mailbox receives the
// company's own paperwork, and every one of those messages was minting a
// prospect that "responded". Production had Stripe, GoDaddy and a domain
// renewal as four of the five replies the account had ever earned.
func TestInbound_MarkLeadResponded_DoesNotMintALeadForMachineMail(t *testing.T) {
	cases := []struct {
		name   string
		sender string
		dto    InboundMessageDTO
	}{
		{"stripe receipts", "notifications@stripe.com", InboundMessageDTO{Subject: "Your receipt"}},
		{"billing mailbox", "billing@titan.email", InboundMessageDTO{Subject: "Renovação"}},
		{"tagged no-reply", "no-reply+xyz@vendor.com", InboundMessageDTO{}},
		{"bulk sender header", "hello@vendor.com", InboundMessageDTO{ListUnsubscribe: true}},
		{"newsletter prose", "hello@vendor.com", InboundMessageDTO{
			Subject: "Novidades", Content: "Você está recebendo porque assinou. Para descadastrar, clique."}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			leadRepo := new(mockLeadRepo)
			funnelRepo := new(MockSalesFunnelRepo)
			uc := newInboundHookUC(funnelRepo, leadRepo)

			ch := &domain.Channel{ID: "ch-1", AccountID: "acc-1"}
			funnelRepo.On("GetByChannelID", mock.Anything, "ch-1").
				Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil)
			leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", mock.Anything).
				Return(nil, nil)

			uc.markLeadResponded(context.Background(), ch,
				&domain.Contact{ID: "ct-1"}, tc.sender, tc.dto)

			leadRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
		})
	}
}

// The other direction, which matters just as much: a real person writing in
// cold IS a lead, and asking for a boleto is what a hospital that wants to
// buy actually writes.
func TestInbound_MarkLeadResponded_StillMintsALeadForAPerson(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	uc := newInboundHookUC(funnelRepo, leadRepo)

	ch := &domain.Channel{ID: "ch-1", AccountID: "acc-1"}
	funnelRepo.On("GetByChannelID", mock.Anything, "ch-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", mock.Anything).
		Return(nil, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	uc.markLeadResponded(context.Background(), ch, &domain.Contact{ID: "ct-1"},
		"compras@hospital.com.br",
		InboundMessageDTO{Subject: "Cotação", Content: "Me manda o boleto da nota fiscal, por favor."})

	leadRepo.AssertCalled(t, "Upsert", mock.Anything, mock.Anything)
}

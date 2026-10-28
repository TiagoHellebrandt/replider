package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockFunnelRepo is a testify mock for domain.SalesFunnelRepository scoped to
// the MCP sales-manage tool tests.
type mockFunnelRepo struct{ mock.Mock }

func (m *mockFunnelRepo) Create(ctx context.Context, funnel *domain.SalesFunnel) error {
	return m.Called(ctx, funnel).Error(0)
}

func (m *mockFunnelRepo) GetByID(ctx context.Context, id string) (*domain.SalesFunnel, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SalesFunnel), args.Error(1)
}

func (m *mockFunnelRepo) GetByChannelID(ctx context.Context, channelID string) (*domain.SalesFunnel, error) {
	args := m.Called(ctx, channelID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SalesFunnel), args.Error(1)
}

func (m *mockFunnelRepo) ListByAccountID(ctx context.Context, accountID string) ([]*domain.SalesFunnel, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SalesFunnel), args.Error(1)
}

func (m *mockFunnelRepo) Update(ctx context.Context, funnel *domain.SalesFunnel) error {
	return m.Called(ctx, funnel).Error(0)
}

func (m *mockFunnelRepo) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

func (m *mockFunnelRepo) EnsureDefault(ctx context.Context, accountID string) (*domain.SalesFunnel, error) {
	args := m.Called(ctx, accountID)
	if f, _ := args.Get(0).(*domain.SalesFunnel); f != nil {
		return f, args.Error(1)
	}
	return nil, args.Error(1)
}

// mockLeadRepoMCP is a testify mock for domain.LeadRepository.
type mockLeadRepoMCP struct{ mock.Mock }

func (m *mockLeadRepoMCP) Upsert(ctx context.Context, lead *domain.Lead) error {
	return m.Called(ctx, lead).Error(0)
}

func (m *mockLeadRepoMCP) GetByFunnelAndContact(ctx context.Context, funnelID, contactID string) (*domain.Lead, error) {
	args := m.Called(ctx, funnelID, contactID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Lead), args.Error(1)
}

func (m *mockLeadRepoMCP) GetByID(ctx context.Context, id string) (*domain.Lead, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Lead), args.Error(1)
}

func (m *mockLeadRepoMCP) ListForApprovalQueue(ctx context.Context, f domain.ApprovalQueueFilter, limit, offset int32) ([]*domain.Lead, error) {
	args := m.Called(ctx, f, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Lead), args.Error(1)
}

func (m *mockLeadRepoMCP) CountApprovalQueueByFunnel(ctx context.Context, f domain.ApprovalQueueFilter) ([]domain.QueueFunnelCount, error) {
	args := m.Called(ctx, f)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.QueueFunnelCount), args.Error(1)
}

func (m *mockLeadRepoMCP) CountByStatus(ctx context.Context, accountID string) (map[domain.LeadStatus]int64, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[domain.LeadStatus]int64), args.Error(1)
}

func (m *mockLeadRepoMCP) CountByStatusForOwner(ctx context.Context, accountID, ownerUserID string) (map[domain.LeadStatus]int64, error) {
	args := m.Called(ctx, accountID, ownerUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[domain.LeadStatus]int64), args.Error(1)
}

func (m *mockLeadRepoMCP) CountUnowned(ctx context.Context, accountID string) (int64, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockLeadRepoMCP) ListByFunnel(ctx context.Context, funnelID string, status domain.LeadStatus, limit, offset int32) ([]*domain.Lead, error) {
	args := m.Called(ctx, funnelID, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Lead), args.Error(1)
}

func (m *mockLeadRepoMCP) UpdateMetadata(ctx context.Context, id string, collectedMetadata, enrichment json.RawMessage) error {
	return m.Called(ctx, id, collectedMetadata, enrichment).Error(0)
}

func (m *mockLeadRepoMCP) UpdateStatus(ctx context.Context, id string, status domain.LeadStatus) error {
	return m.Called(ctx, id, status).Error(0)
}

func (m *mockLeadRepoMCP) MarkBuyIntent(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

func (m *mockLeadRepoMCP) MarkQualified(ctx context.Context, id string) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

// mockContactRepoMCP is a testify mock for domain.ContactRepository.
type mockContactRepoMCP struct{ mock.Mock }

func (m *mockContactRepoMCP) Upsert(ctx context.Context, c *domain.Contact) (*domain.Contact, error) {
	args := m.Called(ctx, c)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Contact), args.Error(1)
}

func (m *mockContactRepoMCP) ListByIDs(ctx context.Context, accountID string, ids []string) ([]*domain.Contact, error) {
	args := m.Called(ctx, accountID, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Contact), args.Error(1)
}

func (m *mockContactRepoMCP) GetByID(ctx context.Context, id, accountID string) (*domain.Contact, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Contact), args.Error(1)
}

func (m *mockContactRepoMCP) GetByExternalID(ctx context.Context, accountID, externalID string) (*domain.Contact, error) {
	args := m.Called(ctx, accountID, externalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Contact), args.Error(1)
}

func (m *mockContactRepoMCP) ListByAccountID(ctx context.Context, accountID, query string, limit, offset int) ([]*domain.Contact, error) {
	args := m.Called(ctx, accountID, query, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Contact), args.Error(1)
}

func (m *mockContactRepoMCP) Update(ctx context.Context, c *domain.Contact) error {
	return m.Called(ctx, c).Error(0)
}

func (m *mockContactRepoMCP) SetOptedOut(ctx context.Context, accountID, externalID string) (bool, error) {
	args := m.Called(ctx, accountID, externalID)
	return args.Bool(0), args.Error(1)
}

func (m *mockContactRepoMCP) ClearOptedOut(ctx context.Context, accountID, externalID string) (bool, error) {
	args := m.Called(ctx, accountID, externalID)
	return args.Bool(0), args.Error(1)
}

func (m *mockContactRepoMCP) ListOptedOutExternalIDs(ctx context.Context, accountID string, externalIDs []string) (map[string]bool, error) {
	args := m.Called(ctx, accountID, externalIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]bool), args.Error(1)
}

func (m *mockContactRepoMCP) IsOptedOut(ctx context.Context, accountID, externalID string) (bool, error) {
	args := m.Called(ctx, accountID, externalID)
	return args.Bool(0), args.Error(1)
}

func identityCtx() context.Context {
	return WithIdentity(context.Background(), Identity{AccountID: "acc-1", UserID: "u1", Role: "admin"})
}

func TestHandleUpsertFunnel_CreateHappyPath(t *testing.T) {
	repo := new(mockFunnelRepo)
	repo.On("Create", mock.Anything, mock.MatchedBy(func(f *domain.SalesFunnel) bool {
		return f.AccountID == "acc-1" && f.Name == "Funil MCP" &&
			f.WebhookSecret == "s3cr3t-hmac" && f.Active
	})).Return(nil)

	out, err := handleUpsertFunnel(identityCtx(), Deps{FunnelUC: usecases.NewFunnelUseCase(repo)}, upsertFunnelInput{
		Name:          "Funil MCP",
		PersonaPrompt: "vendedor consultivo",
		MetadataSchema: []metadataFieldInput{
			{Key: "empresa", Label: "Empresa", Type: "text", Required: true},
			{Key: "porte", Label: "Porte", Type: "enum", EnumOptions: []string{"pequena", "média"}},
		},
		WebhookURL:                "https://example.com/hook",
		WebhookSecret:             "s3cr3t-hmac",
		TriggerOnRequiredComplete: true,
	})

	require.NoError(t, err)
	require.NotNil(t, out)
	assert.NotEmpty(t, out.ID)
	assert.Equal(t, "Funil MCP", out.Name)
	assert.True(t, out.Active)
	assert.True(t, out.HasWebhookSecret)
	assert.True(t, out.TriggerOnRequiredComplete)

	// The webhook secret must never be echoed back to the MCP client.
	blob, merr := json.Marshal(out)
	require.NoError(t, merr)
	assert.NotContains(t, string(blob), "s3cr3t-hmac")

	// Schema round-trips through the use case's validation/normalization.
	// It travels DECODED (see funnelDetail.MetadataSchema), so re-marshal it
	// to read it back as the typed definitions.
	schemaBlob, serr := json.Marshal(out.MetadataSchema)
	require.NoError(t, serr)
	var fields []domain.MetadataFieldDef
	require.NoError(t, json.Unmarshal(schemaBlob, &fields))
	assert.Len(t, fields, 2)
	repo.AssertExpectations(t)
}

func TestHandleUpsertFunnel_UpdateMismatchedAccountUnauthorized(t *testing.T) {
	repo := new(mockFunnelRepo)
	repo.On("GetByID", mock.Anything, "fn-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "other-acc"}, nil)

	out, err := handleUpsertFunnel(identityCtx(), Deps{FunnelUC: usecases.NewFunnelUseCase(repo)}, upsertFunnelInput{
		FunnelID: "fn-1",
		Name:     "Tentativa cross-tenant",
	})

	assert.Nil(t, out)
	require.Error(t, err)
	// AUTH_UNAUTHORIZED maps to the pt-BR tool error, never leaking the funnel.
	assert.Contains(t, err.Error(), "Sem permissão")
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestHandleUpsertFunnel_UpdatePreservesStoredSecret(t *testing.T) {
	repo := new(mockFunnelRepo)
	repo.On("GetByID", mock.Anything, "fn-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1", Name: "Antigo", WebhookSecret: "stored-secret"}, nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(f *domain.SalesFunnel) bool {
		return f.ID == "fn-1" && f.Name == "Novo nome" && f.WebhookSecret == "stored-secret"
	})).Return(nil)

	out, err := handleUpsertFunnel(identityCtx(), Deps{FunnelUC: usecases.NewFunnelUseCase(repo)}, upsertFunnelInput{
		FunnelID: "fn-1",
		Name:     "Novo nome",
		// WebhookSecret deliberately empty: must keep the stored secret.
	})

	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, "Novo nome", out.Name)
	assert.True(t, out.HasWebhookSecret)
	repo.AssertExpectations(t)
}

func TestHandleCreateLead_HappyPath(t *testing.T) {
	funnelRepo := new(mockFunnelRepo)
	leadRepo := new(mockLeadRepoMCP)
	contactRepo := new(mockContactRepoMCP)

	funnelRepo.On("GetByID", mock.Anything, "fn-1").Return(&domain.SalesFunnel{
		ID:             "fn-1",
		AccountID:      "acc-1",
		MetadataSchema: json.RawMessage(`[{"key":"empresa","label":"Empresa","type":"text","required":true}]`),
	}, nil)
	// An unknown number carries the typed name through to the contact.
	// (A number we already know keeps ITS name — see the sibling test.)
	// The repo reports "no such contact" as an error, not (nil, nil).
	// One customer, one key (2026-09-01): the resolver probes every
	// spelling of the number before minting; none of them exists here.
	contactRepo.On("GetByExternalID", mock.Anything, "acc-1", mock.Anything).
		Return(nil, errors.New("no rows in result set"))
	// US-F005: a new contact is minted with an opaque ULID; the phone stays
	// as external_id (the key Upsert dedupes on), so the id is no longer
	// predictable from the phone.
	contactRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(c *domain.Contact) bool {
		// The canonical key is the digits-only E.164 (domain.ContactKey).
		return len(c.ID) == 26 && c.AccountID == "acc-1" &&
			c.ExternalID == "5565999998888" && c.Name != nil && *c.Name == "Padaria Central"
	})).Return(&domain.Contact{ID: "01HZZZZZZZZZZZZZZZZZZZZZZZ", AccountID: "acc-1", ExternalID: "5565999998888"}, nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", "01HZZZZZZZZZZZZZZZZZZZZZZZ").Return(nil, nil)
	leadRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(l *domain.Lead) bool {
		return l.AccountID == "acc-1" && l.FunnelID == "fn-1" &&
			l.ContactID == "01HZZZZZZZZZZZZZZZZZZZZZZZ" && l.Status == domain.LeadStatusNew &&
			l.Source == "manual"
	})).Return(nil)

	leadUC := usecases.NewLeadUseCase(leadRepo, funnelRepo)
	leadUC.SetContactRepo(contactRepo)

	out, err := handleCreateLead(identityCtx(), Deps{LeadUC: leadUC}, createLeadInput{
		FunnelID: "fn-1",
		Name:     "Padaria Central",
		Phone:    "+55 (65) 99999-8888",
		Metadata: map[string]string{"empresa": "Padaria Central"},
	})

	require.NoError(t, err)
	require.NotNil(t, out)
	assert.NotEmpty(t, out.LeadID)
	assert.Equal(t, string(domain.LeadStatusNew), out.Status)
	// contact_id is now the opaque contact id; the phone is echoed separately.
	assert.Equal(t, "01HZZZZZZZZZZZZZZZZZZZZZZZ", out.ContactID)
	assert.Equal(t, "+5565999998888", out.Phone)
	assert.True(t, out.Created)
	funnelRepo.AssertExpectations(t)
	contactRepo.AssertExpectations(t)
	leadRepo.AssertExpectations(t)
}

func TestHandleCreateLead_InvalidPhoneRejected(t *testing.T) {
	funnelRepo := new(mockFunnelRepo)
	leadRepo := new(mockLeadRepoMCP)
	contactRepo := new(mockContactRepoMCP)

	funnelRepo.On("GetByID", mock.Anything, "fn-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil)

	leadUC := usecases.NewLeadUseCase(leadRepo, funnelRepo)
	leadUC.SetContactRepo(contactRepo)

	out, err := handleCreateLead(identityCtx(), Deps{LeadUC: leadUC}, createLeadInput{
		FunnelID: "fn-1",
		Phone:    "não é telefone",
	})

	assert.Nil(t, out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "E.164")
	contactRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
	leadRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}

func TestHandleCreateLead_ExistingLeadIsIdempotent(t *testing.T) {
	funnelRepo := new(mockFunnelRepo)
	leadRepo := new(mockLeadRepoMCP)
	contactRepo := new(mockContactRepoMCP)

	funnelRepo.On("GetByID", mock.Anything, "fn-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil)
	// The resolver probes the number's spellings before minting.
	contactRepo.On("GetByExternalID", mock.Anything, "acc-1", mock.Anything).Return(nil, nil).Maybe()
	contactRepo.On("Upsert", mock.Anything, mock.Anything).
		Return(&domain.Contact{ID: "+5565999998888", AccountID: "acc-1", ExternalID: "+5565999998888"}, nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", "+5565999998888").
		Return(&domain.Lead{ID: "lead-existing", AccountID: "acc-1", FunnelID: "fn-1", ContactID: "+5565999998888", Status: domain.LeadStatusContacted}, nil)

	leadUC := usecases.NewLeadUseCase(leadRepo, funnelRepo)
	leadUC.SetContactRepo(contactRepo)

	out, err := handleCreateLead(identityCtx(), Deps{LeadUC: leadUC}, createLeadInput{
		FunnelID: "fn-1",
		Phone:    "+5565999998888",
	})

	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, "lead-existing", out.LeadID)
	assert.Equal(t, string(domain.LeadStatusContacted), out.Status)
	assert.False(t, out.Created)
	// The existing lead's pipeline state must never be reset by a re-create.
	leadRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}

// US-F009: ownership-aware reads/writes.
func (m *mockLeadRepoMCP) SetVisibility(ctx context.Context, accountID, id string, v domain.ResourceVisibility) (*domain.Lead, error) {
	args := m.Called(ctx, accountID, id, v)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Lead), args.Error(1)
}

func (m *mockLeadRepoMCP) AssignOwner(ctx context.Context, accountID, id string, ownerUserID *string) (*domain.Lead, error) {
	args := m.Called(ctx, accountID, id, ownerUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Lead), args.Error(1)
}

func (m *mockLeadRepoMCP) ListByFunnelForOwner(ctx context.Context, accountID, funnelID, ownerUserID string, status domain.LeadStatus, limit, offset int32) ([]*domain.Lead, error) {
	args := m.Called(ctx, accountID, funnelID, ownerUserID, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Lead), args.Error(1)
}

// A number we already know keeps ITS name. The contact's name is what the
// board shows and what the send confirmation quotes, so letting a second
// registration overwrite it renames a company inside an approval that is
// already waiting on a human.
func TestHandleCreateLead_DoesNotRenameAKnownContact(t *testing.T) {
	funnelRepo := new(mockFunnelRepo)
	leadRepo := new(mockLeadRepoMCP)
	contactRepo := new(mockContactRepoMCP)

	known := "Padaria Central"
	funnelRepo.On("GetByID", mock.Anything, "fn-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil)
	// Any spelling of the number resolves to the SAME known contact.
	contactRepo.On("GetByExternalID", mock.Anything, "acc-1", mock.MatchedBy(func(k string) bool {
		return strings.Contains(k, "5565999998888")
	})).Return(&domain.Contact{
		ID: "01HZZZZZZZZZZZZZZZZZZZZZZZ", AccountID: "acc-1",
		ExternalID: "+5565999998888", Name: &known,
	}, nil)
	contactRepo.On("GetByExternalID", mock.Anything, "acc-1", mock.Anything).Return(nil, nil).Maybe()
	// The resolver completes the known contact (a phone it lacked) — and the
	// known NAME must survive that write.
	contactRepo.On("Update", mock.Anything, mock.MatchedBy(func(c *domain.Contact) bool {
		return c.Name != nil && *c.Name == known
	})).Return(nil).Maybe()
	// The known name must survive: a write, if any, never carries a
	// different name for this contact.
	contactRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(c *domain.Contact) bool {
		return c.Name == nil || *c.Name == known
	})).Maybe().Return(&domain.Contact{
		ID: "01HZZZZZZZZZZZZZZZZZZZZZZZ", AccountID: "acc-1",
		ExternalID: "+5565999998888", Name: &known,
	}, nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", "01HZZZZZZZZZZZZZZZZZZZZZZZ").
		Return(nil, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	leadUC := usecases.NewLeadUseCase(leadRepo, funnelRepo)
	leadUC.SetContactRepo(contactRepo)

	_, err := handleCreateLead(identityCtx(), Deps{LeadUC: leadUC}, createLeadInput{
		FunnelID: "fn-1",
		Name:     "Typo Ltda",
		Phone:    "+5565999998888",
	})

	require.NoError(t, err)
	contactRepo.AssertExpectations(t)
}

// ...but a contact we know only as a number DOES get named: filling a blank
// is an improvement, not an overwrite.
func TestHandleCreateLead_NamesAnUnnamedContact(t *testing.T) {
	funnelRepo := new(mockFunnelRepo)
	leadRepo := new(mockLeadRepoMCP)
	contactRepo := new(mockContactRepoMCP)

	funnelRepo.On("GetByID", mock.Anything, "fn-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil)
	// Any spelling of the number finds the same (still unnamed) contact.
	contactRepo.On("GetByExternalID", mock.Anything, "acc-1", mock.MatchedBy(func(k string) bool {
		return strings.Contains(k, "5565999998888")
	})).Return(&domain.Contact{
		ID: "01HZZZZZZZZZZZZZZZZZZZZZZZ", AccountID: "acc-1",
		ExternalID: "+5565999998888",
	}, nil)
	contactRepo.On("GetByExternalID", mock.Anything, "acc-1", mock.Anything).Return(nil, nil).Maybe()
	// The resolver COMPLETES a found contact: the typed name fills the
	// blank one through Update (never a second Upsert row).
	contactRepo.On("Update", mock.Anything, mock.MatchedBy(func(c *domain.Contact) bool {
		return c.ID == "01HZZZZZZZZZZZZZZZZZZZZZZZ" && c.Name != nil && *c.Name == "Padaria Central"
	})).Return(nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", "01HZZZZZZZZZZZZZZZZZZZZZZZ").
		Return(nil, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	leadUC := usecases.NewLeadUseCase(leadRepo, funnelRepo)
	leadUC.SetContactRepo(contactRepo)

	_, err := handleCreateLead(identityCtx(), Deps{LeadUC: leadUC}, createLeadInput{
		FunnelID: "fn-1",
		Name:     "Padaria Central",
		Phone:    "+5565999998888",
	})

	require.NoError(t, err)
	contactRepo.AssertCalled(t, "Update", mock.Anything, mock.Anything)
	contactRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}

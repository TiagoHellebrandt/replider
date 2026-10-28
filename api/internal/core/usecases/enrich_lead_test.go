package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fakeEnrichmentFactory returns a fixed provider or error, standing in for the
// account-BYOK-aware GeminiEnrichmentFactory.
type fakeEnrichmentFactory struct {
	provider domain.LeadEnrichmentProvider
	err      error
}

func (f *fakeEnrichmentFactory) ProviderForAccount(_ context.Context, _ *domain.Account, _ *domain.Plan) (domain.LeadEnrichmentProvider, error) {
	return f.provider, f.err
}

// fakeEnrichmentProvider returns canned enrichment data / error.
type fakeEnrichmentProvider struct {
	data domain.EnrichmentData
	err  error
}

func (p *fakeEnrichmentProvider) Enrich(_ context.Context, _ *domain.Lead) (domain.EnrichmentData, error) {
	return p.data, p.err
}

func newEnrichTestUC(t *testing.T, factory domain.LeadEnrichmentFactory) (*EnrichLeadUseCase, *mockLeadRepo, *MockContactRepo) {
	t.Helper()
	leadRepo := &mockLeadRepo{}
	contactRepo := &MockContactRepo{}
	accountRepo := &MockAccountRepo{}
	planRepo := &MockPlanRepo{}

	accountRepo.On("GetByID", mock.Anything, "acc-1").Return(&domain.Account{ID: "acc-1"}, nil)

	uc := NewEnrichLeadUseCase(leadRepo, contactRepo, accountRepo, planRepo, factory)
	// Run the background work synchronously so assertions are deterministic.
	uc.dispatch = func(f func()) { f() }
	return uc, leadRepo, contactRepo
}

func TestEnrichLead_RequiresGemini(t *testing.T) {
	factory := &fakeEnrichmentFactory{err: domain.ErrEnrichmentRequiresGemini}
	uc, leadRepo, _ := newEnrichTestUC(t, factory)
	leadRepo.On("GetByID", mock.Anything, "lead-1").Return(&domain.Lead{ID: "lead-1", AccountID: "acc-1", Status: domain.LeadStatusProspected}, nil)

	err := uc.Start(context.Background(), "acc-1", "lead-1")

	assert.ErrorIs(t, err, domain.ErrEnrichmentRequiresGemini)
	// The lead must NOT be flipped to enriching when BYOK gate fails.
	leadRepo.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything)
}

func TestEnrichLead_OwnershipDenied(t *testing.T) {
	factory := &fakeEnrichmentFactory{provider: &fakeEnrichmentProvider{}}
	uc, leadRepo, _ := newEnrichTestUC(t, factory)
	leadRepo.On("GetByID", mock.Anything, "lead-1").Return(&domain.Lead{ID: "lead-1", AccountID: "other-acc"}, nil)

	err := uc.Start(context.Background(), "acc-1", "lead-1")

	assert.ErrorIs(t, err, domain.ErrLeadNotFound)
}

func TestEnrichLead_HappyPathPopulatesLinks(t *testing.T) {
	data := domain.EnrichmentData{
		DecisionMakerName: "Maria Silva",
		DecisionMakerRole: "Sócia",
		LinkedInURL:       "https://linkedin.com/in/maria",
		InstagramURL:      "https://instagram.com/maria",
	}
	factory := &fakeEnrichmentFactory{provider: &fakeEnrichmentProvider{data: data}}
	uc, leadRepo, contactRepo := newEnrichTestUC(t, factory)

	name := "Padaria Pão Quente"
	// US-F005: the lookup is by contact id first; these fixtures still use
	// the legacy shape (contact_id == phone), which the fallback covers.
	contactRepo.On("GetByID", mock.Anything, "+5511999999999", "acc-1").Return(nil, nil)
	contactRepo.On("GetByExternalID", mock.Anything, "acc-1", "+5511999999999").
		Return(&domain.Contact{ID: "c1", Name: &name}, nil)
	leadRepo.On("GetByID", mock.Anything, "lead-1").Return(&domain.Lead{
		ID: "lead-1", AccountID: "acc-1", ContactID: "+5511999999999",
		Status: domain.LeadStatusProspected, Enrichment: json.RawMessage(`{"website":"https://paoquente.com"}`),
	}, nil)
	leadRepo.On("UpdateStatus", mock.Anything, "lead-1", domain.LeadStatusEnriching).Return(nil)

	var saved *domain.Lead
	leadRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(l *domain.Lead) bool {
		saved = l
		return true
	})).Return(nil)

	err := uc.Start(context.Background(), "acc-1", "lead-1")
	require.NoError(t, err)

	require.NotNil(t, saved)
	assert.Equal(t, domain.LeadStatusEnriched, saved.Status)

	var m map[string]any
	require.NoError(t, json.Unmarshal(saved.Enrichment, &m))
	assert.Equal(t, "https://linkedin.com/in/maria", m["linkedin_url"])
	assert.Equal(t, "https://instagram.com/maria", m["instagram_url"])
	assert.Equal(t, "Maria Silva", m["decision_maker_name"])
	// Prospecting seed preserved + company injected.
	assert.Equal(t, "https://paoquente.com", m["website"])
	assert.Equal(t, "Padaria Pão Quente", m["company_name"])
}

func TestEnrichLead_EmptyResultStillEnriches(t *testing.T) {
	// Best-effort: a sparse answer (no decision maker found) is NOT an error
	// and still moves the lead to enriched.
	factory := &fakeEnrichmentFactory{provider: &fakeEnrichmentProvider{data: domain.EnrichmentData{}}}
	uc, leadRepo, contactRepo := newEnrichTestUC(t, factory)

	contactRepo.On("GetByID", mock.Anything, "+5511999999999", "acc-1").Return(nil, nil)
	contactRepo.On("GetByExternalID", mock.Anything, "acc-1", "+5511999999999").Return(nil, nil)
	leadRepo.On("GetByID", mock.Anything, "lead-1").Return(&domain.Lead{
		ID: "lead-1", AccountID: "acc-1", ContactID: "+5511999999999",
		Status: domain.LeadStatusProspected, Enrichment: json.RawMessage(`{"website":"https://x.com"}`),
	}, nil)
	leadRepo.On("UpdateStatus", mock.Anything, "lead-1", domain.LeadStatusEnriching).Return(nil)

	var saved *domain.Lead
	leadRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(l *domain.Lead) bool { saved = l; return true })).Return(nil)

	err := uc.Start(context.Background(), "acc-1", "lead-1")
	require.NoError(t, err)

	require.NotNil(t, saved)
	assert.Equal(t, domain.LeadStatusEnriched, saved.Status)
	var m map[string]any
	require.NoError(t, json.Unmarshal(saved.Enrichment, &m))
	_, hasLinkedIn := m["linkedin_url"]
	assert.False(t, hasLinkedIn, "no link should be set when none found")
}

func TestEnrichLead_ProviderErrorRestoresStatus(t *testing.T) {
	factory := &fakeEnrichmentFactory{provider: &fakeEnrichmentProvider{err: errors.New("boom")}}
	uc, leadRepo, contactRepo := newEnrichTestUC(t, factory)

	contactRepo.On("GetByID", mock.Anything, "+5511999999999", "acc-1").Return(nil, nil)
	contactRepo.On("GetByExternalID", mock.Anything, "acc-1", "+5511999999999").Return(nil, nil)
	leadRepo.On("GetByID", mock.Anything, "lead-1").Return(&domain.Lead{
		ID: "lead-1", AccountID: "acc-1", ContactID: "+5511999999999",
		Status: domain.LeadStatusProspected,
	}, nil)
	leadRepo.On("UpdateStatus", mock.Anything, "lead-1", domain.LeadStatusEnriching).Return(nil)
	// On failure the prior status (prospected) is restored, never Upsert.
	leadRepo.On("UpdateStatus", mock.Anything, "lead-1", domain.LeadStatusProspected).Return(nil)

	err := uc.Start(context.Background(), "acc-1", "lead-1")
	require.NoError(t, err)

	leadRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
	leadRepo.AssertCalled(t, "UpdateStatus", mock.Anything, "lead-1", domain.LeadStatusProspected)
}

// A lead with no company name and no website has nothing to search FOR. The
// adapter skips the call, and the lead must not come out labelled `enriched`
// — the board would claim a research that never ran. The reason is recorded on
// the lead so the panel can say it instead of showing the never-researched
// copy beside a button that would do nothing again.
func TestEnrichLead_NothingToGroundRecordsReasonAndKeepsStatus(t *testing.T) {
	factory := &fakeEnrichmentFactory{provider: &fakeEnrichmentProvider{
		err: domain.ErrEnrichmentNothingToGround,
	}}
	uc, leadRepo, contactRepo := newEnrichTestUC(t, factory)

	contactRepo.On("GetByID", mock.Anything, "+5511999999999", "acc-1").Return(nil, nil)
	contactRepo.On("GetByExternalID", mock.Anything, "acc-1", "+5511999999999").Return(nil, nil)
	leadRepo.On("GetByID", mock.Anything, "lead-1").Return(&domain.Lead{
		ID: "lead-1", AccountID: "acc-1", ContactID: "+5511999999999",
		Status: domain.LeadStatusProspected,
	}, nil)
	leadRepo.On("UpdateStatus", mock.Anything, "lead-1", domain.LeadStatusEnriching).Return(nil)

	var saved *domain.Lead
	leadRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(l *domain.Lead) bool { saved = l; return true })).Return(nil)

	require.NoError(t, uc.Start(context.Background(), "acc-1", "lead-1"))

	require.NotNil(t, saved)
	assert.Equal(t, domain.LeadStatusProspected, saved.Status, "a lead nobody researched is not enriched")
	var m map[string]any
	require.NoError(t, json.Unmarshal(saved.Enrichment, &m))
	assert.Equal(t, domain.EnrichmentBlockedNoIdentity, m[domain.EnrichmentBlockedKey])
}

// Once a real search runs, the old excuse must go — otherwise the panel keeps
// explaining a block that no longer applies.
func TestEnrichLead_SuccessClearsBlockedReason(t *testing.T) {
	factory := &fakeEnrichmentFactory{provider: &fakeEnrichmentProvider{
		data: domain.EnrichmentData{DecisionMakerName: "Ana"},
	}}
	uc, leadRepo, contactRepo := newEnrichTestUC(t, factory)

	contactRepo.On("GetByID", mock.Anything, "+5511999999999", "acc-1").Return(nil, nil)
	contactRepo.On("GetByExternalID", mock.Anything, "acc-1", "+5511999999999").Return(nil, nil)
	leadRepo.On("GetByID", mock.Anything, "lead-1").Return(&domain.Lead{
		ID: "lead-1", AccountID: "acc-1", ContactID: "+5511999999999",
		Status:     domain.LeadStatusProspected,
		Enrichment: json.RawMessage(`{"company_name":"X","enrich_blocked":"no_company_or_website"}`),
	}, nil)
	leadRepo.On("UpdateStatus", mock.Anything, "lead-1", domain.LeadStatusEnriching).Return(nil)

	var saved *domain.Lead
	leadRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(l *domain.Lead) bool { saved = l; return true })).Return(nil)

	require.NoError(t, uc.Start(context.Background(), "acc-1", "lead-1"))

	require.NotNil(t, saved)
	var m map[string]any
	require.NoError(t, json.Unmarshal(saved.Enrichment, &m))
	_, blocked := m[domain.EnrichmentBlockedKey]
	assert.False(t, blocked, "a search that ran clears the reason the previous one did not")
}

// The findings ARE the deeper research. Persisted with their own sources, or the
// dossier is back to attributing every fact to the whole bibliography.
func TestEnrichLead_PersistsFindingsWithTheirSources(t *testing.T) {
	data := domain.EnrichmentData{
		DecisionMakerName: "Ana Souza",
		Findings: []domain.EnrichmentFinding{
			{Field: "decision_maker_name", Value: "Ana Souza",
				Source: "https://www.linkedin.com/in/ana", Confident: true},
			{Field: "what_they_do", Value: "Clínica de fisioterapia",
				Source: "https://clinica.com.br/sobre", Confident: true},
		},
		Sources: []string{"https://www.linkedin.com/in/ana"},
	}
	factory := &fakeEnrichmentFactory{provider: &fakeEnrichmentProvider{data: data}}
	uc, leadRepo, contactRepo := newEnrichTestUC(t, factory)

	contactRepo.On("GetByID", mock.Anything, "+5511999999999", "acc-1").Return(nil, nil)
	contactRepo.On("GetByExternalID", mock.Anything, "acc-1", "+5511999999999").
		Return(nil, nil)
	leadRepo.On("GetByID", mock.Anything, "lead-1").Return(&domain.Lead{
		ID: "lead-1", AccountID: "acc-1", ContactID: "+5511999999999",
		Status:     domain.LeadStatusProspected,
		Enrichment: json.RawMessage(`{"website":"https://clinica.com.br"}`),
	}, nil)
	leadRepo.On("UpdateStatus", mock.Anything, "lead-1", domain.LeadStatusEnriching).Return(nil)

	var saved *domain.Lead
	leadRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(l *domain.Lead) bool {
		saved = l
		return true
	})).Return(nil)

	require.NoError(t, uc.Start(context.Background(), "acc-1", "lead-1"))
	require.NotNil(t, saved)

	var m map[string]any
	require.NoError(t, json.Unmarshal(saved.Enrichment, &m))
	findings, ok := m["findings"].([]any)
	require.True(t, ok, "findings must survive the merge")
	require.Len(t, findings, 2)
	first, _ := findings[0].(map[string]any)
	assert.Equal(t, "https://www.linkedin.com/in/ana", first["source"])
	assert.Equal(t, true, first["confident"])
	// The prospecting seed is still there — a re-search overwrites the reading,
	// never the lead's identity.
	assert.Equal(t, "https://clinica.com.br", m["website"])
}

// A re-search REPLACES the previous reading. Merging two runs would leave a fact
// from six months ago next to the one that replaced it, with nothing to tell
// them apart.
func TestEnrichLead_ASecondSearchReplacesTheOldFindings(t *testing.T) {
	data := domain.EnrichmentData{
		Findings: []domain.EnrichmentFinding{
			{Field: "current_supplier", Value: "Distribuidora Nova",
				Source: "https://x.com.br", Confident: true},
		},
	}
	factory := &fakeEnrichmentFactory{provider: &fakeEnrichmentProvider{data: data}}
	uc, leadRepo, contactRepo := newEnrichTestUC(t, factory)

	contactRepo.On("GetByID", mock.Anything, "+5511999999999", "acc-1").Return(nil, nil)
	contactRepo.On("GetByExternalID", mock.Anything, "acc-1", "+5511999999999").
		Return(nil, nil)
	leadRepo.On("GetByID", mock.Anything, "lead-1").Return(&domain.Lead{
		ID: "lead-1", AccountID: "acc-1", ContactID: "+5511999999999",
		Status: domain.LeadStatusEnriched,
		Enrichment: json.RawMessage(
			`{"findings":[{"field":"current_supplier","value":"Distribuidora Antiga"}]}`),
	}, nil)
	leadRepo.On("UpdateStatus", mock.Anything, "lead-1", domain.LeadStatusEnriching).Return(nil)

	var saved *domain.Lead
	leadRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(l *domain.Lead) bool {
		saved = l
		return true
	})).Return(nil)

	require.NoError(t, uc.Start(context.Background(), "acc-1", "lead-1"))

	var m map[string]any
	require.NoError(t, json.Unmarshal(saved.Enrichment, &m))
	findings, _ := m["findings"].([]any)
	require.Len(t, findings, 1)
	first, _ := findings[0].(map[string]any)
	assert.Equal(t, "Distribuidora Nova", first["value"])
}

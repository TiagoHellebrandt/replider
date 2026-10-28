package services

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fakeFunnelDispatcher records EmitToEndpoint calls so tests can assert exactly
// which funnel webhook events fired and with what payload, without any HTTP.
type fakeFunnelDispatcher struct {
	mu     sync.Mutex
	events []fakeDispatch
}

type fakeDispatch struct {
	rateKey, accountID, url, secret, event string
	payload                                any
}

func (f *fakeFunnelDispatcher) EmitToEndpoint(rateKey, accountID, url, secret, event string, payload any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, fakeDispatch{rateKey, accountID, url, secret, event, payload})
}

func (f *fakeFunnelDispatcher) count(event string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, e := range f.events {
		if e.event == event {
			n++
		}
	}
	return n
}

// leadWith builds a lead whose collected_metadata holds the given key→value
// pairs (all marked source:ai), mirroring how RecordLeadData persists them.
func leadWith(values map[string]string) *domain.Lead {
	collected := map[string]CollectedField{}
	for k, v := range values {
		collected[k] = CollectedField{Value: v, Source: CollectedSourceAI, UpdatedAt: time.Now()}
	}
	blob, _ := json.Marshal(collected)
	return &domain.Lead{
		ID:                "lead_1",
		AccountID:         "acc_1",
		FunnelID:          "fun_1",
		ContactID:         "+5511999999999",
		Status:            domain.LeadStatusResponded,
		CollectedMetadata: blob,
	}
}

func qualifyingFunnel() *domain.SalesFunnel {
	return &domain.SalesFunnel{
		ID:                        "fun_1",
		AccountID:                 "acc_1",
		WebhookURL:                "https://crm.example.com/hook",
		WebhookSecret:             "shh",
		TriggerOnRequiredComplete: true,
		TriggerOnBuyIntent:        true,
	}
}

func TestEvaluateFunnelTriggers_QualifiedFiresExactlyOnce(t *testing.T) {
	disp := &fakeFunnelDispatcher{}
	leadRepo := new(MockLeadRepo)
	// First evaluation freshly stamps qualified_at; a (hypothetical) second
	// would return false. Allow it to be called at most once thanks to the
	// in-memory QualifiedAt guard set after a successful stamp.
	leadRepo.On("MarkQualified", mock.Anything, "lead_1").Return(true, nil).Once()

	r := &AgentRunner{leadRepo: leadRepo, funnelWebhookDispatcher: disp}
	lead := leadWith(map[string]string{"full_name": "Maria Silva", "budget": "5000"})
	leadCtx := &LeadDataContext{Funnel: qualifyingFunnel(), Lead: lead, Fields: twoFieldSchema()}

	// Required field full_name is filled → qualifies.
	r.EvaluateFunnelTriggers(context.Background(), leadCtx, "Maria Silva", "+5511999999999", false)
	// A subsequent turn after qualification must NOT re-fire (QualifiedAt now set).
	r.EvaluateFunnelTriggers(context.Background(), leadCtx, "Maria Silva", "+5511999999999", false)

	assert.Equal(t, 1, disp.count(EventLeadQualified), "lead.qualified must fire exactly once")
	leadRepo.AssertExpectations(t)

	// Payload sanity: carries funnel/lead/contact/status/metadata + event.
	require.Len(t, disp.events, 1)
	ev := disp.events[0]
	assert.Equal(t, "funnel:fun_1", ev.rateKey)
	assert.Equal(t, "acc_1", ev.accountID)
	assert.Equal(t, "https://crm.example.com/hook", ev.url)
	assert.Equal(t, "shh", ev.secret)
	payload, ok := ev.payload.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "fun_1", payload["funnel_id"])
	assert.Equal(t, "lead_1", payload["lead_id"])
	assert.Equal(t, string(domain.LeadStatusQualified), payload["status"])
	contact, ok := payload["contact"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Maria Silva", contact["name"])
	assert.Equal(t, "+5511999999999", contact["phone"])
	cm, ok := payload["collected_metadata"].(map[string]CollectedField)
	require.True(t, ok)
	assert.Equal(t, "Maria Silva", cm["full_name"].Value)
}

func TestEvaluateFunnelTriggers_NoQualifyWhenRequiredMissing(t *testing.T) {
	disp := &fakeFunnelDispatcher{}
	leadRepo := new(MockLeadRepo)

	r := &AgentRunner{leadRepo: leadRepo, funnelWebhookDispatcher: disp}
	// Only the optional budget is filled; required full_name is missing.
	lead := leadWith(map[string]string{"budget": "5000"})
	leadCtx := &LeadDataContext{Funnel: qualifyingFunnel(), Lead: lead, Fields: twoFieldSchema()}

	r.EvaluateFunnelTriggers(context.Background(), leadCtx, "", "+5511999999999", false)

	assert.Equal(t, 0, disp.count(EventLeadQualified))
	leadRepo.AssertNotCalled(t, "MarkQualified", mock.Anything, mock.Anything)
}

func TestEvaluateFunnelTriggers_BuyIntentFiresWhenFlagged(t *testing.T) {
	disp := &fakeFunnelDispatcher{}
	leadRepo := new(MockLeadRepo)

	r := &AgentRunner{leadRepo: leadRepo, funnelWebhookDispatcher: disp}
	// No required field filled, so no qualify — only the buy-intent path.
	lead := leadWith(map[string]string{})
	leadCtx := &LeadDataContext{Funnel: qualifyingFunnel(), Lead: lead, Fields: twoFieldSchema()}

	r.EvaluateFunnelTriggers(context.Background(), leadCtx, "", "+5511999999999", true)

	assert.Equal(t, 1, disp.count(EventLeadBuyIntent))
	assert.Equal(t, 0, disp.count(EventLeadQualified))
	leadRepo.AssertNotCalled(t, "MarkQualified", mock.Anything, mock.Anything)
}

func TestEvaluateFunnelTriggers_BuyIntentSilentWhenToggleOff(t *testing.T) {
	disp := &fakeFunnelDispatcher{}
	r := &AgentRunner{leadRepo: new(MockLeadRepo), funnelWebhookDispatcher: disp}
	funnel := qualifyingFunnel()
	funnel.TriggerOnBuyIntent = false
	lead := leadWith(map[string]string{})
	leadCtx := &LeadDataContext{Funnel: funnel, Lead: lead, Fields: twoFieldSchema()}

	r.EvaluateFunnelTriggers(context.Background(), leadCtx, "", "+55", true)

	assert.Equal(t, 0, disp.count(EventLeadBuyIntent))
}

func TestEvaluateFunnelTriggers_NoWebhookURLNoOp(t *testing.T) {
	disp := &fakeFunnelDispatcher{}
	leadRepo := new(MockLeadRepo)
	r := &AgentRunner{leadRepo: leadRepo, funnelWebhookDispatcher: disp}
	funnel := qualifyingFunnel()
	funnel.WebhookURL = "" // no endpoint configured
	lead := leadWith(map[string]string{"full_name": "Maria"})
	leadCtx := &LeadDataContext{Funnel: funnel, Lead: lead, Fields: twoFieldSchema()}

	r.EvaluateFunnelTriggers(context.Background(), leadCtx, "Maria", "+55", true)

	assert.Empty(t, disp.events)
	leadRepo.AssertNotCalled(t, "MarkQualified", mock.Anything, mock.Anything)
}

func TestEvaluateFunnelTriggers_NilContextNoOp(t *testing.T) {
	disp := &fakeFunnelDispatcher{}
	r := &AgentRunner{leadRepo: new(MockLeadRepo), funnelWebhookDispatcher: disp}
	// Nil lead context (no funnel binding) must be a silent no-op.
	r.EvaluateFunnelTriggers(context.Background(), nil, "", "", true)
	assert.Empty(t, disp.events)
}

func TestRequiredFieldsComplete_NoRequiredFieldsNeverQualifies(t *testing.T) {
	// A schema with no required field must not auto-qualify (vacuous-truth guard).
	fields := []domain.MetadataFieldDef{
		{Key: "budget", Label: "Orçamento", Type: domain.MetadataFieldNumber},
	}
	lead := leadWith(map[string]string{"budget": "5000"})
	assert.False(t, requiredFieldsComplete(fields, lead))
}

// US-F009: ownership-aware methods on the lead port.
func (m *MockLeadRepo) SetVisibility(ctx context.Context, accountID, id string, v domain.ResourceVisibility) (*domain.Lead, error) {
	args := m.Called(ctx, accountID, id, v)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Lead), args.Error(1)
}

func (m *MockLeadRepo) AssignOwner(ctx context.Context, accountID, id string, ownerUserID *string) (*domain.Lead, error) {
	args := m.Called(ctx, accountID, id, ownerUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Lead), args.Error(1)
}

func (m *MockLeadRepo) ListByFunnelForOwner(ctx context.Context, accountID, funnelID, ownerUserID string, status domain.LeadStatus, limit, offset int32) ([]*domain.Lead, error) {
	args := m.Called(ctx, accountID, funnelID, ownerUserID, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Lead), args.Error(1)
}

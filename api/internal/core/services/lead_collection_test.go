package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Mocks for the sales-collection repos (US-011) ---

type MockLeadRepo struct{ mock.Mock }

func (m *MockLeadRepo) Upsert(ctx context.Context, lead *domain.Lead) error {
	return m.Called(ctx, lead).Error(0)
}

func (m *MockLeadRepo) GetByFunnelAndContact(ctx context.Context, funnelID, contactID string) (*domain.Lead, error) {
	args := m.Called(ctx, funnelID, contactID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Lead), args.Error(1)
}

func (m *MockLeadRepo) GetByID(ctx context.Context, id string) (*domain.Lead, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Lead), args.Error(1)
}

func (m *MockLeadRepo) ListForApprovalQueue(ctx context.Context, f domain.ApprovalQueueFilter, limit, offset int32) ([]*domain.Lead, error) {
	args := m.Called(ctx, f, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Lead), args.Error(1)
}

func (m *MockLeadRepo) CountApprovalQueueByFunnel(ctx context.Context, f domain.ApprovalQueueFilter) ([]domain.QueueFunnelCount, error) {
	args := m.Called(ctx, f)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.QueueFunnelCount), args.Error(1)
}

func (m *MockLeadRepo) CountByStatus(ctx context.Context, accountID string) (map[domain.LeadStatus]int64, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[domain.LeadStatus]int64), args.Error(1)
}

func (m *MockLeadRepo) CountByStatusForOwner(ctx context.Context, accountID, ownerUserID string) (map[domain.LeadStatus]int64, error) {
	args := m.Called(ctx, accountID, ownerUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[domain.LeadStatus]int64), args.Error(1)
}

func (m *MockLeadRepo) CountUnowned(ctx context.Context, accountID string) (int64, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockLeadRepo) ListByFunnel(ctx context.Context, funnelID string, status domain.LeadStatus, limit, offset int32) ([]*domain.Lead, error) {
	args := m.Called(ctx, funnelID, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Lead), args.Error(1)
}

func (m *MockLeadRepo) UpdateMetadata(ctx context.Context, id string, collectedMetadata, enrichment json.RawMessage) error {
	return m.Called(ctx, id, collectedMetadata, enrichment).Error(0)
}

func (m *MockLeadRepo) UpdateStatus(ctx context.Context, id string, status domain.LeadStatus) error {
	return m.Called(ctx, id, status).Error(0)
}

func (m *MockLeadRepo) MarkBuyIntent(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockLeadRepo) MarkQualified(ctx context.Context, id string) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

type MockFunnelRepo struct{ mock.Mock }

func (m *MockFunnelRepo) Create(ctx context.Context, f *domain.SalesFunnel) error {
	return m.Called(ctx, f).Error(0)
}
func (m *MockFunnelRepo) GetByID(ctx context.Context, id string) (*domain.SalesFunnel, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SalesFunnel), args.Error(1)
}
func (m *MockFunnelRepo) GetByChannelID(ctx context.Context, channelID string) (*domain.SalesFunnel, error) {
	args := m.Called(ctx, channelID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SalesFunnel), args.Error(1)
}
func (m *MockFunnelRepo) ListByAccountID(ctx context.Context, accountID string) ([]*domain.SalesFunnel, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SalesFunnel), args.Error(1)
}
func (m *MockFunnelRepo) Update(ctx context.Context, f *domain.SalesFunnel) error {
	return m.Called(ctx, f).Error(0)
}
func (m *MockFunnelRepo) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockFunnelRepo) EnsureDefault(ctx context.Context, accountID string) (*domain.SalesFunnel, error) {
	args := m.Called(ctx, accountID)
	if f, _ := args.Get(0).(*domain.SalesFunnel); f != nil {
		return f, args.Error(1)
	}
	return nil, args.Error(1)
}

// --- Helpers ---

func leadRecordCall(fieldKey, value string) domain.ToolCall {
	args, _ := json.Marshal(map[string]string{"field_key": fieldKey, "value": value})
	return domain.ToolCall{FunctionName: RecordLeadDataToolName, Arguments: string(args)}
}

func twoFieldSchema() []domain.MetadataFieldDef {
	return []domain.MetadataFieldDef{
		{Key: "full_name", Label: "Nome", Type: domain.MetadataFieldText, Required: true},
		{Key: "budget", Label: "Orçamento", Type: domain.MetadataFieldNumber},
	}
}

// --- mergeRecordLeadData ---

func TestMergeRecordLeadData_WritesBothFields(t *testing.T) {
	now := time.Now()
	merged, writes := mergeRecordLeadData(map[string]CollectedField{}, twoFieldSchema(), []domain.ToolCall{
		leadRecordCall("full_name", "Maria Silva"),
		leadRecordCall("budget", "5000"),
	}, now)

	assert.Equal(t, 2, writes)
	require.Contains(t, merged, "full_name")
	require.Contains(t, merged, "budget")
	assert.Equal(t, "Maria Silva", merged["full_name"].Value)
	assert.Equal(t, CollectedSourceAI, merged["full_name"].Source)
	assert.Equal(t, "5000", merged["budget"].Value)
}

func TestMergeRecordLeadData_IgnoresUnknownKey(t *testing.T) {
	merged, writes := mergeRecordLeadData(map[string]CollectedField{}, twoFieldSchema(), []domain.ToolCall{
		leadRecordCall("full_name", "Maria"),
		leadRecordCall("not_in_schema", "garbage"),
	}, time.Now())

	assert.Equal(t, 1, writes)
	assert.Contains(t, merged, "full_name")
	assert.NotContains(t, merged, "not_in_schema")
}

func TestMergeRecordLeadData_IgnoresEmptyValue(t *testing.T) {
	_, writes := mergeRecordLeadData(map[string]CollectedField{}, twoFieldSchema(), []domain.ToolCall{
		leadRecordCall("full_name", "   "),
	}, time.Now())
	assert.Equal(t, 0, writes)
}

func TestMergeRecordLeadData_DedupsSameKeyValueInTurn(t *testing.T) {
	_, writes := mergeRecordLeadData(map[string]CollectedField{}, twoFieldSchema(), []domain.ToolCall{
		leadRecordCall("full_name", "Maria"),
		leadRecordCall("full_name", "Maria"),
	}, time.Now())
	assert.Equal(t, 1, writes)
}

func TestMergeRecordLeadData_NoOpWhenValueUnchanged(t *testing.T) {
	existing := map[string]CollectedField{
		"full_name": {Value: "Maria", Source: CollectedSourceAI, UpdatedAt: time.Now()},
	}
	_, writes := mergeRecordLeadData(existing, twoFieldSchema(), []domain.ToolCall{
		leadRecordCall("full_name", "Maria"),
	}, time.Now())
	assert.Equal(t, 0, writes)
}

func TestMergeRecordLeadData_SkipsNonRecordToolCalls(t *testing.T) {
	_, writes := mergeRecordLeadData(map[string]CollectedField{}, twoFieldSchema(), []domain.ToolCall{
		{FunctionName: "check_availability", Arguments: `{"date":"2026-06-09"}`},
	}, time.Now())
	assert.Equal(t, 0, writes)
}

// --- buildRecordLeadDataTool ---

func TestBuildRecordLeadDataTool_ConstrainsKeysToEnum(t *testing.T) {
	tool := buildRecordLeadDataTool(twoFieldSchema())
	assert.Equal(t, RecordLeadDataToolName, tool.Name)

	params := tool.Parameters.(map[string]interface{})
	props := params["properties"].(map[string]interface{})
	fieldKey := props["field_key"].(map[string]interface{})
	enum := fieldKey["enum"].([]string)
	assert.ElementsMatch(t, []string{"full_name", "budget"}, enum)
	assert.Contains(t, props, "value")
}

// --- RecordLeadData (persistence) ---

func TestRecordLeadData_PersistsBothFields(t *testing.T) {
	leadRepo := new(MockLeadRepo)
	runner := NewAgentRunner(nil, nil, nil)
	runner.SetSalesCollection(nil, leadRepo)

	lead := &domain.Lead{ID: "lead_1", CollectedMetadata: json.RawMessage("{}"), Enrichment: json.RawMessage("{}")}
	leadCtx := &LeadDataContext{Lead: lead, Fields: twoFieldSchema()}

	var captured json.RawMessage
	leadRepo.On("UpdateMetadata", mock.Anything, "lead_1", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { captured = args.Get(2).(json.RawMessage) }).
		Return(nil).Once()

	n := runner.RecordLeadData(context.Background(), leadCtx, []domain.ToolCall{
		leadRecordCall("full_name", "Maria Silva"),
		leadRecordCall("budget", "5000"),
	})

	assert.Equal(t, 2, n)
	leadRepo.AssertExpectations(t)

	var stored map[string]CollectedField
	require.NoError(t, json.Unmarshal(captured, &stored))
	assert.Equal(t, "Maria Silva", stored["full_name"].Value)
	assert.Equal(t, "5000", stored["budget"].Value)
}

func TestRecordLeadData_NoWriteWhenNothingNew(t *testing.T) {
	leadRepo := new(MockLeadRepo)
	runner := NewAgentRunner(nil, nil, nil)
	runner.SetSalesCollection(nil, leadRepo)

	lead := &domain.Lead{ID: "lead_1", CollectedMetadata: json.RawMessage("{}"), Enrichment: json.RawMessage("{}")}
	leadCtx := &LeadDataContext{Lead: lead, Fields: twoFieldSchema()}

	// Only an out-of-schema call → no UpdateMetadata should fire.
	n := runner.RecordLeadData(context.Background(), leadCtx, []domain.ToolCall{
		leadRecordCall("bogus", "x"),
	})
	assert.Equal(t, 0, n)
	leadRepo.AssertNotCalled(t, "UpdateMetadata", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRecordLeadData_NilContextIsNoOp(t *testing.T) {
	runner := NewAgentRunner(nil, nil, nil)
	assert.Equal(t, 0, runner.RecordLeadData(context.Background(), nil, []domain.ToolCall{leadRecordCall("full_name", "x")}))
}

// --- resolveLeadContext + injection ---

func TestResolveLeadContext_BindsFunnelAndLead(t *testing.T) {
	schema, _ := json.Marshal(twoFieldSchema())
	funnel := &domain.SalesFunnel{ID: "fn_1", MetadataSchema: schema, Active: true}
	lead := &domain.Lead{ID: "lead_1", FunnelID: "fn_1", ContactID: "ct_1", CollectedMetadata: json.RawMessage("{}")}

	funnelRepo := new(MockFunnelRepo)
	leadRepo := new(MockLeadRepo)
	funnelRepo.On("GetByChannelID", mock.Anything, "ch_1").Return(funnel, nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn_1", "ct_1").Return(lead, nil)

	runner := NewAgentRunner(nil, nil, nil)
	runner.SetSalesCollection(funnelRepo, leadRepo)

	got := runner.resolveLeadContext(context.Background(), "ch_1", "ct_1")
	require.NotNil(t, got)
	assert.Equal(t, "lead_1", got.Lead.ID)
	assert.Len(t, got.Fields, 2)
}

func TestResolveLeadContext_NilWhenNoLead(t *testing.T) {
	schema, _ := json.Marshal(twoFieldSchema())
	funnel := &domain.SalesFunnel{ID: "fn_1", MetadataSchema: schema, Active: true}
	funnelRepo := new(MockFunnelRepo)
	leadRepo := new(MockLeadRepo)
	funnelRepo.On("GetByChannelID", mock.Anything, "ch_1").Return(funnel, nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn_1", "ct_1").Return(nil, nil)

	runner := NewAgentRunner(nil, nil, nil)
	runner.SetSalesCollection(funnelRepo, leadRepo)

	assert.Nil(t, runner.resolveLeadContext(context.Background(), "ch_1", "ct_1"))
}

func TestResolveLeadContext_NilWithoutRepos(t *testing.T) {
	runner := NewAgentRunner(nil, nil, nil)
	assert.Nil(t, runner.resolveLeadContext(context.Background(), "ch_1", "ct_1"))
}

// --- FlagBuyIntent (US-012) ---

func flagBuyIntentCall() domain.ToolCall {
	return domain.ToolCall{FunctionName: FlagBuyIntentToolName, Arguments: "{}"}
}

func TestFlagBuyIntent_MarksOnFirstSignal(t *testing.T) {
	leadRepo := new(MockLeadRepo)
	runner := NewAgentRunner(nil, nil, nil)
	runner.SetSalesCollection(nil, leadRepo)

	lead := &domain.Lead{ID: "lead_1", CollectedMetadata: json.RawMessage("{}")}
	leadCtx := &LeadDataContext{Lead: lead, Fields: twoFieldSchema()}

	leadRepo.On("MarkBuyIntent", mock.Anything, "lead_1").Return(nil).Once()

	ok := runner.FlagBuyIntent(context.Background(), leadCtx, []domain.ToolCall{
		leadRecordCall("full_name", "Maria"),
		flagBuyIntentCall(),
	})

	assert.True(t, ok)
	assert.NotNil(t, lead.BuyIntentAt, "in-memory lead should reflect the stamp")
	leadRepo.AssertExpectations(t)
}

func TestFlagBuyIntent_NoCallIsNoOp(t *testing.T) {
	leadRepo := new(MockLeadRepo)
	runner := NewAgentRunner(nil, nil, nil)
	runner.SetSalesCollection(nil, leadRepo)

	lead := &domain.Lead{ID: "lead_1"}
	leadCtx := &LeadDataContext{Lead: lead, Fields: twoFieldSchema()}

	ok := runner.FlagBuyIntent(context.Background(), leadCtx, []domain.ToolCall{
		leadRecordCall("full_name", "Maria"),
	})

	assert.False(t, ok)
	leadRepo.AssertNotCalled(t, "MarkBuyIntent", mock.Anything, mock.Anything)
}

func TestFlagBuyIntent_IdempotentWhenAlreadyFlagged(t *testing.T) {
	leadRepo := new(MockLeadRepo)
	runner := NewAgentRunner(nil, nil, nil)
	runner.SetSalesCollection(nil, leadRepo)

	already := time.Now().Add(-time.Hour)
	lead := &domain.Lead{ID: "lead_1", BuyIntentAt: &already}
	leadCtx := &LeadDataContext{Lead: lead, Fields: twoFieldSchema()}

	ok := runner.FlagBuyIntent(context.Background(), leadCtx, []domain.ToolCall{flagBuyIntentCall()})

	assert.False(t, ok)
	assert.Equal(t, &already, lead.BuyIntentAt, "first-intent timestamp must be preserved")
	leadRepo.AssertNotCalled(t, "MarkBuyIntent", mock.Anything, mock.Anything)
}

func TestFlagBuyIntent_NilContextIsNoOp(t *testing.T) {
	runner := NewAgentRunner(nil, nil, nil)
	assert.False(t, runner.FlagBuyIntent(context.Background(), nil, []domain.ToolCall{flagBuyIntentCall()}))
}

func TestBuildFlagBuyIntentTool_NoArgs(t *testing.T) {
	tool := buildFlagBuyIntentTool()
	assert.Equal(t, FlagBuyIntentToolName, tool.Name)
	params, ok := tool.Parameters.(map[string]interface{})
	require.True(t, ok)
	props, ok := params["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Empty(t, props, "flag_buy_intent takes no arguments")
}

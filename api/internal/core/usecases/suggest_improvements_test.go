package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestSuggester_AppendSamples_PropagatesCancellation verifies US-033:
// appendSamples no longer hardcodes context.Background() for its message
// lookups — it threads the request ctx through buildPayload, so a cancelled
// request kills the work instead of running on a detached context. The
// MatchedBy(c.Err() != nil) only passes if the cancelled ctx actually
// reaches msgRepo; a regression reverting to context.Background() would
// surface a live ctx and fail.
func TestSuggester_AppendSamples_PropagatesCancellation(t *testing.T) {
	msgRepo := new(MockMessageRepo)
	uc := NewSuggestImprovementsUseCase(nil, nil, nil, nil, nil, msgRepo, nil)

	msgRepo.On("ListByConversationID",
		mock.MatchedBy(func(c context.Context) bool { return c.Err() != nil }),
		"conv-1", mock.Anything, mock.Anything,
	).Return([]*domain.Message{}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // request cancelled before the work runs

	var sb strings.Builder
	uc.appendSamples(ctx, &sb, "HEADER", []domain.ExperimentSample{{ConversationID: "conv-1"}})

	msgRepo.AssertExpectations(t)
}

func TestParseSuggestion_PlainJSON(t *testing.T) {
	raw := `{"rationale":"B was friendlier","suggested_body":"Be warm and concise.","next_hypotheses":["Use first name","Acknowledge faster","Keep replies under 2 sentences"]}`
	out, err := parseSuggestion(raw)
	assert.NoError(t, err)
	assert.Equal(t, "B was friendlier", out.Rationale)
	assert.Equal(t, "Be warm and concise.", out.SuggestedBody)
	assert.Len(t, out.NextHypotheses, 3)
}

func TestParseSuggestion_StripsCodeFence(t *testing.T) {
	raw := "```json\n{\"rationale\":\"x\",\"suggested_body\":\"y\",\"next_hypotheses\":[\"a\"]}\n```"
	out, err := parseSuggestion(raw)
	assert.NoError(t, err)
	assert.Equal(t, "x", out.Rationale)
	assert.Equal(t, "y", out.SuggestedBody)
}

func TestParseSuggestion_TolerantOfPreamble(t *testing.T) {
	raw := `Sure! Here is the JSON: {"rationale":"r","suggested_body":"b","next_hypotheses":[]} thanks!`
	out, err := parseSuggestion(raw)
	assert.NoError(t, err)
	assert.Equal(t, "r", out.Rationale)
}

func TestParseSuggestion_RejectsEmptyFields(t *testing.T) {
	raw := `{"rationale":"","suggested_body":"x","next_hypotheses":[]}`
	_, err := parseSuggestion(raw)
	assert.ErrorIs(t, err, domain.ErrSuggestionParseFailed)
}

func TestParseSuggestion_RejectsGarbage(t *testing.T) {
	_, err := parseSuggestion("nope")
	assert.ErrorIs(t, err, domain.ErrSuggestionParseFailed)
}

func TestParseSuggestion_TruncatesOversizedBody(t *testing.T) {
	huge := make([]byte, domain.MaxPromptBodyBytes+200)
	for i := range huge {
		huge[i] = 'x'
	}
	raw := `{"rationale":"r","suggested_body":"` + string(huge) + `","next_hypotheses":[]}`
	out, err := parseSuggestion(raw)
	assert.NoError(t, err)
	assert.Equal(t, domain.MaxPromptBodyBytes, len(out.SuggestedBody))
}

// Mock for MessageRepository limited to ListByConversationID since the
// suggester only calls that one method.
type MockMessageRepoLite struct{ mock.Mock }

func (m *MockMessageRepoLite) Create(ctx context.Context, msg *domain.Message) (string, error) {
	return "", nil
}
func (m *MockMessageRepoLite) GetByID(ctx context.Context, id string) (*domain.Message, error) {
	return nil, nil
}
func (m *MockMessageRepoLite) ListByConversationID(ctx context.Context, conversationID string, limit, offset int32) ([]*domain.Message, error) {
	a := m.Called(ctx, conversationID, limit, offset)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).([]*domain.Message), a.Error(1)
}

func TestSuggester_PlanLockedOnNonBusiness(t *testing.T) {
	planID := "p-pro"
	account := &domain.Account{ID: "acc-1", PlanID: &planID}
	plan := newTestPlan("Pro") // Pro allows experiments but NOT auto-suggest

	accRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	usageRepo := new(MockUsageRepo)
	auditRepo := new(MockAuditLogRepo)
	expRepo := new(MockExperimentRepo)
	aiFactory := new(MockAIFactory)

	accRepo.On("GetByID", mock.Anything, "acc-1").Return(account, nil)
	planRepo.On("GetByID", mock.Anything, planID).Return(plan, nil)

	uc := NewSuggestImprovementsUseCase(accRepo, planRepo, usageRepo, auditRepo, expRepo, nil, aiFactory)
	_, err := uc.Execute(context.Background(), SuggestImprovementsDTO{
		AccountID:    "acc-1",
		ExperimentID: "e-1",
	})
	assert.ErrorIs(t, err, domain.ErrPlanFeatureLocked)
}

func TestSuggester_RejectsCrossAccountExperiment(t *testing.T) {
	planID := "p-business"
	account := &domain.Account{ID: "acc-1", PlanID: &planID}
	plan := newTestPlan("Business")

	accRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	usageRepo := new(MockUsageRepo)
	auditRepo := new(MockAuditLogRepo)
	expRepo := new(MockExperimentRepo)
	aiFactory := new(MockAIFactory)

	accRepo.On("GetByID", mock.Anything, "acc-1").Return(account, nil)
	planRepo.On("GetByID", mock.Anything, planID).Return(plan, nil)
	expRepo.On("GetByID", mock.Anything, "e-1").Return(&domain.PromptExperiment{ID: "e-1", AccountID: "acc-OTHER"}, nil)

	uc := NewSuggestImprovementsUseCase(accRepo, planRepo, usageRepo, auditRepo, expRepo, nil, aiFactory)
	_, err := uc.Execute(context.Background(), SuggestImprovementsDTO{
		AccountID:    "acc-1",
		ExperimentID: "e-1",
	})
	assert.ErrorIs(t, err, domain.ErrExperimentNotFound)
}

// Compile-time guard that errors.As is in scope.
var _ = errors.As

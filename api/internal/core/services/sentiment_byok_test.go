package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

// The inbound classifiers (sentiment, intent) ran on the PLATFORM key for
// every message. They now ask for the ACCOUNT's provider and skip silently
// when it has none — never the platform key.

type byokAwareFactory struct {
	forAccountCalls int
	platformCalls   int
	err             error
	provider        domain.AIProvider
}

func (f *byokAwareFactory) GetProvider(context.Context, string) (domain.AIProvider, error) {
	f.platformCalls++
	return f.provider, nil
}

func (f *byokAwareFactory) GetProviderForAccount(context.Context, string, *domain.Account, *domain.Plan) (domain.AIProvider, error) {
	f.forAccountCalls++
	if f.err != nil {
		return nil, f.err
	}
	return f.provider, nil
}

func (f *byokAwareFactory) GetCostConfig(string, string) (float64, float64) { return 1, 1 }

type cannedProvider struct {
	content string
	usage   domain.AIUsage
}

func (p cannedProvider) GenerateStream(context.Context, string, []domain.ChatMessage, []domain.AITool) (domain.StreamResult, error) {
	return domain.StreamResult{}, nil
}

func (p cannedProvider) GenerateResponse(context.Context, string, []domain.ChatMessage, []domain.AITool) (*domain.AICallbackResponse, error) {
	return &domain.AICallbackResponse{Content: p.content, Usage: p.usage}, nil
}

func (p cannedProvider) GetEmbedding(context.Context, string) ([]float32, error) { return nil, nil }

type recordingMeter struct {
	ops []string
}

func (m *recordingMeter) Record(_ context.Context, _ string, operation, _, _ string, _ domain.AIUsage) {
	m.ops = append(m.ops, operation)
}

func TestSentimentAnalyzer_SkipsSilentlyWhenTheAccountHasNoKey(t *testing.T) {
	f := &byokAwareFactory{err: domain.ErrBYOKKeyMissing}
	sa := NewSentimentAnalyzer(f)

	_, _, err := sa.Analyze(context.Background(), "acc", "estou muito insatisfeito com o atraso")

	assert.ErrorIs(t, err, ErrHelperSkipped)
	assert.Equal(t, 1, f.forAccountCalls, "asks for the ACCOUNT's provider")
	assert.Equal(t, 0, f.platformCalls, "never falls back to the platform key")
}

func TestIntentClassifier_SkipsSilentlyWhenTheAccountHasNoKey(t *testing.T) {
	f := &byokAwareFactory{err: domain.ErrBYOKKeyMissing}
	c := NewIntentClassifier(f)

	_, _, err := c.Classify(context.Background(), "acc", "quero cancelar minha assinatura")

	assert.ErrorIs(t, err, ErrHelperSkipped)
	assert.Equal(t, 0, f.platformCalls)
}

func TestSentimentAnalyzer_MetersTheCallOnTheAccount(t *testing.T) {
	f := &byokAwareFactory{provider: cannedProvider{
		content: `{"sentiment":"negative","score":0.9}`,
		usage:   domain.AIUsage{PromptTokens: 40, CompletionTokens: 12},
	}}
	meter := &recordingMeter{}
	sa := NewSentimentAnalyzer(f).WithMeter(meter)

	label, score, err := sa.Analyze(context.Background(), "acc", "estou muito insatisfeito com o atraso")

	require.NoError(t, err)
	assert.Equal(t, "negative", label)
	assert.InDelta(t, 0.9, score, 0.001)
	assert.Equal(t, []string{"sentiment"}, meter.ops, "every metered call lands in the ledger")
}

func TestSmartReplier_SurfacesBYOKKeyMissingToTheCaller(t *testing.T) {
	f := &byokAwareFactory{err: domain.ErrBYOKKeyMissing}
	s := NewSmartReplier(f)

	_, err := s.Suggest(context.Background(), "acc", []SmartReplyHistoryItem{{From: "customer", Content: "oi"}}, "", "")

	assert.ErrorIs(t, err, domain.ErrBYOKKeyMissing, "a clear code, not a generic 502")
	assert.Equal(t, 0, f.platformCalls)
}

func TestConversationSummarizer_SurfacesBYOKKeyMissingToTheCaller(t *testing.T) {
	f := &byokAwareFactory{err: domain.ErrBYOKKeyMissing}
	s := NewConversationSummarizer(f)

	_, err := s.Summarize(context.Background(), "acc", []SummaryHistoryItem{{From: "customer", Content: "oi"}}, "", "")

	assert.ErrorIs(t, err, domain.ErrBYOKKeyMissing)
	assert.Equal(t, 0, f.platformCalls)
}

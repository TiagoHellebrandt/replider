package services

import (
	"context"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

// counterValue reads the current value of a counter without pulling in the
// prometheus testutil package (which drags an extra transitive dependency).
func counterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()
	var m dto.Metric
	require.NoError(t, c.Write(&m))
	return m.GetCounter().GetValue()
}

// metricsStubProvider is a controllable AIProvider for exercising the
// tracingProvider metrics instrumentation (US-067).
type metricsStubProvider struct {
	resp      *domain.AICallbackResponse
	respErr   error
	streamErr error
	streamUse func() (int64, int64)
	embedErr  error
}

func (s *metricsStubProvider) GenerateResponse(context.Context, string, []domain.ChatMessage, []domain.AITool) (*domain.AICallbackResponse, error) {
	return s.resp, s.respErr
}

func (s *metricsStubProvider) GenerateStream(context.Context, string, []domain.ChatMessage, []domain.AITool) (domain.StreamResult, error) {
	if s.streamErr != nil {
		return domain.StreamResult{}, s.streamErr
	}
	ch := make(chan string)
	close(ch)
	return domain.StreamResult{Chunks: ch, GetUsage: s.streamUse}, nil
}

func (s *metricsStubProvider) GetEmbedding(context.Context, string) ([]float32, error) {
	if s.embedErr != nil {
		return nil, s.embedErr
	}
	return []float32{0.1}, nil
}

func TestTracingProviderRecordsAIMetrics(t *testing.T) {
	const provider = "test-metrics-prov"

	callsSuccess := func() float64 {
		return counterValue(t, aiCallsTotal.WithLabelValues(provider, "chat", "success"))
	}
	callsError := func() float64 {
		return counterValue(t, aiCallsTotal.WithLabelValues(provider, "chat", "error"))
	}
	promTokens := func() float64 {
		return counterValue(t, aiTokensTotal.WithLabelValues(provider, "prompt"))
	}
	complTokens := func() float64 {
		return counterValue(t, aiTokensTotal.WithLabelValues(provider, "completion"))
	}

	beforeSuccess, beforeErr := callsSuccess(), callsError()
	beforeP, beforeC := promTokens(), complTokens()

	// Successful chat call records success + tokens.
	tp := NewTracingProvider(&metricsStubProvider{
		resp: &domain.AICallbackResponse{
			Content: "hi",
			Usage:   domain.AIUsage{PromptTokens: 12, CompletionTokens: 5},
		},
	}, provider)
	_, err := tp.GenerateResponse(context.Background(), "sys", nil, nil)
	require.NoError(t, err)

	assert.Equal(t, beforeSuccess+1, callsSuccess(), "success counter should increment")
	assert.Equal(t, beforeP+12, promTokens(), "prompt tokens should add")
	assert.Equal(t, beforeC+5, complTokens(), "completion tokens should add")

	// Failed chat call records the error outcome and no tokens.
	tpErr := NewTracingProvider(&metricsStubProvider{respErr: errors.New("boom")}, provider)
	_, err = tpErr.GenerateResponse(context.Background(), "sys", nil, nil)
	require.Error(t, err)
	assert.Equal(t, beforeErr+1, callsError(), "error counter should increment")
}

func TestTracingProviderStreamTokensRecordedOnDrain(t *testing.T) {
	const provider = "test-metrics-stream"
	tokens := func() float64 {
		return counterValue(t, aiTokensTotal.WithLabelValues(provider, "prompt"))
	}
	before := tokens()

	tp := NewTracingProvider(&metricsStubProvider{
		streamUse: func() (int64, int64) { return 7, 3 },
	}, provider)
	res, err := tp.GenerateStream(context.Background(), "sys", nil, nil)
	require.NoError(t, err)

	// Tokens are unknown until the caller resolves GetUsage — not before.
	assert.Equal(t, before, tokens(), "tokens must not be counted before drain")
	require.NotNil(t, res.GetUsage)
	p, c := res.GetUsage()
	assert.Equal(t, int64(7), p)
	assert.Equal(t, int64(3), c)
	assert.Equal(t, before+7, tokens(), "tokens counted once GetUsage resolves")
}

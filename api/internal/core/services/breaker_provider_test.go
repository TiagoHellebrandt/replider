package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// stubProvider always fails GenerateResponse — stands in for a down LLM.
type stubProvider struct{ calls int }

func (s *stubProvider) GenerateResponse(context.Context, string, []domain.ChatMessage, []domain.AITool) (*domain.AICallbackResponse, error) {
	s.calls++
	return nil, errors.New("upstream 503")
}
func (s *stubProvider) GenerateStream(context.Context, string, []domain.ChatMessage, []domain.AITool) (domain.StreamResult, error) {
	return domain.StreamResult{}, errors.New("upstream 503")
}
func (s *stubProvider) GetEmbedding(context.Context, string) ([]float32, error) {
	return nil, errors.New("upstream 503")
}

// TestBreakerProvider_OpenServesFallback locks in US-026: once the breaker
// trips from repeated provider failures, further calls fail fast with the
// graceful domain.ErrAIProviderUnavailable and never reach the dead upstream.
func TestBreakerProvider_OpenServesFallback(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	cb := NewCircuitBreaker().withClock(func() time.Time { return now })
	inner := &stubProvider{}
	p := NewBreakerProvider(inner, cb)
	ctx := context.Background()

	// Drive 5 failures to trip the breaker.
	for i := 0; i < 5; i++ {
		_, _ = p.GenerateResponse(ctx, "", nil, nil)
	}
	if cb.State() != CircuitOpen {
		t.Fatalf("want open after 5 failures, got %s", cb.State())
	}
	callsBefore := inner.calls

	// Next call must fail fast with the fallback error, NOT hit the upstream.
	_, err := p.GenerateResponse(ctx, "", nil, nil)
	if !errors.Is(err, domain.ErrAIProviderUnavailable) {
		t.Fatalf("want ErrAIProviderUnavailable while open, got %v", err)
	}
	if inner.calls != callsBefore {
		t.Fatalf("open breaker must not call upstream: calls %d -> %d", callsBefore, inner.calls)
	}
}

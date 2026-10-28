package services

import (
	"context"
	"errors"
	"log/slog"

	"github.com/replider/api/internal/core/domain"
)

// breakerProvider wraps an AIProvider with a circuit breaker (US-026). When
// the upstream LLM starts failing, the breaker trips and subsequent calls
// fail fast with domain.ErrAIProviderUnavailable instead of each hanging on
// a doomed request — the caller's existing error path then serves a graceful
// fallback (no AI reply) rather than a slow timeout per message.
type breakerProvider struct {
	inner domain.AIProvider
	cb    *CircuitBreaker
}

// NewBreakerProvider decorates inner with the shared breaker cb. The breaker
// MUST be shared across calls (held on the factory) so its failure count
// persists; a per-call breaker would never trip.
func NewBreakerProvider(inner domain.AIProvider, cb *CircuitBreaker) domain.AIProvider {
	return &breakerProvider{inner: inner, cb: cb}
}

// fallbackErr translates the internal open-circuit sentinel into the domain
// error the rest of the system already handles gracefully.
func fallbackErr(err error) error {
	if errors.Is(err, ErrCircuitOpen) {
		return domain.ErrAIProviderUnavailable
	}
	return err
}

func (b *breakerProvider) GenerateResponse(ctx context.Context, systemPrompt string, history []domain.ChatMessage, tools []domain.AITool) (*domain.AICallbackResponse, error) {
	var resp *domain.AICallbackResponse
	err := b.cb.Execute(func() error {
		r, e := b.inner.GenerateResponse(ctx, systemPrompt, history, tools)
		resp = r
		return e
	})
	if err != nil {
		if errors.Is(err, ErrCircuitOpen) {
			slog.Warn("AI circuit breaker open, serving fallback", "call", "GenerateResponse")
		}
		return nil, fallbackErr(err)
	}
	return resp, nil
}

func (b *breakerProvider) GenerateStream(ctx context.Context, systemPrompt string, history []domain.ChatMessage, tools []domain.AITool) (domain.StreamResult, error) {
	// Streaming errors surface at call-initiation here; mid-stream failures
	// aren't observable to the breaker, but a hard-down provider fails to
	// open the stream at all, which is exactly what trips the breaker.
	if err := b.cb.Allow(); err != nil {
		slog.Warn("AI circuit breaker open, serving fallback", "call", "GenerateStream")
		return domain.StreamResult{}, domain.ErrAIProviderUnavailable
	}
	sr, err := b.inner.GenerateStream(ctx, systemPrompt, history, tools)
	b.cb.Record(err)
	return sr, err
}

// GenerateFromDocument forwards the optional DocumentReader capability
// through the breaker. Without it the whole decorator chain answers "no" to
// a domain.DocumentReader assertion even when the innermost provider is
// Gemini — which is exactly how PDF import shipped dead: the use case's
// honest refusal fired on every upload, for every account.
func (b *breakerProvider) GenerateFromDocument(ctx context.Context, systemPrompt, prompt string, doc domain.AIDocument) (*domain.AICallbackResponse, error) {
	reader, ok := b.inner.(domain.DocumentReader)
	if !ok {
		// Not a provider failure — never let it count toward tripping.
		return nil, domain.ErrAIDocumentUnsupported
	}
	var resp *domain.AICallbackResponse
	err := b.cb.Execute(func() error {
		r, e := reader.GenerateFromDocument(ctx, systemPrompt, prompt, doc)
		resp = r
		return e
	})
	if err != nil {
		if errors.Is(err, ErrCircuitOpen) {
			slog.Warn("AI circuit breaker open, serving fallback", "call", "GenerateFromDocument")
		}
		return nil, fallbackErr(err)
	}
	return resp, nil
}

// SubmitBatch / FetchBatch forward the optional AIBatchProvider capability —
// same reason as GenerateFromDocument, and the same silent failure: with the
// chain opaque, AI_BATCH_DRAFTS=true would answer ErrBatchUnsupported forever
// and the night would quietly run at full price.
//
// Deliberately NOT wrapped in the breaker or a retry: batch is asynchronous,
// FetchBatch is a poll, and a job whose own items failed is not a provider
// outage. Tripping the account's breaker on that would take the sync path
// down with it.
func (b *breakerProvider) SubmitBatch(ctx context.Context, displayName string, reqs []domain.AIBatchRequest) (string, error) {
	batcher, ok := b.inner.(domain.AIBatchProvider)
	if !ok {
		return "", domain.ErrBatchUnsupported
	}
	return batcher.SubmitBatch(ctx, displayName, reqs)
}

func (b *breakerProvider) FetchBatch(ctx context.Context, jobRef string) (bool, []domain.AIBatchResult, error) {
	batcher, ok := b.inner.(domain.AIBatchProvider)
	if !ok {
		return false, nil, domain.ErrBatchUnsupported
	}
	return batcher.FetchBatch(ctx, jobRef)
}

func (b *breakerProvider) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	var emb []float32
	err := b.cb.Execute(func() error {
		e, callErr := b.inner.GetEmbedding(ctx, text)
		emb = e
		return callErr
	})
	if err != nil {
		return nil, fallbackErr(err)
	}
	return emb, nil
}

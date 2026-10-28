package ai

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// RetryProvider wraps another AIProvider with bounded exponential backoff on
// transient failures (5xx, timeouts, rate limits). We retry the non-streaming
// calls — GenerateResponse and GetEmbedding — because those are request/
// response and cheap to resubmit. GenerateStream is NOT retried because the
// caller is already piping chunks to the user: failing loudly is better than
// retrying and confusing the downstream with a restarted stream.
type RetryProvider struct {
	inner       domain.AIProvider
	maxAttempts int
	baseDelay   time.Duration
}

// NewRetryProvider wraps p with retry behaviour. Defaults: 3 attempts, 500ms
// base delay (so worst-case wait is ~500ms + 1s + 2s ≈ 3.5s with jitter).
func NewRetryProvider(p domain.AIProvider) *RetryProvider {
	return &RetryProvider{inner: p, maxAttempts: 3, baseDelay: 500 * time.Millisecond}
}

// GenerateStream is passed through without retry. Streaming errors usually
// surface after chunks are already sent; restarting would duplicate content.
func (r *RetryProvider) GenerateStream(ctx context.Context, systemPrompt string, history []domain.ChatMessage, tools []domain.AITool) (domain.StreamResult, error) {
	return r.inner.GenerateStream(ctx, systemPrompt, history, tools)
}

func (r *RetryProvider) GenerateResponse(ctx context.Context, systemPrompt string, history []domain.ChatMessage, tools []domain.AITool) (*domain.AICallbackResponse, error) {
	var last error
	for attempt := 1; attempt <= r.maxAttempts; attempt++ {
		resp, err := r.inner.GenerateResponse(ctx, systemPrompt, history, tools)
		if err == nil {
			return resp, nil
		}
		last = err
		if !isTransient(err) || attempt == r.maxAttempts {
			return nil, err
		}
		sleepWithJitter(ctx, r.baseDelay, attempt)
		slog.Warn("AI GenerateResponse retry", "attempt", attempt, "error", err)
	}
	return nil, last
}

// GenerateFromDocument forwards the optional DocumentReader capability, with
// the same bounded retry GenerateResponse gets: a document read is
// request/response and cheap to resubmit. A provider that cannot read files
// is refused immediately — that is not a transient failure.
func (r *RetryProvider) GenerateFromDocument(ctx context.Context, systemPrompt, prompt string, doc domain.AIDocument) (*domain.AICallbackResponse, error) {
	reader, ok := r.inner.(domain.DocumentReader)
	if !ok {
		return nil, domain.ErrAIDocumentUnsupported
	}
	var last error
	for attempt := 1; attempt <= r.maxAttempts; attempt++ {
		resp, err := reader.GenerateFromDocument(ctx, systemPrompt, prompt, doc)
		if err == nil {
			return resp, nil
		}
		last = err
		if !isTransient(err) || attempt == r.maxAttempts {
			return nil, err
		}
		sleepWithJitter(ctx, r.baseDelay, attempt)
		slog.Warn("AI GenerateFromDocument retry", "attempt", attempt, "error", err)
	}
	return nil, last
}

// SubmitBatch / FetchBatch pass the optional AIBatchProvider capability
// through unchanged. No retry: a submit that half-succeeded would mint a
// second provider job nobody tracks, and FetchBatch is already a poll.
func (r *RetryProvider) SubmitBatch(ctx context.Context, displayName string, reqs []domain.AIBatchRequest) (string, error) {
	batcher, ok := r.inner.(domain.AIBatchProvider)
	if !ok {
		return "", domain.ErrBatchUnsupported
	}
	return batcher.SubmitBatch(ctx, displayName, reqs)
}

func (r *RetryProvider) FetchBatch(ctx context.Context, jobRef string) (bool, []domain.AIBatchResult, error) {
	batcher, ok := r.inner.(domain.AIBatchProvider)
	if !ok {
		return false, nil, domain.ErrBatchUnsupported
	}
	return batcher.FetchBatch(ctx, jobRef)
}

func (r *RetryProvider) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	var last error
	for attempt := 1; attempt <= r.maxAttempts; attempt++ {
		v, err := r.inner.GetEmbedding(ctx, text)
		if err == nil {
			return v, nil
		}
		last = err
		if !isTransient(err) || attempt == r.maxAttempts {
			return nil, err
		}
		sleepWithJitter(ctx, r.baseDelay, attempt)
		slog.Warn("AI GetEmbedding retry", "attempt", attempt, "error", err)
	}
	return nil, last
}

// isTransient classifies an error as worth retrying. We match on substrings
// rather than concrete provider error types because the two SDKs (OpenAI and
// Gemini) return opaque wrappers that vary across minor versions.
func isTransient(err error) bool {
	if err == nil {
		return false
	}
	// An exhausted quota is not a blip. Gemini answers "Please retry in 57s";
	// this backoff tops out around 3.5s, so every retry is spent, fails, and
	// eats another request from the same allowance — three times the burn to
	// reach the same answer, later. Fail fast and let the caller say what is
	// actually wrong.
	if isQuotaExhausted(err) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		// Canceled means upstream gave up — don't retry.
		return errors.Is(err, context.DeadlineExceeded)
	}
	msg := strings.ToLower(err.Error())
	for _, needle := range []string{
		"429", "500", "502", "503", "504",
		"rate limit", "timeout", "deadline exceeded",
		"temporarily unavailable", "try again", "overloaded",
		"connection reset", "connection refused", "eof",
	} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

// isQuotaExhausted reports a hard allowance failure, as opposed to the
// server-side throttling that a short backoff does clear. Both arrive as 429,
// so the text is the only thing that separates them.
func isQuotaExhausted(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "quota") || strings.Contains(msg, "resource_exhausted") ||
		strings.Contains(msg, "billing")
}

// sleepWithJitter waits base * 2^(attempt-1) ± 25% jitter, but bails out if
// the caller's context is canceled (don't hold the request hostage).
func sleepWithJitter(ctx context.Context, base time.Duration, attempt int) {
	d := base << (attempt - 1)
	jitter := time.Duration(rand.Int63n(int64(d) / 2))
	wait := d + jitter - d/4
	select {
	case <-time.After(wait):
	case <-ctx.Done():
	}
}

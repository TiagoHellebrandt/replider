package services

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/replider/api/internal/core/domain"
)

// tracingProvider decorates an AIProvider so every LLM call opens an
// OpenTelemetry span (US-066). This makes the inbound→AI→outbound latency
// — previously a black box — visible per-request: the span captures the
// provider name, GenAI operation and (for non-streaming calls) token
// usage. When tracing is disabled the global tracer is the SDK no-op, so
// these spans cost effectively nothing.
//
// It sits INSIDE the retry/breaker layers (breaker(retry(tracing(p))) so
// the span measures the actual provider round-trip, not retry back-off.
type tracingProvider struct {
	inner    domain.AIProvider
	provider string // "openai" / "gemini" — low-cardinality, safe as a span attr
	tracer   trace.Tracer
}

// NewTracingProvider decorates inner with span instrumentation. providerName
// is recorded as gen_ai.system so traces can be filtered per backend.
func NewTracingProvider(inner domain.AIProvider, providerName string) domain.AIProvider {
	return &tracingProvider{
		inner:    inner,
		provider: providerName,
		tracer:   otel.Tracer("github.com/replider/api"),
	}
}

func (t *tracingProvider) GenerateResponse(ctx context.Context, systemPrompt string, history []domain.ChatMessage, tools []domain.AITool) (*domain.AICallbackResponse, error) {
	ctx, span := t.tracer.Start(ctx, "ai.GenerateResponse",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(t.baseAttrs(history, tools)...),
	)
	defer span.End()

	start := time.Now()
	resp, err := t.inner.GenerateResponse(ctx, systemPrompt, history, tools)
	aiLatencySeconds.WithLabelValues(t.provider, "chat").Observe(time.Since(start).Seconds())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		aiCallsTotal.WithLabelValues(t.provider, "chat", "error").Inc()
		return nil, err
	}
	aiCallsTotal.WithLabelValues(t.provider, "chat", "success").Inc()
	if resp != nil {
		span.SetAttributes(
			attribute.Int64("gen_ai.usage.input_tokens", resp.Usage.PromptTokens),
			attribute.Int64("gen_ai.usage.output_tokens", resp.Usage.CompletionTokens),
			attribute.Int("gen_ai.response.tool_calls", len(resp.ToolCalls)),
		)
		t.recordTokens(resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	}
	return resp, nil
}

func (t *tracingProvider) GenerateStream(ctx context.Context, systemPrompt string, history []domain.ChatMessage, tools []domain.AITool) (domain.StreamResult, error) {
	ctx, span := t.tracer.Start(ctx, "ai.GenerateStream",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(t.baseAttrs(history, tools)...),
	)
	// Streaming spans only measure call initiation — the stream is consumed
	// by the caller after this returns, so we end the span here rather than
	// hold it open for the lifetime of the chunk channel.
	defer span.End()

	start := time.Now()
	res, err := t.inner.GenerateStream(ctx, systemPrompt, history, tools)
	// Latency here measures call INITIATION only — the chunk channel is
	// drained by the caller after this returns (the span is closed for the
	// same reason). Token usage is unknown until the stream finishes, so we
	// wrap GetUsage to record ai_tokens_total when the caller resolves it.
	aiLatencySeconds.WithLabelValues(t.provider, "stream").Observe(time.Since(start).Seconds())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		aiCallsTotal.WithLabelValues(t.provider, "stream", "error").Inc()
		return domain.StreamResult{}, err
	}
	aiCallsTotal.WithLabelValues(t.provider, "stream", "success").Inc()
	if inner := res.GetUsage; inner != nil {
		res.GetUsage = func() (int64, int64) {
			prompt, completion := inner()
			t.recordTokens(prompt, completion)
			return prompt, completion
		}
	}
	return res, nil
}

// GenerateFromDocument forwards the optional DocumentReader capability and
// spans it like any other call. Document reads are the most expensive call
// the product makes (a whole brochure through the model), so they are the
// last thing that should be invisible in traces.
func (t *tracingProvider) GenerateFromDocument(ctx context.Context, systemPrompt, prompt string, doc domain.AIDocument) (*domain.AICallbackResponse, error) {
	reader, ok := t.inner.(domain.DocumentReader)
	if !ok {
		return nil, domain.ErrAIDocumentUnsupported
	}
	ctx, span := t.tracer.Start(ctx, "ai.GenerateFromDocument",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gen_ai.system", t.provider),
			attribute.String("gen_ai.operation.name", "document"),
			// Size and type only — never the document's bytes (LGPD).
			attribute.String("gen_ai.request.document_mime", doc.MIMEType),
			attribute.Int("gen_ai.request.document_bytes", len(doc.Data)),
		),
	)
	defer span.End()

	start := time.Now()
	resp, err := reader.GenerateFromDocument(ctx, systemPrompt, prompt, doc)
	aiLatencySeconds.WithLabelValues(t.provider, "document").Observe(time.Since(start).Seconds())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		aiCallsTotal.WithLabelValues(t.provider, "document", "error").Inc()
		return nil, err
	}
	aiCallsTotal.WithLabelValues(t.provider, "document", "success").Inc()
	if resp != nil {
		span.SetAttributes(
			attribute.Int64("gen_ai.usage.input_tokens", resp.Usage.PromptTokens),
			attribute.Int64("gen_ai.usage.output_tokens", resp.Usage.CompletionTokens),
		)
		t.recordTokens(resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	}
	return resp, nil
}

// SubmitBatch / FetchBatch pass the optional AIBatchProvider capability
// through. Unspanned on purpose: the work happens on the provider's side over
// hours, so a span around the submit call would measure an HTTP POST and imply
// the job took 200ms.
func (t *tracingProvider) SubmitBatch(ctx context.Context, displayName string, reqs []domain.AIBatchRequest) (string, error) {
	batcher, ok := t.inner.(domain.AIBatchProvider)
	if !ok {
		return "", domain.ErrBatchUnsupported
	}
	return batcher.SubmitBatch(ctx, displayName, reqs)
}

func (t *tracingProvider) FetchBatch(ctx context.Context, jobRef string) (bool, []domain.AIBatchResult, error) {
	batcher, ok := t.inner.(domain.AIBatchProvider)
	if !ok {
		return false, nil, domain.ErrBatchUnsupported
	}
	return batcher.FetchBatch(ctx, jobRef)
}

func (t *tracingProvider) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	ctx, span := t.tracer.Start(ctx, "ai.GetEmbedding",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gen_ai.system", t.provider),
			attribute.String("gen_ai.operation.name", "embeddings"),
			attribute.Int("gen_ai.request.input_chars", len(text)),
		),
	)
	defer span.End()

	start := time.Now()
	vec, err := t.inner.GetEmbedding(ctx, text)
	aiLatencySeconds.WithLabelValues(t.provider, "embeddings").Observe(time.Since(start).Seconds())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		aiCallsTotal.WithLabelValues(t.provider, "embeddings", "error").Inc()
		return nil, err
	}
	aiCallsTotal.WithLabelValues(t.provider, "embeddings", "success").Inc()
	return vec, nil
}

// recordTokens increments ai_tokens_total for a completed chat/stream call.
// Negative or zero counts (providers that don't report usage) are skipped so
// the counter only ever moves on real consumption.
func (t *tracingProvider) recordTokens(prompt, completion int64) {
	if prompt > 0 {
		aiTokensTotal.WithLabelValues(t.provider, "prompt").Add(float64(prompt))
	}
	if completion > 0 {
		aiTokensTotal.WithLabelValues(t.provider, "completion").Add(float64(completion))
	}
}

// baseAttrs returns the low-cardinality attributes common to chat calls.
// It deliberately records only structural metadata (counts) — never prompt
// or message CONTENT — so personal data stays out of the trace backend
// (LGPD, mirrors the log-redaction posture in observability.RedactPII).
func (t *tracingProvider) baseAttrs(history []domain.ChatMessage, tools []domain.AITool) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("gen_ai.system", t.provider),
		attribute.String("gen_ai.operation.name", "chat"),
		attribute.Int("gen_ai.request.history_messages", len(history)),
		attribute.Int("gen_ai.request.tools", len(tools)),
	}
}

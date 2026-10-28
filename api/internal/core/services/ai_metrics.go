package services

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// LLM provider metrics (US-067). Recorded by the tracingProvider decorator
// so every chat / stream / embedding call is counted regardless of which
// concrete provider (OpenAI / Gemini) handled it. Exposed via the same
// /metrics scrape endpoint as the HTTP and WhatsApp metrics.
//
// Labels are deliberately low-cardinality — provider name, a fixed
// operation enum and a success/error outcome — so the series count stays
// bounded. Never label with prompt content, account_id or model strings
// that can grow unbounded.
var (
	aiCallsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_calls_total",
			Help: "Total LLM provider calls, labelled by provider, operation (chat/stream/embeddings) and outcome (success/error).",
		},
		[]string{"provider", "operation", "outcome"},
	)

	aiTokensTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_tokens_total",
			Help: "Total tokens consumed by LLM calls, labelled by provider and kind (prompt/completion).",
		},
		[]string{"provider", "kind"},
	)

	aiLatencySeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ai_latency_seconds",
			Help:    "Latency of LLM provider calls in seconds, labelled by provider and operation.",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 5, 10, 20, 30, 60},
		},
		[]string{"provider", "operation"},
	)
)

package observability

import (
	"context"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// logLinesTotal counts every log record the application emits, labelled by
// level. It is the cost guardrail for the log bill (US-075): a managed log
// sink (Loki/CloudWatch/Datadog) charges per GB ingested, and a runaway loop
// or a newly-chatty code path can multiply volume silently. Exporting the
// emit count to Prometheus lets the `HighLogVolume` alert
// (infra/alerts.yaml) fire when the 5m rate exceeds 2x the recent baseline,
// long before the invoice arrives.
//
// `level` is a fixed, low-cardinality enum (DEBUG/INFO/WARN/ERROR) — never
// the message or any attribute — keeping the series count bounded, same
// posture as every other collector (see docs/metrics.md).
var logLinesTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "log_lines_total",
		Help: "Total number of log records emitted, by level.",
	},
	[]string{"level"},
)

// MeteringHandler wraps an slog.Handler and increments logLinesTotal for
// every record that is actually emitted (i.e. that passes the inner
// handler's level filter). It is otherwise fully transparent — all
// formatting, redaction (RedactPII) and output stay with the wrapped
// handler.
type MeteringHandler struct {
	inner slog.Handler
}

// NewMeteringHandler wraps inner so every emitted record is metered into
// log_lines_total{level=...}. Wrap the OUTERMOST handler in main.go so the
// count reflects what is really written to stdout.
func NewMeteringHandler(inner slog.Handler) *MeteringHandler {
	return &MeteringHandler{inner: inner}
}

func (h *MeteringHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *MeteringHandler) Handle(ctx context.Context, r slog.Record) error {
	logLinesTotal.WithLabelValues(r.Level.String()).Inc()
	return h.inner.Handle(ctx, r)
}

func (h *MeteringHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &MeteringHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *MeteringHandler) WithGroup(name string) slog.Handler {
	return &MeteringHandler{inner: h.inner.WithGroup(name)}
}

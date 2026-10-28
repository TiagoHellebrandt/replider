# Observability — Distributed Tracing (OpenTelemetry)

The API emits OpenTelemetry traces so the **inbound → AI → outbound** latency
chain — previously a black box — is visible end to end. Tracing complements the
existing pillars:

| Pillar | Where | Doc |
|--------|-------|-----|
| Logs | structured JSON via `slog`, PII-redacted | [logging.md](logging.md) |
| Metrics | Prometheus `/metrics` (HTTP, LLM, webhook, `whatsapp_*`) | [metrics.md](metrics.md) |
| Errors | Sentry (`SENTRY_DSN`) | — |
| Alerting | Prometheus rules → Alertmanager → Slack/Discord | [runbook_alerts.md](runbook_alerts.md) |
| SLO / error budget | Grafana dashboard over the Prometheus metrics | [error_budget.md](error_budget.md) |
| **Traces** | **OTLP/HTTP → collector** | **this doc** |

## Wiring

`observability.InitTracing` (`api/internal/adapters/observability/tracing.go`)
is called once in `cmd/api/main.go`, right after Sentry init. It builds an
OTLP/HTTP exporter, a batching `TracerProvider` with a service `resource`, and
registers them as the **global** provider plus a W3C `tracecontext`+`baggage`
propagator. The returned `shutdown` flushes buffered spans on exit (deferred in
`main`).

When `OTEL_EXPORTER_OTLP_ENDPOINT` is **empty** (local dev, tests) tracing is
disabled: the global provider stays the SDK no-op, so every instrumentation
point below becomes zero-cost. No code paths need to branch on whether tracing
is on.

## What is instrumented

| Layer | Mechanism | Span(s) |
|-------|-----------|---------|
| **HTTP (server)** | `otelgin.Middleware` in `internal/adapters/http/router/router.go` | one server span per request (`/health`, `/healthz`, `/metrics` excluded) |
| **Database** | `otelpgx.NewTracer()` set on the pgx pool config in `cmd/api/main.go` | one span per query; SQL text (no bound args) as the span name |
| **LLM** | `services.tracingProvider` decorator (`internal/core/services/tracing_provider.go`), layered `breaker(retry(tracing(provider)))` in `AIFactory.buildProvider` | `ai.GenerateResponse`, `ai.GenerateStream`, `ai.GetEmbedding` |

Spans propagate via context, so a single inbound webhook produces a connected
trace: gin server span → pgx query spans → `ai.*` provider span → outbound.

### PII posture

Spans record only **structural metadata** — message/tool counts, token usage,
provider name (`gen_ai.system`) — never prompt or message **content**. DB spans
use trimmed SQL text without bound arguments. This mirrors the log-redaction
posture (`observability.RedactPII`); keep it that way when adding attributes.

## Configuration

| Env var | Default | Meaning |
|---------|---------|---------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | _(empty)_ | OTLP/HTTP collector base, e.g. `otel-collector:4318` or `https://otlp.example.com`. **Empty disables tracing.** A bare `host:port` is sent over plain HTTP; an `https://` URL forces TLS. |
| `OTEL_SERVICE_NAME` | `replider-api` | `service.name` reported on every span and used as the otelgin span scope. |

The standard `OTEL_EXPORTER_OTLP_*` env vars (headers, timeout, compression) are
also honoured by the exporter. Example local setup with the OTel Collector:

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4318 make dev
```

## Manual spans

To trace a new background worker or hot path, draw from the same scope:

```go
import "go.opentelemetry.io/otel"

tracer := otel.Tracer(observability.TracerName) // "github.com/replider/api"
ctx, span := tracer.Start(ctx, "ingestion.embedChunk")
defer span.End()
```

Record only low-cardinality, non-PII attributes, and call `span.RecordError(err)`
+ `span.SetStatus(codes.Error, …)` on failure (see `tracing_provider.go`).

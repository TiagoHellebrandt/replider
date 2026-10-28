package observability

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// TracerName is the instrumentation scope used for every manually-created
// span in the codebase (LLM calls, background workers). Pass it to
// otel.Tracer(TracerName) so all our spans share one scope.
const TracerName = "github.com/replider/api"

// InitTracing wires an OpenTelemetry TracerProvider that exports spans over
// OTLP/HTTP to the collector at endpoint. When endpoint is empty (local
// dev, tests) tracing is disabled: the global provider stays the SDK
// no-op, so otelgin/otelpgx and manual spans become zero-cost. The
// returned shutdown flushes buffered spans on process exit — defer it in
// main right after the call.
//
// endpoint is the OTLP/HTTP collector base, e.g. "otel-collector:4318" or
// "https://otlp.example.com". A bare host:port is treated as insecure
// (plain HTTP); pass an "https://" URL to force TLS. The standard
// OTEL_EXPORTER_OTLP_* env vars are still honoured by the exporter for
// headers, timeouts, etc.
func InitTracing(ctx context.Context, endpoint, serviceName, environment, release string) (shutdown func(context.Context) error, err error) {
	noop := func(context.Context) error { return nil }

	if endpoint == "" {
		slog.Info("observability: OTEL endpoint not set, distributed tracing disabled")
		return noop, nil
	}

	opts := []otlptracehttp.Option{otlptracehttp.WithEndpointURL(normalizeOTLPEndpoint(endpoint))}
	if !isHTTPS(endpoint) {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return noop, err
	}

	if serviceName == "" {
		serviceName = "replider-api"
	}
	if release == "" {
		release = "dev"
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(release),
			semconv.DeploymentEnvironmentName(environment),
		),
	)
	if err != nil {
		// A schema-URL conflict shouldn't sink startup; fall back to just
		// our attributes without the SDK defaults.
		res = resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(release),
			semconv.DeploymentEnvironmentName(environment),
		)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(5*time.Second)),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
	)

	otel.SetTracerProvider(tp)
	// W3C tracecontext + baggage so traces propagate across the WhatsApp
	// webhook → AI → outbound hops and to/from any upstream proxy.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	slog.Info("observability: distributed tracing enabled", "endpoint", endpoint, "service", serviceName)
	return tp.Shutdown, nil
}

// normalizeOTLPEndpoint turns a bare "host:port" into a "http://host:port"
// URL that WithEndpointURL accepts, leaving any explicit scheme intact.
func normalizeOTLPEndpoint(endpoint string) string {
	if isHTTPS(endpoint) || hasScheme(endpoint, "http://") {
		return endpoint
	}
	return "http://" + endpoint
}

func isHTTPS(endpoint string) bool { return hasScheme(endpoint, "https://") }

func hasScheme(endpoint, scheme string) bool {
	return len(endpoint) >= len(scheme) && equalFoldPrefix(endpoint, scheme)
}

func equalFoldPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != prefix[i] {
			return false
		}
	}
	return true
}

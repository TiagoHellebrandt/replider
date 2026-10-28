package observability

import (
	"context"
	"testing"
)

func TestInitTracing_DisabledWhenEndpointEmpty(t *testing.T) {
	shutdown, err := InitTracing(context.Background(), "", "replider-api", "test", "")
	if err != nil {
		t.Fatalf("expected no error with empty endpoint, got %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown func")
	}
	// No-op shutdown must be safe to call.
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("no-op shutdown returned error: %v", err)
	}
}

func TestInitTracing_EnabledWithEndpoint(t *testing.T) {
	// A bare host:port should configure the exporter without dialing it
	// (OTLP/HTTP connects lazily on first export), so init must succeed.
	shutdown, err := InitTracing(context.Background(), "otel-collector:4318", "replider-api", "production", "abc123")
	if err != nil {
		t.Fatalf("expected tracing to initialize, got %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately so Shutdown doesn't block on a flush to a dead collector
		_ = shutdown(ctx)
	})
}

func TestNormalizeOTLPEndpoint(t *testing.T) {
	cases := map[string]string{
		"otel-collector:4318":      "http://otel-collector:4318",
		"http://localhost:4318":    "http://localhost:4318",
		"https://otlp.example.com": "https://otlp.example.com",
		"HTTPS://otlp.example.com": "HTTPS://otlp.example.com",
		"localhost:4318/v1/traces": "http://localhost:4318/v1/traces",
	}
	for in, want := range cases {
		if got := normalizeOTLPEndpoint(in); got != want {
			t.Errorf("normalizeOTLPEndpoint(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsHTTPS(t *testing.T) {
	if !isHTTPS("https://x") {
		t.Error("expected https:// to be detected")
	}
	if isHTTPS("http://x") {
		t.Error("http:// must not be HTTPS")
	}
	if isHTTPS("host:4318") {
		t.Error("bare host must not be HTTPS")
	}
}

package services

import (
	"context"
	"errors"
	"testing"

	"github.com/replider/api/internal/adapters/ai"
	"github.com/replider/api/internal/config"
	"github.com/replider/api/internal/core/domain"
)

// Product-PDF import and the scanned-contract reader both ask the same
// question — can THIS account's provider read a file? — and both ask it by
// type-asserting domain.DocumentReader on whatever the factory returned.
// The factory returns breaker(retry(tracing(provider))), and for one release
// none of those three declared GenerateFromDocument. So the assertion
// answered "no" for every account on every upload, Gemini included: each
// import died with PRODUCT_PDF_PROVIDER_CANT_READ after uploading the whole
// file. No unit test noticed, because every one of them injects a fake that
// implements DocumentReader directly, skipping the chain the app actually
// builds. These tests exercise the chain.

// docReader is a provider that CAN read documents.
type docReader struct {
	stubProvider
	calls int
	resp  *domain.AICallbackResponse
	err   error
}

func (d *docReader) GenerateFromDocument(context.Context, string, string, domain.AIDocument) (*domain.AICallbackResponse, error) {
	d.calls++
	return d.resp, d.err
}

func TestFactoryChain_KeepsDocumentReaderCapability(t *testing.T) {
	f := NewAIFactory(&config.Config{AIProvider: "gemini"})
	// A syntactically valid key is enough: the Gemini client is constructed
	// lazily and no request leaves the process here.
	p, err := f.buildProvider(context.Background(), "google", "gemini-3.7-flash", "", "test-key", "acct-1")
	if err != nil {
		t.Fatalf("buildProvider: %v", err)
	}
	if _, ok := p.(domain.DocumentReader); !ok {
		t.Fatal("the decorated provider does not satisfy domain.DocumentReader — " +
			"PDF import and scanned-contract reading are dead for every account")
	}
}

// The same chain, the same silent loss: with AI_BATCH_DRAFTS on, an opaque
// chain answers ErrBatchUnsupported forever and every night runs at full
// price — and the worker treats that as "batch is down", so nothing complains.
func TestFactoryChain_KeepsBatchCapability(t *testing.T) {
	f := NewAIFactory(&config.Config{AIProvider: "gemini"})
	p, err := f.buildProvider(context.Background(), "google", "gemini-3.7-flash", "", "test-key", "acct-1")
	if err != nil {
		t.Fatalf("buildProvider: %v", err)
	}
	if _, ok := p.(domain.AIBatchProvider); !ok {
		t.Fatal("the decorated provider does not satisfy domain.AIBatchProvider — " +
			"overnight batch drafting can never engage")
	}
}

func TestDecorators_ReportUnsupportedBatch(t *testing.T) {
	chain := NewBreakerProvider(ai.NewRetryProvider(NewTracingProvider(&stubProvider{}, "openai")),
		NewCircuitBreaker())
	batcher, ok := chain.(domain.AIBatchProvider)
	if !ok {
		t.Fatal("decorators must declare the batch methods unconditionally")
	}
	if _, err := batcher.SubmitBatch(context.Background(), "x", nil); !errors.Is(err, domain.ErrBatchUnsupported) {
		t.Fatalf("SubmitBatch got %v, want domain.ErrBatchUnsupported", err)
	}
	if _, _, err := batcher.FetchBatch(context.Background(), "ref"); !errors.Is(err, domain.ErrBatchUnsupported) {
		t.Fatalf("FetchBatch got %v, want domain.ErrBatchUnsupported", err)
	}
}

func TestDecorators_ForwardDocumentRead(t *testing.T) {
	want := &domain.AICallbackResponse{Content: "[]"}
	inner := &docReader{resp: want}
	chain := NewBreakerProvider(ai.NewRetryProvider(NewTracingProvider(inner, "gemini")),
		NewCircuitBreaker())

	reader, ok := chain.(domain.DocumentReader)
	if !ok {
		t.Fatal("chain is not a DocumentReader")
	}
	got, err := reader.GenerateFromDocument(context.Background(), "sys", "ask",
		domain.AIDocument{MIMEType: "application/pdf", Data: []byte("%PDF-1.4")})
	if err != nil {
		t.Fatalf("GenerateFromDocument: %v", err)
	}
	if got != want {
		t.Fatalf("response did not come from the inner provider: %#v", got)
	}
	if inner.calls != 1 {
		t.Fatalf("inner called %d times, want 1", inner.calls)
	}
}

// A decorator must declare the method (a Go method set is static), so the
// honest "cannot read" answer has to survive as an ERROR — otherwise the
// caller sees a generic failure and blames the file.
func TestDecorators_ReportUnsupportedWhenInnerCannotRead(t *testing.T) {
	cases := map[string]domain.AIProvider{
		"tracing": NewTracingProvider(&stubProvider{}, "openai"),
		"retry":   ai.NewRetryProvider(&stubProvider{}),
		"breaker": NewBreakerProvider(&stubProvider{}, NewCircuitBreaker()),
		"chain": NewBreakerProvider(ai.NewRetryProvider(NewTracingProvider(&stubProvider{}, "openai")),
			NewCircuitBreaker()),
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			reader, ok := p.(domain.DocumentReader)
			if !ok {
				t.Fatal("decorator must declare GenerateFromDocument unconditionally")
			}
			_, err := reader.GenerateFromDocument(context.Background(), "", "", domain.AIDocument{})
			if !errors.Is(err, domain.ErrAIDocumentUnsupported) {
				t.Fatalf("got %v, want domain.ErrAIDocumentUnsupported", err)
			}
		})
	}
}

// An unreadable-provider refusal is a capability answer, not an outage: it
// must never count toward tripping the account's breaker.
func TestBreaker_UnsupportedDocumentDoesNotTrip(t *testing.T) {
	cb := NewCircuitBreaker()
	p := NewBreakerProvider(&stubProvider{}, cb).(domain.DocumentReader)
	for i := 0; i < 10; i++ {
		_, _ = p.GenerateFromDocument(context.Background(), "", "", domain.AIDocument{})
	}
	if cb.State() != CircuitClosed {
		t.Fatalf("breaker state %v — a provider that simply cannot read files must not trip it", cb.State())
	}
}

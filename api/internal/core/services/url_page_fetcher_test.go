package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// newTestFetcher returns a fetcher wired to a transport that overrides
// the default SSRF guard — tests hit httptest.Server loopback URLs,
// which the public-IP check would otherwise reject. We keep the
// resource limits in place so the test still exercises size + timeout
// enforcement.
func newTestFetcher(client *http.Client) *URLPageFetcher {
	return &URLPageFetcher{
		client:    client,
		maxBytes:  DefaultFetcherMaxBytes,
		timeout:   DefaultFetcherTimeout,
		userAgent: DefaultFetcherUserAgent,
		// Permissive validator: accept http/https loopback so httptest
		// URLs resolve. Still rejects malformed input.
		validate: func(raw string) (*url.URL, error) {
			u, err := url.Parse(raw)
			if err != nil {
				return nil, fmt.Errorf("%w: %v", ErrInvalidURL, err)
			}
			if u.Scheme != "http" && u.Scheme != "https" {
				return nil, fmt.Errorf("%w: unsupported scheme", ErrInvalidURL)
			}
			return u, nil
		},
	}
}

// handlerFunc wraps an http.HandlerFunc and the server URL so tests
// can craft minimal servers inline.
func startServer(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(h)
}

func TestURLPageFetcher_HappyPath(t *testing.T) {
	body := "<html><body><h1>Política de Privacidade</h1>" +
		"<p>" + strings.Repeat("Texto de exemplo com conteúdo suficiente. ", 20) + "</p></body></html>"
	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != DefaultFetcherUserAgent {
			t.Errorf("expected User-Agent %q, got %q",
				DefaultFetcherUserAgent, r.Header.Get("User-Agent"))
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(body))
	})
	defer srv.Close()

	f := newTestFetcher(srv.Client())
	text, err := f.FetchAndExtract(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "Política de Privacidade") {
		t.Errorf("extracted text missing the H1; got %q", text[:min(120, len(text))])
	}
	if len(text) < MinExtractedChars {
		t.Errorf("extracted text too short: %d", len(text))
	}
}

func TestURLPageFetcher_404IsUnreachable(t *testing.T) {
	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	defer srv.Close()

	f := newTestFetcher(srv.Client())
	_, err := f.FetchAndExtract(context.Background(), srv.URL)
	if !errors.Is(err, ErrURLUnreachable) {
		t.Fatalf("expected ErrURLUnreachable, got %v", err)
	}
}

func TestURLPageFetcher_TooLarge(t *testing.T) {
	// Reply with exactly 3 MiB of padding so we exceed the 2 MiB cap
	// even after HTML tag overhead.
	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		huge := strings.Repeat("a", 3*1024*1024)
		_, _ = w.Write([]byte("<html><body>" + huge + "</body></html>"))
	})
	defer srv.Close()

	f := newTestFetcher(srv.Client())
	_, err := f.FetchAndExtract(context.Background(), srv.URL)
	if !errors.Is(err, ErrURLTooLarge) {
		t.Fatalf("expected ErrURLTooLarge, got %v", err)
	}
}

func TestURLPageFetcher_EmptyContentRejected(t *testing.T) {
	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body><p>oi</p></body></html>"))
	})
	defer srv.Close()

	f := newTestFetcher(srv.Client())
	_, err := f.FetchAndExtract(context.Background(), srv.URL)
	if !errors.Is(err, ErrContentTooShort) {
		t.Fatalf("expected ErrContentTooShort, got %v", err)
	}
}

func TestURLPageFetcher_UnsupportedContentType(t *testing.T) {
	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4"))
	})
	defer srv.Close()

	f := newTestFetcher(srv.Client())
	_, err := f.FetchAndExtract(context.Background(), srv.URL)
	if !errors.Is(err, ErrURLUnsupportedCT) {
		t.Fatalf("expected ErrURLUnsupportedCT, got %v", err)
	}
}

func TestURLPageFetcher_Timeout(t *testing.T) {
	// Server that blocks past the fetcher's configured timeout.
	srv := startServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>too late</body></html>"))
	})
	defer srv.Close()

	f := newTestFetcher(srv.Client())
	// Shrink the per-request timeout to 50ms so the test is fast.
	f.timeout = 50 * time.Millisecond
	_, err := f.FetchAndExtract(context.Background(), srv.URL)
	if !errors.Is(err, ErrURLTimeout) {
		t.Fatalf("expected ErrURLTimeout, got %v", err)
	}
}

// min helper for Go <1.21 parity inside tests.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

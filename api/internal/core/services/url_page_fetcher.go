package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// URLPageFetcher is the single-page variant used by the async ingestion
// worker. The older CrawlURL walks same-origin links BFS; this one
// fetches one URL, enforces stricter content-type + size limits, and
// surfaces structured errors so the job row can carry a friendly
// message back to the UI without leaking stack traces.
//
// Kept as a separate entry point (instead of folded into url_crawler.go)
// so the mission's "UMA URL por vez" contract stays explicit and the
// multi-page crawler keeps its own limits without branching.
type URLPageFetcher struct {
	client    *http.Client
	maxBytes  int64
	timeout   time.Duration
	userAgent string
	// validate is the SSRF + scheme guard. Kept as a field (rather
	// than a direct call to validatePublicHTTPSURL) so tests can
	// substitute a permissive validator and point the fetcher at
	// httptest.NewTLSServer loopback URLs.
	validate func(rawURL string) (*url.URL, error)
}

const (
	// DefaultFetcherMaxBytes caps each fetched page at 2 MiB. Anything
	// larger is almost always a noisy marketing page; the UI surfaces
	// ErrURLTooLarge so the user can paste a more specific URL.
	DefaultFetcherMaxBytes = int64(2 * 1024 * 1024)
	// DefaultFetcherTimeout bounds each fetch at 15s, per the mission.
	DefaultFetcherTimeout = 15 * time.Second
	// DefaultFetcherUserAgent identifies us politely to the upstream.
	DefaultFetcherUserAgent = "Replider-Bot/1.0"
	// MinExtractedChars is the floor below which a page is considered
	// too thin to usefully RAG against. Pages that JS-render all
	// content will fall under this and surface ErrContentTooShort.
	MinExtractedChars = 200
)

// Structured errors the worker maps onto ingestion_jobs.error_message.
// They carry user-facing Portuguese copy — the HTTP boundary is always
// a UC away so this is acceptable.
var (
	ErrURLUnreachable   = errors.New("Site não acessível")
	ErrContentTooShort  = errors.New("Conteúdo insuficiente no link")
	ErrURLTooLarge      = errors.New("Página muito grande. Tente uma URL mais específica")
	ErrURLTimeout       = errors.New("Tempo excedido")
	ErrURLUnsupportedCT = errors.New("Tipo de conteúdo não suportado — use uma página HTML")
)

// NewURLPageFetcher returns a fetcher with production defaults. The
// HTTP client inherits SSRF-guarded redirects from the same helper as
// CrawlURL; we keep the guard because the worker accepts raw user
// input and the first defense against metadata exfiltration stays
// the validation hop.
func NewURLPageFetcher() *URLPageFetcher {
	f := &URLPageFetcher{
		maxBytes:  DefaultFetcherMaxBytes,
		timeout:   DefaultFetcherTimeout,
		userAgent: DefaultFetcherUserAgent,
		validate:  validatePublicHTTPSURL,
	}
	f.client = &http.Client{
		Timeout: DefaultFetcherTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if _, err := f.validate(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}
	return f
}

// FetchAndExtract fetches rawURL, enforces the content-type allow-list
// (text/html or application/xhtml+xml), caps the body size, and
// returns the extracted plain text. Returns one of the structured
// ErrURL* errors on known failure modes.
func (f *URLPageFetcher) FetchAndExtract(ctx context.Context, rawURL string) (string, error) {
	u, err := f.validate(rawURL)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrURLUnreachable, err)
	}

	fetchCtx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrURLUnreachable, err)
	}
	req.Header.Set("User-Agent", f.userAgent)
	req.Header.Set("Accept", "text/html, application/xhtml+xml")

	resp, err := f.client.Do(req)
	if err != nil {
		if fetchCtx.Err() == context.DeadlineExceeded {
			return "", ErrURLTimeout
		}
		return "", fmt.Errorf("%w: %v", ErrURLUnreachable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("%w: HTTP %d", ErrURLUnreachable, resp.StatusCode)
	}

	if !isAllowedContentType(resp.Header.Get("Content-Type")) {
		return "", ErrURLUnsupportedCT
	}

	// Read one byte past the cap so we can detect overflow — if the
	// server advertised a conforming Content-Length but streams more,
	// we reject rather than silently truncate.
	limited := io.LimitReader(resp.Body, f.maxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		if fetchCtx.Err() == context.DeadlineExceeded {
			return "", ErrURLTimeout
		}
		return "", fmt.Errorf("%w: %v", ErrURLUnreachable, err)
	}
	if int64(len(body)) > f.maxBytes {
		return "", ErrURLTooLarge
	}

	text, _, err := parseHTMLForTextAndLinks(strings.NewReader(string(body)), u)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrURLUnreachable, err)
	}
	text = strings.TrimSpace(text)
	if len(text) < MinExtractedChars {
		return "", ErrContentTooShort
	}
	return text, nil
}

// isAllowedContentType enforces the allow-list. Content-Type can carry
// a charset (e.g. "text/html; charset=utf-8") so we match the prefix
// rather than equality.
func isAllowedContentType(ct string) bool {
	if ct == "" {
		// Some CDNs omit the header; assume HTML so we don't reject
		// legitimate content over a cosmetic issue. The parser would
		// surface garbage on non-HTML anyway.
		return true
	}
	ct = strings.ToLower(strings.TrimSpace(ct))
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	return ct == "text/html" || ct == "application/xhtml+xml"
}

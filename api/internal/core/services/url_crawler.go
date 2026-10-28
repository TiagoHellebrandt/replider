package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// URLCrawlerLimits governs both resource usage and the blast radius the
// crawler can reach. The defaults are conservative on purpose — the
// goal is ingesting a business's FAQ / about-us pages, not scraping
// an entire site.
type URLCrawlerLimits struct {
	// MaxPages is the hard cap on how many HTML pages we'll fetch from
	// a single crawl request. A single run never exceeds this.
	MaxPages int

	// MaxBytesPerPage protects us from a hostile upstream that streams
	// gigabytes to exhaust memory. We truncate past this with io.LimitReader.
	MaxBytesPerPage int64

	// RequestTimeout is applied per fetch, not across the whole crawl.
	RequestTimeout time.Duration

	// TotalBudget is the overall deadline applied to the caller's
	// context when it has none. Prevents one bad crawl from burning
	// the server.
	TotalBudget time.Duration
}

// DefaultCrawlerLimits returns sensible production defaults.
func DefaultCrawlerLimits() URLCrawlerLimits {
	return URLCrawlerLimits{
		MaxPages:        25,
		MaxBytesPerPage: 2 << 20, // 2 MB per page
		RequestTimeout:  10 * time.Second,
		TotalBudget:     60 * time.Second,
	}
}

// CrawledPage is a single fetched URL with its extracted plain text.
type CrawledPage struct {
	URL  string
	Text string
}

// Common errors surfaced to the HTTP layer.
var (
	ErrInvalidURL         = errors.New("invalid url")
	ErrDisallowedTarget   = errors.New("target not allowed")
	ErrNoPagesCrawled     = errors.New("no pages crawled")
	ErrUnsupportedContent = errors.New("only text/html pages are indexed")
)

// CrawlURL fetches root, extracts plain text, then breadth-first walks
// same-origin links up to limits.MaxPages. Only pages with Content-Type
// text/html contribute extracted text. Errors on individual pages are
// logged and skipped — we stop only when the budget or page cap runs
// out, not on the first 404.
//
// SSRF protection: host must resolve to a public IP, scheme must be
// https, default ports only. Redirects are intercepted and re-checked
// against the same rules.
func CrawlURL(ctx context.Context, rawURL string, limits URLCrawlerLimits) ([]CrawledPage, error) {
	if limits.MaxPages <= 0 {
		limits = DefaultCrawlerLimits()
	}

	root, err := validatePublicHTTPSURL(rawURL)
	if err != nil {
		return nil, err
	}

	// Apply the global deadline only when the caller didn't give us one.
	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, limits.TotalBudget)
		defer cancel()
	}

	client := &http.Client{
		Timeout: limits.RequestTimeout,
		// Re-validate every hop: a permissive redirect is the classic
		// SSRF bypass (start on public, end on metadata.google.internal).
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if _, err := validatePublicHTTPSURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}

	seen := map[string]struct{}{root.String(): {}}
	queue := []*url.URL{root}
	var pages []CrawledPage

	for len(queue) > 0 && len(pages) < limits.MaxPages {
		select {
		case <-ctx.Done():
			return pages, nil
		default:
		}

		current := queue[0]
		queue = queue[1:]

		text, links, err := fetchAndParse(ctx, client, current, limits.MaxBytesPerPage)
		if err != nil {
			// Skip this page but keep the crawl going — partial success
			// beats forcing the user to retry the whole URL.
			continue
		}
		if text != "" {
			pages = append(pages, CrawledPage{URL: current.String(), Text: text})
		}

		for _, link := range links {
			if len(pages)+len(queue) >= limits.MaxPages {
				break
			}
			if link.Host != root.Host {
				continue
			}
			if _, ok := seen[link.String()]; ok {
				continue
			}
			seen[link.String()] = struct{}{}
			queue = append(queue, link)
		}
	}

	if len(pages) == 0 {
		return nil, ErrNoPagesCrawled
	}
	return pages, nil
}

// validatePublicHTTPSURL enforces the SSRF guard: https only, default
// port, host resolves to a public IP. Loopback (127.0.0.0/8, ::1),
// link-local (169.254.0.0/16, fe80::/10), private ranges (RFC1918,
// fc00::/7), and metadata services (169.254.169.254, metadata.google
// .internal) are all rejected.
func validatePublicHTTPSURL(rawURL string) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("%w: scheme must be https", ErrInvalidURL)
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("%w: missing host", ErrInvalidURL)
	}
	if u.Port() != "" && u.Port() != "443" {
		return nil, fmt.Errorf("%w: non-default port", ErrDisallowedTarget)
	}

	// Resolve and check every IP. A single private IP on the result
	// set is enough to reject the whole hostname — attackers can list
	// both a public decoy and an internal target and race the crawler.
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("%w: dns lookup failed", ErrDisallowedTarget)
	}
	for _, ip := range ips {
		if !isPublicIP(ip) {
			return nil, fmt.Errorf("%w: host resolves to non-public IP %s",
				ErrDisallowedTarget, ip)
		}
	}
	return u, nil
}

func isPublicIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() || ip.IsPrivate() {
		return false
	}
	// GCE/EC2 metadata: explicit deny even though 169.254.169.254 is
	// already covered by IsLinkLocalUnicast. Belt + suspenders.
	if ip.String() == "169.254.169.254" {
		return false
	}
	return true
}

func fetchAndParse(ctx context.Context, client *http.Client, u *url.URL, maxBytes int64) (string, []*url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", nil, err
	}
	// Identify ourselves politely; some servers block empty UA.
	req.Header.Set("User-Agent", "ReplderKnowledgeCrawler/1.0 (+https://replider.com)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.HasPrefix(ct, "text/html") {
		return "", nil, ErrUnsupportedContent
	}

	body := io.LimitReader(resp.Body, maxBytes)
	text, links, err := parseHTMLForTextAndLinks(body, u)
	if err != nil {
		return "", nil, err
	}
	return text, links, nil
}

// parseHTMLForTextAndLinks walks the HTML tree and collects:
//  1. Visible text (skipping <script>, <style>, <noscript> subtrees).
//  2. Absolute URLs from <a href>, resolved against the page base.
func parseHTMLForTextAndLinks(r io.Reader, base *url.URL) (string, []*url.URL, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", nil, err
	}
	var text strings.Builder
	var links []*url.URL
	seenLink := map[string]struct{}{}

	var walk func(n *html.Node, skip bool)
	walk = func(n *html.Node, skip bool) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "noscript", "svg":
				skip = true
			case "a":
				for _, a := range n.Attr {
					if a.Key != "href" {
						continue
					}
					ref, err := base.Parse(strings.TrimSpace(a.Val))
					if err != nil {
						continue
					}
					// Strip fragments; two URLs that differ only in
					// #section are the same page for RAG purposes.
					ref.Fragment = ""
					key := ref.String()
					if _, ok := seenLink[key]; ok {
						continue
					}
					seenLink[key] = struct{}{}
					links = append(links, ref)
				}
			case "br", "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "li":
				text.WriteByte('\n')
			}
		}
		if n.Type == html.TextNode && !skip {
			t := strings.TrimSpace(n.Data)
			if t != "" {
				text.WriteString(t)
				text.WriteByte(' ')
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, skip)
		}
	}
	walk(doc, false)

	return strings.TrimSpace(collapseWhitespace(text.String())), links, nil
}

// collapseWhitespace squashes runs of tabs/spaces but keeps newlines,
// so the chunker's paragraph-break heuristics still work.
func collapseWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	var prevSpace bool
	for _, r := range s {
		if r == '\n' {
			b.WriteRune(r)
			prevSpace = false
			continue
		}
		if r == ' ' || r == '\t' {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return b.String()
}

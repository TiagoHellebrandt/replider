// Package domainage answers when a domain was registered.
//
// RDAP rather than classic WHOIS: WHOIS is unstructured text that differs per
// registry and needs a parser per TLD, while RDAP is JSON with a standard
// shape. Registries have been required to serve it since 2019.
package domainage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// RDAPLookup asks the IANA bootstrap service, which redirects to whichever
// registry owns the TLD.
type RDAPLookup struct {
	client  *http.Client
	baseURL string

	// Registration dates never change, so a hit is good for the life of the
	// process. The cache exists because prospecting hits the same domains
	// repeatedly and registries rate-limit hard.
	mu    sync.RWMutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	at       *time.Time
	cachedAt time.Time
}

// cacheTTL bounds a NEGATIVE answer only. A domain we could not resolve today
// may resolve tomorrow, and caching "unknown" forever would silently disable
// the check for it.
const cacheTTL = 6 * time.Hour

// New returns a lookup, or nil when disabled.
//
// Returns the interface so the caller can compare against nil — a typed nil
// would satisfy it and then fail on every call.
func New(enabled bool, timeout time.Duration) domain.DomainAgeLookup {
	if !enabled {
		return nil
	}
	if timeout <= 0 {
		timeout = 4 * time.Second
	}
	return &RDAPLookup{
		// No SSRF guard needed: the URL is built from a constant base and a
		// domain name, never from anything a sender supplies as a URL.
		client:  &http.Client{Timeout: timeout},
		baseURL: "https://rdap.org/domain/",
		cache:   map[string]cacheEntry{},
	}
}

// RegisteredAt returns when the domain was created.
//
// (nil, nil) means the registry has no answer — plenty of TLDs redact the
// date, and a redacted date is not a young domain. Only a real date is ever
// used to flag anything.
func (r *RDAPLookup) RegisteredAt(ctx context.Context, dom string) (*time.Time, error) {
	dom = strings.ToLower(strings.TrimSpace(dom))
	if dom == "" || strings.ContainsAny(dom, "/?#@ ") {
		return nil, nil
	}

	if hit, ok := r.cached(dom); ok {
		return hit, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+dom, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/rdap+json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rdap: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		// The registry does not know this domain. Not an error, and not a
		// signal: an unregistered domain cannot have sent mail, so this is
		// almost always a TLD rdap.org cannot route.
		r.store(dom, nil)
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rdap: status %d", resp.StatusCode)
	}

	var body struct {
		Events []struct {
			Action string `json:"eventAction"`
			Date   string `json:"eventDate"`
		} `json:"events"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("rdap: %w", err)
	}

	for _, e := range body.Events {
		if !strings.EqualFold(e.Action, "registration") {
			continue
		}
		at, parseErr := time.Parse(time.RFC3339, e.Date)
		if parseErr != nil {
			continue
		}
		r.store(dom, &at)
		return &at, nil
	}
	// Answered, but with the date redacted. Cached as unknown so the next
	// message from the same sender does not pay for the round trip again.
	r.store(dom, nil)
	return nil, nil
}

func (r *RDAPLookup) cached(dom string) (*time.Time, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.cache[dom]
	if !ok {
		return nil, false
	}
	// A known date is permanent; an unknown one is worth retrying later.
	if e.at == nil && time.Since(e.cachedAt) > cacheTTL {
		return nil, false
	}
	return e.at, true
}

func (r *RDAPLookup) store(dom string, at *time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Bound the map: prospecting can touch thousands of domains, and this
	// cache must not become the process's memory profile.
	if len(r.cache) > 5000 {
		r.cache = map[string]cacheEntry{}
	}
	r.cache[dom] = cacheEntry{at: at, cachedAt: time.Now()}
}

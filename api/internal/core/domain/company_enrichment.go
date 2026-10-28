package domain

import (
	"context"
	"encoding/json"
	"time"
)

// Shared company enrichment cache (US-M001).
//
// Two customers prospecting the same hospital should not each pay for the
// same lookup. The cache is keyed by CNPJ and shared across accounts — which
// is exactly why what may enter it is restricted.
//
// The LGPD line: a company's website, address and CNAE are facts about an
// organization. A decision-maker's name, role or LinkedIn is personal data
// that belongs to the account that discovered it. Sharing the first is a
// moat; sharing the second is desvio de finalidade (Art. 6º I).

// CompanyEnrichmentTTL bounds how long a cached row is trusted. 90 days
// matches how fast the facts we store actually move (a clinic changes CNAE
// or address rarely); anything older is re-fetched rather than served.
const CompanyEnrichmentTTL = 90 * 24 * time.Hour

// CompanyEnrichment is one cached, company-level profile.
type CompanyEnrichment struct {
	CNPJ      string          `json:"cnpj"`
	Payload   json.RawMessage `json:"payload"`
	Source    string          `json:"source"`
	FetchedAt time.Time       `json:"fetched_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// IsFresh reports whether the row is still within the TTL at time now.
func (c *CompanyEnrichment) IsFresh(now time.Time) bool {
	return now.Sub(c.FetchedAt) < CompanyEnrichmentTTL
}

// companyLevelKeys is the ALLOWLIST of fields the shared cache may hold.
//
// An allowlist, not a denylist: a denylist silently shares every field nobody
// thought to exclude, and the fields we must not share are precisely the ones
// a future provider will invent names for.
var companyLevelKeys = map[string]struct{}{
	"company_name":  {},
	"legal_name":    {},
	"trade_name":    {},
	"website":       {},
	"address":       {},
	"city":          {},
	"state":         {},
	"zip":           {},
	"cnae":          {},
	"cnae_label":    {},
	"segment":       {},
	"size":          {},
	"employees":     {},
	"revenue_range": {},
	"founded_year":  {},
	"opening_date":  {},
	"sources":       {},
	"place_id":      {},
}

// SanitizeCompanyPayload keeps only company-level keys. Everything else —
// a decision maker's name, a personal LinkedIn, a mobile number — is dropped
// before the row can be shared with another tenant.
//
// Returns the filtered map and whether anything survived: an enrichment made
// entirely of personal data has nothing worth caching.
func SanitizeCompanyPayload(raw json.RawMessage) (json.RawMessage, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var in map[string]any
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, false
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		if _, ok := companyLevelKeys[k]; ok {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return nil, false
	}
	return encoded, true
}

// CompanyEnrichmentRepository is the cache port. Note the absence of an
// accountID parameter anywhere: the cache is deliberately cross-tenant, and
// the sanitizer above is what makes that safe.
type CompanyEnrichmentRepository interface {
	// Get returns the cached row, or (nil, nil) when absent. Freshness is
	// the caller's decision (IsFresh) so a stale row can still be used as a
	// fallback when the provider is down.
	Get(ctx context.Context, cnpj string) (*CompanyEnrichment, error)
	// Upsert stores/refreshes the row. The payload MUST already be
	// sanitized; the repository re-applies the allowlist as a backstop.
	Upsert(ctx context.Context, entry *CompanyEnrichment) (*CompanyEnrichment, error)
}

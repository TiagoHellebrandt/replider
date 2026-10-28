package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// CompanyLookup is the cache-in-front-of-provider path for company data
// (US-M001 + US-M002).
//
// Every account prospecting the same vertical hits the same CNPJs, so the
// naive design pays a provider call per account per company. This puts the
// shared cache in front: a fresh row answers in a millisecond and costs
// nothing, and the provider is only dialled on a miss.
//
// What makes the sharing legal is upstream: only company-level fields are
// ever written (domain.SanitizeCompanyPayload). A decision maker's name never
// reaches this table.
type CompanyLookup struct {
	cache    domain.CompanyEnrichmentRepository
	provider domain.CompanyDataProvider
	// now is injectable so TTL expiry is testable without sleeping 90 days.
	now func() time.Time
}

func NewCompanyLookup(cache domain.CompanyEnrichmentRepository, provider domain.CompanyDataProvider) *CompanyLookup {
	return &CompanyLookup{cache: cache, provider: provider, now: time.Now}
}

// WithClock overrides the clock (tests).
func (l *CompanyLookup) WithClock(now func() time.Time) *CompanyLookup {
	l.now = now
	return l
}

// LookupResult carries the record plus where it came from, so callers (and
// tests) can tell a cache hit from a provider call — and so the UI can be
// honest about data age.
type LookupResult struct {
	Record    *domain.CompanyRecord
	FromCache bool
	FetchedAt time.Time
}

// ByCNPJ resolves one company, preferring a fresh cached row.
//
// Degradation ladder, in order:
//  1. fresh cache row → return it, no provider call;
//  2. miss/stale → provider, then refresh the cache;
//  3. provider fails but a STALE row exists → return the stale row. Month-old
//     company facts beat no answer: the alternative is blocking a
//     salesperson's prospecting because a vendor is down.
func (l *CompanyLookup) ByCNPJ(ctx context.Context, cnpj string) (*LookupResult, error) {
	digits := domain.NormalizeCNPJ(cnpj)
	if len(digits) != 14 {
		return nil, domain.ErrInvalidCNPJ
	}

	var stale *domain.CompanyEnrichment
	if l.cache != nil {
		cached, err := l.cache.Get(ctx, digits)
		if err != nil {
			// A cache read failure must not break the lookup — it only
			// costs a provider call.
			slog.WarnContext(ctx, "company lookup: cache read failed", "cnpj", digits, "error", err)
		} else if cached != nil {
			if cached.IsFresh(l.now()) {
				return &LookupResult{
					Record:    recordFromPayload(digits, cached.Payload),
					FromCache: true,
					FetchedAt: cached.FetchedAt,
				}, nil
			}
			stale = cached
		}
	}

	if l.provider == nil {
		if stale != nil {
			return &LookupResult{Record: recordFromPayload(digits, stale.Payload), FromCache: true, FetchedAt: stale.FetchedAt}, nil
		}
		return nil, domain.ErrDataProviderUnconfigured
	}

	record, err := l.provider.Lookup(ctx, digits)
	if err != nil {
		if stale != nil {
			slog.WarnContext(ctx, "company lookup: provider failed, serving stale cache",
				"cnpj", digits, "age", l.now().Sub(stale.FetchedAt).String(), "error", err)
			return &LookupResult{Record: recordFromPayload(digits, stale.Payload), FromCache: true, FetchedAt: stale.FetchedAt}, nil
		}
		return nil, err
	}
	if record == nil {
		return nil, nil
	}

	l.store(ctx, *record)
	return &LookupResult{Record: record, FromCache: false, FetchedAt: l.now()}, nil
}

// Search runs a prospecting query against the provider and warms the cache
// with every result. Search itself is never served FROM the cache: a filter
// answer is only as good as the index behind it, and a stale local set would
// silently hide companies that started matching yesterday.
func (l *CompanyLookup) Search(ctx context.Context, q domain.CompanySearchQuery) ([]domain.CompanyRecord, error) {
	if l.provider == nil {
		return nil, domain.ErrDataProviderUnconfigured
	}
	if !q.Valid() {
		return nil, domain.ErrDataProviderQueryTooWide
	}
	records, err := l.provider.Search(ctx, q)
	if err != nil {
		return nil, err
	}
	for _, r := range records {
		l.store(ctx, r)
	}
	return records, nil
}

// store writes a record into the shared cache, best-effort: the caller
// already has the data, and a cache write failure must never fail the lookup
// that produced it.
func (l *CompanyLookup) store(ctx context.Context, r domain.CompanyRecord) {
	if l.cache == nil {
		return
	}
	payload, err := json.Marshal(companyPayload(r))
	if err != nil {
		return
	}
	sanitized, ok := domain.SanitizeCompanyPayload(payload)
	if !ok {
		return
	}
	source := ""
	if l.provider != nil {
		source = l.provider.Name()
	}
	if _, err := l.cache.Upsert(ctx, &domain.CompanyEnrichment{
		CNPJ:      r.CNPJ,
		Payload:   sanitized,
		Source:    source,
		FetchedAt: l.now(),
	}); err != nil {
		slog.WarnContext(ctx, "company lookup: cache write failed", "cnpj", r.CNPJ, "error", err)
	}
}

// companyPayload maps a record onto the cache's allowlisted key names. Note
// what is NOT here: `phone` is a company switchboard, but it is also the
// field most likely to be someone's mobile, and this row is shared across
// tenants — so it stays out of the shared cache and lives only on the
// account's own lead.
func companyPayload(r domain.CompanyRecord) map[string]any {
	m := map[string]any{}
	set := func(k, v string) {
		if v != "" {
			m[k] = v
		}
	}
	set("company_name", r.DisplayName())
	set("legal_name", r.LegalName)
	set("trade_name", r.TradeName)
	set("cnae", r.CNAE)
	set("cnae_label", r.CNAELabel)
	set("website", r.Website)
	set("address", r.Address)
	set("city", r.City)
	set("state", r.State)
	set("zip", r.Zip)
	set("revenue_range", r.RevenueRange)
	set("opening_date", r.OpeningDate)
	if r.Employees > 0 {
		m["employees"] = r.Employees
	}
	return m
}

// recordFromPayload rebuilds a record from a cached payload. Fields the cache
// deliberately does not hold (phone, active) come back zero — callers that
// need them must go to the provider.
func recordFromPayload(cnpj string, payload json.RawMessage) *domain.CompanyRecord {
	m := map[string]any{}
	_ = json.Unmarshal(payload, &m)

	str := func(k string) string {
		if v, ok := m[k].(string); ok {
			return v
		}
		return ""
	}
	rec := &domain.CompanyRecord{
		CNPJ:         cnpj,
		LegalName:    str("legal_name"),
		TradeName:    str("trade_name"),
		CNAE:         str("cnae"),
		CNAELabel:    str("cnae_label"),
		Website:      str("website"),
		Address:      str("address"),
		City:         str("city"),
		State:        str("state"),
		Zip:          str("zip"),
		RevenueRange: str("revenue_range"),
		OpeningDate:  str("opening_date"),
	}
	if rec.TradeName == "" && rec.LegalName == "" {
		rec.TradeName = str("company_name")
	}
	if v, ok := m["employees"].(float64); ok {
		rec.Employees = int(v)
	}
	return rec
}

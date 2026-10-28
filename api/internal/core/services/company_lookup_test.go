package services

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-M001/M002. The cache exists to stop paying a data provider once per
// account for the same company, so the assertions here count provider calls —
// a cache that returns the right answer while still dialling the provider has
// achieved nothing.

// countingProvider records how many times it was actually called.
type countingProvider struct {
	mu       sync.Mutex
	lookups  int
	searches int
	record   *domain.CompanyRecord
	records  []domain.CompanyRecord
	err      error
}

func (p *countingProvider) Name() string { return "counting" }

func (p *countingProvider) Lookup(_ context.Context, cnpj string) (*domain.CompanyRecord, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lookups++
	if p.err != nil {
		return nil, p.err
	}
	if p.record == nil {
		return nil, nil
	}
	cp := *p.record
	cp.CNPJ = cnpj
	return &cp, nil
}

func (p *countingProvider) Search(_ context.Context, _ domain.CompanySearchQuery) ([]domain.CompanyRecord, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.searches++
	return p.records, p.err
}

func (p *countingProvider) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lookups
}

// memCache is an in-memory stand-in for the Postgres cache that applies the
// same sanitizer, so a test cannot "pass" by storing something the real
// repository would strip.
type memCache struct {
	mu   sync.Mutex
	rows map[string]*domain.CompanyEnrichment
}

func newMemCache() *memCache { return &memCache{rows: map[string]*domain.CompanyEnrichment{}} }

func (c *memCache) Get(_ context.Context, cnpj string) (*domain.CompanyEnrichment, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	row, ok := c.rows[domain.NormalizeCNPJ(cnpj)]
	if !ok {
		return nil, nil
	}
	cp := *row
	return &cp, nil
}

func (c *memCache) Upsert(_ context.Context, e *domain.CompanyEnrichment) (*domain.CompanyEnrichment, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	payload, ok := domain.SanitizeCompanyPayload(e.Payload)
	if !ok {
		return nil, nil
	}
	cp := *e
	cp.CNPJ = domain.NormalizeCNPJ(e.CNPJ)
	cp.Payload = payload
	c.rows[cp.CNPJ] = &cp
	return &cp, nil
}

const testCNPJ = "12345678000199"

func sampleRecord() *domain.CompanyRecord {
	return &domain.CompanyRecord{
		CNPJ:      testCNPJ,
		LegalName: "HOSPITAL SAO LUCAS LTDA",
		TradeName: "Hospital São Lucas",
		CNAE:      "8610101",
		CNAELabel: "Atividades de atendimento hospitalar",
		Website:   "https://saolucas.example.com",
		City:      "São Paulo",
		State:     "SP",
		Employees: 240,
		Phone:     "+551133334444",
		Active:    true,
	}
}

func TestCompanyLookup_CacheHitDoesNotCallProvider(t *testing.T) {
	ctx := context.Background()
	provider := &countingProvider{record: sampleRecord()}
	lookup := NewCompanyLookup(newMemCache(), provider)

	first, err := lookup.ByCNPJ(ctx, "12.345.678/0001-99")
	require.NoError(t, err)
	require.NotNil(t, first.Record)
	assert.False(t, first.FromCache)
	assert.Equal(t, 1, provider.callCount())

	// Second call — same company, possibly a different account entirely.
	second, err := lookup.ByCNPJ(ctx, testCNPJ)
	require.NoError(t, err)
	require.NotNil(t, second.Record)
	assert.True(t, second.FromCache, "the second lookup must be served from cache")
	assert.Equal(t, 1, provider.callCount(), "a cache hit must NOT dial the provider")
	assert.Equal(t, "Hospital São Lucas", second.Record.DisplayName())
	assert.Equal(t, "8610101", second.Record.CNAE)
}

func TestCompanyLookup_ExpiredEntryRefetches(t *testing.T) {
	ctx := context.Background()
	provider := &countingProvider{record: sampleRecord()}
	cache := newMemCache()
	now := time.Now()
	lookup := NewCompanyLookup(cache, provider).WithClock(func() time.Time { return now })

	_, err := lookup.ByCNPJ(ctx, testCNPJ)
	require.NoError(t, err)
	require.Equal(t, 1, provider.callCount())

	// Just inside the TTL: still a hit.
	now = now.Add(domain.CompanyEnrichmentTTL - time.Hour)
	res, err := lookup.ByCNPJ(ctx, testCNPJ)
	require.NoError(t, err)
	assert.True(t, res.FromCache)
	assert.Equal(t, 1, provider.callCount())

	// Past the TTL: the facts may have moved, so re-fetch.
	now = now.Add(2 * time.Hour)
	res, err = lookup.ByCNPJ(ctx, testCNPJ)
	require.NoError(t, err)
	assert.False(t, res.FromCache)
	assert.Equal(t, 2, provider.callCount())
}

// The cache is shared across tenants, so this is the test that keeps it
// legal: nothing about a PERSON may be written, no matter what the provider
// returns.
func TestCompanyLookup_CachedPayloadHasNoPersonalData(t *testing.T) {
	ctx := context.Background()
	cache := newMemCache()
	lookup := NewCompanyLookup(cache, &countingProvider{record: sampleRecord()})

	_, err := lookup.ByCNPJ(ctx, testCNPJ)
	require.NoError(t, err)

	row, err := cache.Get(ctx, testCNPJ)
	require.NoError(t, err)
	require.NotNil(t, row)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(row.Payload, &payload))

	for _, forbidden := range []string{
		"decision_maker_name", "decision_maker_role", "linkedin_url",
		"instagram_url", "email", "phone", "contact_name", "cpf",
	} {
		_, present := payload[forbidden]
		assert.False(t, present, "%q must never reach the shared cache", forbidden)
	}
	// ...while the company facts that justify the cache DID land.
	assert.Equal(t, "8610101", payload["cnae"])
	assert.Equal(t, "https://saolucas.example.com", payload["website"])
}

// Even if a caller hands the cache an enrichment full of personal fields
// directly, the sanitizer drops the row rather than storing a filtered
// husk of someone's contact details.
func TestSanitizeCompanyPayload_RejectsPurelyPersonalPayload(t *testing.T) {
	raw := json.RawMessage(`{"decision_maker_name":"Dra. Ana","linkedin_url":"https://linkedin.com/in/ana"}`)
	_, ok := domain.SanitizeCompanyPayload(raw)
	assert.False(t, ok, "a payload with nothing company-level must not be cached")

	mixed := json.RawMessage(`{"decision_maker_name":"Dra. Ana","cnae":"8610101"}`)
	out, ok := domain.SanitizeCompanyPayload(mixed)
	require.True(t, ok)
	assert.NotContains(t, string(out), "Ana")
	assert.Contains(t, string(out), "8610101")
}

// A provider outage must not block prospecting when we already know the
// company: month-old facts beat no answer.
func TestCompanyLookup_ServesStaleWhenProviderFails(t *testing.T) {
	ctx := context.Background()
	provider := &countingProvider{record: sampleRecord()}
	cache := newMemCache()
	now := time.Now()
	lookup := NewCompanyLookup(cache, provider).WithClock(func() time.Time { return now })

	_, err := lookup.ByCNPJ(ctx, testCNPJ)
	require.NoError(t, err)

	now = now.Add(domain.CompanyEnrichmentTTL + time.Hour) // stale
	provider.err = errors.New("speedio: 503")

	res, err := lookup.ByCNPJ(ctx, testCNPJ)
	require.NoError(t, err, "a stale row is better than failing the salesperson's search")
	require.NotNil(t, res.Record)
	assert.True(t, res.FromCache)
	assert.Equal(t, "8610101", res.Record.CNAE)
}

func TestCompanyLookup_NoStaleRowSurfacesProviderError(t *testing.T) {
	ctx := context.Background()
	provider := &countingProvider{err: errors.New("speedio: 503")}
	lookup := NewCompanyLookup(newMemCache(), provider)

	_, err := lookup.ByCNPJ(ctx, testCNPJ)
	assert.Error(t, err, "with nothing cached there is nothing to degrade to")
}

// Search is never served from cache — a stale local set would hide companies
// that started matching yesterday — but its results DO warm it.
func TestCompanyLookup_SearchWarmsTheCache(t *testing.T) {
	ctx := context.Background()
	provider := &countingProvider{records: []domain.CompanyRecord{*sampleRecord()}}
	cache := newMemCache()
	lookup := NewCompanyLookup(cache, provider)

	got, err := lookup.Search(ctx, domain.CompanySearchQuery{CNAEs: []string{"8610101"}, UF: "SP"})
	require.NoError(t, err)
	require.Len(t, got, 1)

	res, err := lookup.ByCNPJ(ctx, testCNPJ)
	require.NoError(t, err)
	assert.True(t, res.FromCache, "a searched company must not be looked up again one by one")
	assert.Zero(t, provider.callCount())
}

func TestCompanyLookup_RejectsTooWideSearch(t *testing.T) {
	lookup := NewCompanyLookup(newMemCache(), &countingProvider{})
	_, err := lookup.Search(context.Background(), domain.CompanySearchQuery{})
	assert.ErrorIs(t, err, domain.ErrDataProviderQueryTooWide)
}

package e2e

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-M001 against a real Postgres. The in-memory tests in core/services pin
// the policy (call counts, TTL, sanitizing); these pin the things only the
// database can answer: the JSONB merge, the cross-account read, and the fact
// that the repository re-sanitizes even when a caller does not.

func TestCompanyEnrichmentCache_MergesProvidersInsteadOfReplacing(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewCompanyEnrichmentRepository(testPool)

	// Speedio knows the registry facts...
	_, err := repo.Upsert(ctx, &domain.CompanyEnrichment{
		CNPJ:      "12.345.678/0001-99",
		Source:    "speedio",
		Payload:   json.RawMessage(`{"cnae":"8610101","city":"São Paulo","employees":240}`),
		FetchedAt: time.Now(),
	})
	require.NoError(t, err)

	// ...a grounded search knows the web facts. Neither is complete; the row
	// must end up with both, or the second provider erases the first.
	merged, err := repo.Upsert(ctx, &domain.CompanyEnrichment{
		CNPJ:      "12345678000199",
		Source:    "gemini_grounding",
		Payload:   json.RawMessage(`{"website":"https://saolucas.example.com","sources":["https://saolucas.example.com/sobre"]}`),
		FetchedAt: time.Now(),
	})
	require.NoError(t, err)
	require.NotNil(t, merged)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(merged.Payload, &payload))
	assert.Equal(t, "8610101", payload["cnae"], "the earlier provider's facts survive")
	assert.Equal(t, "https://saolucas.example.com", payload["website"])
	assert.Equal(t, float64(240), payload["employees"], "numeric fields merge too")
	// Source records who wrote LAST — useful to invalidate a bad provider's
	// rows wholesale.
	assert.Equal(t, "gemini_grounding", merged.Source)
}

func TestCompanyEnrichmentCache_IsReadableAcrossAccounts(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewCompanyEnrichmentRepository(testPool)

	// Account A pays for the lookup...
	_, err := repo.Upsert(ctx, &domain.CompanyEnrichment{
		CNPJ:      "11222333000181",
		Source:    "speedio",
		Payload:   json.RawMessage(`{"cnae":"4645101","company_name":"Distribuidora Med"}`),
		FetchedAt: time.Now(),
	})
	require.NoError(t, err)

	// ...and account B reads it for free. There is no accountID parameter on
	// this repository AT ALL — that is the design, and the sanitizer is what
	// makes it safe.
	got, err := repo.Get(ctx, "11.222.333/0001-81")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Contains(t, string(got.Payload), "4645101")
}

// The use case sanitizes, but the repository is the last line before a shared
// table: a caller that forgets must not be able to leak one tenant's personal
// data to every other tenant.
func TestCompanyEnrichmentCache_RepositoryReSanitizes(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewCompanyEnrichmentRepository(testPool)

	_, err := repo.Upsert(ctx, &domain.CompanyEnrichment{
		CNPJ:      "11222333000181",
		Source:    "careless_caller",
		Payload:   json.RawMessage(`{"cnae":"4645101","decision_maker_name":"Dra. Ana","linkedin_url":"https://linkedin.com/in/ana"}`),
		FetchedAt: time.Now(),
	})
	require.NoError(t, err)

	got, err := repo.Get(ctx, "11222333000181")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotContains(t, string(got.Payload), "Ana")
	assert.NotContains(t, string(got.Payload), "linkedin")
	assert.Contains(t, string(got.Payload), "4645101")
}

// A payload with nothing company-level is not worth a row — storing an empty
// husk would make later lookups think the company is known and cached.
func TestCompanyEnrichmentCache_SkipsPurelyPersonalPayload(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewCompanyEnrichmentRepository(testPool)

	out, err := repo.Upsert(ctx, &domain.CompanyEnrichment{
		CNPJ:      "11222333000181",
		Payload:   json.RawMessage(`{"decision_maker_name":"Dra. Ana"}`),
		FetchedAt: time.Now(),
	})
	require.NoError(t, err)
	assert.Nil(t, out)

	got, err := repo.Get(ctx, "11222333000181")
	require.NoError(t, err)
	assert.Nil(t, got, "no row should have been created")
}

func TestCompanyEnrichmentCache_RejectsMalformedCNPJ(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewCompanyEnrichmentRepository(testPool)

	_, err := repo.Upsert(ctx, &domain.CompanyEnrichment{
		CNPJ: "123", Payload: json.RawMessage(`{"cnae":"1"}`), FetchedAt: time.Now(),
	})
	assert.ErrorIs(t, err, domain.ErrInvalidCNPJ)

	// A malformed CNPJ on read is a miss, not an error: the caller simply
	// has nothing to key on.
	got, err := repo.Get(ctx, "not-a-cnpj")
	assert.NoError(t, err)
	assert.Nil(t, got)
}

// A refresh restarts the TTL: we just confirmed the facts, so the row is as
// good as new.
func TestCompanyEnrichmentCache_UpsertRefreshesFetchedAt(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewCompanyEnrichmentRepository(testPool)

	old := time.Now().Add(-100 * 24 * time.Hour)
	_, err := repo.Upsert(ctx, &domain.CompanyEnrichment{
		CNPJ: "11222333000181", Payload: json.RawMessage(`{"cnae":"4645101"}`), FetchedAt: old,
	})
	require.NoError(t, err)

	stale, err := repo.Get(ctx, "11222333000181")
	require.NoError(t, err)
	require.NotNil(t, stale)
	// Upsert stamps NOW, ignoring the caller's timestamp — the row was just
	// written, whatever the caller claims.
	assert.True(t, stale.IsFresh(time.Now()))
}

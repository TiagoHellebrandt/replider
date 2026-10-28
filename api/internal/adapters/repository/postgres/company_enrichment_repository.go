package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// CompanyEnrichmentRepository backs the shared, cross-account enrichment
// cache (US-M001). Raw SQL: two statements, and the ON CONFLICT semantics
// (refresh fetched_at so the TTL restarts) are the interesting part.
type CompanyEnrichmentRepository struct {
	db DBTX
}

func NewCompanyEnrichmentRepository(db DBTX) *CompanyEnrichmentRepository {
	return &CompanyEnrichmentRepository{db: db}
}

var _ domain.CompanyEnrichmentRepository = (*CompanyEnrichmentRepository)(nil)

const companyEnrichmentColumns = `cnpj, payload, source, fetched_at, updated_at`

func (r *CompanyEnrichmentRepository) Get(ctx context.Context, cnpj string) (*domain.CompanyEnrichment, error) {
	digits := domain.NormalizeCNPJ(cnpj)
	if len(digits) != 14 {
		// Not a CNPJ — a miss, not an error: the caller simply has nothing
		// to key the cache on.
		return nil, nil
	}
	var e domain.CompanyEnrichment
	err := r.db.QueryRow(ctx,
		`SELECT `+companyEnrichmentColumns+` FROM company_enrichments WHERE cnpj = $1`,
		digits,
	).Scan(&e.CNPJ, &e.Payload, &e.Source, &e.FetchedAt, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *CompanyEnrichmentRepository) Upsert(ctx context.Context, entry *domain.CompanyEnrichment) (*domain.CompanyEnrichment, error) {
	digits := domain.NormalizeCNPJ(entry.CNPJ)
	if len(digits) != 14 {
		return nil, domain.ErrInvalidCNPJ
	}
	// Backstop: the use case sanitizes, but this table is shared across
	// tenants and a single unfiltered write leaks one account's personal
	// data to every other one. Cheap enough to do twice.
	payload, ok := domain.SanitizeCompanyPayload(entry.Payload)
	if !ok {
		return nil, nil // nothing company-level worth caching
	}

	now := time.Now()
	var out domain.CompanyEnrichment
	err := r.db.QueryRow(ctx,
		`INSERT INTO company_enrichments (cnpj, payload, source, fetched_at, updated_at)
		 VALUES ($1, $2, $3, $4, $4)
		 ON CONFLICT (cnpj) DO UPDATE SET
		   -- Merge, don't replace: a Speedio row (CNAE, size) and a grounded
		   -- search row (website, sources) describe the same company from
		   -- different angles, and replacing would make them fight.
		   payload    = company_enrichments.payload || EXCLUDED.payload,
		   source     = EXCLUDED.source,
		   -- Refreshing fetched_at restarts the TTL: we just confirmed the
		   -- facts, so the row is as good as new.
		   fetched_at = $4,
		   updated_at = $4
		 RETURNING `+companyEnrichmentColumns,
		digits, payload, entry.Source, now,
	).Scan(&out.CNPJ, &out.Payload, &out.Source, &out.FetchedAt, &out.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

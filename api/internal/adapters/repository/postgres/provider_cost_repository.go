package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// ProviderCostRepository is a raw-SQL repo (NOT sqlc) over the same
// pool, owning provider_costs + platform_settings (US-005, vendedores
// v3). Same rationale as SellerRefRepository: isolated feature, keeps
// the expensive-to-extend sqlc repos untouched. Implements both
// domain.ProviderCostRepository and domain.PlatformSettingRepository.
type ProviderCostRepository struct {
	db DBTX
}

func NewProviderCostRepository(db DBTX) *ProviderCostRepository {
	return &ProviderCostRepository{db: db}
}

const providerCostSelect = `SELECT id, provider, model, input_cost_per_1m_cents_usd, output_cost_per_1m_cents_usd, effective_from, created_at FROM provider_costs`

func scanProviderCost(row pgx.Row) (*domain.ProviderCost, error) {
	var c domain.ProviderCost
	if err := row.Scan(&c.ID, &c.Provider, &c.Model, &c.InputCostPer1MCentsUSD, &c.OutputCostPer1MCentsUSD, &c.EffectiveFrom, &c.CreatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ProviderCostRepository) List(ctx context.Context) ([]*domain.ProviderCost, error) {
	rows, err := r.db.Query(ctx, providerCostSelect+` ORDER BY provider, model, effective_from DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*domain.ProviderCost{}
	for rows.Next() {
		c, err := scanProviderCost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// errProviderCostDuplicate translates the PG 23505 on the
// (provider, model, effective_from) UNIQUE into a domain error so the
// use case never inspects pg codes.
func errProviderCostDuplicate() error {
	return &domain.AppError{Code: "PROVIDER_COST_DUPLICATE", Message: "A price for this provider/model/effective_from already exists"}
}

func (r *ProviderCostRepository) Insert(ctx context.Context, c *domain.ProviderCost) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO provider_costs (id, provider, model, input_cost_per_1m_cents_usd, output_cost_per_1m_cents_usd, effective_from)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING created_at`,
		c.ID, c.Provider, c.Model, c.InputCostPer1MCentsUSD, c.OutputCostPer1MCentsUSD, c.EffectiveFrom,
	).Scan(&c.CreatedAt)
	if isUniqueViolation(err) {
		return errProviderCostDuplicate()
	}
	return err
}

func (r *ProviderCostRepository) Update(ctx context.Context, id string, inputCents, outputCents int64, effectiveFrom time.Time) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE provider_costs
		 SET input_cost_per_1m_cents_usd = $2, output_cost_per_1m_cents_usd = $3, effective_from = $4
		 WHERE id = $1`,
		id, inputCents, outputCents, effectiveFrom,
	)
	if isUniqueViolation(err) {
		return false, errProviderCostDuplicate()
	}
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *ProviderCostRepository) Delete(ctx context.Context, id string) (bool, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM provider_costs WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *ProviderCostRepository) GetEffective(ctx context.Context, provider, model string, at time.Time) (*domain.ProviderCost, error) {
	c, err := scanProviderCost(r.db.QueryRow(ctx,
		providerCostSelect+` WHERE provider = $1 AND model = $2 AND effective_from <= $3
		 ORDER BY effective_from DESC LIMIT 1`,
		provider, model, at,
	))
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

// GetSetting returns "" (no error) when the key is absent.
func (r *ProviderCostRepository) GetSetting(ctx context.Context, key string) (string, error) {
	var v string
	err := r.db.QueryRow(ctx, `SELECT value FROM platform_settings WHERE key = $1`, key).Scan(&v)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return v, nil
}

func (r *ProviderCostRepository) SetSetting(ctx context.Context, key, value, updatedBy string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO platform_settings (key, value, updated_by, updated_at)
		 VALUES ($1, $2, $3, now())
		 ON CONFLICT (key) DO UPDATE SET value = $2, updated_by = $3, updated_at = now()`,
		key, value, updatedBy,
	)
	return err
}

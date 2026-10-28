package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// The cached index series (§6.11, G5/G6).
//
// Public data, identical for every account, so this table has no account_id —
// a SHARED cache like company_enrichments. Rows are effectively append-only:
// a published month never changes, and that is what keeps a past
// renegotiation's accumulation from moving under somebody's feet.
type EconomicIndexRepository struct{ db DBTX }

func NewEconomicIndexRepository(db DBTX) *EconomicIndexRepository {
	return &EconomicIndexRepository{db: db}
}

func (r *EconomicIndexRepository) Range(
	ctx context.Context, code domain.IndexCode, from, to time.Time,
) ([]domain.IndexValue, error) {
	rows, err := r.db.Query(ctx,
		`SELECT reference_month, monthly_rate
		   FROM economic_index_values
		  WHERE index_code = $1 AND reference_month >= $2 AND reference_month <= $3
		  ORDER BY reference_month`,
		string(code), domain.FirstOfMonth(from), domain.FirstOfMonth(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.IndexValue
	for rows.Next() {
		v := domain.IndexValue{Code: code}
		if err := rows.Scan(&v.ReferenceMonth, &v.MonthlyRate); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Upsert stores fetched months.
//
// DO UPDATE rather than DO NOTHING: BCB publishes a preliminary figure and
// revises it once. Overwriting is correct while the revision is still in the
// window, and the alternative — keeping the first number forever — would quote a
// customer a rate that no longer exists.
func (r *EconomicIndexRepository) Upsert(ctx context.Context, values []domain.IndexValue) error {
	for _, v := range values {
		if v.Code == "" {
			continue
		}
		if _, err := r.db.Exec(ctx,
			`INSERT INTO economic_index_values (index_code, reference_month, monthly_rate)
			 VALUES ($1,$2,$3)
			 ON CONFLICT (index_code, reference_month) DO UPDATE
			    SET monthly_rate = EXCLUDED.monthly_rate, fetched_at = NOW()`,
			string(v.Code), domain.FirstOfMonth(v.ReferenceMonth), v.MonthlyRate,
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *EconomicIndexRepository) LatestMonth(
	ctx context.Context, code domain.IndexCode,
) (*time.Time, error) {
	var month time.Time
	err := r.db.QueryRow(ctx,
		`SELECT MAX(reference_month) FROM economic_index_values WHERE index_code = $1`,
		string(code)).Scan(&month)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if month.IsZero() {
		return nil, nil
	}
	return &month, nil
}

var _ domain.EconomicIndexRepository = (*EconomicIndexRepository)(nil)

package postgres

import (
	"context"

	"github.com/replider/api/internal/core/domain"
)

// Raw SQL, like the other tables sqlc does not own.
type ContactPaymentRepository struct{ db DBTX }

func NewContactPaymentRepository(db DBTX) *ContactPaymentRepository {
	return &ContactPaymentRepository{db: db}
}

// Known returns every payment detail this contact has used before, as a set
// keyed for direct comparison.
func (r *ContactPaymentRepository) Known(ctx context.Context, accountID, contactKey string) (map[string]struct{}, error) {
	rows, err := r.db.Query(ctx,
		`SELECT kind, value FROM contact_payment_details
		  WHERE account_id = $1 AND contact_key = $2`, accountID, contactKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]struct{}{}
	for rows.Next() {
		var kind, value string
		if err := rows.Scan(&kind, &value); err != nil {
			return nil, err
		}
		out[domain.PaymentKey(domain.PaymentKind(kind), value)] = struct{}{}
	}
	return out, rows.Err()
}

// Record remembers what a message used.
//
// Upsert rather than insert: a supplier sending the same account every month
// should raise times_seen, not accumulate rows. That counter is what later
// separates "the account they always use" from "one they used once".
func (r *ContactPaymentRepository) Record(ctx context.Context, accountID, contactKey string, details []domain.PaymentDetail) error {
	for _, d := range details {
		_, err := r.db.Exec(ctx,
			`INSERT INTO contact_payment_details
			   (account_id, contact_key, kind, value, raw)
			 VALUES ($1,$2,$3,$4,$5)
			 ON CONFLICT (account_id, contact_key, kind, value) DO UPDATE
			    SET last_seen_at = NOW(),
			        times_seen = contact_payment_details.times_seen + 1`,
			accountID, contactKey, string(d.Kind), d.Value, d.Raw)
		if err != nil {
			return err
		}
	}
	return nil
}

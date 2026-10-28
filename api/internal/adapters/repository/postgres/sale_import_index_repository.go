package postgres

import (
	"context"
	"time"
)

// SaleImportIndexRepository answers the sales import's idempotency question.
//
// Its own type rather than a method on SaleRepository: the domain port for
// sales is implemented by a dozen test doubles, and this one question belongs
// to the import alone.
type SaleImportIndexRepository struct{ db DBTX }

func NewSaleImportIndexRepository(db DBTX) *SaleImportIndexRepository {
	return &SaleImportIndexRepository{db: db}
}

// HasSaleAt reports whether the account already holds a sale for this
// customer on this DAY for this total.
//
// The day, not the instant: a re-export of the same ERP report carries the
// date and the amount and nothing else stable, and a manual entry made at
// 14:32 must still block the import of the same sale at 00:00.
func (r *SaleImportIndexRepository) HasSaleAt(
	ctx context.Context, accountID, contactID string, soldAt time.Time, grossCents int64,
) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM sales
		    WHERE account_id = $1 AND contact_id = $2
		      AND (sold_at AT TIME ZONE 'UTC')::date = ($3::timestamptz AT TIME ZONE 'UTC')::date
		      AND gross_cents = $4)`,
		accountID, contactID, soldAt.UTC(), grossCents).Scan(&exists)
	return exists, err
}

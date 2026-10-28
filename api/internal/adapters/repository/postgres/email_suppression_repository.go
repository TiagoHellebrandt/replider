package postgres

import (
	"context"

	"github.com/replider/api/internal/core/domain"
)

// Raw SQL, like the other tables sqlc does not own.
type EmailSuppressionRepository struct{ db DBTX }

func NewEmailSuppressionRepository(db DBTX) *EmailSuppressionRepository {
	return &EmailSuppressionRepository{db: db}
}

// Suppress records an address as undeliverable.
//
// DO NOTHING on conflict rather than an update: the FIRST diagnostic is the
// reason the address died, and later reports are consequences of continuing
// to write to it. Overwriting would replace "550 user unknown" — the answer
// to "why?" — with whatever the last retry happened to say.
func (r *EmailSuppressionRepository) Suppress(ctx context.Context, s *domain.EmailSuppression) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO email_suppressions (id, account_id, address, reason, diagnostic)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (account_id, LOWER(address)) DO NOTHING`,
		s.ID, s.AccountID, s.Address, string(s.Reason), s.Diagnostic)
	return err
}

func (r *EmailSuppressionRepository) IsSuppressed(ctx context.Context, accountID, address string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM email_suppressions
		    WHERE account_id = $1 AND LOWER(address) = LOWER($2))`,
		accountID, address).Scan(&exists)
	return exists, err
}

func (r *EmailSuppressionRepository) List(ctx context.Context, accountID string, limit, offset int) ([]*domain.EmailSuppression, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, account_id, address, reason, diagnostic, created_at
		   FROM email_suppressions
		  WHERE account_id = $1
		  ORDER BY created_at DESC
		  LIMIT $2 OFFSET $3`, accountID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.EmailSuppression
	for rows.Next() {
		s := &domain.EmailSuppression{}
		var reason string
		if err := rows.Scan(&s.ID, &s.AccountID, &s.Address, &reason,
			&s.Diagnostic, &s.CreatedAt); err != nil {
			return nil, err
		}
		s.Reason = domain.SuppressionReason(reason)
		out = append(out, s)
	}
	return out, rows.Err()
}

// Release lifts a suppression. A hard delete, not a flag: the address is
// deliverable again, and a released row that still matched IsSuppressed
// would be the same bug with extra steps.
func (r *EmailSuppressionRepository) Release(ctx context.Context, accountID, address string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM email_suppressions
		  WHERE account_id = $1 AND LOWER(address) = LOWER($2)`, accountID, address)
	return err
}

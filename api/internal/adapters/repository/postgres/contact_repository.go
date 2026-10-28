package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

type ContactRepository struct {
	db DBTX
}

func NewContactRepository(db DBTX) *ContactRepository {
	return &ContactRepository{db: db}
}

func (r *ContactRepository) Upsert(ctx context.Context, contact *domain.Contact) (*domain.Contact, error) {
	if err := contact.Validate(); err != nil {
		return nil, err
	}
	now := time.Now()
	err := r.db.QueryRow(ctx,
		`INSERT INTO contacts (id, account_id, external_id, channel_type, name, company_name, email, phone, avatar_url, metadata, first_seen_at, last_seen_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11, $11, $11)
		 ON CONFLICT (account_id, external_id) DO UPDATE SET
		   name = COALESCE(EXCLUDED.name, contacts.name),
		   -- A second door reaching the same key fills what the first left
		   -- empty and never blanks what it knew.
		   company_name = COALESCE(EXCLUDED.company_name, contacts.company_name),
		   email = COALESCE(contacts.email, EXCLUDED.email),
		   phone = COALESCE(contacts.phone, EXCLUDED.phone),
		   last_seen_at = $11,
		   updated_at = $11
		 RETURNING id, account_id, external_id, channel_type, name, company_name, email, phone, avatar_url, metadata, opted_out_at, first_seen_at, last_seen_at, created_at, updated_at`,
		contact.ID, contact.AccountID, contact.ExternalID, contact.ChannelType, contact.Name, contact.CompanyName,
		contact.Email, contact.Phone, contact.AvatarURL, contact.Metadata, now,
	).Scan(
		&contact.ID, &contact.AccountID, &contact.ExternalID, &contact.ChannelType, &contact.Name, &contact.CompanyName,
		&contact.Email, &contact.Phone, &contact.AvatarURL, &contact.Metadata, &contact.OptedOutAt,
		&contact.FirstSeenAt, &contact.LastSeenAt, &contact.CreatedAt, &contact.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return contact, nil
}

func (r *ContactRepository) GetByID(ctx context.Context, id, accountID string) (*domain.Contact, error) {
	c := &domain.Contact{}
	err := r.db.QueryRow(ctx,
		`SELECT id, account_id, external_id, channel_type, name, company_name, email, phone, avatar_url, metadata, opted_out_at, first_seen_at, last_seen_at, created_at, updated_at
		 FROM contacts WHERE id = $1 AND account_id = $2`, id, accountID).
		Scan(&c.ID, &c.AccountID, &c.ExternalID, &c.ChannelType, &c.Name, &c.CompanyName, &c.Email, &c.Phone,
			&c.AvatarURL, &c.Metadata, &c.OptedOutAt, &c.FirstSeenAt, &c.LastSeenAt, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		// A foreign-account UUID matches no row — identical to a missing
		// one, so callers can't tell "not found" from "not yours".
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return c, nil
}

func (r *ContactRepository) GetByExternalID(ctx context.Context, accountID, externalID string) (*domain.Contact, error) {
	c := &domain.Contact{}
	err := r.db.QueryRow(ctx,
		`SELECT id, account_id, external_id, channel_type, name, company_name, email, phone, avatar_url, metadata, opted_out_at, first_seen_at, last_seen_at, created_at, updated_at
		 FROM contacts WHERE account_id = $1 AND external_id = $2`, accountID, externalID).
		Scan(&c.ID, &c.AccountID, &c.ExternalID, &c.ChannelType, &c.Name, &c.CompanyName, &c.Email, &c.Phone,
			&c.AvatarURL, &c.Metadata, &c.OptedOutAt, &c.FirstSeenAt, &c.LastSeenAt, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// ListByIDs resolves a batch of contact IDs. Account-scoped like GetByID,
// so ids belonging to another tenant simply do not come back — the caller
// cannot use this to probe for existence across accounts.
func (r *ContactRepository) ListByIDs(ctx context.Context, accountID string, ids []string) ([]*domain.Contact, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, account_id, external_id, channel_type, name, company_name, email, phone, avatar_url, metadata, opted_out_at, first_seen_at, last_seen_at, created_at, updated_at
		 FROM contacts WHERE account_id = $1 AND id = ANY($2)`, accountID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Contact
	for rows.Next() {
		c := &domain.Contact{}
		if err := rows.Scan(&c.ID, &c.AccountID, &c.ExternalID, &c.ChannelType, &c.Name, &c.CompanyName, &c.Email, &c.Phone,
			&c.AvatarURL, &c.Metadata, &c.OptedOutAt, &c.FirstSeenAt, &c.LastSeenAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *ContactRepository) ListByAccountID(ctx context.Context, accountID, query string, limit, offset int) ([]*domain.Contact, error) {
	var sql string
	var args []any

	if query != "" {
		sql = `SELECT id, account_id, external_id, channel_type, name, company_name, email, phone, avatar_url, metadata, opted_out_at, first_seen_at, last_seen_at, created_at, updated_at
			 FROM contacts WHERE account_id = $1 AND (name ILIKE '%' || $2 || '%' OR email ILIKE '%' || $2 || '%' OR phone ILIKE '%' || $2 || '%' OR external_id ILIKE '%' || $2 || '%')
			 ORDER BY last_seen_at DESC LIMIT $3 OFFSET $4`
		args = []any{accountID, query, limit, offset}
	} else {
		sql = `SELECT id, account_id, external_id, channel_type, name, company_name, email, phone, avatar_url, metadata, opted_out_at, first_seen_at, last_seen_at, created_at, updated_at
			 FROM contacts WHERE account_id = $1
			 ORDER BY last_seen_at DESC LIMIT $2 OFFSET $3`
		args = []any{accountID, limit, offset}
	}

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Non-nil so empty result serialises to `[]` not `null`.
	results := make([]*domain.Contact, 0)
	for rows.Next() {
		c := &domain.Contact{}
		if err := rows.Scan(&c.ID, &c.AccountID, &c.ExternalID, &c.ChannelType, &c.Name, &c.CompanyName, &c.Email, &c.Phone,
			&c.AvatarURL, &c.Metadata, &c.OptedOutAt, &c.FirstSeenAt, &c.LastSeenAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		results = append(results, c)
	}
	return results, nil
}

func (r *ContactRepository) Update(ctx context.Context, contact *domain.Contact) error {
	if err := contact.Validate(); err != nil {
		return err
	}
	contact.UpdatedAt = time.Now()
	_, err := r.db.Exec(ctx,
		`UPDATE contacts SET name = $1, email = $2, phone = $3, avatar_url = $4, metadata = $5, updated_at = $6, company_name = $8 WHERE id = $7`,
		contact.Name, contact.Email, contact.Phone, contact.AvatarURL, contact.Metadata, contact.UpdatedAt, contact.ID, contact.CompanyName,
	)
	return err
}

// SetOptedOut flips opted_out_at on a specific contact. Idempotent: the
// WHERE clause protects the original timestamp from being overwritten on
// repeat calls (customer sending STOP twice doesn't "re-opt-out").
// A contact row is lazily created if the customer opts out BEFORE having
// been observed — that way we never miss an opt-out just because the
// contact wasn't seeded yet.
func (r *ContactRepository) SetOptedOut(ctx context.Context, accountID, externalID string) (bool, error) {
	// First try to update an existing row. Returns the affected rows count.
	tag, err := r.db.Exec(ctx,
		`UPDATE contacts
		 SET opted_out_at = NOW(), updated_at = NOW()
		 WHERE account_id = $1 AND external_id = $2 AND opted_out_at IS NULL`,
		accountID, externalID)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() > 0 {
		return true, nil
	}

	// Either no row yet OR already opted out. Try an insert; conflict is
	// fine (means an opt-out already recorded and we preserve the old ts).
	now := time.Now()
	_, err = r.db.Exec(ctx,
		`INSERT INTO contacts (id, account_id, external_id, opted_out_at, first_seen_at, last_seen_at, created_at, updated_at)
		 VALUES (gen_random_uuid()::text, $1, $2, $3, $3, $3, $3, $3)
		 ON CONFLICT (account_id, external_id) DO NOTHING`,
		accountID, externalID, now)
	if err != nil {
		return false, err
	}
	return false, nil
}

// ClearOptedOut reverses an opt-out. Returns true when a row was flipped
// (false = either no such contact or already opted-in). Intentionally
// strict: does not create a row on the fly — opting IN a contact that
// never existed locally would be meaningless.
func (r *ContactRepository) ClearOptedOut(ctx context.Context, accountID, externalID string) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE contacts
		 SET opted_out_at = NULL, updated_at = NOW()
		 WHERE account_id = $1 AND external_id = $2 AND opted_out_at IS NOT NULL`,
		accountID, externalID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// IsOptedOut reports whether the single contact (accountID, externalID) has
// opted out. A missing row is NOT an error — it just means we've never seen
// the contact, so it is treated as not opted out (false). Used by the
// outbound send guard (US-025) to abort a send before reaching Meta.
func (r *ContactRepository) IsOptedOut(ctx context.Context, accountID, externalID string) (bool, error) {
	var optedOut bool
	err := r.db.QueryRow(ctx,
		`SELECT opted_out_at IS NOT NULL FROM contacts
		 WHERE account_id = $1 AND external_id = $2`,
		accountID, externalID).Scan(&optedOut)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return optedOut, nil
}

// ListOptedOutExternalIDs returns a set of externalIDs (phone/chat_id) that
// have already opted out. The input list is used as a filter so the query
// stays bounded even on large accounts.
func (r *ContactRepository) ListOptedOutExternalIDs(ctx context.Context, accountID string, externalIDs []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(externalIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx,
		`SELECT external_id FROM contacts
		 WHERE account_id = $1 AND opted_out_at IS NOT NULL AND external_id = ANY($2::text[])`,
		accountID, externalIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ext string
		if err := rows.Scan(&ext); err != nil {
			return nil, err
		}
		out[ext] = true
	}
	return out, nil
}

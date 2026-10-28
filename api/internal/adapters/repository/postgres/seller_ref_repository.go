package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// SellerRefRepository is a raw-SQL repo (NOT sqlc) over the same pool,
// owning users.seller_ref_code and the seller-attribution stamp on
// accounts (US-003, vendedores). Same rationale as ReferralRepository:
// keeps the feature off the expensive-to-extend sqlc repos. Codes share
// the unambiguous-alphabet generator with referral codes but live in a
// separate namespace (users vs accounts) — the two never cross-resolve.
type SellerRefRepository struct {
	db DBTX
}

func NewSellerRefRepository(db DBTX) *SellerRefRepository {
	return &SellerRefRepository{db: db}
}

// GetOrCreateSellerRefCode returns the user's prospecting code, minting
// one lazily on first call. Concurrent first-calls (or a code collision)
// converge on a single persisted code via the partial UNIQUE index +
// re-read — same scheme as ReferralRepository.GetOrCreateCode.
func (r *SellerRefRepository) GetOrCreateSellerRefCode(ctx context.Context, userID string) (string, error) {
	var existing *string
	err := r.db.QueryRow(ctx, `SELECT seller_ref_code FROM users WHERE id = $1`, userID).Scan(&existing)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", domain.ErrUserNotFound
		}
		return "", err
	}
	if existing != nil && *existing != "" {
		return *existing, nil
	}

	for attempt := 0; attempt < 5; attempt++ {
		code, err := generateReferralCode()
		if err != nil {
			return "", err
		}
		var written *string
		err = r.db.QueryRow(ctx,
			`UPDATE users SET seller_ref_code = $2, updated_at = now()
			 WHERE id = $1 AND seller_ref_code IS NULL
			 RETURNING seller_ref_code`,
			userID, code,
		).Scan(&written)
		if err == nil && written != nil {
			return *written, nil
		}
		if err == pgx.ErrNoRows {
			// A concurrent caller already set it — read the winner.
			if rerr := r.db.QueryRow(ctx, `SELECT seller_ref_code FROM users WHERE id = $1`, userID).Scan(&written); rerr == nil && written != nil {
				return *written, nil
			}
			continue
		}
		if isUniqueViolation(err) {
			continue // code collision — try a fresh one
		}
		if err != nil {
			return "", err
		}
	}
	return "", errors.New("could not allocate a unique seller ref code")
}

// ResolveSellerRef maps a code to its owning seller user id + email,
// requiring an active user of the platform account WITH the explicit
// seller flag (US-014 — a leftover code of someone never/no-longer
// flagged must not attribute new accounts). ("", "") with no error when
// nothing matches.
func (r *SellerRefRepository) ResolveSellerRef(ctx context.Context, code, platformAccountID string) (string, string, error) {
	if code == "" || platformAccountID == "" {
		return "", "", nil
	}
	var id, email string
	err := r.db.QueryRow(ctx,
		`SELECT u.id, u.email FROM users u
		 WHERE u.seller_ref_code = $1 AND u.account_id = $2 AND u.is_active = true
		   AND EXISTS (SELECT 1 FROM seller_settings ss
		                WHERE ss.user_id = u.id AND ss.is_seller)`,
		code, platformAccountID,
	).Scan(&id, &email)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", "", nil
		}
		return "", "", err
	}
	return id, email, nil
}

// IsSeller reports whether the user carries the explicit owner-granted
// seller flag (US-014). No seller_settings row (or the column false)
// means NOT a seller — being a platform-account user is not enough.
func (r *SellerRefRepository) IsSeller(ctx context.Context, userID string) (bool, error) {
	if userID == "" {
		return false, nil
	}
	var isSeller bool
	err := r.db.QueryRow(ctx,
		`SELECT is_seller FROM seller_settings WHERE user_id = $1`, userID,
	).Scan(&isSeller)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return isSeller, nil
}

// ── Convert-lead (US-004) — SellerLeadRepository impl ───────────────────────

// sellerLeadSelect is the slim account shape the convert-lead flow reads.
// admin_email is the OLDEST admin so the dialog shows the account owner.
// has_payment_history: a subscription id means Stripe billed (or is
// billing) this account — 'trialing' is the only never-paid status, so
// any other status also counts (covers canceled-then-cleared rows).
const sellerLeadSelect = `
	SELECT a.id, a.name, a.seller_user_id, a.lead_source, a.seller_linked_at, a.created_at,
	       (a.stripe_subscription_id IS NOT NULL
	        OR a.subscription_status <> 'trialing') AS has_payment_history,
	       (SELECT u.email FROM users u
	         WHERE u.account_id = a.id AND u.role = 'admin'
	         ORDER BY u.created_at ASC LIMIT 1) AS admin_email
	FROM accounts a`

func scanSellerLeadAccount(row pgx.Row) (*domain.SellerLeadAccount, error) {
	var acc domain.SellerLeadAccount
	err := row.Scan(&acc.ID, &acc.Name, &acc.SellerUserID, &acc.LeadSource,
		&acc.SellerLinkedAt, &acc.CreatedAt, &acc.HasPaymentHistory, &acc.AdminEmail)
	if err != nil {
		return nil, err
	}
	return &acc, nil
}

// SearchCustomerAccounts finds candidate accounts by name or by any of
// their users' emails. The platform account is always excluded — it can
// never be "converted". Newest accounts first (a just-signed-up lead is
// the common case).
func (r *SellerRefRepository) SearchCustomerAccounts(ctx context.Context, search, platformAccountID string, limit int) ([]domain.SellerLeadAccount, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	rows, err := r.db.Query(ctx, sellerLeadSelect+`
		WHERE a.id <> $2
		  AND (a.name ILIKE '%' || $1 || '%'
		       OR EXISTS (SELECT 1 FROM users u
		                   WHERE u.account_id = a.id AND u.email ILIKE '%' || $1 || '%'))
		ORDER BY a.created_at DESC
		LIMIT $3`,
		search, platformAccountID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.SellerLeadAccount
	for rows.Next() {
		acc, err := scanSellerLeadAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *acc)
	}
	return out, rows.Err()
}

// GetLeadAccount returns one account in the slim shape, nil when absent.
func (r *SellerRefRepository) GetLeadAccount(ctx context.Context, accountID string) (*domain.SellerLeadAccount, error) {
	acc, err := scanSellerLeadAccount(r.db.QueryRow(ctx, sellerLeadSelect+` WHERE a.id = $1`, accountID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return acc, nil
}

// StampInboundConversion links the account to the converting seller at the
// 20% inbound track and keeps the converting conversation id for the
// co-pilot deep link (US-015). seller_user_id IS NULL makes the write
// race-safe: the second of two concurrent converts affects 0 rows and
// reports false.
func (r *SellerRefRepository) StampInboundConversion(ctx context.Context, accountID, sellerUserID, conversationID string) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE accounts
		 SET seller_user_id = $2, lead_source = $3, seller_linked_at = now(),
		     converted_conversation_id = NULLIF($4, ''), updated_at = now()
		 WHERE id = $1 AND seller_user_id IS NULL`,
		accountID, sellerUserID, domain.LeadSourceInbound, conversationID,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// AccountHasUserEmail reports whether any user of the account has this
// exact email (case-insensitive). Convert-lead relation check (US-014).
func (r *SellerRefRepository) AccountHasUserEmail(ctx context.Context, accountID, email string) (bool, error) {
	if accountID == "" || email == "" {
		return false, nil
	}
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM users
		  WHERE account_id = $1 AND lower(email) = lower($2))`,
		accountID, email,
	).Scan(&exists)
	return exists, err
}

// StampSellerAttribution links a freshly created account to its
// prospecting seller at the 50% track. The seller_user_id IS NULL guard
// keeps an already-linked account untouched (first attribution wins).
func (r *SellerRefRepository) StampSellerAttribution(ctx context.Context, accountID, sellerUserID string) error {
	if accountID == "" || sellerUserID == "" {
		return nil
	}
	_, err := r.db.Exec(ctx,
		`UPDATE accounts
		 SET seller_user_id = $2, lead_source = $3, seller_linked_at = now(), updated_at = now()
		 WHERE id = $1 AND seller_user_id IS NULL`,
		accountID, sellerUserID, domain.LeadSourceSellerRef,
	)
	return err
}

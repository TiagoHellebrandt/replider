package postgres

import (
	"context"

	"github.com/replider/api/internal/core/domain"
)

// SellerSupportRepository is a raw-SQL repo (NOT sqlc) over the same pool
// — the WeeklyDigest/SupportNotify pattern. It owns the seller-panel
// grouped view of internal CS threads (canal-interno US-007) and the
// shared account-level internal note (account_internal_notes, migration
// 000144). Raw SQL keeps these off the sqlc repositories and their many
// test mocks.
type SellerSupportRepository struct {
	db DBTX
}

func NewSellerSupportRepository(db DBTX) *SellerSupportRepository {
	return &SellerSupportRepository{db: db}
}

// ListCarteiraThreads returns every internal thread belonging to the
// seller's carteira accounts (accounts.seller_user_id = seller), flat —
// one row per thread with its account keys + the account's shared note.
// Unread = inbound messages newer than the newest outbound one (awaiting
// a platform reply); there is no platform-side read cursor.
// Gotcha: the channels column is `provider`, not `type`, in raw SQL.
func (r *SellerSupportRepository) ListCarteiraThreads(ctx context.Context, sellerUserID string) ([]domain.SellerSupportThreadRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT a.id,
		       a.name,
		       c.id,
		       c.contact_id,
		       COALESCE(u.name, c.contact_id),
		       COALESCE(u.role, ''),
		       COALESCE(lm.content, ''),
		       lm.created_at,
		       COALESCE(pend.cnt, 0),
		       COALESCE(n.note, ''),
		       COALESCE(nu.name, ''),
		       n.updated_at
		FROM conversations c
		JOIN channels ch ON ch.id = c.channel_id AND ch.provider = 'internal'
		JOIN accounts a ON a.id = c.customer_account_id AND a.seller_user_id = $1
		LEFT JOIN users u ON u.id = c.contact_id
		LEFT JOIN account_internal_notes n ON n.account_id = a.id
		LEFT JOIN users nu ON nu.id = n.updated_by
		LEFT JOIN LATERAL (
			SELECT m.content, m.created_at
			FROM messages m
			WHERE m.conversation_id = c.id
			ORDER BY m.created_at DESC
			LIMIT 1
		) lm ON TRUE
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS cnt
			FROM messages m
			WHERE m.conversation_id = c.id
			  AND m.direction = 'inbound'
			  AND m.created_at > COALESCE((
				SELECT MAX(m2.created_at)
				FROM messages m2
				WHERE m2.conversation_id = c.id AND m2.direction = 'outbound'
			  ), '-infinity'::timestamptz)
		) pend ON TRUE
		ORDER BY a.name, a.id, lm.created_at DESC NULLS LAST, c.created_at DESC`,
		sellerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.SellerSupportThreadRow
	for rows.Next() {
		var t domain.SellerSupportThreadRow
		if err := rows.Scan(&t.AccountID, &t.AccountName, &t.ConversationID,
			&t.UserID, &t.UserName, &t.UserRole, &t.LastMessage, &t.LastMessageAt,
			&t.UnreadCount, &t.Note, &t.NoteUpdatedBy, &t.NoteUpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ResolveThreadAccount maps an internal-thread conversation to its
// customer account. Returns ("", "", nil) when the conversation doesn't
// exist or isn't an internal thread (no customer_account_id) — callers
// turn that into a 404.
func (r *SellerSupportRepository) ResolveThreadAccount(ctx context.Context, conversationID string) (accountID, accountName string, err error) {
	rows, err := r.db.Query(ctx, `
		SELECT a.id, a.name
		FROM conversations c
		JOIN accounts a ON a.id = c.customer_account_id
		WHERE c.id = $1`, conversationID)
	if err != nil {
		return "", "", err
	}
	defer rows.Close()
	if !rows.Next() {
		return "", "", rows.Err()
	}
	if err := rows.Scan(&accountID, &accountName); err != nil {
		return "", "", err
	}
	return accountID, accountName, rows.Err()
}

// GetNote fetches the account's shared note with its visible audit
// metadata. A missing row returns an empty note (not nil, not an error)
// so the side panel renders an editable blank.
func (r *SellerSupportRepository) GetNote(ctx context.Context, accountID string) (*domain.AccountInternalNote, error) {
	rows, err := r.db.Query(ctx, `
		SELECT n.note, COALESCE(u.name, ''), n.updated_at
		FROM account_internal_notes n
		LEFT JOIN users u ON u.id = n.updated_by
		WHERE n.account_id = $1`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := &domain.AccountInternalNote{AccountID: accountID}
	if !rows.Next() {
		return out, rows.Err()
	}
	if err := rows.Scan(&out.Note, &out.UpdatedByName, &out.UpdatedAt); err != nil {
		return nil, err
	}
	return out, rows.Err()
}

// UpsertNote writes the account's shared note, stamping who/when. One
// row per account (PK) — a concurrent edit is last-writer-wins, which is
// the intended semantics for a small shared scratchpad.
func (r *SellerSupportRepository) UpsertNote(ctx context.Context, accountID, note, editorUserID string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO account_internal_notes (account_id, note, updated_by, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (account_id)
		DO UPDATE SET note = EXCLUDED.note,
		              updated_by = EXCLUDED.updated_by,
		              updated_at = NOW()`,
		accountID, note, editorUserID)
	return err
}

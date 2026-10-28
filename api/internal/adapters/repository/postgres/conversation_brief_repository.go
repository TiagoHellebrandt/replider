package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// Raw SQL, like the other tables sqlc does not own.
type ConversationBriefRepository struct{ db DBTX }

func NewConversationBriefRepository(db DBTX) *ConversationBriefRepository {
	return &ConversationBriefRepository{db: db}
}

// Get is scoped by account in the WHERE clause, so a conversation id from
// another tenant reads as absent rather than as somebody else's summary.
func (r *ConversationBriefRepository) Get(ctx context.Context, conversationID, accountID string) (*domain.ConversationBrief, error) {
	b := &domain.ConversationBrief{}
	err := r.db.QueryRow(ctx,
		`SELECT conversation_id, account_id, text, model, agreed, pending,
		        next_step, covers_until, generated_at
		   FROM conversation_briefs
		  WHERE conversation_id = $1 AND account_id = $2`, conversationID, accountID).
		Scan(&b.ConversationID, &b.AccountID, &b.Text, &b.Model, &b.Agreed,
			&b.Pending, &b.NextStep, &b.CoversUntil, &b.GeneratedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return b, nil
}

// Save replaces the brief. One per conversation: a history of summaries is
// not something anyone reads, and keeping it would mean deciding which one is
// current on every read.
func (r *ConversationBriefRepository) Save(ctx context.Context, b *domain.ConversationBrief) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO conversation_briefs
		   (conversation_id, account_id, text, model, agreed, pending,
		    next_step, covers_until, generated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW())
		 ON CONFLICT (conversation_id) DO UPDATE
		    SET text = EXCLUDED.text,
		        model = EXCLUDED.model,
		        agreed = EXCLUDED.agreed,
		        pending = EXCLUDED.pending,
		        next_step = EXCLUDED.next_step,
		        covers_until = EXCLUDED.covers_until,
		        generated_at = NOW()`,
		b.ConversationID, b.AccountID, b.Text, b.Model, b.Agreed, b.Pending,
		b.NextStep, b.CoversUntil)
	return err
}

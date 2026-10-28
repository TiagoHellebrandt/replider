package postgres

import (
	"context"

	"github.com/replider/api/internal/core/domain"
)

type NoteRepository struct {
	db DBTX
}

func NewNoteRepository(db DBTX) *NoteRepository {
	return &NoteRepository{db: db}
}

func (r *NoteRepository) Create(ctx context.Context, note *domain.ConversationNote) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO conversation_notes (id, conversation_id, user_id, user_name, content, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		note.ID, note.ConversationID, note.UserID, note.UserName, note.Content, note.CreatedAt, note.UpdatedAt,
	)
	return err
}

func (r *NoteRepository) ListByConversationID(ctx context.Context, conversationID string) ([]*domain.ConversationNote, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, conversation_id, user_id, user_name, content, created_at, updated_at
		 FROM conversation_notes WHERE conversation_id = $1 ORDER BY created_at ASC`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Non-nil slice → empty JSON array, not null.
	results := make([]*domain.ConversationNote, 0)
	for rows.Next() {
		n := &domain.ConversationNote{}
		if err := rows.Scan(&n.ID, &n.ConversationID, &n.UserID, &n.UserName, &n.Content, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		results = append(results, n)
	}
	return results, nil
}

func (r *NoteRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM conversation_notes WHERE id = $1`, id)
	return err
}

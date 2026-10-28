package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

type TagRepository struct {
	db DBTX
}

func NewTagRepository(db DBTX) *TagRepository {
	return &TagRepository{db: db}
}

func (r *TagRepository) Create(ctx context.Context, tag *domain.Tag) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO tags (id, account_id, name, color, created_at) VALUES ($1, $2, $3, $4, $5)`,
		tag.ID, tag.AccountID, tag.Name, tag.Color, tag.CreatedAt,
	)
	return err
}

func (r *TagRepository) ListByAccountID(ctx context.Context, accountID string) ([]*domain.Tag, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, account_id, name, color, created_at FROM tags WHERE account_id = $1 ORDER BY name`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Initialise as an empty (non-nil) slice so the handler's c.JSON
	// serialises to `[]` instead of `null` for accounts with no tags.
	// The Flutter Retrofit client does `response.data!` (bang on null)
	// when the declared return type is List<TagModel>, so a null body
	// surfaces as a cryptic "Erro ao carregar tags" on the client.
	results := make([]*domain.Tag, 0)
	for rows.Next() {
		t := &domain.Tag{}
		if err := rows.Scan(&t.ID, &t.AccountID, &t.Name, &t.Color, &t.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, t)
	}
	return results, nil
}

func (r *TagRepository) GetByID(ctx context.Context, id string) (*domain.Tag, error) {
	t := &domain.Tag{}
	err := r.db.QueryRow(ctx,
		`SELECT id, account_id, name, color, created_at FROM tags WHERE id = $1`, id).
		Scan(&t.ID, &t.AccountID, &t.Name, &t.Color, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (r *TagRepository) GetByAccountAndName(ctx context.Context, accountID, name string) (*domain.Tag, error) {
	t := &domain.Tag{}
	err := r.db.QueryRow(ctx,
		`SELECT id, account_id, name, color, created_at
		 FROM tags
		 WHERE account_id = $1 AND LOWER(name) = LOWER($2)
		 LIMIT 1`,
		accountID, name).
		Scan(&t.ID, &t.AccountID, &t.Name, &t.Color, &t.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return t, nil
}

func (r *TagRepository) Update(ctx context.Context, tag *domain.Tag) error {
	_, err := r.db.Exec(ctx,
		`UPDATE tags SET name = $2, color = $3 WHERE id = $1`,
		tag.ID, tag.Name, tag.Color,
	)
	return err
}

func (r *TagRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM tags WHERE id = $1`, id)
	return err
}

func (r *TagRepository) AddToConversation(ctx context.Context, conversationID, tagID string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO conversation_tags (conversation_id, tag_id, created_at) VALUES ($1, $2, NOW()) ON CONFLICT DO NOTHING`,
		conversationID, tagID,
	)
	return err
}

func (r *TagRepository) RemoveFromConversation(ctx context.Context, conversationID, tagID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM conversation_tags WHERE conversation_id = $1 AND tag_id = $2`,
		conversationID, tagID,
	)
	return err
}

func (r *TagRepository) ListByConversation(ctx context.Context, accountID, conversationID string) ([]*domain.Tag, error) {
	rows, err := r.db.Query(ctx,
		`SELECT t.id, t.account_id, t.name, t.color, t.created_at
		 FROM tags t
		 JOIN conversation_tags ct ON ct.tag_id = t.id
		 JOIN conversations c ON c.id = ct.conversation_id
		 WHERE ct.conversation_id = $1 AND c.account_id = $2
		 ORDER BY t.name`, conversationID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Non-nil slice so empty result serialises to `[]` not `null`.
	results := make([]*domain.Tag, 0)
	for rows.Next() {
		t := &domain.Tag{}
		if err := rows.Scan(&t.ID, &t.AccountID, &t.Name, &t.Color, &t.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, t)
	}
	return results, nil
}

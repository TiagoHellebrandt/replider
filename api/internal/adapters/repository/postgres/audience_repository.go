package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// Raw SQL, like the other tables sqlc does not own.
type AudienceRepository struct{ db DBTX }

func NewAudienceRepository(db DBTX) *AudienceRepository {
	return &AudienceRepository{db: db}
}

const audienceCols = `id, account_id, name, segment, size, decision_maker,
	regions, keywords, suggested, unexplored, is_active, created_at, updated_at`

func scanAudience(row pgx.Row) (*domain.Audience, error) {
	a := &domain.Audience{}
	err := row.Scan(&a.ID, &a.AccountID, &a.Name, &a.Segment, &a.Size,
		&a.DecisionMaker, &a.Regions, &a.Keywords, &a.Suggested,
		&a.Unexplored, &a.IsActive, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return a, nil
}

func (r *AudienceRepository) List(ctx context.Context, accountID string) ([]*domain.Audience, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+audienceCols+` FROM audiences
		  WHERE account_id = $1 AND is_active
		  -- Suggestions last: an audience the customer typed themselves is
		  -- the one they mean to work, and a proposal should not outrank it
		  -- in their own list.
		  ORDER BY suggested, created_at`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Audience
	for rows.Next() {
		a, err := scanAudience(rows)
		if err != nil {
			return nil, err
		}
		if a != nil {
			out = append(out, a)
		}
	}
	return out, rows.Err()
}

func (r *AudienceRepository) Get(ctx context.Context, id, accountID string) (*domain.Audience, error) {
	return scanAudience(r.db.QueryRow(ctx,
		`SELECT `+audienceCols+` FROM audiences WHERE id = $1 AND account_id = $2`,
		id, accountID))
}

func (r *AudienceRepository) Upsert(ctx context.Context, a *domain.Audience) error {
	// Nil slices reach a NOT NULL TEXT[] as NULL. Normalised here rather
	// than at every call site.
	regions, keywords := a.Regions, a.Keywords
	if regions == nil {
		regions = []string{}
	}
	if keywords == nil {
		keywords = []string{}
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO audiences (id, account_id, name, segment, size,
		    decision_maker, regions, keywords, suggested, unexplored, is_active)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		 ON CONFLICT (id) DO UPDATE
		    SET name           = EXCLUDED.name,
		        segment        = EXCLUDED.segment,
		        size           = EXCLUDED.size,
		        decision_maker = EXCLUDED.decision_maker,
		        regions        = EXCLUDED.regions,
		        keywords       = EXCLUDED.keywords,
		        is_active      = EXCLUDED.is_active,
		        -- An update comes from a human editing the row, and editing a
		        -- proposal makes it theirs. Keeping the flag would go on
		        -- calling their own words "sugerido pela IA".
		        suggested      = FALSE,
		        unexplored     = FALSE,
		        updated_at     = NOW()
		  WHERE audiences.account_id = EXCLUDED.account_id`,
		a.ID, a.AccountID, a.Name, a.Segment, a.Size, a.DecisionMaker,
		regions, keywords, a.Suggested, a.Unexplored, a.IsActive)
	return err
}

// Delete is a soft delete: an audience that produced leads is referenced by
// the story of where they came from.
func (r *AudienceRepository) Delete(ctx context.Context, id, accountID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE audiences SET is_active = FALSE, updated_at = NOW()
		  WHERE id = $1 AND account_id = $2`, id, accountID)
	return err
}

var _ domain.AudienceRepository = (*AudienceRepository)(nil)

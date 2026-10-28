package postgres

import (
	"context"

	"github.com/replider/api/internal/core/domain"
)

type CannedResponseRepository struct {
	db DBTX
}

func NewCannedResponseRepository(db DBTX) *CannedResponseRepository {
	return &CannedResponseRepository{db: db}
}

const cannedResponseColumns = `id, account_id, shortcut, title, content, category, visibility, owner_user_id, created_at, updated_at`

func scanCannedResponse(row interface{ Scan(...any) error }) (*domain.CannedResponse, error) {
	cr := &domain.CannedResponse{}
	if err := row.Scan(&cr.ID, &cr.AccountID, &cr.Shortcut, &cr.Title, &cr.Content, &cr.Category, &cr.Visibility, &cr.OwnerUserID, &cr.CreatedAt, &cr.UpdatedAt); err != nil {
		return nil, err
	}
	return cr, nil
}

func (r *CannedResponseRepository) Create(ctx context.Context, cr *domain.CannedResponse) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO canned_responses (id, account_id, shortcut, title, content, category, visibility, owner_user_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		cr.ID, cr.AccountID, cr.Shortcut, cr.Title, cr.Content, cr.Category, cr.Visibility, cr.OwnerUserID, cr.CreatedAt, cr.UpdatedAt,
	)
	// Partial unique indexes are the race backstop for the use-case's
	// visible-set check.
	if isUniqueViolation(err) {
		return domain.ErrCannedResponseShortcutExists
	}
	return err
}

func (r *CannedResponseRepository) ListVisible(ctx context.Context, accountID, userID string) ([]*domain.CannedResponse, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+cannedResponseColumns+`
		 FROM canned_responses
		 WHERE account_id = $1 AND (visibility = 'account' OR owner_user_id = $2)
		 ORDER BY category, title`, accountID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Non-nil slice so empty result serialises to `[]` not `null` —
	// matching the Flutter Retrofit client's List<T> expectation.
	results := make([]*domain.CannedResponse, 0)
	for rows.Next() {
		cr, err := scanCannedResponse(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, cr)
	}
	return results, nil
}

func (r *CannedResponseRepository) GetByID(ctx context.Context, id string) (*domain.CannedResponse, error) {
	return scanCannedResponse(r.db.QueryRow(ctx,
		`SELECT `+cannedResponseColumns+` FROM canned_responses WHERE id = $1`, id))
}

func (r *CannedResponseRepository) GetVisibleByShortcut(ctx context.Context, accountID, userID, shortcut string) (*domain.CannedResponse, error) {
	return scanCannedResponse(r.db.QueryRow(ctx,
		`SELECT `+cannedResponseColumns+`
		 FROM canned_responses
		 WHERE account_id = $1 AND shortcut = $3 AND (visibility = 'account' OR owner_user_id = $2)
		 LIMIT 1`, accountID, userID, shortcut))
}

func (r *CannedResponseRepository) Update(ctx context.Context, cr *domain.CannedResponse) error {
	_, err := r.db.Exec(ctx,
		`UPDATE canned_responses SET shortcut = $2, title = $3, content = $4, category = $5, updated_at = NOW() WHERE id = $1`,
		cr.ID, cr.Shortcut, cr.Title, cr.Content, cr.Category,
	)
	if isUniqueViolation(err) {
		return domain.ErrCannedResponseShortcutExists
	}
	return err
}

func (r *CannedResponseRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM canned_responses WHERE id = $1`, id)
	return err
}

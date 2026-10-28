package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/replider/api/internal/core/domain"
)

type PromptRepository struct {
	db DBTX
}

func NewPromptRepository(db DBTX) *PromptRepository {
	return &PromptRepository{db: db}
}

func (r *PromptRepository) Create(ctx context.Context, p *domain.Prompt) error {
	answers, err := marshalWizardAnswers(p.WizardAnswers)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx,
		`INSERT INTO prompts (id, account_id, name, description, body, source, wizard_answers, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())`,
		p.ID, p.AccountID, p.Name, p.Description, p.Body, p.Source, answers, p.CreatedBy,
	)
	return mapPromptErr(err)
}

func (r *PromptRepository) GetByID(ctx context.Context, id string) (*domain.Prompt, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, account_id, name, description, body, source, wizard_answers, created_by, created_at, updated_at
		 FROM prompts WHERE id = $1`, id)
	return scanPrompt(row)
}

func (r *PromptRepository) ListByAccountID(ctx context.Context, accountID string, limit, offset int) ([]*domain.Prompt, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, account_id, name, description, body, source, wizard_answers, created_by, created_at, updated_at
		 FROM prompts WHERE account_id = $1 ORDER BY updated_at DESC LIMIT $2 OFFSET $3`,
		accountID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]*domain.Prompt, 0)
	for rows.Next() {
		p, err := scanPrompt(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, p)
	}
	return results, rows.Err()
}

func (r *PromptRepository) CountByAccountID(ctx context.Context, accountID string) (int64, error) {
	var n int64
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM prompts WHERE account_id = $1`, accountID).Scan(&n)
	return n, err
}

func (r *PromptRepository) Update(ctx context.Context, p *domain.Prompt) error {
	answers, err := marshalWizardAnswers(p.WizardAnswers)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx,
		`UPDATE prompts SET name = $2, description = $3, body = $4, wizard_answers = $5, updated_at = NOW() WHERE id = $1`,
		p.ID, p.Name, p.Description, p.Body, answers,
	)
	return mapPromptErr(err)
}

func (r *PromptRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM prompts WHERE id = $1`, id)
	return err
}

// scanPrompt accepts both pgx.Row and pgx.Rows via the shared interface.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanPrompt(row rowScanner) (*domain.Prompt, error) {
	p := &domain.Prompt{}
	var answers []byte
	if err := row.Scan(&p.ID, &p.AccountID, &p.Name, &p.Description, &p.Body, &p.Source, &answers, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	if len(answers) > 0 {
		var wa domain.WizardAnswers
		if err := json.Unmarshal(answers, &wa); err == nil {
			p.WizardAnswers = &wa
		}
	}
	return p, nil
}

func marshalWizardAnswers(wa *domain.WizardAnswers) ([]byte, error) {
	if wa == nil {
		return nil, nil
	}
	return json.Marshal(wa)
}

// mapPromptErr translates the unique-name violation into a domain error
// so handlers return 409 instead of an opaque 500.
func mapPromptErr(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "uniq_prompts_account_name") {
		return domain.ErrPromptNameExists
	}
	return err
}

var _ domain.PromptRepository = (*PromptRepository)(nil)

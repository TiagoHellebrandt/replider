package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// Raw SQL, like the other tables sqlc does not own.
type EmailTemplateRepository struct{ db DBTX }

func NewEmailTemplateRepository(db DBTX) *EmailTemplateRepository {
	return &EmailTemplateRepository{db: db}
}

const emailTemplateCols = `id, account_id, name, subject, body, funnel_id,
	user_id, version, is_active, created_at, updated_at`

func scanEmailTemplate(row pgx.Row) (*domain.EmailTemplate, error) {
	t := &domain.EmailTemplate{}
	err := row.Scan(&t.ID, &t.AccountID, &t.Name, &t.Subject, &t.Body,
		&t.FunnelID, &t.UserID, &t.Version, &t.IsActive, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return t, nil
}

func (r *EmailTemplateRepository) List(ctx context.Context, accountID string) ([]*domain.EmailTemplate, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+emailTemplateCols+` FROM email_templates
		  WHERE account_id = $1 AND is_active
		  ORDER BY name`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.EmailTemplate
	for rows.Next() {
		t, err := scanEmailTemplate(rows)
		if err != nil {
			return nil, err
		}
		if t != nil {
			out = append(out, t)
		}
	}
	return out, rows.Err()
}

func (r *EmailTemplateRepository) Get(ctx context.Context, id, accountID string) (*domain.EmailTemplate, error) {
	return scanEmailTemplate(r.db.QueryRow(ctx,
		`SELECT `+emailTemplateCols+` FROM email_templates
		  WHERE id = $1 AND account_id = $2`, id, accountID))
}

// Upsert creates or updates.
//
// The version is bumped only when the TEXT changed. Renaming a template, or
// moving its scope, must not orphan the numbers it earned — and bumping on
// every save would mean every template permanently shows "0 enviados".
func (r *EmailTemplateRepository) Upsert(ctx context.Context, t *domain.EmailTemplate) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO email_templates
		   (id, account_id, name, subject, body, funnel_id, user_id, version, is_active)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,1,$8)
		 ON CONFLICT (id) DO UPDATE
		    SET name = EXCLUDED.name,
		        subject = EXCLUDED.subject,
		        body = EXCLUDED.body,
		        funnel_id = EXCLUDED.funnel_id,
		        user_id = EXCLUDED.user_id,
		        is_active = EXCLUDED.is_active,
		        version = CASE
		          WHEN email_templates.subject IS DISTINCT FROM EXCLUDED.subject
		            OR email_templates.body IS DISTINCT FROM EXCLUDED.body
		          THEN email_templates.version + 1
		          ELSE email_templates.version
		        END,
		        updated_at = NOW()
		  WHERE email_templates.account_id = EXCLUDED.account_id`,
		t.ID, t.AccountID, t.Name, t.Subject, t.Body, t.FunnelID, t.UserID, t.IsActive)
	return err
}

// Delete is a soft delete: the stats reference the template, and a hard
// delete would erase the evidence for every campaign that used it.
func (r *EmailTemplateRepository) Delete(ctx context.Context, id, accountID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE email_templates SET is_active = FALSE, updated_at = NOW()
		  WHERE id = $1 AND account_id = $2`, id, accountID)
	return err
}

// Stats returns every version's numbers, newest version first, so the reader
// sees what the CURRENT text has earned before what an older one did.
func (r *EmailTemplateRepository) Stats(ctx context.Context, templateID string) ([]domain.EmailTemplateStats, error) {
	rows, err := r.db.Query(ctx,
		`SELECT template_id, version, sent, opened, clicked, replied, bounced
		   FROM email_template_stats
		  WHERE template_id = $1
		  ORDER BY version DESC`, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.EmailTemplateStats
	for rows.Next() {
		var s domain.EmailTemplateStats
		if err := rows.Scan(&s.TemplateID, &s.Version, &s.Sent, &s.Opened,
			&s.Clicked, &s.Replied, &s.Bounced); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// RecordSend bumps the sent counter for one template version.
//
// Upsert rather than insert: the stats row for a version is created by
// whichever event arrives first, and for a template being used the first time
// that is the send.
func (r *EmailTemplateRepository) RecordSend(ctx context.Context, templateID string, version int) error {
	return r.bump(ctx, templateID, version, "sent")
}

func (r *EmailTemplateRepository) RecordOpen(ctx context.Context, templateID string, version int) error {
	return r.bump(ctx, templateID, version, "opened")
}

func (r *EmailTemplateRepository) RecordClick(ctx context.Context, templateID string, version int) error {
	return r.bump(ctx, templateID, version, "clicked")
}

func (r *EmailTemplateRepository) RecordReply(ctx context.Context, templateID string, version int) error {
	return r.bump(ctx, templateID, version, "replied")
}

func (r *EmailTemplateRepository) RecordBounce(ctx context.Context, templateID string, version int) error {
	return r.bump(ctx, templateID, version, "bounced")
}

// bump increments one counter atomically.
//
// The column name is chosen from a fixed set by the exported methods above
// and never comes from a caller, so the interpolation is not a parameter in
// disguise — there is no path from a request to this string.
func (r *EmailTemplateRepository) bump(ctx context.Context, templateID string, version int, column string) error {
	if templateID == "" {
		return nil
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO email_template_stats (template_id, version, `+column+`)
		 VALUES ($1,$2,1)
		 ON CONFLICT (template_id, version) DO UPDATE
		    SET `+column+` = email_template_stats.`+column+` + 1`,
		templateID, version)
	return err
}

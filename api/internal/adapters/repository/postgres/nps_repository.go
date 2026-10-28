package postgres

import (
	"context"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// NPSRepository is a raw-SQL repo (NOT sqlc) for in-app NPS responses
// (US-178). It owns the write path plus the cross-account aggregation that
// backs the admin NPS dashboard, so it's deliberately kept off the sqlc
// repositories.
type NPSRepository struct {
	db DBTX
}

func NewNPSRepository(db DBTX) *NPSRepository {
	return &NPSRepository{db: db}
}

func (r *NPSRepository) Create(ctx context.Context, n *domain.NPSResponse) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO nps_responses (id, account_id, user_id, score, comment, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		n.ID, n.AccountID, n.UserID, n.Score, n.Comment, n.CreatedAt,
	)
	return err
}

// Summarize buckets every response since `since` into promoters (9-10),
// passives (7-8) and detractors (0-6) and computes the NPS in one pass.
func (r *NPSRepository) Summarize(ctx context.Context, since time.Time) (domain.NPSSummary, error) {
	var s domain.NPSSummary
	err := r.db.QueryRow(ctx,
		`SELECT
			count(*)                                               AS total,
			count(*) FILTER (WHERE score >= 9)                     AS promoters,
			count(*) FILTER (WHERE score BETWEEN 7 AND 8)          AS passives,
			count(*) FILTER (WHERE score <= 6)                     AS detractors,
			COALESCE(avg(score), 0)                                AS avg_score
		 FROM nps_responses
		 WHERE created_at >= $1`,
		since,
	).Scan(&s.Total, &s.Promoters, &s.Passives, &s.Detractors, &s.AverageScore)
	if err != nil {
		return domain.NPSSummary{}, err
	}
	if s.Total > 0 {
		// NPS = (%promoters − %detractors) on a [-100, 100] scale.
		s.Score = int(((float64(s.Promoters) - float64(s.Detractors)) / float64(s.Total)) * 100)
	}
	return s, nil
}

// IsPromptEligible returns true iff the account is old enough (created at
// or before createdBefore) AND has not handled the NPS prompt in `quarter`.
// `IS DISTINCT FROM` treats a NULL last-quarter (never prompted) as
// eligible. Single round-trip against the accounts row.
func (r *NPSRepository) IsPromptEligible(ctx context.Context, accountID, quarter string, createdBefore time.Time) (bool, error) {
	var eligible bool
	err := r.db.QueryRow(ctx,
		`SELECT created_at <= $2 AND (nps_prompt_last_quarter IS DISTINCT FROM $3)
		 FROM accounts WHERE id = $1`,
		accountID, createdBefore, quarter,
	).Scan(&eligible)
	if err != nil {
		return false, err
	}
	return eligible, nil
}

// StampPromptHandled records that the account handled (submitted or
// dismissed) the NPS prompt in `quarter`, suppressing it until next quarter.
func (r *NPSRepository) StampPromptHandled(ctx context.Context, accountID, quarter string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE accounts SET nps_prompt_last_quarter = $2 WHERE id = $1`,
		accountID, quarter,
	)
	return err
}

func (r *NPSRepository) RecentComments(ctx context.Context, since time.Time, limit int) ([]domain.NPSComment, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(ctx,
		`SELECT account_id, score, comment, created_at
		 FROM nps_responses
		 WHERE created_at >= $1 AND comment <> ''
		 ORDER BY created_at DESC
		 LIMIT $2`,
		since, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.NPSComment, 0)
	for rows.Next() {
		var c domain.NPSComment
		if err := rows.Scan(&c.AccountID, &c.Score, &c.Comment, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

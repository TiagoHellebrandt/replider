package postgres

import (
	"context"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// CancellationSurveyRepository is a raw-SQL repo (NOT sqlc) for churn-reason
// surveys (US-180). It owns the write path plus the cross-account breakdown
// that backs the admin churn dashboard, so it's deliberately kept off the
// sqlc repositories — mirroring NPSRepository.
type CancellationSurveyRepository struct {
	db DBTX
}

func NewCancellationSurveyRepository(db DBTX) *CancellationSurveyRepository {
	return &CancellationSurveyRepository{db: db}
}

func (r *CancellationSurveyRepository) Create(ctx context.Context, s *domain.CancellationSurvey) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO cancellation_surveys (id, account_id, user_id, reason, comment, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		s.ID, s.AccountID, s.UserID, s.Reason, s.Comment, s.CreatedAt,
	)
	return err
}

// Breakdown counts cancellations per reason since `since`, busiest reason
// first, so the admin dashboard can render the churn-driver chart directly.
func (r *CancellationSurveyRepository) Breakdown(ctx context.Context, since time.Time) ([]domain.CancellationReasonCount, error) {
	rows, err := r.db.Query(ctx,
		`SELECT reason, count(*) AS cnt
		   FROM cancellation_surveys
		  WHERE created_at >= $1
		  GROUP BY reason
		  ORDER BY cnt DESC, reason ASC`,
		since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.CancellationReasonCount, 0)
	for rows.Next() {
		var c domain.CancellationReasonCount
		if err := rows.Scan(&c.Reason, &c.Count); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *CancellationSurveyRepository) RecentComments(ctx context.Context, since time.Time, limit int) ([]domain.CancellationSurveyComment, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(ctx,
		`SELECT account_id, reason, comment, created_at
		   FROM cancellation_surveys
		  WHERE created_at >= $1 AND comment <> ''
		  ORDER BY created_at DESC
		  LIMIT $2`,
		since, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.CancellationSurveyComment, 0)
	for rows.Next() {
		var c domain.CancellationSurveyComment
		if err := rows.Scan(&c.AccountID, &c.Reason, &c.Comment, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

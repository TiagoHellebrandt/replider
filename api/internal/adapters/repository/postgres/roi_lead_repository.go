package postgres

import (
	"context"
	"time"

	"github.com/replider/api/internal/core/domain"
)

type ROILeadRepository struct {
	db DBTX
}

func NewROILeadRepository(db DBTX) *ROILeadRepository {
	return &ROILeadRepository{db: db}
}

func (r *ROILeadRepository) Create(ctx context.Context, l *domain.ROILead) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO roi_leads
		    (id, email, messages_per_month, current_cost, hourly_rate, estimated_savings, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		l.ID, l.Email, l.MessagesPerMonth, l.CurrentCost, l.HourlyRate, l.EstimatedSavings, l.CreatedAt,
	)
	return err
}

func (r *ROILeadRepository) ExistsRecent(ctx context.Context, email string, since time.Time) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM roi_leads
			WHERE email = $1 AND created_at >= $2
		)`,
		email, since,
	).Scan(&exists)
	return exists, err
}

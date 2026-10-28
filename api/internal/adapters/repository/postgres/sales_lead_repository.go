package postgres

import (
	"context"
	"time"

	"github.com/replider/api/internal/core/domain"
)

type SalesLeadRepository struct {
	db DBTX
}

func NewSalesLeadRepository(db DBTX) *SalesLeadRepository {
	return &SalesLeadRepository{db: db}
}

func (r *SalesLeadRepository) Create(ctx context.Context, l *domain.SalesLead) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO sales_leads
		    (id, company, email, company_size, channels, monthly_volume, message, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		l.ID, l.Company, l.Email, l.CompanySize, l.Channels, l.MonthlyVolume, l.Message, l.CreatedAt,
	)
	return err
}

func (r *SalesLeadRepository) ExistsRecent(ctx context.Context, email string, since time.Time) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM sales_leads
			WHERE email = $1 AND created_at >= $2
		)`,
		email, since,
	).Scan(&exists)
	return exists, err
}

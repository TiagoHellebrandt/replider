package postgres

import (
	"context"
	"time"

	"github.com/replider/api/internal/core/domain"
)

type NewsletterRepository struct {
	db DBTX
}

func NewNewsletterRepository(db DBTX) *NewsletterRepository {
	return &NewsletterRepository{db: db}
}

func (r *NewsletterRepository) Create(ctx context.Context, s *domain.NewsletterSubscriber) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO newsletter_subscribers (id, email, created_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (email) DO NOTHING`,
		s.ID, s.Email, time.Now(),
	)
	return err
}

func (r *NewsletterRepository) ExistsRecent(ctx context.Context, email string, since time.Time) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM newsletter_subscribers
			WHERE email = $1 AND created_at >= $2
		)`,
		email, since,
	).Scan(&exists)
	return exists, err
}

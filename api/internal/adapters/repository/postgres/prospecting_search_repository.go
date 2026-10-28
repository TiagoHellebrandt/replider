package postgres

import (
	"context"

	"github.com/replider/api/internal/core/domain"
)

// ProspectingSearchRepository is a raw-SQL repo (NOT sqlc) over the
// prospecting_searches audit table (US-022). The table is append-only and
// tiny, so a hand-written INSERT keeps it off the sqlc generation path.
type ProspectingSearchRepository struct {
	db DBTX
}

func NewProspectingSearchRepository(db DBTX) *ProspectingSearchRepository {
	return &ProspectingSearchRepository{db: db}
}

// Create records one prospecting run.
func (r *ProspectingSearchRepository) Create(ctx context.Context, s *domain.ProspectingSearch) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO prospecting_searches
		   (id, account_id, funnel_id, query, latitude, longitude, radius_meters, results_count, leads_created)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		s.ID, s.AccountID, s.FunnelID, s.Query, s.Latitude, s.Longitude, s.RadiusMeters, s.ResultsCount, s.LeadsCreated,
	)
	return err
}

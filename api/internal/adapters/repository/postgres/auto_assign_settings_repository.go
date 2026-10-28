package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// AutoAssignSettingsRepository is a raw-SQL repo (NOT sqlc) over the same
// pool, owning the auto_assign_settings table (US-016). Same rationale as
// SellerRefRepository: isolated feature, cheap to keep off the sqlc repos.
type AutoAssignSettingsRepository struct {
	db DBTX
}

func NewAutoAssignSettingsRepository(db DBTX) *AutoAssignSettingsRepository {
	return &AutoAssignSettingsRepository{db: db}
}

func (r *AutoAssignSettingsRepository) GetAutoAssignSettings(ctx context.Context, accountID string) (*domain.AutoAssignSettings, error) {
	s := domain.AutoAssignSettings{AccountID: accountID}
	err := r.db.QueryRow(ctx,
		`SELECT enabled, participant_user_ids, updated_at
		 FROM auto_assign_settings WHERE account_id = $1`,
		accountID,
	).Scan(&s.Enabled, &s.ParticipantUserIDs, &s.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if s.ParticipantUserIDs == nil {
		s.ParticipantUserIDs = []string{}
	}
	return &s, nil
}

func (r *AutoAssignSettingsRepository) UpsertAutoAssignSettings(ctx context.Context, settings *domain.AutoAssignSettings) error {
	participants := settings.ParticipantUserIDs
	if participants == nil {
		participants = []string{}
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO auto_assign_settings (account_id, enabled, participant_user_ids, updated_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (account_id)
		 DO UPDATE SET enabled = EXCLUDED.enabled,
		               participant_user_ids = EXCLUDED.participant_user_ids,
		               updated_at = NOW()`,
		settings.AccountID, settings.Enabled, participants,
	)
	return err
}

package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/replider/api/internal/core/domain"
)

type PushSubscriptionRepository struct {
	db DBTX
}

func NewPushSubscriptionRepository(db DBTX) *PushSubscriptionRepository {
	return &PushSubscriptionRepository{db: db}
}

func (r *PushSubscriptionRepository) Upsert(ctx context.Context, s *domain.PushSubscription) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO push_subscriptions (
			id, user_id, account_id, platform, endpoint,
			p256dh_key, auth_key, device_token, user_agent,
			enabled, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, TRUE, NOW(), NOW())
		ON CONFLICT (user_id, endpoint) DO UPDATE
		SET p256dh_key   = EXCLUDED.p256dh_key,
		    auth_key     = EXCLUDED.auth_key,
		    device_token = EXCLUDED.device_token,
		    user_agent   = EXCLUDED.user_agent,
		    platform     = EXCLUDED.platform,
		    enabled      = TRUE,
		    updated_at   = NOW()
	`,
		s.ID, s.UserID, s.AccountID, s.Platform, s.Endpoint,
		s.P256dhKey, s.AuthKey, s.DeviceToken, s.UserAgent,
	)
	return err
}

func (r *PushSubscriptionRepository) ListByAccountID(ctx context.Context, accountID string) ([]*domain.PushSubscription, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, account_id, platform, endpoint,
		       p256dh_key, auth_key, device_token, user_agent,
		       enabled, last_used_at, created_at, updated_at
		FROM push_subscriptions
		WHERE account_id = $1 AND enabled = TRUE
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPushSubscriptions(rows)
}

func (r *PushSubscriptionRepository) ListByUserID(ctx context.Context, userID string) ([]*domain.PushSubscription, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, account_id, platform, endpoint,
		       p256dh_key, auth_key, device_token, user_agent,
		       enabled, last_used_at, created_at, updated_at
		FROM push_subscriptions
		WHERE user_id = $1 AND enabled = TRUE
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPushSubscriptions(rows)
}

func (r *PushSubscriptionRepository) DeleteByEndpoint(ctx context.Context, userID, endpoint string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM push_subscriptions WHERE user_id = $1 AND endpoint = $2`,
		userID, endpoint)
	return err
}

func (r *PushSubscriptionRepository) DisableByEndpoint(ctx context.Context, endpoint string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE push_subscriptions SET enabled = FALSE, updated_at = NOW() WHERE endpoint = $1`,
		endpoint)
	return err
}

func (r *PushSubscriptionRepository) Touch(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE push_subscriptions SET last_used_at = NOW() WHERE id = $1`, id)
	return err
}

func scanPushSubscriptions(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]*domain.PushSubscription, error) {
	out := make([]*domain.PushSubscription, 0)
	for rows.Next() {
		s := &domain.PushSubscription{}
		var lastUsed pgtype.Timestamptz
		var created, updated pgtype.Timestamptz
		if err := rows.Scan(
			&s.ID, &s.UserID, &s.AccountID, &s.Platform, &s.Endpoint,
			&s.P256dhKey, &s.AuthKey, &s.DeviceToken, &s.UserAgent,
			&s.Enabled, &lastUsed, &created, &updated,
		); err != nil {
			return nil, err
		}
		if lastUsed.Valid {
			t := lastUsed.Time
			s.LastUsedAt = &t
		}
		s.CreatedAt = created.Time
		s.UpdatedAt = updated.Time
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

var _ domain.PushSubscriptionRepository = (*PushSubscriptionRepository)(nil)

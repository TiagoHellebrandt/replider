package postgres

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/replider/api/internal/adapters/secrets"
	"github.com/replider/api/internal/core/domain"
)

// GoogleCalendarIntegrationRepository is a raw-SQL repo (no sqlc — the
// feature is isolated to one table). The refresh token is sealed with
// the BYOK cipher (text form). Repo convention for encrypted columns:
// write fails closed on a nil cipher; read treats an unopenable value
// as absent so a key rotation never hard-fails the booking path.
type GoogleCalendarIntegrationRepository struct {
	pool   *pgxpool.Pool
	cipher *secrets.AESGCMCipher
}

func NewGoogleCalendarIntegrationRepository(pool *pgxpool.Pool, cipher *secrets.AESGCMCipher) *GoogleCalendarIntegrationRepository {
	return &GoogleCalendarIntegrationRepository{pool: pool, cipher: cipher}
}

func (r *GoogleCalendarIntegrationRepository) Get(ctx context.Context, accountID string) (*domain.GoogleCalendarIntegration, error) {
	var (
		sealed      string
		googleEmail string
		connectedAt time.Time
	)
	err := r.pool.QueryRow(ctx,
		`SELECT refresh_token_encrypted, google_email, connected_at
		   FROM google_calendar_integrations WHERE account_id = $1`,
		accountID,
	).Scan(&sealed, &googleEmail, &connectedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if r.cipher == nil {
		slog.Error("gcal integration read with nil cipher; treating as not connected", "account_id", accountID)
		return nil, nil
	}
	token, err := r.cipher.Open(sealed)
	if err != nil {
		slog.Error("gcal refresh token decrypt failed; treating as not connected", "account_id", accountID, "error", err)
		return nil, nil
	}
	return &domain.GoogleCalendarIntegration{
		AccountID:    accountID,
		RefreshToken: token,
		GoogleEmail:  googleEmail,
		ConnectedAt:  connectedAt,
	}, nil
}

func (r *GoogleCalendarIntegrationRepository) Save(ctx context.Context, integ *domain.GoogleCalendarIntegration) error {
	if r.cipher == nil {
		return domain.ErrGCalCipherUnconfigured
	}
	sealed, err := r.cipher.Seal(integ.RefreshToken)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx,
		`INSERT INTO google_calendar_integrations (account_id, refresh_token_encrypted, google_email)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (account_id) DO UPDATE SET
		   refresh_token_encrypted = EXCLUDED.refresh_token_encrypted,
		   google_email            = EXCLUDED.google_email,
		   updated_at              = NOW()`,
		integ.AccountID, sealed, integ.GoogleEmail,
	)
	return err
}

func (r *GoogleCalendarIntegrationRepository) Delete(ctx context.Context, accountID string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM google_calendar_integrations WHERE account_id = $1`, accountID)
	return err
}

package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/replider/api/internal/core/domain"
)

// --- OAuthCodeRepository ---

type OAuthCodeRepositoryPG struct {
	queries *Queries
}

func NewOAuthCodeRepository(db DBTX) *OAuthCodeRepositoryPG {
	return &OAuthCodeRepositoryPG{queries: New(db)}
}

func (r *OAuthCodeRepositoryPG) Create(ctx context.Context, code *domain.OAuthAuthorizationCode) error {
	q := getQueries(ctx, r.queries)
	return q.CreateAuthorizationCode(ctx, CreateAuthorizationCodeParams{
		CodeHash:      code.CodeHash,
		ClientID:      code.ClientID,
		UserID:        code.UserID,
		AccountID:     code.AccountID,
		RedirectUri:   code.RedirectURI,
		Scope:         code.Scope,
		CodeChallenge: code.CodeChallenge,
		ExpiresAt:     pgtype.Timestamptz{Time: code.ExpiresAt, Valid: true},
	})
}

func (r *OAuthCodeRepositoryPG) Consume(ctx context.Context, codeHash string) (*domain.ConsumedAuthCode, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.ConsumeAuthorizationCode(ctx, codeHash)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &domain.ConsumedAuthCode{
		ClientID:      row.ClientID,
		UserID:        row.UserID,
		AccountID:     row.AccountID,
		RedirectURI:   row.RedirectUri,
		Scope:         row.Scope,
		CodeChallenge: row.CodeChallenge,
	}, nil
}

func (r *OAuthCodeRepositoryPG) GetByHash(ctx context.Context, codeHash string) (*domain.OAuthAuthorizationCode, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetAuthorizationCodeByHash(ctx, codeHash)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &domain.OAuthAuthorizationCode{
		CodeHash:      row.CodeHash,
		ClientID:      row.ClientID,
		UserID:        row.UserID,
		AccountID:     row.AccountID,
		RedirectURI:   row.RedirectUri,
		Scope:         row.Scope,
		CodeChallenge: row.CodeChallenge,
		ExpiresAt:     row.ExpiresAt.Time,
		UsedAt:        timestamptzPtr(row.UsedAt),
	}, nil
}

func (r *OAuthCodeRepositoryPG) DeleteExpired(ctx context.Context) error {
	q := getQueries(ctx, r.queries)
	return q.DeleteExpiredAuthorizationCodes(ctx)
}

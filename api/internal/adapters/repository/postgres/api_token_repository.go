package postgres

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// --- APITokenRepository (also satisfies domain.APITokenResolver) ---

type APITokenRepositoryPG struct {
	queries *Queries
}

func NewAPITokenRepository(db DBTX) *APITokenRepositoryPG {
	return &APITokenRepositoryPG{queries: New(db)}
}

func (r *APITokenRepositoryPG) Create(ctx context.Context, token *domain.APIToken) error {
	q := getQueries(ctx, r.queries)
	return q.CreateAPIToken(ctx, CreateAPITokenParams{
		ID:          token.ID,
		TokenHash:   token.TokenHash,
		TokenType:   token.TokenType,
		UserID:      token.UserID,
		AccountID:   token.AccountID,
		ClientID:    token.ClientID,
		FamilyID:    token.FamilyID,
		Name:        token.Name,
		TokenPrefix: token.TokenPrefix,
		Scope:       token.Scope,
		ExpiresAt:   timeToTimestamptz(token.ExpiresAt),
	})
}

// Resolve implements domain.APITokenResolver: nil, nil means no active token.
func (r *APITokenRepositoryPG) Resolve(ctx context.Context, tokenHash string) (*domain.ResolvedAPIToken, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.ResolveAPIToken(ctx, tokenHash)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	resolved := &domain.ResolvedAPIToken{
		TokenID:   row.ID,
		TokenType: row.TokenType,
		UserID:    row.UserID,
		UserName:  row.UserName,
		AccountID: row.AccountID,
		Scope:     row.Scope,
	}
	if row.Role != nil {
		resolved.Role = *row.Role
	}
	if row.FamilyID != nil {
		resolved.FamilyID = *row.FamilyID
	}
	if row.IsActive != nil {
		resolved.UserActive = *row.IsActive
	}
	return resolved, nil
}

// TouchLastUsed is best-effort: failures are logged, never surfaced —
// authentication must not depend on a bookkeeping write.
func (r *APITokenRepositoryPG) TouchLastUsed(ctx context.Context, tokenID string) {
	q := getQueries(ctx, r.queries)
	if err := q.TouchAPITokenLastUsed(ctx, tokenID); err != nil {
		slog.Warn("api_token touch last_used failed", "token_id", tokenID, "error", err)
	}
}

func (r *APITokenRepositoryPG) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*domain.APIToken, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return mapAPIToken(row), nil
}

func (r *APITokenRepositoryPG) Revoke(ctx context.Context, id string) error {
	q := getQueries(ctx, r.queries)
	return q.RevokeToken(ctx, id)
}

func (r *APITokenRepositoryPG) RevokeByHash(ctx context.Context, tokenHash string) error {
	q := getQueries(ctx, r.queries)
	return q.RevokeTokenByHash(ctx, tokenHash)
}

func (r *APITokenRepositoryPG) RevokeFamily(ctx context.Context, familyID string) error {
	q := getQueries(ctx, r.queries)
	return q.RevokeTokenFamily(ctx, &familyID)
}

func (r *APITokenRepositoryPG) ListPATsByUser(ctx context.Context, userID string) ([]*domain.APIToken, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListPATsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	tokens := make([]*domain.APIToken, 0, len(rows))
	for _, row := range rows {
		tokens = append(tokens, &domain.APIToken{
			ID:          row.ID,
			TokenType:   domain.APITokenTypePAT,
			UserID:      userID,
			Name:        row.Name,
			TokenPrefix: row.TokenPrefix,
			ExpiresAt:   timestamptzPtr(row.ExpiresAt),
			LastUsedAt:  timestamptzPtr(row.LastUsedAt),
			CreatedAt:   row.CreatedAt.Time,
		})
	}
	return tokens, nil
}

func (r *APITokenRepositoryPG) RevokePATByIDAndUser(ctx context.Context, id, userID string) (int64, error) {
	q := getQueries(ctx, r.queries)
	return q.RevokePATByIDAndUser(ctx, RevokePATByIDAndUserParams{ID: id, UserID: userID})
}

func (r *APITokenRepositoryPG) CountActivePATsByUser(ctx context.Context, userID string) (int64, error) {
	q := getQueries(ctx, r.queries)
	return q.CountActivePATsByUser(ctx, userID)
}

func (r *APITokenRepositoryPG) ListOAuthConnectionsByUser(ctx context.Context, userID string) ([]*domain.OAuthConnection, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListOAuthConnectionsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	conns := make([]*domain.OAuthConnection, 0, len(rows))
	for _, row := range rows {
		conn := &domain.OAuthConnection{
			ClientName:     row.ClientName,
			LogoURI:        row.LogoUri,
			FirstGrantedAt: row.FirstGrantedAt.Time,
			LastUsedAt:     timestamptzPtr(row.LastUsedAt),
		}
		if row.ClientID != nil {
			conn.ClientID = *row.ClientID
		}
		conns = append(conns, conn)
	}
	return conns, nil
}

func (r *APITokenRepositoryPG) RevokeOAuthConnection(ctx context.Context, userID, clientID string) (int64, error) {
	q := getQueries(ctx, r.queries)
	return q.RevokeOAuthConnection(ctx, RevokeOAuthConnectionParams{UserID: userID, ClientID: &clientID})
}

func (r *APITokenRepositoryPG) DeleteExpired(ctx context.Context) error {
	q := getQueries(ctx, r.queries)
	return q.DeleteExpiredAPITokens(ctx)
}

func mapAPIToken(row ApiToken) *domain.APIToken {
	return &domain.APIToken{
		ID:          row.ID,
		TokenHash:   row.TokenHash,
		TokenType:   row.TokenType,
		UserID:      row.UserID,
		AccountID:   row.AccountID,
		ClientID:    row.ClientID,
		FamilyID:    row.FamilyID,
		Name:        row.Name,
		TokenPrefix: row.TokenPrefix,
		Scope:       row.Scope,
		ExpiresAt:   timestamptzPtr(row.ExpiresAt),
		RevokedAt:   timestamptzPtr(row.RevokedAt),
		LastUsedAt:  timestamptzPtr(row.LastUsedAt),
		CreatedAt:   row.CreatedAt.Time,
	}
}

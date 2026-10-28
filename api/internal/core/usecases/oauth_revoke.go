package usecases

import (
	"context"
	"log/slog"

	"github.com/replider/api/internal/core/domain"
)

// OAuthRevokeUseCase implements RFC 7009 token revocation. Per spec,
// revoking an unknown token is a success (200) — the endpoint must not
// leak whether a credential exists.
type OAuthRevokeUseCase struct {
	tokenRepo domain.APITokenRepository
}

func NewOAuthRevokeUseCase(tokenRepo domain.APITokenRepository) *OAuthRevokeUseCase {
	return &OAuthRevokeUseCase{tokenRepo: tokenRepo}
}

func (uc *OAuthRevokeUseCase) Execute(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	hash := domain.HashToken(token)

	// A refresh token revocation kills the whole family (spec: SHOULD
	// revoke related access tokens).
	stored, err := uc.tokenRepo.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		slog.Error("oauth revoke: lookup failed", "error", err)
		return oauthErr("server_error", "temporary failure, try again")
	}
	if stored != nil && stored.FamilyID != nil {
		return uc.tokenRepo.RevokeFamily(ctx, *stored.FamilyID)
	}
	return uc.tokenRepo.RevokeByHash(ctx, hash)
}

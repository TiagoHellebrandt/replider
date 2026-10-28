package usecases

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"log/slog"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// OAuthTokenUseCase implements POST /oauth/token: the
// authorization_code grant (with mandatory PKCE verification and
// code-replay family revocation) and the refresh_token grant (rotation
// with reuse detection).
type OAuthTokenUseCase struct {
	clientRepo domain.OAuthClientRepository
	codeRepo   domain.OAuthCodeRepository
	tokenRepo  domain.APITokenRepository
	now        func() time.Time
}

func NewOAuthTokenUseCase(
	clientRepo domain.OAuthClientRepository,
	codeRepo domain.OAuthCodeRepository,
	tokenRepo domain.APITokenRepository,
) *OAuthTokenUseCase {
	return &OAuthTokenUseCase{
		clientRepo: clientRepo,
		codeRepo:   codeRepo,
		tokenRepo:  tokenRepo,
		now:        time.Now,
	}
}

type TokenRequestDTO struct {
	GrantType    string
	Code         string
	CodeVerifier string
	RedirectURI  string
	RefreshToken string
	ClientID     string
	ClientSecret string
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

func (uc *OAuthTokenUseCase) Execute(ctx context.Context, dto TokenRequestDTO) (*TokenResponse, error) {
	client, err := uc.authenticateClient(ctx, dto)
	if err != nil {
		return nil, err
	}
	switch dto.GrantType {
	case "authorization_code":
		return uc.exchangeCode(ctx, client, dto)
	case "refresh_token":
		return uc.refresh(ctx, client, dto)
	default:
		return nil, oauthErr("unsupported_grant_type", "supported grants: authorization_code, refresh_token")
	}
}

func (uc *OAuthTokenUseCase) authenticateClient(ctx context.Context, dto TokenRequestDTO) (*domain.OAuthClient, error) {
	if dto.ClientID == "" {
		return nil, oauthErr("invalid_client", "client_id is required")
	}
	client, err := uc.clientRepo.GetByID(ctx, dto.ClientID)
	if err != nil {
		slog.Error("oauth token: client lookup failed", "error", err)
		return nil, oauthErr("server_error", "temporary failure, try again")
	}
	if client == nil {
		return nil, oauthErr("invalid_client", "unknown client")
	}
	if client.ClientSecretHash != nil {
		if dto.ClientSecret == "" {
			return nil, oauthErr("invalid_client", "client authentication required")
		}
		presented := domain.HashToken(dto.ClientSecret)
		if subtle.ConstantTimeCompare([]byte(presented), []byte(*client.ClientSecretHash)) != 1 {
			return nil, oauthErr("invalid_client", "client authentication failed")
		}
	}
	return client, nil
}

func (uc *OAuthTokenUseCase) exchangeCode(ctx context.Context, client *domain.OAuthClient, dto TokenRequestDTO) (*TokenResponse, error) {
	if dto.Code == "" || dto.CodeVerifier == "" {
		return nil, oauthErr("invalid_request", "code and code_verifier are required")
	}
	codeHash := domain.HashToken(dto.Code)
	consumed, err := uc.codeRepo.Consume(ctx, codeHash)
	if err != nil {
		slog.Error("oauth token: code consume failed", "error", err)
		return nil, oauthErr("server_error", "temporary failure, try again")
	}
	if consumed == nil {
		// Replay defense (OAuth 2.1): if the code exists but was already
		// consumed, every token minted from it must be revoked.
		if existing, lookupErr := uc.codeRepo.GetByHash(ctx, codeHash); lookupErr == nil && existing != nil && existing.UsedAt != nil {
			slog.Warn("oauth token: authorization code replay detected", "client_id", existing.ClientID, "user_id", existing.UserID)
			uc.revokeFamilyForReplayedCode(ctx, existing)
		}
		return nil, oauthErr("invalid_grant", "authorization code is invalid, expired, or already used")
	}
	if consumed.ClientID != client.ID {
		return nil, oauthErr("invalid_grant", "authorization code was issued to another client")
	}
	if consumed.RedirectURI != dto.RedirectURI {
		return nil, oauthErr("invalid_grant", "redirect_uri does not match the authorization request")
	}
	expected := domain.S256Challenge(dto.CodeVerifier)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(consumed.CodeChallenge)) != 1 {
		return nil, oauthErr("invalid_grant", "PKCE verification failed")
	}

	return uc.mintTokenPair(ctx, client.ID, consumed.UserID, consumed.AccountID, ulid.Make().String())
}

func (uc *OAuthTokenUseCase) refresh(ctx context.Context, client *domain.OAuthClient, dto TokenRequestDTO) (*TokenResponse, error) {
	if dto.RefreshToken == "" {
		return nil, oauthErr("invalid_request", "refresh_token is required")
	}
	stored, err := uc.tokenRepo.GetRefreshTokenByHash(ctx, domain.HashToken(dto.RefreshToken))
	if err != nil {
		slog.Error("oauth token: refresh lookup failed", "error", err)
		return nil, oauthErr("server_error", "temporary failure, try again")
	}
	if stored == nil {
		return nil, oauthErr("invalid_grant", "refresh token is invalid")
	}
	if stored.ClientID == nil || *stored.ClientID != client.ID {
		return nil, oauthErr("invalid_grant", "refresh token was issued to another client")
	}
	// Rotation reuse detection: a revoked refresh token being presented
	// again means the credential leaked — kill the whole family.
	if stored.RevokedAt != nil {
		if stored.FamilyID != nil {
			slog.Warn("oauth token: refresh token reuse detected — revoking family", "family_id", *stored.FamilyID, "user_id", stored.UserID)
			if err := uc.tokenRepo.RevokeFamily(ctx, *stored.FamilyID); err != nil {
				slog.Error("oauth token: family revoke failed", "error", err)
			}
		}
		return nil, oauthErr("invalid_grant", "refresh token has been revoked")
	}
	if stored.ExpiresAt != nil && stored.ExpiresAt.Before(uc.now()) {
		return nil, oauthErr("invalid_grant", "refresh token has expired")
	}

	// Rotate: revoke the presented refresh token, mint a new pair in the
	// same family so reuse detection covers the entire lineage.
	if err := uc.tokenRepo.Revoke(ctx, stored.ID); err != nil {
		slog.Error("oauth token: refresh rotation revoke failed", "error", err)
		return nil, oauthErr("server_error", "temporary failure, try again")
	}
	familyID := ulid.Make().String()
	if stored.FamilyID != nil {
		familyID = *stored.FamilyID
	}
	return uc.mintTokenPair(ctx, client.ID, stored.UserID, stored.AccountID, familyID)
}

func (uc *OAuthTokenUseCase) revokeFamilyForReplayedCode(ctx context.Context, code *domain.OAuthAuthorizationCode) {
	// Tokens minted from a code share a family created at exchange time;
	// we can't recover the family id from the code alone, so revoke every
	// active OAuth token the user granted this client. Coarse but safe —
	// replay of a consumed code is an active-attack signal.
	if _, err := uc.tokenRepo.RevokeOAuthConnection(ctx, code.UserID, code.ClientID); err != nil {
		slog.Error("oauth token: replay family revoke failed", "error", err)
	}
}

func (uc *OAuthTokenUseCase) mintTokenPair(ctx context.Context, clientID, userID, accountID, familyID string) (*TokenResponse, error) {
	access, err := randomBearerToken(domain.TokenPrefixAccess)
	if err != nil {
		return nil, err
	}
	refresh, err := randomBearerToken(domain.TokenPrefixRefresh)
	if err != nil {
		return nil, err
	}
	now := uc.now()
	accessExp := now.Add(domain.AccessTokenTTL)
	refreshExp := now.Add(domain.RefreshTokenTTL)

	accessRow := &domain.APIToken{
		ID:          ulid.Make().String(),
		TokenHash:   domain.HashToken(access),
		TokenType:   domain.APITokenTypeOAuthAccess,
		UserID:      userID,
		AccountID:   accountID,
		ClientID:    &clientID,
		FamilyID:    &familyID,
		TokenPrefix: safePrefix(access),
		Scope:       domain.OAuthScopeMCP,
		ExpiresAt:   &accessExp,
	}
	refreshRow := &domain.APIToken{
		ID:          ulid.Make().String(),
		TokenHash:   domain.HashToken(refresh),
		TokenType:   domain.APITokenTypeOAuthRefresh,
		UserID:      userID,
		AccountID:   accountID,
		ClientID:    &clientID,
		FamilyID:    &familyID,
		TokenPrefix: safePrefix(refresh),
		Scope:       domain.OAuthScopeMCP,
		ExpiresAt:   &refreshExp,
	}
	if err := uc.tokenRepo.Create(ctx, accessRow); err != nil {
		return nil, err
	}
	if err := uc.tokenRepo.Create(ctx, refreshRow); err != nil {
		return nil, err
	}
	if err := uc.clientRepo.TouchLastUsed(ctx, clientID); err != nil {
		slog.Warn("oauth token: client touch failed", "error", err)
	}
	return &TokenResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int64(domain.AccessTokenTTL.Seconds()),
		RefreshToken: refresh,
		Scope:        domain.OAuthScopeMCP,
	}, nil
}

func randomBearerToken(prefix string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

// safePrefix returns the loggable head of a token (prefix + 4 chars).
func safePrefix(token string) string {
	if len(token) <= 16 {
		return token
	}
	return token[:16]
}

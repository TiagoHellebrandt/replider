package usecases

import (
	"context"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// MCPPatUseCase manages personal access tokens for the MCP surface —
// the "paste a token into Claude Code" alternative to the OAuth flow.
type MCPPatUseCase struct {
	tokenRepo domain.APITokenRepository
	now       func() time.Time
}

func NewMCPPatUseCase(tokenRepo domain.APITokenRepository) *MCPPatUseCase {
	return &MCPPatUseCase{tokenRepo: tokenRepo, now: time.Now}
}

type CreatePATDTO struct {
	UserID        string
	AccountID     string
	Name          string
	ExpiresInDays *int
}

type CreatePATResponse struct {
	ID string `json:"id"`
	// Token is the plaintext credential — returned exactly once at
	// creation; only the SHA-256 hash is stored.
	Token       string     `json:"token"`
	Name        string     `json:"name"`
	TokenPrefix string     `json:"token_prefix"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (uc *MCPPatUseCase) Create(ctx context.Context, dto CreatePATDTO) (*CreatePATResponse, error) {
	count, err := uc.tokenRepo.CountActivePATsByUser(ctx, dto.UserID)
	if err != nil {
		return nil, err
	}
	if count >= domain.MaxActivePATsPerUser {
		return nil, domain.ErrPATLimitReached
	}

	token, err := randomBearerToken(domain.TokenPrefixPAT)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(dto.Name)
	if name == "" {
		name = "Claude"
	}

	var expiresAt *time.Time
	if dto.ExpiresInDays != nil && *dto.ExpiresInDays > 0 {
		exp := uc.now().AddDate(0, 0, *dto.ExpiresInDays)
		expiresAt = &exp
	}

	row := &domain.APIToken{
		ID:          ulid.Make().String(),
		TokenHash:   domain.HashToken(token),
		TokenType:   domain.APITokenTypePAT,
		UserID:      dto.UserID,
		AccountID:   dto.AccountID,
		Name:        name,
		TokenPrefix: safePrefix(token),
		Scope:       domain.OAuthScopeMCP,
		ExpiresAt:   expiresAt,
	}
	if err := uc.tokenRepo.Create(ctx, row); err != nil {
		return nil, err
	}
	return &CreatePATResponse{
		ID:          row.ID,
		Token:       token,
		Name:        name,
		TokenPrefix: row.TokenPrefix,
		ExpiresAt:   expiresAt,
		CreatedAt:   uc.now(),
	}, nil
}

type PATInfo struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	TokenPrefix string     `json:"token_prefix"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (uc *MCPPatUseCase) List(ctx context.Context, userID string) ([]PATInfo, error) {
	tokens, err := uc.tokenRepo.ListPATsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	infos := make([]PATInfo, 0, len(tokens))
	for _, t := range tokens {
		infos = append(infos, PATInfo{
			ID:          t.ID,
			Name:        t.Name,
			TokenPrefix: t.TokenPrefix,
			ExpiresAt:   t.ExpiresAt,
			LastUsedAt:  t.LastUsedAt,
			CreatedAt:   t.CreatedAt,
		})
	}
	return infos, nil
}

// Revoke is scoped by user_id so a user can never revoke someone else's token.
func (uc *MCPPatUseCase) Revoke(ctx context.Context, userID, tokenID string) error {
	affected, err := uc.tokenRepo.RevokePATByIDAndUser(ctx, tokenID, userID)
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrPATNotFound
	}
	return nil
}

// ListConnections lists the user's active OAuth grants (per client).
func (uc *MCPPatUseCase) ListConnections(ctx context.Context, userID string) ([]*domain.OAuthConnection, error) {
	return uc.tokenRepo.ListOAuthConnectionsByUser(ctx, userID)
}

// RevokeConnection revokes every active token the user granted a client.
func (uc *MCPPatUseCase) RevokeConnection(ctx context.Context, userID, clientID string) error {
	affected, err := uc.tokenRepo.RevokeOAuthConnection(ctx, userID, clientID)
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrPATNotFound
	}
	return nil
}

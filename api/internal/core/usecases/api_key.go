package usecases

import (
	"context"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// Machine API keys (§6.19, J1).

// APIKeyUseCase mints, lists and revokes account-level keys.
type APIKeyUseCase struct {
	keys   domain.APIKeyRepository
	tokens domain.APITokenRepository
	now    func() time.Time
}

func NewAPIKeyUseCase(keys domain.APIKeyRepository, tokens domain.APITokenRepository) *APIKeyUseCase {
	return &APIKeyUseCase{keys: keys, tokens: tokens, now: time.Now}
}

// CreateAPIKeyDTO is what a caller may ask for.
type CreateAPIKeyDTO struct {
	AccountID string
	// CreatedByUserID records WHO minted it. The key belongs to the account and
	// outlives them; this is the audit fact, not an ownership claim.
	CreatedByUserID string
	Name            string
	Scopes          []string
	ExpiresInDays   *int
}

// CreateAPIKeyResponse carries the plaintext key — once.
type CreateAPIKeyResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Key is the credential, returned at creation and never again: only its
	// SHA-256 hash is stored, so there is no "show me the key" to build later
	// even if somebody asks for one.
	Key         string            `json:"key"`
	TokenPrefix string            `json:"token_prefix"`
	Scopes      []domain.APIScope `json:"scopes"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

// Create mints a key.
func (uc *APIKeyUseCase) Create(ctx context.Context, dto CreateAPIKeyDTO) (*CreateAPIKeyResponse, error) {
	scopes, err := domain.NormalizeAPIScopes(dto.Scopes)
	if err != nil {
		return nil, err
	}

	count, err := uc.keys.CountActive(ctx, dto.AccountID)
	if err != nil {
		return nil, err
	}
	if count >= domain.MaxActiveAPIKeysPerAccount {
		return nil, domain.ErrAPIKeyLimitReached
	}

	plaintext, err := randomBearerToken(domain.TokenPrefixAPIKey)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(dto.Name)
	if name == "" {
		// A wall of keys called "" is a wall nobody can revoke confidently —
		// and revoking the wrong one takes a live integration down.
		name = "Integração"
	}

	var expiresAt *time.Time
	if dto.ExpiresInDays != nil && *dto.ExpiresInDays > 0 {
		exp := uc.now().AddDate(0, 0, *dto.ExpiresInDays)
		expiresAt = &exp
	}

	row := &domain.APIToken{
		ID:          ulid.Make().String(),
		TokenHash:   domain.HashToken(plaintext),
		TokenType:   domain.APITokenTypeAPIKey,
		UserID:      dto.CreatedByUserID,
		AccountID:   dto.AccountID,
		Name:        name,
		TokenPrefix: safePrefix(plaintext),
		Scope:       domain.EncodeAPIScopes(scopes),
		ExpiresAt:   expiresAt,
	}
	if err := uc.tokens.Create(ctx, row); err != nil {
		return nil, err
	}

	return &CreateAPIKeyResponse{
		ID:          row.ID,
		Name:        name,
		Key:         plaintext,
		TokenPrefix: row.TokenPrefix,
		Scopes:      scopes,
		ExpiresAt:   expiresAt,
		CreatedAt:   uc.now(),
	}, nil
}

// List returns the account's live keys.
func (uc *APIKeyUseCase) List(ctx context.Context, accountID string) ([]*domain.APIKeyInfo, error) {
	return uc.keys.ListByAccount(ctx, accountID)
}

// Revoke kills a key. Reports false when nothing matched, so the caller answers
// 404 rather than a success that did nothing — somebody revoking a leaked key
// needs to know it actually happened.
func (uc *APIKeyUseCase) Revoke(ctx context.Context, accountID, id string) (bool, error) {
	return uc.keys.Revoke(ctx, accountID, strings.TrimSpace(id))
}

// Scopes is the catalogue, for the screen that mints a key.
func (uc *APIKeyUseCase) Scopes() []map[string]string {
	catalogue := domain.APIScopeCatalogue()
	out := make([]map[string]string, 0, len(catalogue))
	for _, entry := range catalogue {
		out = append(out, map[string]string{
			"scope":       string(entry.Scope),
			"description": entry.Description,
		})
	}
	return out
}

package domain

import (
	"context"
	"time"
)

// APIToken is the unified bearer credential for the MCP surface:
// OAuth access/refresh tokens and personal access tokens share one table
// so a single middleware lookup authenticates both.

const (
	APITokenTypeOAuthAccess  = "oauth_access"
	APITokenTypeOAuthRefresh = "oauth_refresh" //nolint:gosec // G101: token *type* discriminator, not a credential
	APITokenTypePAT          = "pat"
)

type APIToken struct {
	ID          string
	TokenHash   string
	TokenType   string
	UserID      string
	AccountID   string
	ClientID    *string
	FamilyID    *string
	Name        string
	TokenPrefix string
	Scope       string
	ExpiresAt   *time.Time
	RevokedAt   *time.Time
	LastUsedAt  *time.Time
	CreatedAt   time.Time
}

// OAuthConnection summarizes a user's active grant to one OAuth client
// (dashboard "conexões ativas" listing).
type OAuthConnection struct {
	ClientID       string     `json:"client_id"`
	ClientName     string     `json:"client_name"`
	LogoURI        string     `json:"logo_uri,omitempty"`
	FirstGrantedAt time.Time  `json:"first_granted_at"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
}

// ResolvedAPIToken is what the MCP auth middleware needs to authenticate a
// request: the token joined with its user so the gin context can be
// populated identically to the JWT path.
type ResolvedAPIToken struct {
	TokenID    string
	TokenType  string
	UserID     string
	UserName   string
	AccountID  string
	Role       string
	Scope      string
	FamilyID   string
	UserActive bool
}

// APITokenResolver is the narrow port the middleware depends on.
type APITokenResolver interface {
	// Resolve returns nil, nil when no active token matches the hash.
	Resolve(ctx context.Context, tokenHash string) (*ResolvedAPIToken, error)
	// TouchLastUsed is best-effort and throttled server-side.
	TouchLastUsed(ctx context.Context, tokenID string)
}

type APITokenRepository interface {
	Create(ctx context.Context, token *APIToken) error
	GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*APIToken, error)
	Revoke(ctx context.Context, id string) error
	RevokeByHash(ctx context.Context, tokenHash string) error
	RevokeFamily(ctx context.Context, familyID string) error
	ListPATsByUser(ctx context.Context, userID string) ([]*APIToken, error)
	RevokePATByIDAndUser(ctx context.Context, id, userID string) (int64, error)
	CountActivePATsByUser(ctx context.Context, userID string) (int64, error)
	ListOAuthConnectionsByUser(ctx context.Context, userID string) ([]*OAuthConnection, error)
	RevokeOAuthConnection(ctx context.Context, userID, clientID string) (int64, error)
	DeleteExpired(ctx context.Context) error
}

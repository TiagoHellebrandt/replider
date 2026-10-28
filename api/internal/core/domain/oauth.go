package domain

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"
)

// OAuth 2.1 authorization server for the remote MCP integration
// (Claude.ai custom connectors register via Dynamic Client Registration
// and exchange authorization codes with mandatory PKCE S256).

const (
	OAuthScopeMCP = "mcp"

	AuthCodeTTL     = 10 * time.Minute
	AccessTokenTTL  = time.Hour
	RefreshTokenTTL = 30 * 24 * time.Hour

	TokenPrefixPAT          = "rpl_mcp_" //nolint:gosec // G101: public token PREFIX (identifies the credential class), not a secret
	TokenPrefixAccess       = "rpl_at_"
	TokenPrefixRefresh      = "rpl_rt_"
	TokenPrefixClientSecret = "rpl_cs_"

	// MaxActivePATsPerUser caps token sprawl per user.
	MaxActivePATsPerUser = 10
)

type OAuthClient struct {
	ID                      string
	ClientSecretHash        *string // nil = public client (token_endpoint_auth_method "none")
	TokenEndpointAuthMethod string
	ClientName              string
	RedirectURIs            []string
	GrantTypes              []string
	Scope                   string
	LogoURI                 string
	CreatedAt               time.Time
}

type OAuthAuthorizationCode struct {
	CodeHash      string
	ClientID      string
	UserID        string
	AccountID     string
	RedirectURI   string
	Scope         string
	CodeChallenge string
	ExpiresAt     time.Time
	UsedAt        *time.Time
}

// ConsumedAuthCode is the result of the atomic single-use consume.
type ConsumedAuthCode struct {
	ClientID      string
	UserID        string
	AccountID     string
	RedirectURI   string
	Scope         string
	CodeChallenge string
}

type OAuthClientRepository interface {
	Create(ctx context.Context, client *OAuthClient) error
	GetByID(ctx context.Context, id string) (*OAuthClient, error)
	TouchLastUsed(ctx context.Context, id string) error
}

type OAuthCodeRepository interface {
	Create(ctx context.Context, code *OAuthAuthorizationCode) error
	// Consume marks the code used and returns its payload atomically.
	// Returns nil, nil when the code is unknown, expired, or already used.
	Consume(ctx context.Context, codeHash string) (*ConsumedAuthCode, error)
	// GetByHash returns the raw row (including consumed ones) so the token
	// endpoint can distinguish replay from garbage and revoke the family.
	GetByHash(ctx context.Context, codeHash string) (*OAuthAuthorizationCode, error)
	DeleteExpired(ctx context.Context) error
}

// HashToken is the canonical bearer-credential hash: tokens, authorization
// codes, and client secrets are stored as hex-encoded SHA-256 only.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// S256Challenge derives the PKCE code_challenge from a code_verifier
// (base64url without padding, per RFC 7636).
func S256Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

package usecases

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/url"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// OAuthRegisterClientUseCase implements Dynamic Client Registration
// (RFC 7591). Claude.ai registers itself as a public client before
// starting the authorization-code flow for the MCP connector.
type OAuthRegisterClientUseCase struct {
	clientRepo domain.OAuthClientRepository
}

func NewOAuthRegisterClientUseCase(clientRepo domain.OAuthClientRepository) *OAuthRegisterClientUseCase {
	return &OAuthRegisterClientUseCase{clientRepo: clientRepo}
}

type RegisterClientDTO struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	Scope                   string   `json:"scope"`
	LogoURI                 string   `json:"logo_uri"`
}

type RegisterClientResponse struct {
	ClientID                string   `json:"client_id"`
	ClientSecret            string   `json:"client_secret,omitempty"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
	ClientName              string   `json:"client_name,omitempty"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	Scope                   string   `json:"scope"`
}

// OAuthProtocolError carries an RFC 6749/7591-shaped error code; the
// handlers serialize it as {"error": ..., "error_description": ...}
// instead of the platform AppError envelope.
type OAuthProtocolError struct {
	Code        string
	Description string
}

func (e *OAuthProtocolError) Error() string {
	return e.Code + ": " + e.Description
}

func oauthErr(code, desc string) *OAuthProtocolError {
	return &OAuthProtocolError{Code: code, Description: desc}
}

func (uc *OAuthRegisterClientUseCase) Execute(ctx context.Context, dto RegisterClientDTO) (*RegisterClientResponse, error) {
	if len(dto.RedirectURIs) == 0 {
		return nil, oauthErr("invalid_redirect_uri", "redirect_uris is required")
	}
	for _, raw := range dto.RedirectURIs {
		if err := validateRedirectURI(raw); err != nil {
			return nil, err
		}
	}

	authMethod := dto.TokenEndpointAuthMethod
	if authMethod == "" {
		authMethod = "none"
	}
	switch authMethod {
	case "none", "client_secret_basic", "client_secret_post":
	default:
		return nil, oauthErr("invalid_client_metadata", "unsupported token_endpoint_auth_method")
	}

	grantTypes := dto.GrantTypes
	if len(grantTypes) == 0 {
		grantTypes = []string{"authorization_code", "refresh_token"}
	}
	for _, gt := range grantTypes {
		if gt != "authorization_code" && gt != "refresh_token" {
			return nil, oauthErr("invalid_client_metadata", "unsupported grant_type: "+gt)
		}
	}

	client := &domain.OAuthClient{
		ID:                      ulid.Make().String(),
		TokenEndpointAuthMethod: authMethod,
		ClientName:              strings.TrimSpace(dto.ClientName),
		RedirectURIs:            dto.RedirectURIs,
		GrantTypes:              grantTypes,
		Scope:                   domain.OAuthScopeMCP,
		LogoURI:                 dto.LogoURI,
	}

	var plainSecret string
	if authMethod != "none" {
		secretBytes := make([]byte, 32)
		if _, err := rand.Read(secretBytes); err != nil {
			return nil, err
		}
		plainSecret = domain.TokenPrefixClientSecret + base64.RawURLEncoding.EncodeToString(secretBytes)
		hash := domain.HashToken(plainSecret)
		client.ClientSecretHash = &hash
	}

	if err := uc.clientRepo.Create(ctx, client); err != nil {
		return nil, err
	}

	return &RegisterClientResponse{
		ClientID:                client.ID,
		ClientSecret:            plainSecret,
		ClientIDIssuedAt:        time.Now().Unix(),
		ClientName:              client.ClientName,
		RedirectURIs:            client.RedirectURIs,
		TokenEndpointAuthMethod: client.TokenEndpointAuthMethod,
		GrantTypes:              client.GrantTypes,
		Scope:                   client.Scope,
	}, nil
}

// validateRedirectURI enforces absolute https URIs, allowing plain http
// only for loopback addresses (RFC 8252 native-app convention).
func validateRedirectURI(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" {
		return oauthErr("invalid_redirect_uri", "redirect_uris must be absolute URLs")
	}
	if u.Fragment != "" {
		return oauthErr("invalid_redirect_uri", "redirect_uris must not contain a fragment")
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		host := u.Hostname()
		if host == "localhost" || host == "127.0.0.1" || host == "::1" {
			return nil
		}
		return oauthErr("invalid_redirect_uri", "http redirect_uris are only allowed for loopback addresses")
	default:
		return oauthErr("invalid_redirect_uri", "unsupported redirect_uri scheme")
	}
}

package usecases

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
	"golang.org/x/crypto/bcrypt"
)

// OAuthAuthorizeUseCase drives the /oauth/authorize leg: request
// validation (client + redirect_uri + PKCE), the server-rendered
// login/consent step (password + optional TOTP, mirroring
// AuthenticateUserUseCase), and authorization-code minting.
type OAuthAuthorizeUseCase struct {
	clientRepo  domain.OAuthClientRepository
	codeRepo    domain.OAuthCodeRepository
	userRepo    domain.UserRepository
	jwtService  domain.JWTService
	twoFactorUC *TwoFactorUseCase
	flowSecret  []byte // HMAC key for the consent-form flow token (reuses the JWT secret)
	now         func() time.Time
}

func NewOAuthAuthorizeUseCase(
	clientRepo domain.OAuthClientRepository,
	codeRepo domain.OAuthCodeRepository,
	userRepo domain.UserRepository,
	jwtService domain.JWTService,
	twoFactorUC *TwoFactorUseCase,
	flowSecret string,
) *OAuthAuthorizeUseCase {
	return &OAuthAuthorizeUseCase{
		clientRepo:  clientRepo,
		codeRepo:    codeRepo,
		userRepo:    userRepo,
		jwtService:  jwtService,
		twoFactorUC: twoFactorUC,
		flowSecret:  []byte(flowSecret),
		now:         time.Now,
	}
}

const flowTokenTTL = 15 * time.Minute

// AuthorizeRequest is the validated GET /oauth/authorize parameter set.
type AuthorizeRequest struct {
	ClientID            string
	RedirectURI         string
	State               string
	Scope               string
	CodeChallenge       string
	CodeChallengeMethod string
	ResponseType        string
}

// ValidatedAuthorizeRequest carries what the consent page needs to render.
type ValidatedAuthorizeRequest struct {
	Client    *domain.OAuthClient
	Request   AuthorizeRequest
	FlowToken string
}

// ErrMfaRequired is returned by Authorize when the user has TOTP
// enabled and no code was supplied; MfaSession must be echoed back in
// the 2FA form step.
type MfaRequiredError struct {
	MfaSession string
}

func (e *MfaRequiredError) Error() string { return "mfa code required" }

// ValidateAuthorizeRequest checks the GET parameters. Client or
// redirect_uri problems return an error the handler must RENDER (never
// redirect — open-redirect defense); other RFC violations are safe to
// send back to the validated redirect_uri as error= parameters.
func (uc *OAuthAuthorizeUseCase) ValidateAuthorizeRequest(ctx context.Context, req AuthorizeRequest) (*ValidatedAuthorizeRequest, error) {
	if req.ClientID == "" {
		return nil, oauthErr("invalid_request", "client_id is required")
	}
	client, err := uc.clientRepo.GetByID(ctx, req.ClientID)
	if err != nil {
		slog.Error("oauth authorize: client lookup failed", "error", err)
		return nil, oauthErr("server_error", "temporary failure, try again")
	}
	if client == nil {
		return nil, oauthErr("invalid_request", "unknown client_id")
	}
	if !redirectURIRegistered(client, req.RedirectURI) {
		return nil, oauthErr("invalid_request", "redirect_uri is not registered for this client")
	}
	if req.ResponseType != "code" {
		return nil, oauthErr("unsupported_response_type", "only response_type=code is supported")
	}
	// PKCE S256 is mandatory (OAuth 2.1): absent or plain are rejected.
	if req.CodeChallenge == "" {
		return nil, oauthErr("invalid_request", "code_challenge is required (PKCE)")
	}
	if req.CodeChallengeMethod != "S256" {
		return nil, oauthErr("invalid_request", "code_challenge_method must be S256")
	}
	if req.Scope != "" && req.Scope != domain.OAuthScopeMCP {
		return nil, oauthErr("invalid_scope", "only the 'mcp' scope is supported")
	}
	return &ValidatedAuthorizeRequest{
		Client:    client,
		Request:   req,
		FlowToken: uc.mintFlowToken(req),
	}, nil
}

type AuthorizeDTO struct {
	Request    AuthorizeRequest
	FlowToken  string
	Email      string
	Password   string
	MfaSession string // set on the 2FA step
	MfaCode    string
}

// Authorize runs the POST leg: flow-token integrity, credential check
// (password or MFA session + TOTP), then mints the authorization code
// and returns the full redirect URL.
func (uc *OAuthAuthorizeUseCase) Authorize(ctx context.Context, dto AuthorizeDTO) (string, error) {
	if !uc.verifyFlowToken(dto.Request, dto.FlowToken) {
		return "", oauthErr("invalid_request", "authorization session expired — restart the flow")
	}
	// Re-validate client + redirect on the POST too: the flow token pins
	// the params, but the client may have been deleted meanwhile.
	validated, err := uc.ValidateAuthorizeRequest(ctx, dto.Request)
	if err != nil {
		return "", err
	}

	var user *domain.User
	if dto.MfaSession != "" {
		claims, err := uc.jwtService.ValidateMfaLoginToken(dto.MfaSession)
		if err != nil {
			return "", domain.ErrUnauthorized
		}
		if err := uc.twoFactorUC.VerifyForLogin(ctx, claims.UserID, dto.MfaCode); err != nil {
			return "", err
		}
		user, err = uc.userRepo.GetByID(ctx, claims.UserID)
		if err != nil || user == nil {
			return "", domain.ErrInvalidCredentials
		}
	} else {
		user, err = uc.authenticatePassword(ctx, dto.Email, dto.Password)
		if err != nil {
			return "", err
		}
		if user.TOTPEnabled {
			mfaToken, err := uc.jwtService.GenerateMfaLoginToken(user.ID)
			if err != nil {
				return "", err
			}
			return "", &MfaRequiredError{MfaSession: mfaToken}
		}
	}

	if !user.IsActive {
		return "", domain.ErrInvalidCredentials
	}

	code, err := uc.mintAuthorizationCode(ctx, validated, user)
	if err != nil {
		return "", err
	}

	redirect := dto.Request.RedirectURI
	sep := "?"
	if strings.Contains(redirect, "?") {
		sep = "&"
	}
	redirect += sep + "code=" + code
	if dto.Request.State != "" {
		redirect += "&state=" + urlQueryEscape(dto.Request.State)
	}
	return redirect, nil
}

func (uc *OAuthAuthorizeUseCase) authenticatePassword(ctx context.Context, email, password string) (*domain.User, error) {
	user, err := uc.userRepo.GetByEmail(ctx, email)
	if err != nil {
		slog.Error("oauth authorize: user lookup failed", "error", err)
		return nil, domain.ErrInvalidCredentials
	}
	if user == nil || user.PasswordHash == nil {
		return nil, domain.ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}
	return user, nil
}

func (uc *OAuthAuthorizeUseCase) mintAuthorizationCode(ctx context.Context, v *ValidatedAuthorizeRequest, user *domain.User) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	code := base64.RawURLEncoding.EncodeToString(raw)
	err := uc.codeRepo.Create(ctx, &domain.OAuthAuthorizationCode{
		CodeHash:      domain.HashToken(code),
		ClientID:      v.Client.ID,
		UserID:        user.ID,
		AccountID:     user.AccountID,
		RedirectURI:   v.Request.RedirectURI,
		Scope:         domain.OAuthScopeMCP,
		CodeChallenge: v.Request.CodeChallenge,
		ExpiresAt:     uc.now().Add(domain.AuthCodeTTL),
	})
	if err != nil {
		return "", err
	}
	return code, nil
}

// --- consent-form flow token (CSRF/integrity, stateless) ---

func (uc *OAuthAuthorizeUseCase) flowPayload(req AuthorizeRequest, issuedAt int64) string {
	return strings.Join([]string{
		req.ClientID, req.RedirectURI, req.State, req.CodeChallenge,
		req.CodeChallengeMethod, req.Scope, strconv.FormatInt(issuedAt, 10),
	}, "\n")
}

func (uc *OAuthAuthorizeUseCase) mintFlowToken(req AuthorizeRequest) string {
	issuedAt := uc.now().Unix()
	mac := hmac.New(sha256.New, uc.flowSecret)
	mac.Write([]byte(uc.flowPayload(req, issuedAt)))
	return fmt.Sprintf("%d.%s", issuedAt, base64.RawURLEncoding.EncodeToString(mac.Sum(nil)))
}

func (uc *OAuthAuthorizeUseCase) verifyFlowToken(req AuthorizeRequest, token string) bool {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return false
	}
	issuedAt, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}
	if uc.now().Sub(time.Unix(issuedAt, 0)) > flowTokenTTL {
		return false
	}
	mac := hmac.New(sha256.New, uc.flowSecret)
	mac.Write([]byte(uc.flowPayload(req, issuedAt)))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(expected), []byte(parts[1])) == 1
}

func urlQueryEscape(s string) string {
	// Minimal escape for the state echo; state is opaque to us.
	replacer := strings.NewReplacer("&", "%26", "#", "%23", "+", "%2B", " ", "%20")
	return replacer.Replace(s)
}

func redirectURIRegistered(client *domain.OAuthClient, uri string) bool {
	for _, registered := range client.RedirectURIs {
		if registered == uri {
			return true
		}
	}
	return false
}

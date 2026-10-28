package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

// --- mocks for the OAuth/PAT repos (narrow, feature-local) ---

type MockOAuthClientRepo struct{ mock.Mock }

func (m *MockOAuthClientRepo) Create(ctx context.Context, c *domain.OAuthClient) error {
	return m.Called(ctx, c).Error(0)
}
func (m *MockOAuthClientRepo) GetByID(ctx context.Context, id string) (*domain.OAuthClient, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.(*domain.OAuthClient), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockOAuthClientRepo) TouchLastUsed(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

type MockOAuthCodeRepo struct{ mock.Mock }

func (m *MockOAuthCodeRepo) Create(ctx context.Context, c *domain.OAuthAuthorizationCode) error {
	return m.Called(ctx, c).Error(0)
}
func (m *MockOAuthCodeRepo) Consume(ctx context.Context, hash string) (*domain.ConsumedAuthCode, error) {
	args := m.Called(ctx, hash)
	if v := args.Get(0); v != nil {
		return v.(*domain.ConsumedAuthCode), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockOAuthCodeRepo) GetByHash(ctx context.Context, hash string) (*domain.OAuthAuthorizationCode, error) {
	args := m.Called(ctx, hash)
	if v := args.Get(0); v != nil {
		return v.(*domain.OAuthAuthorizationCode), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockOAuthCodeRepo) DeleteExpired(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

type MockAPITokenRepo struct{ mock.Mock }

func (m *MockAPITokenRepo) Create(ctx context.Context, t *domain.APIToken) error {
	return m.Called(ctx, t).Error(0)
}
func (m *MockAPITokenRepo) GetRefreshTokenByHash(ctx context.Context, h string) (*domain.APIToken, error) {
	args := m.Called(ctx, h)
	if v := args.Get(0); v != nil {
		return v.(*domain.APIToken), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockAPITokenRepo) Revoke(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockAPITokenRepo) RevokeByHash(ctx context.Context, h string) error {
	return m.Called(ctx, h).Error(0)
}
func (m *MockAPITokenRepo) RevokeFamily(ctx context.Context, f string) error {
	return m.Called(ctx, f).Error(0)
}
func (m *MockAPITokenRepo) ListPATsByUser(ctx context.Context, u string) ([]*domain.APIToken, error) {
	args := m.Called(ctx, u)
	if v := args.Get(0); v != nil {
		return v.([]*domain.APIToken), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockAPITokenRepo) RevokePATByIDAndUser(ctx context.Context, id, u string) (int64, error) {
	args := m.Called(ctx, id, u)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockAPITokenRepo) CountActivePATsByUser(ctx context.Context, u string) (int64, error) {
	args := m.Called(ctx, u)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockAPITokenRepo) ListOAuthConnectionsByUser(ctx context.Context, u string) ([]*domain.OAuthConnection, error) {
	args := m.Called(ctx, u)
	if v := args.Get(0); v != nil {
		return v.([]*domain.OAuthConnection), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockAPITokenRepo) RevokeOAuthConnection(ctx context.Context, u, c string) (int64, error) {
	args := m.Called(ctx, u, c)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockAPITokenRepo) DeleteExpired(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

// --- helpers ---

func testClient(secretHash *string) *domain.OAuthClient {
	return &domain.OAuthClient{
		ID:                      "01CLIENT00000000000000TEST",
		ClientSecretHash:        secretHash,
		TokenEndpointAuthMethod: "none",
		ClientName:              "Claude",
		RedirectURIs:            []string{"https://claude.ai/api/mcp/auth_callback"},
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		Scope:                   domain.OAuthScopeMCP,
	}
}

func validAuthorizeRequest() AuthorizeRequest {
	return AuthorizeRequest{
		ClientID:            "01CLIENT00000000000000TEST",
		RedirectURI:         "https://claude.ai/api/mcp/auth_callback",
		State:               "xyz",
		CodeChallenge:       domain.S256Challenge("verifier-123"),
		CodeChallengeMethod: "S256",
		ResponseType:        "code",
	}
}

// --- DCR ---

func TestOAuthRegisterClient_RejectsNonHTTPSRedirect(t *testing.T) {
	uc := NewOAuthRegisterClientUseCase(new(MockOAuthClientRepo))
	_, err := uc.Execute(context.Background(), RegisterClientDTO{
		RedirectURIs: []string{"http://evil.example.com/cb"},
	})
	var oe *OAuthProtocolError
	assert.True(t, errors.As(err, &oe))
	assert.Equal(t, "invalid_redirect_uri", oe.Code)
}

func TestOAuthRegisterClient_AllowsLoopbackHTTPAndCreatesPublicClient(t *testing.T) {
	repo := new(MockOAuthClientRepo)
	repo.On("Create", mock.Anything, mock.MatchedBy(func(c *domain.OAuthClient) bool {
		return c.ClientSecretHash == nil && c.TokenEndpointAuthMethod == "none"
	})).Return(nil)
	uc := NewOAuthRegisterClientUseCase(repo)
	resp, err := uc.Execute(context.Background(), RegisterClientDTO{
		ClientName:   "Claude",
		RedirectURIs: []string{"http://localhost:33418/cb", "https://claude.ai/api/mcp/auth_callback"},
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.ClientID)
	assert.Empty(t, resp.ClientSecret)
	repo.AssertExpectations(t)
}

func TestOAuthRegisterClient_MintsSecretForConfidentialClient(t *testing.T) {
	repo := new(MockOAuthClientRepo)
	repo.On("Create", mock.Anything, mock.MatchedBy(func(c *domain.OAuthClient) bool {
		return c.ClientSecretHash != nil
	})).Return(nil)
	uc := NewOAuthRegisterClientUseCase(repo)
	resp, err := uc.Execute(context.Background(), RegisterClientDTO{
		RedirectURIs:            []string{"https://claude.ai/cb"},
		TokenEndpointAuthMethod: "client_secret_post",
	})
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp.ClientSecret, domain.TokenPrefixClientSecret))
}

// --- authorize ---

func newAuthorizeUC(clientRepo *MockOAuthClientRepo, codeRepo *MockOAuthCodeRepo, userRepo *MockUserRepo, jwt *MockJWTService) *OAuthAuthorizeUseCase {
	return NewOAuthAuthorizeUseCase(clientRepo, codeRepo, userRepo, jwt, NewTwoFactorUseCase(userRepo, "Replider"), "test-secret")
}

func TestOAuthAuthorize_RejectsMissingPKCE(t *testing.T) {
	clientRepo := new(MockOAuthClientRepo)
	clientRepo.On("GetByID", mock.Anything, mock.Anything).Return(testClient(nil), nil)
	uc := newAuthorizeUC(clientRepo, new(MockOAuthCodeRepo), new(MockUserRepo), new(MockJWTService))

	req := validAuthorizeRequest()
	req.CodeChallenge = ""
	_, err := uc.ValidateAuthorizeRequest(context.Background(), req)
	var oe *OAuthProtocolError
	assert.True(t, errors.As(err, &oe))
	assert.Contains(t, oe.Description, "code_challenge")

	req = validAuthorizeRequest()
	req.CodeChallengeMethod = "plain"
	_, err = uc.ValidateAuthorizeRequest(context.Background(), req)
	assert.Error(t, err)
}

func TestOAuthAuthorize_RejectsUnregisteredRedirect(t *testing.T) {
	clientRepo := new(MockOAuthClientRepo)
	clientRepo.On("GetByID", mock.Anything, mock.Anything).Return(testClient(nil), nil)
	uc := newAuthorizeUC(clientRepo, new(MockOAuthCodeRepo), new(MockUserRepo), new(MockJWTService))

	req := validAuthorizeRequest()
	req.RedirectURI = "https://attacker.example.com/cb"
	_, err := uc.ValidateAuthorizeRequest(context.Background(), req)
	var oe *OAuthProtocolError
	assert.True(t, errors.As(err, &oe))
	assert.Contains(t, oe.Description, "redirect_uri")
}

func TestOAuthAuthorize_WrongPasswordRejected(t *testing.T) {
	clientRepo := new(MockOAuthClientRepo)
	clientRepo.On("GetByID", mock.Anything, mock.Anything).Return(testClient(nil), nil)
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct"), bcrypt.MinCost)
	hashStr := string(hash)
	userRepo := new(MockUserRepo)
	userRepo.On("GetByEmail", mock.Anything, "a@b.com").Return(&domain.User{ID: "u1", AccountID: "acc1", PasswordHash: &hashStr, IsActive: true}, nil)

	uc := newAuthorizeUC(clientRepo, new(MockOAuthCodeRepo), userRepo, new(MockJWTService))
	req := validAuthorizeRequest()
	validated, err := uc.ValidateAuthorizeRequest(context.Background(), req)
	assert.NoError(t, err)

	_, err = uc.Authorize(context.Background(), AuthorizeDTO{
		Request: req, FlowToken: validated.FlowToken,
		Email: "a@b.com", Password: "wrong",
	})
	assert.Equal(t, domain.ErrInvalidCredentials, err)
}

func TestOAuthAuthorize_MfaBranchReturnsSession(t *testing.T) {
	clientRepo := new(MockOAuthClientRepo)
	clientRepo.On("GetByID", mock.Anything, mock.Anything).Return(testClient(nil), nil)
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.MinCost)
	hashStr := string(hash)
	userRepo := new(MockUserRepo)
	userRepo.On("GetByEmail", mock.Anything, "a@b.com").Return(&domain.User{ID: "u1", AccountID: "acc1", PasswordHash: &hashStr, IsActive: true, TOTPEnabled: true}, nil)
	jwt := new(MockJWTService)
	jwt.On("GenerateMfaLoginToken", "u1").Return("mfa-token", nil)

	uc := newAuthorizeUC(clientRepo, new(MockOAuthCodeRepo), userRepo, jwt)
	req := validAuthorizeRequest()
	validated, _ := uc.ValidateAuthorizeRequest(context.Background(), req)

	_, err := uc.Authorize(context.Background(), AuthorizeDTO{
		Request: req, FlowToken: validated.FlowToken,
		Email: "a@b.com", Password: "pass",
	})
	var mfaErr *MfaRequiredError
	assert.True(t, errors.As(err, &mfaErr))
	assert.Equal(t, "mfa-token", mfaErr.MfaSession)
}

func TestOAuthAuthorize_HappyPathIssuesCodeRedirect(t *testing.T) {
	clientRepo := new(MockOAuthClientRepo)
	clientRepo.On("GetByID", mock.Anything, mock.Anything).Return(testClient(nil), nil)
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.MinCost)
	hashStr := string(hash)
	userRepo := new(MockUserRepo)
	userRepo.On("GetByEmail", mock.Anything, "a@b.com").Return(&domain.User{ID: "u1", AccountID: "acc1", PasswordHash: &hashStr, IsActive: true}, nil)
	codeRepo := new(MockOAuthCodeRepo)
	codeRepo.On("Create", mock.Anything, mock.MatchedBy(func(c *domain.OAuthAuthorizationCode) bool {
		return c.UserID == "u1" && c.AccountID == "acc1" && c.CodeHash != "" && c.UsedAt == nil
	})).Return(nil)

	uc := newAuthorizeUC(clientRepo, codeRepo, userRepo, new(MockJWTService))
	req := validAuthorizeRequest()
	validated, _ := uc.ValidateAuthorizeRequest(context.Background(), req)

	redirect, err := uc.Authorize(context.Background(), AuthorizeDTO{
		Request: req, FlowToken: validated.FlowToken,
		Email: "a@b.com", Password: "pass",
	})
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(redirect, "https://claude.ai/api/mcp/auth_callback?code="))
	assert.Contains(t, redirect, "&state=xyz")
	codeRepo.AssertExpectations(t)
}

func TestOAuthAuthorize_TamperedFlowTokenRejected(t *testing.T) {
	clientRepo := new(MockOAuthClientRepo)
	clientRepo.On("GetByID", mock.Anything, mock.Anything).Return(testClient(nil), nil)
	uc := newAuthorizeUC(clientRepo, new(MockOAuthCodeRepo), new(MockUserRepo), new(MockJWTService))
	req := validAuthorizeRequest()
	_, err := uc.Authorize(context.Background(), AuthorizeDTO{Request: req, FlowToken: "123.bogus"})
	var oe *OAuthProtocolError
	assert.True(t, errors.As(err, &oe))
}

// --- token endpoint ---

func TestOAuthToken_CodeExchangeHappyPath(t *testing.T) {
	clientRepo := new(MockOAuthClientRepo)
	clientRepo.On("GetByID", mock.Anything, mock.Anything).Return(testClient(nil), nil)
	clientRepo.On("TouchLastUsed", mock.Anything, mock.Anything).Return(nil)
	codeRepo := new(MockOAuthCodeRepo)
	codeRepo.On("Consume", mock.Anything, domain.HashToken("the-code")).Return(&domain.ConsumedAuthCode{
		ClientID:      "01CLIENT00000000000000TEST",
		UserID:        "u1",
		AccountID:     "acc1",
		RedirectURI:   "https://claude.ai/api/mcp/auth_callback",
		Scope:         domain.OAuthScopeMCP,
		CodeChallenge: domain.S256Challenge("verifier-123"),
	}, nil)
	tokenRepo := new(MockAPITokenRepo)
	var created []*domain.APIToken
	tokenRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		created = append(created, args.Get(1).(*domain.APIToken))
	}).Return(nil)

	uc := NewOAuthTokenUseCase(clientRepo, codeRepo, tokenRepo)
	resp, err := uc.Execute(context.Background(), TokenRequestDTO{
		GrantType:    "authorization_code",
		Code:         "the-code",
		CodeVerifier: "verifier-123",
		RedirectURI:  "https://claude.ai/api/mcp/auth_callback",
		ClientID:     "01CLIENT00000000000000TEST",
	})
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp.AccessToken, domain.TokenPrefixAccess))
	assert.True(t, strings.HasPrefix(resp.RefreshToken, domain.TokenPrefixRefresh))
	assert.Len(t, created, 2)
	assert.Equal(t, *created[0].FamilyID, *created[1].FamilyID)
}

func TestOAuthToken_PKCEMismatchRejected(t *testing.T) {
	clientRepo := new(MockOAuthClientRepo)
	clientRepo.On("GetByID", mock.Anything, mock.Anything).Return(testClient(nil), nil)
	codeRepo := new(MockOAuthCodeRepo)
	codeRepo.On("Consume", mock.Anything, mock.Anything).Return(&domain.ConsumedAuthCode{
		ClientID:      "01CLIENT00000000000000TEST",
		RedirectURI:   "https://claude.ai/api/mcp/auth_callback",
		CodeChallenge: domain.S256Challenge("other-verifier"),
	}, nil)
	uc := NewOAuthTokenUseCase(clientRepo, codeRepo, new(MockAPITokenRepo))
	_, err := uc.Execute(context.Background(), TokenRequestDTO{
		GrantType: "authorization_code", Code: "c", CodeVerifier: "verifier-123",
		RedirectURI: "https://claude.ai/api/mcp/auth_callback",
		ClientID:    "01CLIENT00000000000000TEST",
	})
	var oe *OAuthProtocolError
	assert.True(t, errors.As(err, &oe))
	assert.Equal(t, "invalid_grant", oe.Code)
}

func TestOAuthToken_CodeReplayRevokesGrants(t *testing.T) {
	clientRepo := new(MockOAuthClientRepo)
	clientRepo.On("GetByID", mock.Anything, mock.Anything).Return(testClient(nil), nil)
	usedAt := time.Now()
	codeRepo := new(MockOAuthCodeRepo)
	codeRepo.On("Consume", mock.Anything, mock.Anything).Return(nil, nil)
	codeRepo.On("GetByHash", mock.Anything, mock.Anything).Return(&domain.OAuthAuthorizationCode{
		ClientID: "01CLIENT00000000000000TEST", UserID: "u1", UsedAt: &usedAt,
	}, nil)
	tokenRepo := new(MockAPITokenRepo)
	tokenRepo.On("RevokeOAuthConnection", mock.Anything, "u1", "01CLIENT00000000000000TEST").Return(int64(2), nil)

	uc := NewOAuthTokenUseCase(clientRepo, codeRepo, tokenRepo)
	_, err := uc.Execute(context.Background(), TokenRequestDTO{
		GrantType: "authorization_code", Code: "replayed", CodeVerifier: "v",
		ClientID: "01CLIENT00000000000000TEST",
	})
	assert.Error(t, err)
	tokenRepo.AssertExpectations(t)
}

func TestOAuthToken_RefreshRotationAndReuseDetection(t *testing.T) {
	clientID := "01CLIENT00000000000000TEST"
	familyID := "01FAMILY00000000000000TEST"
	future := time.Now().Add(time.Hour)

	// Happy rotation.
	clientRepo := new(MockOAuthClientRepo)
	clientRepo.On("GetByID", mock.Anything, mock.Anything).Return(testClient(nil), nil)
	clientRepo.On("TouchLastUsed", mock.Anything, mock.Anything).Return(nil)
	tokenRepo := new(MockAPITokenRepo)
	tokenRepo.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(&domain.APIToken{
		ID: "tok1", ClientID: &clientID, FamilyID: &familyID,
		UserID: "u1", AccountID: "acc1", ExpiresAt: &future,
	}, nil)
	tokenRepo.On("Revoke", mock.Anything, "tok1").Return(nil)
	tokenRepo.On("Create", mock.Anything, mock.MatchedBy(func(tk *domain.APIToken) bool {
		return *tk.FamilyID == familyID
	})).Return(nil)

	uc := NewOAuthTokenUseCase(clientRepo, new(MockOAuthCodeRepo), tokenRepo)
	resp, err := uc.Execute(context.Background(), TokenRequestDTO{
		GrantType: "refresh_token", RefreshToken: "rt", ClientID: clientID,
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.AccessToken)
	tokenRepo.AssertExpectations(t)

	// Reuse of a revoked refresh token revokes the family.
	revokedAt := time.Now()
	tokenRepo2 := new(MockAPITokenRepo)
	tokenRepo2.On("GetRefreshTokenByHash", mock.Anything, mock.Anything).Return(&domain.APIToken{
		ID: "tok1", ClientID: &clientID, FamilyID: &familyID, RevokedAt: &revokedAt,
	}, nil)
	tokenRepo2.On("RevokeFamily", mock.Anything, familyID).Return(nil)
	uc2 := NewOAuthTokenUseCase(clientRepo, new(MockOAuthCodeRepo), tokenRepo2)
	_, err = uc2.Execute(context.Background(), TokenRequestDTO{
		GrantType: "refresh_token", RefreshToken: "rt", ClientID: clientID,
	})
	var oe *OAuthProtocolError
	assert.True(t, errors.As(err, &oe))
	assert.Equal(t, "invalid_grant", oe.Code)
	tokenRepo2.AssertExpectations(t)
}

// --- PAT ---

func TestMCPPat_CreateShownOnceWithPrefix(t *testing.T) {
	tokenRepo := new(MockAPITokenRepo)
	tokenRepo.On("CountActivePATsByUser", mock.Anything, "u1").Return(int64(0), nil)
	tokenRepo.On("Create", mock.Anything, mock.MatchedBy(func(tk *domain.APIToken) bool {
		return tk.TokenType == domain.APITokenTypePAT && tk.TokenHash != "" && tk.UserID == "u1"
	})).Return(nil)

	uc := NewMCPPatUseCase(tokenRepo)
	resp, err := uc.Create(context.Background(), CreatePATDTO{UserID: "u1", AccountID: "acc1", Name: "Meu Claude"})
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp.Token, domain.TokenPrefixPAT))
	assert.True(t, strings.HasPrefix(resp.Token, resp.TokenPrefix))
}

func TestMCPPat_CapEnforced(t *testing.T) {
	tokenRepo := new(MockAPITokenRepo)
	tokenRepo.On("CountActivePATsByUser", mock.Anything, "u1").Return(int64(domain.MaxActivePATsPerUser), nil)
	uc := NewMCPPatUseCase(tokenRepo)
	_, err := uc.Create(context.Background(), CreatePATDTO{UserID: "u1", AccountID: "acc1"})
	assert.Equal(t, domain.ErrPATLimitReached, err)
}

func TestMCPPat_RevokeScopedToOwner(t *testing.T) {
	tokenRepo := new(MockAPITokenRepo)
	tokenRepo.On("RevokePATByIDAndUser", mock.Anything, "tok1", "u1").Return(int64(0), nil)
	uc := NewMCPPatUseCase(tokenRepo)
	err := uc.Revoke(context.Background(), "u1", "tok1")
	assert.Equal(t, domain.ErrPATNotFound, err)
}

package usecases

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

// makeTwoFactorUC wires a TwoFactorUseCase against a fresh MockUserRepo.
// All tests in this file use the mock — TOTP math is exercised against
// the real pquerna/otp library so we cover the actual code paths a
// production user hits.
func makeTwoFactorUC(t *testing.T) (*TwoFactorUseCase, *MockUserRepo) {
	t.Helper()
	userRepo := new(MockUserRepo)
	uc := NewTwoFactorUseCase(userRepo, "Replider")
	return uc, userRepo
}

func TestTwoFactor_Setup_StagesSecretWithoutEnabling(t *testing.T) {
	uc, userRepo := makeTwoFactorUC(t)
	userRepo.On("SetTOTPSecret", mock.Anything, "u-1", mock.Anything).Return(nil)

	out, err := uc.Setup(context.Background(), "u-1", "alice@example.com")
	assert.NoError(t, err)
	assert.NotEmpty(t, out.Secret, "secret must be returned for QR rendering")
	assert.Contains(t, out.OTPAuthURL, "otpauth://totp/")
	assert.Contains(t, out.OTPAuthURL, "Replider")
	userRepo.AssertCalled(t, "SetTOTPSecret", mock.Anything, "u-1", out.Secret)
}

func TestTwoFactor_Setup_RejectsBlankInputs(t *testing.T) {
	uc, _ := makeTwoFactorUC(t)
	_, err := uc.Setup(context.Background(), "", "alice@example.com")
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "INVALID_REQUEST", appErr.Code)
}

// The /me/2fa/setup handler passes an empty email (authMw never stamps
// userEmail — only the super-admin middleware does), so Setup must
// resolve the address from the user row instead of failing with
// "user is required". Regression for the broken "Ativar 2FA" enroll.
func TestTwoFactor_Setup_ResolvesEmailFromRepoWhenBlank(t *testing.T) {
	uc, userRepo := makeTwoFactorUC(t)
	userRepo.On("GetByID", mock.Anything, "u-1").Return(&domain.User{
		ID:    "u-1",
		Email: "owner@example.com",
	}, nil)
	userRepo.On("SetTOTPSecret", mock.Anything, "u-1", mock.Anything).Return(nil)

	out, err := uc.Setup(context.Background(), "u-1", "")
	assert.NoError(t, err)
	assert.NotEmpty(t, out.Secret)
	// The resolved email is the authenticator AccountName label.
	assert.Contains(t, out.OTPAuthURL, "owner@example.com")
}

// A typed *AppError from the repo (e.g. TOTP_CIPHER_UNCONFIGURED when
// the server has no TOTP_KEK) must surface unwrapped so respondError
// maps it to a clear 4xx instead of an opaque 500. Regression for the
// "internal server error" the owner saw on Ativar 2FA.
func TestTwoFactor_Setup_SurfacesRepoAppErrorUnwrapped(t *testing.T) {
	uc, userRepo := makeTwoFactorUC(t)
	cipherErr := &domain.AppError{Code: "TOTP_CIPHER_UNCONFIGURED", Message: "2FA is not available"}
	userRepo.On("SetTOTPSecret", mock.Anything, "u-1", mock.Anything).Return(cipherErr)

	_, err := uc.Setup(context.Background(), "u-1", "alice@example.com")
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok, "repo AppError must not be wrapped away")
	assert.Equal(t, "TOTP_CIPHER_UNCONFIGURED", appErr.Code)
}

// Status must read the enrolled flag from the user row, not a gin
// context value (nothing stamps one on /me/2fa/status), so an enrolled
// user reports enabled=true. Regression for "clicked done but 2FA never
// showed as active".
func TestTwoFactor_Status_ReadsEnrolledFlagFromUserRow(t *testing.T) {
	uc, userRepo := makeTwoFactorUC(t)
	userRepo.On("GetByID", mock.Anything, "u-1").Return(&domain.User{
		ID:          "u-1",
		Email:       "owner@example.com",
		TOTPEnabled: true,
	}, nil)

	enabled, err := uc.Status(context.Background(), "u-1")
	assert.NoError(t, err)
	assert.True(t, enabled)
}

func TestTwoFactor_Verify_InvalidCodeReturnsAppError(t *testing.T) {
	uc, userRepo := makeTwoFactorUC(t)
	secret := newTestSecret(t)
	userRepo.On("GetByID", mock.Anything, "u-1").Return(&domain.User{
		ID: "u-1", TOTPSecret: &secret,
	}, nil)

	_, err := uc.Verify(context.Background(), "u-1", "000000")
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "TOTP_INVALID_CODE", appErr.Code)
}

func TestTwoFactor_Verify_HappyPathEnablesAndReturnsRecoveryCodes(t *testing.T) {
	uc, userRepo := makeTwoFactorUC(t)
	secret := newTestSecret(t)
	userRepo.On("GetByID", mock.Anything, "u-1").Return(&domain.User{
		ID: "u-1", TOTPSecret: &secret,
	}, nil)
	userRepo.On("EnableTOTP", mock.Anything, "u-1", mock.MatchedBy(func(codes []string) bool {
		return len(codes) == 10
	})).Return(nil)

	code, err := totp.GenerateCode(secret, time.Now())
	assert.NoError(t, err)
	out, err := uc.Verify(context.Background(), "u-1", code)
	assert.NoError(t, err)
	assert.Len(t, out.RecoveryCodes, 10)
	for _, rc := range out.RecoveryCodes {
		assert.Equal(t, 9, len(rc), "recovery codes are 8 chars + dash")
		assert.True(t, strings.Contains(rc, "-"))
	}
}

func TestTwoFactor_VerifyForLogin_AcceptsTOTPCode(t *testing.T) {
	uc, userRepo := makeTwoFactorUC(t)
	secret := newTestSecret(t)
	userRepo.On("GetByID", mock.Anything, "u-1").Return(&domain.User{
		ID: "u-1", TOTPSecret: &secret, TOTPEnabled: true,
	}, nil)
	userRepo.On("StampTOTPVerified", mock.Anything, "u-1").Return(nil)

	code, _ := totp.GenerateCode(secret, time.Now())
	err := uc.VerifyForLogin(context.Background(), "u-1", code)
	assert.NoError(t, err)
}

func TestTwoFactor_VerifyForLogin_AcceptsRecoveryCodeAndConsumesIt(t *testing.T) {
	uc, userRepo := makeTwoFactorUC(t)
	plain := "ABCD-EFGH"
	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.MinCost)
	assert.NoError(t, err)

	userRepo.On("GetByID", mock.Anything, "u-1").Return(&domain.User{
		ID:                "u-1",
		TOTPSecret:        ptrString("JBSWY3DPEHPK3PXP"),
		TOTPEnabled:       true,
		TOTPRecoveryCodes: []string{string(hashed)},
	}, nil)
	userRepo.On("ConsumeTOTPRecoveryCode", mock.Anything, "u-1", string(hashed)).Return(true, nil)
	userRepo.On("StampTOTPVerified", mock.Anything, "u-1").Return(nil)

	err = uc.VerifyForLogin(context.Background(), "u-1", plain)
	assert.NoError(t, err)
	userRepo.AssertCalled(t, "ConsumeTOTPRecoveryCode", mock.Anything, "u-1", string(hashed))
}

// A recovery code that matches a stored hash but has already been
// consumed (another login raced ahead and ConsumeTOTPRecoveryCode
// removed it, reporting ok=false) MUST NOT grant access. This is the
// security crux of US-077: a plaintext-on-leak bypass is closed by
// hashing, and reuse of a spent code is closed by the consume guard.
func TestTwoFactor_VerifyForLogin_RejectsAlreadyConsumedRecoveryCode(t *testing.T) {
	uc, userRepo := makeTwoFactorUC(t)
	plain := "ABCD-EFGH"
	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.MinCost)
	assert.NoError(t, err)

	userRepo.On("GetByID", mock.Anything, "u-1").Return(&domain.User{
		ID:                "u-1",
		TOTPSecret:        ptrString("JBSWY3DPEHPK3PXP"),
		TOTPEnabled:       true,
		TOTPRecoveryCodes: []string{string(hashed)},
	}, nil)
	// Hash matches but the row was already removed → ok=false.
	userRepo.On("ConsumeTOTPRecoveryCode", mock.Anything, "u-1", string(hashed)).Return(false, nil)

	err = uc.VerifyForLogin(context.Background(), "u-1", plain)
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok, "expected AppError, got %v", err)
	assert.Equal(t, "TOTP_INVALID_CODE", appErr.Code)
	// A spent code must never be re-stamped as a successful verification.
	userRepo.AssertNotCalled(t, "StampTOTPVerified", mock.Anything, "u-1")
}

// Recovery codes are persisted only as bcrypt hashes — never plaintext.
// Verify enables 2FA with hashes that (a) are valid bcrypt and (b) match
// the plaintext shown once to the user, so a DB leak yields no usable code.
func TestTwoFactor_Verify_PersistsBcryptHashesNotPlaintext(t *testing.T) {
	uc, userRepo := makeTwoFactorUC(t)
	secret := newTestSecret(t)
	userRepo.On("GetByID", mock.Anything, "u-1").Return(&domain.User{
		ID: "u-1", TOTPSecret: &secret,
	}, nil)

	var stored []string
	userRepo.On("EnableTOTP", mock.Anything, "u-1", mock.Anything).
		Run(func(args mock.Arguments) {
			stored = args.Get(2).([]string)
		}).Return(nil)

	code, err := totp.GenerateCode(secret, time.Now())
	assert.NoError(t, err)
	out, err := uc.Verify(context.Background(), "u-1", code)
	assert.NoError(t, err)

	assert.Len(t, stored, 10)
	for i, h := range stored {
		// Stored value must not be the plaintext code.
		assert.NotEqual(t, out.RecoveryCodes[i], h, "stored value must be hashed, not plaintext")
		assert.True(t, strings.HasPrefix(h, "$2"), "stored value must be a bcrypt hash")
		// The hash must verify against exactly the plaintext shown to the user.
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(h), []byte(out.RecoveryCodes[i])))
	}
}

func TestTwoFactor_VerifyForLogin_RejectsWhenNotEnabled(t *testing.T) {
	uc, userRepo := makeTwoFactorUC(t)
	userRepo.On("GetByID", mock.Anything, "u-1").Return(&domain.User{
		ID: "u-1", TOTPEnabled: false,
	}, nil)

	err := uc.VerifyForLogin(context.Background(), "u-1", "123456")
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "TOTP_NOT_ENABLED", appErr.Code)
}

func TestTwoFactor_Disable_RequiresProof(t *testing.T) {
	uc, _ := makeTwoFactorUC(t)
	err := uc.Disable(context.Background(), "u-1", "")
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "TOTP_PROOF_REQUIRED", appErr.Code)
}

func TestTwoFactor_Disable_AfterValidProof(t *testing.T) {
	uc, userRepo := makeTwoFactorUC(t)
	secret := newTestSecret(t)
	userRepo.On("GetByID", mock.Anything, "u-1").Return(&domain.User{
		ID: "u-1", TOTPSecret: &secret, TOTPEnabled: true,
	}, nil)
	userRepo.On("StampTOTPVerified", mock.Anything, "u-1").Return(nil)
	userRepo.On("DisableTOTP", mock.Anything, "u-1").Return(nil)

	code, _ := totp.GenerateCode(secret, time.Now())
	err := uc.Disable(context.Background(), "u-1", code)
	assert.NoError(t, err)
	userRepo.AssertCalled(t, "DisableTOTP", mock.Anything, "u-1")
}

// newTestSecret returns a Base32-encoded TOTP secret without persisting
// it — fixture for the validate path.
func newTestSecret(t *testing.T) string {
	t.Helper()
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "Replider", AccountName: "test@example.com"})
	if err != nil {
		t.Fatalf("totp.Generate: %v", err)
	}
	return key.Secret()
}

func ptrString(s string) *string { return &s }

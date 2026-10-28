package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const testPlatformAccountID = "01PLATFORMACCOUNTID0000000"

func TestCreateAccount_SellerRefAttribution(t *testing.T) {
	ctx := context.Background()

	setupSignupMocks := func() (*MockAccountRepo, *MockUserRepo, *MockUsageRepo, *MockTxManager, *MockEmailService) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		emailSvc := new(MockEmailService)
		emailSvc.On("SendVerificationEmail", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		userRepo.On("GetByEmail", ctx, "new@acme.com").Return(nil, nil)
		accountRepo.On("Create", ctx, mock.Anything).Return(nil)
		userRepo.On("Create", ctx, mock.Anything).Return(nil)
		userRepo.On("SetVerificationToken", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		usageRepo.On("Create", ctx, mock.Anything).Return(nil)
		return accountRepo, userRepo, usageRepo, txManager, emailSvc
	}

	newUC := func() (*CreateAccountUseCase, *MockSellerRefRepository, *MockReferralRepository) {
		accountRepo, userRepo, usageRepo, txManager, emailSvc := setupSignupMocks()
		uc := NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, emailSvc, "http://localhost:8080")
		sellerRepo := new(MockSellerRefRepository)
		referralRepo := new(MockReferralRepository)
		// Both attribution paths wired at once — mirrors production, where
		// a single ?ref= param serves two separate namespaces (US-003 vs
		// US-174).
		uc.SetSellerAttribution(sellerRepo, testPlatformAccountID)
		uc.SetReferralLinker(referralRepo)
		return uc, sellerRepo, referralRepo
	}

	dto := func(ref string) CreateAccountDTO {
		return CreateAccountDTO{
			AccountName: "Acme", AdminName: "A", AdminEmail: "new@acme.com",
			Password: "securepassword123", TermsAccepted: true, ReferralCode: ref,
		}
	}

	t.Run("seller code stamps seller attribution and skips the referral path", func(t *testing.T) {
		uc, sellerRepo, referralRepo := newUC()
		sellerRepo.On("ResolveSellerRef", ctx, "SELLER01", testPlatformAccountID).Return("user_seller", "seller@vendas.com", nil)
		sellerRepo.On("StampSellerAttribution", ctx, mock.AnythingOfType("string"), "user_seller").Return(nil)

		assert.NoError(t, uc.Execute(ctx, dto("SELLER01")))
		sellerRepo.AssertExpectations(t)
		referralRepo.AssertNotCalled(t, "ResolveReferrer", mock.Anything, mock.Anything)
		referralRepo.AssertNotCalled(t, "LinkReferral", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("customer referral code is not a seller code and follows the referral path", func(t *testing.T) {
		uc, sellerRepo, referralRepo := newUC()
		sellerRepo.On("ResolveSellerRef", ctx, "FRIEND01", testPlatformAccountID).Return("", "", nil)
		referralRepo.On("ResolveReferrer", ctx, "FRIEND01").Return("acc_referrer", nil)
		referralRepo.On("LinkReferral", ctx, mock.AnythingOfType("string"), "acc_referrer").Return(nil)

		assert.NoError(t, uc.Execute(ctx, dto("FRIEND01")))
		sellerRepo.AssertNotCalled(t, "StampSellerAttribution", mock.Anything, mock.Anything, mock.Anything)
		referralRepo.AssertExpectations(t)
	})

	t.Run("unknown code matches neither path and never fails signup", func(t *testing.T) {
		uc, sellerRepo, referralRepo := newUC()
		sellerRepo.On("ResolveSellerRef", ctx, "BOGUS", testPlatformAccountID).Return("", "", nil)
		referralRepo.On("ResolveReferrer", ctx, "BOGUS").Return("", nil)

		assert.NoError(t, uc.Execute(ctx, dto("BOGUS")))
		sellerRepo.AssertNotCalled(t, "StampSellerAttribution", mock.Anything, mock.Anything, mock.Anything)
		referralRepo.AssertNotCalled(t, "LinkReferral", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("seller resolution error falls back to the referral path silently", func(t *testing.T) {
		uc, sellerRepo, referralRepo := newUC()
		sellerRepo.On("ResolveSellerRef", ctx, "SELLER01", testPlatformAccountID).Return("", "", assert.AnError)
		referralRepo.On("ResolveReferrer", ctx, "SELLER01").Return("", nil)

		assert.NoError(t, uc.Execute(ctx, dto("SELLER01")))
		sellerRepo.AssertNotCalled(t, "StampSellerAttribution", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("stamp error still consumes the code as a seller ref (never double-attributes)", func(t *testing.T) {
		uc, sellerRepo, referralRepo := newUC()
		sellerRepo.On("ResolveSellerRef", ctx, "SELLER01", testPlatformAccountID).Return("user_seller", "seller@vendas.com", nil)
		sellerRepo.On("StampSellerAttribution", ctx, mock.AnythingOfType("string"), "user_seller").Return(assert.AnError)

		assert.NoError(t, uc.Execute(ctx, dto("SELLER01")))
		referralRepo.AssertNotCalled(t, "ResolveReferrer", mock.Anything, mock.Anything)
	})

	t.Run("self-referral (same email) is ignored: account gets NO seller attribution", func(t *testing.T) {
		uc, sellerRepo, referralRepo := newUC()
		// The seller behind SELLER01 signs up their own account with the
		// same email (different case — the guard is case-insensitive).
		sellerRepo.On("ResolveSellerRef", ctx, "SELLER01", testPlatformAccountID).Return("user_seller", "New@Acme.COM", nil)

		assert.NoError(t, uc.Execute(ctx, dto("SELLER01")))
		sellerRepo.AssertNotCalled(t, "StampSellerAttribution", mock.Anything, mock.Anything, mock.Anything)
		// Code was still consumed as a seller code — no referral fallback.
		referralRepo.AssertNotCalled(t, "ResolveReferrer", mock.Anything, mock.Anything)
		referralRepo.AssertNotCalled(t, "LinkReferral", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("a different seller's code still attributes normally", func(t *testing.T) {
		uc, sellerRepo, _ := newUC()
		sellerRepo.On("ResolveSellerRef", ctx, "SELLER02", testPlatformAccountID).Return("user_other_seller", "other@vendas.com", nil)
		sellerRepo.On("StampSellerAttribution", ctx, mock.AnythingOfType("string"), "user_other_seller").Return(nil)

		assert.NoError(t, uc.Execute(ctx, dto("SELLER02")))
		sellerRepo.AssertExpectations(t)
	})

	t.Run("PLATFORM_ACCOUNT_ID unset disables seller attribution but not referral", func(t *testing.T) {
		accountRepo, userRepo, usageRepo, txManager, emailSvc := setupSignupMocks()
		uc := NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, emailSvc, "http://localhost:8080")
		sellerRepo := new(MockSellerRefRepository)
		referralRepo := new(MockReferralRepository)
		uc.SetSellerAttribution(sellerRepo, "") // feature off
		uc.SetReferralLinker(referralRepo)
		referralRepo.On("ResolveReferrer", ctx, "ANYCODE1").Return("", nil)

		assert.NoError(t, uc.Execute(ctx, dto("ANYCODE1")))
		sellerRepo.AssertNotCalled(t, "ResolveSellerRef", mock.Anything, mock.Anything, mock.Anything)
		referralRepo.AssertExpectations(t)
	})
}

func TestGoogleAuth_SellerRefAttribution(t *testing.T) {
	ctx := context.Background()
	originalVerify := verifyGoogleTokenFunc
	t.Cleanup(func() { verifyGoogleTokenFunc = originalVerify })

	verifyGoogleTokenFunc = func(idToken string) (*GoogleTokenInfo, error) {
		return &GoogleTokenInfo{
			Sub: "google-777", Aud: "my-client-id",
			Email: "prospect@gmail.com", EmailVerified: "true", Name: "Prospect",
		}, nil
	}

	t.Run("new Google signup with a seller ?ref= stamps the new account", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)
		sellerRepo := new(MockSellerRefRepository)

		userRepo.On("GetByGoogleID", ctx, "google-777").Return(nil, nil)
		userRepo.On("GetByEmail", ctx, "prospect@gmail.com").Return(nil, nil)
		var createdAccountID string
		accountRepo.On("Create", ctx, mock.MatchedBy(func(a *domain.Account) bool {
			createdAccountID = a.ID
			return len(a.ID) == 26
		})).Return(nil)
		userRepo.On("Create", ctx, mock.Anything).Return(nil)
		usageRepo.On("Create", ctx, mock.Anything).Return(nil)
		jwtService.On("GenerateToken", mock.AnythingOfType("*domain.User")).Return("jwt", nil)
		sellerRepo.On("ResolveSellerRef", ctx, "SELLER01", testPlatformAccountID).Return("user_seller", "seller@vendas.com", nil)
		sellerRepo.On("StampSellerAttribution", ctx, mock.MatchedBy(func(id string) bool {
			return id == createdAccountID
		}), "user_seller").Return(nil)

		uc := NewGoogleAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService, "my-client-id")
		uc.SetSellerAttribution(sellerRepo, testPlatformAccountID)
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "tok", ReferralCode: "SELLER01"})

		assert.NoError(t, err)
		assert.True(t, resp.IsNew)
		sellerRepo.AssertExpectations(t)
	})

	t.Run("Google signup with the seller's own email ignores the self-referral", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)
		sellerRepo := new(MockSellerRefRepository)

		userRepo.On("GetByGoogleID", ctx, "google-777").Return(nil, nil)
		userRepo.On("GetByEmail", ctx, "prospect@gmail.com").Return(nil, nil)
		accountRepo.On("Create", ctx, mock.Anything).Return(nil)
		userRepo.On("Create", ctx, mock.Anything).Return(nil)
		usageRepo.On("Create", ctx, mock.Anything).Return(nil)
		jwtService.On("GenerateToken", mock.AnythingOfType("*domain.User")).Return("jwt", nil)
		// The code resolves to a seller whose email IS the signup email.
		sellerRepo.On("ResolveSellerRef", ctx, "SELLER01", testPlatformAccountID).Return("user_seller", "prospect@gmail.com", nil)

		uc := NewGoogleAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService, "my-client-id")
		uc.SetSellerAttribution(sellerRepo, testPlatformAccountID)
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "tok", ReferralCode: "SELLER01"})

		assert.NoError(t, err)
		assert.True(t, resp.IsNew)
		sellerRepo.AssertNotCalled(t, "StampSellerAttribution", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("existing Google user never triggers seller attribution", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)
		sellerRepo := new(MockSellerRefRepository)

		existing := &domain.User{ID: "u1", AccountID: "acc1", Email: "prospect@gmail.com"}
		userRepo.On("GetByGoogleID", ctx, "google-777").Return(existing, nil)
		jwtService.On("GenerateToken", existing).Return("jwt", nil)
		accountRepo.On("GetByID", ctx, "acc1").Return(&domain.Account{ID: "acc1", OnboardingCompleted: true}, nil)

		uc := NewGoogleAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService, "my-client-id")
		uc.SetSellerAttribution(sellerRepo, testPlatformAccountID)
		_, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "tok", ReferralCode: "SELLER01"})

		assert.NoError(t, err)
		sellerRepo.AssertNotCalled(t, "ResolveSellerRef", mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestIsSellerSelfReferral(t *testing.T) {
	assert.True(t, isSellerSelfReferral("u1", "a@b.com", "u1", "c@d.com"), "same user id matches")
	assert.True(t, isSellerSelfReferral("u1", "Seller@X.com", "u2", " seller@x.com "), "same email matches case-insensitively and trimmed")
	assert.False(t, isSellerSelfReferral("u1", "a@b.com", "u2", "c@d.com"), "different person")
	assert.False(t, isSellerSelfReferral("u1", "", "u2", ""), "empty emails never match vacuously")
	assert.False(t, isSellerSelfReferral("", "", "", ""), "all empty never matches")
}

func TestEnsureSellerRefCode_Execute(t *testing.T) {
	ctx := context.Background()
	sellerRepo := new(MockSellerRefRepository)
	sellerRepo.On("GetOrCreateSellerRefCode", ctx, "user1").Return("VNDR2345", nil)

	uc := NewEnsureSellerRefCodeUseCase(sellerRepo, "https://replider.com")
	info, err := uc.Execute(ctx, "user1")

	assert.NoError(t, err)
	assert.Equal(t, "VNDR2345", info.Code)
	// Canonical prospecting link lives on the marketing site — the landing
	// forwards ?ref= into the app register flow.
	assert.Equal(t, "https://replider.com/register?ref=VNDR2345", info.ShareURL)
	sellerRepo.AssertExpectations(t)
}

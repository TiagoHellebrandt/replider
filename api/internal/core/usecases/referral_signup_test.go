package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateAccount_ReferralLinking(t *testing.T) {
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

	t.Run("valid ?ref=CODE links the new account to its referrer", func(t *testing.T) {
		accountRepo, userRepo, usageRepo, txManager, emailSvc := setupSignupMocks()
		referralRepo := new(MockReferralRepository)
		referralRepo.On("ResolveReferrer", ctx, "FRIEND01").Return("acc_referrer", nil)
		referralRepo.On("LinkReferral", ctx, mock.AnythingOfType("string"), "acc_referrer").Return(nil)

		uc := NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, emailSvc, "http://localhost:8080")
		uc.SetReferralLinker(referralRepo)

		err := uc.Execute(ctx, CreateAccountDTO{
			AccountName: "Acme", AdminName: "A", AdminEmail: "new@acme.com",
			Password: "securepassword123", TermsAccepted: true, ReferralCode: "FRIEND01",
		})
		assert.NoError(t, err)
		referralRepo.AssertExpectations(t)
	})

	t.Run("unknown referral code is ignored and never fails signup", func(t *testing.T) {
		accountRepo, userRepo, usageRepo, txManager, emailSvc := setupSignupMocks()
		referralRepo := new(MockReferralRepository)
		referralRepo.On("ResolveReferrer", ctx, "BOGUS").Return("", nil)

		uc := NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, emailSvc, "http://localhost:8080")
		uc.SetReferralLinker(referralRepo)

		err := uc.Execute(ctx, CreateAccountDTO{
			AccountName: "Acme", AdminName: "A", AdminEmail: "new@acme.com",
			Password: "securepassword123", TermsAccepted: true, ReferralCode: "BOGUS",
		})
		assert.NoError(t, err)
		referralRepo.AssertNotCalled(t, "LinkReferral", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("no referral code skips linking entirely", func(t *testing.T) {
		accountRepo, userRepo, usageRepo, txManager, emailSvc := setupSignupMocks()
		referralRepo := new(MockReferralRepository)

		uc := NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, emailSvc, "http://localhost:8080")
		uc.SetReferralLinker(referralRepo)

		err := uc.Execute(ctx, CreateAccountDTO{
			AccountName: "Acme", AdminName: "A", AdminEmail: "new@acme.com",
			Password: "securepassword123", TermsAccepted: true,
		})
		assert.NoError(t, err)
		referralRepo.AssertNotCalled(t, "ResolveReferrer", mock.Anything, mock.Anything)
	})
}

func TestGetReferralInfo_Execute(t *testing.T) {
	ctx := context.Background()
	referralRepo := new(MockReferralRepository)
	referralRepo.On("GetOrCreateCode", ctx, "acc1").Return("ABCD2345", nil)
	referralRepo.On("StatsByReferrer", ctx, "acc1").Return(&domain.ReferralStats{TotalReferred: 3, TotalRewarded: 1}, nil)

	uc := NewGetReferralInfoUseCase(referralRepo, "https://app.replider.com")
	info, err := uc.Execute(ctx, "acc1")

	assert.NoError(t, err)
	assert.Equal(t, "ABCD2345", info.Code)
	assert.Equal(t, "https://app.replider.com/register?ref=ABCD2345", info.ShareURL)
	assert.Equal(t, 3, info.Stats.TotalReferred)
	assert.Equal(t, 1, info.Stats.TotalRewarded)
	referralRepo.AssertExpectations(t)
}

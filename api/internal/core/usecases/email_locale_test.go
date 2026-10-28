package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// US-018 review fixes: transactional emails resolve locale from the
// recipient account's billing currency. Before the fix every composer
// hardcoded pt-BR, so a USD (English-signup) customer got Portuguese
// password-reset / payment-failed emails.

func TestForgotPassword_USDAccountGetsEnglishEmail(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)
	accountRepo := new(MockAccountRepo)
	emailSvc := new(MockEmailService)

	user := &domain.User{ID: "user1", AccountID: "acc-usd", Email: "user@example.com"}
	userRepo.On("GetByEmail", ctx, "user@example.com").Return(user, nil)
	userRepo.On("SetResetToken", ctx, "user1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)
	accountRepo.On("GetByID", ctx, "acc-usd").Return(&domain.Account{ID: "acc-usd", Currency: "usd"}, nil)
	emailSvc.On("SendPasswordResetEmail", "user@example.com", mock.AnythingOfType("string"),
		"http://localhost:8080", "en").Return(nil)

	uc := NewForgotPasswordUseCase(userRepo, emailSvc, "http://localhost:8080")
	uc.SetAccountRepo(accountRepo)

	assert.NoError(t, uc.Execute(ctx, ForgotPasswordDTO{Email: "user@example.com"}))
	emailSvc.AssertExpectations(t)
}

func TestForgotPassword_BRLAccountKeepsPortuguese(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)
	accountRepo := new(MockAccountRepo)
	emailSvc := new(MockEmailService)

	user := &domain.User{ID: "user1", AccountID: "acc-brl", Email: "user@example.com"}
	userRepo.On("GetByEmail", ctx, "user@example.com").Return(user, nil)
	userRepo.On("SetResetToken", ctx, "user1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)
	accountRepo.On("GetByID", ctx, "acc-brl").Return(&domain.Account{ID: "acc-brl", Currency: "brl"}, nil)
	emailSvc.On("SendPasswordResetEmail", "user@example.com", mock.AnythingOfType("string"),
		"http://localhost:8080", "pt-BR").Return(nil)

	uc := NewForgotPasswordUseCase(userRepo, emailSvc, "http://localhost:8080")
	uc.SetAccountRepo(accountRepo)

	assert.NoError(t, uc.Execute(ctx, ForgotPasswordDTO{Email: "user@example.com"}))
	emailSvc.AssertExpectations(t)
}

// Unwired resolver (or a failed account lookup) must never block the
// send — it degrades to "" which email.resolveLocale reads as pt-BR.
func TestForgotPassword_NoAccountRepoDegradesToDefault(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)
	emailSvc := new(MockEmailService)

	user := &domain.User{ID: "user1", AccountID: "acc-1", Email: "user@example.com"}
	userRepo.On("GetByEmail", ctx, "user@example.com").Return(user, nil)
	userRepo.On("SetResetToken", ctx, "user1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)
	emailSvc.On("SendPasswordResetEmail", "user@example.com", mock.AnythingOfType("string"),
		"http://localhost:8080", "").Return(nil)

	uc := NewForgotPasswordUseCase(userRepo, emailSvc, "http://localhost:8080")

	assert.NoError(t, uc.Execute(ctx, ForgotPasswordDTO{Email: "user@example.com"}))
	emailSvc.AssertExpectations(t)
}

// The Stripe payment-failed notification localizes both the email and
// the "your plan" fallback name for USD accounts.
func TestNotifyPaymentFailed_USDAccountGetsEnglish(t *testing.T) {
	userRepo := new(MockUserRepo)
	accountRepo := new(MockAccountRepo)
	emailSvc := new(MockEmailService)

	admin := &domain.User{ID: "adm1", Email: "admin@x.com", Name: "Admin"}
	userRepo.On("GetAdminByAccountID", mock.Anything, "acc-usd").Return(admin, nil)
	accountRepo.On("GetByID", mock.Anything, "acc-usd").Return(&domain.Account{ID: "acc-usd", Currency: "usd"}, nil)
	emailSvc.On("SendPaymentFailedEmail", "admin@x.com", "Admin", "your plan",
		"https://app.x.com/settings?billing=retry", "en").Return(nil)

	uc := NewProcessStripeWebhookUseCase(accountRepo, new(MockUsageRepo), new(MockStripeEventRepo))
	uc.SetEmailNotifier(userRepo, nil, emailSvc, "https://app.x.com")

	uc.notifyPaymentFailed("acc-usd")
	emailSvc.AssertExpectations(t)
}

func TestNotifyPaymentFailed_BRLAccountKeepsPortuguese(t *testing.T) {
	userRepo := new(MockUserRepo)
	accountRepo := new(MockAccountRepo)
	emailSvc := new(MockEmailService)

	admin := &domain.User{ID: "adm1", Email: "admin@x.com", Name: "Admin"}
	userRepo.On("GetAdminByAccountID", mock.Anything, "acc-brl").Return(admin, nil)
	accountRepo.On("GetByID", mock.Anything, "acc-brl").Return(&domain.Account{ID: "acc-brl", Currency: "brl"}, nil)
	emailSvc.On("SendPaymentFailedEmail", "admin@x.com", "Admin", "seu plano",
		"https://app.x.com/settings?billing=retry", "pt-BR").Return(nil)

	uc := NewProcessStripeWebhookUseCase(accountRepo, new(MockUsageRepo), new(MockStripeEventRepo))
	uc.SetEmailNotifier(userRepo, nil, emailSvc, "https://app.x.com")

	uc.notifyPaymentFailed("acc-brl")
	emailSvc.AssertExpectations(t)
}

// Team invites follow the inviting account's currency.
func TestInviteTeamMember_USDAccountSendsEnglishInvite(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	emailSvc := new(MockEmailService)

	plan := &domain.Plan{ID: "plan-1", MaxSeats: 10}
	planID := "plan-1"
	account := &domain.Account{ID: "acc-usd", Currency: "usd", PlanID: &planID}
	accountRepo.On("GetByID", ctx, "acc-usd").Return(account, nil)
	planRepo.On("GetByID", ctx, "plan-1").Return(plan, nil)
	userRepo.On("ListByAccountID", ctx, "acc-usd").Return([]*domain.User{}, nil)
	userRepo.On("GetByEmail", ctx, "new@x.com").Return(nil, nil)
	userRepo.On("Create", ctx, mock.Anything).Return(nil)
	userRepo.On("SetResetToken", ctx, mock.AnythingOfType("string"),
		mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)
	userRepo.On("MarkInviteSent", ctx, mock.AnythingOfType("string")).Return(nil)
	emailSvc.On("SendTeamInviteEmail", "new@x.com", "Boss",
		mock.AnythingOfType("string"), "https://app.x.com", "en").Return(nil)

	uc := NewInviteTeamMemberUseCase(userRepo, accountRepo, planRepo, emailSvc, "https://app.x.com")
	_, err := uc.Execute(ctx, InviteTeamMemberDTO{
		AccountID:   "acc-usd",
		InviterName: "Boss",
		Name:        "New Person",
		Email:       "new@x.com",
		Role:        "attendant",
	})

	assert.NoError(t, err)
	emailSvc.AssertExpectations(t)
}

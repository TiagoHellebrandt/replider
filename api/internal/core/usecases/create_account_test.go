package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateAccountUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("should successfully create account, admin user and initial usage", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		emailSvc := new(MockEmailService)
		emailSvc.On("SendVerificationEmail", mock.AnythingOfType("string"), mock.AnythingOfType("string"), "http://localhost:8080", mock.AnythingOfType("string")).Return(nil).Maybe()

		// UseCase instantiation
		uc := NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, emailSvc, "http://localhost:8080")

		// Expectations
		userRepo.On("GetByEmail", ctx, "admin@acme.com").Return(nil, nil)
		accountRepo.On("Create", ctx, mock.MatchedBy(func(a *domain.Account) bool {
			return a.Name == "Acme Corp" && a.SubscriptionStatus == "trialing" && len(a.ID) == 26
		})).Return(nil)

		userRepo.On("Create", ctx, mock.MatchedBy(func(u *domain.User) bool {
			return u.Email == "admin@acme.com" && *u.Role == "owner" && len(u.ID) == 26 && u.PasswordHash != nil && *u.PasswordHash != ""
		})).Return(nil)

		usageRepo.On("Create", ctx, mock.MatchedBy(func(aus *domain.AccountUsage) bool {
			return aus.AccountID != "" && len(aus.ID) == 26 && aus.BillingCycleStart.Day() == 1
		})).Return(nil)

		userRepo.On("SetVerificationToken", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)

		dto := CreateAccountDTO{
			AccountName:   "Acme Corp",
			AdminName:     "Admin",
			AdminEmail:    "admin@acme.com",
			Password:      "securepassword123",
			TermsAccepted: true,
		}

		err := uc.Execute(ctx, dto)
		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
		userRepo.AssertExpectations(t)
		usageRepo.AssertExpectations(t)
	})

	t.Run("should persist UTM attribution and nil-ify blank campaign", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		emailSvc := new(MockEmailService)
		emailSvc.On("SendVerificationEmail", mock.AnythingOfType("string"), mock.AnythingOfType("string"), "http://localhost:8080", mock.AnythingOfType("string")).Return(nil).Maybe()

		uc := NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, emailSvc, "http://localhost:8080")

		userRepo.On("GetByEmail", ctx, "admin@acme.com").Return(nil, nil)
		accountRepo.On("Create", ctx, mock.MatchedBy(func(a *domain.Account) bool {
			return a.UTMSource != nil && *a.UTMSource == "marketing" &&
				a.UTMMedium != nil && *a.UTMMedium == "hero" &&
				a.UTMCampaign == nil // whitespace-only trimmed to NULL
		})).Return(nil)
		userRepo.On("Create", ctx, mock.Anything).Return(nil)
		usageRepo.On("Create", ctx, mock.Anything).Return(nil)
		userRepo.On("SetVerificationToken", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)

		err := uc.Execute(ctx, CreateAccountDTO{
			AccountName:   "Acme Corp",
			AdminName:     "Admin",
			AdminEmail:    "admin@acme.com",
			Password:      "securepassword123",
			TermsAccepted: true,
			UTMSource:     "marketing",
			UTMMedium:     "  hero  ",
			UTMCampaign:   "   ",
		})
		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
	})

	t.Run("should persist signup origin for currency-arbitrage audit", func(t *testing.T) {
		// US-015 (review fixes): the handler stamps IP/country from the
		// request; the account row keeps them (plus the raw locale) so a
		// BR-origin signup that forced an "en" locale is auditable.
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		emailSvc := new(MockEmailService)
		emailSvc.On("SendVerificationEmail", mock.AnythingOfType("string"), mock.AnythingOfType("string"), "http://localhost:8080", mock.AnythingOfType("string")).Return(nil).Maybe()

		uc := NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, emailSvc, "http://localhost:8080")

		userRepo.On("GetByEmail", ctx, "admin@acme.com").Return(nil, nil)
		accountRepo.On("Create", ctx, mock.MatchedBy(func(a *domain.Account) bool {
			return a.Currency == "usd" &&
				a.SignupIP != nil && *a.SignupIP == "203.0.113.7" &&
				a.SignupCountry != nil && *a.SignupCountry == "BR" &&
				a.SignupLocale != nil && *a.SignupLocale == "en-US"
		})).Return(nil)
		userRepo.On("Create", ctx, mock.Anything).Return(nil)
		usageRepo.On("Create", ctx, mock.Anything).Return(nil)
		userRepo.On("SetVerificationToken", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)

		err := uc.Execute(ctx, CreateAccountDTO{
			AccountName:   "Acme Corp",
			AdminName:     "Admin",
			AdminEmail:    "admin@acme.com",
			Password:      "securepassword123",
			TermsAccepted: true,
			Locale:        "en-US",
			SignupIP:      "203.0.113.7",
			SignupCountry: "BR",
		})
		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
	})

	t.Run("should set account currency from signup locale", func(t *testing.T) {
		cases := []struct {
			locale string
			want   string
		}{
			{"en-US", "usd"},
			// A bare "en" is ambiguous (the app's Slang fallback reports
			// it for every non-Portuguese device) — defaults to BRL, it
			// must not force-lock the account into USD (US-017).
			{"en", "brl"},
			{"en-GB", "brl"},
			{"pt-BR", "brl"},
			{"", "brl"}, // legacy clients that don't send locale
		}
		for _, tc := range cases {
			accountRepo := new(MockAccountRepo)
			userRepo := new(MockUserRepo)
			usageRepo := new(MockUsageRepo)
			txManager := new(MockTxManager)
			emailSvc := new(MockEmailService)
			emailSvc.On("SendVerificationEmail", mock.AnythingOfType("string"), mock.AnythingOfType("string"), "http://localhost:8080", mock.AnythingOfType("string")).Return(nil).Maybe()

			uc := NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, emailSvc, "http://localhost:8080")

			userRepo.On("GetByEmail", ctx, "admin@acme.com").Return(nil, nil)
			accountRepo.On("Create", ctx, mock.MatchedBy(func(a *domain.Account) bool {
				return a.Currency == tc.want
			})).Return(nil)
			userRepo.On("Create", ctx, mock.Anything).Return(nil)
			usageRepo.On("Create", ctx, mock.Anything).Return(nil)
			userRepo.On("SetVerificationToken", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)

			err := uc.Execute(ctx, CreateAccountDTO{
				AccountName:   "Acme Corp",
				AdminName:     "Admin",
				AdminEmail:    "admin@acme.com",
				Password:      "securepassword123",
				TermsAccepted: true,
				Locale:        tc.locale,
			})
			assert.NoError(t, err, "locale %q", tc.locale)
			accountRepo.AssertExpectations(t)
		}
	})

	t.Run("should rollback and return error if user creation fails", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)

		userRepo.On("GetByEmail", ctx, "admin@acme.com").Return(nil, nil)
		accountRepo.On("Create", ctx, mock.Anything).Return(nil)
		userRepo.On("Create", ctx, mock.Anything).Return(errors.New("db error"))

		uc := NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, new(MockEmailService), "http://localhost:8080")
		err := uc.Execute(ctx, CreateAccountDTO{
			AccountName:   "Acme Corp",
			AdminName:     "Admin",
			AdminEmail:    "admin@acme.com",
			Password:      "password",
			TermsAccepted: true,
		})

		assert.Error(t, err)
		assert.Equal(t, "db error", err.Error())
	})

	t.Run("should fail if email already exists", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)

		existingUser := &domain.User{Email: "admin@acme.com"}
		userRepo.On("GetByEmail", ctx, "admin@acme.com").Return(existingUser, nil)

		uc := NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, new(MockEmailService), "http://localhost:8080")
		err := uc.Execute(ctx, CreateAccountDTO{
			AccountName:   "Acme Corp",
			AdminName:     "Admin",
			AdminEmail:    "admin@acme.com",
			Password:      "password",
			TermsAccepted: true,
		})

		assert.Error(t, err)
		assert.Equal(t, domain.ErrEmailAlreadyExists, err)
		// Verify Error Code
		var appErr *domain.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, "AUTH_EMAIL_ALREADY_EXISTS", appErr.Code)
	})

	t.Run("should fail when terms_accepted is false (LGPD)", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		// userRepo.GetByEmail is hit BEFORE the terms check (existence is
		// validated first), so we need to stub it returning nil.
		userRepo.On("GetByEmail", ctx, "new@acme.com").Return(nil, nil)

		uc := NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, new(MockEmailService), "http://localhost:8080")
		err := uc.Execute(ctx, CreateAccountDTO{
			AccountName:   "Acme",
			AdminName:     "Admin",
			AdminEmail:    "new@acme.com",
			Password:      "password123",
			TermsAccepted: false,
		})

		assert.Error(t, err)
		var appErr *domain.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, "TERMS_NOT_ACCEPTED", appErr.Code)

		// CRITICAL: nothing was persisted.
		accountRepo.AssertNotCalled(t, "Create")
		userRepo.AssertNotCalled(t, "Create")
		usageRepo.AssertNotCalled(t, "Create")
	})

	t.Run("should record terms_accepted_at + version on success", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		txManager.On("WithinTransaction", ctx, mock.Anything).Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(context.Context) error)
			_ = fn(ctx)
		}).Return(nil)

		userRepo.On("GetByEmail", ctx, "new2@acme.com").Return(nil, nil)
		accountRepo.On("Create", ctx, mock.Anything).Return(nil)

		var captured *domain.User
		userRepo.On("Create", ctx, mock.MatchedBy(func(u *domain.User) bool {
			captured = u
			return true
		})).Return(nil)
		usageRepo.On("Create", ctx, mock.Anything).Return(nil)
		userRepo.On("SetVerificationToken", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		emailService := new(MockEmailService)
		// SendVerificationEmail runs in a background goroutine; mock it as
		// optional so the test doesn't race on assertion order.
		emailService.On("SendVerificationEmail", "new2@acme.com", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		uc := NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, emailService, "http://localhost:8080")
		err := uc.Execute(ctx, CreateAccountDTO{
			AccountName:   "A",
			AdminName:     "Admin",
			AdminEmail:    "new2@acme.com",
			Password:      "password123",
			TermsAccepted: true,
		})
		assert.NoError(t, err)
		assert.NotNil(t, captured)
		assert.NotNil(t, captured.TermsAcceptedAt)
		assert.NotNil(t, captured.TermsVersion)
		assert.Equal(t, domain.CurrentTermsVersion, *captured.TermsVersion)
	})
}

// A new account is born with its default funnel, inside the same
// transaction: the Clientes tab must have somewhere to put customers before
// the person has done anything, and a signup that could not make that place
// rolls back rather than leaving an account without it.
func TestCreateAccountUseCase_ANewAccountGetsItsDefaultFunnel(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	userRepo := new(MockUserRepo)
	usageRepo := new(MockUsageRepo)
	txManager := new(MockTxManager)
	emailSvc := new(MockEmailService)
	funnelRepo := new(MockSalesFunnelRepo)
	emailSvc.On("SendVerificationEmail", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	uc := NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, emailSvc, "http://localhost:8080")
	uc.SetFunnelRepo(funnelRepo)

	userRepo.On("GetByEmail", ctx, "dona@cuiabamed.com.br").Return(nil, nil)
	var createdAccountID string
	accountRepo.On("Create", ctx, mock.MatchedBy(func(a *domain.Account) bool {
		createdAccountID = a.ID
		return true
	})).Return(nil)
	userRepo.On("Create", ctx, mock.Anything).Return(nil)
	userRepo.On("SetVerificationToken", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	usageRepo.On("Create", ctx, mock.Anything).Return(nil)
	funnelRepo.On("EnsureDefault", ctx, mock.MatchedBy(func(id string) bool {
		return id != "" && id == createdAccountID
	})).Return(&domain.SalesFunnel{ID: "f-def", IsDefault: true}, nil)

	err := uc.Execute(ctx, CreateAccountDTO{
		AccountName: "Cuiabamed", AdminName: "Dona", AdminEmail: "dona@cuiabamed.com.br",
		Password: "securepassword123", TermsAccepted: true,
	})

	assert.NoError(t, err)
	funnelRepo.AssertExpectations(t)
}

func TestCreateAccountUseCase_SignupFailsWhenTheDefaultFunnelCannotBeMade(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	userRepo := new(MockUserRepo)
	usageRepo := new(MockUsageRepo)
	txManager := new(MockTxManager)
	emailSvc := new(MockEmailService)
	funnelRepo := new(MockSalesFunnelRepo)
	emailSvc.On("SendVerificationEmail", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	uc := NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, emailSvc, "http://localhost:8080")
	uc.SetFunnelRepo(funnelRepo)

	userRepo.On("GetByEmail", ctx, "dona@cuiabamed.com.br").Return(nil, nil)
	accountRepo.On("Create", ctx, mock.Anything).Return(nil)
	userRepo.On("Create", ctx, mock.Anything).Return(nil)
	userRepo.On("SetVerificationToken", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	usageRepo.On("Create", ctx, mock.Anything).Return(nil)
	funnelRepo.On("EnsureDefault", ctx, mock.Anything).Return(nil, assert.AnError)

	err := uc.Execute(ctx, CreateAccountDTO{
		AccountName: "Cuiabamed", AdminName: "Dona", AdminEmail: "dona@cuiabamed.com.br",
		Password: "securepassword123", TermsAccepted: true,
	})

	assert.ErrorIs(t, err, assert.AnError)
}

// Unwired (self-hosted, older tests): signup behaves exactly as before.
func TestCreateAccountUseCase_NoFunnelRepoIsANoOp(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	userRepo := new(MockUserRepo)
	usageRepo := new(MockUsageRepo)
	txManager := new(MockTxManager)
	emailSvc := new(MockEmailService)
	emailSvc.On("SendVerificationEmail", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	uc := NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, emailSvc, "http://localhost:8080")

	userRepo.On("GetByEmail", ctx, "dona@cuiabamed.com.br").Return(nil, nil)
	accountRepo.On("Create", ctx, mock.Anything).Return(nil)
	userRepo.On("Create", ctx, mock.Anything).Return(nil)
	userRepo.On("SetVerificationToken", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	usageRepo.On("Create", ctx, mock.Anything).Return(nil)

	assert.NoError(t, uc.Execute(ctx, CreateAccountDTO{
		AccountName: "Cuiabamed", AdminName: "Dona", AdminEmail: "dona@cuiabamed.com.br",
		Password: "securepassword123", TermsAccepted: true,
	}))
}

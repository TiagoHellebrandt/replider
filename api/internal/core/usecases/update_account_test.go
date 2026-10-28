package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUpdateAccountUseCase_Execute(t *testing.T) {
	ctx := context.Background()
	adminRole := "admin"
	attendantRole := "attendant"

	newName := "Acme Corp"
	emptyName := "   "

	t.Run("should update account name when admin", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)

		userRepo.On("GetByID", ctx, "u1").Return(&domain.User{
			ID: "u1", AccountID: "a1", Role: &adminRole,
		}, nil)
		accountRepo.On("UpdateName", ctx, "a1", "Acme Corp").Return(nil)

		uc := NewUpdateAccountUseCase(accountRepo, userRepo)
		err := uc.Execute(ctx, "a1", "u1", UpdateAccountDTO{Name: &newName})

		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
		userRepo.AssertExpectations(t)
	})

	t.Run("should trim whitespace from name before persisting", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)

		paddedName := "  Acme  "
		userRepo.On("GetByID", ctx, "u1").Return(&domain.User{
			ID: "u1", AccountID: "a1", Role: &adminRole,
		}, nil)
		accountRepo.On("UpdateName", ctx, "a1", "Acme").Return(nil)

		uc := NewUpdateAccountUseCase(accountRepo, userRepo)
		err := uc.Execute(ctx, "a1", "u1", UpdateAccountDTO{Name: &paddedName})

		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
	})

	t.Run("should reject when Name is nil (no fields to update)", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)

		userRepo.On("GetByID", ctx, "u1").Return(&domain.User{
			ID: "u1", AccountID: "a1", Role: &adminRole,
		}, nil)

		uc := NewUpdateAccountUseCase(accountRepo, userRepo)
		err := uc.Execute(ctx, "a1", "u1", UpdateAccountDTO{Name: nil})

		appErr, ok := err.(*domain.AppError)
		assert.True(t, ok)
		assert.Equal(t, "INVALID_REQUEST", appErr.Code)
		// UpdateName must NOT be called for invalid input
		accountRepo.AssertNotCalled(t, "UpdateName")
	})

	t.Run("should reject when Name is empty/whitespace", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)

		userRepo.On("GetByID", ctx, "u1").Return(&domain.User{
			ID: "u1", AccountID: "a1", Role: &adminRole,
		}, nil)

		uc := NewUpdateAccountUseCase(accountRepo, userRepo)
		err := uc.Execute(ctx, "a1", "u1", UpdateAccountDTO{Name: &emptyName})

		appErr, ok := err.(*domain.AppError)
		assert.True(t, ok)
		assert.Equal(t, "INVALID_REQUEST", appErr.Code)
		accountRepo.AssertNotCalled(t, "UpdateName")
	})

	t.Run("should return ErrForbidden when user is not admin", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)

		userRepo.On("GetByID", ctx, "u1").Return(&domain.User{
			ID: "u1", AccountID: "a1", Role: &attendantRole,
		}, nil)

		uc := NewUpdateAccountUseCase(accountRepo, userRepo)
		err := uc.Execute(ctx, "a1", "u1", UpdateAccountDTO{Name: &newName})

		assert.Equal(t, domain.ErrForbidden, err)
	})

	t.Run("should return ErrForbidden when user belongs to a different account", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)

		userRepo.On("GetByID", ctx, "u1").Return(&domain.User{
			ID: "u1", AccountID: "other-acc", Role: &adminRole,
		}, nil)

		uc := NewUpdateAccountUseCase(accountRepo, userRepo)
		err := uc.Execute(ctx, "a1", "u1", UpdateAccountDTO{Name: &newName})

		assert.Equal(t, domain.ErrForbidden, err)
	})

	t.Run("should return ErrForbidden when user not found", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)

		userRepo.On("GetByID", ctx, "missing").Return(nil, nil)

		uc := NewUpdateAccountUseCase(accountRepo, userRepo)
		err := uc.Execute(ctx, "a1", "missing", UpdateAccountDTO{Name: &newName})

		assert.Equal(t, domain.ErrForbidden, err)
	})

	t.Run("should propagate userRepo error", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)

		userRepo.On("GetByID", ctx, "u1").Return(nil, errors.New("db down"))

		uc := NewUpdateAccountUseCase(accountRepo, userRepo)
		err := uc.Execute(ctx, "a1", "u1", UpdateAccountDTO{Name: &newName})

		assert.EqualError(t, err, "db down")
	})

	t.Run("should propagate accountRepo error", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)

		userRepo.On("GetByID", ctx, "u1").Return(&domain.User{
			ID: "u1", AccountID: "a1", Role: &adminRole,
		}, nil)
		accountRepo.On("UpdateName", ctx, "a1", "Acme Corp").Return(errors.New("constraint violated"))

		uc := NewUpdateAccountUseCase(accountRepo, userRepo)
		err := uc.Execute(ctx, "a1", "u1", UpdateAccountDTO{Name: &newName})

		assert.EqualError(t, err, "constraint violated")
	})

	t.Run("should update branding with both logo and timezone", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)

		userRepo.On("GetByID", ctx, "u1").Return(&domain.User{
			ID: "u1", AccountID: "a1", Role: &adminRole,
		}, nil)
		accountRepo.On("GetByID", ctx, "a1").Return(&domain.Account{
			ID: "a1", Timezone: "America/Sao_Paulo",
		}, nil)
		newLogo := "accounts/a1/logo.png"
		newTZ := "Europe/Lisbon"
		accountRepo.On("UpdateBranding", ctx, "a1", &newLogo, newTZ).Return(nil)

		uc := NewUpdateAccountUseCase(accountRepo, userRepo)
		err := uc.Execute(ctx, "a1", "u1", UpdateAccountDTO{LogoURL: &newLogo, Timezone: &newTZ})

		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
	})

	t.Run("should clear logo when LogoURL is empty string", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)

		existing := "accounts/a1/old.png"
		userRepo.On("GetByID", ctx, "u1").Return(&domain.User{
			ID: "u1", AccountID: "a1", Role: &adminRole,
		}, nil)
		accountRepo.On("GetByID", ctx, "a1").Return(&domain.Account{
			ID: "a1", LogoURL: &existing, Timezone: "America/Sao_Paulo",
		}, nil)
		// Expect nil for LogoURL (clear) and preserved timezone
		accountRepo.On("UpdateBranding", ctx, "a1", (*string)(nil), "America/Sao_Paulo").Return(nil)

		empty := ""
		uc := NewUpdateAccountUseCase(accountRepo, userRepo)
		err := uc.Execute(ctx, "a1", "u1", UpdateAccountDTO{LogoURL: &empty})

		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
	})

	t.Run("should reject invalid IANA timezone", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)

		userRepo.On("GetByID", ctx, "u1").Return(&domain.User{
			ID: "u1", AccountID: "a1", Role: &adminRole,
		}, nil)
		accountRepo.On("GetByID", ctx, "a1").Return(&domain.Account{
			ID: "a1", Timezone: "America/Sao_Paulo",
		}, nil)

		bogus := "Mars/Olympus_Mons"
		uc := NewUpdateAccountUseCase(accountRepo, userRepo)
		err := uc.Execute(ctx, "a1", "u1", UpdateAccountDTO{Timezone: &bogus})

		appErr, ok := err.(*domain.AppError)
		assert.True(t, ok)
		assert.Equal(t, "INVALID_REQUEST", appErr.Code)
		accountRepo.AssertNotCalled(t, "UpdateBranding")
	})

	t.Run("should preserve existing logo when only timezone is provided", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)

		existing := "accounts/a1/logo.png"
		userRepo.On("GetByID", ctx, "u1").Return(&domain.User{
			ID: "u1", AccountID: "a1", Role: &adminRole,
		}, nil)
		accountRepo.On("GetByID", ctx, "a1").Return(&domain.Account{
			ID: "a1", LogoURL: &existing, Timezone: "America/Sao_Paulo",
		}, nil)
		accountRepo.On("UpdateBranding", ctx, "a1", &existing, "Europe/Lisbon").Return(nil)

		tz := "Europe/Lisbon"
		uc := NewUpdateAccountUseCase(accountRepo, userRepo)
		err := uc.Execute(ctx, "a1", "u1", UpdateAccountDTO{Timezone: &tz})

		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
	})
}

// What the company does and where — asked first in onboarding, stored on
// the account, read by every search and every offer downstream.
func TestUpdateAccountUseCase_BusinessProfile(t *testing.T) {
	ctx := context.Background()
	adminRole := "admin"
	admin := &domain.User{ID: "u1", AccountID: "a1", Role: &adminRole}

	t.Run("stores the lines and the territory, normalised", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)
		userRepo.On("GetByID", ctx, "u1").Return(admin, nil)
		accountRepo.On("GetByID", ctx, "a1").Return(&domain.Account{ID: "a1"}, nil)
		accountRepo.On("UpdateBusinessProfile", ctx, "a1",
			[]domain.BusinessLine{domain.BusinessLineService, domain.BusinessLineSell},
			&domain.Territory{UF: "MT", Cities: []string{"Cuiabá", "Várzea Grande"}},
		).Return(nil)

		uc := NewUpdateAccountUseCase(accountRepo, userRepo)
		lines := []domain.BusinessLine{"service", "sell", "service"} // duplicate dropped
		err := uc.Execute(ctx, "a1", "u1", UpdateAccountDTO{
			BusinessLines: &lines,
			Territory:     &domain.Territory{UF: " mt ", Cities: []string{"Cuiabá", "", "Várzea Grande", "cuiabá"}},
		})

		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
	})

	t.Run("refuses a line it does not know", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)
		userRepo.On("GetByID", ctx, "u1").Return(admin, nil)
		accountRepo.On("GetByID", ctx, "a1").Return(&domain.Account{ID: "a1"}, nil)

		uc := NewUpdateAccountUseCase(accountRepo, userRepo)
		lines := []domain.BusinessLine{"consulting"}
		err := uc.Execute(ctx, "a1", "u1", UpdateAccountDTO{BusinessLines: &lines})

		var appErr *domain.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, "INVALID_REQUEST", appErr.Code)
		accountRepo.AssertNotCalled(t, "UpdateBusinessProfile", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("refuses a territory whose state is not two letters", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)
		userRepo.On("GetByID", ctx, "u1").Return(admin, nil)
		accountRepo.On("GetByID", ctx, "a1").Return(&domain.Account{ID: "a1"}, nil)

		uc := NewUpdateAccountUseCase(accountRepo, userRepo)
		err := uc.Execute(ctx, "a1", "u1", UpdateAccountDTO{Territory: &domain.Territory{UF: "Mato Grosso"}})

		var appErr *domain.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, "INVALID_REQUEST", appErr.Code)
	})

	t.Run("touching only the territory keeps the lines already stored", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)
		userRepo.On("GetByID", ctx, "u1").Return(admin, nil)
		accountRepo.On("GetByID", ctx, "a1").Return(&domain.Account{
			ID: "a1", BusinessLines: []domain.BusinessLine{domain.BusinessLineRent},
		}, nil)
		accountRepo.On("UpdateBusinessProfile", ctx, "a1",
			[]domain.BusinessLine{domain.BusinessLineRent},
			&domain.Territory{UF: "MT", Cities: []string{}},
		).Return(nil)

		uc := NewUpdateAccountUseCase(accountRepo, userRepo)
		err := uc.Execute(ctx, "a1", "u1", UpdateAccountDTO{Territory: &domain.Territory{UF: "MT"}})

		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
	})

	t.Run("an empty state clears the territory", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)
		userRepo.On("GetByID", ctx, "u1").Return(admin, nil)
		accountRepo.On("GetByID", ctx, "a1").Return(&domain.Account{
			ID: "a1", Territory: &domain.Territory{UF: "MT"},
		}, nil)
		accountRepo.On("UpdateBusinessProfile", ctx, "a1", []domain.BusinessLine(nil), (*domain.Territory)(nil)).Return(nil)

		uc := NewUpdateAccountUseCase(accountRepo, userRepo)
		err := uc.Execute(ctx, "a1", "u1", UpdateAccountDTO{Territory: &domain.Territory{UF: ""}})

		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
	})
}

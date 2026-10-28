package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

func TestUpdateProfileUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("should update profile successfully", func(t *testing.T) {
		userRepo := new(MockUserRepo)

		userRepo.On("GetByEmail", ctx, "new@example.com").Return(nil, nil)
		userRepo.On("UpdateProfile", ctx, "user1", "New Name", "new@example.com", (*string)(nil)).Return(nil)
		role := "admin"
		updatedUser := &domain.User{
			ID:    "user1",
			Name:  "New Name",
			Email: "new@example.com",
			Role:  &role,
		}
		userRepo.On("GetByID", ctx, "user1").Return(updatedUser, nil)

		uc := NewUpdateProfileUseCase(userRepo)
		result, err := uc.Execute(ctx, UpdateProfileDTO{
			UserID:    "user1",
			AccountID: "acc1",
			Name:      "New Name",
			Email:     "new@example.com",
		})

		assert.NoError(t, err)
		assert.Equal(t, "New Name", result.Name)
		assert.Equal(t, "new@example.com", result.Email)
		userRepo.AssertExpectations(t)
	})

	t.Run("should pass signature through to the repo when provided", func(t *testing.T) {
		userRepo := new(MockUserRepo)

		sig := "Abraços,\nTiago"
		userRepo.On("GetByEmail", ctx, "new@example.com").Return(nil, nil)
		userRepo.On("UpdateProfile", ctx, "user1", "New Name", "new@example.com", &sig).Return(nil)
		userRepo.On("GetByID", ctx, "user1").Return(&domain.User{ID: "user1", Signature: &sig}, nil)

		uc := NewUpdateProfileUseCase(userRepo)
		result, err := uc.Execute(ctx, UpdateProfileDTO{
			UserID:    "user1",
			AccountID: "acc1",
			Name:      "New Name",
			Email:     "new@example.com",
			Signature: &sig,
		})

		assert.NoError(t, err)
		assert.Equal(t, sig, *result.Signature)
		userRepo.AssertExpectations(t)
	})

	t.Run("should reject signature over 280 chars", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		long := make([]byte, 281)
		for i := range long {
			long[i] = 'a'
		}
		sig := string(long)

		uc := NewUpdateProfileUseCase(userRepo)
		_, err := uc.Execute(ctx, UpdateProfileDTO{
			UserID: "user1", AccountID: "acc1",
			Name: "N", Email: "n@example.com", Signature: &sig,
		})

		appErr, ok := err.(*domain.AppError)
		assert.True(t, ok)
		assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
		userRepo.AssertNotCalled(t, "UpdateProfile")
	})

	t.Run("should return ErrEmailAlreadyExists when email is taken by another user", func(t *testing.T) {
		userRepo := new(MockUserRepo)

		existingUser := &domain.User{ID: "other-user", Email: "taken@example.com"}
		userRepo.On("GetByEmail", ctx, "taken@example.com").Return(existingUser, nil)

		uc := NewUpdateProfileUseCase(userRepo)
		result, err := uc.Execute(ctx, UpdateProfileDTO{
			UserID:    "user1",
			AccountID: "acc1",
			Name:      "Name",
			Email:     "taken@example.com",
		})

		assert.Nil(t, result)
		assert.Equal(t, domain.ErrEmailAlreadyExists, err)
	})

	t.Run("should allow keeping same email", func(t *testing.T) {
		userRepo := new(MockUserRepo)

		// GetByEmail returns the same user (same ID)
		sameUser := &domain.User{ID: "user1", Email: "same@example.com"}
		userRepo.On("GetByEmail", ctx, "same@example.com").Return(sameUser, nil)
		userRepo.On("UpdateProfile", ctx, "user1", "Updated Name", "same@example.com", (*string)(nil)).Return(nil)
		role := "admin"
		userRepo.On("GetByID", ctx, "user1").Return(&domain.User{
			ID: "user1", Name: "Updated Name", Email: "same@example.com", Role: &role,
		}, nil)

		uc := NewUpdateProfileUseCase(userRepo)
		result, err := uc.Execute(ctx, UpdateProfileDTO{
			UserID:    "user1",
			AccountID: "acc1",
			Name:      "Updated Name",
			Email:     "same@example.com",
		})

		assert.NoError(t, err)
		assert.Equal(t, "Updated Name", result.Name)
		userRepo.AssertExpectations(t)
	})

	t.Run("should return validation error when name is empty", func(t *testing.T) {
		userRepo := new(MockUserRepo)

		uc := NewUpdateProfileUseCase(userRepo)
		result, err := uc.Execute(ctx, UpdateProfileDTO{
			UserID:    "user1",
			AccountID: "acc1",
			Name:      "",
			Email:     "test@example.com",
		})

		assert.Nil(t, result)
		assert.Error(t, err)
		var appErr *domain.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
	})
}

func TestChangePasswordUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("current-password"), bcrypt.DefaultCost)
	hashStr := string(hashedPassword)

	t.Run("should change password successfully", func(t *testing.T) {
		userRepo := new(MockUserRepo)

		user := &domain.User{
			ID:           "user1",
			PasswordHash: &hashStr,
		}
		userRepo.On("GetByID", ctx, "user1").Return(user, nil)
		userRepo.On("UpdatePasswordHash", ctx, "user1", mock.AnythingOfType("string")).Return(nil)

		uc := NewChangePasswordUseCase(userRepo)
		err := uc.Execute(ctx, ChangePasswordDTO{
			UserID:          "user1",
			CurrentPassword: "current-password",
			NewPassword:     "new-secure-password",
		})

		assert.NoError(t, err)
		userRepo.AssertExpectations(t)
	})

	t.Run("should return ErrInvalidCredentials for wrong current password", func(t *testing.T) {
		userRepo := new(MockUserRepo)

		user := &domain.User{
			ID:           "user1",
			PasswordHash: &hashStr,
		}
		userRepo.On("GetByID", ctx, "user1").Return(user, nil)

		uc := NewChangePasswordUseCase(userRepo)
		err := uc.Execute(ctx, ChangePasswordDTO{
			UserID:          "user1",
			CurrentPassword: "wrong-password",
			NewPassword:     "new-password",
		})

		assert.Equal(t, domain.ErrInvalidCredentials, err)
	})

	t.Run("should return ErrUnauthorized when user not found", func(t *testing.T) {
		userRepo := new(MockUserRepo)

		userRepo.On("GetByID", ctx, "nonexistent").Return(nil, nil)

		uc := NewChangePasswordUseCase(userRepo)
		err := uc.Execute(ctx, ChangePasswordDTO{
			UserID:          "nonexistent",
			CurrentPassword: "any",
			NewPassword:     "new",
		})

		assert.Equal(t, domain.ErrUnauthorized, err)
	})
}

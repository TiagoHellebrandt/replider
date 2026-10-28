package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func TestDeleteAccountUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("should delete account when user is admin of the account", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)

		role := "admin"
		user := &domain.User{
			ID:        "user1",
			AccountID: "acc1",
			Role:      &role,
		}
		userRepo.On("GetByID", ctx, "user1").Return(user, nil)
		accountRepo.On("Delete", ctx, "acc1").Return(nil)

		uc := NewDeleteAccountUseCase(accountRepo, userRepo)
		err := uc.Execute(ctx, "acc1", "user1")

		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
		userRepo.AssertExpectations(t)
	})

	t.Run("should return ErrForbidden when user is not admin", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)

		role := "attendant"
		user := &domain.User{
			ID:        "user1",
			AccountID: "acc1",
			Role:      &role,
		}
		userRepo.On("GetByID", ctx, "user1").Return(user, nil)

		uc := NewDeleteAccountUseCase(accountRepo, userRepo)
		err := uc.Execute(ctx, "acc1", "user1")

		assert.Equal(t, domain.ErrForbidden, err)
	})

	t.Run("should return ErrForbidden when user belongs to different account", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)

		role := "admin"
		user := &domain.User{
			ID:        "user1",
			AccountID: "other-acc",
			Role:      &role,
		}
		userRepo.On("GetByID", ctx, "user1").Return(user, nil)

		uc := NewDeleteAccountUseCase(accountRepo, userRepo)
		err := uc.Execute(ctx, "acc1", "user1")

		assert.Equal(t, domain.ErrForbidden, err)
	})

	t.Run("should return ErrForbidden when user not found", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)

		userRepo.On("GetByID", ctx, "bad-user").Return(nil, nil)

		uc := NewDeleteAccountUseCase(accountRepo, userRepo)
		err := uc.Execute(ctx, "acc1", "bad-user")

		assert.Equal(t, domain.ErrForbidden, err)
	})

	t.Run("should return ErrForbidden when role is nil", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)

		user := &domain.User{
			ID:        "user1",
			AccountID: "acc1",
			Role:      nil,
		}
		userRepo.On("GetByID", ctx, "user1").Return(user, nil)

		uc := NewDeleteAccountUseCase(accountRepo, userRepo)
		err := uc.Execute(ctx, "acc1", "user1")

		assert.Equal(t, domain.ErrForbidden, err)
	})
}

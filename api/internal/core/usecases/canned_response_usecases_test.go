package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockCannedResponseRepo struct{ mock.Mock }

func (m *MockCannedResponseRepo) Create(ctx context.Context, cr *domain.CannedResponse) error {
	return m.Called(ctx, cr).Error(0)
}
func (m *MockCannedResponseRepo) ListVisible(ctx context.Context, accountID, userID string) ([]*domain.CannedResponse, error) {
	args := m.Called(ctx, accountID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.CannedResponse), args.Error(1)
}
func (m *MockCannedResponseRepo) GetByID(ctx context.Context, id string) (*domain.CannedResponse, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.CannedResponse), args.Error(1)
}
func (m *MockCannedResponseRepo) GetVisibleByShortcut(ctx context.Context, accountID, userID, shortcut string) (*domain.CannedResponse, error) {
	args := m.Called(ctx, accountID, userID, shortcut)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.CannedResponse), args.Error(1)
}
func (m *MockCannedResponseRepo) Update(ctx context.Context, cr *domain.CannedResponse) error {
	return m.Called(ctx, cr).Error(0)
}
func (m *MockCannedResponseRepo) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

func TestCannedResponseUseCase(t *testing.T) {
	ctx := context.Background()

	t.Run("Create account response as admin succeeds", func(t *testing.T) {
		repo := new(MockCannedResponseRepo)
		uc := NewCannedResponseUseCase(repo)

		repo.On("GetVisibleByShortcut", ctx, "acc-1", "user-1", "/hello").Return(nil, nil)
		repo.On("Create", ctx, mock.MatchedBy(func(cr *domain.CannedResponse) bool {
			return cr.Shortcut == "/hello" && cr.Visibility == domain.CannedVisibilityAccount &&
				cr.OwnerUserID == nil && len(cr.ID) == 26
		})).Return(nil)

		cr, err := uc.Create(ctx, CreateCannedResponseDTO{
			AccountID: "acc-1",
			UserID:    "user-1",
			Role:      "admin",
			Shortcut:  "/hello",
			Title:     "Greeting",
			Content:   "Hello! How can I help?",
			// Visibility empty → defaults to 'account' (legacy clients)
		})

		assert.NoError(t, err)
		assert.NotNil(t, cr)
		assert.Equal(t, "general", cr.Category)
		assert.Equal(t, domain.CannedVisibilityAccount, cr.Visibility)
		repo.AssertExpectations(t)
	})

	t.Run("Create account response as attendant is forbidden", func(t *testing.T) {
		repo := new(MockCannedResponseRepo)
		uc := NewCannedResponseUseCase(repo)

		_, err := uc.Create(ctx, CreateCannedResponseDTO{
			AccountID: "acc-1",
			UserID:    "user-1",
			Role:      "attendant",
			Shortcut:  "/hello",
			Title:     "Greeting",
			Content:   "Hello!",
		})

		assert.Equal(t, domain.ErrCannedResponseForbidden, err)
		repo.AssertNotCalled(t, "Create")
	})

	t.Run("Create personal response as attendant stamps owner", func(t *testing.T) {
		repo := new(MockCannedResponseRepo)
		uc := NewCannedResponseUseCase(repo)

		repo.On("GetVisibleByShortcut", ctx, "acc-1", "user-1", "/mine").Return(nil, nil)
		repo.On("Create", ctx, mock.MatchedBy(func(cr *domain.CannedResponse) bool {
			return cr.Visibility == domain.CannedVisibilityPersonal &&
				cr.OwnerUserID != nil && *cr.OwnerUserID == "user-1"
		})).Return(nil)

		cr, err := uc.Create(ctx, CreateCannedResponseDTO{
			AccountID:  "acc-1",
			UserID:     "user-1",
			Role:       "attendant",
			Shortcut:   "/mine",
			Title:      "Mine",
			Content:    "my text",
			Visibility: "personal",
		})

		assert.NoError(t, err)
		assert.Equal(t, "user-1", *cr.OwnerUserID)
		repo.AssertExpectations(t)
	})

	t.Run("Create with invalid visibility fails", func(t *testing.T) {
		repo := new(MockCannedResponseRepo)
		uc := NewCannedResponseUseCase(repo)

		_, err := uc.Create(ctx, CreateCannedResponseDTO{
			AccountID:  "acc-1",
			UserID:     "user-1",
			Role:       "admin",
			Shortcut:   "/x",
			Title:      "X",
			Content:    "x",
			Visibility: "team",
		})

		appErr, ok := err.(*domain.AppError)
		assert.True(t, ok)
		assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
	})

	t.Run("Create should fail with duplicate shortcut in visible set", func(t *testing.T) {
		repo := new(MockCannedResponseRepo)
		uc := NewCannedResponseUseCase(repo)

		existing := &domain.CannedResponse{ID: "cr-1", Shortcut: "/hello"}
		repo.On("GetVisibleByShortcut", ctx, "acc-1", "user-1", "/hello").Return(existing, nil)

		_, err := uc.Create(ctx, CreateCannedResponseDTO{
			AccountID:  "acc-1",
			UserID:     "user-1",
			Role:       "attendant",
			Shortcut:   "/hello",
			Title:      "Greeting",
			Content:    "Hello!",
			Visibility: "personal",
		})

		assert.Equal(t, domain.ErrCannedResponseShortcutExists, err)
	})

	t.Run("Update account response as attendant is forbidden", func(t *testing.T) {
		repo := new(MockCannedResponseRepo)
		uc := NewCannedResponseUseCase(repo)

		cr := &domain.CannedResponse{ID: "cr-1", AccountID: "acc-1", Visibility: domain.CannedVisibilityAccount, Shortcut: "/hello"}
		repo.On("GetByID", ctx, "cr-1").Return(cr, nil)

		_, err := uc.Update(ctx, UpdateCannedResponseDTO{
			AccountID: "acc-1", UserID: "user-1", Role: "attendant",
			ID: "cr-1", Shortcut: "/hello", Title: "T", Content: "C",
		})

		assert.Equal(t, domain.ErrCannedResponseForbidden, err)
		repo.AssertNotCalled(t, "Update")
	})

	t.Run("Update own personal response as attendant succeeds", func(t *testing.T) {
		repo := new(MockCannedResponseRepo)
		uc := NewCannedResponseUseCase(repo)

		cr := &domain.CannedResponse{
			ID: "cr-1", AccountID: "acc-1", Shortcut: "/mine",
			Visibility: domain.CannedVisibilityPersonal, OwnerUserID: strPtr("user-1"),
		}
		repo.On("GetByID", ctx, "cr-1").Return(cr, nil)
		repo.On("Update", ctx, mock.MatchedBy(func(u *domain.CannedResponse) bool {
			return u.ID == "cr-1" && u.Title == "New title"
		})).Return(nil)

		out, err := uc.Update(ctx, UpdateCannedResponseDTO{
			AccountID: "acc-1", UserID: "user-1", Role: "attendant",
			ID: "cr-1", Shortcut: "/mine", Title: "New title", Content: "C", Category: "general",
		})

		assert.NoError(t, err)
		assert.Equal(t, "New title", out.Title)
		repo.AssertExpectations(t)
	})

	t.Run("Update foreign personal response returns not found even for admin", func(t *testing.T) {
		repo := new(MockCannedResponseRepo)
		uc := NewCannedResponseUseCase(repo)

		cr := &domain.CannedResponse{
			ID: "cr-1", AccountID: "acc-1", Shortcut: "/theirs",
			Visibility: domain.CannedVisibilityPersonal, OwnerUserID: strPtr("user-2"),
		}
		repo.On("GetByID", ctx, "cr-1").Return(cr, nil)

		_, err := uc.Update(ctx, UpdateCannedResponseDTO{
			AccountID: "acc-1", UserID: "user-1", Role: "admin",
			ID: "cr-1", Shortcut: "/theirs", Title: "T", Content: "C",
		})

		// NotFound, not Forbidden — must not leak someone else's private rows
		assert.Equal(t, domain.ErrCannedResponseNotFound, err)
		repo.AssertNotCalled(t, "Update")
	})

	t.Run("Delete should fail for wrong account", func(t *testing.T) {
		repo := new(MockCannedResponseRepo)
		uc := NewCannedResponseUseCase(repo)

		cr := &domain.CannedResponse{ID: "cr-1", AccountID: "acc-other", Visibility: domain.CannedVisibilityAccount}
		repo.On("GetByID", ctx, "cr-1").Return(cr, nil)

		err := uc.Delete(ctx, "acc-1", "user-1", "admin", "cr-1")

		assert.Equal(t, domain.ErrCannedResponseNotFound, err)
	})

	t.Run("Delete own personal response as attendant succeeds", func(t *testing.T) {
		repo := new(MockCannedResponseRepo)
		uc := NewCannedResponseUseCase(repo)

		cr := &domain.CannedResponse{
			ID: "cr-1", AccountID: "acc-1",
			Visibility: domain.CannedVisibilityPersonal, OwnerUserID: strPtr("user-1"),
		}
		repo.On("GetByID", ctx, "cr-1").Return(cr, nil)
		repo.On("Delete", ctx, "cr-1").Return(nil)

		err := uc.Delete(ctx, "acc-1", "user-1", "attendant", "cr-1")

		assert.NoError(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("Delete account response as attendant is forbidden", func(t *testing.T) {
		repo := new(MockCannedResponseRepo)
		uc := NewCannedResponseUseCase(repo)

		cr := &domain.CannedResponse{ID: "cr-1", AccountID: "acc-1", Visibility: domain.CannedVisibilityAccount}
		repo.On("GetByID", ctx, "cr-1").Return(cr, nil)

		err := uc.Delete(ctx, "acc-1", "user-1", "attendant", "cr-1")

		assert.Equal(t, domain.ErrCannedResponseForbidden, err)
		repo.AssertNotCalled(t, "Delete")
	})

	t.Run("List returns visible set for the caller", func(t *testing.T) {
		repo := new(MockCannedResponseRepo)
		uc := NewCannedResponseUseCase(repo)

		responses := []*domain.CannedResponse{
			{ID: "cr-1", Shortcut: "/hello", Visibility: domain.CannedVisibilityAccount},
			{ID: "cr-2", Shortcut: "/mine", Visibility: domain.CannedVisibilityPersonal, OwnerUserID: strPtr("user-1")},
		}
		repo.On("ListVisible", ctx, "acc-1", "user-1").Return(responses, nil)

		result, err := uc.List(ctx, "acc-1", "user-1")

		assert.NoError(t, err)
		assert.Len(t, result, 2)
	})
}

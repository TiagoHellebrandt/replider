package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockTagRepo struct{ mock.Mock }

func (m *MockTagRepo) Create(ctx context.Context, tag *domain.Tag) error {
	return m.Called(ctx, tag).Error(0)
}
func (m *MockTagRepo) ListByAccountID(ctx context.Context, accountID string) ([]*domain.Tag, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Tag), args.Error(1)
}
func (m *MockTagRepo) GetByID(ctx context.Context, id string) (*domain.Tag, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tag), args.Error(1)
}
func (m *MockTagRepo) GetByAccountAndName(ctx context.Context, accountID, name string) (*domain.Tag, error) {
	args := m.Called(ctx, accountID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tag), args.Error(1)
}
func (m *MockTagRepo) Update(ctx context.Context, tag *domain.Tag) error {
	return m.Called(ctx, tag).Error(0)
}
func (m *MockTagRepo) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockTagRepo) AddToConversation(ctx context.Context, conversationID, tagID string) error {
	return m.Called(ctx, conversationID, tagID).Error(0)
}
func (m *MockTagRepo) RemoveFromConversation(ctx context.Context, conversationID, tagID string) error {
	return m.Called(ctx, conversationID, tagID).Error(0)
}
func (m *MockTagRepo) ListByConversation(ctx context.Context, accountID, conversationID string) ([]*domain.Tag, error) {
	args := m.Called(ctx, accountID, conversationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Tag), args.Error(1)
}

func TestTagUseCase(t *testing.T) {
	ctx := context.Background()

	t.Run("Create tag with default color", func(t *testing.T) {
		tagRepo := new(MockTagRepo)
		convRepo := new(MockConversationRepo)
		uc := NewTagUseCase(tagRepo, convRepo)

		tagRepo.On("Create", ctx, mock.MatchedBy(func(tag *domain.Tag) bool {
			return tag.Name == "VIP" && tag.Color == "#6366f1" && len(tag.ID) == 26
		})).Return(nil)

		tag, err := uc.Create(ctx, CreateTagDTO{AccountID: "acc-1", Name: "VIP"})

		assert.NoError(t, err)
		assert.Equal(t, "VIP", tag.Name)
		assert.Equal(t, "#6366f1", tag.Color)
		tagRepo.AssertExpectations(t)
	})

	t.Run("Create tag with custom color", func(t *testing.T) {
		tagRepo := new(MockTagRepo)
		convRepo := new(MockConversationRepo)
		uc := NewTagUseCase(tagRepo, convRepo)

		tagRepo.On("Create", ctx, mock.MatchedBy(func(tag *domain.Tag) bool {
			return tag.Color == "#ff0000"
		})).Return(nil)

		tag, err := uc.Create(ctx, CreateTagDTO{AccountID: "acc-1", Name: "Urgent", Color: "#ff0000"})

		assert.NoError(t, err)
		assert.Equal(t, "#ff0000", tag.Color)
	})

	t.Run("Delete tag fails for wrong account", func(t *testing.T) {
		tagRepo := new(MockTagRepo)
		convRepo := new(MockConversationRepo)
		uc := NewTagUseCase(tagRepo, convRepo)

		tagRepo.On("GetByID", ctx, "tag-1").Return(&domain.Tag{ID: "tag-1", AccountID: "acc-other"}, nil)

		err := uc.Delete(ctx, "acc-1", "tag-1")

		assert.Equal(t, domain.ErrUnauthorized, err)
	})

	t.Run("AddToConversation verifies ownership", func(t *testing.T) {
		tagRepo := new(MockTagRepo)
		convRepo := new(MockConversationRepo)
		uc := NewTagUseCase(tagRepo, convRepo)

		convRepo.On("GetByID", ctx, "conv-1", "acc-1").Return(&domain.Conversation{ID: "conv-1", AccountID: "acc-1"}, nil)
		tagRepo.On("GetByID", ctx, "tag-1").Return(&domain.Tag{ID: "tag-1", AccountID: "acc-1"}, nil)
		tagRepo.On("AddToConversation", ctx, "conv-1", "tag-1").Return(nil)

		err := uc.AddToConversation(ctx, "acc-1", "conv-1", "tag-1")

		assert.NoError(t, err)
		tagRepo.AssertExpectations(t)
		convRepo.AssertExpectations(t)
	})

	t.Run("AddToConversation fails for tag from different account", func(t *testing.T) {
		tagRepo := new(MockTagRepo)
		convRepo := new(MockConversationRepo)
		uc := NewTagUseCase(tagRepo, convRepo)

		convRepo.On("GetByID", ctx, "conv-1", "acc-1").Return(&domain.Conversation{ID: "conv-1", AccountID: "acc-1"}, nil)
		tagRepo.On("GetByID", ctx, "tag-1").Return(&domain.Tag{ID: "tag-1", AccountID: "acc-other"}, nil)

		err := uc.AddToConversation(ctx, "acc-1", "conv-1", "tag-1")

		assert.Equal(t, domain.ErrUnauthorized, err)
	})

	t.Run("ListByConversation scopes to account", func(t *testing.T) {
		tagRepo := new(MockTagRepo)
		convRepo := new(MockConversationRepo)
		uc := NewTagUseCase(tagRepo, convRepo)

		convRepo.On("GetByID", ctx, "conv-1", "acc-1").Return(&domain.Conversation{ID: "conv-1", AccountID: "acc-1"}, nil)
		tagRepo.On("ListByConversation", ctx, "acc-1", "conv-1").Return([]*domain.Tag{{ID: "tag-1", AccountID: "acc-1", Name: "VIP"}}, nil)

		tags, err := uc.ListByConversation(ctx, "acc-1", "conv-1")

		assert.NoError(t, err)
		assert.Len(t, tags, 1)
		assert.Equal(t, "tag-1", tags[0].ID)
		tagRepo.AssertExpectations(t)
		convRepo.AssertExpectations(t)
	})

	t.Run("ListByConversation returns ErrConversationNotFound for foreign conversation", func(t *testing.T) {
		tagRepo := new(MockTagRepo)
		convRepo := new(MockConversationRepo)
		uc := NewTagUseCase(tagRepo, convRepo)

		// Conversation belongs to another account; GetByID with the caller's
		// accountID returns nil so no rows match. The repo's
		// ListByConversation must never be invoked.
		convRepo.On("GetByID", ctx, "conv-foreign", "acc-1").Return(nil, nil)

		tags, err := uc.ListByConversation(ctx, "acc-1", "conv-foreign")

		assert.Nil(t, tags)
		assert.Equal(t, domain.ErrConversationNotFound, err)
		tagRepo.AssertNotCalled(t, "ListByConversation", mock.Anything, mock.Anything, mock.Anything)
	})
}

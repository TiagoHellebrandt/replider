package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mock ---

type MockContactRepo struct{ mock.Mock }

func (m *MockContactRepo) Upsert(ctx context.Context, c *domain.Contact) (*domain.Contact, error) {
	args := m.Called(ctx, c)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Contact), args.Error(1)
}
func (m *MockContactRepo) ListByIDs(ctx context.Context, accountID string, ids []string) ([]*domain.Contact, error) {
	args := m.Called(ctx, accountID, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Contact), args.Error(1)
}

func (m *MockContactRepo) GetByID(ctx context.Context, id, accountID string) (*domain.Contact, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Contact), args.Error(1)
}
func (m *MockContactRepo) GetByExternalID(ctx context.Context, accountID, externalID string) (*domain.Contact, error) {
	args := m.Called(ctx, accountID, externalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Contact), args.Error(1)
}
func (m *MockContactRepo) ListByAccountID(ctx context.Context, accountID, query string, limit, offset int) ([]*domain.Contact, error) {
	args := m.Called(ctx, accountID, query, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Contact), args.Error(1)
}
func (m *MockContactRepo) Update(ctx context.Context, c *domain.Contact) error {
	return m.Called(ctx, c).Error(0)
}
func (m *MockContactRepo) SetOptedOut(ctx context.Context, accountID, externalID string) (bool, error) {
	args := m.Called(ctx, accountID, externalID)
	return args.Bool(0), args.Error(1)
}
func (m *MockContactRepo) ClearOptedOut(ctx context.Context, accountID, externalID string) (bool, error) {
	args := m.Called(ctx, accountID, externalID)
	return args.Bool(0), args.Error(1)
}
func (m *MockContactRepo) ListOptedOutExternalIDs(ctx context.Context, accountID string, externalIDs []string) (map[string]bool, error) {
	args := m.Called(ctx, accountID, externalIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]bool), args.Error(1)
}
func (m *MockContactRepo) IsOptedOut(ctx context.Context, accountID, externalID string) (bool, error) {
	args := m.Called(ctx, accountID, externalID)
	return args.Bool(0), args.Error(1)
}

// --- Tests ---

func TestListContactsUseCase(t *testing.T) {
	ctx := context.Background()
	repo := new(MockContactRepo)
	uc := NewListContactsUseCase(repo)

	contacts := []*domain.Contact{{ID: "c1", AccountID: "acc1"}}
	repo.On("ListByAccountID", ctx, "acc1", "", 50, 0).Return(contacts, nil)

	result, err := uc.Execute(ctx, "acc1", "", 50, 0)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	repo.AssertExpectations(t)
}

func TestListContactsUseCase_DefaultLimit(t *testing.T) {
	ctx := context.Background()
	repo := new(MockContactRepo)
	uc := NewListContactsUseCase(repo)

	repo.On("ListByAccountID", ctx, "acc1", "", 50, 0).Return([]*domain.Contact{}, nil)

	_, err := uc.Execute(ctx, "acc1", "", 0, 0)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestGetContactUseCase(t *testing.T) {
	ctx := context.Background()
	repo := new(MockContactRepo)
	uc := NewGetContactUseCase(repo)

	contact := &domain.Contact{ID: "c1", AccountID: "acc1"}
	repo.On("GetByID", ctx, "c1", "acc1").Return(contact, nil)

	result, err := uc.Execute(ctx, "c1", "acc1")
	assert.NoError(t, err)
	assert.Equal(t, "c1", result.ID)
}

func TestGetContactUseCase_WrongAccount(t *testing.T) {
	ctx := context.Background()
	repo := new(MockContactRepo)
	uc := NewGetContactUseCase(repo)

	// The repository scopes by accountID in SQL, so a contact owned by
	// another account comes back as (nil, nil) — the usecase must map
	// that to ErrContactNotFound, and must call the repo with the
	// caller's accountID (not the contact's true owner).
	repo.On("GetByID", ctx, "c1", "acc_other").Return(nil, nil)

	_, err := uc.Execute(ctx, "c1", "acc_other")
	assert.Equal(t, domain.ErrContactNotFound, err)
	repo.AssertCalled(t, "GetByID", ctx, "c1", "acc_other")
}

func TestUpdateContactUseCase(t *testing.T) {
	ctx := context.Background()
	repo := new(MockContactRepo)
	uc := NewUpdateContactUseCase(repo)

	name := "Updated Name"
	contact := &domain.Contact{ID: "c1", AccountID: "acc1"}
	repo.On("GetByID", ctx, "c1", "acc1").Return(contact, nil)
	repo.On("Update", ctx, mock.AnythingOfType("*domain.Contact")).Return(nil)

	result, err := uc.Execute(ctx, "c1", "acc1", UpdateContactDTO{Name: &name})
	assert.NoError(t, err)
	assert.Equal(t, "Updated Name", *result.Name)
	repo.AssertExpectations(t)
}

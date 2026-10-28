package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockProductRepo struct{ mock.Mock }

func (m *MockProductRepo) Create(ctx context.Context, p *domain.Product) error {
	return m.Called(ctx, p).Error(0)
}

func (m *MockProductRepo) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Product), args.Error(1)
}

func (m *MockProductRepo) ListByAccountID(ctx context.Context, accountID string) ([]*domain.Product, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Product), args.Error(1)
}

func (m *MockProductRepo) Update(ctx context.Context, p *domain.Product) error {
	return m.Called(ctx, p).Error(0)
}

func (m *MockProductRepo) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

func TestProductUseCase_CreateDefaultsAndPersists(t *testing.T) {
	ctx := context.Background()
	repo := new(MockProductRepo)
	uc := NewProductUseCase(repo)

	repo.On("Create", ctx, mock.MatchedBy(func(p *domain.Product) bool {
		return p.AccountID == "acc-1" &&
			p.Name == "Plano Pro" &&
			p.PriceCents == 9900 &&
			p.Currency == "BRL" && // defaulted
			p.Active && // defaulted to true
			len(p.ID) == 26 &&
			string(p.Attributes) == "{}" // normalized
	})).Return(nil)

	p, err := uc.Create(ctx, CreateProductDTO{
		AccountID:  "acc-1",
		Name:       "Plano Pro",
		PriceCents: 9900,
	})

	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, "BRL", p.Currency)
	assert.True(t, p.Active)
	repo.AssertExpectations(t)
}

func TestProductUseCase_GetDeniedForOtherAccount(t *testing.T) {
	ctx := context.Background()
	repo := new(MockProductRepo)
	uc := NewProductUseCase(repo)

	repo.On("GetByID", ctx, "prod-1").
		Return(&domain.Product{ID: "prod-1", AccountID: "acc-other"}, nil)

	_, err := uc.Get(ctx, "acc-1", "prod-1")
	assert.Equal(t, domain.ErrUnauthorized, err)
}

func TestProductUseCase_GetNotFound(t *testing.T) {
	ctx := context.Background()
	repo := new(MockProductRepo)
	uc := NewProductUseCase(repo)

	repo.On("GetByID", ctx, "missing").Return(nil, nil)

	_, err := uc.Get(ctx, "acc-1", "missing")
	assert.Equal(t, domain.ErrProductNotFound, err)
}

func TestProductUseCase_UpdateDeniedForOtherAccount(t *testing.T) {
	ctx := context.Background()
	repo := new(MockProductRepo)
	uc := NewProductUseCase(repo)

	repo.On("GetByID", ctx, "prod-1").
		Return(&domain.Product{ID: "prod-1", AccountID: "acc-other"}, nil)

	_, err := uc.Update(ctx, UpdateProductDTO{AccountID: "acc-1", ID: "prod-1", Name: "x"})
	assert.Equal(t, domain.ErrUnauthorized, err)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

// The product form never sends attributes, and the update used to replace
// them with "{}" on every rename — silently erasing the sku/brand/model a PDF
// import extracted, the one copy of those facts.
func TestProductUseCase_UpdateWithoutAttributesPreservesStoredOnes(t *testing.T) {
	ctx := context.Background()
	repo := new(MockProductRepo)
	uc := NewProductUseCase(repo)

	stored := []byte(`{"sku":"ABC-1","brand":"GE"}`)
	repo.On("GetByID", ctx, "prod-1").Return(&domain.Product{
		ID: "prod-1", AccountID: "acc-1", Attributes: stored,
	}, nil)
	repo.On("Update", ctx, mock.MatchedBy(func(p *domain.Product) bool {
		return string(p.Attributes) == string(stored)
	})).Return(nil)

	p, err := uc.Update(ctx, UpdateProductDTO{
		AccountID: "acc-1", ID: "prod-1", Name: "Novo nome", PriceCents: 100,
	})
	assert.NoError(t, err)
	assert.Equal(t, string(stored), string(p.Attributes))
	repo.AssertExpectations(t)
}

// A non-empty object still replaces: an explicit write wins.
func TestProductUseCase_UpdateWithAttributesReplaces(t *testing.T) {
	ctx := context.Background()
	repo := new(MockProductRepo)
	uc := NewProductUseCase(repo)

	repo.On("GetByID", ctx, "prod-1").Return(&domain.Product{
		ID: "prod-1", AccountID: "acc-1", Attributes: []byte(`{"sku":"OLD"}`),
	}, nil)
	repo.On("Update", ctx, mock.MatchedBy(func(p *domain.Product) bool {
		return string(p.Attributes) == `{"sku":"NEW"}`
	})).Return(nil)

	_, err := uc.Update(ctx, UpdateProductDTO{
		AccountID: "acc-1", ID: "prod-1", Name: "n", PriceCents: 100,
		Attributes: []byte(`{"sku":"NEW"}`),
	})
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestProductUseCase_DeleteDeniedForOtherAccount(t *testing.T) {
	ctx := context.Background()
	repo := new(MockProductRepo)
	uc := NewProductUseCase(repo)

	repo.On("GetByID", ctx, "prod-1").
		Return(&domain.Product{ID: "prod-1", AccountID: "acc-other"}, nil)

	err := uc.Delete(ctx, "acc-1", "prod-1")
	assert.Equal(t, domain.ErrUnauthorized, err)
	repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

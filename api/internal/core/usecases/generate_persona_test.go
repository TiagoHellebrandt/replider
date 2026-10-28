package usecases

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGeneratePersona_FromProduct_Success(t *testing.T) {
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	productRepo := new(MockProductRepo)
	aiFactory := new(MockAIFactory)
	provider := new(MockAIProvider)

	accountRepo.On("GetByID", mock.Anything, "acc1").Return(&domain.Account{ID: "acc1"}, nil)
	productRepo.On("GetByID", mock.Anything, "prod1").Return(&domain.Product{
		ID:         "prod1",
		AccountID:  "acc1",
		Name:       "Plano Premium",
		PriceCents: 9900,
		Currency:   "BRL",
		Attributes: json.RawMessage(`{"warranty":"12 meses"}`),
	}, nil)
	aiFactory.On("GetProvider", mock.Anything, mock.Anything).Return(provider, nil)
	provider.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&domain.AICallbackResponse{Content: "  Você é um vendedor consultivo...  "}, nil)

	uc := NewGeneratePersonaUseCase(accountRepo, planRepo, productRepo, aiFactory)
	res, err := uc.Execute(context.Background(), GeneratePersonaDTO{AccountID: "acc1", ProductID: "prod1"})

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "Você é um vendedor consultivo...", res.Persona)
	provider.AssertExpectations(t)
}

func TestGeneratePersona_FromDescription_Success(t *testing.T) {
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	productRepo := new(MockProductRepo)
	aiFactory := new(MockAIFactory)
	provider := new(MockAIProvider)

	accountRepo.On("GetByID", mock.Anything, "acc1").Return(&domain.Account{ID: "acc1"}, nil)
	aiFactory.On("GetProvider", mock.Anything, mock.Anything).Return(provider, nil)
	provider.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&domain.AICallbackResponse{Content: "Persona gerada"}, nil)

	uc := NewGeneratePersonaUseCase(accountRepo, planRepo, productRepo, aiFactory)
	res, err := uc.Execute(context.Background(), GeneratePersonaDTO{
		AccountID:   "acc1",
		Description: "Curso de inglês online com método imersivo",
	})

	assert.NoError(t, err)
	assert.Equal(t, "Persona gerada", res.Persona)
	// No product fetch when only a free description is supplied.
	productRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

func TestGeneratePersona_BYOKNotConfigured(t *testing.T) {
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	productRepo := new(MockProductRepo)
	aiFactory := new(MockAIFactory)

	accountRepo.On("GetByID", mock.Anything, "acc1").Return(&domain.Account{ID: "acc1"}, nil)
	aiFactory.On("GetProvider", mock.Anything, mock.Anything).Return(nil, domain.ErrBYOKKeyMissing)

	uc := NewGeneratePersonaUseCase(accountRepo, planRepo, productRepo, aiFactory)
	res, err := uc.Execute(context.Background(), GeneratePersonaDTO{
		AccountID:   "acc1",
		Description: "Consultoria financeira",
	})

	assert.Nil(t, res)
	assert.ErrorIs(t, err, domain.ErrBYOKNotConfigured)
}

func TestGeneratePersona_ProductOwnershipDenied(t *testing.T) {
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	productRepo := new(MockProductRepo)
	aiFactory := new(MockAIFactory)

	accountRepo.On("GetByID", mock.Anything, "acc1").Return(&domain.Account{ID: "acc1"}, nil)
	productRepo.On("GetByID", mock.Anything, "prodX").Return(&domain.Product{
		ID: "prodX", AccountID: "other-account", Name: "Alheio",
	}, nil)

	uc := NewGeneratePersonaUseCase(accountRepo, planRepo, productRepo, aiFactory)
	res, err := uc.Execute(context.Background(), GeneratePersonaDTO{AccountID: "acc1", ProductID: "prodX"})

	assert.Nil(t, res)
	assert.ErrorIs(t, err, domain.ErrProductNotFound)
	// Ownership rejected before any AI call.
	aiFactory.AssertNotCalled(t, "GetProvider", mock.Anything, mock.Anything)
}

func TestGeneratePersona_InputRequired(t *testing.T) {
	uc := NewGeneratePersonaUseCase(new(MockAccountRepo), new(MockPlanRepo), new(MockProductRepo), new(MockAIFactory))
	res, err := uc.Execute(context.Background(), GeneratePersonaDTO{AccountID: "acc1"})

	assert.Nil(t, res)
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "PERSONA_INPUT_REQUIRED", appErr.Code)
}

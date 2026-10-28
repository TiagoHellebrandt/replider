package usecases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/replider/api/internal/core/domain"
)

// Extract used to read a conversation by id alone (ListByConversationID is not
// account-scoped), so any authenticated user could pull another tenant's
// messages by posting a foreign conversation id. The ownership gate must run
// BEFORE a single message is loaded.
func TestSaleExtract_RefusesAConversationFromAnotherAccount(t *testing.T) {
	convRepo := &MockConversationRepo{}
	msgRepo := &MockMessageRepo{}
	aiFactory := &MockAIFactory{}
	accountRepo := &MockAccountRepo{}
	planRepo := &MockPlanRepo{}

	// Account A asks to extract a conversation that belongs to account B:
	// GetByID scoped to A returns nothing.
	convRepo.On("GetByID", mock.Anything, "conv-of-account-B", "account-A").
		Return(nil, nil)

	uc := NewSaleUseCase(&fakeSaleRepo{}, newFakeContactStore())
	uc.SetAI(aiFactory, accountRepo, planRepo, msgRepo)
	uc.SetConvRepo(convRepo)

	out, err := uc.Extract(context.Background(), "account-A", "conv-of-account-B")

	assert.Nil(t, out)
	assert.ErrorIs(t, err, domain.ErrConversationNotFound)
	// The messages of another tenant must never be touched.
	msgRepo.AssertNotCalled(t, "ListByConversationID", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	aiFactory.AssertNotCalled(t, "GetProviderForAccount", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// Extraction is refused outright when the ownership check is not wired, rather
// than silently falling back to the unscoped read.
func TestSaleExtract_RefusesWhenConvRepoUnwired(t *testing.T) {
	uc := NewSaleUseCase(&fakeSaleRepo{}, newFakeContactStore())
	uc.SetAI(&MockAIFactory{}, &MockAccountRepo{}, &MockPlanRepo{}, &MockMessageRepo{})
	// no SetConvRepo

	out, err := uc.Extract(context.Background(), "account-A", "conv-1")
	assert.Nil(t, out)
	assert.ErrorIs(t, err, errSaleAIUnavailable)
}

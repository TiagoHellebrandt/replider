package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/replider/api/internal/core/domain"
)

// --- DiscardBatch ----------------------------------------------------------

func TestDiscardBatch_OnlyPendingApprovalLeadsAreDiscarded(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	uc := NewLeadUseCase(leadRepo, funnelRepo)
	actor := domain.Actor{AccountID: "acc-1", Role: domain.RoleGestor}

	leadRepo.On("GetByID", mock.Anything, "lead-queued").
		Return(&domain.Lead{ID: "lead-queued", AccountID: "acc-1", Status: domain.LeadStatusPendingApproval}, nil)
	leadRepo.On("GetByID", mock.Anything, "lead-sent").
		Return(&domain.Lead{ID: "lead-sent", AccountID: "acc-1", Status: domain.LeadStatusContacted}, nil)
	leadRepo.On("GetByID", mock.Anything, "lead-gone").Return(nil, nil)
	leadRepo.On("UpdateStatus", mock.Anything, "lead-queued", domain.LeadStatusLost).Return(nil)

	res, err := uc.DiscardBatch(context.Background(), actor,
		[]string{"lead-queued", "lead-sent", "lead-gone"})

	assert.NoError(t, err)
	assert.Equal(t, 1, res.Discarded)
	assert.Equal(t, 2, res.Skipped)
	// The already-sent lead must never be flipped to lost by a sweep.
	leadRepo.AssertNotCalled(t, "UpdateStatus", mock.Anything, "lead-sent", mock.Anything)
}

func TestDiscardBatch_ForeignLeadIsSkippedNotDiscarded(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	uc := NewLeadUseCase(leadRepo, funnelRepo)
	// A vendedor sweeping the queue must not reach a colleague's lead.
	actor := domain.Actor{AccountID: "acc-1", UserID: "u-1", Role: domain.RoleVendedor}

	other := "u-2"
	leadRepo.On("GetByID", mock.Anything, "lead-colleague").
		Return(&domain.Lead{ID: "lead-colleague", AccountID: "acc-1",
			Status: domain.LeadStatusPendingApproval, OwnerUserID: &other}, nil)

	res, err := uc.DiscardBatch(context.Background(), actor, []string{"lead-colleague"})

	assert.NoError(t, err)
	assert.Equal(t, 0, res.Discarded)
	assert.Equal(t, 1, res.Skipped)
	leadRepo.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything)
}

func TestDiscardBatch_RefusesOversizedBatch(t *testing.T) {
	uc := NewLeadUseCase(new(mockLeadRepo), new(MockSalesFunnelRepo))
	ids := make([]string, maxBatchLeadIDs+1)
	_, err := uc.DiscardBatch(context.Background(), domain.Actor{AccountID: "acc-1", Role: domain.RoleGestor}, ids)
	assert.ErrorContains(t, err, "too many")
}

// --- RedraftBatch ----------------------------------------------------------

func TestRedraftBatch_StopsWholeBatchWhenCatalogueIsEmpty(t *testing.T) {
	uc, leadRepo, funnelRepo, _, contactRepo, accountRepo, _, _, _, _, _, _ := newApproachUC()

	// A dedicated product repo answering EMPTY, so the account-wide gate
	// fires on the first lead (the harness default seeds one product).
	emptyProducts := new(MockProductRepo)
	emptyProducts.On("ListByAccountID", mock.Anything, "acc1").Return([]*domain.Product{}, nil)
	uc.productRepo = emptyProducts

	leadRepo.On("GetByID", mock.Anything, "lead-1").
		Return(&domain.Lead{ID: "lead-1", AccountID: "acc1", FunnelID: "f1",
			ContactID: "+5511999999999", Status: domain.LeadStatusPendingApproval}, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1"}, nil)
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", "+5511999999999").Return(false, nil)
	accountRepo.On("GetByID", mock.Anything, "acc1").
		Return(&domain.Account{ID: "acc1", CreatedAt: time.Now()}, nil)

	res, err := uc.RedraftBatch(context.Background(), "acc1", []string{"lead-1", "lead-2", "lead-3"})

	assert.NoError(t, err)
	assert.Equal(t, 0, res.Redrafted)
	assert.Equal(t, domain.ErrApproachNeedsProducts.Code, res.StoppedReason)
	// lead-2 and lead-3 were never attempted: same account, same refusal,
	// and each attempt would still bill a provider call.
	assert.Equal(t, []string{"lead-1", "lead-2", "lead-3"}, res.RemainingLeadIDs)
	leadRepo.AssertNotCalled(t, "GetByID", mock.Anything, "lead-2")
}

func TestRedraftBatch_PerLeadFailureDoesNotStopTheRest(t *testing.T) {
	uc, leadRepo, funnelRepo, _, _, _, _, _, _, _, _, _ := newApproachUC()

	// lead-sent is not draftable; the batch keeps going past it.
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1"}, nil).Maybe()
	leadRepo.On("GetByID", mock.Anything, "lead-sent").
		Return(&domain.Lead{ID: "lead-sent", AccountID: "acc1", FunnelID: "f1",
			Status: domain.LeadStatusContacted}, nil)
	leadRepo.On("GetByID", mock.Anything, "lead-gone").Return(nil, nil)

	res, err := uc.RedraftBatch(context.Background(), "acc1", []string{"lead-sent", "lead-gone"})

	assert.NoError(t, err)
	assert.Equal(t, 0, res.Redrafted)
	assert.ElementsMatch(t, []string{"lead-sent", "lead-gone"}, res.FailedLeadIDs)
	assert.Empty(t, res.StoppedReason)
}

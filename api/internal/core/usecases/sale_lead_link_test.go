package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

// A sale filed from a lead's card. The app used to send the lead's own id as
// the customer and get CONTACT_NOT_FOUND; now the lead names the customer,
// and closing the deal closes it on the board too.

func newLeadLinkedSaleUC(leads *mockLeadRepo) (*SaleUseCase, *fakeSaleRepo, *fakeContactStore) {
	sales := &fakeSaleRepo{}
	contacts := newFakeContactStore()
	uc := NewSaleUseCase(sales, contacts).WithClock(func() time.Time {
		return time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	})
	uc.SetLeadRepo(leads)
	return uc, sales, contacts
}

func TestSaleRegister_LeadOnlyResolvesTheCustomerAndWinsTheLead(t *testing.T) {
	leads := &mockLeadRepo{}
	uc, sales, contacts := newLeadLinkedSaleUC(leads)
	// The lead carries the contact's EXTERNAL id, the legacy identity shape
	// the approach path uses — Register's fallback must still find it.
	contacts.byExternal["5511999990000"] = &domain.Contact{ID: "c-1", ExternalID: "5511999990000"}
	leads.On("GetByID", mock.Anything, "lead-1").Return(&domain.Lead{
		ID: "lead-1", AccountID: "acc-1", ContactID: "5511999990000", Status: domain.LeadStatusQualified,
	}, nil)
	leads.On("UpdateStatus", mock.Anything, "lead-1", domain.LeadStatusWon).Return(nil)

	saved, err := uc.Register(context.Background(), "acc-1", &domain.Sale{
		LeadID: "lead-1", GrossCents: 150000,
	})

	require.NoError(t, err)
	assert.Equal(t, "c-1", saved.ContactID, "resolved from the lead, then to the contact row")
	assert.Equal(t, "lead-1", saved.LeadID)
	require.Len(t, sales.created, 1)
	leads.AssertCalled(t, "UpdateStatus", mock.Anything, "lead-1", domain.LeadStatusWon)
}

// A lead already won is won; a lost one somebody sold to anyway is a decision
// for the person who marked it lost.
func TestSaleRegister_ATerminalLeadIsLeftAlone(t *testing.T) {
	leads := &mockLeadRepo{}
	uc, _, contacts := newLeadLinkedSaleUC(leads)
	contacts.byID["c-1"] = &domain.Contact{ID: "c-1"}
	leads.On("GetByID", mock.Anything, "lead-lost").Return(&domain.Lead{
		ID: "lead-lost", AccountID: "acc-1", ContactID: "c-1", Status: domain.LeadStatusLost,
	}, nil)

	_, err := uc.Register(context.Background(), "acc-1", &domain.Sale{
		LeadID: "lead-lost", ContactID: "c-1", GrossCents: 100,
	})

	require.NoError(t, err)
	leads.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything)
}

// Another account's lead is indistinguishable from a missing one.
func TestSaleRegister_RefusesALeadFromAnotherAccount(t *testing.T) {
	leads := &mockLeadRepo{}
	uc, sales, _ := newLeadLinkedSaleUC(leads)
	leads.On("GetByID", mock.Anything, "lead-x").Return(&domain.Lead{
		ID: "lead-x", AccountID: "acc-other", ContactID: "c-9", Status: domain.LeadStatusNew,
	}, nil)

	_, err := uc.Register(context.Background(), "acc-1", &domain.Sale{LeadID: "lead-x", GrossCents: 100})

	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "LEAD_NOT_FOUND", appErr.Code)
	assert.Empty(t, sales.created)
}

// An explicit sold_at is kept; only a missing one becomes "now".
func TestSaleRegister_KeepsAnExplicitSoldAt(t *testing.T) {
	uc, sales, contacts := newLeadLinkedSaleUC(&mockLeadRepo{})
	contacts.byID["c-1"] = &domain.Contact{ID: "c-1"}
	when := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)

	_, err := uc.Register(context.Background(), "acc-1", &domain.Sale{
		ContactID: "c-1", GrossCents: 100, SoldAt: when,
		Items: []domain.SaleItem{{ProductID: "p-1", Quantity: 2, UnitCents: 50}},
	})

	require.NoError(t, err)
	require.Len(t, sales.created, 1)
	assert.True(t, sales.created[0].SoldAt.Equal(when))
	assert.Equal(t, "p-1", sales.created[0].Items[0].ProductID)
}

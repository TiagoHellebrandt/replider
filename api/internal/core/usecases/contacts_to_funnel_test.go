package usecases

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

// Putting customers she already has into a funnel.
//
// The rules that matter are the ones about a real, dirty base: it runs twice
// without duplicating, one broken row does not take the other 1,999 down, and
// the report says WHICH rows failed.

func contactsFunnelUC(t *testing.T) (*LeadUseCase, *mockLeadRepo, *MockSalesFunnelRepo, *MockContactRepo) {
	t.Helper()
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	contactRepo := new(MockContactRepo)
	uc := NewLeadUseCase(leadRepo, funnelRepo)
	uc.SetContactRepo(contactRepo)
	return uc, leadRepo, funnelRepo, contactRepo
}

func ownedFunnel(funnelRepo *MockSalesFunnelRepo) {
	funnelRepo.On("GetByID", mock.Anything, "fn-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "acc-1"}, nil).Maybe()
}

func contactsNamed(ids ...string) []*domain.Contact {
	out := make([]*domain.Contact, 0, len(ids))
	for _, id := range ids {
		out = append(out, &domain.Contact{ID: id, AccountID: "acc-1"})
	}
	return out
}

func TestCreateFromContacts_CreatesOneLeadPerContact(t *testing.T) {
	uc, leadRepo, funnelRepo, contactRepo := contactsFunnelUC(t)
	ownedFunnel(funnelRepo)

	contactRepo.On("ListByIDs", mock.Anything, "acc-1", []string{"c1", "c2"}).
		Return(contactsNamed("c1", "c2"), nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", mock.Anything).Return(nil, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	res, err := uc.CreateFromContacts(context.Background(), ContactsToFunnelInput{
		AccountID: "acc-1", FunnelID: "fn-1", ContactIDs: []string{"c1", "c2"},
	})
	require.NoError(t, err)

	assert.Equal(t, 2, res.Created)
	assert.Zero(t, res.Skipped)
	assert.Zero(t, res.Failed)
	assert.Len(t, res.LeadIDs, 2)

	// The contact is used AS IS — this path must never invent or rewrite one.
	// The whole reason it exists is that the person is already in the CRM.
	contactRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
	// And the leads point at the contact ULID, which is what the timeline and
	// the approval queue resolve the recipient by.
	for _, call := range leadRepo.Calls {
		if call.Method != "Upsert" {
			continue
		}
		lead := call.Arguments.Get(1).(*domain.Lead)
		assert.Contains(t, []string{"c1", "c2"}, lead.ContactID)
		assert.Equal(t, domain.LeadStatusNew, lead.Status)
		assert.Equal(t, "existing_customer", lead.Source,
			"source distinguishes a known customer from a cold prospect — an approach written as if they were strangers reads as amnesia")
	}
}

// Running it twice is the normal case: she selects a page, selects it again
// after a scroll, or a colleague already imported half the base. The second run
// must not reset a pipeline that has moved.
func TestCreateFromContacts_AContactAlreadyInTheFunnelIsSkippedNotReset(t *testing.T) {
	uc, leadRepo, funnelRepo, contactRepo := contactsFunnelUC(t)
	ownedFunnel(funnelRepo)

	contactRepo.On("ListByIDs", mock.Anything, "acc-1", []string{"c1"}).
		Return(contactsNamed("c1"), nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", "c1").
		Return(&domain.Lead{ID: "lead-old", Status: domain.LeadStatusResponded}, nil)

	res, err := uc.CreateFromContacts(context.Background(), ContactsToFunnelInput{
		AccountID: "acc-1", FunnelID: "fn-1", ContactIDs: []string{"c1"},
	})
	require.NoError(t, err)

	assert.Zero(t, res.Created)
	assert.Equal(t, 1, res.Skipped)
	assert.Equal(t, []string{"lead-old"}, res.LeadIDs,
		"the existing lead is reported, so the client can still open it")
	leadRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}

// The rule the whole feature turns on with a spreadsheet-sourced base: some rows
// are always broken, and aborting on the first one means the good 1,990 wait for
// the bad 10 to be fixed.
func TestCreateFromContacts_OneFailureDoesNotAbortTheBatch(t *testing.T) {
	uc, leadRepo, funnelRepo, contactRepo := contactsFunnelUC(t)
	ownedFunnel(funnelRepo)

	contactRepo.On("ListByIDs", mock.Anything, "acc-1", []string{"c1", "c2", "c3"}).
		Return(contactsNamed("c1", "c2", "c3"), nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", mock.Anything).Return(nil, nil)
	leadRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(l *domain.Lead) bool {
		return l.ContactID == "c2"
	})).Return(errors.New("deadlock"))
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	res, err := uc.CreateFromContacts(context.Background(), ContactsToFunnelInput{
		AccountID: "acc-1", FunnelID: "fn-1", ContactIDs: []string{"c1", "c2", "c3"},
	})
	require.NoError(t, err, "a batch with a bad row is not a failed request")

	assert.Equal(t, 2, res.Created)
	assert.Equal(t, 1, res.Failed)
	require.Len(t, res.Problems, 1)
	assert.Equal(t, "c2", res.Problems[0].ContactID,
		"naming the row is the point — '1 of 3 failed' makes her re-import all three to find it")
	assert.Equal(t, ContactToFunnelReasonFailed, res.Problems[0].Reason)
}

// A contact id from another tenant reads exactly like one that never existed.
// Anything else turns this endpoint into a probe for other accounts' ids.
func TestCreateFromContacts_AForeignContactIsIndistinguishableFromAMissingOne(t *testing.T) {
	uc, leadRepo, funnelRepo, contactRepo := contactsFunnelUC(t)
	ownedFunnel(funnelRepo)

	// The repo is account-scoped, so a foreign id simply is not in the result.
	contactRepo.On("ListByIDs", mock.Anything, "acc-1", []string{"c1", "foreign"}).
		Return(contactsNamed("c1"), nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", "c1").Return(nil, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	res, err := uc.CreateFromContacts(context.Background(), ContactsToFunnelInput{
		AccountID: "acc-1", FunnelID: "fn-1", ContactIDs: []string{"c1", "foreign"},
	})
	require.NoError(t, err)

	assert.Equal(t, 1, res.Created)
	assert.Equal(t, 1, res.Failed)
	require.Len(t, res.Problems, 1)
	assert.Equal(t, ContactToFunnelReasonNotFound, res.Problems[0].Reason)
	leadRepo.AssertNotCalled(t, "GetByFunnelAndContact", mock.Anything, "fn-1", "foreign")
}

// The destination is checked before the payload is judged: somebody who may not
// write to this funnel should learn that, not that their selection was malformed.
func TestCreateFromContacts_RefusesAnotherAccountsFunnelBeforeReadingTheSelection(t *testing.T) {
	uc, leadRepo, funnelRepo, contactRepo := contactsFunnelUC(t)
	funnelRepo.On("GetByID", mock.Anything, "fn-1").
		Return(&domain.SalesFunnel{ID: "fn-1", AccountID: "other-acc"}, nil)

	// Deliberately an empty selection, which would otherwise be a 400.
	_, err := uc.CreateFromContacts(context.Background(), ContactsToFunnelInput{
		AccountID: "acc-1", FunnelID: "fn-1", ContactIDs: nil,
	})

	assert.ErrorIs(t, err, domain.ErrUnauthorized)
	contactRepo.AssertNotCalled(t, "ListByIDs", mock.Anything, mock.Anything, mock.Anything)
	leadRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}

// The same person selected twice counts once, or the reported totals do not add
// up to what she selected and the numbers stop being trustworthy.
func TestCreateFromContacts_ARepeatedSelectionCountsOnce(t *testing.T) {
	uc, leadRepo, funnelRepo, contactRepo := contactsFunnelUC(t)
	ownedFunnel(funnelRepo)

	contactRepo.On("ListByIDs", mock.Anything, "acc-1", []string{"c1"}).
		Return(contactsNamed("c1"), nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", "c1").Return(nil, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil).Once()

	res, err := uc.CreateFromContacts(context.Background(), ContactsToFunnelInput{
		AccountID: "acc-1", FunnelID: "fn-1", ContactIDs: []string{"c1", " c1 ", "", "c1"},
	})
	require.NoError(t, err)

	assert.Equal(t, 1, res.Created)
	assert.Equal(t, 1, res.Created+res.Skipped+res.Failed)
	leadRepo.AssertExpectations(t)
}

// Refuse, do not truncate. Silently importing the first 500 of a 2,000-row
// "select all" reports success for work it did not do, and the missing leads
// surface weeks later as customers nobody followed up.
func TestCreateFromContacts_RefusesAnOversizedSelectionInsteadOfTruncating(t *testing.T) {
	uc, leadRepo, funnelRepo, contactRepo := contactsFunnelUC(t)
	ownedFunnel(funnelRepo)

	ids := make([]string, MaxContactsPerFunnelImport+1)
	for i := range ids {
		ids[i] = "c" + strconv.Itoa(i)
	}
	_, err := uc.CreateFromContacts(context.Background(), ContactsToFunnelInput{
		AccountID: "acc-1", FunnelID: "fn-1", ContactIDs: ids,
	})

	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
	contactRepo.AssertNotCalled(t, "ListByIDs", mock.Anything, mock.Anything, mock.Anything)
	leadRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}

// Omitting the owner leaves the leads unassigned and team-visible. An admin
// importing the company's base is not claiming all of it for themselves.
func TestCreateFromContacts_NoOwnerMeansUnassignedAndTeamVisible(t *testing.T) {
	uc, leadRepo, funnelRepo, contactRepo := contactsFunnelUC(t)
	ownedFunnel(funnelRepo)

	contactRepo.On("ListByIDs", mock.Anything, "acc-1", []string{"c1"}).
		Return(contactsNamed("c1"), nil)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fn-1", "c1").Return(nil, nil)
	var saved *domain.Lead
	leadRepo.On("Upsert", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { saved = args.Get(1).(*domain.Lead) }).Return(nil)

	_, err := uc.CreateFromContacts(context.Background(), ContactsToFunnelInput{
		AccountID: "acc-1", FunnelID: "fn-1", ContactIDs: []string{"c1"},
	})
	require.NoError(t, err)

	require.NotNil(t, saved)
	assert.Nil(t, saved.OwnerUserID)
	assert.Equal(t, domain.VisibilityTeam, saved.Visibility)
}

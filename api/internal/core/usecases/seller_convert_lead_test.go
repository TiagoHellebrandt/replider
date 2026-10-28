package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

const (
	testPlatformAcc = "PLATACC00000000000000000000"
	testCustomerAcc = "CUSTACC00000000000000000000"
	testSellerUser  = "SELLERUSER00000000000000000"
	testOtherUser   = "OTHERUSER000000000000000000"
	testConvID      = "CONV0000000000000000000000"
)

type mockConvGetter struct {
	conv *domain.Conversation
	err  error
	// captured
	gotID, gotAccountID string
}

func (m *mockConvGetter) GetByID(_ context.Context, id, accountID string) (*domain.Conversation, error) {
	m.gotID, m.gotAccountID = id, accountID
	return m.conv, m.err
}

type mockSellerLeadRepo struct {
	accounts      map[string]*domain.SellerLeadAccount
	searchResults []domain.SellerLeadAccount
	searchErr     error
	stampErr      error
	// accountEmails maps accountID -> the emails its users own (lowercase),
	// backing AccountHasUserEmail.
	accountEmails map[string][]string
	// captured
	stampedAccount, stampedSeller string
	stampedConversation           string
	stampCalls                    int
	gotSearch                     string
	gotSearchPlatform             string
}

func (m *mockSellerLeadRepo) SearchCustomerAccounts(_ context.Context, search, platformAccountID string, _ int) ([]domain.SellerLeadAccount, error) {
	m.gotSearch, m.gotSearchPlatform = search, platformAccountID
	return m.searchResults, m.searchErr
}

func (m *mockSellerLeadRepo) GetLeadAccount(_ context.Context, accountID string) (*domain.SellerLeadAccount, error) {
	return m.accounts[accountID], nil
}

func (m *mockSellerLeadRepo) StampInboundConversion(_ context.Context, accountID, sellerUserID, conversationID string) (bool, error) {
	m.stampCalls++
	if m.stampErr != nil {
		return false, m.stampErr
	}
	acc := m.accounts[accountID]
	if acc == nil || acc.SellerUserID != nil {
		return false, nil
	}
	m.stampedAccount, m.stampedSeller = accountID, sellerUserID
	m.stampedConversation = conversationID
	seller := sellerUserID
	source := domain.LeadSourceInbound
	now := time.Now()
	acc.SellerUserID, acc.LeadSource, acc.SellerLinkedAt = &seller, &source, &now
	return true, nil
}

func (m *mockSellerLeadRepo) AccountHasUserEmail(_ context.Context, accountID, email string) (bool, error) {
	for _, e := range m.accountEmails[accountID] {
		if strings.EqualFold(e, email) {
			return true, nil
		}
	}
	return false, nil
}

// mockSellerContacts resolves the conversation's contact (US-014
// relation check).
type mockSellerContacts struct {
	contact *domain.Contact
	err     error
}

func (m *mockSellerContacts) GetByExternalID(_ context.Context, _, _ string) (*domain.Contact, error) {
	return m.contact, m.err
}

// mockSellerFlags is the explicit is_seller lookup (US-014).
type mockSellerFlags struct {
	sellers map[string]bool
	err     error
}

func (m *mockSellerFlags) IsSeller(_ context.Context, userID string) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.sellers[userID], nil
}

// allSellers treats every user as flagged — for tests exercising other
// rules.
func allSellers() *mockSellerFlags {
	return &mockSellerFlags{sellers: map[string]bool{testSellerUser: true, testOtherUser: true}}
}

// newConvertUC wires a use case with no contact on file and everyone
// flagged as seller — the pre-US-014 baseline the older tests assume.
func newConvertUC(convs *mockConvGetter, repo domain.SellerLeadRepository) *ConvertLeadUseCase {
	return NewConvertLeadUseCase(convs, repo, &mockSellerContacts{}, allSellers(), testPlatformAcc)
}

func assignedConv(userID string) *domain.Conversation {
	return &domain.Conversation{ID: testConvID, AccountID: testPlatformAcc, AssignedTo: &userID, ContactID: "+5565999990000"}
}

func unlinkedAccount() map[string]*domain.SellerLeadAccount {
	return map[string]*domain.SellerLeadAccount{
		testCustomerAcc: {ID: testCustomerAcc, Name: "Padaria do Zé", CreatedAt: time.Now()},
	}
}

func TestConvertLead_AssigneeConverts(t *testing.T) {
	convs := &mockConvGetter{conv: assignedConv(testSellerUser)}
	repo := &mockSellerLeadRepo{accounts: unlinkedAccount()}
	uc := newConvertUC(convs, repo)

	got, err := uc.Execute(context.Background(), testSellerUser, "attendant", testConvID, testCustomerAcc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if convs.gotAccountID != testPlatformAcc {
		t.Errorf("conversation looked up on account %q, want platform account", convs.gotAccountID)
	}
	if repo.stampedAccount != testCustomerAcc || repo.stampedSeller != testSellerUser {
		t.Errorf("stamped (%q,%q), want (%q,%q)", repo.stampedAccount, repo.stampedSeller, testCustomerAcc, testSellerUser)
	}
	if repo.stampedConversation != testConvID {
		t.Errorf("stamped conversation = %q, want %q (co-pilot deep link, US-015)", repo.stampedConversation, testConvID)
	}
	if got.SellerUserID == nil || *got.SellerUserID != testSellerUser {
		t.Errorf("returned account seller = %v, want %q", got.SellerUserID, testSellerUser)
	}
	if got.LeadSource == nil || *got.LeadSource != domain.LeadSourceInbound {
		t.Errorf("lead source = %v, want inbound", got.LeadSource)
	}
}

func TestConvertLead_AdminConvertsForAssignee(t *testing.T) {
	// Admin who is NOT the assignee may convert; the stamped seller is
	// still the assigned attendant, not the admin.
	convs := &mockConvGetter{conv: assignedConv(testSellerUser)}
	repo := &mockSellerLeadRepo{accounts: unlinkedAccount()}
	uc := newConvertUC(convs, repo)

	_, err := uc.Execute(context.Background(), testOtherUser, "admin", testConvID, testCustomerAcc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.stampedSeller != testSellerUser {
		t.Errorf("stamped seller = %q, want assigned attendant %q", repo.stampedSeller, testSellerUser)
	}
}

func TestConvertLead_NonAssigneeAttendantForbidden(t *testing.T) {
	convs := &mockConvGetter{conv: assignedConv(testSellerUser)}
	repo := &mockSellerLeadRepo{accounts: unlinkedAccount()}
	uc := newConvertUC(convs, repo)

	_, err := uc.Execute(context.Background(), testOtherUser, "attendant", testConvID, testCustomerAcc)
	if !errors.Is(err, domain.ErrSellerNotConvAssignee) {
		t.Fatalf("err = %v, want ErrSellerNotConvAssignee", err)
	}
	if repo.stampCalls != 0 {
		t.Errorf("stamp called %d times, want 0", repo.stampCalls)
	}
}

func TestConvertLead_UnassignedConversation(t *testing.T) {
	convs := &mockConvGetter{conv: &domain.Conversation{ID: testConvID, AccountID: testPlatformAcc}}
	repo := &mockSellerLeadRepo{accounts: unlinkedAccount()}
	uc := newConvertUC(convs, repo)

	_, err := uc.Execute(context.Background(), testSellerUser, "admin", testConvID, testCustomerAcc)
	if !errors.Is(err, domain.ErrSellerConvUnassigned) {
		t.Fatalf("err = %v, want ErrSellerConvUnassigned", err)
	}
}

func TestConvertLead_ConversationNotFound(t *testing.T) {
	convs := &mockConvGetter{conv: nil}
	uc := newConvertUC(convs, &mockSellerLeadRepo{})

	_, err := uc.Execute(context.Background(), testSellerUser, "attendant", testConvID, testCustomerAcc)
	if !errors.Is(err, domain.ErrConversationNotFound) {
		t.Fatalf("err = %v, want ErrConversationNotFound", err)
	}
}

func TestConvertLead_AccountNotFound(t *testing.T) {
	convs := &mockConvGetter{conv: assignedConv(testSellerUser)}
	uc := newConvertUC(convs, &mockSellerLeadRepo{accounts: map[string]*domain.SellerLeadAccount{}})

	_, err := uc.Execute(context.Background(), testSellerUser, "attendant", testConvID, testCustomerAcc)
	if !errors.Is(err, domain.ErrAccountNotFound) {
		t.Fatalf("err = %v, want ErrAccountNotFound", err)
	}
}

func TestConvertLead_AlreadyLinkedConflict(t *testing.T) {
	other := testOtherUser
	convs := &mockConvGetter{conv: assignedConv(testSellerUser)}
	repo := &mockSellerLeadRepo{accounts: map[string]*domain.SellerLeadAccount{
		testCustomerAcc: {ID: testCustomerAcc, Name: "Já Vinculada", SellerUserID: &other},
	}}
	uc := newConvertUC(convs, repo)

	_, err := uc.Execute(context.Background(), testSellerUser, "attendant", testConvID, testCustomerAcc)
	if !errors.Is(err, domain.ErrSellerAccountAlreadyLinked) {
		t.Fatalf("err = %v, want ErrSellerAccountAlreadyLinked", err)
	}
	if repo.stampCalls != 0 {
		t.Errorf("stamp called %d times, want 0 (pre-check must short-circuit)", repo.stampCalls)
	}
}

func TestConvertLead_StampRaceLostConflict(t *testing.T) {
	// GetLeadAccount sees the account unlinked, but the conditional UPDATE
	// affects 0 rows (a concurrent convert won) — must surface the same
	// conflict, never a silent success.
	convs := &mockConvGetter{conv: assignedConv(testSellerUser)}
	repo := &raceLostRepo{inner: &mockSellerLeadRepo{accounts: unlinkedAccount()}}
	uc := newConvertUC(convs, repo)

	_, err := uc.Execute(context.Background(), testSellerUser, "attendant", testConvID, testCustomerAcc)
	if !errors.Is(err, domain.ErrSellerAccountAlreadyLinked) {
		t.Fatalf("err = %v, want ErrSellerAccountAlreadyLinked", err)
	}
}

type raceLostRepo struct{ inner *mockSellerLeadRepo }

func (r *raceLostRepo) SearchCustomerAccounts(ctx context.Context, s, p string, l int) ([]domain.SellerLeadAccount, error) {
	return r.inner.SearchCustomerAccounts(ctx, s, p, l)
}
func (r *raceLostRepo) GetLeadAccount(ctx context.Context, id string) (*domain.SellerLeadAccount, error) {
	return r.inner.GetLeadAccount(ctx, id)
}
func (r *raceLostRepo) StampInboundConversion(context.Context, string, string, string) (bool, error) {
	return false, nil
}
func (r *raceLostRepo) AccountHasUserEmail(ctx context.Context, id, email string) (bool, error) {
	return r.inner.AccountHasUserEmail(ctx, id, email)
}

func TestConvertLead_CannotConvertPlatformAccount(t *testing.T) {
	convs := &mockConvGetter{conv: assignedConv(testSellerUser)}
	repo := &mockSellerLeadRepo{accounts: map[string]*domain.SellerLeadAccount{
		testPlatformAcc: {ID: testPlatformAcc, Name: "Replider"},
	}}
	uc := newConvertUC(convs, repo)

	_, err := uc.Execute(context.Background(), testSellerUser, "attendant", testConvID, testPlatformAcc)
	if !errors.Is(err, domain.ErrSellerCannotConvertPlatform) {
		t.Fatalf("err = %v, want ErrSellerCannotConvertPlatform", err)
	}
}

// ── US-014 (review fixes): convert-lead tie-down + explicit seller flag ─────

func payingAccount() map[string]*domain.SellerLeadAccount {
	return map[string]*domain.SellerLeadAccount{
		testCustomerAcc: {ID: testCustomerAcc, Name: "Cliente Orgânico Pagante",
			HasPaymentHistory: true, CreatedAt: time.Now()},
	}
}

// FAILS on the pre-US-014 code: any attendant could point convert-lead at
// an arbitrary paying customer account and steal its attribution.
func TestConvertLead_PayingUnrelatedAccountRejected(t *testing.T) {
	convs := &mockConvGetter{conv: assignedConv(testSellerUser)}
	repo := &mockSellerLeadRepo{accounts: payingAccount()}
	// No contact on file for the conversation — the claim is unverifiable.
	uc := NewConvertLeadUseCase(convs, repo, &mockSellerContacts{}, allSellers(), testPlatformAcc)

	_, err := uc.Execute(context.Background(), testSellerUser, "attendant", testConvID, testCustomerAcc)
	if !errors.Is(err, domain.ErrSellerConvTargetUnrelated) {
		t.Fatalf("err = %v, want ErrSellerConvTargetUnrelated", err)
	}
	if repo.stampCalls != 0 {
		t.Errorf("stamp called %d times, want 0 — a paying unrelated account must never be claimed", repo.stampCalls)
	}
}

func TestConvertLead_PayingAccountContactEmailMismatchRejected(t *testing.T) {
	convs := &mockConvGetter{conv: assignedConv(testSellerUser)}
	email := "lead@gmail.com"
	repo := &mockSellerLeadRepo{
		accounts:      payingAccount(),
		accountEmails: map[string][]string{testCustomerAcc: {"dono@padaria.com"}},
	}
	contacts := &mockSellerContacts{contact: &domain.Contact{ExternalID: "+5565999990000", Email: &email}}
	uc := NewConvertLeadUseCase(convs, repo, contacts, allSellers(), testPlatformAcc)

	_, err := uc.Execute(context.Background(), testSellerUser, "attendant", testConvID, testCustomerAcc)
	if !errors.Is(err, domain.ErrSellerConvTargetUnrelated) {
		t.Fatalf("err = %v, want ErrSellerConvTargetUnrelated", err)
	}
}

func TestConvertLead_PayingAccountContactEmailMatchesAllowed(t *testing.T) {
	convs := &mockConvGetter{conv: assignedConv(testSellerUser)}
	email := "Dono@Padaria.com" // case-insensitive match
	repo := &mockSellerLeadRepo{
		accounts:      payingAccount(),
		accountEmails: map[string][]string{testCustomerAcc: {"dono@padaria.com"}},
	}
	contacts := &mockSellerContacts{contact: &domain.Contact{ExternalID: "+5565999990000", Email: &email}}
	uc := NewConvertLeadUseCase(convs, repo, contacts, allSellers(), testPlatformAcc)

	got, err := uc.Execute(context.Background(), testSellerUser, "attendant", testConvID, testCustomerAcc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.SellerUserID == nil || *got.SellerUserID != testSellerUser {
		t.Errorf("returned account seller = %v, want %q", got.SellerUserID, testSellerUser)
	}
}

// Trial account (no payment history) needs no relation proof — the
// existing happy-path tests cover it via unlinkedAccount(); this pins the
// distinction explicitly.
func TestConvertLead_TrialAccountNeedsNoRelation(t *testing.T) {
	convs := &mockConvGetter{conv: assignedConv(testSellerUser)}
	repo := &mockSellerLeadRepo{accounts: unlinkedAccount()} // HasPaymentHistory=false
	uc := NewConvertLeadUseCase(convs, repo, &mockSellerContacts{}, allSellers(), testPlatformAcc)

	if _, err := uc.Execute(context.Background(), testSellerUser, "attendant", testConvID, testCustomerAcc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// FAILS on the pre-US-014 code: converting stamped ANY assigned attendant
// as a payable seller — no explicit grant required.
func TestConvertLead_UnflaggedAttendantRejected(t *testing.T) {
	convs := &mockConvGetter{conv: assignedConv(testSellerUser)}
	repo := &mockSellerLeadRepo{accounts: unlinkedAccount()}
	flags := &mockSellerFlags{sellers: map[string]bool{}} // nobody flagged
	uc := NewConvertLeadUseCase(convs, repo, &mockSellerContacts{}, flags, testPlatformAcc)

	_, err := uc.Execute(context.Background(), testSellerUser, "attendant", testConvID, testCustomerAcc)
	if !errors.Is(err, domain.ErrSellerNotFlagged) {
		t.Fatalf("err = %v, want ErrSellerNotFlagged", err)
	}
	if repo.stampCalls != 0 {
		t.Errorf("stamp called %d times, want 0 — conversion must never mint a payable seller", repo.stampCalls)
	}
}

func TestConvertLead_FlagLookupErrorFailsClosed(t *testing.T) {
	convs := &mockConvGetter{conv: assignedConv(testSellerUser)}
	repo := &mockSellerLeadRepo{accounts: unlinkedAccount()}
	flags := &mockSellerFlags{err: errors.New("db down")}
	uc := NewConvertLeadUseCase(convs, repo, &mockSellerContacts{}, flags, testPlatformAcc)

	_, err := uc.Execute(context.Background(), testSellerUser, "attendant", testConvID, testCustomerAcc)
	if err == nil {
		t.Fatal("expected error, got nil — flag lookup failure must fail closed")
	}
	if repo.stampCalls != 0 {
		t.Errorf("stamp called %d times, want 0", repo.stampCalls)
	}
}

func TestConvertLeadSearch_ShortQueryReturnsEmpty(t *testing.T) {
	repo := &mockSellerLeadRepo{searchResults: []domain.SellerLeadAccount{{ID: "X"}}}
	uc := newConvertUC(&mockConvGetter{}, repo)

	got, err := uc.Search(context.Background(), " a ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("short query returned %d results, want 0", len(got))
	}
	if repo.gotSearch != "" {
		t.Errorf("repo was queried with %q, want no call", repo.gotSearch)
	}
}

func TestConvertLeadSearch_PassesPlatformExclusion(t *testing.T) {
	repo := &mockSellerLeadRepo{searchResults: nil} // nil from repo → [] out
	uc := newConvertUC(&mockConvGetter{}, repo)

	got, err := uc.Search(context.Background(), "padaria")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("Search returned nil slice, want empty (JSON [] not null)")
	}
	if repo.gotSearch != "padaria" || repo.gotSearchPlatform != testPlatformAcc {
		t.Errorf("repo called with (%q,%q), want (padaria,%q)", repo.gotSearch, repo.gotSearchPlatform, testPlatformAcc)
	}
}

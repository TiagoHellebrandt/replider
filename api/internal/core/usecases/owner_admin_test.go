package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockOwnerAccountRepo covers the listing-only half of the owner repo
// surface (TopAccountsByTokens + ExtendTrial). The regular AccountRepo
// methods come from MockAccountRepo in mocks_test.go.
type MockOwnerAccountRepo struct{ mock.Mock }

func (m *MockOwnerAccountRepo) ListForOwner(ctx context.Context, search, status string, limit, offset int) ([]*domain.AccountListing, error) {
	a := m.Called(ctx, search, status, limit, offset)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).([]*domain.AccountListing), a.Error(1)
}

func (m *MockOwnerAccountRepo) CountForOwner(ctx context.Context, search, status string) (int64, error) {
	a := m.Called(ctx, search, status)
	return int64(a.Int(0)), a.Error(1)
}

func (m *MockOwnerAccountRepo) TopAccountsByTokens(ctx context.Context, limit int) ([]*domain.AccountTokenUsage, error) {
	a := m.Called(ctx, limit)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).([]*domain.AccountTokenUsage), a.Error(1)
}

func (m *MockOwnerAccountRepo) ExtendTrial(ctx context.Context, accountID string, newExpiresAt time.Time) error {
	return m.Called(ctx, accountID, newExpiresAt).Error(0)
}

func (m *MockOwnerAccountRepo) ListConversationsForOwner(ctx context.Context, accountID string, limit, offset int) ([]*domain.OwnerConversationRow, error) {
	a := m.Called(ctx, accountID, limit, offset)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).([]*domain.OwnerConversationRow), a.Error(1)
}

func (m *MockOwnerAccountRepo) SystemHealth(ctx context.Context) (*domain.SystemHealthSummary, error) {
	a := m.Called(ctx)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*domain.SystemHealthSummary), a.Error(1)
}

func (m *MockOwnerAccountRepo) UsageTrend(ctx context.Context, accountID string) ([]*domain.UsageTrendDay, error) {
	a := m.Called(ctx, accountID)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).([]*domain.UsageTrendDay), a.Error(1)
}

func (m *MockOwnerAccountRepo) CountActiveInternalThreads(ctx context.Context, accountID string) (int64, error) {
	a := m.Called(ctx, accountID)
	return int64(a.Int(0)), a.Error(1)
}

func (m *MockOwnerAccountRepo) SetSeller(ctx context.Context, accountID, sellerUserID, leadSource string) error {
	return m.Called(ctx, accountID, sellerUserID, leadSource).Error(0)
}

func (m *MockOwnerAccountRepo) ClearSeller(ctx context.Context, accountID string) error {
	return m.Called(ctx, accountID).Error(0)
}

func (m *MockOwnerAccountRepo) SetCurrency(ctx context.Context, accountID, currency string) error {
	return m.Called(ctx, accountID, currency).Error(0)
}

// stubJWT lets the impersonate tests assert the claims passed to the
// JWT service without spinning up a real signer.
type stubJWT struct {
	lastClaims domain.ImpersonationClaims
	tokenOut   string
	err        error
}

func (s *stubJWT) GenerateToken(*domain.User) (string, error)       { return "", nil }
func (s *stubJWT) ValidateToken(string) (*domain.UserClaims, error) { return nil, nil }
func (s *stubJWT) GenerateImpersonationToken(c domain.ImpersonationClaims) (string, error) {
	s.lastClaims = c
	return s.tokenOut, s.err
}
func (s *stubJWT) GenerateMfaLoginToken(string) (string, error)                 { return "", nil }
func (s *stubJWT) ValidateMfaLoginToken(string) (*domain.MfaLoginClaims, error) { return nil, nil }

func makeOwnerUC(t *testing.T) (*OwnerAdminUseCase, *MockAccountRepo, *MockOwnerAccountRepo, *MockUserRepo, *MockPlanRepo, *MockUsageRepo, *MockAuditLogRepo) {
	t.Helper()
	accRepo := new(MockAccountRepo)
	ownerRepo := new(MockOwnerAccountRepo)
	userRepo := new(MockUserRepo)
	planRepo := new(MockPlanRepo)
	usageRepo := new(MockUsageRepo)
	auditRepo := new(MockAuditLogRepo)
	auditRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Maybe()
	uc := NewOwnerAdminUseCase(accRepo, ownerRepo, userRepo, planRepo, usageRepo, auditRepo)
	return uc, accRepo, ownerRepo, userRepo, planRepo, usageRepo, auditRepo
}

func TestOwnerAdmin_Suspend_RequiresReason(t *testing.T) {
	uc, _, _, _, _, _, _ := makeOwnerUC(t)
	err := uc.Suspend(context.Background(), "ops@x.com", "acc-1", "  ")
	assert.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "OWNER_SUSPEND_REASON_REQUIRED", appErr.Code)
}

func TestOwnerAdmin_Suspend_DelegatesToRepo(t *testing.T) {
	uc, accRepo, _, _, _, _, _ := makeOwnerUC(t)
	accRepo.On("Suspend", mock.Anything, "acc-1", "abuse").Return(nil)
	err := uc.Suspend(context.Background(), "ops@x.com", "acc-1", "abuse")
	assert.NoError(t, err)
	accRepo.AssertCalled(t, "Suspend", mock.Anything, "acc-1", "abuse")
}

func TestOwnerAdmin_Reactivate_DelegatesToRepo(t *testing.T) {
	uc, accRepo, _, _, _, _, _ := makeOwnerUC(t)
	accRepo.On("Reactivate", mock.Anything, "acc-1").Return(nil)
	err := uc.Reactivate(context.Background(), "ops@x.com", "acc-1")
	assert.NoError(t, err)
}

func TestOwnerAdmin_SetSubscriptionStatus_RejectsInvalid(t *testing.T) {
	uc, _, _, _, _, _, _ := makeOwnerUC(t)
	err := uc.SetSubscriptionStatus(context.Background(), "ops@x.com", "acc-1", "bogus")
	assert.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "OWNER_INVALID_STATUS", appErr.Code)
}

func TestOwnerAdmin_SetCurrency_RejectsInvalidCurrency(t *testing.T) {
	uc, _, _, _, _, _, _ := makeOwnerUC(t)
	err := uc.SetCurrency(context.Background(), "ops@x.com", "acc-1", "eur")
	assert.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "OWNER_INVALID_CURRENCY", appErr.Code)
}

func TestOwnerAdmin_SetCurrency_CorrectsArbitragedAccount(t *testing.T) {
	// US-015: a pt-BR user forced an "en" locale to land on the USD
	// ladder; the owner corrects the account back to BRL.
	uc, accRepo, ownerRepo, _, _, _, auditRepo := makeOwnerUC(t)
	accRepo.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1", Currency: domain.CurrencyUSD}, nil)
	ownerRepo.On("SetCurrency", mock.Anything, "acc-1", "brl").Return(nil)

	err := uc.SetCurrency(context.Background(), "ops@x.com", "acc-1", "BRL ")
	assert.NoError(t, err)
	ownerRepo.AssertCalled(t, "SetCurrency", mock.Anything, "acc-1", "brl")
	auditRepo.AssertCalled(t, "Create", mock.Anything, mock.MatchedBy(func(l *domain.AuditLog) bool {
		return l.Action == "owner.account.set_currency" && l.AccountID == "acc-1"
	}))
}

func TestOwnerAdmin_SetCurrency_BlocksActiveSubscription(t *testing.T) {
	// An active Stripe subscription bills in the current currency —
	// flipping the account under it would desync checkout/referrals.
	uc, accRepo, ownerRepo, _, _, _, _ := makeOwnerUC(t)
	sub := "sub_live"
	accRepo.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1", Currency: domain.CurrencyUSD, StripeSubscriptionID: &sub}, nil)

	err := uc.SetCurrency(context.Background(), "ops@x.com", "acc-1", "brl")
	assert.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "OWNER_CURRENCY_HAS_SUBSCRIPTION", appErr.Code)
	ownerRepo.AssertNotCalled(t, "SetCurrency", mock.Anything, mock.Anything, mock.Anything)
}

func TestOwnerAdmin_SetCurrency_SameCurrencyIsNoOp(t *testing.T) {
	uc, accRepo, ownerRepo, _, _, _, _ := makeOwnerUC(t)
	accRepo.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1", Currency: domain.CurrencyBRL}, nil)

	err := uc.SetCurrency(context.Background(), "ops@x.com", "acc-1", "brl")
	assert.NoError(t, err)
	ownerRepo.AssertNotCalled(t, "SetCurrency", mock.Anything, mock.Anything, mock.Anything)
}

func TestOwnerAdmin_GrantCredits_RejectsZero(t *testing.T) {
	uc, _, _, _, _, _, _ := makeOwnerUC(t)
	err := uc.GrantCredits(context.Background(), "ops@x.com", "acc-1", 0)
	assert.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "OWNER_GRANT_INVALID", appErr.Code)
}

func TestOwnerAdmin_GrantCredits_AddsAddonTokens(t *testing.T) {
	uc, _, _, _, _, usageRepo, _ := makeOwnerUC(t)
	usageRepo.On("AddAddonTokens", mock.Anything, "acc-1", int64(5000)).Return(nil)
	err := uc.GrantCredits(context.Background(), "ops@x.com", "acc-1", 5000)
	assert.NoError(t, err)
	usageRepo.AssertCalled(t, "AddAddonTokens", mock.Anything, "acc-1", int64(5000))
}

func TestOwnerAdmin_ExtendTrial_RejectsZeroOrTooLong(t *testing.T) {
	uc, _, _, _, _, _, _ := makeOwnerUC(t)
	for _, days := range []int{0, -5, 366, 1000} {
		err := uc.ExtendTrial(context.Background(), "ops@x.com", "acc-1", days)
		assert.Error(t, err, "should reject %d days", days)
	}
}

func TestOwnerAdmin_ExtendTrial_PushesExpiresAt(t *testing.T) {
	uc, _, ownerRepo, _, _, _, _ := makeOwnerUC(t)
	ownerRepo.On("ExtendTrial", mock.Anything, "acc-1", mock.AnythingOfType("time.Time")).Return(nil)
	err := uc.ExtendTrial(context.Background(), "ops@x.com", "acc-1", 30)
	assert.NoError(t, err)
}

func TestOwnerAdmin_Impersonate_NoAdminUser(t *testing.T) {
	uc, accRepo, _, userRepo, _, _, _ := makeOwnerUC(t)
	accRepo.On("GetByID", mock.Anything, "acc-1").Return(&domain.Account{ID: "acc-1", Name: "X"}, nil)
	userRepo.On("GetAdminByAccountID", mock.Anything, "acc-1").Return(nil, nil)
	jwt := &stubJWT{tokenOut: "tok"}
	_, err := uc.Impersonate(context.Background(), jwt, "ops@x.com", "acc-1", 15)
	assert.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "OWNER_NO_ADMIN", appErr.Code)
}

func TestOwnerAdmin_Impersonate_StampsActorOnClaim(t *testing.T) {
	uc, accRepo, _, userRepo, _, _, _ := makeOwnerUC(t)
	role := domain.RoleAdmin
	accRepo.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1", Name: "Acme"}, nil)
	userRepo.On("GetAdminByAccountID", mock.Anything, "acc-1").
		Return(&domain.User{ID: "u-1", Name: "Maria", Email: "maria@acme.com", Role: &role, AccountID: "acc-1"}, nil)
	jwt := &stubJWT{tokenOut: "imp.jwt"}

	out, err := uc.Impersonate(context.Background(), jwt, "ops@x.com", "acc-1", 30)
	assert.NoError(t, err)
	assert.Equal(t, "imp.jwt", out.Token)
	assert.Equal(t, "Acme", out.AccountName)
	assert.Equal(t, "maria@acme.com", out.ImpersonatingUser)
	assert.Equal(t, "ops@x.com", jwt.lastClaims.ImpersonatingActor)
	assert.Equal(t, "u-1", jwt.lastClaims.UserID)
	assert.Equal(t, "acc-1", jwt.lastClaims.AccountID)
	assert.Equal(t, domain.RoleAdmin, jwt.lastClaims.Role)
}

func TestOwnerAdmin_Impersonate_ClampsMinutes(t *testing.T) {
	uc, accRepo, _, userRepo, _, _, _ := makeOwnerUC(t)
	role := domain.RoleAdmin
	accRepo.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1", Name: "X"}, nil)
	userRepo.On("GetAdminByAccountID", mock.Anything, "acc-1").
		Return(&domain.User{ID: "u-1", Email: "x@y.com", Role: &role}, nil)
	jwt := &stubJWT{tokenOut: "tok"}

	// 0 → defaults to 15
	_, err := uc.Impersonate(context.Background(), jwt, "ops@x.com", "acc-1", 0)
	assert.NoError(t, err)
	delta := jwt.lastClaims.ExpiresAt.Sub(time.Now())
	assert.True(t, delta > 14*time.Minute && delta <= 15*time.Minute, "default 15min")

	// 999 → clamped to 15 (anything > 60 also)
	_, err = uc.Impersonate(context.Background(), jwt, "ops@x.com", "acc-1", 999)
	assert.NoError(t, err)
	delta = jwt.lastClaims.ExpiresAt.Sub(time.Now())
	assert.True(t, delta > 14*time.Minute && delta <= 15*time.Minute, "999min clamped to 15")
}

func TestOwnerAdmin_CreateContract_RejectsBlankCompany(t *testing.T) {
	uc, _, _, _, _, _, _ := makeOwnerUC(t)
	_, err := uc.CreateContract(context.Background(), CreateContractDTO{
		ActorEmail: "ops@x.com",
		AdminEmail: "admin@acme.com",
		PlanID:     "plan-1",
	})
	assert.Error(t, err)
}

func TestOwnerAdmin_CreateContract_RejectsBadEmail(t *testing.T) {
	uc, _, _, _, _, _, _ := makeOwnerUC(t)
	_, err := uc.CreateContract(context.Background(), CreateContractDTO{
		ActorEmail:  "ops@x.com",
		CompanyName: "Acme",
		AdminEmail:  "not-an-email",
		PlanID:      "plan-1",
	})
	assert.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "OWNER_EMAIL_INVALID", appErr.Code)
}

func TestOwnerAdmin_CreateContract_RejectsExistingEmail(t *testing.T) {
	uc, _, _, userRepo, _, _, _ := makeOwnerUC(t)
	userRepo.On("GetByEmail", mock.Anything, "admin@acme.com").
		Return(&domain.User{ID: "u-1"}, nil)
	_, err := uc.CreateContract(context.Background(), CreateContractDTO{
		ActorEmail:  "ops@x.com",
		CompanyName: "Acme",
		AdminEmail:  "admin@acme.com",
		PlanID:      "plan-1",
	})
	assert.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "OWNER_EMAIL_EXISTS", appErr.Code)
}

func TestOwnerAdmin_ApplyContract_RejectsMissingAccount(t *testing.T) {
	uc, accRepo, _, _, _, _, _ := makeOwnerUC(t)
	accRepo.On("GetByID", mock.Anything, "acc-x").Return(nil, nil)
	_, err := uc.ApplyContract(context.Background(), ApplyContractDTO{
		ActorEmail: "ops@x.com",
		AccountID:  "acc-x",
		PlanID:     "plan-1",
	})
	assert.ErrorIs(t, err, domain.ErrAccountNotFound)
}

func TestOwnerAdmin_ApplyContract_RejectsUnknownPlan(t *testing.T) {
	uc, accRepo, _, _, planRepo, _, _ := makeOwnerUC(t)
	accRepo.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1", Name: "Acme"}, nil)
	planRepo.On("GetByID", mock.Anything, "ghost").Return(nil, nil)
	_, err := uc.ApplyContract(context.Background(), ApplyContractDTO{
		ActorEmail: "ops@x.com",
		AccountID:  "acc-1",
		PlanID:     "ghost",
	})
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "OWNER_PLAN_NOT_FOUND", appErr.Code)
}

func TestOwnerAdmin_ApplyContract_AppliesPlanBypassAndActiveStatus(t *testing.T) {
	uc, accRepo, _, _, planRepo, _, _ := makeOwnerUC(t)
	accRepo.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1", Name: "Acme"}, nil)
	planRepo.On("GetByID", mock.Anything, "plan-pro").
		Return(&domain.Plan{ID: "plan-pro", Name: "Pro"}, nil)
	accRepo.On("SetPlan", mock.Anything, "acc-1", "plan-pro").Return(nil)
	accRepo.On("SetLimitOverrides", mock.Anything, "acc-1", mock.Anything).Return(nil)
	accRepo.On("SetTrialBypass", mock.Anything, "acc-1", true, "B2B").Return(nil)
	accRepo.On("SetSubscriptionStatus", mock.Anything, "acc-1", domain.SubscriptionStatusActive).
		Return(nil)

	platformKey := true
	accRepo.On("SetPlatformKeyAllowed", mock.Anything, "acc-1", true).Return(nil)

	out, err := uc.ApplyContract(context.Background(), ApplyContractDTO{
		ActorEmail:         "ops@x.com",
		AccountID:          "acc-1",
		PlanID:             "plan-pro",
		TrialBypass:        true,
		BypassReason:       "B2B",
		PlatformKeyAllowed: &platformKey,
	})
	assert.NoError(t, err)
	assert.NotNil(t, out)
	accRepo.AssertCalled(t, "SetPlan", mock.Anything, "acc-1", "plan-pro")
	accRepo.AssertCalled(t, "SetSubscriptionStatus", mock.Anything, "acc-1",
		domain.SubscriptionStatusActive)
	accRepo.AssertCalled(t, "SetPlatformKeyAllowed", mock.Anything, "acc-1", true)
}

// stubStripeLister captures the (customer, limit) pair the UC passes
// to the adapter, so tests can assert wiring without reaching Stripe.
type stubStripeLister struct {
	lastCustomer string
	lastLimit    int64
	out          []domain.StripeInvoiceSummary
	err          error
}

func (s *stubStripeLister) ListInvoicesForCustomer(customerID string, limit int64) ([]domain.StripeInvoiceSummary, error) {
	s.lastCustomer = customerID
	s.lastLimit = limit
	return s.out, s.err
}

func TestOwnerAdmin_ListInvoices_NotConfigured(t *testing.T) {
	uc, _, _, _, _, _, _ := makeOwnerUC(t)
	_, err := uc.ListInvoices(context.Background(), "acc-1", 10)
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "STRIPE_NOT_CONFIGURED", appErr.Code)
}

func TestOwnerAdmin_ListInvoices_NoCustomerReturnsEmpty(t *testing.T) {
	uc, accRepo, _, _, _, _, _ := makeOwnerUC(t)
	stub := &stubStripeLister{}
	uc.SetStripeLister(stub)
	accRepo.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1"}, nil)

	out, err := uc.ListInvoices(context.Background(), "acc-1", 10)
	assert.NoError(t, err)
	assert.Empty(t, out)
	assert.Equal(t, "", stub.lastCustomer, "adapter must not be called without a customer id")
}

func TestOwnerAdmin_ListInvoices_DelegatesWithCustomer(t *testing.T) {
	uc, accRepo, _, _, _, _, _ := makeOwnerUC(t)
	cus := "cus_123"
	stub := &stubStripeLister{
		out: []domain.StripeInvoiceSummary{{ID: "in_1", Status: "paid", AmountPaid: 1000}},
	}
	uc.SetStripeLister(stub)
	accRepo.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1", StripeCustomerID: &cus}, nil)

	out, err := uc.ListInvoices(context.Background(), "acc-1", 10)
	assert.NoError(t, err)
	assert.Len(t, out, 1)
	assert.Equal(t, "in_1", out[0].ID)
	assert.Equal(t, "cus_123", stub.lastCustomer)
	assert.Equal(t, int64(10), stub.lastLimit)
}

// --- SetSeller (vendedores v3, US-002) ---

func sellerFixtures() (*domain.Account, *domain.User) {
	acc := &domain.Account{ID: "acc-1", Name: "Cliente X"}
	seller := &domain.User{
		ID:        "usr-seller",
		AccountID: "acc-platform",
		Name:      "Vendedor",
		Email:     "v@replider.com",
		IsActive:  true,
	}
	return acc, seller
}

func TestOwnerAdmin_SetSeller_LinksSellerRef(t *testing.T) {
	uc, accRepo, ownerRepo, userRepo, _, _, auditRepo := makeOwnerUC(t)
	uc.SetPlatformAccountID("acc-platform")
	acc, seller := sellerFixtures()
	accRepo.On("GetByID", mock.Anything, "acc-1").Return(acc, nil)
	userRepo.On("GetByEmail", mock.Anything, "v@replider.com").Return(seller, nil)
	userRepo.On("GetAdminByAccountID", mock.Anything, "acc-1").Return(nil, nil)
	ownerRepo.On("SetSeller", mock.Anything, "acc-1", "usr-seller", domain.LeadSourceSellerRef).Return(nil)

	out, err := uc.SetSeller(context.Background(), "ops@x.com", "acc-1", " V@Replider.com ", domain.LeadSourceSellerRef)
	assert.NoError(t, err)
	assert.NotNil(t, out)
	ownerRepo.AssertCalled(t, "SetSeller", mock.Anything, "acc-1", "usr-seller", "seller_ref")
	auditRepo.AssertCalled(t, "Create", mock.Anything, mock.Anything)
}

// US-013: linking a seller who is the account's own admin is an explicit
// owner override — allowed (the write still happens), but signaled via a
// WARN log + the seller_self_match flag on the detail payload.
func TestOwnerAdmin_SetSeller_SelfMatchStillLinks(t *testing.T) {
	uc, accRepo, ownerRepo, userRepo, _, _, _ := makeOwnerUC(t)
	uc.SetPlatformAccountID("acc-platform")
	acc, seller := sellerFixtures()
	admin := &domain.User{ID: "usr-admin", AccountID: "acc-1", Email: seller.Email}
	accRepo.On("GetByID", mock.Anything, "acc-1").Return(acc, nil)
	userRepo.On("GetByEmail", mock.Anything, "v@replider.com").Return(seller, nil)
	userRepo.On("GetAdminByAccountID", mock.Anything, "acc-1").Return(admin, nil)
	ownerRepo.On("SetSeller", mock.Anything, "acc-1", "usr-seller", domain.LeadSourceSellerRef).Return(nil)

	_, err := uc.SetSeller(context.Background(), "ops@x.com", "acc-1", "v@replider.com", domain.LeadSourceSellerRef)
	assert.NoError(t, err)
	ownerRepo.AssertCalled(t, "SetSeller", mock.Anything, "acc-1", "usr-seller", "seller_ref")
}

func TestOwnerAdmin_SetSeller_UnlinkOnEmptyEmail(t *testing.T) {
	uc, accRepo, ownerRepo, _, _, _, _ := makeOwnerUC(t)
	// Deliberately NO platform account configured — unlink must still work.
	prevSeller := "usr-old"
	acc, _ := sellerFixtures()
	acc.SellerUserID = &prevSeller
	accRepo.On("GetByID", mock.Anything, "acc-1").Return(acc, nil)
	ownerRepo.On("ClearSeller", mock.Anything, "acc-1").Return(nil)

	_, err := uc.SetSeller(context.Background(), "ops@x.com", "acc-1", "", "")
	assert.NoError(t, err)
	ownerRepo.AssertCalled(t, "ClearSeller", mock.Anything, "acc-1")
}

// mockCommissionFlagger stubs the US-008 unlink reconciliation seam.
type mockCommissionFlagger struct {
	mock.Mock
}

func (m *mockCommissionFlagger) FlagOrphanedPending(ctx context.Context, accountID string) (int64, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(int64), args.Error(1)
}

// US-008 review fix: unlinking a seller flags the account's pending
// ledger rows for reconciliation BEFORE clearing the link — orphan
// pendencias must never be swept silently into the next payout.
func TestOwnerAdmin_SetSeller_UnlinkFlagsOrphanedPending(t *testing.T) {
	uc, accRepo, ownerRepo, _, _, _, _ := makeOwnerUC(t)
	prevSeller := "usr-old"
	acc, _ := sellerFixtures()
	acc.SellerUserID = &prevSeller
	accRepo.On("GetByID", mock.Anything, "acc-1").Return(acc, nil)
	ownerRepo.On("ClearSeller", mock.Anything, "acc-1").Return(nil)

	flagger := new(mockCommissionFlagger)
	flagger.On("FlagOrphanedPending", mock.Anything, "acc-1").Return(int64(2), nil)
	uc.SetCommissionFlagger(flagger)

	_, err := uc.SetSeller(context.Background(), "ops@x.com", "acc-1", "", "")
	assert.NoError(t, err)
	flagger.AssertCalled(t, "FlagOrphanedPending", mock.Anything, "acc-1")
	ownerRepo.AssertCalled(t, "ClearSeller", mock.Anything, "acc-1")
}

// A flagging failure aborts the unlink — the link and the flag can't
// diverge (an unflagged orphan is exactly the silent-settle bug).
func TestOwnerAdmin_SetSeller_UnlinkAbortsWhenFlaggingFails(t *testing.T) {
	uc, accRepo, ownerRepo, _, _, _, _ := makeOwnerUC(t)
	prevSeller := "usr-old"
	acc, _ := sellerFixtures()
	acc.SellerUserID = &prevSeller
	accRepo.On("GetByID", mock.Anything, "acc-1").Return(acc, nil)

	flagger := new(mockCommissionFlagger)
	flagger.On("FlagOrphanedPending", mock.Anything, "acc-1").Return(int64(0), errors.New("db down"))
	uc.SetCommissionFlagger(flagger)

	_, err := uc.SetSeller(context.Background(), "ops@x.com", "acc-1", "", "")
	assert.Error(t, err)
	ownerRepo.AssertNotCalled(t, "ClearSeller", mock.Anything, mock.Anything)
}

func TestOwnerAdmin_SetSeller_PlatformNotConfigured(t *testing.T) {
	uc, accRepo, _, _, _, _, _ := makeOwnerUC(t)
	acc, _ := sellerFixtures()
	accRepo.On("GetByID", mock.Anything, "acc-1").Return(acc, nil)

	_, err := uc.SetSeller(context.Background(), "ops@x.com", "acc-1", "v@replider.com", domain.LeadSourceInbound)
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "OWNER_PLATFORM_NOT_CONFIGURED", appErr.Code)
}

func TestOwnerAdmin_SetSeller_RejectsInvalidLeadSource(t *testing.T) {
	uc, accRepo, _, _, _, _, _ := makeOwnerUC(t)
	uc.SetPlatformAccountID("acc-platform")
	acc, _ := sellerFixtures()
	accRepo.On("GetByID", mock.Anything, "acc-1").Return(acc, nil)

	_, err := uc.SetSeller(context.Background(), "ops@x.com", "acc-1", "v@replider.com", "bogus")
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "OWNER_INVALID_LEAD_SOURCE", appErr.Code)
}

func TestOwnerAdmin_SetSeller_RejectsNonPlatformUser(t *testing.T) {
	uc, accRepo, _, userRepo, _, _, _ := makeOwnerUC(t)
	uc.SetPlatformAccountID("acc-platform")
	acc, seller := sellerFixtures()
	seller.AccountID = "acc-other" // customer-account user, not a seller
	accRepo.On("GetByID", mock.Anything, "acc-1").Return(acc, nil)
	userRepo.On("GetByEmail", mock.Anything, "v@replider.com").Return(seller, nil)

	_, err := uc.SetSeller(context.Background(), "ops@x.com", "acc-1", "v@replider.com", domain.LeadSourceInbound)
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "OWNER_SELLER_NOT_FOUND", appErr.Code)
}

func TestOwnerAdmin_SetSeller_RejectsInactiveSeller(t *testing.T) {
	uc, accRepo, _, userRepo, _, _, _ := makeOwnerUC(t)
	uc.SetPlatformAccountID("acc-platform")
	acc, seller := sellerFixtures()
	seller.IsActive = false
	accRepo.On("GetByID", mock.Anything, "acc-1").Return(acc, nil)
	userRepo.On("GetByEmail", mock.Anything, "v@replider.com").Return(seller, nil)

	_, err := uc.SetSeller(context.Background(), "ops@x.com", "acc-1", "v@replider.com", domain.LeadSourceInbound)
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "OWNER_SELLER_INACTIVE", appErr.Code)
}

func TestOwnerAdmin_SetSeller_AccountNotFound(t *testing.T) {
	uc, accRepo, _, _, _, _, _ := makeOwnerUC(t)
	uc.SetPlatformAccountID("acc-platform")
	accRepo.On("GetByID", mock.Anything, "acc-missing").Return(nil, nil)

	_, err := uc.SetSeller(context.Background(), "ops@x.com", "acc-missing", "v@replider.com", domain.LeadSourceInbound)
	assert.Equal(t, domain.ErrAccountNotFound, err)
}

func TestOwnerAdmin_Get_IncludesSeller(t *testing.T) {
	uc, accRepo, ownerRepo, userRepo, _, usageRepo, _ := makeOwnerUC(t)
	acc, seller := sellerFixtures()
	acc.SellerUserID = &seller.ID
	accRepo.On("GetByID", mock.Anything, "acc-1").Return(acc, nil)
	usageRepo.On("GetByAccountID", mock.Anything, "acc-1").Return(nil, nil)
	userRepo.On("GetAdminByAccountID", mock.Anything, "acc-1").Return(nil, nil)
	userRepo.On("GetByID", mock.Anything, "usr-seller").Return(seller, nil)
	ownerRepo.On("CountActiveInternalThreads", mock.Anything, "acc-1").Return(0, nil)

	out, err := uc.Get(context.Background(), "acc-1")
	assert.NoError(t, err)
	assert.NotNil(t, out.Seller)
	assert.Equal(t, "usr-seller", out.Seller.ID)
	assert.False(t, out.SellerSelfMatch)
}

// US-013: the detail payload flags a seller whose email matches the
// account's own admin (a match established AFTER signup — email change or
// manual owner link) so the owner panel can surface it.
func TestOwnerAdmin_Get_FlagsSellerSelfMatch(t *testing.T) {
	uc, accRepo, ownerRepo, userRepo, _, usageRepo, _ := makeOwnerUC(t)
	acc, seller := sellerFixtures()
	acc.SellerUserID = &seller.ID
	admin := &domain.User{ID: "usr-admin", AccountID: "acc-1", Email: "V@Replider.com"}
	accRepo.On("GetByID", mock.Anything, "acc-1").Return(acc, nil)
	usageRepo.On("GetByAccountID", mock.Anything, "acc-1").Return(nil, nil)
	userRepo.On("GetAdminByAccountID", mock.Anything, "acc-1").Return(admin, nil)
	userRepo.On("GetByID", mock.Anything, "usr-seller").Return(seller, nil)
	ownerRepo.On("CountActiveInternalThreads", mock.Anything, "acc-1").Return(0, nil)

	out, err := uc.Get(context.Background(), "acc-1")
	assert.NoError(t, err)
	assert.True(t, out.SellerSelfMatch)
}

// Canal-interno US-010: the detail payload carries the count of OPEN
// internal CS threads; a count failure degrades to 0, never errors the
// whole detail.
func TestOwnerAdmin_Get_IncludesInternalThreadCount(t *testing.T) {
	uc, accRepo, ownerRepo, userRepo, _, usageRepo, _ := makeOwnerUC(t)
	accRepo.On("GetByID", mock.Anything, "acc-1").Return(&domain.Account{ID: "acc-1"}, nil)
	usageRepo.On("GetByAccountID", mock.Anything, "acc-1").Return(nil, nil)
	userRepo.On("GetAdminByAccountID", mock.Anything, "acc-1").Return(nil, nil)
	ownerRepo.On("CountActiveInternalThreads", mock.Anything, "acc-1").Return(3, nil)

	out, err := uc.Get(context.Background(), "acc-1")
	assert.NoError(t, err)
	assert.Equal(t, int64(3), out.InternalThreadsActive)
}

func TestOwnerAdmin_Get_InternalThreadCountFailureDegrades(t *testing.T) {
	uc, accRepo, ownerRepo, userRepo, _, usageRepo, _ := makeOwnerUC(t)
	accRepo.On("GetByID", mock.Anything, "acc-1").Return(&domain.Account{ID: "acc-1"}, nil)
	usageRepo.On("GetByAccountID", mock.Anything, "acc-1").Return(nil, nil)
	userRepo.On("GetAdminByAccountID", mock.Anything, "acc-1").Return(nil, nil)
	ownerRepo.On("CountActiveInternalThreads", mock.Anything, "acc-1").Return(0, assert.AnError)

	out, err := uc.Get(context.Background(), "acc-1")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), out.InternalThreadsActive)
}

func TestOwnerAdmin_ApplyContract_LedgersPlanUpgrade(t *testing.T) {
	uc, accRepo, _, _, planRepo, _, _ := makeOwnerUC(t)
	billing := new(mockBillingEventRepo)
	uc.SetBillingEvents(billing)

	oldPlanID := "plan-core"
	accRepo.On("GetByID", mock.Anything, "acc-1").
		Return(&domain.Account{ID: "acc-1", Name: "Acme", PlanID: &oldPlanID}, nil)
	planRepo.On("GetByID", mock.Anything, "plan-pro").
		Return(&domain.Plan{ID: "plan-pro", Name: "Pro", Currency: "brl", Price: 89900}, nil)
	planRepo.On("GetByID", mock.Anything, "plan-core").
		Return(&domain.Plan{ID: "plan-core", Name: "Core", Currency: "brl", Price: 39900}, nil)
	accRepo.On("SetPlan", mock.Anything, "acc-1", "plan-pro").Return(nil)
	accRepo.On("SetLimitOverrides", mock.Anything, "acc-1", mock.Anything).Return(nil)
	accRepo.On("SetTrialBypass", mock.Anything, "acc-1", false, "").Return(nil)
	accRepo.On("SetSubscriptionStatus", mock.Anything, "acc-1", domain.SubscriptionStatusActive).Return(nil)
	billing.On("Create", mock.Anything, mock.MatchedBy(func(e *domain.BillingEvent) bool {
		return e.EventType == domain.BillingEventPlanUpgraded &&
			e.AccountID == "acc-1" && e.Plan == "Pro" && e.AmountCents == 89900
	})).Return(nil)

	_, err := uc.ApplyContract(context.Background(), ApplyContractDTO{
		ActorEmail: "ops@x.com", AccountID: "acc-1", PlanID: "plan-pro",
	})
	assert.NoError(t, err)
	billing.AssertNumberOfCalls(t, "Create", 1)
}

func TestOwnerAdmin_ApplyContract_NoUpgradeEventFromTrialOrDowngrade(t *testing.T) {
	cases := []struct {
		name     string
		oldPlan  *domain.Plan
		newPrice int
	}{
		// Trial (price 0) -> paid is a conversion, not an upgrade — it
		// must NOT flip an inbound closing onto the 50% track.
		{"trial to paid", &domain.Plan{ID: "plan-trial", Name: "Trial", Currency: "brl", Price: 0}, 89900},
		// Cheaper plan = downgrade.
		{"downgrade", &domain.Plan{ID: "plan-old", Name: "Scale", Currency: "brl", Price: 399000}, 89900},
		// Cross-currency prices aren't comparable.
		{"cross currency", &domain.Plan{ID: "plan-old", Name: "Solo USD", Currency: "usd", Price: 3900}, 89900},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc, accRepo, _, _, planRepo, _, _ := makeOwnerUC(t)
			billing := new(mockBillingEventRepo)
			uc.SetBillingEvents(billing)

			accRepo.On("GetByID", mock.Anything, "acc-1").
				Return(&domain.Account{ID: "acc-1", Name: "Acme", PlanID: &tc.oldPlan.ID}, nil)
			planRepo.On("GetByID", mock.Anything, "plan-new").
				Return(&domain.Plan{ID: "plan-new", Name: "Pro", Currency: "brl", Price: tc.newPrice}, nil)
			planRepo.On("GetByID", mock.Anything, tc.oldPlan.ID).Return(tc.oldPlan, nil)
			accRepo.On("SetPlan", mock.Anything, "acc-1", "plan-new").Return(nil)
			accRepo.On("SetLimitOverrides", mock.Anything, "acc-1", mock.Anything).Return(nil)
			accRepo.On("SetTrialBypass", mock.Anything, "acc-1", false, "").Return(nil)
			accRepo.On("SetSubscriptionStatus", mock.Anything, "acc-1", domain.SubscriptionStatusActive).Return(nil)

			_, err := uc.ApplyContract(context.Background(), ApplyContractDTO{
				ActorEmail: "ops@x.com", AccountID: "acc-1", PlanID: "plan-new",
			})
			assert.NoError(t, err)
			billing.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
		})
	}
}

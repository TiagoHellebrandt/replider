package usecases

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// StripeInvoiceLister is the slim port the use case needs from the
// Stripe adapter. Kept here so core/ doesn't import the payment
// adapter package directly — main.go wires the concrete implementation
// from internal/adapters/payment. The returned shape lives in domain
// so adapter + use case share it without a cyclic import.
type StripeInvoiceLister interface {
	ListInvoicesForCustomer(customerID string, limit int64) ([]domain.StripeInvoiceSummary, error)
}

// OwnerAdminUseCase bundles the platform-level (super-admin) operations
// the /owner UI calls into. Plan-aware logic stays out — these methods
// presume the caller has already passed the SuperAdmin middleware.
type OwnerAdminUseCase struct {
	accountRepo  domain.AccountRepository
	ownerRepo    domain.OwnerAccountRepository
	userRepo     domain.UserRepository
	planRepo     domain.PlanRepository
	usageRepo    domain.UsageRepository
	auditRepo    domain.AuditLogRepository
	stripeLister StripeInvoiceLister
	// billingEvents, when wired, ledgers a 'plan_upgraded' event on
	// apply-contract tier increases (US-008 — the commission recompute
	// reads it to grant the upgrade bonus). Optional and best-effort.
	billingEvents domain.BillingEventRepository
	// platformAccountID gates seller-link operations (vendedores v3):
	// sellers are users OF this account. Empty = feature off, SetSeller
	// refuses with OWNER_PLATFORM_NOT_CONFIGURED.
	platformAccountID string
	// commissionFlagger, when wired, stamps an account's pending
	// commission_ledger rows for reconciliation on seller unlink
	// (US-008 review fix) — orphan pendências must be flagged, never
	// silently swept into the next payout.
	commissionFlagger orphanedCommissionFlagger
	// funnelRepo — see ensureDefaultFunnel. Optional.
	funnelRepo domain.SalesFunnelRepository
}

// SetFunnelRepo wires default-funnel creation into contract accounts.
func (uc *OwnerAdminUseCase) SetFunnelRepo(r domain.SalesFunnelRepository) { uc.funnelRepo = r }

// orphanedCommissionFlagger is the slim slice of the commission repo
// the unlink path needs (narrow local interface — avoids widening
// domain.CommissionRepository and its mocks).
type orphanedCommissionFlagger interface {
	FlagOrphanedPending(ctx context.Context, accountID string) (int64, error)
}

func NewOwnerAdminUseCase(
	accountRepo domain.AccountRepository,
	ownerRepo domain.OwnerAccountRepository,
	userRepo domain.UserRepository,
	planRepo domain.PlanRepository,
	usageRepo domain.UsageRepository,
	auditRepo domain.AuditLogRepository,
) *OwnerAdminUseCase {
	return &OwnerAdminUseCase{
		accountRepo: accountRepo,
		ownerRepo:   ownerRepo,
		userRepo:    userRepo,
		planRepo:    planRepo,
		usageRepo:   usageRepo,
		auditRepo:   auditRepo,
	}
}

// SetStripeLister wires the optional Stripe-invoice port. Owner panel
// invoice endpoint returns 503 when this is nil so deploys without
// Stripe configured don't crash on the route.
func (uc *OwnerAdminUseCase) SetStripeLister(s StripeInvoiceLister) {
	uc.stripeLister = s
}

// SetPlatformAccountID wires the PLATFORM_ACCOUNT_ID config (setter
// pattern keeps the constructor stable for existing call sites).
func (uc *OwnerAdminUseCase) SetPlatformAccountID(id string) {
	uc.platformAccountID = id
}

// SetBillingEvents wires the optional revenue ledger so apply-contract
// upgrades feed the US-008 upgrade bonus. nil skips the recording.
func (uc *OwnerAdminUseCase) SetBillingEvents(r domain.BillingEventRepository) {
	uc.billingEvents = r
}

// SetCommissionFlagger wires the ledger flagging the unlink path runs
// (US-008 review fix). Optional — nil skips the flagging.
func (uc *OwnerAdminUseCase) SetCommissionFlagger(f orphanedCommissionFlagger) {
	uc.commissionFlagger = f
}

// ListInvoices returns the customer's last N Stripe invoices via the
// configured adapter. Empty (not error) when the account has no Stripe
// customer id (contract plans / pre-checkout).
func (uc *OwnerAdminUseCase) ListInvoices(ctx context.Context, accountID string, limit int) ([]domain.StripeInvoiceSummary, error) {
	if uc.stripeLister == nil {
		return nil, &domain.AppError{Code: "STRIPE_NOT_CONFIGURED", Message: "Stripe is not configured on this deployment"}
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	acc, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return nil, domain.ErrAccountNotFound
	}
	if acc.StripeCustomerID == nil || *acc.StripeCustomerID == "" {
		return []domain.StripeInvoiceSummary{}, nil
	}
	return uc.stripeLister.ListInvoicesForCustomer(*acc.StripeCustomerID, int64(limit))
}

// AccountDetail is the rich payload the owner-panel detail screen
// renders. Bundles the account row + resolved plan + current usage so
// the client doesn't have to fan out three GETs.
type AccountDetail struct {
	Account *domain.Account      `json:"account"`
	Plan    *domain.Plan         `json:"plan,omitempty"`
	Usage   *domain.AccountUsage `json:"usage,omitempty"`
	Admin   *domain.User         `json:"admin,omitempty"`
	// Seller is the platform-account user this customer is linked to
	// (vendedores v3). nil when unmanaged or the user row is gone.
	Seller *domain.User `json:"seller,omitempty"`
	// SellerSelfMatch flags that the linked seller looks like the
	// account's own admin (same user id or same email) — a seller must
	// not be the AM of their own account (US-013). Signup blocks this
	// up-front; this flag surfaces matches established later (email
	// change, manual owner link) so the owner panel can flag them.
	SellerSelfMatch bool `json:"seller_self_match,omitempty"`
	// InternalThreadsActive is how many OPEN internal CS threads this
	// account's users hold with the platform (canal-interno US-010).
	// Best-effort: a count failure renders as 0, never blocks the detail.
	InternalThreadsActive int64 `json:"internal_threads_active"`
}

// List paginates through every account on the platform. Empty search
// returns all rows; status filters to active/suspended/trialing/
// past_due when non-empty (empty returns all statuses).
func (uc *OwnerAdminUseCase) List(ctx context.Context, search, status string, limit, offset int) ([]*domain.AccountListing, int64, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := uc.ownerRepo.ListForOwner(ctx, search, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := uc.ownerRepo.CountForOwner(ctx, search, status)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// Get returns a full detail bundle for the owner-panel detail screen.
func (uc *OwnerAdminUseCase) Get(ctx context.Context, accountID string) (*AccountDetail, error) {
	acc, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return nil, domain.ErrAccountNotFound
	}
	out := &AccountDetail{Account: acc}
	if acc.PlanID != nil {
		if p, err := uc.planRepo.GetByID(ctx, *acc.PlanID); err == nil && p != nil {
			out.Plan = p
		}
	}
	if u, err := uc.usageRepo.GetByAccountID(ctx, accountID); err == nil && u != nil {
		out.Usage = u
	}
	if admin, err := uc.userRepo.GetAdminByAccountID(ctx, accountID); err == nil && admin != nil {
		out.Admin = admin
	}
	if acc.SellerUserID != nil {
		if seller, err := uc.userRepo.GetByID(ctx, *acc.SellerUserID); err == nil && seller != nil {
			out.Seller = seller
		}
	}
	if out.Seller != nil && out.Admin != nil {
		out.SellerSelfMatch = isSellerSelfReferral(out.Seller.ID, out.Seller.Email, out.Admin.ID, out.Admin.Email)
	}
	if n, err := uc.ownerRepo.CountActiveInternalThreads(ctx, accountID); err == nil {
		out.InternalThreadsActive = n
	} else {
		slog.Warn("owner: internal thread count failed", "account_id", accountID, "error", err)
	}
	return out, nil
}

// SetSeller links, reallocates or (with an empty sellerEmail) unlinks
// the seller of a customer account — the owner-panel manual override
// behind carteira attribution. Linking validates the target user is an
// ACTIVE member of the platform account and stamps the lead source that
// decides the commission track. Every mutation is audited (audit_logs)
// and logged structurally.
func (uc *OwnerAdminUseCase) SetSeller(ctx context.Context, actorEmail, accountID, sellerEmail, leadSource string) (*domain.Account, error) {
	acc, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return nil, domain.ErrAccountNotFound
	}

	sellerEmail = strings.ToLower(strings.TrimSpace(sellerEmail))
	if sellerEmail == "" {
		// Unlink path — valid even with PLATFORM_ACCOUNT_ID unset so a
		// stale link can always be cleaned up.
		//
		// US-008 review fix: pending ledger rows the recompute will no
		// longer manage (ListCommissionBases only sees linked accounts)
		// get flagged for owner reconciliation BEFORE the unlink — a
		// payout sweep must never settle them silently. Flagging failure
		// aborts the unlink so the two can't diverge.
		var flagged int64
		if uc.commissionFlagger != nil {
			var err error
			flagged, err = uc.commissionFlagger.FlagOrphanedPending(ctx, accountID)
			if err != nil {
				return nil, err
			}
		}
		if err := uc.ownerRepo.ClearSeller(ctx, accountID); err != nil {
			return nil, err
		}
		uc.audit(ctx, actorEmail, accountID, "owner.account.seller_unlink")
		if flagged > 0 {
			slog.Warn("owner: seller unlink left pending commission flagged for reconciliation",
				"actor", actorEmail, "account_id", accountID,
				"previous_seller_user_id", derefOrEmpty(acc.SellerUserID),
				"flagged_pending_entries", flagged)
		}
		slog.Info("owner: seller unlinked",
			"actor", actorEmail, "account_id", accountID,
			"previous_seller_user_id", derefOrEmpty(acc.SellerUserID),
			"flagged_pending_entries", flagged)
		return uc.accountRepo.GetByID(ctx, accountID)
	}

	if uc.platformAccountID == "" {
		return nil, &domain.AppError{Code: "OWNER_PLATFORM_NOT_CONFIGURED", Message: "PLATFORM_ACCOUNT_ID is not configured on this deployment"}
	}
	if !domain.IsValidLeadSource(leadSource) {
		return nil, &domain.AppError{Code: "OWNER_INVALID_LEAD_SOURCE", Message: "lead_source must be 'seller_ref' or 'inbound'"}
	}
	seller, err := uc.userRepo.GetByEmail(ctx, sellerEmail)
	if err != nil {
		return nil, err
	}
	if seller == nil || seller.AccountID != uc.platformAccountID {
		// Same code for "no such user" and "user of another account" —
		// the owner panel message is identical and we don't leak which.
		return nil, &domain.AppError{Code: "OWNER_SELLER_NOT_FOUND", Message: "no active seller with this email on the platform account"}
	}
	if !seller.IsActive {
		return nil, &domain.AppError{Code: "OWNER_SELLER_INACTIVE", Message: "seller user is deactivated"}
	}

	// US-013: a seller must not be the AM of their own account. The owner
	// link is a deliberate manual override, so it's allowed — but the
	// match is signaled (log here + seller_self_match on the detail
	// payload) so it never slips through silently.
	if admin, aerr := uc.userRepo.GetAdminByAccountID(ctx, accountID); aerr == nil && admin != nil &&
		isSellerSelfReferral(seller.ID, seller.Email, admin.ID, admin.Email) {
		slog.Warn("owner: linked seller matches the account's own admin (self-referral)",
			"actor", actorEmail, "account_id", accountID,
			"seller_user_id", seller.ID)
	}

	if err := uc.ownerRepo.SetSeller(ctx, accountID, seller.ID, leadSource); err != nil {
		return nil, err
	}
	uc.audit(ctx, actorEmail, accountID, "owner.account.seller_link")
	slog.Info("owner: seller linked",
		"actor", actorEmail, "account_id", accountID,
		"seller_user_id", seller.ID, "seller_email", seller.Email,
		"lead_source", leadSource,
		"previous_seller_user_id", derefOrEmpty(acc.SellerUserID))
	return uc.accountRepo.GetByID(ctx, accountID)
}

// SetOverrides records per-account caps. Pass nil pointers to clear a
// dimension (resets to plan default).
func (uc *OwnerAdminUseCase) SetOverrides(ctx context.Context, actorEmail, accountID string, ov domain.AccountLimitOverrides) error {
	if err := uc.accountRepo.SetLimitOverrides(ctx, accountID, ov); err != nil {
		return err
	}
	uc.audit(ctx, actorEmail, accountID, "owner.account.set_overrides")
	return nil
}

// Suspend freezes the account; reason is required so the audit log
// captures the why and the dashboard banner has copy to surface.
func (uc *OwnerAdminUseCase) Suspend(ctx context.Context, actorEmail, accountID, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return &domain.AppError{Code: "OWNER_SUSPEND_REASON_REQUIRED", Message: "reason is required"}
	}
	if err := uc.accountRepo.Suspend(ctx, accountID, reason); err != nil {
		return err
	}
	uc.audit(ctx, actorEmail, accountID, "owner.account.suspend")
	return nil
}

// Reactivate clears the suspend flags.
func (uc *OwnerAdminUseCase) Reactivate(ctx context.Context, actorEmail, accountID string) error {
	if err := uc.accountRepo.Reactivate(ctx, accountID); err != nil {
		return err
	}
	uc.audit(ctx, actorEmail, accountID, "owner.account.reactivate")
	return nil
}

// SetSubscriptionStatus force-overrides the Stripe-managed status.
// Used to flag a contract account as 'active' even though no Stripe
// subscription is backing it.
func (uc *OwnerAdminUseCase) SetSubscriptionStatus(ctx context.Context, actorEmail, accountID, status string) error {
	switch status {
	case domain.SubscriptionStatusActive,
		domain.SubscriptionStatusTrialing,
		domain.SubscriptionStatusPastDue,
		domain.SubscriptionStatusCanceled:
	default:
		return &domain.AppError{Code: "OWNER_INVALID_STATUS", Message: "invalid subscription status"}
	}
	if err := uc.accountRepo.SetSubscriptionStatus(ctx, accountID, status); err != nil {
		return err
	}
	uc.audit(ctx, actorEmail, accountID, "owner.account.set_status")
	return nil
}

// SetCurrency corrects an account's billing currency — the owner-panel
// escape hatch for locale-based currency arbitrage (US-015 review
// fixes): signup derives currency from the client-sent locale, so a
// forced "en" locale buys the USD ladder. The signup_ip/signup_country
// audit columns surface the mismatch; this fixes it. Refused once an
// active Stripe subscription exists — that subscription bills in the
// old currency and must be cancelled first.
func (uc *OwnerAdminUseCase) SetCurrency(ctx context.Context, actorEmail, accountID, currency string) error {
	currency = strings.ToLower(strings.TrimSpace(currency))
	if currency != domain.CurrencyBRL && currency != domain.CurrencyUSD {
		return &domain.AppError{Code: "OWNER_INVALID_CURRENCY", Message: "currency must be 'brl' or 'usd'"}
	}
	acc, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return err
	}
	if acc == nil {
		return domain.ErrAccountNotFound
	}
	if acc.Currency == currency {
		return nil // idempotent no-op, no audit noise
	}
	if acc.StripeSubscriptionID != nil && *acc.StripeSubscriptionID != "" {
		return &domain.AppError{
			Code:    "OWNER_CURRENCY_HAS_SUBSCRIPTION",
			Message: "account has an active Stripe subscription billing in the current currency — cancel it before changing the currency",
		}
	}
	if err := uc.ownerRepo.SetCurrency(ctx, accountID, currency); err != nil {
		return err
	}
	uc.audit(ctx, actorEmail, accountID, "owner.account.set_currency")
	slog.Info("owner corrected account currency",
		"actor", actorEmail, "account_id", accountID, "from", acc.Currency, "to", currency)
	return nil
}

// TopAccounts returns the heaviest-spending accounts this billing
// period, capped at `limit`. Used by the owner-panel "top spenders"
// widget to spot churn risks early.
func (uc *OwnerAdminUseCase) TopAccounts(ctx context.Context, limit int) ([]*domain.AccountTokenUsage, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	return uc.ownerRepo.TopAccountsByTokens(ctx, limit)
}

// SystemHealth aggregates platform-wide counts for the owner-panel
// "system" widget. Cheap enough to call on every panel open.
func (uc *OwnerAdminUseCase) SystemHealth(ctx context.Context) (*domain.SystemHealthSummary, error) {
	return uc.ownerRepo.SystemHealth(ctx)
}

// UsageTrend returns the 30-day daily token + conversation rollup for
// a target account — drives the owner-panel mini-chart.
func (uc *OwnerAdminUseCase) UsageTrend(ctx context.Context, accountID string) ([]*domain.UsageTrendDay, error) {
	return uc.ownerRepo.UsageTrend(ctx, accountID)
}

// ListConversations returns the most recent conversations for a
// target account. Read-only support tool — only the triage row, no
// message bodies. Operators tap a row to drill in via impersonation.
func (uc *OwnerAdminUseCase) ListConversations(ctx context.Context, accountID string, limit, offset int) ([]*domain.OwnerConversationRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return uc.ownerRepo.ListConversationsForOwner(ctx, accountID, limit, offset)
}

// GrantCredits adds N tokens to the account's addon bucket — bypasses
// the regular plan cap and shows up as `addon_tokens` on the next
// usage read. Used as a goodwill gesture when AI credits are eaten by
// a bug, or to extend a stuck contract while billing catches up.
func (uc *OwnerAdminUseCase) GrantCredits(ctx context.Context, actorEmail, accountID string, tokens int64) error {
	if tokens <= 0 {
		return &domain.AppError{Code: "OWNER_GRANT_INVALID", Message: "tokens must be positive"}
	}
	if err := uc.usageRepo.AddAddonTokens(ctx, accountID, tokens); err != nil {
		return err
	}
	uc.audit(ctx, actorEmail, accountID, "owner.account.grant_credits")
	return nil
}

// ExtendTrial pushes the trial end date by N days from NOW(). Refuses
// negative values so the panel can't accidentally retro-expire a
// trial. The account's existing trial_expires_at is overwritten — if
// the operator wants to add to the existing window they need to read
// the current value first.
func (uc *OwnerAdminUseCase) ExtendTrial(ctx context.Context, actorEmail, accountID string, days int) error {
	if days <= 0 || days > 365 {
		return &domain.AppError{Code: "OWNER_EXTEND_INVALID", Message: "days must be between 1 and 365"}
	}
	newExpiresAt := time.Now().UTC().Add(time.Duration(days) * 24 * time.Hour)
	if err := uc.ownerRepo.ExtendTrial(ctx, accountID, newExpiresAt); err != nil {
		return err
	}
	uc.audit(ctx, actorEmail, accountID, "owner.account.extend_trial")
	return nil
}

// Impersonate mints a short-lived JWT scoped to the supplied account's
// admin user, with an additional `impersonating_actor` claim that the
// frontend reads to render a banner. Audit log captures the operator,
// target account, and duration. Refuses if the account has no admin
// user (no one to impersonate as) or has been suspended.
type ImpersonateResult struct {
	Token             string    `json:"token"`
	ExpiresAt         time.Time `json:"expires_at"`
	ImpersonatingUser string    `json:"impersonating_user"` // email of the target admin user
	AccountID         string    `json:"account_id"`
	AccountName       string    `json:"account_name"`
}

func (uc *OwnerAdminUseCase) Impersonate(ctx context.Context, jwtSvc domain.JWTService, actorEmail, accountID string, minutes int) (*ImpersonateResult, error) {
	if minutes <= 0 || minutes > 60 {
		minutes = 15
	}
	acc, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return nil, domain.ErrAccountNotFound
	}
	admin, err := uc.userRepo.GetAdminByAccountID(ctx, accountID)
	if err != nil || admin == nil {
		return nil, &domain.AppError{Code: "OWNER_NO_ADMIN", Message: "account has no admin user to impersonate"}
	}
	role := domain.RoleAdmin
	if admin.Role != nil {
		role = *admin.Role
	}
	expiresAt := time.Now().UTC().Add(time.Duration(minutes) * time.Minute)
	token, err := jwtSvc.GenerateImpersonationToken(domain.ImpersonationClaims{
		UserID:             admin.ID,
		UserName:           admin.Name,
		AccountID:          acc.ID,
		Role:               role,
		ImpersonatingActor: actorEmail,
		ExpiresAt:          expiresAt,
	})
	if err != nil {
		return nil, err
	}
	uc.audit(ctx, actorEmail, accountID, "owner.account.impersonate")
	return &ImpersonateResult{
		Token:             token,
		ExpiresAt:         expiresAt,
		ImpersonatingUser: admin.Email,
		AccountID:         acc.ID,
		AccountName:       acc.Name,
	}, nil
}

// CreateContractDTO captures the payload for provisioning a new B2B
// contract account in one call.
type CreateContractDTO struct {
	ActorEmail   string
	CompanyName  string
	AdminEmail   string
	AdminName    string
	PlanID       string
	Overrides    domain.AccountLimitOverrides
	TrialBypass  bool
	BypassReason string
}

// CreateContract provisions a fresh account + admin user (passwordless
// — invite email triggers the password-set flow), assigns the plan and
// optional caps, and (optionally) flips the trial-bypass flag so the
// account is usable immediately without a Stripe subscription.
//
// Returns the freshly created Account so the caller can navigate the
// operator straight to the detail screen.
func (uc *OwnerAdminUseCase) CreateContract(ctx context.Context, dto CreateContractDTO) (*domain.Account, error) {
	if strings.TrimSpace(dto.CompanyName) == "" {
		return nil, &domain.AppError{Code: "OWNER_NAME_REQUIRED", Message: "company name is required"}
	}
	email := strings.ToLower(strings.TrimSpace(dto.AdminEmail))
	if email == "" || !strings.Contains(email, "@") {
		return nil, &domain.AppError{Code: "OWNER_EMAIL_INVALID", Message: "admin email is invalid"}
	}
	if existing, err := uc.userRepo.GetByEmail(ctx, email); err == nil && existing != nil {
		return nil, &domain.AppError{Code: "OWNER_EMAIL_EXISTS", Message: "a user with this email already exists"}
	}
	if strings.TrimSpace(dto.PlanID) == "" {
		return nil, &domain.AppError{Code: "OWNER_PLAN_REQUIRED", Message: "plan_id is required"}
	}
	plan, err := uc.planRepo.GetByID(ctx, dto.PlanID)
	if err != nil || plan == nil {
		return nil, &domain.AppError{Code: "OWNER_PLAN_NOT_FOUND", Message: "plan_id not found"}
	}

	now := time.Now().UTC()
	acc := &domain.Account{
		ID:                 ulid.Make().String(),
		Name:               strings.TrimSpace(dto.CompanyName),
		PlanID:             &plan.ID,
		SubscriptionStatus: domain.SubscriptionStatusActive,
		Timezone:           "America/Sao_Paulo",
		CSATEnabled:        true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := uc.accountRepo.Create(ctx, acc); err != nil {
		return nil, err
	}
	if err := ensureDefaultFunnel(ctx, uc.funnelRepo, acc.ID); err != nil {
		return nil, err
	}

	// Apply optional contract caps.
	if hasOverrides(dto.Overrides) {
		if err := uc.accountRepo.SetLimitOverrides(ctx, acc.ID, dto.Overrides); err != nil {
			return nil, err
		}
	}
	if dto.TrialBypass {
		_ = uc.accountRepo.SetTrialBypass(ctx, acc.ID, true, dto.BypassReason)
	}

	// Admin user is created without a password and with a verification
	// token in the email column — the regular reset-password flow lets
	// the operator pick their own. PasswordHash stays nil so the magic
	// path is the only login surface.
	// Contract account creator is the OWNER (migration 000149 model).
	role := "owner"
	verifyToken := randomToken()
	verifyExpires := now.Add(72 * time.Hour)
	user := &domain.User{
		ID:                         ulid.Make().String(),
		AccountID:                  acc.ID,
		Name:                       firstNonEmpty(dto.AdminName, dto.CompanyName),
		Email:                      email,
		Role:                       &role,
		IsActive:                   true,
		IsEmailVerified:            false,
		VerificationToken:          &verifyToken,
		VerificationTokenExpiresAt: &verifyExpires,
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}
	if err := uc.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}
	_ = uc.userRepo.MarkInviteSent(ctx, user.ID)

	uc.audit(ctx, dto.ActorEmail, acc.ID, "owner.account.create_contract")
	return acc, nil
}

// ApplyContractDTO captures the payload for converting an ALREADY-
// registered account onto a contract plan (the create-contract flow
// only works for a brand-new email).
type ApplyContractDTO struct {
	ActorEmail   string
	AccountID    string
	PlanID       string
	Overrides    domain.AccountLimitOverrides
	TrialBypass  bool
	BypassReason string
	// PlatformKeyAllowed, when non-nil, sets whether the (contract)
	// account may run on the platform AI key instead of BYOK. nil = leave
	// the current value untouched.
	PlatformKeyAllowed *bool
}

// ApplyContract moves an existing account onto a contract plan in one
// call: assigns the plan, applies optional caps, optionally flips the
// trial-bypass flag, and force-marks the subscription 'active' so the
// account is usable immediately without a Stripe subscription. Unlike
// CreateContract it provisions NO user — the account already has its
// admin. Returns the refreshed account for the detail screen.
func (uc *OwnerAdminUseCase) ApplyContract(ctx context.Context, dto ApplyContractDTO) (*domain.Account, error) {
	acc, err := uc.accountRepo.GetByID(ctx, dto.AccountID)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return nil, domain.ErrAccountNotFound
	}
	if strings.TrimSpace(dto.PlanID) == "" {
		return nil, &domain.AppError{Code: "OWNER_PLAN_REQUIRED", Message: "plan_id is required"}
	}
	plan, err := uc.planRepo.GetByID(ctx, dto.PlanID)
	if err != nil || plan == nil {
		return nil, &domain.AppError{Code: "OWNER_PLAN_NOT_FOUND", Message: "plan_id not found"}
	}

	if err := uc.accountRepo.SetPlan(ctx, acc.ID, plan.ID); err != nil {
		return nil, err
	}
	if err := uc.accountRepo.SetLimitOverrides(ctx, acc.ID, dto.Overrides); err != nil {
		return nil, err
	}
	if err := uc.accountRepo.SetTrialBypass(ctx, acc.ID, dto.TrialBypass, dto.BypassReason); err != nil {
		return nil, err
	}
	if err := uc.accountRepo.SetSubscriptionStatus(ctx, acc.ID, domain.SubscriptionStatusActive); err != nil {
		return nil, err
	}
	if dto.PlatformKeyAllowed != nil {
		if err := uc.accountRepo.SetPlatformKeyAllowed(ctx, acc.ID, *dto.PlatformKeyAllowed); err != nil {
			return nil, err
		}
	}

	uc.recordPlanUpgrade(ctx, acc, plan)
	uc.audit(ctx, dto.ActorEmail, acc.ID, "owner.account.apply_contract")
	return uc.accountRepo.GetByID(ctx, acc.ID)
}

// recordPlanUpgrade ledgers a 'plan_upgraded' billing event when the
// account moved from one PAID plan to a pricier one in the same
// currency (US-008 — feeds the commission upgrade bonus). Trial→paid is
// a conversion, not an upgrade (it would turn every inbound closing
// into the 50% track); cross-currency moves aren't comparable, so both
// are skipped. Best-effort: a ledger failure never fails the apply.
func (uc *OwnerAdminUseCase) recordPlanUpgrade(ctx context.Context, acc *domain.Account, newPlan *domain.Plan) {
	if uc.billingEvents == nil || acc.PlanID == nil || *acc.PlanID == newPlan.ID {
		return
	}
	oldPlan, err := uc.planRepo.GetByID(ctx, *acc.PlanID)
	if err != nil || oldPlan == nil {
		return
	}
	if oldPlan.Price <= 0 || oldPlan.Currency != newPlan.Currency || newPlan.Price <= oldPlan.Price {
		return
	}
	if err := uc.billingEvents.Create(ctx, &domain.BillingEvent{
		ID:          ulid.Make().String(),
		AccountID:   acc.ID,
		EventType:   domain.BillingEventPlanUpgraded,
		Plan:        newPlan.Name,
		AmountCents: int64(newPlan.Price),
		Currency:    newPlan.Currency,
		OccurredAt:  time.Now().UTC(),
	}); err != nil {
		slog.Warn("failed to ledger plan upgrade", "account_id", acc.ID, "plan", newPlan.Name, "error", err)
		return
	}
	slog.Info("plan upgrade ledgered",
		"account_id", acc.ID, "old_plan", oldPlan.Name, "new_plan", newPlan.Name)
}

func (uc *OwnerAdminUseCase) audit(ctx context.Context, actorEmail, accountID, action string) {
	// Defensive: if the operator is somehow ALSO impersonating
	// (theoretical — owner panel routes don't run under impersonation
	// today, but the guard costs nothing) include both fields so the
	// audit trail attributes the action correctly.
	imp := domain.GetImpersonator(ctx)
	payload := `{"actor":"` + escapeJSONString(actorEmail) + `"`
	if imp != "" && imp != actorEmail {
		payload += `,"impersonator":"` + escapeJSONString(imp) + `"`
	}
	payload += "}"
	_ = uc.auditRepo.Create(ctx, &domain.AuditLog{
		ID:         ulid.Make().String(),
		AccountID:  accountID,
		Action:     action,
		EntityType: "account",
		EntityID:   accountID,
		Payload:    []byte(payload),
		CreatedAt:  time.Now().UTC(),
	})
}

func hasOverrides(o domain.AccountLimitOverrides) bool {
	return o.MaxAgents != nil || o.MaxChannels != nil || o.MaxSeats != nil ||
		o.MaxConversationsPerMonth != nil || o.MaxStorageMB != nil ||
		o.MaxAITokensPerMonth != nil || o.MaxBroadcastMessagesPerMonth != nil ||
		o.MaxRecipientsPerCampaign != nil
}

func derefOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func randomToken() string {
	var b [24]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// escapeJSONString is a minimal escaper for the actor email going into
// the audit metadata. We're building JSON by hand to avoid an extra
// allocation; only the standard problematic chars need handling.
func escapeJSONString(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
	)
	return r.Replace(s)
}

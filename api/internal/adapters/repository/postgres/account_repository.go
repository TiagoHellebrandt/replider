package postgres

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/replider/api/internal/adapters/secrets"
	"github.com/replider/api/internal/core/domain"
)

// --- AccountRepository ---

// secretsCipher abstracts the AES-256-GCM Seal/Open used to protect
// BYOK API keys at rest. Defined as a tiny interface so unit tests can
// swap in an identity stub without touching real crypto.
type secretsCipher interface {
	Seal(plaintext string) (string, error)
	Open(sealed string) (string, error)
}

type AccountRepository struct {
	queries *Queries
	cipher  secretsCipher
}

// NewAccountRepository wires the repo with the encryption cipher used
// for BYOK API keys. Passing nil disables encryption entirely — only
// safe in unit tests; production deployments must configure
// BYOK_ENCRYPTION_KEY so the secrets package returns a real cipher.
func NewAccountRepository(db DBTX, cipher *secrets.AESGCMCipher) *AccountRepository {
	r := &AccountRepository{queries: New(db)}
	if cipher != nil {
		r.cipher = cipher
	}
	return r
}

func (r *AccountRepository) Create(ctx context.Context, account *domain.Account) error {
	q := getQueries(ctx, r.queries)
	trialExp := pgtype.Timestamptz{}
	if account.TrialExpiresAt != nil {
		trialExp = pgtype.Timestamptz{Time: *account.TrialExpiresAt, Valid: true}
	}
	// Callers that predate currency capture (owner-provisioned contract
	// accounts) leave it empty — persist the BRL default rather than
	// tripping the CHECK constraint.
	currency := account.Currency
	if currency == "" {
		currency = domain.CurrencyBRL
	}
	return q.CreateAccount(ctx, CreateAccountParams{
		ID:                   account.ID,
		Name:                 account.Name,
		Currency:             currency,
		PlanID:               account.PlanID,
		StripeCustomerID:     account.StripeCustomerID,
		StripeSubscriptionID: account.StripeSubscriptionID,
		SubscriptionStatus:   &account.SubscriptionStatus,
		TrialExpiresAt:       trialExp,
		UtmSource:            account.UTMSource,
		UtmMedium:            account.UTMMedium,
		UtmCampaign:          account.UTMCampaign,
		SignupIp:             account.SignupIP,
		SignupCountry:        account.SignupCountry,
		SignupLocale:         account.SignupLocale,
		CreatedAt:            pgtype.Timestamptz{Time: account.CreatedAt, Valid: true},
		UpdatedAt:            pgtype.Timestamptz{Time: account.UpdatedAt, Valid: true},
	})
}

func (r *AccountRepository) GetByID(ctx context.Context, id string) (*domain.Account, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetAccount(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return withBusinessProfile(r.rowToAccount(
		row.ID, row.Name, row.LogoUrl, row.Timezone, row.Currency, row.PlanID,
		row.StripeCustomerID, row.StripeSubscriptionID, row.SubscriptionStatus,
		row.OnboardingCompleted, row.CsatEnabled, row.TrialExpiresAt, row.PastDueAt,
		row.TrialBypass, row.TrialBypassReason,
		row.OpenaiApiKeyEncrypted, row.GeminiApiKeyEncrypted,
		row.MaxAgentsOverride, row.MaxChannelsOverride, row.MaxSeatsOverride,
		row.MaxConversationsPerMonthOverride, row.MaxStorageMbOverride,
		row.MaxAiTokensPerMonthOverride, row.MaxBroadcastMessagesPerMonthOverride, row.MaxRecipientsPerCampaignOverride,
		row.SuspendedAt, row.SuspendedReason,
		row.ReplyReminderEnabled, row.ReplyReminderThresholdMinutes,
		row.PlatformKeyAllowed,
		row.SellerUserID, row.LeadSource, row.SellerLinkedAt,
		row.InboundDebounceSeconds,
		row.AiMonthlyBudgetCents,
		row.CreatedAt, row.UpdatedAt,
	), row.BusinessLines, row.Territory), nil
}

func (r *AccountRepository) GetAccountByStripeSubscription(ctx context.Context, stripeSubID string) (*domain.Account, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetAccountByStripeSubscription(ctx, &stripeSubID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Return nil if no matching subscription exists
		}
		return nil, err
	}

	return withBusinessProfile(r.rowToAccount(
		row.ID, row.Name, row.LogoUrl, row.Timezone, row.Currency, row.PlanID,
		row.StripeCustomerID, row.StripeSubscriptionID, row.SubscriptionStatus,
		row.OnboardingCompleted, row.CsatEnabled, row.TrialExpiresAt, row.PastDueAt,
		row.TrialBypass, row.TrialBypassReason,
		row.OpenaiApiKeyEncrypted, row.GeminiApiKeyEncrypted,
		row.MaxAgentsOverride, row.MaxChannelsOverride, row.MaxSeatsOverride,
		row.MaxConversationsPerMonthOverride, row.MaxStorageMbOverride,
		row.MaxAiTokensPerMonthOverride, row.MaxBroadcastMessagesPerMonthOverride, row.MaxRecipientsPerCampaignOverride,
		row.SuspendedAt, row.SuspendedReason,
		row.ReplyReminderEnabled, row.ReplyReminderThresholdMinutes,
		row.PlatformKeyAllowed,
		row.SellerUserID, row.LeadSource, row.SellerLinkedAt,
		row.InboundDebounceSeconds,
		row.AiMonthlyBudgetCents,
		row.CreatedAt, row.UpdatedAt,
	), row.BusinessLines, row.Territory), nil
}

// rowToAccount centralises the column → domain mapping so GetByID and
// GetAccountByStripeSubscription don't drift. Decrypts the BYOK key
// columns inline; on cipher error logs and treats as "no key
// configured" — surfacing a decrypt failure as a hard error here would
// break every read of an Enterprise account on key rotation, which is
// strictly worse than a transient BYOK_KEY_MISSING fallback.
// debounceSecondsPtr narrows the sqlc int32 pointer to the domain's int pointer,
// preserving NIL — which is a distinct value here ("use the platform default")
// from an explicit 0 ("answer immediately"). Collapsing them would silently
// enable the debounce for accounts that turned it off.
func debounceSecondsPtr(v *int32) *int {
	if v == nil {
		return nil
	}
	seconds := int(*v)
	return &seconds
}

func (r *AccountRepository) rowToAccount(
	id, name string,
	logoURL *string,
	timezone, currency string,
	planID, stripeCustomerID, stripeSubID, subscriptionStatus *string,
	onboardingCompleted, csatEnabled bool,
	trialExpiresAt, pastDueAt pgtype.Timestamptz,
	trialBypass bool,
	trialBypassReason *string,
	openAIEncrypted, geminiEncrypted *string,
	maxAgentsOverride, maxChannelsOverride, maxSeatsOverride, maxConvsOverride, maxStorageOverride *int32,
	maxTokensOverride, maxBroadcastOverride, maxRecipientsOverride *int32,
	suspendedAt pgtype.Timestamptz,
	suspendedReason *string,
	replyReminderEnabled bool,
	replyReminderThresholdMinutes int32,
	platformKeyAllowed bool,
	sellerUserID, leadSource *string,
	sellerLinkedAt pgtype.Timestamptz,
	inboundDebounceSeconds *int32,
	aiMonthlyBudgetCents *int64,
	createdAt, updatedAt pgtype.Timestamptz,
) *domain.Account {
	return &domain.Account{
		ID:                                   id,
		Name:                                 name,
		LogoURL:                              logoURL,
		Timezone:                             timezone,
		InboundDebounceSeconds:               debounceSecondsPtr(inboundDebounceSeconds),
		AIMonthlyBudgetCents:                 aiMonthlyBudgetCents,
		Currency:                             currency,
		PlanID:                               planID,
		StripeCustomerID:                     stripeCustomerID,
		StripeSubscriptionID:                 stripeSubID,
		SubscriptionStatus:                   stringPtrValue(subscriptionStatus),
		OnboardingCompleted:                  onboardingCompleted,
		CSATEnabled:                          csatEnabled,
		TrialExpiresAt:                       timestamptzPtr(trialExpiresAt),
		PastDueAt:                            timestamptzPtr(pastDueAt),
		TrialBypass:                          trialBypass,
		TrialBypassReason:                    trialBypassReason,
		OpenAIAPIKey:                         r.decryptKey(id, "openai", openAIEncrypted),
		GeminiAPIKey:                         r.decryptKey(id, "gemini", geminiEncrypted),
		MaxAgentsOverride:                    int32PtrToIntPtr(maxAgentsOverride),
		MaxChannelsOverride:                  int32PtrToIntPtr(maxChannelsOverride),
		MaxSeatsOverride:                     int32PtrToIntPtr(maxSeatsOverride),
		MaxConversationsPerMonthOverride:     int32PtrToIntPtr(maxConvsOverride),
		MaxStorageMBOverride:                 int32PtrToIntPtr(maxStorageOverride),
		MaxAITokensPerMonthOverride:          int32PtrToIntPtr(maxTokensOverride),
		MaxBroadcastMessagesPerMonthOverride: int32PtrToIntPtr(maxBroadcastOverride),
		MaxRecipientsPerCampaignOverride:     int32PtrToIntPtr(maxRecipientsOverride),
		SuspendedAt:                          timestamptzPtr(suspendedAt),
		SuspendedReason:                      suspendedReason,
		ReplyReminderEnabled:                 replyReminderEnabled,
		ReplyReminderThresholdMinutes:        int(replyReminderThresholdMinutes),
		PlatformKeyAllowed:                   platformKeyAllowed,
		SellerUserID:                         sellerUserID,
		LeadSource:                           leadSource,
		SellerLinkedAt:                       timestamptzPtr(sellerLinkedAt),
		CreatedAt:                            createdAt.Time,
		UpdatedAt:                            updatedAt.Time,
	}
}

func (r *AccountRepository) decryptKey(accountID, label string, sealed *string) *string {
	if sealed == nil || *sealed == "" {
		return nil
	}
	if r.cipher == nil {
		// No cipher wired (test or misconfig). Treat as "key not
		// usable" — never return raw ciphertext upstream.
		slog.Warn("account: BYOK cipher not configured, treating key as absent",
			"account_id", accountID, "key", label)
		return nil
	}
	plain, err := r.cipher.Open(*sealed)
	if err != nil {
		slog.Error("account: failed to decrypt BYOK key",
			"account_id", accountID, "key", label, "error", err)
		return nil
	}
	if plain == "" {
		return nil
	}
	return &plain
}

func int32PtrToIntPtr(p *int32) *int {
	if p == nil {
		return nil
	}
	v := int(*p)
	return &v
}

func intPtrToInt32Ptr(p *int) *int32 {
	if p == nil {
		return nil
	}
	v := int32(*p)
	return &v
}

func (r *AccountRepository) UpdateAccountSubscription(ctx context.Context, accountID string, stripeCustomerID, stripeSubID *string, status string) error {
	q := getQueries(ctx, r.queries)
	return q.UpdateAccountSubscription(ctx, UpdateAccountSubscriptionParams{
		ID:                   accountID,
		StripeCustomerID:     stripeCustomerID,
		StripeSubscriptionID: stripeSubID,
		SubscriptionStatus:   &status,
	})
}

func (r *AccountRepository) SetStripeCustomerIfAbsent(ctx context.Context, accountID, customerID string) (string, error) {
	q := getQueries(ctx, r.queries)
	effective, err := q.SetStripeCustomerIfAbsent(ctx, SetStripeCustomerIfAbsentParams{
		ID:         accountID,
		CustomerID: &customerID,
	})
	if err != nil {
		return "", err
	}
	if effective == nil {
		return "", nil
	}
	return *effective, nil
}

func (r *AccountRepository) UpdateName(ctx context.Context, accountID string, name string) error {
	q := getQueries(ctx, r.queries)
	return q.UpdateAccountName(ctx, UpdateAccountNameParams{
		ID:   accountID,
		Name: name,
	})
}

// withBusinessProfile decodes the two profile columns onto an account. A
// helper rather than two more positional parameters on rowToAccount, whose
// argument list is already the kind nobody can review by eye. A territory
// that fails to decode is treated as unset: it is a search hint, and a
// corrupt hint must not make the account unreadable.
func withBusinessProfile(acc *domain.Account, lines []string, territory []byte) *domain.Account {
	acc.BusinessLines = make([]domain.BusinessLine, 0, len(lines))
	for _, l := range lines {
		acc.BusinessLines = append(acc.BusinessLines, domain.BusinessLine(l))
	}
	if len(territory) > 0 {
		var t domain.Territory
		if err := json.Unmarshal(territory, &t); err == nil && t.UF != "" {
			acc.Territory = &t
		}
	}
	return acc
}

func (r *AccountRepository) UpdateBusinessProfile(ctx context.Context, accountID string, lines []domain.BusinessLine, territory *domain.Territory) error {
	q := getQueries(ctx, r.queries)
	raw := make([]string, 0, len(lines))
	for _, l := range lines {
		raw = append(raw, string(l))
	}
	var terr []byte
	if territory != nil {
		b, err := json.Marshal(territory)
		if err != nil {
			return err
		}
		terr = b
	}
	return q.UpdateAccountBusinessProfile(ctx, UpdateAccountBusinessProfileParams{
		ID:            accountID,
		BusinessLines: raw,
		Territory:     terr,
	})
}

func (r *AccountRepository) UpdateBranding(ctx context.Context, accountID string, logoURL *string, timezone string) error {
	q := getQueries(ctx, r.queries)
	return q.UpdateAccountBranding(ctx, UpdateAccountBrandingParams{
		ID:       accountID,
		LogoUrl:  logoURL,
		Timezone: timezone,
	})
}

// SetTrialBypass flips the trial-bypass flag and records the reason
// for audit. Admin-only — controllers must enforce role + log.
func (r *AccountRepository) SetTrialBypass(ctx context.Context, accountID string, bypass bool, reason string) error {
	q := getQueries(ctx, r.queries)
	return q.SetAccountTrialBypass(ctx, SetAccountTrialBypassParams{
		ID:     accountID,
		Bypass: bypass,
		Reason: reason,
	})
}

func (r *AccountRepository) UpdateCSATEnabled(ctx context.Context, accountID string, enabled bool) error {
	q := getQueries(ctx, r.queries)
	return q.UpdateAccountCSATEnabled(ctx, UpdateAccountCSATEnabledParams{
		ID:          accountID,
		CsatEnabled: enabled,
	})
}

func (r *AccountRepository) CompleteOnboarding(ctx context.Context, accountID string) error {
	q := getQueries(ctx, r.queries)
	return q.CompleteOnboarding(ctx, accountID)
}

func (r *AccountRepository) GetOnboardingProgress(ctx context.Context, accountID string) (*domain.OnboardingProgress, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetOnboardingProgress(ctx, accountID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	draft := json.RawMessage(row.OnboardingDraft)
	if len(draft) == 0 {
		draft = json.RawMessage("{}")
	}
	progress := &domain.OnboardingProgress{
		AgentCreated:         row.AgentCreated,
		PreviewChatTested:    row.OnboardingPreviewChatTested,
		ChannelConnected:     row.ChannelConnected,
		KnowledgeAdded:       row.KnowledgeAdded,
		ProductsAdded:        row.ProductsAdded,
		CustomersAdded:       row.CustomersAdded,
		ProposalMade:         row.ProposalMade,
		TeamInvited:          row.TeamInvited,
		FirstInboundReceived: row.OnboardingFirstInboundAt.Valid,
		OnboardingCompleted:  row.OnboardingCompleted,
		FirstInboundAt:       timestamptzPtr(row.OnboardingFirstInboundAt),
		ChecklistDismissedAt: timestamptzPtr(row.OnboardingChecklistDismissedAt),
		CurrentStep:          int(row.OnboardingCurrentStep),
		Draft:                draft,
	}
	return progress, nil
}

func (r *AccountRepository) UpdateOnboardingDraft(ctx context.Context, accountID string, currentStep int, draft json.RawMessage) error {
	q := getQueries(ctx, r.queries)
	if len(draft) == 0 {
		draft = json.RawMessage("{}")
	}
	return q.UpdateOnboardingDraft(ctx, UpdateOnboardingDraftParams{
		ID:                    accountID,
		OnboardingCurrentStep: int32(currentStep),
		OnboardingDraft:       []byte(draft),
	})
}

func (r *AccountRepository) MarkPreviewChatTested(ctx context.Context, accountID string) error {
	q := getQueries(ctx, r.queries)
	return q.MarkPreviewChatTested(ctx, accountID)
}

func (r *AccountRepository) MarkFirstInboundReceived(ctx context.Context, accountID string, at time.Time) error {
	q := getQueries(ctx, r.queries)
	return q.MarkFirstInboundReceived(ctx, MarkFirstInboundReceivedParams{
		ID:                       accountID,
		OnboardingFirstInboundAt: pgtype.Timestamptz{Time: at, Valid: true},
	})
}

func (r *AccountRepository) UpdateSubscriptionPeriodEnd(ctx context.Context, accountID string, periodEnd time.Time) error {
	q := getQueries(ctx, r.queries)
	return q.UpdateSubscriptionPeriodEnd(ctx, UpdateSubscriptionPeriodEndParams{
		ID:               accountID,
		CurrentPeriodEnd: pgtype.Timestamptz{Time: periodEnd, Valid: true},
	})
}

func (r *AccountRepository) ListAccountsDueForPrebilling(ctx context.Context, cutoff time.Time) ([]*domain.PrebillingAccount, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListAccountsDueForPrebilling(ctx, pgtype.Timestamptz{Time: cutoff, Valid: true})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.PrebillingAccount, 0, len(rows))
	for _, row := range rows {
		out = append(out, &domain.PrebillingAccount{
			AccountID: row.ID,
			PlanID:    row.PlanID,
			PeriodEnd: row.CurrentPeriodEnd.Time,
		})
	}
	return out, nil
}

// ListAccountIDsWithActiveSubscription returns the ids of self-serve
// accounts on an active Stripe subscription — the set the overage
// reporter (US-168) iterates each cycle to push metered usage.
func (r *AccountRepository) ListAccountIDsWithActiveSubscription(ctx context.Context) ([]string, error) {
	q := getQueries(ctx, r.queries)
	return q.ListAccountIDsWithActiveSubscription(ctx)
}

func (r *AccountRepository) MarkPrebillingNotified(ctx context.Context, accountID string, periodEnd time.Time) error {
	q := getQueries(ctx, r.queries)
	return q.MarkPrebillingNotified(ctx, MarkPrebillingNotifiedParams{
		ID:                          accountID,
		PrebillingNotifiedPeriodEnd: pgtype.Timestamptz{Time: periodEnd, Valid: true},
	})
}

func (r *AccountRepository) DismissOnboardingChecklist(ctx context.Context, accountID string, at time.Time) error {
	q := getQueries(ctx, r.queries)
	return q.DismissOnboardingChecklist(ctx, DismissOnboardingChecklistParams{
		ID:                             accountID,
		OnboardingChecklistDismissedAt: pgtype.Timestamptz{Time: at, Valid: true},
	})
}

// SetAPIKeys encrypts the supplied plaintext keys (empty string clears
// the slot) and writes them to the account row. Keeps the cipher
// dependency localised: callers and use-cases stay in plaintext space.
func (r *AccountRepository) SetAPIKeys(ctx context.Context, accountID string, openAIKey, geminiKey string) error {
	openAISealed, err := r.sealKey(openAIKey)
	if err != nil {
		return err
	}
	geminiSealed, err := r.sealKey(geminiKey)
	if err != nil {
		return err
	}
	q := getQueries(ctx, r.queries)
	return q.SetAccountAPIKeys(ctx, SetAccountAPIKeysParams{
		ID:        accountID,
		OpenaiKey: openAISealed,
		GeminiKey: geminiSealed,
	})
}

func (r *AccountRepository) sealKey(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	if r.cipher == nil {
		// Reject writes when cipher is missing rather than persisting
		// plaintext — the migration column is named
		// `*_api_key_encrypted` for a reason.
		return "", &domain.AppError{Code: "BYOK_CIPHER_UNCONFIGURED", Message: "encryption key is not configured on this server"}
	}
	return r.cipher.Seal(plain)
}

// SetLimitOverrides writes the per-account caps. Pass nil pointers to
// clear a dimension. The sqlc query uses NULLIF so a nil pointer
// translates cleanly to SQL NULL.
func (r *AccountRepository) SetLimitOverrides(ctx context.Context, accountID string, overrides domain.AccountLimitOverrides) error {
	q := getQueries(ctx, r.queries)
	return q.SetAccountLimitOverrides(ctx, SetAccountLimitOverridesParams{
		ID:                                   accountID,
		MaxAgentsOverride:                    intPtrToInt32Ptr(overrides.MaxAgents),
		MaxChannelsOverride:                  intPtrToInt32Ptr(overrides.MaxChannels),
		MaxSeatsOverride:                     intPtrToInt32Ptr(overrides.MaxSeats),
		MaxConversationsPerMonthOverride:     intPtrToInt32Ptr(overrides.MaxConversationsPerMonth),
		MaxStorageMbOverride:                 intPtrToInt32Ptr(overrides.MaxStorageMB),
		MaxAiTokensPerMonthOverride:          intPtrToInt32Ptr(overrides.MaxAITokensPerMonth),
		MaxBroadcastMessagesPerMonthOverride: intPtrToInt32Ptr(overrides.MaxBroadcastMessagesPerMonth),
		MaxRecipientsPerCampaignOverride:     intPtrToInt32Ptr(overrides.MaxRecipientsPerCampaign),
	})
}

func (r *AccountRepository) Delete(ctx context.Context, id string) error {
	q := getQueries(ctx, r.queries)
	return q.DeleteAccount(ctx, id)
}

// Suspend freezes the account. Subsequent inbound webhooks return 403
// ACCOUNT_SUSPENDED until Reactivate clears the row.
func (r *AccountRepository) Suspend(ctx context.Context, accountID, reason string) error {
	q := getQueries(ctx, r.queries)
	return q.SuspendAccount(ctx, SuspendAccountParams{
		ID:     accountID,
		Reason: reason,
	})
}

// Reactivate clears suspended_at + suspended_reason.
func (r *AccountRepository) Reactivate(ctx context.Context, accountID string) error {
	q := getQueries(ctx, r.queries)
	return q.ReactivateAccount(ctx, accountID)
}

// SetSubscriptionStatus force-overrides accounts.subscription_status,
// independent of Stripe. Owner-panel only — used for contract accounts
// that don't have a Stripe subscription backing them.
func (r *AccountRepository) SetSubscriptionStatus(ctx context.Context, accountID, status string) error {
	q := getQueries(ctx, r.queries)
	return q.SetAccountSubscriptionStatus(ctx, SetAccountSubscriptionStatusParams{
		ID:     accountID,
		Status: status,
	})
}

// SetPlan assigns a plan to an existing account — owner-panel "apply
// contract to an already-registered account" flow. Independent of
// Stripe; the use case pairs it with SetSubscriptionStatus('active').
func (r *AccountRepository) SetPlan(ctx context.Context, accountID, planID string) error {
	q := getQueries(ctx, r.queries)
	return q.SetAccountPlan(ctx, SetAccountPlanParams{
		ID:     accountID,
		PlanID: planID,
	})
}

// ClearTrialExpiresAt nulls the trial deadline after a successful
// upgrade to a paid plan — subscription_status governs access from then on.
func (r *AccountRepository) ClearTrialExpiresAt(ctx context.Context, accountID string) error {
	q := getQueries(ctx, r.queries)
	return q.ClearTrialExpiresAt(ctx, accountID)
}

// SetSeller links (or re-links) the account to a seller user with the
// given lead source. seller_linked_at is stamped in SQL on every call.
func (r *AccountRepository) SetSeller(ctx context.Context, accountID, sellerUserID, leadSource string) error {
	q := getQueries(ctx, r.queries)
	return q.SetAccountSeller(ctx, SetAccountSellerParams{
		ID:           accountID,
		SellerUserID: &sellerUserID,
		LeadSource:   &leadSource,
	})
}

// ClearSeller unlinks the seller from the account (owner manual action).
func (r *AccountRepository) ClearSeller(ctx context.Context, accountID string) error {
	q := getQueries(ctx, r.queries)
	return q.ClearAccountSeller(ctx, accountID)
}

// SetCurrency corrects the account's billing currency (owner-panel
// escape hatch, US-015 review fixes). Validation and the active-
// subscription guard live in OwnerAdminUseCase.SetCurrency.
func (r *AccountRepository) SetCurrency(ctx context.Context, accountID, currency string) error {
	q := getQueries(ctx, r.queries)
	return q.SetAccountCurrency(ctx, SetAccountCurrencyParams{
		ID:       accountID,
		Currency: currency,
	})
}

// SetPlatformKeyAllowed flips whether a contract account may run on the
// platform AI key instead of BYOK (Enterprise "our key" deals).
func (r *AccountRepository) SetPlatformKeyAllowed(ctx context.Context, accountID string, allowed bool) error {
	q := getQueries(ctx, r.queries)
	return q.SetAccountPlatformKeyAllowed(ctx, SetAccountPlatformKeyAllowedParams{
		ID:                 accountID,
		PlatformKeyAllowed: allowed,
	})
}

// SetReplyReminderSettings flips the per-account reply-reminder
// opt-in and threshold. Clamped at the SQL CHECK; callers pass
// already-validated values from the use case.
func (r *AccountRepository) SetReplyReminderSettings(ctx context.Context, accountID string, enabled bool, thresholdMinutes int) error {
	q := getQueries(ctx, r.queries)
	return q.SetReplyReminderSettings(ctx, SetReplyReminderSettingsParams{
		ID:                            accountID,
		ReplyReminderEnabled:          enabled,
		ReplyReminderThresholdMinutes: int32(thresholdMinutes),
	})
}

// ListForOwner returns paginated account rows + adjacent owner/plan
// metadata + current-period token usage. `search` matches account
// name OR any user email (ILIKE). `status` filters to one of
// 'active' / 'suspended' / 'trialing' / 'past_due' (empty = all).
func (r *AccountRepository) ListForOwner(ctx context.Context, search, status string, limit, offset int) ([]*domain.AccountListing, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListAccountsForOwner(ctx, ListAccountsForOwnerParams{
		Search: search,
		Status: status,
		Lim:    int32(limit),
		Off:    int32(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.AccountListing, 0, len(rows))
	for _, row := range rows {
		acc := withBusinessProfile(r.rowToAccount(
			row.ID, row.Name, row.LogoUrl, row.Timezone, row.Currency, row.PlanID,
			row.StripeCustomerID, row.StripeSubscriptionID, row.SubscriptionStatus,
			row.OnboardingCompleted, row.CsatEnabled, row.TrialExpiresAt, row.PastDueAt,
			row.TrialBypass, row.TrialBypassReason,
			row.OpenaiApiKeyEncrypted, row.GeminiApiKeyEncrypted,
			row.MaxAgentsOverride, row.MaxChannelsOverride, row.MaxSeatsOverride,
			row.MaxConversationsPerMonthOverride, row.MaxStorageMbOverride,
			row.MaxAiTokensPerMonthOverride, row.MaxBroadcastMessagesPerMonthOverride, row.MaxRecipientsPerCampaignOverride,
			row.SuspendedAt, row.SuspendedReason,
			row.ReplyReminderEnabled, row.ReplyReminderThresholdMinutes,
			row.PlatformKeyAllowed,
			row.SellerUserID, row.LeadSource, row.SellerLinkedAt,
			row.InboundDebounceSeconds,
			row.AiMonthlyBudgetCents,
			row.CreatedAt, row.UpdatedAt,
		), row.BusinessLines, row.Territory)
		// Override the plan default with the per-account override
		// when set so the cap badge reflects the contract reality.
		planMax := row.PlanMaxAiTokensPerMonth
		if acc.MaxAITokensPerMonthOverride != nil {
			planMax = int64(*acc.MaxAITokensPerMonthOverride)
		}
		out = append(out, &domain.AccountListing{
			Account:            acc,
			PlanName:           stringPtrValue(row.PlanName),
			OwnerEmail:         row.OwnerEmail,
			OwnerName:          row.OwnerName,
			TokensUsed:         row.TokensUsed,
			AddonTokens:        row.AddonTokens,
			ConversationsCount: row.ConversationsCount,
			PlanMaxTokens:      planMax,
			WhatsAppQuality:    row.WhatsappQuality,
		})
	}
	return out, nil
}

// CountForOwner returns the total row count matching the filters,
// independent of limit/offset — drives owner-panel pagination.
func (r *AccountRepository) CountForOwner(ctx context.Context, search, status string) (int64, error) {
	q := getQueries(ctx, r.queries)
	return q.CountAccountsForOwner(ctx, CountAccountsForOwnerParams{
		Search: search,
		Status: status,
	})
}

// TopAccountsByTokens returns the top N accounts by current-period
// tokens used (descending). Mirrors the sqlc query — the use case
// re-shapes the rows into domain.AccountTokenUsage so the handler
// returns a UI-friendly payload.
func (r *AccountRepository) TopAccountsByTokens(ctx context.Context, limit int) ([]*domain.AccountTokenUsage, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.TopAccountsByTokens(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	out := make([]*domain.AccountTokenUsage, 0, len(rows))
	for _, row := range rows {
		out = append(out, &domain.AccountTokenUsage{
			AccountID:          row.ID,
			AccountName:        row.Name,
			PlanID:             stringPtrValue(row.PlanID),
			PlanName:           stringPtrValue(row.PlanName),
			TokensUsed:         row.TokensUsed,
			ConversationsCount: row.ConversationsCount,
			SuspendedAt:        timestamptzPtr(row.SuspendedAt),
		})
	}
	return out, nil
}

// ExtendTrial bumps trial_expires_at to the supplied moment. The use
// case is responsible for adding the requested duration to NOW().
func (r *AccountRepository) ExtendTrial(ctx context.Context, accountID string, newExpiresAt time.Time) error {
	q := getQueries(ctx, r.queries)
	return q.ExtendTrial(ctx, ExtendTrialParams{
		ID:           accountID,
		NewExpiresAt: pgtype.Timestamptz{Time: newExpiresAt, Valid: true},
	})
}

// ListConversationsForOwner reads conversations belonging to a target
// account. Bypasses the regular account-scoping baked into
// ConversationRepository — only the owner panel's super-admin routes
// reach this method (the audit middleware logs every call).
func (r *AccountRepository) ListConversationsForOwner(ctx context.Context, accountID string, limit, offset int) ([]*domain.OwnerConversationRow, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListConversationsForOwner(ctx, ListConversationsForOwnerParams{
		AccountID: accountID,
		Lim:       int32(limit),
		Off:       int32(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.OwnerConversationRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, &domain.OwnerConversationRow{
			ID:                    row.ID,
			ChannelID:             row.ChannelID,
			AgentID:               row.AgentID,
			ContactName:           row.ContactName,
			Status:                row.Status,
			IsBotPaused:           row.IsBotPaused,
			ChannelType:           row.ChannelType,
			MessageCount:          row.MessageCount,
			LastCustomerMessageAt: timestamptzPtr(row.LastCustomerMessageAt),
			CreatedAt:             row.CreatedAt.Time,
		})
	}
	return out, nil
}

// SystemHealth aggregates account/user/conv/msg counts for the
// owner-panel widget. Single round-trip via 7 subqueries — cheap.
func (r *AccountRepository) SystemHealth(ctx context.Context) (*domain.SystemHealthSummary, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.SystemHealthSummary(ctx)
	if err != nil {
		return nil, err
	}
	return &domain.SystemHealthSummary{
		AccountsTotal:     row.AccountsTotal,
		AccountsActive:    row.AccountsActive,
		AccountsTrialing:  row.AccountsTrialing,
		AccountsSuspended: row.AccountsSuspended,
		UsersActive:       row.UsersActive,
		Conversations24h:  row.Conversations24h,
		Messages24h:       row.Messages24h,
	}, nil
}

// CountActiveInternalThreads counts the customer account's OPEN
// internal CS threads (canal-interno US-010) for the owner-panel
// account detail. customer_account_id is only ever stamped on
// internal-channel conversations, so the filter needs no channel join.
func (r *AccountRepository) CountActiveInternalThreads(ctx context.Context, accountID string) (int64, error) {
	q := getQueries(ctx, r.queries)
	return q.CountActiveInternalThreads(ctx, accountID)
}

// UsageTrend reads the 30-day token + conversation rollup for a
// target account. Returns 30 rows oldest-first (zero-filled days
// inclusive) so the frontend never has to interpolate gaps.
func (r *AccountRepository) UsageTrend(ctx context.Context, accountID string) ([]*domain.UsageTrendDay, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.UsageTrendByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.UsageTrendDay, 0, len(rows))
	for _, row := range rows {
		out = append(out, &domain.UsageTrendDay{
			Day:               row.Day.Time,
			Tokens:            row.Tokens,
			ConversationCount: row.Conversations,
		})
	}
	return out, nil
}

// SetAIMonthlyBudget writes the customer-set AI ceiling (nil clears it).
// Concrete-only method (narrow port), not on domain.AccountRepository —
// its mocks are legion.
func (r *AccountRepository) SetAIMonthlyBudget(ctx context.Context, accountID string, cents *int64) error {
	q := getQueries(ctx, r.queries)
	return q.SetAccountAIMonthlyBudget(ctx, SetAccountAIMonthlyBudgetParams{
		ID:                   accountID,
		AiMonthlyBudgetCents: cents,
	})
}

// ListTerritories returns every distinct territory somebody sells into.
//
// Feeds the contract sweep, which is shared work: a hospital's public
// contracts are the same public record for every supplier in that state.
//
// A territory that will not parse is skipped with a warning rather than
// failing the sweep — one malformed JSON blob must not stop the calendar for
// every other account.
func (r *AccountRepository) ListTerritories(ctx context.Context) ([]domain.Territory, error) {
	rows, err := getQueries(ctx, r.queries).ListSellingTerritories(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Territory, 0, len(rows))
	for _, raw := range rows {
		if len(raw) == 0 {
			continue
		}
		var t domain.Territory
		if err := json.Unmarshal(raw, &t); err != nil {
			slog.WarnContext(ctx, "territory list: unparsable territory skipped", "error", err)
			continue
		}
		if strings.TrimSpace(t.UF) == "" {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

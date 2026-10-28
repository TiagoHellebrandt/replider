package domain

import (
	"context"
	"encoding/json"
	"time"
)

// AccountRepository defines the persistence operations for Accounts
type AccountRepository interface {
	Create(ctx context.Context, account *Account) error
	GetByID(ctx context.Context, id string) (*Account, error)
	GetAccountByStripeSubscription(ctx context.Context, stripeSubID string) (*Account, error)
	UpdateAccountSubscription(ctx context.Context, accountID string, stripeCustomerID, stripeSubID *string, status string) error
	// SetStripeCustomerIfAbsent atomically claims a Stripe customer id for
	// an account, keeping any id already persisted. Returns the effective
	// (winning) customer id so concurrent checkouts converge on one Stripe
	// customer instead of racing to create duplicates. (US-017)
	SetStripeCustomerIfAbsent(ctx context.Context, accountID, customerID string) (string, error)
	UpdateName(ctx context.Context, accountID string, name string) error
	// UpdateBranding writes logo_url (nullable) and timezone (IANA name)
	// in one call. Pass logoURL=nil to clear the stored logo.
	UpdateBranding(ctx context.Context, accountID string, logoURL *string, timezone string) error
	// UpdateBusinessProfile stores what the company does and where. An
	// empty lines slice clears the lines; a nil territory clears it.
	UpdateBusinessProfile(ctx context.Context, accountID string, lines []BusinessLine, territory *Territory) error
	// UpdateCSATEnabled flips the account-level CSAT asker kill switch.
	// false = worker stops creating pending surveys for this tenant.
	UpdateCSATEnabled(ctx context.Context, accountID string, enabled bool) error
	// SetTrialBypass flips the trial-bypass flag (admin-only). Used
	// for the owner's self-test account + Meta App Reviewer accounts
	// so they don't need a Stripe subscription to drive the full
	// product surface.
	SetTrialBypass(ctx context.Context, accountID string, bypass bool, reason string) error
	CompleteOnboarding(ctx context.Context, accountID string) error
	GetOnboardingProgress(ctx context.Context, accountID string) (*OnboardingProgress, error)
	MarkPreviewChatTested(ctx context.Context, accountID string) error
	MarkFirstInboundReceived(ctx context.Context, accountID string, at time.Time) error
	DismissOnboardingChecklist(ctx context.Context, accountID string, at time.Time) error
	UpdateOnboardingDraft(ctx context.Context, accountID string, currentStep int, draft json.RawMessage) error
	// SetAPIKeys updates the BYOK slots for an account. Empty strings
	// clear the stored value. The repository handles AES-256-GCM
	// encryption on its way to the DB so callers always work with
	// plaintext.
	SetAPIKeys(ctx context.Context, accountID string, openAIKey, geminiKey string) error
	// SetLimitOverrides records per-account caps for contract plans.
	// Pass nil for any dimension to clear it (resetting to plan default).
	SetLimitOverrides(ctx context.Context, accountID string, overrides AccountLimitOverrides) error
	// Suspend freezes the account; the inbound webhook short-circuits
	// with 403 ACCOUNT_SUSPENDED until Reactivate is called.
	Suspend(ctx context.Context, accountID, reason string) error
	Reactivate(ctx context.Context, accountID string) error
	// SetSubscriptionStatus force-overrides accounts.subscription_status
	// independent of Stripe — used for contract accounts.
	SetSubscriptionStatus(ctx context.Context, accountID, status string) error
	// SetPlan assigns a plan to an existing account — the owner panel's
	// "apply contract to an already-registered account" flow, and the
	// Stripe checkout webhook's price→plan activation.
	SetPlan(ctx context.Context, accountID, planID string) error
	// ClearTrialExpiresAt nulls the trial deadline after a successful
	// upgrade to a paid plan — subscription_status governs access from
	// then on.
	ClearTrialExpiresAt(ctx context.Context, accountID string) error
	// SetPlatformKeyAllowed flips whether a contract account may run on
	// the platform AI key instead of BYOK (Enterprise "our key" deals).
	SetPlatformKeyAllowed(ctx context.Context, accountID string, allowed bool) error
	// UpdateSubscriptionPeriodEnd caches the Stripe subscription's
	// current_period_end on the account so the pre-billing cron (US-124)
	// can find accounts whose next charge is approaching. Synced from
	// customer.subscription.{created,updated} webhooks.
	UpdateSubscriptionPeriodEnd(ctx context.Context, accountID string, periodEnd time.Time) error
	// ListAccountsDueForPrebilling returns active accounts whose next
	// charge falls between now and cutoff and which have not yet been
	// emailed for the current billing cycle. The repository's idempotency
	// guard (prebilling_notified_period_end) keeps this list shrinking as
	// reminders go out, so the cron can tick repeatedly inside the window
	// without double-sending.
	ListAccountsDueForPrebilling(ctx context.Context, cutoff time.Time) ([]*PrebillingAccount, error)
	// MarkPrebillingNotified stamps the period_end just emailed about so
	// the same cycle is not re-notified. A renewal moves period_end
	// forward, which re-arms the reminder for the next cycle.
	MarkPrebillingNotified(ctx context.Context, accountID string, periodEnd time.Time) error
	// SetReplyReminderSettings flips the per-account opt-in and
	// threshold for the reply-reminder worker. thresholdMinutes is
	// clamped to [1, 1440] at the SQL CHECK so callers get a domain
	// error rather than a corrupt row when out of range.
	SetReplyReminderSettings(ctx context.Context, accountID string, enabled bool, thresholdMinutes int) error
	Delete(ctx context.Context, id string) error
}

// ReferralRepository owns the referral-program reads/writes (US-174).
// Kept separate from AccountRepository (which is sqlc-generated and
// expensive to extend) — the concrete impl is a raw-SQL repo over the
// same pool, touching the accounts.referral_code /
// referred_by_account_id columns and the `referrals` table directly.
type ReferralRepository interface {
	// GetOrCreateCode returns the account's shareable referral code,
	// minting and persisting one on first call (codes are lazy so
	// existing accounts don't need a backfill). Concurrent first-calls
	// converge on a single code.
	GetOrCreateCode(ctx context.Context, accountID string) (string, error)
	// ResolveReferrer maps a referral code to the owning account id.
	// Returns "" (no error) when the code is unknown so a bad ?ref= on
	// signup is silently ignored rather than blocking registration.
	ResolveReferrer(ctx context.Context, code string) (string, error)
	// LinkReferral records that refereeAccountID was referred by
	// referrerAccountID: stamps accounts.referred_by_account_id and
	// inserts a pending `referrals` row (idempotent — a second call for
	// the same referee is a no-op via the UNIQUE(referee) constraint).
	// A self-referral (referee == referrer) is ignored.
	LinkReferral(ctx context.Context, refereeAccountID, referrerAccountID string) error
	// GetPendingByReferee returns the referee's still-unrewarded referral,
	// or nil when there is none / it was already rewarded.
	GetPendingByReferee(ctx context.Context, refereeAccountID string) (*Referral, error)
	// MarkRewarded flips a referral to rewarded and records the granted
	// amount. The status guard makes the reward fire exactly once.
	MarkRewarded(ctx context.Context, id string, amountCents int64, currency string, at time.Time) error
	// StatsByReferrer summarises how many accounts a referrer brought in
	// and how many converted to a reward.
	StatsByReferrer(ctx context.Context, referrerAccountID string) (*ReferralStats, error)
}

// SellerRefRepository owns users.seller_ref_code and the seller-attribution
// stamp written at signup (US-003, vendedores). A seller is a user of the
// Replider platform account; their personal ?ref=SELLERCODE link attributes
// new customer accounts to them at the 50% commission track. Deliberately a
// separate namespace from ReferralRepository (accounts.referral_code,
// US-174) — a seller code never resolves as a customer referral and
// vice-versa. Raw-SQL impl over the same pool, off the sqlc repos.
type SellerRefRepository interface {
	// GetOrCreateSellerRefCode returns the user's prospecting code,
	// minting one lazily on first call (idempotent — subsequent calls
	// return the same code; concurrent first-calls converge).
	GetOrCreateSellerRefCode(ctx context.Context, userID string) (string, error)
	// ResolveSellerRef maps a code to the owning seller's user id and
	// email, requiring the user to be active and belong to
	// platformAccountID. Returns ("", "") (no error) when the code
	// doesn't match, so a bad ?ref= on signup is silently ignored rather
	// than blocking registration. The email is returned so the signup
	// path can reject self-referrals (US-013) without a second lookup.
	ResolveSellerRef(ctx context.Context, code, platformAccountID string) (userID, email string, err error)
	// StampSellerAttribution links a freshly created customer account to
	// its prospecting seller: seller_user_id + lead_source='seller_ref'
	// + seller_linked_at. No-op if the account already has a seller.
	StampSellerAttribution(ctx context.Context, accountID, sellerUserID string) error
}

// SellerLeadAccount is the slim customer-account row the seller convert-lead
// flow (US-004) reads and returns: enough to pick the right account in the
// search dialog (name + primary admin email) and to see whether it already
// belongs to a seller. AdminEmail is the oldest admin user's email.
type SellerLeadAccount struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	AdminEmail   *string `json:"admin_email,omitempty"`
	SellerUserID *string `json:"seller_user_id,omitempty"`
	LeadSource   *string `json:"lead_source,omitempty"`
	// HasPaymentHistory is true when the account ever had a Stripe
	// subscription (US-014): such an organically paying account can only
	// be converted when the lead conversation's contact provably belongs
	// to it — otherwise an attendant could steal attribution of any
	// existing customer.
	HasPaymentHistory bool       `json:"has_payment_history"`
	CreatedAt         time.Time  `json:"created_at"`
	SellerLinkedAt    *time.Time `json:"seller_linked_at,omitempty"`
}

// SellerLeadRepository backs the explicit inbound lead conversion (US-004):
// a seller on the platform account marks their lead conversation as
// converted into an existing customer account. Implemented by the same
// raw-SQL postgres repo as SellerRefRepository; split here so use cases
// (and their mocks) depend only on the three conversion operations.
type SellerLeadRepository interface {
	// SearchCustomerAccounts finds candidate customer accounts by name or
	// by any of their users' emails (ILIKE substring). platformAccountID
	// itself is always excluded. Ordered newest-first, capped at limit.
	// NOTE: there is no phone column on accounts/users, so a phone query
	// only matches if it happens to appear in the account name.
	SearchCustomerAccounts(ctx context.Context, search, platformAccountID string, limit int) ([]SellerLeadAccount, error)
	// GetLeadAccount returns one account in the same slim shape, or nil
	// when it doesn't exist.
	GetLeadAccount(ctx context.Context, accountID string) (*SellerLeadAccount, error)
	// StampInboundConversion links the account to the seller at the 20%
	// inbound track (lead_source='inbound') and records the converting
	// platform conversation so the co-pilot can deep-link back to the
	// client (US-015). Guarded by seller_user_id IS NULL — returns false
	// (no error) when the account was already linked, so a concurrent
	// double-convert loses cleanly.
	StampInboundConversion(ctx context.Context, accountID, sellerUserID, conversationID string) (bool, error)
	// AccountHasUserEmail reports whether any user of the account has
	// exactly this email (case-insensitive). Backs the convert-lead
	// relation check (US-014): the lead conversation's contact email
	// must belong to the target account when it has payment history.
	AccountHasUserEmail(ctx context.Context, accountID, email string) (bool, error)
}

// PrebillingAccount is the slim row the pre-billing cron (US-124) reads:
// just enough to resolve the admin recipient (AccountID), the plan name
// (PlanID), and how soon the charge lands (PeriodEnd).
type PrebillingAccount struct {
	AccountID string
	PlanID    *string
	PeriodEnd time.Time
}

// AccountListing is the lean owner-panel listing row. Adjacent fields
// (plan name, primary admin email/name, current-period usage) are
// joined in the same query to avoid N+1. UsedTokens / PlanMaxTokens
// drive the cap-alert badge.
// AccountListing is the owner-panel read model returned by
// OwnerAdminUseCase.List and serialized straight to the dashboard. The
// json tags are REQUIRED: without them Go emits PascalCase field names
// (`Account`, `OwnerEmail`, …), the Flutter OwnerAccountListing model
// (FieldRename.snake) fails to find `account`/`owner_email`, its
// fromJson throws, and the owner-access probe swallows the error as
// `forbidden` — hiding the panel for a fully-authorized operator.
type AccountListing struct {
	Account            *Account `json:"account"`
	PlanName           string   `json:"plan_name"`
	OwnerEmail         string   `json:"owner_email"`
	OwnerName          string   `json:"owner_name"`
	TokensUsed         int64    `json:"tokens_used"`
	AddonTokens        int64    `json:"addon_tokens"`
	ConversationsCount int32    `json:"conversations_count"`
	PlanMaxTokens      int64    `json:"plan_max_tokens"`
	// WhatsAppQuality is the worst Meta quality rating (GREEN/YELLOW/RED)
	// across the account's WhatsApp numbers, cached on the channel by the
	// WhatsAppQualityWorker. Empty when no number has been probed yet.
	// RED means the number is at risk of being banned.
	WhatsAppQuality string `json:"whatsapp_quality,omitempty"`
}

// CapPercent returns 0..100 (clamped) — the proportion of the
// current-period token cap consumed. Used by the listing badge to
// flag accounts approaching their limit. Returns 0 when there is no
// cap (contract plan with cap=0). The addon bucket counts toward
// available capacity.
func (l *AccountListing) CapPercent() int {
	if l == nil || l.PlanMaxTokens <= 0 {
		return 0
	}
	denom := l.PlanMaxTokens + l.AddonTokens
	if denom <= 0 {
		return 0
	}
	pct := (l.TokensUsed * 100) / denom
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return int(pct)
}

// OwnerAccountRepository extends AccountRepository with the listing
// queries the owner panel needs. Kept as a separate interface so the
// production AccountRepository can satisfy both without forcing every
// test mock to implement listings it never uses.
type OwnerAccountRepository interface {
	ListForOwner(ctx context.Context, search, status string, limit, offset int) ([]*AccountListing, error)
	CountForOwner(ctx context.Context, search, status string) (int64, error)
	TopAccountsByTokens(ctx context.Context, limit int) ([]*AccountTokenUsage, error)
	// ExtendTrial pushes trial_expires_at to the supplied timestamp.
	// The use case resolves "now + N days" before calling.
	ExtendTrial(ctx context.Context, accountID string, newExpiresAt time.Time) error
	// ListConversationsForOwner reads conversations belonging to a
	// target account, bypassing the regular account-scoping. Only
	// callable from the /v1/owner/* routes; the repository implements
	// the read directly so the conversation-domain repo does NOT have
	// to expose a "no-scope" variant.
	ListConversationsForOwner(ctx context.Context, accountID string, limit, offset int) ([]*OwnerConversationRow, error)
	// SystemHealth returns aggregate counts (accounts/users/conv/msg)
	// for the owner-panel system widget.
	SystemHealth(ctx context.Context) (*SystemHealthSummary, error)
	// UsageTrend returns a 30-day daily rollup of tokens+conv counts
	// for a target account, oldest day first.
	UsageTrend(ctx context.Context, accountID string) ([]*UsageTrendDay, error)
	// CountActiveInternalThreads counts the account's OPEN internal CS
	// threads (conversations stamped with this customer_account_id) for
	// the owner-panel account detail (canal-interno US-010).
	CountActiveInternalThreads(ctx context.Context, accountID string) (int64, error)
	// SetSeller links a customer account to a seller (user of the
	// Replider platform account) with the given lead source
	// ('seller_ref' | 'inbound'), stamping seller_linked_at.
	// ClearSeller unlinks (all three columns back to NULL).
	SetSeller(ctx context.Context, accountID, sellerUserID, leadSource string) error
	ClearSeller(ctx context.Context, accountID string) error
	// SetCurrency corrects an account's billing currency (owner-panel
	// escape hatch for locale-based currency arbitrage, US-015 review
	// fixes). The use case validates the value and refuses accounts
	// with an active Stripe subscription before calling.
	SetCurrency(ctx context.Context, accountID, currency string) error
}

// UsageTrendDay is one bucket in the owner-panel usage chart.
type UsageTrendDay struct {
	Day               time.Time `json:"day"`
	Tokens            int64     `json:"tokens"`
	ConversationCount int64     `json:"conversation_count"`
}

// SystemHealthSummary is the at-a-glance owner-panel widget payload.
// All counts are int64 because Postgres returns COUNT(*) as bigint.
type SystemHealthSummary struct {
	AccountsTotal     int64 `json:"accounts_total"`
	AccountsActive    int64 `json:"accounts_active"`
	AccountsTrialing  int64 `json:"accounts_trialing"`
	AccountsSuspended int64 `json:"accounts_suspended"`
	UsersActive       int64 `json:"users_active"`
	Conversations24h  int64 `json:"conversations_24h"`
	Messages24h       int64 `json:"messages_24h"`
}

// OwnerConversationRow is the slim payload surfaced on the owner-panel
// "view conversations" tab — just enough to triage without exposing
// raw message content. Operators tap a row to view the full thread on
// the regular dashboard via impersonation.
// json tags REQUIRED — serialized straight to the Flutter
// OwnerConversationRow model (FieldRename.snake). Missing tags emit
// PascalCase, the model's fromJson throws, and the detail screen's
// "recent conversations" section silently shows empty (cf. AccountListing).
type OwnerConversationRow struct {
	ID                    string     `json:"id"`
	ChannelID             string     `json:"channel_id"`
	AgentID               *string    `json:"agent_id"`
	ContactName           string     `json:"contact_name"`
	Status                string     `json:"status"`
	IsBotPaused           bool       `json:"is_bot_paused"`
	ChannelType           string     `json:"channel_type"`
	MessageCount          int64      `json:"message_count"`
	LastCustomerMessageAt *time.Time `json:"last_customer_message_at"`
	CreatedAt             time.Time  `json:"created_at"`
}

// AccountTokenUsage is the lean row returned by TopAccountsByTokens —
// just enough to render the "biggest spenders" table without forcing
// the panel to fan out one usage call per account.
// json tags REQUIRED and deliberately NOT 1:1 with the Go field names:
// the Flutter TopAccountUsage model (FieldRename.snake) keys the account
// id/name as `id`/`name`, so AccountID→`id` and AccountName→`name`.
// Missing/mismatched tags throw in fromJson and blank the "biggest
// spenders" widget (cf. AccountListing).
type AccountTokenUsage struct {
	AccountID          string     `json:"id"`
	AccountName        string     `json:"name"`
	PlanID             string     `json:"plan_id"`
	PlanName           string     `json:"plan_name"`
	TokensUsed         int64      `json:"tokens_used"`
	ConversationsCount int32      `json:"conversations_count"`
	SuspendedAt        *time.Time `json:"suspended_at"`
}

// AccountLimitOverrides bundles the optional per-account caps so a
// single repository call can clear or set any subset. Nil pointer ==
// "leave at plan default" on the account row.
type AccountLimitOverrides struct {
	MaxAgents                    *int
	MaxChannels                  *int
	MaxSeats                     *int
	MaxConversationsPerMonth     *int
	MaxStorageMB                 *int
	MaxAITokensPerMonth          *int
	MaxBroadcastMessagesPerMonth *int
	MaxRecipientsPerCampaign     *int
}

// UserRepository defines the persistence operations for Users
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByID(ctx context.Context, id string) (*User, error)
	GetByGoogleID(ctx context.Context, googleID string) (*User, error)
	GetByMetaID(ctx context.Context, metaID string) (*User, error)
	UpdatePasswordHash(ctx context.Context, userID string, hash string) error
	// UpdateProfile updates name/email and, when signature is non-nil,
	// the personal signature too ("" clears it; nil preserves it so old
	// clients that don't send the field never wipe it).
	UpdateProfile(ctx context.Context, userID string, name string, email string, signature *string) error
	SetResetToken(ctx context.Context, userID string, token string, expiresAt time.Time) error
	GetByResetToken(ctx context.Context, token string) (*User, error)
	ClearResetToken(ctx context.Context, userID string) error
	SetGoogleID(ctx context.Context, userID string, googleID string) error
	SetMetaID(ctx context.Context, userID string, metaID string) error
	SetPicture(ctx context.Context, userID string, picture string) error
	SetVerificationToken(ctx context.Context, userID string, token string, expiresAt time.Time) error
	GetByVerificationToken(ctx context.Context, token string) (*User, error)
	VerifyEmail(ctx context.Context, userID string) error
	ListByAccountID(ctx context.Context, accountID string) ([]*User, error)
	UpdateRole(ctx context.Context, userID string, role string) error
	Deactivate(ctx context.Context, userID string) error
	GetAdminByAccountID(ctx context.Context, accountID string) (*User, error)
	// MarkInviteSent stamps users.invite_sent_at = NOW(). Used by the
	// invite-create and resend-invite flows.
	MarkInviteSent(ctx context.Context, userID string) error
	// UpdateLastSeen writes users.last_seen_at = NOW() at most once per
	// 5 minutes (throttle in SQL). Called by the auth middleware on
	// every authenticated request.
	UpdateLastSeen(ctx context.Context, userID string) error
	// SetTOTPSecret stages a Base32 TOTP secret without enabling 2FA;
	// flips totp_enabled to false and clears any existing recovery codes
	// so a re-enrollment cannot inherit the previous bucket.
	SetTOTPSecret(ctx context.Context, userID string, secret string) error
	// EnableTOTP turns on 2FA after a Verify call and persists the
	// bcrypt'd recovery codes.
	EnableTOTP(ctx context.Context, userID string, hashedRecoveryCodes []string) error
	// DisableTOTP wipes the secret + recovery codes and turns 2FA off.
	// The handler must already have proved possession before calling.
	DisableTOTP(ctx context.Context, userID string) error
	// ConsumeTOTPRecoveryCode atomically removes the given hashed code
	// from the user's recovery-code array. Returns true when a code was
	// removed (i.e. the supplied hash matched).
	ConsumeTOTPRecoveryCode(ctx context.Context, userID, hashedCode string) (bool, error)
	// StampTOTPVerified bumps users.totp_verified_at — read by phase 2
	// to decide whether the owner middleware should re-prompt.
	StampTOTPVerified(ctx context.Context, userID string) error
}

// UsageRepository defines the persistence operations for Account Usage
type UsageRepository interface {
	Create(ctx context.Context, usage *AccountUsage) error
	GetByAccountID(ctx context.Context, accountID string) (*AccountUsage, error)
	// GetSummaryByAccountID reads from the account_usage_summary materialized
	// view (US-065) for the read-only dashboard endpoint. May be up to one
	// refresh cycle stale; never use it for live plan-limit enforcement.
	GetSummaryByAccountID(ctx context.Context, accountID string) (*AccountUsage, error)
	Update(ctx context.Context, usage *AccountUsage) error
	AddUsage(ctx context.Context, accountID string, tokensToAdd int64, inputTokens int64, outputTokens int64, conversationsToAdd int32) error
	AddAddonTokens(ctx context.Context, accountID string, tokens int64) error
	AddBroadcastMessages(ctx context.Context, accountID string, count int32) error
	ResetUsage(ctx context.Context, accountID string) error
	SetUsageWarned80(ctx context.Context, accountID string) error
	SetUsageWarned100(ctx context.Context, accountID string) error
	// ListOverageCandidates returns active/past_due, Stripe-billed
	// accounts on an overage-enabled plan whose conversation count has
	// passed the effective franchise — the overage meter worker's queue.
	ListOverageCandidates(ctx context.Context) ([]OverageCandidate, error)
	// GetOverageCandidate returns the single-account overage snapshot
	// used to settle outstanding overage before a cycle reset. Returns
	// (nil, nil) when the account is not overage-billable.
	GetOverageCandidate(ctx context.Context, accountID string) (*OverageCandidate, error)
	// RecordOverageCharge advances the per-cycle overage tallies after
	// the worker bills Stripe. capReached stamps cap_reached_at once.
	RecordOverageCharge(ctx context.Context, accountID string, overageConvs, totalCentsCharged int32, capReached bool) error
}

// AgentRepository defines the persistence operations for AI Agents
type AgentRepository interface {
	Create(ctx context.Context, agent *Agent) error
	GetByID(ctx context.Context, id string) (*Agent, error)
	Update(ctx context.Context, agent *Agent) error
	ListByAccountID(ctx context.Context, accountID string) ([]*Agent, error)
	Delete(ctx context.Context, id string) error
	// ListAllTimezones returns every agent's timezone across all accounts.
	// System-table scope — used only by the admin timezone-audit endpoint to
	// flag invalid tz strings; never exposed to tenant-scoped routes.
	ListAllTimezones(ctx context.Context) ([]*AgentTimezone, error)
}

// PlanRepository defines the persistence operations for Plans
type PlanRepository interface {
	GetByID(ctx context.Context, id string) (*Plan, error)
	// GetByStripePriceID resolves the plan a Stripe price id belongs to,
	// matching either the monthly or annual price column. Returns
	// (nil, nil) when no plan maps to the price. Used by the checkout
	// webhook to activate the purchased plan on the account.
	GetByStripePriceID(ctx context.Context, priceID string) (*Plan, error)
	ListAll(ctx context.Context) ([]*Plan, error)
}

// ChannelRepository defines the persistence operations for Channels
type ChannelRepository interface {
	Create(ctx context.Context, ch *Channel) error
	GetByID(ctx context.Context, id string) (*Channel, error)
	GetByWebhookToken(ctx context.Context, token string) (*Channel, error)
	GetByPhoneNumberID(ctx context.Context, phoneNumberID string) (*Channel, error)
	// GetByWABAID resolves the active WhatsApp channel for a Meta WABA.
	// Used to fan out non-message webhook events (template approvals,
	// quality drops, account review) to the right account.
	GetByWABAID(ctx context.Context, wabaID string) (*Channel, error)
	// ListByWABAID returns every active WhatsApp channel under the
	// WABA. Required by webhook fan-out — when two accounts share a
	// WABA (separate phone numbers under one BSP-managed business),
	// the single-row GetByWABAID delivered events to only the oldest
	// channel and silently leaked them to the wrong tenant.
	ListByWABAID(ctx context.Context, wabaID string) ([]*Channel, error)
	ListByAccountID(ctx context.Context, accountID string) ([]*Channel, error)
	ListByAgentID(ctx context.Context, agentID string) ([]*Channel, error)
	ListAllByProvider(ctx context.Context, provider string) ([]*Channel, error)
	Update(ctx context.Context, ch *Channel) error
	// UpdateCredentialField performs a partial JSONB write on
	// credentials so two writers touching different keys (admin
	// updating registration_status while the token worker rotates
	// access_token) don't clobber each other via read-modify-write.
	UpdateCredentialField(ctx context.Context, channelID, key, value string) error
	// MergeCredentials applies multiple key/value pairs to credentials
	// in one query via PostgreSQL's JSONB `||` merge. Used by the
	// WhatsApp register/verify-code/deregister handlers after they
	// touch several status fields at once.
	MergeCredentials(ctx context.Context, channelID string, fields map[string]string) error
	Delete(ctx context.Context, id string) error
	Toggle(ctx context.Context, id string, isActive bool) (*Channel, error)
}

// WAWebhookEvent is one row in the WhatsApp webhook outbox. The handler
// records every signed payload before parsing so a transient failure
// doesn't drop the message — the worker can replay pending rows.
type WAWebhookEvent struct {
	ID        string
	Payload   []byte
	Signature string
	Attempts  int
}

// WAWebhookEventDigest is the lightweight ops view of an outbox row —
// no payload bytes (so a list endpoint stays cheap) but everything the
// admin needs to triage: status, attempts, the last error message,
// timestamps. The dashboard or runbook curl renders this directly.
type WAWebhookEventDigest struct {
	ID            string     `json:"id"`
	Status        string     `json:"status"`
	Attempts      int        `json:"attempts"`
	ReceivedAt    time.Time  `json:"received_at"`
	ProcessedAt   *time.Time `json:"processed_at,omitempty"`
	LastAttemptAt *time.Time `json:"last_attempt_at,omitempty"`
	Error         string     `json:"error,omitempty"`
}

// WAWebhookOutboxStaleRecover is the optional contract for repos
// that support multi-replica claim recovery. The outbox worker
// uses it to flip stuck `processing` rows back to `pending`.
type WAWebhookOutboxStaleRecover interface {
	RecoverStale(ctx context.Context, staleMinutes int) (int64, error)
}

// WAWebhookOutbox persists raw inbound webhooks for replay. Decouples
// receipt from processing so a parser bug, AI quota error, or DB blip
// during fanout doesn't cost us the message — the worker re-attempts
// pending rows on a 30s+ cooldown.
type WAWebhookOutbox interface {
	Insert(ctx context.Context, id string, payload []byte, signature string) error
	MarkProcessed(ctx context.Context, id string) error
	MarkSkipped(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string, errMsg string) error
	MarkDead(ctx context.Context, id string) error
	ListPending(ctx context.Context, limit int) ([]*WAWebhookEvent, error)
	// ListByStatus surfaces digests for the ops dashboard. status="all"
	// skips the filter; otherwise matches one of pending / processed /
	// skipped / dead.
	ListByStatus(ctx context.Context, status string, limit int) ([]*WAWebhookEventDigest, error)
	// ResetForRetry rolls a dead/pending row back to pending with the
	// retry counters cleared so the next worker tick re-attempts it.
	ResetForRetry(ctx context.Context, id string) error
	PruneProcessed(ctx context.Context, olderThan time.Time) (int64, error)
}

// AccountWebhook is one external HTTP endpoint a tenant subscribed to
// our event stream. The `Secret` is the HMAC-SHA256 key the dispatcher
// signs each POST with, so the consumer can verify the call came from
// us and not a forger spoofing our domain.
type AccountWebhook struct {
	ID              string
	AccountID       string
	URL             string
	Secret          string
	Events          []string
	IsActive        bool
	Description     *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	LastFiredAt     *time.Time
	LastSucceededAt *time.Time
	LastError       *string
	FailureCount    int
}

// AccountWebhookRepository persists tenant-managed outbound webhooks.
// The dispatcher uses ListActiveForEvent on every event emit; the
// settings UI uses the rest.
type AccountWebhookRepository interface {
	Create(ctx context.Context, w *AccountWebhook) (*AccountWebhook, error)
	GetByID(ctx context.Context, accountID, id string) (*AccountWebhook, error)
	ListByAccount(ctx context.Context, accountID string) ([]*AccountWebhook, error)
	ListActiveForEvent(ctx context.Context, accountID, event string) ([]*AccountWebhook, error)
	Update(ctx context.Context, w *AccountWebhook) (*AccountWebhook, error)
	Delete(ctx context.Context, accountID, id string) error
	// RecordSuccess + RecordFailure feed the failure counter the
	// dispatcher uses to decide when to auto-disable a flaky endpoint.
	RecordSuccess(ctx context.Context, id string) error
	RecordFailure(ctx context.Context, id string, errMsg string) error
	Disable(ctx context.Context, id string) error
}

// MessageRepository defines the persistence operations for Messages
type MessageRepository interface {
	// Create persists the message. Returns inserted=false when a row with
	// the same (channel_id, external_message_id) already exists — used to
	// short-circuit AI/billing/websocket side effects when a Meta retry
	// races our handler before the first delivery committed.
	Create(ctx context.Context, msg *Message) (inserted bool, err error)
	GetByExternalID(ctx context.Context, channelID, externalID string) (*Message, error)
	// GetByID loads a message scoped to the account via a conversations
	// JOIN — the on-demand transcription endpoint only gets a message_id
	// from the client and must reject cross-tenant access. Returns
	// (nil, nil) for both missing and cross-account so callers cannot
	// distinguish.
	GetByID(ctx context.Context, messageID, accountID string) (*Message, error)
	// SetTranscript caches the transcriber output on the message. No
	// account check — callers must load the message first to verify
	// ownership.
	SetTranscript(ctx context.Context, messageID, transcript string) error
	// MarkSendFailed stamps failed_at + failed_reason on an outbound
	// message whose channel send call returned an error. Distinct from
	// ApplyWhatsAppStatus, which records a *delivery* failure from a
	// status webhook; this is the local "the SDK call itself blew up"
	// path so the dashboard does not keep showing the bubble as sent.
	MarkSendFailed(ctx context.Context, messageID, reason string) error
	// LastInboundInConversation returns the latest inbound (customer)
	// message of the conversation, (nil, nil) when there is none. Human
	// email replies thread onto it (In-Reply-To / References) and honour
	// its Reply-To.
	LastInboundInConversation(ctx context.Context, conversationID string) (*Message, error)
	// ApplyWhatsAppStatus persists a Cloud API status update onto the
	// outbound row. Records timestamps for delivered/read, error
	// reason on failed, plus the conversation/pricing envelope Meta
	// attaches once per 24h window — required for accurate metered
	// billing and the dashboard's 24h-window indicator.
	ApplyWhatsAppStatus(ctx context.Context, status WhatsAppStatusUpdate) error
	// CreateMessageAttempt records one provider send attempt for a
	// parent message. Used by the WhatsApp bot reply pipeline so each
	// bubble in a split AI reply has its own row to correlate the
	// later status webhook against.
	CreateMessageAttempt(ctx context.Context, a *MessageAttempt) error
	// ApplyMessageAttemptStatus mirrors ApplyWhatsAppStatus on the
	// child attempts table — used when a status webhook references
	// a wamid that belongs to a fan-out attempt rather than the
	// parent's external_message_id.
	ApplyMessageAttemptStatus(ctx context.Context, status WhatsAppStatusUpdate) error
	// GetMessageAttemptByExternalID resolves a wamid back to its
	// parent message + bubble index for dashboard rendering.
	GetMessageAttemptByExternalID(ctx context.Context, channelID, externalID string) (*MessageAttempt, error)
	ListByConversationID(ctx context.Context, conversationID string, limit, offset int32) ([]*Message, error)
	// ListConversationMedia returns every message in the conversation
	// that carries a media_url, newest first. Backs the "Arquivos" tab
	// on the contact detail sheet (images, videos, audios, documents).
	ListConversationMedia(ctx context.Context, conversationID string, limit, offset int32) ([]*Message, error)
	ListByConversationSince(ctx context.Context, conversationID string, since time.Time) ([]*Message, error)
	MarkConversationRead(ctx context.Context, conversationID string, readAt time.Time) error
	ListUnreadInbound(ctx context.Context, conversationID string) ([]*Message, error)
	SearchMessages(ctx context.Context, accountID, query string, limit, offset int) ([]*MessageSearchResult, error)
}

type MessageSearchResult struct {
	MessageID      string    `json:"message_id"`
	ConversationID string    `json:"conversation_id"`
	Content        string    `json:"content"`
	ContactName    *string   `json:"contact_name,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	Rank           float64   `json:"rank"`
}

// ConversationRepository defines the persistence operations for Conversations
// ConversationListFilter is every knob of the inbox list, all applied in
// SQL before LIMIT/OFFSET. Consumed through the optional ConversationLister
// port rather than through ListEnriched, whose positional signature is
// implemented by dozens of test mocks — see the note on that method.
type ConversationListFilter struct {
	AccountID string
	Status    string
	Limit     int32
	Offset    int32
	// Sort: "" / "last_message", "first_in_conv", "oldest", "importance".
	Sort string
	// AssignedTo: user id, AssignedFilterUnassigned, or "" for no filter.
	AssignedTo  string
	ChannelType string
	Category    string
	// Search matches contact name / username / raw sender / last message
	// (ILIKE). The repository escapes the LIKE metacharacters.
	Search   string
	Priority string
}

// ConversationLister is the optional port the inbox handler prefers over
// ListEnriched: the full filter set (search and priority included) in
// SQL. Implemented by the postgres repository; a mock without it degrades
// to ListEnriched plus a Go-side filter over the page.
type ConversationLister interface {
	ListEnrichedQuery(ctx context.Context, f ConversationListFilter) ([]*ConversationSummary, error)
}

// ConversationReopener is the optional port behind "a resolved thread
// comes back when the customer answers" (inbound) and PATCH
// /conversations/:id/reopen (dashboard). Kept off ConversationRepository
// for the same reason as ConversationLister — a required method would
// force an expectation into every SaveInbound test that mocks the repo.
type ConversationReopener interface {
	// FindLatestByContact is FindByContact without the status guard:
	// the newest thread for the (channel, raw sender) pair, closed
	// included. nil, nil when the sender never wrote.
	FindLatestByContact(ctx context.Context, channelID, contactID string) (*Conversation, error)
	// Reopen flips a closed conversation back to open (resolved_at NULL,
	// waiting_since NOW). Returns false when the row was not closed (or
	// not in the account) — a double reopen is a no-op, not an error.
	// ErrConversationAlreadyOpen when another open thread exists for
	// the same sender (partial unique index).
	Reopen(ctx context.Context, conversationID, accountID string) (bool, error)
}

// UnreadCounter backs GET /conversations/unread-count: how many open
// threads hold a customer message nobody opened, and how many such
// messages there are in total.
type UnreadCounter interface {
	CountUnread(ctx context.Context, accountID string) (conversations, messages int64, err error)
}

type ConversationRepository interface {
	Create(ctx context.Context, conv *Conversation) error
	GetByID(ctx context.Context, id string, accountID string) (*Conversation, error)
	Update(ctx context.Context, conv *Conversation) error
	ListByAccountID(ctx context.Context, accountID string, status string) ([]*Conversation, error)
	// sort: "" / "last_message" (default, most recent activity first),
	// "first_in_conv" (oldest first-inbound-at — surfaces forgotten
	// customers), "oldest" (created_at ASC — absolute age).
	// assignedTo: non-empty filters to conversations assigned to that
	// user id IN SQL, before LIMIT/OFFSET — post-pagination filtering
	// hides rows ranked past the page ("" = no filter,
	// AssignedFilterUnassigned = only rows with no assignee).
	ListEnriched(ctx context.Context, accountID string, status string, limit, offset int32, sort string, assignedTo string, channelType string) ([]*ConversationSummary, error)
	SetBotPaused(ctx context.Context, conversationID string, accountID string, paused bool) error
	// AutoResumeStuckConversations flips is_bot_paused back to FALSE
	// for any conversation where an attendant clicked "take over"
	// but never followed up. Returns the number of conversations
	// touched so the worker can log the sweep size. Skips opted-out
	// contacts so the resume doesn't violate LGPD.
	AutoResumeStuckConversations(ctx context.Context, idleMinutes int) (int64, error)
	// RefreshContactAvatar updates contact_avatar_url + contact_bio
	// and bumps contact_avatar_fetched_at. Used by the inbound pipeline
	// to keep the stored Telegram profile in sync when the user changes
	// it on their side. Pass empty strings to clear a field.
	RefreshContactAvatar(ctx context.Context, conversationID, accountID, url, bio string) error
	UpdateLastCustomerMessageAt(ctx context.Context, conversationID string, accountID string) error
	// UpdateSummary persists the rolling digest of aged-out turns (US-024).
	// Scoped by accountID so a guessed conversation id can't overwrite
	// another tenant's summary.
	UpdateSummary(ctx context.Context, conversationID, accountID, summary string) error
	ClearWaitingSince(ctx context.Context, conversationID string, accountID string) error
	FindByContact(ctx context.Context, channelID string, contactID string) (*Conversation, error)
	// GetContactStats aggregates lifetime numbers for a single contact
	// (phone) inside an account: how many conversations, how many
	// messages, and the timestamps of the first and last interaction.
	// Used by the contact detail sheet on the dashboard so the
	// attendant sees "cliente desde X / Y conversas" without doing
	// a client-side scan of the conversation list.
	GetContactStats(ctx context.Context, accountID, contactID string) (*ContactStats, error)
	AssignConversation(ctx context.Context, conversationID, accountID string, assignedTo *string) error
	GetActiveAttendants(ctx context.Context, accountID string) ([]string, error)
	// SetFirstInboundAt stamps conversations.first_inbound_at the
	// first time an inbound message lands. Idempotent — the SQL
	// guards with IS NULL so subsequent calls are no-ops.
	SetFirstInboundAt(ctx context.Context, conversationID, accountID string) error
	// SetLanguage stamps the detected IETF language tag on a conversation.
	// Idempotent — the SQL guards with `language IS NULL` so only the
	// first detection wins, preventing mid-conversation flapping when the
	// customer quotes text in another language.
	SetLanguage(ctx context.Context, conversationID, lang string) error
	// SetSentiment records the analyzer's verdict on the rolling
	// sentiment of inbound messages. Tile-level surface — the
	// dashboard shows a colored badge so attendants can spot angry
	// threads at a glance. Score is the model's confidence (0..1).
	SetSentiment(ctx context.Context, conversationID, accountID, sentiment string, score float32) error
	// SetFirstResponse records who answered first (bot or human) and
	// when. Idempotent: later replies leave the first marker intact
	// so TTFR stats don't jitter when someone sends a follow-up.
	SetFirstResponse(ctx context.Context, conversationID, accountID, by string) error
	// AssumeConversation is the human-takeover op: stamps assigned_to,
	// assigned_at=NOW() and is_bot_paused=TRUE in a single write so the
	// pipeline cannot race the AI path.
	AssumeConversation(ctx context.Context, conversationID, accountID, userID string) error
	// ReleaseConversation is the inverse: clears assigned_to,
	// assigned_at and resumes the bot.
	ReleaseConversation(ctx context.Context, conversationID, accountID string) error
	// GetAssignment returns the takeover state + assigned user's name
	// in one roundtrip (joined with users). Returns nil when the
	// conversation is not found in the account.
	GetAssignment(ctx context.Context, conversationID, accountID string) (*ConversationAssignment, error)
	// IncrementCost folds a single AI turn's estimated provider cost
	// (USD) into conversations.total_cost_usd (US-030). Scoped by
	// accountID so a guessed conversation id can't poison another
	// tenant's cost analytics.
	IncrementCost(ctx context.Context, conversationID, accountID string, costUSD float64) error
	// ListPendingReminders returns conversations whose customer is
	// waiting past the per-account threshold and the bot is paused
	// (or the channel never had an agent). Each row already carries the
	// account threshold, current sent count, and assigned attendant so
	// the worker can dispatch without follow-up roundtrips.
	ListPendingReminders(ctx context.Context, limit int) ([]*ReminderCandidate, error)
	// MarkReminderSent bumps reply_reminder_sent_count and stamps
	// reply_reminder_last_at = NOW(). Idempotent at the SQL level — the
	// worker calls it once per dispatched reminder.
	MarkReminderSent(ctx context.Context, conversationID string) error
}

// Note: PushSubscriptionRepository lives in push_subscription.go alongside
// its domain entity to keep push-related types co-located.

// TransactionManager handles atomic operations across multiple repositories
type TransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// AuditLogRepository defines the persistence operations for Audit Logs
type AuditLogRepository interface {
	Create(ctx context.Context, log *AuditLog) error
	ListByAccount(ctx context.Context, accountID string, limit, offset int32) ([]*AuditLog, error)
	CountByAccount(ctx context.Context, accountID string) (int64, error)
	// ListFiltered + CountFiltered power the Settings → Conformidade
	// filter bar and the CSV export. Each filter is nil-to-skip.
	ListFiltered(ctx context.Context, f AuditLogFilter) ([]*AuditLog, error)
	CountFiltered(ctx context.Context, f AuditLogFilter) (int64, error)
}

// AuditLogFilter groups the optional predicates used by
// AuditLogRepository.ListFiltered / CountFiltered. Each field is
// independently nullable so the caller can mix-and-match.
type AuditLogFilter struct {
	AccountID  string
	Action     *string
	EntityType *string
	ActorID    *string
	From       *time.Time
	To         *time.Time
	Limit      int32
	Offset     int32
}

// AnalyticsRepository defines the persistence operations for analytics queries
type AnalyticsRepository interface {
	GetConversationStats(ctx context.Context, accountID string) (*ConversationStats, error)
	GetMessageStats(ctx context.Context, accountID string) (*MessageStats, error)
	GetDailyConversationCounts(ctx context.Context, accountID string) ([]DailyCount, error)
	GetSLAMetrics(ctx context.Context, accountID string) (*SLAMetrics, error)
	GetAgentPerformance(ctx context.Context, accountID string) ([]AgentPerformance, error)
	GetAIPerformance(ctx context.Context, accountID string) (*AIPerformance, error)
	// GetAttendantPerformance returns one row per human user in the
	// account (LEFT-joined with aggregates over [From, To]). Passing a
	// non-nil Filter.UserID scopes the result to a single member.
	GetAttendantPerformance(ctx context.Context, f AttendantPerformanceFilter) ([]AttendantPerformance, error)
	// GetAttendantLiveSnapshot returns a real-time load view for all
	// active attendants of the account — open conversation count and
	// oldest unanswered customer message timestamp per user. Powers
	// the supervisor panel.
	GetAttendantLiveSnapshot(ctx context.Context, accountID string) ([]AttendantLiveRow, error)
	// GetLiveQueueSnapshot returns the summary of conversations that
	// currently have no assigned attendant (unassigned pile).
	GetLiveQueueSnapshot(ctx context.Context, accountID string) (*LiveQueue, error)
	// GetOverview returns the Mission 6 operational MVP payload
	// (total + %AI + avg TTFR + channel breakdown + top questions
	// + CSAT distribution) for the given account over the last
	// `days` days.
	GetOverview(ctx context.Context, accountID string, days int) (*AnalyticsOverview, error)
	// ListIdleForCSAT returns conversations whose last bot message
	// is older than idleMinutes, the customer hasn't replied since,
	// and no CSAT survey has been asked yet. Worker caps at `limit`
	// to keep per-tick latency bounded.
	ListIdleForCSAT(ctx context.Context, idleMinutes, limit int) ([]*IdleConversationForCSAT, error)
	// GetTopExpensiveConversations returns the account's conversations
	// ranked by accumulated estimated AI provider cost, capped at limit
	// (US-030 cost-attribution dashboard).
	GetTopExpensiveConversations(ctx context.Context, accountID string, limit int) ([]*ExpensiveConversation, error)
}

// StripeEventRecord is a stored webhook event awaiting (re)processing.
// Payload is the raw Stripe event JSON so the reconciler can replay a
// crashed event without a Stripe round-trip. (US-011)
type StripeEventRecord struct {
	EventID   string
	EventType string
	Payload   []byte
}

// StripeEventRepository tracks Stripe webhook events for idempotency and
// crash recovery (US-011).
//
// Handlers MUST gate processing on ClaimEvent — never on IsProcessed →
// process → MarkProcessed. Stripe retries non-2xx with at-least-once
// delivery; two retries in parallel both saw IsProcessed=false and both
// ran the side-effects (e.g. AddAddonTokens twice). ClaimEvent is the
// atomic gate and returns whether THIS caller is the unique processor.
//
// claimed_at and processed_at are distinct: claiming marks "a worker is
// on it", MarkProcessed marks "side-effects committed". A handler that
// crashes between the two leaves a stuck claim that the reconciler
// re-runs via ListStuckEvents.
type StripeEventRepository interface {
	IsProcessed(ctx context.Context, eventID string) (bool, error)
	MarkProcessed(ctx context.Context, eventID string, eventType string) error
	// ClaimEvent atomically claims an event, persisting payload for replay.
	// Returns true for the unique winner (fresh event or re-claimed stuck
	// row); false when already processed or freshly claimed elsewhere.
	ClaimEvent(ctx context.Context, eventID string, eventType string, payload []byte) (bool, error)
	// ListStuckEvents returns events claimed >5m ago that never completed —
	// the reconciler's crash-recovery work-list. (US-011)
	ListStuckEvents(ctx context.Context) ([]StripeEventRecord, error)
	// CleanupOldEvents prunes idempotency rows past Stripe's retry window
	// so the table doesn't grow forever (US-020). Returns rows deleted.
	CleanupOldEvents(ctx context.Context) (int64, error)
}

// OverageChargeRepository is the durable journal of overage charges
// that makes the settler's bill-then-record step crash-safe.
type OverageChargeRepository interface {
	Create(ctx context.Context, c *OverageCharge) error
	// GetPending returns the account's unresolved charge, or (nil, nil).
	GetPending(ctx context.Context, accountID string) (*OverageCharge, error)
	MarkCharged(ctx context.Context, id string) error
	MarkNeedsReview(ctx context.Context, id string) error
	// SumCharged is the cycle-to-date charged total (archived excluded).
	SumCharged(ctx context.Context, accountID string) (int32, error)
	// ArchiveCharged closes the cycle so charged rows stop counting.
	ArchiveCharged(ctx context.Context, accountID string) error
}

// CannedResponseRepository defines the persistence operations for Canned Responses
type CannedResponseRepository interface {
	Create(ctx context.Context, cr *CannedResponse) error
	// ListVisible returns the responses userID can see: every 'account'
	// row of the account plus their own 'personal' rows.
	ListVisible(ctx context.Context, accountID, userID string) ([]*CannedResponse, error)
	GetByID(ctx context.Context, id string) (*CannedResponse, error)
	// GetVisibleByShortcut resolves a shortcut within userID's visible
	// set (account rows + own personal rows) — used for uniqueness checks
	// so a personal shortcut can't shadow an account one for its owner.
	GetVisibleByShortcut(ctx context.Context, accountID, userID, shortcut string) (*CannedResponse, error)
	Update(ctx context.Context, cr *CannedResponse) error
	Delete(ctx context.Context, id string) error
}

// TagRepository defines the persistence operations for Tags
type TagRepository interface {
	Create(ctx context.Context, tag *Tag) error
	ListByAccountID(ctx context.Context, accountID string) ([]*Tag, error)
	GetByID(ctx context.Context, id string) (*Tag, error)
	// GetByAccountAndName is the dedupe lookup the auto-tag pipeline
	// uses before creating. Case-insensitive so "Cobrança" and
	// "cobrança" don't end up as two separate tags. Returns nil + nil
	// when no match.
	GetByAccountAndName(ctx context.Context, accountID, name string) (*Tag, error)
	Update(ctx context.Context, tag *Tag) error
	Delete(ctx context.Context, id string) error
	AddToConversation(ctx context.Context, conversationID, tagID string) error
	RemoveFromConversation(ctx context.Context, conversationID, tagID string) error
	// ListByConversation enforces tenant isolation by requiring the caller's
	// accountID and joining conversations so foreign-account tags cannot leak.
	ListByConversation(ctx context.Context, accountID, conversationID string) ([]*Tag, error)
}

// ContactRepository defines the persistence operations for Contacts (CRM)
type ContactRepository interface {
	Upsert(ctx context.Context, contact *Contact) (*Contact, error)
	// GetByID loads a contact scoped to the account: the WHERE clause
	// filters on account_id so a guessed contact UUID from a foreign
	// account is indistinguishable from a missing one. Returns
	// (nil, nil) for both missing and cross-account so callers cannot
	// leak existence or expose another tenant's PII (phone/email).
	GetByID(ctx context.Context, id, accountID string) (*Contact, error)
	GetByExternalID(ctx context.Context, accountID, externalID string) (*Contact, error)
	// ListByIDs loads several contacts in one round trip, account-scoped.
	// The pipeline views render a recipient per row; resolving them one by
	// one is an N+1 that grows with the customer's own success.
	ListByIDs(ctx context.Context, accountID string, ids []string) ([]*Contact, error)
	ListByAccountID(ctx context.Context, accountID, query string, limit, offset int) ([]*Contact, error)
	Update(ctx context.Context, contact *Contact) error

	// SetOptedOut marks the contact identified by (accountID, externalID)
	// as opted-out at the current moment. Idempotent: re-running it does
	// NOT reset the timestamp. Returns true when a row was updated.
	SetOptedOut(ctx context.Context, accountID, externalID string) (bool, error)

	// ClearOptedOut reverses an opt-out. Admin-only operation — customers
	// can't opt themselves back in, only the account owner can, after
	// confirming the contact gave verbal/written consent. Returns true
	// when a row was actually flipped (false = already opted-in, no-op).
	ClearOptedOut(ctx context.Context, accountID, externalID string) (bool, error)

	// ListOptedOutExternalIDs returns the subset of externalIDs (e.g.
	// phone numbers) that have opted out. Used by CreateCampaignUseCase
	// to filter recipients pre-flight so we never enqueue a send.
	ListOptedOutExternalIDs(ctx context.Context, accountID string, externalIDs []string) (map[string]bool, error)

	// IsOptedOut reports whether the single contact (accountID, externalID)
	// has opted out (contacts.opted_out_at IS NOT NULL). Used by the outbound
	// send guard to abort a template/text send before calling Meta (US-025).
	// A missing contact row is treated as NOT opted out (false).
	IsOptedOut(ctx context.Context, accountID, externalID string) (bool, error)
}

// NoteRepository defines the persistence operations for Conversation Notes
type NoteRepository interface {
	Create(ctx context.Context, note *ConversationNote) error
	ListByConversationID(ctx context.Context, conversationID string) ([]*ConversationNote, error)
	Delete(ctx context.Context, id string) error
}

// PromptRepository defines the persistence operations for the
// "Meus Prompts" library (account-scoped catalogue of system prompts).
type PromptRepository interface {
	Create(ctx context.Context, p *Prompt) error
	GetByID(ctx context.Context, id string) (*Prompt, error)
	ListByAccountID(ctx context.Context, accountID string, limit, offset int) ([]*Prompt, error)
	CountByAccountID(ctx context.Context, accountID string) (int64, error)
	Update(ctx context.Context, p *Prompt) error
	Delete(ctx context.Context, id string) error
}

// PromptVersionRepository defines the per-agent audit trail for prompt
// applies (snapshot of agent.system_prompt before the apply happened,
// so rollback is a 1-tap revert).
type PromptVersionRepository interface {
	Create(ctx context.Context, v *PromptVersion) error
	GetByID(ctx context.Context, id string) (*PromptVersion, error)
	ListByAgentID(ctx context.Context, agentID string, limit, offset int) ([]*PromptVersion, error)
}

// PromptExperimentRepository persists A/B experiments and surfaces the
// per-variant aggregations the results screen renders.
type PromptExperimentRepository interface {
	Create(ctx context.Context, e *PromptExperiment) error
	GetByID(ctx context.Context, id string) (*PromptExperiment, error)
	GetRunningByAgentID(ctx context.Context, agentID string) (*PromptExperiment, error)
	ListByAccountID(ctx context.Context, accountID string, limit, offset int) ([]*PromptExperiment, error)
	ListByAgentID(ctx context.Context, agentID string, limit, offset int) ([]*PromptExperiment, error)
	UpdateStatus(ctx context.Context, id, status string, startedAt, stoppedAt *time.Time) error
	SetWinner(ctx context.Context, id string, winner *string) error
	Delete(ctx context.Context, id string) error

	// MetricsForVariant aggregates per-variant counts + averages by
	// joining experiment_assignments to messages/conversations/csat.
	// Returning a single struct keeps the round-trip to one query.
	MetricsForVariant(ctx context.Context, experimentID, variant string) (*VariantMetrics, error)

	// SampleConversations returns up to `bestLimit` conversations
	// sorted by CSAT rating descending and up to `worstLimit`
	// sorted ascending — the slice that the auto-suggester (MVP-3)
	// hands to the LLM. Implementations must skip conversations
	// without any messages so the bundle stays informative.
	SampleConversations(ctx context.Context, experimentID, variant string, bestLimit, worstLimit int) (best, worst []ExperimentSample, err error)
}

// ExperimentAssignmentRepository handles the sticky per-conversation
// bucket. Implementations MUST upsert with ON CONFLICT DO NOTHING so
// concurrent inbound messages on the same fresh conversation don't
// double-assign and end up on different variants.
type ExperimentAssignmentRepository interface {
	Assign(ctx context.Context, a *ExperimentAssignment) error
	// AssignReturning upserts the bucket and returns the variant that is
	// actually committed for (experiment_id, conversation_id) in a single
	// atomic statement. On a conflicting insert (a concurrent caller won
	// the race) it returns the EXISTING row's variant, never the caller's
	// proposed one — eliminating the Assign-then-Get window where a second
	// reader could observe a flipped variant mid-conversation. Prefer this
	// over Assign+Get on the hot path.
	AssignReturning(ctx context.Context, a *ExperimentAssignment) (string, error)
	Get(ctx context.Context, experimentID, conversationID string) (*ExperimentAssignment, error)
	GetByConversation(ctx context.Context, conversationID string) (*ExperimentAssignment, error)
}

// PaymentAdapter defines the operations for payment processing (Stripe)
type PaymentAdapter interface {
	// CreateCheckoutSession mints a Stripe Checkout URL for the given
	// price/account. [promoID] is the Stripe Promotion Code id (NOT
	// the public-facing slug) — pass "" when no coupon applies. We
	// accept the resolved promo id rather than the raw code so the
	// adapter doesn't have to round-trip Stripe to translate it; the
	// use case looks it up against our local coupons table first.
	CreateCheckoutSession(priceID, accountID, customerID, successURL, cancelURL, promoID string) (string, error)
	// EnsureCustomer returns a Stripe customer id for the account, creating
	// one if needed. The creation call carries an idempotency key derived
	// from accountID, so concurrent first-checkouts for the same account
	// resolve to a single Stripe customer rather than duplicates. (US-017)
	EnsureCustomer(accountID string) (string, error)
	// CreateAddonCheckoutSession creates a one-time checkout for an add-on
	// credit pack. [currency] is the account's billing currency the line
	// item is priced in (US-003) — the caller resolves it, never the adapter.
	CreateAddonCheckoutSession(packID, packName string, priceInCents int64, accountID, currency, successURL, cancelURL string) (string, error)
	// CreateBillingPortalSession returns a short-lived URL for the
	// Stripe-hosted Customer Portal where the signed-in user can
	// manage their payment method, download invoices, or cancel the
	// subscription. Returns ErrNoCustomer when the account has never
	// transacted (no stripe_customer_id persisted yet).
	CreateBillingPortalSession(customerID, returnURL string) (string, error)
	// GetPromotionCodeRedeemed returns the live `times_redeemed` count for
	// a Stripe promotion code. Used to pre-empt an exhausted coupon at
	// checkout (US-013) instead of letting Stripe reject the redemption
	// mid-flow. Returns 0 for an empty promoID.
	GetPromotionCodeRedeemed(promoID string) (int, error)
	// ReportMeteredUsage reports usage-based (metered) billing overage to
	// Stripe for the subscription item on [subscriptionID] whose price
	// matches [meteredPriceID] (US-168). Implementations resolve the
	// subscription item, then POST to
	// /subscription_items/{id}/usage_records with action=`set` (the
	// running cycle total). It is a no-op (nil) when subscriptionID or
	// meteredPriceID is empty, or when the subscription carries no line
	// item for that metered price (the plan simply doesn't bill that
	// meter). [idempotencyKey] collapses retried reports for the same
	// (account, meter, cycle) into one Stripe call.
	ReportMeteredUsage(subscriptionID, meteredPriceID string, quantity int64, idempotencyKey string) error
	// GrantReferralCredit applies an account credit (a negative customer
	// balance transaction) to [customerID], used to grant the "1 month
	// free" referral reward to both the referrer and the referee (US-174).
	// [amountCents] is the positive credit amount; the adapter posts it as
	// a negative balance delta so Stripe deducts it from the next invoice.
	// [idempotencyKey] collapses retried grants for the same referral so a
	// webhook replay can't double-credit.
	GrantReferralCredit(customerID string, amountCents int64, currency, description, idempotencyKey string) error
	// GetSubscriptionPriceID fetches the subscription from Stripe and
	// returns the price id of its first recurring line item. Used by the
	// checkout webhook to resolve which plan was purchased — the
	// checkout.session.completed payload carries only the subscription
	// id, not its line items. Returns "" (no error) when the
	// subscription has no line items.
	GetSubscriptionPriceID(subscriptionID string) (string, error)
}

// CalendarAdapter resolves availability/booking per ACCOUNT: an account
// with its own Google OAuth integration is served under its token; the
// rest use the platform service account (calendar shared with the SA).
type CalendarAdapter interface {
	ListFreeSlots(ctx context.Context, accountID, calendarID string, date time.Time) ([]time.Time, error)
	CreateAppointment(ctx context.Context, accountID, calendarID string, start, end time.Time, attendeeEmail string) error
}

package domain

import (
	"encoding/json"
	"math"
	"strings"
	"time"
)

const (
	SubscriptionStatusTrialing = "trialing"
	SubscriptionStatusActive   = "active"
	SubscriptionStatusPastDue  = "past_due"
	// SubscriptionStatusPaused mirrors Stripe's `paused` collection
	// status (free tier of "subscription pause" flow). Treated like
	// past_due by the usage guard so AI access is gated until the
	// subscription resumes.
	SubscriptionStatusPaused   = "paused"
	SubscriptionStatusCanceled = "canceled"
)

const (
	CurrencyBRL = "brl"
	CurrencyUSD = "usd"
)

// ConvertCents converts a money amount between the two supported
// billing currencies using the owner-managed manual rate
// (SettingUSDBRLRateCents — BRL cents per 1 USD, e.g. 550 = R$5.50).
// Same-currency passes through unchanged; a pair involving any other
// currency (or a non-positive rate) returns ok=false so callers never
// treat a raw cents figure from one currency as another (R$199 must
// not become $199). Rounds once, at the end.
func ConvertCents(cents int64, from, to string, usdBRLRateCents int64) (int64, bool) {
	from = strings.ToLower(strings.TrimSpace(from))
	to = strings.ToLower(strings.TrimSpace(to))
	if from == to && (from == CurrencyBRL || from == CurrencyUSD) {
		return cents, true
	}
	if usdBRLRateCents <= 0 {
		return 0, false
	}
	switch {
	case from == CurrencyUSD && to == CurrencyBRL:
		return int64(math.Round(float64(cents) * float64(usdBRLRateCents) / 100.0)), true
	case from == CurrencyBRL && to == CurrencyUSD:
		return int64(math.Round(float64(cents) * 100.0 / float64(usdBRLRateCents))), true
	default:
		return 0, false
	}
}

// CurrencyFromLocale maps a signup locale to the account's billing
// currency: only a real US-English locale ("en-US"/"en_US",
// case-insensitive) → USD; everything else — including a bare "en",
// which the app's Slang fallback reports for EVERY non-Portuguese
// device — → BRL. The USD ladder is a US launch; treating any English
// tag as US force-locked German/Spanish/UK users into USD with no way
// back (US-017 review fixes). Ambiguity defaults to BRL because the
// owner-panel currency correction (SetCurrency) is the escape hatch
// and it only works before the first subscription.
func CurrencyFromLocale(locale string) string {
	l := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(locale)), "_", "-")
	if l == "en-us" {
		return CurrencyUSD
	}
	return CurrencyBRL
}

// EmailLocaleForCurrency maps an account's billing currency to the
// locale of its transactional emails (US-018 review fixes): USD
// accounts signed up in English, so they get "en"; everything else
// (brl, empty legacy rows) keeps pt-BR. The returned string feeds
// email.resolveLocale — pt-BR is the safe fallback there too.
func EmailLocaleForCurrency(currency string) string {
	if strings.EqualFold(strings.TrimSpace(currency), CurrencyUSD) {
		return "en"
	}
	return "pt-BR"
}

// Account represents a tenant or user of the platform
type Account struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// LogoURL is the public GCS URL of the workspace logo (nullable).
	// Displayed in the UserAccountChip and future customer-facing
	// surfaces (email header, webchat header).
	LogoURL *string `json:"logo_url,omitempty"`
	// Timezone is an IANA name (e.g. "America/Sao_Paulo") used as the
	// default when new agents are created. Stored NOT NULL with a
	// reasonable BR default in the DB; existing per-agent tzs are
	// not retro-migrated.
	Timezone string `json:"timezone"`
	// BusinessLines is how the company earns: sell, rent, service. A
	// hospital-equipment distributor is often all three, and each line
	// reads a different signal — so a set, not one value. Empty means
	// "not told yet", which the onboarding asks before anything else.
	BusinessLines []BusinessLine `json:"business_lines"`
	// AIMonthlyBudgetCents is the CUSTOMER-set hard ceiling on AI spend per
	// account month, in BRL cents (sales-led BYOK cost model). Nil = no
	// ceiling. The spend gate compares the ai_costs ledger against it and
	// refuses new paid generations once crossed — "conta astronômica" is
	// impossible by construction, not by promise.
	AIMonthlyBudgetCents *int64 `json:"ai_monthly_budget_cents,omitempty"`
	// Territory is where she sells. Nil until she says.
	Territory *Territory `json:"territory,omitempty"`
	// InboundDebounceSeconds is the silence window before the AI answers an
	// inbound burst (K1). Nil means the platform default; an explicit 0 means
	// answer immediately, which is a different statement from "unset" and is
	// why this is a pointer.
	InboundDebounceSeconds *int `json:"inbound_debounce_seconds,omitempty"`
	// Currency is the account's billing currency ('brl' or 'usd'),
	// captured at signup from the user's locale. Not user-editable;
	// the only escape hatch is the owner-panel correction
	// (OwnerAdminUseCase.SetCurrency, US-015 review fixes), which is
	// blocked once an active Stripe subscription exists. Plans,
	// checkout and referral credits all follow this value.
	Currency             string  `json:"currency" enums:"brl,usd"`
	PlanID               *string `json:"plan_id,omitempty"`
	StripeCustomerID     *string `json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID *string `json:"stripe_subscription_id,omitempty"`
	SubscriptionStatus   string  `json:"subscription_status" enums:"trialing,active,past_due,canceled"`
	OnboardingCompleted  bool    `json:"onboarding_completed"`
	// CSATEnabled controls the post-conversation satisfaction asker.
	// When false, the worker skips rows for this account entirely — the
	// B2B kill switch users asked for in the "texto chumbado" thread.
	CSATEnabled bool `json:"csat_enabled"`
	// ReplyReminderEnabled gates the reply-reminder worker per tenant.
	// Off by default so existing accounts don't suddenly start firing
	// pushes; flipped on from Settings → Conta.
	ReplyReminderEnabled bool `json:"reply_reminder_enabled"`
	// ReplyReminderThresholdMinutes is the first-tick delay; the
	// worker derives the 30/60/120-minute escalation ladder from it
	// (threshold, 2*threshold, 4*threshold).
	ReplyReminderThresholdMinutes int        `json:"reply_reminder_threshold_minutes"`
	TrialExpiresAt                *time.Time `json:"trial_expires_at,omitempty"`
	PastDueAt                     *time.Time `json:"past_due_at,omitempty"`
	// TrialBypass exempts the account from RequireActiveSubscription —
	// used for the owner's own account + Meta App Reviewer accounts so
	// they can drive the full happy path without a Stripe subscription.
	// Plan limits still apply normally; only the trial-expiration gate
	// is short-circuited.
	TrialBypass       bool    `json:"trial_bypass"`
	TrialBypassReason *string `json:"trial_bypass_reason,omitempty"`
	// BYOK keys (Bring Your Own Key). Only consulted for contract plans
	// (Plan.StripePriceID == nil) — the AI factory falls back to the
	// platform's env-level keys for everyone else. Stored on the row
	// as the decrypted plaintext so use-cases never need to know
	// about the cipher; the repository encrypts on Upsert and decrypts
	// on read using the AES-256-GCM cipher wired in main.go. Empty
	// string means "no key configured" (BYOK accounts in this state
	// surface a "Configure sua chave de API" banner — they don't get
	// to fall back to the shared env keys).
	OpenAIAPIKey *string `json:"-"`
	GeminiAPIKey *string `json:"-"`
	// PlatformKeyAllowed lets a CONTRACT account run on the platform's
	// AI key instead of BYOK (e.g. an Enterprise deal billed on our key
	// at the plan's higher multipliers). Default false keeps strict-BYOK
	// for every other contract account — the AI factory only falls back
	// to the platform key when this is true and no BYOK key is set.
	PlatformKeyAllowed bool `json:"platform_key_allowed"`
	// Per-account limit overrides for contract plans where each deal
	// negotiates its own caps. NULL means "use the plan default".
	// Read via the EffectiveMax* helpers below so callers don't have
	// to think about override-vs-plan resolution.
	MaxAgentsOverride                    *int `json:"max_agents_override,omitempty"`
	MaxChannelsOverride                  *int `json:"max_channels_override,omitempty"`
	MaxSeatsOverride                     *int `json:"max_seats_override,omitempty"`
	MaxConversationsPerMonthOverride     *int `json:"max_conversations_per_month_override,omitempty"`
	MaxStorageMBOverride                 *int `json:"max_storage_mb_override,omitempty"`
	MaxAITokensPerMonthOverride          *int `json:"max_ai_tokens_per_month_override,omitempty"`
	MaxBroadcastMessagesPerMonthOverride *int `json:"max_broadcast_messages_per_month_override,omitempty"`
	MaxRecipientsPerCampaignOverride     *int `json:"max_recipients_per_campaign_override,omitempty"`
	// SuspendedAt — non-null means the owner panel has frozen this
	// account. The inbound webhook short-circuits with 403
	// ACCOUNT_SUSPENDED and the dashboard renders a banner. Reactivate
	// by setting it back to NULL.
	SuspendedAt     *time.Time `json:"suspended_at,omitempty"`
	SuspendedReason *string    `json:"suspended_reason,omitempty"`
	// First-touch acquisition attribution captured from the marketing
	// site's register CTA (utm_*). Write-only on signup; never read back
	// into the hot account-fetch path, so the repository rowToAccount
	// mapper intentionally leaves these nil for existing reads.
	UTMSource   *string `json:"utm_source,omitempty"`
	UTMMedium   *string `json:"utm_medium,omitempty"`
	UTMCampaign *string `json:"utm_campaign,omitempty"`
	// Signup network origin (US-015 review fixes): audit trail for
	// locale-based currency arbitrage. IP from the request, country
	// from the CDN geo header when present, locale as the client sent
	// it. Write-only on signup, same convention as the UTM columns —
	// rowToAccount leaves these nil on reads; audits query them in SQL.
	SignupIP      *string `json:"-"`
	SignupCountry *string `json:"-"`
	SignupLocale  *string `json:"-"`
	// Seller attribution (vendedores v3). SellerUserID references a user
	// of the Replider platform account who manages this customer;
	// LeadSource decides the commission track: 'seller_ref' (prospected
	// via personal ?ref= link, 50%) or 'inbound' (platform channels,
	// 20%). All three are NULL for unmanaged accounts.
	SellerUserID   *string    `json:"seller_user_id,omitempty"`
	LeadSource     *string    `json:"lead_source,omitempty" enums:"seller_ref,inbound"`
	SellerLinkedAt *time.Time `json:"seller_linked_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Lead-source values for seller attribution (accounts.lead_source).
const (
	LeadSourceSellerRef = "seller_ref"
	LeadSourceInbound   = "inbound"
)

// IsValidLeadSource reports whether s is one of the CHECK-constrained
// accounts.lead_source values.
func IsValidLeadSource(s string) bool {
	return s == LeadSourceSellerRef || s == LeadSourceInbound
}

// IsSuspended is the boolean reading of SuspendedAt — keeps callers
// from sprinkling != nil checks all over the codebase.
func (a *Account) IsSuspended() bool {
	return a.SuspendedAt != nil
}

// IsContractPlan reports whether the account is on a contract-billed
// plan (Partner / Enterprise). Determined by the absence of a Stripe
// price id on the plan, which the seed migrations leave NULL on
// purpose for off-platform billing tiers. Pass the resolved Plan; the
// account row alone cannot answer this question because plan limits
// live one indirection away.
func (a *Account) IsContractPlan(plan *Plan) bool {
	return plan != nil && plan.StripePriceID == nil
}

// EffectiveMaxAgents returns the agent cap the account should be held
// to. Resolution order: per-account override → plan limit → unlimited
// (only for contract plans whose plan limit is 0 — the seed leaves
// Enterprise limits at 0 so admins can fill them in via overrides).
func (a *Account) EffectiveMaxAgents(plan *Plan) int {
	return resolveLimit(a.MaxAgentsOverride, plan.MaxAgents, a.IsContractPlan(plan), CeilingAgents)
}

// EffectiveMaxChannels — see EffectiveMaxAgents.
func (a *Account) EffectiveMaxChannels(plan *Plan) int {
	return resolveLimit(a.MaxChannelsOverride, plan.MaxChannels, a.IsContractPlan(plan), CeilingChannels)
}

// EffectiveMaxSeats — see EffectiveMaxAgents.
func (a *Account) EffectiveMaxSeats(plan *Plan) int {
	return resolveLimit(a.MaxSeatsOverride, plan.MaxSeats, a.IsContractPlan(plan), CeilingSeats)
}

// EffectiveMaxConversationsPerMonth — see EffectiveMaxAgents.
func (a *Account) EffectiveMaxConversationsPerMonth(plan *Plan) int {
	return resolveLimit(a.MaxConversationsPerMonthOverride, plan.MaxConversationsPerMonth, a.IsContractPlan(plan), CeilingConversationsPerMonth)
}

// EffectiveMaxStorageMB — see EffectiveMaxAgents.
func (a *Account) EffectiveMaxStorageMB(plan *Plan) int {
	return resolveLimit(a.MaxStorageMBOverride, plan.MaxStorageMB, a.IsContractPlan(plan), CeilingStorageMB)
}

// EffectiveMaxAITokensPerMonth — see EffectiveMaxAgents. Token cap is
// stored as int64 in plan.MaxAITokensPerMonth (high-volume tiers blow
// past int32) so we promote the override + UnlimitedLimit to int64
// instead of forcing the plan column to truncate.
func (a *Account) EffectiveMaxAITokensPerMonth(plan *Plan) int64 {
	if a.MaxAITokensPerMonthOverride != nil && *a.MaxAITokensPerMonthOverride > 0 {
		return int64(*a.MaxAITokensPerMonthOverride)
	}
	if plan.MaxAITokensPerMonth > 0 {
		return plan.MaxAITokensPerMonth
	}
	if a.IsContractPlan(plan) {
		// Same reason as resolveLimit: a contract with no number written gets the
		// platform ceiling, not infinity.
		return int64(CeilingAITokensPerMonth)
	}
	return plan.MaxAITokensPerMonth
}

// EffectiveMaxBroadcastMessagesPerMonth — see EffectiveMaxAgents. The
// CampaignUseCase rejects when this returns <= 0, so contract plans
// with a 0 plan-row default still need IsContractPlan to escalate to
// UnlimitedLimit (otherwise BYOK customers cannot run a campaign).
func (a *Account) EffectiveMaxBroadcastMessagesPerMonth(plan *Plan) int {
	return resolveLimit(a.MaxBroadcastMessagesPerMonthOverride, plan.MaxBroadcastMessagesPerMonth, a.IsContractPlan(plan), CeilingBroadcastPerMonth)
}

// EffectiveMaxRecipientsPerCampaign — see EffectiveMaxAgents.
func (a *Account) EffectiveMaxRecipientsPerCampaign(plan *Plan) int {
	return resolveLimit(a.MaxRecipientsPerCampaignOverride, plan.MaxRecipientsPerCampaign, a.IsContractPlan(plan), CeilingRecipientsPerCampaign)
}

// SDR tier caps (US-M013). No per-account override columns exist for
// these yet, so they resolve plan value → contract escalation. Going
// through resolveLimit anyway means adding an override later is a
// one-line change instead of a hunt through call sites.

// EffectiveMaxLeadsPerMonth — see EffectiveMaxAgents.
func (a *Account) EffectiveMaxLeadsPerMonth(plan *Plan) int {
	return resolveLimit(nil, plan.MaxLeadsPerMonth, a.IsContractPlan(plan), CeilingLeadsPerMonth)
}

// EffectiveMaxEnrichmentsPerMonth — see EffectiveMaxAgents.
func (a *Account) EffectiveMaxEnrichmentsPerMonth(plan *Plan) int {
	return resolveLimit(nil, plan.MaxEnrichmentsPerMonth, a.IsContractPlan(plan), CeilingEnrichmentsPerMonth)
}

// EffectiveMaxAIDraftsPerMonth — see EffectiveMaxAgents.
func (a *Account) EffectiveMaxAIDraftsPerMonth(plan *Plan) int {
	return resolveLimit(nil, plan.MaxAIDraftsPerMonth, a.IsContractPlan(plan), CeilingAIDraftsPerMonth)
}

// EffectiveMaxDocumentExtractionsPerMonth — see EffectiveMaxAgents.
func (a *Account) EffectiveMaxDocumentExtractionsPerMonth(plan *Plan) int {
	return resolveLimit(nil, plan.MaxDocumentExtractionsPerMonth, a.IsContractPlan(plan), CeilingDocumentExtractions)
}

// EffectiveMaxEmailMailboxes — see EffectiveMaxAgents.
func (a *Account) EffectiveMaxEmailMailboxes(plan *Plan) int {
	return resolveLimit(nil, plan.MaxEmailMailboxes, a.IsContractPlan(plan), CeilingEmailMailboxes)
}

// EffectiveMaxFunnels — see EffectiveMaxAgents.
func (a *Account) EffectiveMaxFunnels(plan *Plan) int {
	return resolveLimit(nil, plan.MaxFunnels, a.IsContractPlan(plan), CeilingFunnels)
}

// UnlimitedLimit is the sentinel used when a contract plan has a
// 0-valued cap and the admin hasn't recorded a per-account override.
// Picked at int max so callers comparing `count >= limit` short-circuit
// without ever blocking. Keeping it as a constant (rather than
// math.MaxInt at the call site) makes the intent explicit in slog
// output and unit-test assertions.
const UnlimitedLimit = 1 << 31

// resolveLimit applies the cascade override → plan → platform ceiling.
//
// A 0 limit on a plan normally means "feature off" (the trial fires no
// campaigns), but on a contract plan it means "agreed per deal". That case used
// to resolve to UnlimitedLimit and the customer's screen said "Unlimited
// storage" — a promise nobody made, coming out of a blank column. Nothing is
// unlimited: disk runs out, tokens cost money, and a number that does not exist
// does not become a contract.
//
// The unnegotiated case now answers with the platform ceiling: a real number,
// above the largest off-the-shelf plan so it does not get in the way of whoever
// is being served — and finite, so it is true.
func resolveLimit(override *int, planLimit int, contract bool, ceiling int) int {
	if override != nil && *override > 0 {
		return *override
	}
	if planLimit > 0 {
		return planLimit
	}
	if contract {
		return ceiling
	}
	return planLimit
}

// PastDueGraceDays is the number of days an account can remain past_due
// before AI access is suspended.
const PastDueGraceDays = 7

// IsPastDueGraceExpired returns true when the account has been past_due
// for longer than PastDueGraceDays. Returns false for non-past-due accounts
// or when past_due_at is not set.
func (a *Account) IsPastDueGraceExpired(now time.Time) bool {
	if a.SubscriptionStatus != SubscriptionStatusPastDue {
		return false
	}
	if a.PastDueAt == nil {
		return false
	}
	return now.After(a.PastDueAt.AddDate(0, 0, PastDueGraceDays))
}

// OnboardingProgress captures every milestone the dashboard checklist
// renders. Booleans come from a mix of EXISTS lookups (agent/channel/
// knowledge) and persisted flags (preview chat tested, dismissed_at)
// computed in a single round-trip — see GetOnboardingProgress in the
// account repository. CompletionPercentage is derived in the use-case
// layer so the rule lives in one place.
//
// CurrentStep + Draft let the client resume the wizard mid-flow after a
// cold start. Draft is an opaque JSON blob — the schema is owned by the
// Flutter onboarding controller; the API just round-trips it. An empty
// object ("{}") means "nothing saved yet".
type OnboardingProgress struct {
	AgentCreated      bool `json:"agent_created"`
	PreviewChatTested bool `json:"preview_chat_tested"`
	ChannelConnected  bool `json:"channel_connected"`
	KnowledgeAdded    bool `json:"knowledge_added"`
	// The SALES milestones — what "getting started" means for this product.
	// Products first because nothing else works without knowing what she sells;
	// a proposal last because the three before it are setup and this is the first
	// time the product did the thing it exists for.
	ProductsAdded  bool `json:"products_added"`
	CustomersAdded bool `json:"customers_added"`
	ProposalMade   bool `json:"proposal_made"`
	// TeamInvited: somebody besides the owner has a seat. The seller who
	// approves the offers is usually not the person who signed up.
	TeamInvited          bool            `json:"team_invited"`
	FirstInboundReceived bool            `json:"first_inbound_received"`
	OnboardingCompleted  bool            `json:"onboarding_completed"`
	FirstInboundAt       *time.Time      `json:"first_inbound_at,omitempty"`
	ChecklistDismissedAt *time.Time      `json:"checklist_dismissed_at,omitempty"`
	CompletionPercentage int             `json:"completion_percentage"`
	CurrentStep          int             `json:"current_step"`
	Draft                json.RawMessage `json:"draft"`
	// AccountName mirrors accounts.name. The onboarding wizard pre-fills
	// the "Qual o nome da sua empresa?" field with this value. For
	// social-login (Google/Meta) accounts it starts as the user's
	// personal name — the new step 0 is specifically here so they can
	// replace it with their real business name.
	AccountName string `json:"account_name"`
}

// IsTrialExpired reports whether a trialing account has reached its deadline.
// Non-trialing accounts (active/past_due/canceled) are never "expired" by
// this check — their access is governed by Stripe subscription status instead.
func (a *Account) IsTrialExpired(now time.Time) bool {
	if a.SubscriptionStatus != SubscriptionStatusTrialing {
		return false
	}
	if a.TrialExpiresAt == nil {
		return false
	}
	return now.After(*a.TrialExpiresAt)
}

// BusinessLine is one way a company earns from the equipment it handles.
type BusinessLine string

const (
	// BusinessLineSell — resale of new or refurbished equipment.
	BusinessLineSell BusinessLine = "sell"
	// BusinessLineRent — locação: equipment out for a period.
	BusinessLineRent BusinessLine = "rent"
	// BusinessLineService — assistência técnica: maintenance, calibration,
	// repair. The recurring line, and the one an idle machine points at.
	BusinessLineService BusinessLine = "service"
)

// KnownBusinessLines is the closed set the DB constraint mirrors.
var KnownBusinessLines = []BusinessLine{BusinessLineSell, BusinessLineRent, BusinessLineService}

// Valid reports whether the line is one of the known three.
func (b BusinessLine) Valid() bool {
	for _, k := range KnownBusinessLines {
		if b == k {
			return true
		}
	}
	return false
}

// Territory is where the company sells: a state and, optionally, the cities
// inside it she actually covers. Kept small on purpose — the consumer is a
// search ("hospitals in these cities"), and every extra field here is a
// question onboarding would have to ask.
type Territory struct {
	UF     string   `json:"uf"`
	Cities []string `json:"cities,omitempty"`
}

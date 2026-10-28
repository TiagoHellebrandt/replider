package domain

import (
	"time"
)

// Plan defines the SaaS subscription tier and its limits
type Plan struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	StripePriceID *string `json:"stripe_price_id,omitempty"`
	// StripePriceIDAnnual is the Stripe Price id for the yearly recurring
	// charge (US-169). nil/empty means the plan has no annual option —
	// checkout then falls back to the monthly price. Contract plans keep
	// both nil.
	StripePriceIDAnnual      *string `json:"stripe_price_id_annual,omitempty"`
	Currency                 string  `json:"currency"`
	Price                    int     `json:"price"` // in cents
	AIProvider               string  `json:"ai_provider"`
	AIModel                  string  `json:"ai_model"`
	InputMultiplier          float64 `json:"input_multiplier"`
	OutputMultiplier         float64 `json:"output_multiplier"`
	MaxAgents                int     `json:"max_agents"`
	MaxChannels              int     `json:"max_channels"`
	MaxSeats                 int     `json:"max_seats"`
	MaxConversationsPerMonth int     `json:"max_conversations_per_month"`
	MaxAITokensPerMonth      int64   `json:"max_ai_tokens_per_month"`
	// MaxDailyAITokens caps AI tokens per calendar day (US-025). 0 means
	// "use the platform default" — see EffectiveMaxDailyAITokens. A
	// contract plan can set a higher explicit value.
	MaxDailyAITokens int64 `json:"max_daily_ai_tokens"`
	// MaxHistoryMessages caps how many recent conversation messages are
	// replayed into the AI prompt each turn (US-024). 0 means "use the
	// platform default" — see EffectiveMaxHistoryMessages. Older turns are
	// folded into conversations.summary so context isn't lost. A contract
	// plan can raise this for richer multi-turn coherence.
	MaxHistoryMessages int `json:"max_history_messages"`
	MaxStorageMB       int `json:"max_storage_mb"`
	// Broadcast caps — 0 means the plan does not permit broadcasts.
	MaxBroadcastMessagesPerMonth int `json:"max_broadcast_messages_per_month"`
	MaxRecipientsPerCampaign     int `json:"max_recipients_per_campaign"`

	// SDR tier caps (US-M013). The product sells prospecting, so these are
	// the numbers a customer actually shops on. 0 keeps the platform-wide
	// meaning: "not allowed" on a self-serve plan, "use the account
	// override" on a contract plan (see resolveLimit).
	MaxLeadsPerMonth       int `json:"max_leads_per_month"`
	MaxEnrichmentsPerMonth int `json:"max_enrichments_per_month"`
	MaxEmailMailboxes      int `json:"max_email_mailboxes"`
	MaxFunnels             int `json:"max_funnels"`
	// The two AI operations a single request can spend in bulk (K4):
	// `draft_approaches_batch` fills a whole queue, and the scanned-contract
	// path sends entire PDFs to a multimodal model.
	MaxAIDraftsPerMonth            int `json:"max_ai_drafts_per_month"`
	MaxDocumentExtractionsPerMonth int `json:"max_document_extractions_per_month"`
	// Overage metering (Mission 8 pricing revamp).
	// OverageConversationCents is the per-extra-conversation rate in
	// centavos BRL once the plan's franchise is exhausted.
	// OverageCapMultiplier caps total overage at N× the plan price
	// in the same billing cycle (typically 2.00; 1.50 for Partner).
	OverageConversationCents int     `json:"overage_conversation_cents"`
	OverageCapMultiplier     float64 `json:"overage_cap_multiplier"`
	// IsListed gates the public catalogue (GET /plans). FALSE keeps the
	// row alive for accounts already on it (FKs, invoices) while hiding
	// it from signup/upgrade flows — used to retire Partner (US-001,
	// vendedores v3).
	IsListed  bool      `json:"is_listed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Billing intervals supported at checkout (US-169). The wire value the
// app/web send in `?interval=` / the checkout body.
const (
	BillingIntervalMonthly = "monthly"
	BillingIntervalAnnual  = "annual"
)

// NormalizeBillingInterval maps a (possibly empty / unknown) interval
// string to a supported value, defaulting to monthly. The bool reports
// whether the input was a recognised interval — handlers use it to 400
// on garbage rather than silently billing monthly.
func NormalizeBillingInterval(s string) (string, bool) {
	switch s {
	case "", BillingIntervalMonthly:
		return BillingIntervalMonthly, true
	case BillingIntervalAnnual:
		return BillingIntervalAnnual, true
	default:
		return BillingIntervalMonthly, false
	}
}

// ResolveStripePriceID returns the Stripe Price id to bill for the given
// interval. Annual falls back to the monthly price when the plan has no
// annual price configured, so enabling annual per-plan is incremental and
// an unseeded annual price never blocks an upgrade. Returns "" only when
// the plan has no monthly price at all (contract plans).
func (p *Plan) ResolveStripePriceID(interval string) string {
	if p == nil {
		return ""
	}
	if interval == BillingIntervalAnnual && p.StripePriceIDAnnual != nil && *p.StripePriceIDAnnual != "" {
		return *p.StripePriceIDAnnual
	}
	if p.StripePriceID != nil {
		return *p.StripePriceID
	}
	return ""
}

// DefaultMaxDailyAITokens is the per-day AI token ceiling applied when a
// plan doesn't set its own (US-025). Bounds a single runaway day to a
// fraction of a typical monthly allowance.
const DefaultMaxDailyAITokens int64 = 100_000

// EffectiveMaxDailyAITokens returns the plan's daily cap, falling back to
// the platform default when unset (0). (US-025)
func (p *Plan) EffectiveMaxDailyAITokens() int64 {
	if p == nil || p.MaxDailyAITokens <= 0 {
		return DefaultMaxDailyAITokens
	}
	return p.MaxDailyAITokens
}

// DefaultMaxHistoryMessages is the prompt-window size applied when a plan
// doesn't set its own (US-024). Bounds the per-turn prompt so an
// arbitrarily long conversation can't blow up token cost — older turns
// live in conversations.summary instead.
const DefaultMaxHistoryMessages = 20

// EffectiveMaxHistoryMessages returns the plan's prompt-window size,
// falling back to the platform default when unset (0). (US-024)
func (p *Plan) EffectiveMaxHistoryMessages() int {
	if p == nil || p.MaxHistoryMessages <= 0 {
		return DefaultMaxHistoryMessages
	}
	return p.MaxHistoryMessages
}

// Feature gates are keyed by (currency, name), not name alone: the BRL
// ladder (Solo/Starter/Pro/Business/Scale) and the USD ladder
// (Solo/Core/Pro, migration 000109) reuse the names "Solo" and "Pro"
// for tiers at very different price points, so a name-only switch
// silently couples them (US-017 review fixes). Contract plans
// (Partner/Enterprise) and every pre-currency row are 'brl'. The check
// stays name-based (vs a boolean column) because the catalogue is
// small and stable.

// AllowsExperiments reports whether the plan exposes A/B testing of
// system prompts. BRL: Pro and above; USD: Pro only (Core at ~R$550
// sits below the BRL Pro anchor). Trial/entry tiers see an upsell card
// on the Experiments tab.
func (p *Plan) AllowsExperiments() bool {
	if p == nil {
		return false
	}
	if p.Currency == CurrencyUSD {
		return p.Name == "Pro"
	}
	switch p.Name {
	case "Pro", "Business", "Scale", "Partner", "Enterprise":
		return true
	}
	return false
}

// AllowsAutoSuggest reports whether the plan exposes the LLM-driven
// improvement suggester (MVP-3) — the synthesizer call is not free.
// BRL: Business and above. USD: Pro — it's the top self-serve USD tier,
// so excluding it would make the feature unreachable for every USD
// customer (the bug this gate split fixes).
func (p *Plan) AllowsAutoSuggest() bool {
	if p == nil {
		return false
	}
	if p.Currency == CurrencyUSD {
		return p.Name == "Pro"
	}
	switch p.Name {
	case "Business", "Scale", "Partner", "Enterprise":
		return true
	}
	return false
}

// AllowsBrandingRemoval reports whether the plan may hide the discreet
// "Powered by Replider" badge rendered by the webchat widget (US-172). The
// badge is a free-tier distribution loop: it shows on Trial/free accounts
// and is removable on any paid plan — every tier of both ladders. Nil
// plan → false (keep the badge), the safe default when an account's plan
// can't be resolved.
func (p *Plan) AllowsBrandingRemoval() bool {
	if p == nil {
		return false
	}
	if p.Currency == CurrencyUSD {
		switch p.Name {
		case "Solo", "Core", "Pro":
			return true
		}
		return false
	}
	switch p.Name {
	case "Solo", "Starter", "Pro", "Business", "Scale", "Partner", "Enterprise":
		return true
	}
	return false
}

// TrialPlan returns a free trial plan for accounts without a paid subscription.
// Limits are intentionally low and non-renewable once exhausted.
// This allows new users to test the platform before committing to a plan.
//
// StripePriceID is set to a sentinel string so IsContractPlan returns
// false for trial accounts. Without it the EffectiveMax* accessors on
// Account would treat trial's cap-of-zero broadcast/recipient fields
// as "negotiate per deal" → unlimited, which would silently let trial
// users run campaigns. The sentinel value is not a real Stripe id and
// nothing reads it; the only contract is "non-nil → not a contract plan".
var trialPlanStripeSentinel = "__trial_no_stripe__"

func TrialPlan() *Plan {
	return &Plan{
		ID:                       "__trial__",
		Name:                     "Trial",
		StripePriceID:            &trialPlanStripeSentinel,
		AIProvider:               "google",
		AIModel:                  "gemini-3.7-flash",
		InputMultiplier:          1.0,
		OutputMultiplier:         1.0,
		MaxAITokensPerMonth:      10_000, // ~80 short interactions
		MaxAgents:                1,
		MaxChannels:              1,
		MaxSeats:                 1,
		MaxConversationsPerMonth: 50, // enough to validate, well below the Solo tier (200)
		MaxStorageMB:             10,
		// The trial MUST be allowed to draft: with this at 0 every self-serve
		// account answered AI_DRAFTS_NOT_ALLOWED on its very first offer
		// (resolveLimit keeps 0 as "not allowed" for non-contract plans —
		// see the K4 gotcha in CLAUDE.md). Twenty is a week of real use.
		MaxAIDraftsPerMonth:            20,
		MaxDocumentExtractionsPerMonth: 5,
	}
}

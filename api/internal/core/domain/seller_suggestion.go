package domain

import (
	"context"
	"time"
)

// Seller co-pilot suggestion types (US-014). Each maps to one signal
// computed on demand from data the platform already collects — the
// engine never invents metrics, it reads existing ones.
const (
	SuggestionAddKnowledge     = "add_knowledge"      // takeover rate > 30% in the month
	SuggestionChurnRiskCheckin = "churn_risk_checkin" // conversations pace < 50% of last month
	SuggestionSuggestUpgrade   = "suggest_upgrade"    // >= 80% of plan allowance used
	SuggestionOnboardingStuck  = "onboarding_stuck"   // checklist < 100% after 7 days
	SuggestionWhatsAppBanRisk  = "whatsapp_ban_risk"  // quality worker flagged a RED number
	SuggestionNPSPromoter      = "nps_promoter"       // NPS 9-10 this month
	SuggestionNPSDetractor     = "nps_detractor"      // NPS 0-6 this month
)

// Suggestion severities, ordered: critical > warning > info.
const (
	SuggestionSeverityCritical = "critical"
	SuggestionSeverityWarning  = "warning"
	SuggestionSeverityInfo     = "info"
)

// Suggestion state values a seller can persist per suggestion/cycle.
const (
	SuggestionStatusDismissed = "dismissed"
	SuggestionStatusDone      = "done"
)

func IsValidSuggestionType(t string) bool {
	switch t {
	case SuggestionAddKnowledge, SuggestionChurnRiskCheckin, SuggestionSuggestUpgrade,
		SuggestionOnboardingStuck, SuggestionWhatsAppBanRisk, SuggestionNPSPromoter,
		SuggestionNPSDetractor:
		return true
	}
	return false
}

func IsValidSuggestionStatus(s string) bool {
	return s == SuggestionStatusDismissed || s == SuggestionStatusDone
}

// SellerSuggestion is one actionable co-pilot item for a carteira
// account: the type, the severity, the raw data that triggered it
// (Signal — kept structured so the panel can render the evidence, not
// just the advice) and a ready action text.
type SellerSuggestion struct {
	Type        string                 `json:"type"`
	AccountID   string                 `json:"account_id"`
	AccountName string                 `json:"account_name"`
	Severity    string                 `json:"severity"`
	Signal      map[string]interface{} `json:"signal"`
	ActionText  string                 `json:"action_text"`
	// Platform conversation with this client (the one the convert action
	// stamped, US-015) — lets the feed open the chat with a pre-filled
	// template. Nil when no conversation ever existed (?ref= signup).
	ConversationID *string `json:"conversation_id,omitempty"`
	// ChannelID scopes channel-level signals (whatsapp_ban_risk emits one
	// suggestion per RED number, review-fixes US-022) so dismissing one
	// channel's risk never hides another's. Nil on account-level types.
	ChannelID *string `json:"channel_id,omitempty"`
}

// SellerConvStats is the per-account conversation aggregate feeding the
// takeover-rate and churn-pace signals: totals for the current month
// (plus how many had at least one human reply) and the previous month.
type SellerConvStats struct {
	AccountID    string
	CurrentTotal int
	CurrentHuman int
	PrevTotal    int
}

// SellerChannelRisk is a WhatsApp number the quality worker flagged as
// at risk of a ban (quality_rating cached on channels.credentials).
type SellerChannelRisk struct {
	AccountID     string
	ChannelID     string
	ChannelName   string
	QualityRating string
}

// SellerNPSSignal is the latest NPS response an account gave inside the
// window (one row per account).
type SellerNPSSignal struct {
	AccountID string
	Score     int
	CreatedAt time.Time
}

// SellerSuggestionState is a persisted dismissed/done mark for one
// suggestion type on one account in one cycle. ChannelID is "" for
// account-level types and the channel id for channel-scoped ones
// (whatsapp_ban_risk) — the dismissal key includes it (US-022).
type SellerSuggestionState struct {
	AccountID      string
	SuggestionType string
	ChannelID      string
	Cycle          string
	Status         string
}

// SellerSuggestionRepository reads the raw signals for the co-pilot
// engine and persists per-suggestion dismissal state (US-014).
type SellerSuggestionRepository interface {
	// ListConversationStats aggregates conversations of every carteira
	// account of the seller: current-month total + human-touched count
	// over [curFrom, curTo) and previous-month total over [prevFrom,
	// prevTo). Accounts with zero conversations still get a row.
	ListConversationStats(ctx context.Context, sellerUserID string, curFrom, curTo, prevFrom, prevTo time.Time) ([]SellerConvStats, error)
	// ListWhatsAppRisk returns the seller's carteira WhatsApp channels
	// whose cached quality_rating is RED.
	ListWhatsAppRisk(ctx context.Context, sellerUserID string) ([]SellerChannelRisk, error)
	// ListLatestNPS returns the newest NPS response per carteira account
	// inside [from, to).
	ListLatestNPS(ctx context.Context, sellerUserID string, from, to time.Time) ([]SellerNPSSignal, error)
	ListSuggestionStates(ctx context.Context, sellerUserID, cycle string) ([]SellerSuggestionState, error)
	// UpsertSuggestionState records dismissed/done; calling again for the
	// same (seller, account, type, channel, cycle) overwrites the status.
	// channelID is "" for account-level suggestion types.
	UpsertSuggestionState(ctx context.Context, sellerUserID, accountID, suggestionType, channelID, cycle, status string) error
}

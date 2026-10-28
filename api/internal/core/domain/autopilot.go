package domain

import (
	"context"
	"time"
)

// Progressive autopilot (§3.4).
//
// Human-in-the-loop stays the default and the posture: nothing is sent that a
// person did not read. Autopilot is the exception for somebody who has ALREADY
// proved, twenty times over, that they approve the drafts unchanged — at which
// point the queue is not a review, it is a formality that costs them an
// evening.
//
// The rules below are enforced HERE, in the domain, and not in the UI. A
// safety rule that lives in a screen is a safety rule that a second client, a
// script, or an MCP tool walks straight past.
type Autopilot struct {
	AccountID string `json:"-"`

	Enabled bool `json:"enabled"`
	// DailyCap bounds a bad day. Even a perfect model with a wrong prompt is
	// only allowed to be wrong this many times before a human has to look.
	DailyCap int `json:"daily_cap"`
	// MinScore is the lead quality floor. Cold outbound to a badly-matched
	// lead is what burns a domain, and it is the case a tired human catches
	// and an eager one does not.
	MinScore int `json:"min_score"`

	// UneditedStreak counts consecutive approvals sent EXACTLY as drafted.
	// Reset by any edit or discard: the streak is evidence that the model
	// writes what this person would have written, and an edit is that
	// evidence expiring.
	UneditedStreak int `json:"unedited_streak"`

	// OfferedAt records when we last asked. Nil means never — the offer is
	// made once and not nagged.
	OfferedAt *time.Time `json:"offered_at,omitempty"`
	// EnabledAt / EnabledBy is who turned it on, for the day somebody asks.
	EnabledAt *time.Time `json:"enabled_at,omitempty"`
	EnabledBy string     `json:"enabled_by,omitempty"`

	UpdatedAt time.Time `json:"updated_at"`
}

// Defaults, deliberately conservative. Somebody who wants more can raise
// them; nobody gets them by accident.
const (
	// AutopilotStreakToOffer is how much proof it takes before we even ask.
	// Twenty unchanged approvals is roughly a week of work, which is long
	// enough to be a habit and short enough to still be worth automating.
	AutopilotStreakToOffer = 20
	AutopilotDefaultCap    = 20
	// AutopilotDefaultMinScore is high on purpose. Autopilot is for the easy
	// half of the queue; the rest still deserves a person.
	AutopilotDefaultMinScore = 80
)

// EligibleToOffer reports an account that has earned the offer.
func (a *Autopilot) EligibleToOffer() bool {
	return a != nil && !a.Enabled && a.OfferedAt == nil &&
		a.UneditedStreak >= AutopilotStreakToOffer
}

// AutopilotRefusal says why a send was not automated. Returned rather than a
// bare bool so the audit line can name the reason.
type AutopilotRefusal string

const (
	AutopilotAllowed AutopilotRefusal = ""
	// AutopilotOff is the ordinary state: the feature is not on.
	AutopilotOff AutopilotRefusal = "disabled"
	// AutopilotChannelForbidden is the one refusal that can never be
	// configured away — see CanAutoSend.
	AutopilotChannelForbidden AutopilotRefusal = "channel_forbidden"
	AutopilotScoreTooLow      AutopilotRefusal = "score_below_floor"
	AutopilotCapReached       AutopilotRefusal = "daily_cap_reached"
	AutopilotNoText           AutopilotRefusal = "empty_draft"
)

// AutopilotChannelAllowed reports whether a channel type may ever be
// automated.
//
// EMAIL ONLY, and this is not a setting.
//
// A cold WhatsApp message sent by a machine is how a number gets banned by
// Meta — the account that pays for this product loses the channel its whole
// business runs on, permanently, and no amount of "but the user enabled it"
// gives it back. The rule lives in the domain precisely so that no screen,
// script or tool can be the place somebody turns it off.
func AutopilotChannelAllowed(channelType string) bool {
	return channelType == "email"
}

// CanAutoSend is the single gate every automated send passes.
func (a *Autopilot) CanAutoSend(channelType string, leadScore, sentToday int, text string) AutopilotRefusal {
	// Channel first, and before the enabled check, so the refusal reads as
	// "never on WhatsApp" rather than "turn autopilot on".
	if !AutopilotChannelAllowed(channelType) {
		return AutopilotChannelForbidden
	}
	if a == nil || !a.Enabled {
		return AutopilotOff
	}
	if text == "" {
		return AutopilotNoText
	}
	if leadScore < a.EffectiveMinScore() {
		return AutopilotScoreTooLow
	}
	if sentToday >= a.EffectiveDailyCap() {
		return AutopilotCapReached
	}
	return AutopilotAllowed
}

// CanAutoSendGreeting is the gate for an unattended RELATIONSHIP greeting
// (§6.5, F4) — a birthday, a company anniversary, a professional day.
//
// Same rules as CanAutoSend minus the lead-score floor, because a greeting
// has no lead: it goes to somebody who already bought. Applying the
// cold-outbound floor would refuse every one of them, which is not a safety
// property, just a bug wearing one.
//
// What it deliberately keeps is the channel rule and the daily cap. The
// channel rule is the one that protects a WhatsApp number from Meta, and it
// does not become negotiable because the message is friendly. The cap is
// shared with cold outbound on purpose: the promise to the operator is a
// bound on everything the machine says while they are away, not a bound per
// category that adds up to no bound at all.
//
// Only the CALLER decides a routine is low-risk (GreetingIsLowRisk) — a
// maintenance reminder and a check-in never reach here.
func (a *Autopilot) CanAutoSendGreeting(channelType string, sentToday int, text string) AutopilotRefusal {
	if !AutopilotChannelAllowed(channelType) {
		return AutopilotChannelForbidden
	}
	if a == nil || !a.Enabled {
		return AutopilotOff
	}
	if text == "" {
		return AutopilotNoText
	}
	if sentToday >= a.EffectiveDailyCap() {
		return AutopilotCapReached
	}
	return AutopilotAllowed
}

// EffectiveDailyCap and EffectiveMinScore fall back to the defaults when a
// row was written before either existed, or by a client that omitted them.
// A zero cap would mean "unlimited" to a naive reader and "nothing" to the
// comparison above; neither is what an absent value means.
func (a *Autopilot) EffectiveDailyCap() int {
	if a == nil || a.DailyCap <= 0 {
		return AutopilotDefaultCap
	}
	return a.DailyCap
}

func (a *Autopilot) EffectiveMinScore() int {
	if a == nil || a.MinScore <= 0 {
		return AutopilotDefaultMinScore
	}
	return a.MinScore
}

// AutopilotRepository persists the setting and the streak.
type AutopilotRepository interface {
	Get(ctx context.Context, accountID string) (*Autopilot, error)
	Save(ctx context.Context, a *Autopilot) error
	// RecordApproval moves the streak: +1 for an untouched approval, back to
	// zero for anything else. One method rather than two so the reset can
	// never be forgotten at a call site.
	RecordApproval(ctx context.Context, accountID string, unedited bool) error
	// MarkOffered stamps the one time we ask.
	MarkOffered(ctx context.Context, accountID string, at time.Time) error
	// CountSentToday backs the daily cap. Counted from the audit rows rather
	// than a counter column: a counter that drifts silently uncaps the
	// feature, and the rows have to exist anyway.
	CountSentToday(ctx context.Context, accountID string, since time.Time) (int, error)
	// RecordSend writes the audit line. Everything autopilot sends is
	// auditable — it is the only way an operator can answer "what did it do
	// while I was away?".
	RecordSend(ctx context.Context, entry *AutopilotSend) error
	ListSends(ctx context.Context, accountID string, limit, offset int) ([]*AutopilotSend, error)
}

// AutopilotSend is one automated send, kept for audit.
type AutopilotSend struct {
	ID        string    `json:"id"`
	AccountID string    `json:"-"`
	LeadID    string    `json:"lead_id"`
	Recipient string    `json:"recipient"`
	Text      string    `json:"text"`
	LeadScore int       `json:"lead_score"`
	SentAt    time.Time `json:"sent_at"`
}

package domain

import (
	"fmt"
	"time"
)

// Agent represents an AI agent instance
type Agent struct {
	ID                string  `json:"id"`
	AccountID         string  `json:"account_id"`
	Name              string  `json:"name"`
	SystemPrompt      *string `json:"system_prompt,omitempty"`
	PersonalityPrompt *string `json:"personality_prompt,omitempty"` // auto-generated from human messages
	LearnFromHumans   bool    `json:"learn_from_humans"`            // analyze human replies to build personality
	Provider          string  `json:"provider"`
	Model             string  `json:"model"`
	Timezone          string  `json:"timezone"`
	// BusinessHoursStart/End are local-time strings "HH:MM" in the agent's
	// Timezone. Both empty = 24/7 bot. When set, the bot silently skips AI
	// processing outside the window; a human picking up the conversation
	// still works (IsBotPaused semantics unchanged).
	BusinessHoursStart *string `json:"business_hours_start,omitempty" example:"08:00"`
	BusinessHoursEnd   *string `json:"business_hours_end,omitempty" example:"18:00"`
	// BusinessHoursDays is a bitmask of weekdays when the bot responds:
	// bit 0 = Sunday, ..., bit 6 = Saturday. 0 means 24/7 (ignored along
	// with start/end).
	BusinessHoursDays int `json:"business_hours_days,omitempty"`
	// HideAIIdentity = true (default) prevents the bot from admitting it's
	// an AI/virtual assistant, so customers keep conversing as if with a
	// human attendant. Flipping it off lets the bot acknowledge its nature
	// when directly asked.
	HideAIIdentity bool `json:"hide_ai_identity"`
	// SilentOnUnknown = true (default) switches the bot to a silent
	// handoff when it would otherwise admit it doesn't know — the bot
	// emits the [HANDOFF] sentinel instead of a reply; the pipeline
	// swallows it and pauses the bot so a human takes over without the
	// customer seeing a "I don't know" message. Off = bot produces a
	// warm apology in the customer's language (legacy behavior).
	SilentOnUnknown bool      `json:"silent_on_unknown"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// AgentTimezone is a lightweight projection of an agent used by the admin
// timezone-audit endpoint. It carries only the fields needed to flag agents
// whose Timezone fails time.LoadLocation (silent UTC fallback breaks booking).
type AgentTimezone struct {
	ID        string `json:"id"`
	AccountID string `json:"account_id"`
	Name      string `json:"name"`
	Timezone  string `json:"timezone"`
}

// IsValidTimezone reports whether tz resolves via the Go tzdata, i.e. whether
// AgentRunner will use it instead of silently falling back to UTC.
func IsValidTimezone(tz string) bool {
	loc, err := time.LoadLocation(tz)
	return err == nil && loc != nil
}

// IsWithinBusinessHours reports whether the given instant falls inside the
// agent's configured window. Returns true when business hours are not
// configured (24/7 behavior preserved). Invalid timezone or malformed
// HH:MM strings also return true so a misconfiguration never silences a
// working bot — operators notice the issue via logs.
func (a *Agent) IsWithinBusinessHours(now time.Time) bool {
	if a.BusinessHoursStart == nil || a.BusinessHoursEnd == nil || a.BusinessHoursDays == 0 {
		return true
	}
	loc, err := time.LoadLocation(a.Timezone)
	if err != nil || loc == nil {
		return true
	}
	local := now.In(loc)
	if a.BusinessHoursDays&(1<<int(local.Weekday())) == 0 {
		return false
	}
	startH, startM, ok1 := parseHHMM(*a.BusinessHoursStart)
	endH, endM, ok2 := parseHHMM(*a.BusinessHoursEnd)
	if !ok1 || !ok2 {
		return true
	}
	mins := local.Hour()*60 + local.Minute()
	startMins := startH*60 + startM
	endMins := endH*60 + endM
	if startMins == endMins {
		return true
	}
	if startMins < endMins {
		return mins >= startMins && mins < endMins
	}
	// Window crosses midnight (e.g. 22:00-06:00).
	return mins >= startMins || mins < endMins
}

func parseHHMM(s string) (hour, min int, ok bool) {
	if len(s) < 4 || len(s) > 5 {
		return 0, 0, false
	}
	var h, m int
	n, err := fmt.Sscanf(s, "%d:%d", &h, &m)
	if err != nil || n != 2 || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}

package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Human-in-the-loop is the product's posture. Autopilot is the one exception,
// and these tests are about the rules that keep it an exception.

func enabledAutopilot() *Autopilot {
	return &Autopilot{Enabled: true, DailyCap: 20, MinScore: 80}
}

// THE rule. A cold WhatsApp message sent by a machine is how a number gets
// banned by Meta — the account loses the channel its whole business runs on,
// permanently, and "but the user enabled it" does not give it back.
func TestAutopilot_NeverSendsOnWhatsApp(t *testing.T) {
	a := enabledAutopilot()

	for _, channel := range []string{"whatsapp", "telegram", "instagram", "sms"} {
		assert.Equal(t, AutopilotChannelForbidden,
			a.CanAutoSend(channel, 100, 0, "texto"), "channel %q", channel)
	}
	assert.Equal(t, AutopilotAllowed, a.CanAutoSend("email", 100, 0, "texto"))
}

// The channel refusal outranks even "turn it on", so the message reads as
// "never on WhatsApp" rather than "enable autopilot".
func TestAutopilot_ChannelIsCheckedBeforeEnabled(t *testing.T) {
	off := &Autopilot{Enabled: false}

	assert.Equal(t, AutopilotChannelForbidden, off.CanAutoSend("whatsapp", 100, 0, "x"))
}

func TestAutopilot_OffMeansOff(t *testing.T) {
	assert.Equal(t, AutopilotOff,
		(&Autopilot{Enabled: false}).CanAutoSend("email", 100, 0, "x"))
	// A nil setting is an account that never touched the feature.
	var missing *Autopilot
	assert.Equal(t, AutopilotOff, missing.CanAutoSend("email", 100, 0, "x"))
}

// Autopilot is for the easy half of the queue; the rest deserves a person.
func TestAutopilot_RefusesBelowTheScoreFloor(t *testing.T) {
	a := enabledAutopilot()

	assert.Equal(t, AutopilotScoreTooLow, a.CanAutoSend("email", 79, 0, "x"))
	assert.Equal(t, AutopilotAllowed, a.CanAutoSend("email", 80, 0, "x"))
}

// Even a perfect model with a wrong prompt only gets to be wrong this many
// times before a human has to look.
func TestAutopilot_StopsAtTheDailyCap(t *testing.T) {
	a := enabledAutopilot()

	assert.Equal(t, AutopilotAllowed, a.CanAutoSend("email", 100, 19, "x"))
	assert.Equal(t, AutopilotCapReached, a.CanAutoSend("email", 100, 20, "x"))
}

func TestAutopilot_RefusesAnEmptyDraft(t *testing.T) {
	assert.Equal(t, AutopilotNoText,
		enabledAutopilot().CanAutoSend("email", 100, 0, ""))
}

// A zero would mean "unlimited" to a naive reader and "nothing" to the
// comparison. Neither is what an absent value means.
func TestAutopilot_AbsentLimitsFallBackToTheDefaults(t *testing.T) {
	a := &Autopilot{Enabled: true}

	assert.Equal(t, AutopilotDefaultCap, a.EffectiveDailyCap())
	assert.Equal(t, AutopilotDefaultMinScore, a.EffectiveMinScore())
	assert.Equal(t, AutopilotScoreTooLow, a.CanAutoSend("email", 50, 0, "x"))
}

// ── Earning the offer ─────────────────────────────────────────────────────

func TestAutopilot_OfferedOnlyAfterEnoughProof(t *testing.T) {
	almost := &Autopilot{UneditedStreak: AutopilotStreakToOffer - 1}
	earned := &Autopilot{UneditedStreak: AutopilotStreakToOffer}

	assert.False(t, almost.EligibleToOffer())
	assert.True(t, earned.EligibleToOffer())
}

// The offer is made once and not nagged.
func TestAutopilot_NotOfferedTwice(t *testing.T) {
	now := time.Now()
	a := &Autopilot{UneditedStreak: 50, OfferedAt: &now}

	assert.False(t, a.EligibleToOffer())
}

// Somebody already running it does not need to be asked.
func TestAutopilot_NotOfferedWhenAlreadyOn(t *testing.T) {
	a := &Autopilot{Enabled: true, UneditedStreak: 50}

	assert.False(t, a.EligibleToOffer())
}

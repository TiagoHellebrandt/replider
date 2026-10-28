package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A cadence exists because the biggest reason a B2B deal dies is that nobody
// followed up a third time — and the second biggest is that somebody followed
// up eleven times. These tests are about the rules that keep it the first.

var approved = map[string]bool{"followup_aprovado": true}

func steps(kinds ...SequenceStepKind) []SequenceStep {
	out := make([]SequenceStep, 0, len(kinds))
	for i, k := range kinds {
		step := SequenceStep{Order: i + 1, Kind: k}
		switch k {
		case StepEmail, StepSMS:
			step.Body = "Bom dia!"
		case StepHumanTask:
			step.TaskTitle = "Ligar"
		case StepWhatsApp:
			step.TemplateID = "followup_aprovado"
		}
		out = append(out, step)
	}
	return out
}

func codes(violations []SequenceViolation) []string {
	out := make([]string, 0, len(violations))
	for _, v := range violations {
		out = append(out, v.Code)
	}
	return out
}

func TestSequence_AValidCadencePasses(t *testing.T) {
	s := &Sequence{Steps: steps(StepEmail, StepWait, StepEmail, StepHumanTask)}

	assert.Empty(t, s.Validate(approved))
	assert.True(t, s.Runnable(approved))
}

// THE rule that protects the customer's number. A WhatsApp step this early is
// by definition cold — the lead has not replied to anything — and spending a
// template send on somebody who has not reacted to two emails is paying the
// most for the least likely.
func TestSequence_RefusesToOpenWithWhatsApp(t *testing.T) {
	s := &Sequence{Steps: steps(StepWhatsApp, StepEmail)}

	assert.Contains(t, codes(s.Validate(approved)), ViolationOpensWithWhatsApp)
}

func TestSequence_WhatsAppIsFineOnceTheLeadHasSeenTwoEmails(t *testing.T) {
	s := &Sequence{Steps: steps(StepEmail, StepEmail, StepWhatsApp)}

	assert.Empty(t, s.Validate(approved))
}

// Without an approved template the message is not delivered at all, and
// sending against one is what gets a number banned.
func TestSequence_WhatsAppNeedsAnApprovedTemplate(t *testing.T) {
	s := &Sequence{Steps: []SequenceStep{
		{Order: 1, Kind: StepEmail, Body: "oi"},
		{Order: 2, Kind: StepEmail, Body: "oi de novo"},
		{Order: 3, Kind: StepWhatsApp, TemplateID: "nao_aprovado"},
	}}

	assert.Contains(t, codes(s.Validate(approved)), ViolationWhatsAppNeedsTemplate)
}

// An account with no approved templates cannot have a WhatsApp step. Failing
// closed is the safe direction: an unapproved template is undeliverable.
func TestSequence_NoApprovedTemplatesBlocksEveryWhatsAppStep(t *testing.T) {
	s := &Sequence{Steps: steps(StepEmail, StepEmail, StepWhatsApp)}

	assert.Contains(t, codes(s.Validate(map[string]bool{})),
		ViolationWhatsAppNeedsTemplate)
}

func TestSequence_AnEmailStepNeedsSomethingToSay(t *testing.T) {
	s := &Sequence{Steps: []SequenceStep{{Order: 1, Kind: StepEmail}}}

	assert.Contains(t, codes(s.Validate(approved)), ViolationEmptyStep)
}

// "Tarefa" with no instruction is a notification nobody can action.
func TestSequence_AHumanTaskNeedsAnInstruction(t *testing.T) {
	s := &Sequence{Steps: []SequenceStep{{Order: 1, Kind: StepHumanTask}}}

	assert.Contains(t, codes(s.Validate(approved)), ViolationTaskNeedsTitle)
}

// A cadence that ends on a wait ends by doing nothing, forever — almost
// always a step somebody deleted and did not notice.
func TestSequence_RefusesToEndOnAWait(t *testing.T) {
	s := &Sequence{Steps: steps(StepEmail, StepWait)}

	assert.Contains(t, codes(s.Validate(approved)), ViolationWaitIsLast)
}

func TestSequence_AnEmptyCadenceIsRefused(t *testing.T) {
	assert.Equal(t, []string{ViolationNoSteps}, codes((&Sequence{}).Validate(approved)))
}

// An editor that reports one problem per save turns a five-step cadence into
// five round trips.
func TestSequence_ReportsEveryViolationAtOnce(t *testing.T) {
	s := &Sequence{Steps: []SequenceStep{
		{Order: 1, Kind: StepWhatsApp, TemplateID: "nao_aprovado"},
		{Order: 2, Kind: StepEmail},
		{Order: 3, Kind: StepWait},
	}}

	got := codes(s.Validate(approved))

	assert.Contains(t, got, ViolationOpensWithWhatsApp)
	assert.Contains(t, got, ViolationWhatsAppNeedsTemplate)
	assert.Contains(t, got, ViolationEmptyStep)
	assert.Contains(t, got, ViolationWaitIsLast)
}

// The §1.5 shape: the channel that costs nothing goes first, and the one that
// costs a number's reputation is a decision somebody makes after seeing a
// reaction.
func TestDefaultSequence_OpensWithEmailAndNeverAutomatesWhatsApp(t *testing.T) {
	s := DefaultSequence("Padrão")

	require.NotEmpty(t, s.Steps)
	assert.Equal(t, StepEmail, s.Steps[0].Kind)
	for _, step := range s.Steps {
		assert.NotEqual(t, StepWhatsApp, step.Kind)
	}
	assert.True(t, s.StopOnReply)

	// It does NOT validate yet, and that is correct: the default is a SHAPE,
	// and two empty emails cannot be sent. The violations are the editor's
	// to-do list — "escreva a mensagem" on exactly the steps that need one.
	violations := s.Validate(map[string]bool{})
	require.NotEmpty(t, violations)
	for _, v := range violations {
		assert.Equal(t, ViolationEmptyStep, v.Code)
	}
}

// ── Business hours ────────────────────────────────────────────────────────

var saoPaulo = time.FixedZone("BRT", -3*3600)

// A cold email at 03:14 reads as a machine, which is exactly what it must not.
func TestNextSendWindow_MovesTheEarlyMorningToNineAM(t *testing.T) {
	at := time.Date(2026, 3, 10, 3, 14, 0, 0, saoPaulo) // a Tuesday

	got := NextSendWindow(at, saoPaulo)

	assert.Equal(t, 9, got.Hour())
	assert.Equal(t, 10, got.Day())
}

func TestNextSendWindow_LeavesAWorkingHourAlone(t *testing.T) {
	at := time.Date(2026, 3, 10, 14, 30, 0, 0, saoPaulo)

	assert.Equal(t, at, NextSendWindow(at, saoPaulo))
}

func TestNextSendWindow_PushesTheEveningToTomorrow(t *testing.T) {
	at := time.Date(2026, 3, 10, 21, 0, 0, 0, saoPaulo)

	got := NextSendWindow(at, saoPaulo)

	assert.Equal(t, 11, got.Day())
	assert.Equal(t, 9, got.Hour())
}

// Nobody reads a cold email on Saturday, and one that arrives then is under
// forty others by Monday.
func TestNextSendWindow_SkipsTheWeekend(t *testing.T) {
	saturday := time.Date(2026, 3, 14, 10, 0, 0, 0, saoPaulo)
	sunday := time.Date(2026, 3, 15, 10, 0, 0, 0, saoPaulo)

	assert.Equal(t, time.Monday, NextSendWindow(saturday, saoPaulo).Weekday())
	assert.Equal(t, time.Monday, NextSendWindow(sunday, saoPaulo).Weekday())
}

// It must never move a send EARLIER: a cadence that fires ahead of schedule
// arrives before the wait somebody configured.
func TestNextSendWindow_NeverMovesASendEarlier(t *testing.T) {
	for _, at := range []time.Time{
		time.Date(2026, 3, 10, 3, 0, 0, 0, saoPaulo),
		time.Date(2026, 3, 10, 21, 0, 0, 0, saoPaulo),
		time.Date(2026, 3, 14, 10, 0, 0, 0, saoPaulo),
		time.Date(2026, 3, 15, 23, 59, 0, 0, saoPaulo),
	} {
		assert.False(t, NextSendWindow(at, saoPaulo).Before(at), "input %v", at)
	}
}

// ── Step stats ────────────────────────────────────────────────────────────

// Zero replies out of zero is unknown, not zero. Rendering 0% would condemn a
// step nobody has run.
func TestSequenceStepStat_UnusedStepHasNoRate(t *testing.T) {
	assert.Nil(t, SequenceStepStat{}.ReplyRate())

	rate := SequenceStepStat{Reached: 10, Replied: 3}.ReplyRate()
	require.NotNil(t, rate)
	assert.InDelta(t, 0.3, *rate, 0.001)
}

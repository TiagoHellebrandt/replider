package domain

import "time"

// Onboarding nudge stages, expressed as days after signup. Each stage maps
// to one activation milestone we nudge the account toward (US-182):
//
//	D1 → connect a messaging channel    (OnboardingNudgeStageChannel)
//	D3 → add knowledge to the agent     (OnboardingNudgeStageKnowledge)
//	D7 → share the agent / community    (OnboardingNudgeStageShare)
const (
	OnboardingNudgeStageChannel   = 1
	OnboardingNudgeStageKnowledge = 3
	OnboardingNudgeStageShare     = 7
)

// OnboardingNudgeStages lists the milestones in ascending order;
// NextOnboardingNudgeStage relies on the order.
var OnboardingNudgeStages = []int{
	OnboardingNudgeStageChannel,
	OnboardingNudgeStageKnowledge,
	OnboardingNudgeStageShare,
}

// OnboardingNudgeAccount is one row of the onboarding-nudge worker's
// due-list: a recently-created account that has not yet received every
// activation nudge. ChannelConnected / KnowledgeAdded are derived from the
// same EXISTS lookups the onboarding-progress endpoint uses, so a stage
// whose step is already done is skipped (the AC's "skip if already done").
// The admin recipient is resolved separately so this stays a cheap
// accounts-only scan.
type OnboardingNudgeAccount struct {
	AccountID string
	Name      string
	CreatedAt time.Time
	// StageSent is the last milestone day already emailed or skipped (0 = none).
	StageSent        int
	ChannelConnected bool
	KnowledgeAdded   bool
}

// NextOnboardingNudgeStage returns the milestone day that is due now: the
// highest stage that has already elapsed (stage <= daysSinceSignup) and has
// not yet been sent (stage > stageSent). Returns 0 when nothing is due.
//
// Picking the HIGHEST due stage (rather than the next-after-stageSent)
// collapses a backlog: an account that's suddenly 7 days old with nothing
// sent gets the day-7 nudge once, not a burst of D1+D3+D7. Mirrors
// NextReactivationStage.
func NextOnboardingNudgeStage(daysSinceSignup, stageSent int) int {
	next := 0
	for _, s := range OnboardingNudgeStages {
		if s <= daysSinceSignup && s > stageSent {
			next = s
		}
	}
	return next
}

// OnboardingNudgeStageDone reports whether the activation milestone a stage
// nudges toward is already complete, in which case the nudge is skipped
// (but the stamp still advances so the drip moves on). The share stage has
// no completion gate — sharing is always worth a nudge.
func OnboardingNudgeStageDone(stage int, acc OnboardingNudgeAccount) bool {
	switch stage {
	case OnboardingNudgeStageChannel:
		return acc.ChannelConnected
	case OnboardingNudgeStageKnowledge:
		return acc.KnowledgeAdded
	default:
		return false
	}
}

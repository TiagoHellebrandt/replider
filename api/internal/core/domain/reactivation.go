package domain

import "time"

// ReactivationStages are the milestones (in days after a trial lapses) at
// which we email a reactivation nudge to a never-converted account
// (US-181). Kept ascending — NextReactivationStage relies on the order.
var ReactivationStages = []int{7, 14, 30}

// ReactivationAccount is one row of the reactivation worker's due-list: a
// still-trialing account whose trial has expired and which has not yet
// received every reactivation nudge. The admin recipient is resolved
// separately so this stays a cheap accounts-only scan.
type ReactivationAccount struct {
	AccountID      string
	Name           string
	TrialExpiresAt time.Time
	// StageSent is the last milestone day already emailed (0 = none).
	StageSent int
}

// NextReactivationStage returns the milestone day that is due to be
// emailed now: the highest stage that has already elapsed
// (stage <= daysSinceTrial) and has not yet been sent (stage > stageSent).
// Returns 0 when nothing is due.
//
// Picking the HIGHEST due stage (rather than the next-after-stageSent)
// deliberately collapses a backlog: if the worker was down and an account
// is suddenly 20 days lapsed with nothing sent, it gets the day-14 nudge
// once, not a burst of day-7 + day-14. The day-30 nudge still follows
// later on its own.
func NextReactivationStage(daysSinceTrial, stageSent int) int {
	next := 0
	for _, s := range ReactivationStages {
		if s <= daysSinceTrial && s > stageSent {
			next = s
		}
	}
	return next
}

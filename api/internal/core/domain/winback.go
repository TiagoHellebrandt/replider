package domain

import "time"

// WinBackStages are the follow-up milestones (in days after a paid
// subscription is canceled) at which we re-email a churned customer the
// still-valid recovery coupon (US-179). The day-0 offer is sent inline on
// cancellation; these are the reminders. Kept ascending —
// NextWinBackStage relies on the order.
var WinBackStages = []int{7, 30}

// The automatic recovery coupon minted on cancellation: 50% off for 3
// months. Centralized so the Stripe adapter, the email copy, and the
// reactivate link all quote the same offer.
const (
	WinBackDiscountPercent = 50
	WinBackDurationMonths  = 3
)

// WinBackAccount is one row of the win-back worker's follow-up due-list: a
// recently-churned account still inside the recovery window. The admin
// recipient is resolved separately so this stays a cheap accounts-only
// scan.
type WinBackAccount struct {
	AccountID  string
	Name       string
	StartedAt  time.Time
	CouponCode string
	// StageSent is the last follow-up milestone already emailed (0 = only
	// the initial offer went out).
	StageSent int
}

// NextWinBackStage returns the follow-up milestone day due to be emailed
// now: the highest stage that has already elapsed (stage <= daysSince) and
// has not yet been sent (stage > stageSent). Returns 0 when nothing is due.
//
// Picking the HIGHEST due stage (rather than the next-after-stageSent)
// deliberately collapses a backlog into a single email, mirroring
// NextReactivationStage — see its godoc for the rationale.
func NextWinBackStage(daysSince, stageSent int) int {
	next := 0
	for _, s := range WinBackStages {
		if s <= daysSince && s > stageSent {
			next = s
		}
	}
	return next
}

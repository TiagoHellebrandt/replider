package domain

import (
	"context"
	"time"
)

// PIX renewal + dunning ladder (US-M014).
//
// A card subscription renews itself. PIX does not: there is nothing on
// file to charge, so the platform asks again every cycle and then has to
// decide how long to keep serving someone who has not paid. That decision
// is this file — deliberately a pure function over (days from renewal,
// stage already sent), so the whole ladder is testable without waiting a
// week or faking a database.

// PixDunningStage is a rung on the ladder. The numbers are persisted
// (accounts.pix_cycle_stage_sent), so they are append-only: renumbering
// them would re-send or skip a step for every account mid-cycle.
type PixDunningStage int

const (
	// PixStageNone means nothing has been sent for this period.
	PixStageNone PixDunningStage = 0
	// PixStageCharge is the renewal charge, sent 3 days BEFORE the period
	// ends — PIX is manual, so the customer needs a working day or two to
	// actually pay it.
	PixStageCharge PixDunningStage = 1
	// PixStageDueReminder is the nudge on the renewal date itself.
	PixStageDueReminder PixDunningStage = 2
	// PixStageOverdueReminder is the second nudge, 2 days late.
	PixStageOverdueReminder PixDunningStage = 3
	// PixStageSuspended cuts service at 7 days late.
	PixStageSuspended PixDunningStage = 4
)

// pixLadder maps "days relative to the renewal date" to the stage that
// should have happened by then. Negative = before renewal.
var pixLadder = []struct {
	day   int
	stage PixDunningStage
}{
	{-3, PixStageCharge},
	{0, PixStageDueReminder},
	{2, PixStageOverdueReminder},
	{7, PixStageSuspended},
}

// NextPixDunningStage returns the stage due now, or PixStageNone when
// there is nothing to do.
//
// It returns the HIGHEST elapsed-but-unsent rung, not the next one up: a
// worker that was down for a week must not walk an account through four
// emails on the way to suspending it. The customer gets the message that
// is true today.
func NextPixDunningStage(daysFromRenewal int, alreadySent PixDunningStage) PixDunningStage {
	due := PixStageNone
	for _, rung := range pixLadder {
		if daysFromRenewal >= rung.day && rung.stage > alreadySent {
			due = rung.stage
		}
	}
	return due
}

// DaysFromRenewal is whole days between now and the renewal date,
// negative before it. Both are truncated to the day so a job that runs at
// 08:00 and one that runs at 23:00 agree on which rung is due — otherwise
// an account could be suspended a run early purely by clock time.
func DaysFromRenewal(now, periodEnd time.Time) int {
	n := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	e := time.Date(periodEnd.Year(), periodEnd.Month(), periodEnd.Day(), 0, 0, 0, 0, time.UTC)
	return int(n.Sub(e).Hours() / 24)
}

// PixRenewalAccount is one account's renewal state.
type PixRenewalAccount struct {
	AccountID string
	PlanID    string
	PeriodEnd time.Time
	// StageSent is the furthest rung already delivered FOR PeriodEnd. A
	// stamp from an older period reads as PixStageNone (see
	// StageForPeriod) so a renewal starts the ladder over.
	StageSent PixDunningStage
	// StagePeriodEnd is the period StageSent belongs to.
	StagePeriodEnd *time.Time
	// ChargeID is the outstanding PIX charge, so a reminder points at the
	// same code instead of minting a second one the customer might also
	// pay.
	ChargeID string
}

// StageForPeriod is StageSent, but only when it belongs to the current
// period. A subscription that renewed starts clean.
func (a PixRenewalAccount) StageForPeriod() PixDunningStage {
	if a.StagePeriodEnd == nil || !a.StagePeriodEnd.Equal(a.PeriodEnd) {
		return PixStageNone
	}
	return a.StageSent
}

// PixRenewalRepository is the ladder's persistence.
type PixRenewalRepository interface {
	// ListDue returns accounts whose renewal is inside the ladder's reach.
	ListDue(ctx context.Context, now time.Time, limit int) ([]PixRenewalAccount, error)
	// MarkStage records the furthest rung delivered for a period.
	MarkStage(ctx context.Context, accountID string, stage PixDunningStage, periodEnd time.Time) error
	// SetChargeID remembers the outstanding charge.
	SetChargeID(ctx context.Context, accountID, chargeID string) error
}

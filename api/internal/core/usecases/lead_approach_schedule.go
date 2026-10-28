package usecases

import (
	"context"
	"log/slog"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/replider/api/internal/core/domain"
)

// Approving past the mailbox's daily cap schedules the send instead of
// refusing it (Ofertas → Hoje, phase 1b).
//
// The human's decision is already made at the moment they click "aprovar e
// enviar"; making them come back tomorrow to repeat it was busywork the
// error message used to disguise as a feature ("o restante sai amanhã").
// The worker replays the decision through ApproveSend itself — every gate a
// live click faces (warmup, caps, opt-out, suppression) still runs at fire
// time.

// scheduledSendHourLocal is when a scheduled approach goes out, in the
// account's timezone: the start of the business morning, when a cold email
// is read rather than buried.
const scheduledSendHourLocal = 8

// maxScheduleAttempts bounds transient failures. A cap refusal at fire time
// is NOT an attempt — it reschedules to the next morning instead.
const maxScheduleAttempts = 3

// SetScheduleRepo wires the schedule store (optional; unwired keeps the old
// hard refusal at the daily cap).
func (uc *ApproachUseCase) SetScheduleRepo(r domain.ApproachScheduleRepository) {
	uc.scheduleRepo = r
}

// CanScheduleApproach reports whether the fallback exists at all — the
// handler uses it to decide between scheduling and the legacy refusal.
func (uc *ApproachUseCase) CanScheduleApproach() bool { return uc.scheduleRepo != nil }

// ScheduleApproach records the approved send for the next morning the cap
// allows. The TEXT rides on the lead's approach_draft (persisted here), so
// an edit made while the send waits ships instead of a stale copy; the
// subject is frozen on the row.
func (uc *ApproachUseCase) ScheduleApproach(ctx context.Context, accountID, leadID string, in ApproachSendInput) (time.Time, error) {
	if uc.scheduleRepo == nil {
		return time.Time{}, domain.ErrEmailDailyCap
	}
	lead, _, err := uc.ownedLeadAndFunnel(ctx, accountID, leadID)
	if err != nil {
		return time.Time{}, err
	}
	if lead.Status != domain.LeadStatusPendingApproval {
		return time.Time{}, domain.ErrLeadNotPendingApproval
	}

	// Persist the human-edited text as the draft the worker will send.
	if _, err := uc.SaveDraft(ctx, accountID, leadID, in.Text); err != nil {
		return time.Time{}, err
	}

	sendAt := uc.nextScheduledSendAt(ctx, accountID)
	if err := uc.scheduleRepo.Upsert(ctx, &domain.ApproachSchedule{
		ID:        ulid.Make().String(),
		AccountID: accountID,
		LeadID:    leadID,
		Subject:   in.Subject,
		SendAt:    sendAt,
	}); err != nil {
		return time.Time{}, err
	}
	return sendAt, nil
}

// nextScheduledSendAt is tomorrow at the business-morning hour in the
// account's timezone. Always tomorrow: this path only runs when TODAY's cap
// is already spent, so a same-day slot would just bounce at fire time.
func (uc *ApproachUseCase) nextScheduledSendAt(ctx context.Context, accountID string) time.Time {
	loc := time.UTC
	if account, err := uc.accountRepo.GetByID(ctx, accountID); err == nil && account != nil && account.Timezone != "" {
		if l, err := time.LoadLocation(account.Timezone); err == nil {
			loc = l
		}
	}
	now := uc.now().In(loc)
	return time.Date(now.Year(), now.Month(), now.Day(), scheduledSendHourLocal, 0, 0, 0, loc).
		AddDate(0, 0, 1)
}

// ProcessDueScheduledApproaches drains due rows, called by the worker on a
// tick. Returns how many were sent.
func (uc *ApproachUseCase) ProcessDueScheduledApproaches(ctx context.Context, limit int32) (int, error) {
	if uc.scheduleRepo == nil {
		return 0, nil
	}
	due, err := uc.scheduleRepo.ClaimDue(ctx, limit)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, row := range due {
		if ctx.Err() != nil {
			return sent, ctx.Err()
		}
		if uc.fireScheduledApproach(ctx, row) {
			sent++
		}
	}
	return sent, nil
}

// fireScheduledApproach replays one approved send. Reports true on delivery.
func (uc *ApproachUseCase) fireScheduledApproach(ctx context.Context, row *domain.ApproachSchedule) bool {
	lead, err := uc.leadRepo.GetByID(ctx, row.LeadID)
	if err != nil || lead == nil || lead.AccountID != row.AccountID {
		_ = uc.scheduleRepo.MarkFailed(ctx, row.ID, "lead not found")
		return false
	}
	// A lead somebody discarded or hand-sent while the row waited is a
	// decision that supersedes this one — the schedule dies quietly.
	if lead.Status != domain.LeadStatusPendingApproval {
		_ = uc.scheduleRepo.MarkFailed(ctx, row.ID, "lead no longer pending approval")
		return false
	}

	_, err = uc.ApproveSend(ctx, row.AccountID, row.LeadID, ApproachSendInput{
		Text:    lead.ApproachDraft,
		Subject: row.Subject,
	})
	if err == nil {
		_ = uc.scheduleRepo.MarkSent(ctx, row.ID, uc.now())
		return true
	}

	// Cap still spent (the day's budget was smaller than the backlog):
	// tomorrow morning, without burning an attempt — waiting is the plan
	// working, not a failure.
	if appErrorCode(err) == domain.ErrEmailDailyCap.Code {
		next := uc.nextScheduledSendAt(ctx, row.AccountID)
		_ = uc.scheduleRepo.Reschedule(ctx, row.ID, next)
		slog.InfoContext(ctx, "scheduled approach: cap still spent, moved to next morning",
			"lead_id", row.LeadID, "send_at", next)
		return false
	}

	if row.Attempts+1 >= maxScheduleAttempts {
		_ = uc.scheduleRepo.MarkFailed(ctx, row.ID, err.Error())
		slog.WarnContext(ctx, "scheduled approach: giving up",
			"lead_id", row.LeadID, "error", err)
		return false
	}
	_ = uc.scheduleRepo.TouchAttempt(ctx, row.ID, err.Error())
	return false
}

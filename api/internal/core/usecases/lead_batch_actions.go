package usecases

import (
	"context"
	"errors"

	"github.com/replider/api/internal/core/domain"
)

// Batch actions over the approval queue (Ofertas → Hoje, phase 1).
//
// The queue accumulates drafts faster than anyone decides them — the shipped
// account sat on 38 generic Google Places drafts nobody would ever approve
// one by one. Both operations below are loops over the existing single-lead
// paths on purpose: the batch must obey exactly the same rules (ownership,
// experiment bookkeeping, spend caps, the empty-catalogue gate) as clicking
// forty times, or it becomes a side door.

// maxBatchLeadIDs bounds one discard request. The queue endpoint pages at
// 200, so a single page can always be discarded in one call.
const maxBatchLeadIDs = 200

// maxRedraftPerCall bounds one redraft request. Each redraft is a live AI
// call (seconds each); eight keeps the request comfortably inside HTTP
// timeouts and the client loops with the remainder.
const maxRedraftPerCall = 8

var errBatchTooLarge = &domain.AppError{
	Code:    "LEAD_BATCH_TOO_LARGE",
	Message: "too many lead ids in one batch (max 200)",
}

type BatchDiscardResult struct {
	Discarded int `json:"discarded"`
	// Skipped counts leads that were not waiting for approval (already sent,
	// already lost, not the caller's) — not an error, just not queue items.
	Skipped int `json:"skipped"`
}

// DiscardBatch moves every listed lead that is still waiting for approval to
// lost. Each lead passes the same ownership check as a manual discard
// (MutableForActor — a vendedor cannot sweep a colleague's queue), and the
// write routes through UpdateStatus so the A/B experiment hears about each
// discard exactly like a manual one.
func (uc *LeadUseCase) DiscardBatch(ctx context.Context, actor domain.Actor, leadIDs []string) (*BatchDiscardResult, error) {
	if len(leadIDs) > maxBatchLeadIDs {
		return nil, errBatchTooLarge
	}
	res := &BatchDiscardResult{}
	for _, id := range leadIDs {
		lead, err := uc.MutableForActor(ctx, actor, id)
		if err != nil || lead == nil {
			res.Skipped++
			continue
		}
		if lead.Status != domain.LeadStatusPendingApproval {
			res.Skipped++
			continue
		}
		if _, err := uc.UpdateStatus(ctx, actor.AccountID, id, domain.LeadStatusLost); err != nil {
			res.Skipped++
			continue
		}
		res.Discarded++
	}
	return res, nil
}

type BatchRedraftResult struct {
	Redrafted int `json:"redrafted"`
	// FailedLeadIDs are per-lead refusals (opted out, not draftable). The
	// rest of the batch still ran.
	FailedLeadIDs []string `json:"failed_lead_ids"`
	// RemainingLeadIDs were not attempted: past the per-call cap, or after
	// an account-wide stop (spend cap, empty catalogue). The client sends
	// them again on the next call.
	RemainingLeadIDs []string `json:"remaining_lead_ids"`
	// StoppedReason is the AppError code of an account-wide refusal that
	// aborted the loop, empty when the batch simply hit the per-call cap.
	StoppedReason string `json:"stopped_reason,omitempty"`
}

// RedraftBatch regenerates the AI draft for the listed leads, at most
// maxRedraftPerCall per request. An account-wide refusal (spend cap, no
// products in the catalogue) stops the loop immediately: every remaining
// lead would fail the same way, and each attempt before the stop already
// cost a provider call.
func (uc *ApproachUseCase) RedraftBatch(ctx context.Context, accountID string, leadIDs []string) (*BatchRedraftResult, error) {
	if len(leadIDs) > maxBatchLeadIDs {
		return nil, errBatchTooLarge
	}
	res := &BatchRedraftResult{FailedLeadIDs: []string{}, RemainingLeadIDs: []string{}}
	for i, id := range leadIDs {
		if res.Redrafted >= maxRedraftPerCall {
			res.RemainingLeadIDs = append(res.RemainingLeadIDs, leadIDs[i:]...)
			break
		}
		if _, err := uc.Draft(ctx, accountID, id); err != nil {
			if code := appErrorCode(err); domain.IsAccountLevelDraftRefusal(code) {
				res.StoppedReason = code
				res.RemainingLeadIDs = append(res.RemainingLeadIDs, leadIDs[i:]...)
				break
			}
			res.FailedLeadIDs = append(res.FailedLeadIDs, id)
			continue
		}
		res.Redrafted++
	}
	return res, nil
}

func appErrorCode(err error) string {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

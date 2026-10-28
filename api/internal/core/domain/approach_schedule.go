package domain

import (
	"context"
	"time"
)

// ApproachSchedule is one approved-but-capped send waiting for the mailbox's
// daily budget to renew (Ofertas → Hoje, phase 1b).
//
// It records the DECISION, not the message: the human already said "send
// this", and the worker replays that decision through the normal ApproveSend
// gates. The body is read from the lead's approach_draft at fire time so a
// later edit ships instead of a stale copy; only the subject is frozen here.
type ApproachSchedule struct {
	ID        string     `json:"id"`
	AccountID string     `json:"account_id"`
	LeadID    string     `json:"lead_id"`
	Subject   string     `json:"subject"`
	SendAt    time.Time  `json:"send_at"`
	Status    string     `json:"status"`
	Attempts  int        `json:"attempts"`
	LastError string     `json:"last_error,omitempty"`
	SentAt    *time.Time `json:"sent_at,omitempty"`
}

// ApproachScheduleRepository persists the waiting sends.
type ApproachScheduleRepository interface {
	// Upsert refreshes the lead's live schedule row or creates one — a
	// second approval while the first waits must move the send, not queue a
	// duplicate.
	Upsert(ctx context.Context, s *ApproachSchedule) error
	// ClaimDue flips a batch of due pending rows to processing and returns
	// them, multi-replica safe (FOR UPDATE SKIP LOCKED).
	ClaimDue(ctx context.Context, limit int32) ([]*ApproachSchedule, error)
	MarkSent(ctx context.Context, id string, at time.Time) error
	// Reschedule sends a claimed row back to pending at a later time —
	// today's cap can be smaller than the backlog it queued.
	Reschedule(ctx context.Context, id string, at time.Time) error
	MarkFailed(ctx context.Context, id, reason string) error
	// TouchAttempt counts a transient failure and returns the row to
	// pending for the next tick.
	TouchAttempt(ctx context.Context, id, reason string) error
	// PendingByLeadIDs answers which of these leads have a live schedule,
	// so the approval queue can label them instead of hiding them.
	PendingByLeadIDs(ctx context.Context, accountID string, leadIDs []string) (map[string]time.Time, error)
}

package domain

import (
	"context"
	"time"
)

// CancellationSurvey is a single churn-reason submission captured when a
// customer cancels their subscription (US-180). The survey is presented on
// the Stripe Portal cancel return URL; this row is the durable record fed
// into the admin churn-reason breakdown.
type CancellationSurvey struct {
	ID        string    `json:"id"`
	AccountID string    `json:"account_id"`
	UserID    string    `json:"user_id"`
	Reason    string    `json:"reason"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

// CancellationReason is the closed set of structured churn reasons the
// survey offers. Free-text nuance goes in the optional comment; the reason
// is constrained so the admin breakdown is groupable.
const (
	CancellationReasonTooExpensive       = "too_expensive"
	CancellationReasonMissingFeatures    = "missing_features"
	CancellationReasonNotUsing           = "not_using"
	CancellationReasonSwitchedCompetitor = "switched_competitor"
	CancellationReasonTechnicalIssues    = "technical_issues"
	CancellationReasonTemporaryPause     = "temporary_pause"
	CancellationReasonOther              = "other"
)

// CancellationReasons is the whitelist enforced on submission. Anything
// outside this set is rejected with ErrInvalidCancellationReason so the
// breakdown never accrues garbage buckets.
var CancellationReasons = []string{
	CancellationReasonTooExpensive,
	CancellationReasonMissingFeatures,
	CancellationReasonNotUsing,
	CancellationReasonSwitchedCompetitor,
	CancellationReasonTechnicalIssues,
	CancellationReasonTemporaryPause,
	CancellationReasonOther,
}

// IsValidCancellationReason reports whether reason is one of the whitelisted
// structured churn reasons.
func IsValidCancellationReason(reason string) bool {
	for _, r := range CancellationReasons {
		if r == reason {
			return true
		}
	}
	return false
}

// CancellationReasonCount is one row of the admin churn-reason breakdown:
// how many cancellations cited a given reason over the window.
type CancellationReasonCount struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// CancellationSurveyComment is a recent verbatim surfaced on the admin
// dashboard so support can read the qualitative signal behind a reason.
type CancellationSurveyComment struct {
	AccountID string    `json:"account_id"`
	Reason    string    `json:"reason"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

// CancellationSurveyRepository persists cancellation surveys and computes the
// admin breakdown. Raw-SQL repo (cross-account aggregation), deliberately
// separate from the sqlc-generated repositories.
type CancellationSurveyRepository interface {
	Create(ctx context.Context, s *CancellationSurvey) error
	// Breakdown returns the per-reason cancellation counts for surveys
	// created at or after `since`, ordered by count descending.
	Breakdown(ctx context.Context, since time.Time) ([]CancellationReasonCount, error)
	// RecentComments returns up to `limit` non-empty comments created at or
	// after `since`, newest first.
	RecentComments(ctx context.Context, since time.Time, limit int) ([]CancellationSurveyComment, error)
}

// ErrInvalidCancellationReason is returned when a submitted reason is not in
// the CancellationReasons whitelist.
var ErrInvalidCancellationReason = &AppError{Code: "INVALID_CANCELLATION_REASON", Message: "reason must be one of the supported cancellation reasons"}

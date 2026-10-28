package domain

import (
	"context"
	"time"
)

// NPSResponse is a single in-app Net Promoter Score submission (US-178).
// The quarterly "show once" cadence is enforced client-side; this row is
// the durable record fed into the admin NPS dashboard.
type NPSResponse struct {
	ID        string    `json:"id"`
	AccountID string    `json:"account_id"`
	UserID    string    `json:"user_id"`
	Score     int       `json:"score"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

// NPSComment is a recent verbatim surfaced on the admin dashboard so support
// can read the qualitative signal behind the number.
type NPSComment struct {
	AccountID string    `json:"account_id"`
	Score     int       `json:"score"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

// NPSSummary is the aggregated Net Promoter Score over a trailing window
// (US-178). NPS = %promoters (9-10) − %detractors (0-6); passives (7-8) do
// not move the score. Range is [-100, 100].
type NPSSummary struct {
	Total      int `json:"total"`
	Promoters  int `json:"promoters"`
	Passives   int `json:"passives"`
	Detractors int `json:"detractors"`
	// Score is the rounded NPS in [-100, 100]; 0 when there are no responses.
	Score int `json:"score"`
	// AverageScore is the mean raw 0-10 rating; 0 when there are no responses.
	AverageScore float64 `json:"average_score"`
}

// NPSRepository persists NPS responses and computes the admin aggregation.
// Raw-SQL repo (cross-account aggregation), deliberately separate from the
// sqlc-generated repositories.
type NPSRepository interface {
	Create(ctx context.Context, r *NPSResponse) error
	// Summarize aggregates responses created at or after `since`.
	Summarize(ctx context.Context, since time.Time) (NPSSummary, error)
	// RecentComments returns up to `limit` non-empty comments created at or
	// after `since`, newest first.
	RecentComments(ctx context.Context, since time.Time, limit int) ([]NPSComment, error)
	// IsPromptEligible reports whether the in-app NPS prompt should be shown
	// to the account: true iff the account was created at or before
	// `createdBefore` (so brand-new accounts are never prompted on first
	// login) AND it has not already submitted or dismissed the prompt in
	// `quarter` (e.g. "2026-Q2"). The cadence state lives in
	// accounts.nps_prompt_last_quarter — durable and per-account.
	IsPromptEligible(ctx context.Context, accountID, quarter string, createdBefore time.Time) (bool, error)
	// StampPromptHandled records that the account handled the NPS prompt
	// (submitted or dismissed) in `quarter`, suppressing it until the next
	// quarter.
	StampPromptHandled(ctx context.Context, accountID, quarter string) error
}

// ErrInvalidNPSScore is returned when a submitted score is outside [0, 10].
var ErrInvalidNPSScore = &AppError{Code: "INVALID_NPS_SCORE", Message: "score must be between 0 and 10"}

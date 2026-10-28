package domain

import (
	"context"
	"time"
)

// ROILead is a qualified lead captured by the public ROI-calculator lead magnet
// on the marketing site (US-163). The visitor enters their support volume and
// costs to see an estimated savings figure, and must hand over an email to
// reveal the result — so every row is a self-qualified lead with the inputs
// that produced its savings estimate attached for sales follow-up.
type ROILead struct {
	ID               string    `json:"id"`
	Email            string    `json:"email"`
	MessagesPerMonth int       `json:"messages_per_month"`
	CurrentCost      float64   `json:"current_cost"`
	HourlyRate       float64   `json:"hourly_rate"`
	EstimatedSavings float64   `json:"estimated_savings"`
	CreatedAt        time.Time `json:"created_at"`
}

type ROILeadRepository interface {
	Create(ctx context.Context, lead *ROILead) error
	// ExistsRecent reports whether the email already submitted at or after
	// `since`. Used to enforce the per-email submit rate limit.
	ExistsRecent(ctx context.Context, email string, since time.Time) (bool, error)
}

// ErrMissingROIFields is returned when an ROI-calculator submission is missing
// one of the numeric inputs needed to compute a savings estimate. Mapped to
// HTTP 400.
var ErrMissingROIFields = &AppError{Code: "MISSING_FIELDS", Message: "messages per month, current cost and hourly rate are required"}

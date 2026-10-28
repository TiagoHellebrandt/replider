package domain

import (
	"context"
	"time"
)

// ProspectingSearch records a single outbound-prospecting run against a funnel
// (US-022): the criteria used (free-text query and/or a lat/long radius), how
// many candidates the directory returned, and how many became NEW leads. It is
// an append-only audit log — re-running the same search creates a new row but
// the idempotent lead upsert keeps LeadsCreated honest (0 on a repeat run).
type ProspectingSearch struct {
	ID           string    `json:"id"`
	AccountID    string    `json:"account_id"`
	FunnelID     string    `json:"funnel_id"`
	Query        string    `json:"query"`
	Latitude     *float64  `json:"latitude,omitempty"`
	Longitude    *float64  `json:"longitude,omitempty"`
	RadiusMeters *float64  `json:"radius_meters,omitempty"`
	ResultsCount int       `json:"results_count"`
	LeadsCreated int       `json:"leads_created"`
	CreatedAt    time.Time `json:"created_at"`
}

// ProspectingSearchRepository persists prospecting-search audit rows.
type ProspectingSearchRepository interface {
	Create(ctx context.Context, s *ProspectingSearch) error
}

var (
	// ErrProspectingNotConfigured is returned when the prospecting provider has
	// no API key configured (or is not wired at all). Mapped to 503 so the UI
	// can tell the admin the integration is unavailable rather than that their
	// request was malformed.
	ErrProspectingNotConfigured = &AppError{Code: "PROSPECTING_NOT_CONFIGURED", Message: "prospecting provider is not configured"}
	// ErrProspectingInputRequired is returned when a prospecting request has
	// neither a free-text query nor a positive search radius — there is nothing
	// to search on.
	ErrProspectingInputRequired = &AppError{Code: "PROSPECTING_INPUT_REQUIRED", Message: "a search query or a location radius is required"}
	// ErrProspectingRateLimited surfaces the directory provider's rate-limit
	// response so the admin can retry later instead of seeing a generic failure.
	ErrProspectingRateLimited = &AppError{Code: "PROSPECTING_RATE_LIMITED", Message: "prospecting provider rate limit exceeded — try again shortly"}
)

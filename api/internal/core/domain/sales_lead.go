package domain

import (
	"context"
	"time"
)

// SalesLead is an Enterprise-tier qualification request submitted from the
// public marketing site (US-157). It replaces the old mailto-only CTA: the
// structured fields let sales triage volume/fit before the first reply.
type SalesLead struct {
	ID            string    `json:"id"`
	Company       string    `json:"company"`
	Email         string    `json:"email"`
	CompanySize   string    `json:"company_size"`
	Channels      string    `json:"channels"`
	MonthlyVolume string    `json:"monthly_volume"`
	Message       string    `json:"message"`
	CreatedAt     time.Time `json:"created_at"`
}

type SalesLeadRepository interface {
	Create(ctx context.Context, lead *SalesLead) error
	// ExistsRecent reports whether the email already submitted at or after
	// `since`. Used to enforce the per-email submit rate limit.
	ExistsRecent(ctx context.Context, email string, since time.Time) (bool, error)
}

// ErrMissingSalesFields is returned when a sales-qualification submission is
// missing one of its required business fields (company / size / channels /
// monthly volume). Mapped to HTTP 400.
var ErrMissingSalesFields = &AppError{Code: "MISSING_FIELDS", Message: "company, size, channels and monthly volume are required"}

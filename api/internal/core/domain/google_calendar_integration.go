package domain

import (
	"context"
	"time"
)

// GoogleCalendarIntegration is an account's OAuth link to Google
// Calendar. RefreshToken is plaintext in memory only — the repository
// seals it at rest and never returns it through read paths that feed
// HTTP responses (the handler-facing Status projection omits it).
type GoogleCalendarIntegration struct {
	AccountID    string    `json:"account_id"`
	RefreshToken string    `json:"-"`
	GoogleEmail  string    `json:"google_email"`
	ConnectedAt  time.Time `json:"connected_at"`
}

// GoogleUserCalendar is one entry of the connected account's calendar
// list (the dashboard's picker).
type GoogleUserCalendar struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Primary bool   `json:"primary"`
}

// GoogleCalendarIntegrationRepository persists per-account Google
// Calendar OAuth links.
type GoogleCalendarIntegrationRepository interface {
	// Get returns (nil, nil) when the account has no integration or the
	// stored token can no longer be decrypted (key rotation) — callers
	// treat both as "not connected".
	Get(ctx context.Context, accountID string) (*GoogleCalendarIntegration, error)
	Save(ctx context.Context, integ *GoogleCalendarIntegration) error
	Delete(ctx context.Context, accountID string) error
}

var (
	// ErrGCalNotConfigured — the platform has no Google OAuth client
	// credentials; the connect feature is disabled in this installation.
	ErrGCalNotConfigured = &AppError{Code: "GCAL_NOT_CONFIGURED", Message: "google calendar oauth is not configured"}
	// ErrGCalNotConnected — the account has not connected Google Calendar.
	ErrGCalNotConnected = &AppError{Code: "GCAL_NOT_CONNECTED", Message: "google calendar is not connected for this account"}
	// ErrGCalInvalidState — the OAuth callback state failed validation
	// (tampered, expired, or from another installation).
	ErrGCalInvalidState = &AppError{Code: "GCAL_INVALID_STATE", Message: "invalid or expired oauth state"}
	// ErrGCalCipherUnconfigured — BYOK_ENCRYPTION_KEY is missing; refusing
	// to persist a plaintext refresh token.
	ErrGCalCipherUnconfigured = &AppError{Code: "GCAL_CIPHER_UNCONFIGURED", Message: "encryption key not configured; cannot store google credentials"}
)

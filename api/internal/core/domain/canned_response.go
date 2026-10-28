package domain

import "time"

// Canned response visibility scopes (US-018). Account rows are team-wide
// assets managed by admins; personal rows belong to a single user and are
// invisible to everyone else.
const (
	CannedVisibilityAccount  = "account"
	CannedVisibilityPersonal = "personal"
)

func IsValidCannedVisibility(v string) bool {
	return v == CannedVisibilityAccount || v == CannedVisibilityPersonal
}

type CannedResponse struct {
	ID        string `json:"id"`
	AccountID string `json:"account_id"`
	Shortcut  string `json:"shortcut"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Category  string `json:"category"`
	// Visibility is 'account' (team-wide, admin-managed) or 'personal'
	// (owner-only). Immutable after creation.
	Visibility string `json:"visibility"`
	// OwnerUserID is set only for personal responses.
	OwnerUserID *string   `json:"owner_user_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

var ErrCannedResponseNotFound = &AppError{Code: "CANNED_RESPONSE_NOT_FOUND", Message: "canned response not found"}
var ErrCannedResponseShortcutExists = &AppError{Code: "SHORTCUT_EXISTS", Message: "shortcut already exists"}

// ErrCannedResponseForbidden: attendants cannot manage account-wide
// responses (admin manages those); personal responses are owner-only.
var ErrCannedResponseForbidden = &AppError{Code: "CANNED_RESPONSE_FORBIDDEN", Message: "you cannot manage this canned response"}

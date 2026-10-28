package domain

import (
	"context"
	"time"
)

// AutoAssignSettings is the per-account round-robin configuration
// (US-016). Absence of a row means "disabled" for customer accounts and
// "enabled, all active attendants" only for the platform account
// (US-020) — the feature was built for the platform inbox and must not
// silently change existing customer accounts' behavior. An empty
// ParticipantUserIDs means "all active attendants"; a non-empty list
// restricts the rotation to those users.
type AutoAssignSettings struct {
	AccountID          string
	Enabled            bool
	ParticipantUserIDs []string
	UpdatedAt          time.Time
}

// AutoAssignSettingsRepository persists per-account auto-assign config.
type AutoAssignSettingsRepository interface {
	// GetAutoAssignSettings returns nil (no error) when the account has
	// no persisted row yet.
	GetAutoAssignSettings(ctx context.Context, accountID string) (*AutoAssignSettings, error)
	UpsertAutoAssignSettings(ctx context.Context, settings *AutoAssignSettings) error
}

// AutoAssignRotationRepository owns the persisted round-robin cursor
// (US-021 review fixes). The cursor lives in the database — never in
// process memory — so rotation survives deploys and stays consistent
// across replicas.
type AutoAssignRotationRepository interface {
	// AssignNextRoundRobin atomically claims the next attendant after
	// the account's persisted cursor (smallest pool id greater than it,
	// wrapping to the smallest) AND assigns the conversation to them,
	// in one transaction: on any failure — including a missing
	// conversation (ErrNotFound) — the cursor does not advance, so an
	// error never skips an attendant's turn nor orphans a conversation
	// against an advanced cursor.
	AssignNextRoundRobin(ctx context.Context, conversationID, accountID string, pool []string) (string, error)
}

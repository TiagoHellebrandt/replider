package domain

import (
	"context"
	"time"
)

// SessionInvalidationReader answers "since when are this user's sessions
// dead?". users.session_invalidated_at is stamped when a member is removed
// (deactivated); the auth middleware and the WebSocket first-frame auth
// compare the token's iat against it, so a JWT that outlives the membership
// stops working immediately instead of at its 24h expiry.
//
// Nil means the user's sessions were never invalidated.
type SessionInvalidationReader interface {
	SessionInvalidatedAt(ctx context.Context, userID string) (*time.Time, error)
}

// SessionInvalidatedBefore reports whether a token issued at issuedAt is
// dead given the user's invalidation stamp. Tokens issued in the same
// second as the stamp are refused too: JWT iat has second precision and
// the removal must win the tie.
func SessionInvalidatedBefore(issuedAt time.Time, invalidatedAt *time.Time) bool {
	if invalidatedAt == nil || issuedAt.IsZero() {
		return false
	}
	return !issuedAt.Truncate(time.Second).After(invalidatedAt.Truncate(time.Second))
}

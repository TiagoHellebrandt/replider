package domain

import (
	"context"
	"time"
)

type NewsletterSubscriber struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type NewsletterRepository interface {
	Create(ctx context.Context, subscriber *NewsletterSubscriber) error
	// ExistsRecent reports whether the email already subscribed at or after `since`.
	// Used to enforce the per-email subscribe rate limit.
	ExistsRecent(ctx context.Context, email string, since time.Time) (bool, error)
}

var ErrInvalidEmail = &AppError{Code: "INVALID_EMAIL", Message: "invalid email address"}

// ErrSpamDetected is returned when the newsletter honeypot field is filled,
// which only an automated bot would do. The HTTP layer hides this from the
// caller (returns a normal 200) so the trap cannot be detected.
var ErrSpamDetected = &AppError{Code: "SPAM_DETECTED", Message: "spam detected"}

// ErrTooManyRequests is returned when the same email tries to subscribe more
// than once within the rate-limit window (1 per 24h).
var ErrTooManyRequests = &AppError{Code: "RATE_LIMITED", Message: "too many requests"}

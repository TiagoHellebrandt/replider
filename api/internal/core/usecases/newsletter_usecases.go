package usecases

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// subscribeRateWindow is the minimum interval allowed between subscribe
// attempts for the same email address (1 subscription / email / 24h).
const subscribeRateWindow = 24 * time.Hour

type SubscribeNewsletterUseCase struct {
	repo domain.NewsletterRepository
	now  func() time.Time
}

func NewSubscribeNewsletterUseCase(repo domain.NewsletterRepository) *SubscribeNewsletterUseCase {
	return &SubscribeNewsletterUseCase{repo: repo, now: time.Now}
}

// Execute validates and persists a newsletter subscription.
//
// honeypot is the hidden "website" form field: legitimate users never see or
// fill it, so any non-empty value means a bot submitted the form. The request
// is rejected (ErrSpamDetected) before any DB work.
//
// The same email may only subscribe once per subscribeRateWindow (24h), guarded
// by ExistsRecent (ErrTooManyRequests). The DB UNIQUE(email) constraint is the
// durable backstop; this check makes the limit observable to the caller.
func (uc *SubscribeNewsletterUseCase) Execute(ctx context.Context, email, honeypot string) error {
	if strings.TrimSpace(honeypot) != "" {
		return domain.ErrSpamDetected
	}

	email = strings.TrimSpace(strings.ToLower(email))
	if !emailRegex.MatchString(email) {
		return domain.ErrInvalidEmail
	}

	recent, err := uc.repo.ExistsRecent(ctx, email, uc.now().Add(-subscribeRateWindow))
	if err != nil {
		return err
	}
	if recent {
		return domain.ErrTooManyRequests
	}

	subscriber := &domain.NewsletterSubscriber{
		ID:        ulid.Make().String(),
		Email:     email,
		CreatedAt: uc.now(),
	}

	return uc.repo.Create(ctx, subscriber)
}

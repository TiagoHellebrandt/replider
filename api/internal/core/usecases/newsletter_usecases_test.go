package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

type MockNewsletterRepo struct {
	createErr      error
	created        bool
	existsRecent   bool
	existsRecented bool // whether ExistsRecent was consulted
	existsErr      error
}

func (m *MockNewsletterRepo) Create(ctx context.Context, s *domain.NewsletterSubscriber) error {
	m.created = true
	return m.createErr
}

func (m *MockNewsletterRepo) ExistsRecent(ctx context.Context, email string, since time.Time) (bool, error) {
	m.existsRecented = true
	return m.existsRecent, m.existsErr
}

func TestSubscribeNewsletter_Success(t *testing.T) {
	repo := &MockNewsletterRepo{}
	uc := NewSubscribeNewsletterUseCase(repo)

	err := uc.Execute(context.Background(), "user@example.com", "")
	assert.NoError(t, err)
	assert.True(t, repo.created)
}

func TestSubscribeNewsletter_Normalizes_Email(t *testing.T) {
	repo := &MockNewsletterRepo{}
	uc := NewSubscribeNewsletterUseCase(repo)

	// should trim spaces and lowercase
	err := uc.Execute(context.Background(), "  USER@EXAMPLE.COM  ", "")
	assert.NoError(t, err)
}

func TestSubscribeNewsletter_AlreadyExists(t *testing.T) {
	// ON CONFLICT DO NOTHING means repo returns nil even for duplicates
	repo := &MockNewsletterRepo{createErr: nil}
	uc := NewSubscribeNewsletterUseCase(repo)

	err := uc.Execute(context.Background(), "existing@example.com", "")
	assert.NoError(t, err)
}

func TestSubscribeNewsletter_InvalidEmail_Empty(t *testing.T) {
	repo := &MockNewsletterRepo{}
	uc := NewSubscribeNewsletterUseCase(repo)

	err := uc.Execute(context.Background(), "", "")
	assert.Equal(t, domain.ErrInvalidEmail, err)
}

func TestSubscribeNewsletter_InvalidEmail_BadFormat(t *testing.T) {
	repo := &MockNewsletterRepo{}
	uc := NewSubscribeNewsletterUseCase(repo)

	cases := []string{
		"notanemail",
		"missing@",
		"@nodomain.com",
		"no spaces@example.com",
		"double@@example.com",
	}
	for _, email := range cases {
		err := uc.Execute(context.Background(), email, "")
		assert.Equal(t, domain.ErrInvalidEmail, err, "expected invalid for: %s", email)
	}
}

func TestSubscribeNewsletter_Honeypot_BlocksBot(t *testing.T) {
	repo := &MockNewsletterRepo{}
	uc := NewSubscribeNewsletterUseCase(repo)

	// A bot fills the hidden "website" honeypot field.
	err := uc.Execute(context.Background(), "bot@example.com", "http://spam.example")
	assert.Equal(t, domain.ErrSpamDetected, err)
	// The bot must never be persisted, and we short-circuit before any DB work.
	assert.False(t, repo.created, "honeypot submission must not be persisted")
	assert.False(t, repo.existsRecented, "honeypot submission must short-circuit before rate-limit check")
}

func TestSubscribeNewsletter_RateLimit_BlocksRepeatWithin24h(t *testing.T) {
	// ExistsRecent=true => the same email already subscribed inside the window.
	repo := &MockNewsletterRepo{existsRecent: true}
	uc := NewSubscribeNewsletterUseCase(repo)

	err := uc.Execute(context.Background(), "user@example.com", "")
	assert.Equal(t, domain.ErrTooManyRequests, err)
	assert.True(t, repo.existsRecented)
	assert.False(t, repo.created, "rate-limited email must not be re-inserted")
}

func TestSubscribeNewsletter_RateLimit_AllowsAfterWindow(t *testing.T) {
	// ExistsRecent=false => no subscription inside the window, so allow it.
	repo := &MockNewsletterRepo{existsRecent: false}
	uc := NewSubscribeNewsletterUseCase(repo)

	err := uc.Execute(context.Background(), "user@example.com", "")
	assert.NoError(t, err)
	assert.True(t, repo.created)
}

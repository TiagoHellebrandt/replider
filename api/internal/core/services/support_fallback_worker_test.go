package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSupportFallbackRepo struct {
	mu       sync.Mutex
	due      []*domain.SupportFallbackCandidate
	listErr  error
	marked   []string
	markErr  error
	lastArgs struct {
		cutoff time.Time
		limit  int
	}
}

func (f *fakeSupportFallbackRepo) ListDueEmailFallbacks(ctx context.Context, cutoff time.Time, limit int) ([]*domain.SupportFallbackCandidate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastArgs.cutoff = cutoff
	f.lastArgs.limit = limit
	return f.due, f.listErr
}

func (f *fakeSupportFallbackRepo) MarkFallbackEmailSent(ctx context.Context, conversationID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.markErr != nil {
		return f.markErr
	}
	f.marked = append(f.marked, conversationID)
	return nil
}

type sentSupportEmail struct {
	to, userName, ctaURL, locale string
}

type fakeSupportEmailSender struct {
	mu      sync.Mutex
	sent    []sentSupportEmail
	failFor map[string]error // keyed by `to`
}

func (f *fakeSupportEmailSender) SendSupportUnreadEmail(to, userName, ctaURL, locale string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.failFor[to]; err != nil {
		return err
	}
	f.sent = append(f.sent, sentSupportEmail{to, userName, ctaURL, locale})
	return nil
}

func candidate(convID, email, currency string) *domain.SupportFallbackCandidate {
	return &domain.SupportFallbackCandidate{
		ConversationID:    convID,
		UserID:            "01USER" + convID,
		UserEmail:         email,
		UserName:          "User " + convID,
		CustomerAccountID: "01ACC000000000000000000000",
		Currency:          currency,
		LastOutboundAt:    time.Now().Add(-5 * time.Hour),
	}
}

func TestSupportFallbackWorker_SendsAndMarks(t *testing.T) {
	repo := &fakeSupportFallbackRepo{due: []*domain.SupportFallbackCandidate{
		candidate("c1", "ana@cliente.com", "brl"),
		candidate("c2", "john@customer.com", "usd"),
	}}
	sender := &fakeSupportEmailSender{}
	w := NewSupportFallbackWorker(repo, sender, "https://app.replider.com", 4*time.Hour)

	sent := w.tick(context.Background())

	assert.Equal(t, 2, sent)
	require.Len(t, sender.sent, 2)
	// Locale follows the CUSTOMER account currency.
	assert.Equal(t, "pt-BR", sender.sent[0].locale)
	assert.Equal(t, "en", sender.sent[1].locale)
	// Deep link points at the support widget.
	assert.Equal(t, "https://app.replider.com/support", sender.sent[0].ctaURL)
	// Both threads stamped so the same unread window never re-sends.
	assert.Equal(t, []string{"c1", "c2"}, repo.marked)
	// The cutoff honors the configured window.
	assert.WithinDuration(t, time.Now().Add(-4*time.Hour), repo.lastArgs.cutoff, 5*time.Second)
}

func TestSupportFallbackWorker_SendFailureLeavesCandidateUnstamped(t *testing.T) {
	repo := &fakeSupportFallbackRepo{due: []*domain.SupportFallbackCandidate{
		candidate("c1", "broken@cliente.com", ""),
		candidate("c2", "ok@cliente.com", ""),
	}}
	sender := &fakeSupportEmailSender{failFor: map[string]error{
		"broken@cliente.com": errors.New("smtp down"),
	}}
	w := NewSupportFallbackWorker(repo, sender, "https://app.replider.com", 4*time.Hour)

	sent := w.tick(context.Background())

	// The failed send is NOT stamped — it stays due for the next tick;
	// the sibling still goes out (one bad row never aborts the batch).
	assert.Equal(t, 1, sent)
	assert.Equal(t, []string{"c2"}, repo.marked)
}

func TestSupportFallbackWorker_SkipsEmptyEmail(t *testing.T) {
	repo := &fakeSupportFallbackRepo{due: []*domain.SupportFallbackCandidate{
		candidate("c1", "", "brl"),
	}}
	sender := &fakeSupportEmailSender{}
	w := NewSupportFallbackWorker(repo, sender, "https://app.replider.com", 0 /* → default 4h */)

	sent := w.tick(context.Background())

	assert.Equal(t, 0, sent)
	assert.Empty(t, sender.sent)
	assert.Empty(t, repo.marked)
	assert.Equal(t, DefaultSupportUnreadAfter, w.unreadAfter, "non-positive window falls back to the PRD default")
}

func TestSupportFallbackWorker_ListErrorIsANoOp(t *testing.T) {
	repo := &fakeSupportFallbackRepo{listErr: errors.New("db down")}
	sender := &fakeSupportEmailSender{}
	w := NewSupportFallbackWorker(repo, sender, "https://app.replider.com", 4*time.Hour)

	assert.Equal(t, 0, w.tick(context.Background()))
	assert.Empty(t, sender.sent)
}

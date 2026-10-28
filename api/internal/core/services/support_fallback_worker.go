package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// SupportFallbackRepo is the minimal slice of the support-notify repo the
// worker drives (satisfied by *postgres.SupportNotifyRepository).
type SupportFallbackRepo interface {
	ListDueEmailFallbacks(ctx context.Context, cutoff time.Time, limit int) ([]*domain.SupportFallbackCandidate, error)
	MarkFallbackEmailSent(ctx context.Context, conversationID string) error
}

// SupportFallbackEmailSender mirrors the one SMTPAdapter composer the
// worker needs, so it stays free of adapter imports.
type SupportFallbackEmailSender interface {
	SendSupportUnreadEmail(to, userName, ctaURL, locale string) error
}

// SupportFallbackWorker (canal-interno US-004) sweeps internal CS threads
// whose owner hasn't read a platform-side reply within the configured
// window and sends ONE fallback email per unread window. Idempotency under
// restart is the DB stamp (conversations.support_fallback_email_at) plus
// the due-query's window logic — the worker itself is stateless, so any
// cadence only bounds worst-case email latency past the window.
type SupportFallbackWorker struct {
	repo         SupportFallbackRepo
	emailSender  SupportFallbackEmailSender
	appBaseURL   string
	unreadAfter  time.Duration
	pollInterval time.Duration
	batchLimit   int
	running      atomic.Bool
}

// DefaultSupportUnreadAfter is the PRD's 4h unread window.
const DefaultSupportUnreadAfter = 4 * time.Hour

func NewSupportFallbackWorker(
	repo SupportFallbackRepo,
	emailSender SupportFallbackEmailSender,
	appBaseURL string,
	unreadAfter time.Duration,
) *SupportFallbackWorker {
	if unreadAfter <= 0 {
		unreadAfter = DefaultSupportUnreadAfter
	}
	return &SupportFallbackWorker{
		repo:         repo,
		emailSender:  emailSender,
		appBaseURL:   appBaseURL,
		unreadAfter:  unreadAfter,
		pollInterval: 10 * time.Minute,
		batchLimit:   200,
	}
}

// WithPollInterval overrides the tick cadence (tests use ms-scale).
func (w *SupportFallbackWorker) WithPollInterval(d time.Duration) *SupportFallbackWorker {
	w.pollInterval = d
	return w
}

// Start runs the fallback loop until ctx is cancelled. Once-per-process.
func (w *SupportFallbackWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("support fallback worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("support fallback worker started",
		"poll_interval", w.pollInterval,
		"unread_after", w.unreadAfter)

	w.tick(ctx)
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("support fallback worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

// tick runs one sweep; exported through Execute-style shape for tests via
// direct call. Returns the number of emails sent.
func (w *SupportFallbackWorker) tick(ctx context.Context) int {
	cutoff := time.Now().Add(-w.unreadAfter)
	rows, err := w.repo.ListDueEmailFallbacks(ctx, cutoff, w.batchLimit)
	if err != nil {
		slog.Warn("support fallback worker: list failed", "error", err)
		return 0
	}
	sent := 0
	for _, c := range rows {
		if w.dispatch(ctx, c) {
			sent++
		}
	}
	if sent > 0 {
		slog.Info("support fallback worker: tick done", "candidates", len(rows), "sent", sent)
	}
	return sent
}

// dispatch sends the fallback email for one thread and stamps the DB
// guard. The stamp happens only AFTER a successful send, so a crashed or
// failed send stays a candidate for the next tick (at-least-once; the
// window logic caps it at one email per unread session once stamped).
func (w *SupportFallbackWorker) dispatch(ctx context.Context, c *domain.SupportFallbackCandidate) bool {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("support fallback dispatch panic",
				"conversation_id", c.ConversationID, "recover", r)
		}
	}()
	if c.UserEmail == "" {
		return false
	}
	locale := domain.EmailLocaleForCurrency(c.Currency)
	ctaURL := w.appBaseURL + "/support"
	if err := w.emailSender.SendSupportUnreadEmail(c.UserEmail, c.UserName, ctaURL, locale); err != nil {
		slog.Warn("support fallback email failed",
			"conversation_id", c.ConversationID, "error", err)
		return false
	}
	if err := w.repo.MarkFallbackEmailSent(ctx, c.ConversationID); err != nil {
		// The email went out but the stamp failed — the next tick may
		// re-send once. Prefer that over stamping first and silently
		// losing the notification on a send failure.
		slog.Warn("support fallback: mark sent failed",
			"conversation_id", c.ConversationID, "error", err)
	}
	return true
}

package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// ReminderPusher mirrors a sliver of usecases.PushNotificationUseCase
// so the worker doesn't import the usecases package (services is
// already imported BY usecases via AgentRunner, so the reverse import
// would create a cycle).
//
// The second argument is `excludeUserID` (matches the underlying
// implementation): pass empty string to notify every team device.
// Reminder dispatch always notifies the whole account so the team can
// self-organize — pinpointing only the assigned attendant is a v2
// concern and would need a new NotifyUser method on the use case.
type ReminderPusher interface {
	NotifyAccount(ctx context.Context, accountID, excludeUserID string, payload domain.PushPayload)
}

// ReplyReminderRepo is the minimal slice of ConversationRepository the
// worker needs. Kept narrow so tests can satisfy with two methods.
type ReplyReminderRepo interface {
	ListPendingReminders(ctx context.Context, limit int) ([]*domain.ReminderCandidate, error)
	MarkReminderSent(ctx context.Context, conversationID string) error
}

// ReminderBroadcaster delivers the reply-reminder dashboard event.
// Implemented by handlers.Hub via domain.EventBroadcaster — declared
// locally to keep the worker free of adapter imports.
type ReminderBroadcaster interface {
	BroadcastToAccount(accountID string, payload []byte)
}

// ReminderEmailSender mirrors a sliver of email.EmailService so the
// worker can fire off the reminder email without importing the adapter.
// Optional — when nil, the worker skips email and just fires push + WS.
type ReminderEmailSender interface {
	SendReplyReminderEmail(to, attendantName, contactName, conversationURL string, minutesWaiting int) error
}

// ReminderAdminFinder yields the email/name of the attendant to notify.
// When AssignedTo is set we look up that user; when nil we fall back to
// the account admin so the reminder still reaches a human.
type ReminderAdminFinder interface {
	GetByID(ctx context.Context, userID string) (*domain.User, error)
	GetAdminByAccountID(ctx context.Context, accountID string) (*domain.User, error)
}

// ReplyReminderWorker periodically sweeps conversations whose customer
// is waiting past the per-account threshold and fires push + WebSocket
// + email reminders to the assigned attendant. Escalation ladder
// 30m / 1h / 2h (or 2x/4x the configured threshold) keeps the worker
// from spamming a single forgotten thread.
//
// All side-effects are best-effort: a push failure does not block the
// email or the WS broadcast, and a single bad row never aborts the
// whole tick. The DB MarkReminderSent runs after dispatch so a crash
// mid-tick is replayable on the next tick (the row stays a candidate).
type ReplyReminderWorker struct {
	convRepo     ReplyReminderRepo
	pushUC       ReminderPusher
	broadcaster  ReminderBroadcaster
	userFinder   ReminderAdminFinder
	emailSender  ReminderEmailSender
	appBaseURL   string
	pollInterval time.Duration
	batchLimit   int
	running      atomic.Bool
}

func NewReplyReminderWorker(
	convRepo ReplyReminderRepo,
	pushUC ReminderPusher,
	broadcaster ReminderBroadcaster,
	userFinder ReminderAdminFinder,
	appBaseURL string,
) *ReplyReminderWorker {
	return &ReplyReminderWorker{
		convRepo:     convRepo,
		pushUC:       pushUC,
		broadcaster:  broadcaster,
		userFinder:   userFinder,
		appBaseURL:   appBaseURL,
		pollInterval: 1 * time.Minute,
		batchLimit:   200,
	}
}

// WithEmailSender wires the email channel. Optional — when not set,
// reminders fire as push + WS only.
func (w *ReplyReminderWorker) WithEmailSender(s ReminderEmailSender) *ReplyReminderWorker {
	w.emailSender = s
	return w
}

// WithPollInterval overrides the tick cadence (tests use ms-scale).
func (w *ReplyReminderWorker) WithPollInterval(d time.Duration) *ReplyReminderWorker {
	w.pollInterval = d
	return w
}

// Start runs the reminder loop until ctx is cancelled. Once-per-process.
func (w *ReplyReminderWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("reply reminder worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("reply reminder worker started",
		"poll_interval", w.pollInterval,
		"batch_limit", w.batchLimit,
		"email_enabled", w.emailSender != nil)

	w.tick(ctx)
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("reply reminder worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *ReplyReminderWorker) tick(ctx context.Context) {
	rows, err := w.convRepo.ListPendingReminders(ctx, w.batchLimit)
	if err != nil {
		slog.Warn("reply reminder worker: list failed", "error", err)
		return
	}
	if len(rows) == 0 {
		return
	}
	for _, c := range rows {
		w.dispatch(ctx, c)
	}
	slog.Info("reply reminder worker: tick done", "candidates", len(rows))
}

// dispatch fires push + WS + email for one reminder candidate and bumps
// the counter. Side-effects are isolated with defer/recover so one bad
// row doesn't kill the rest of the batch.
func (w *ReplyReminderWorker) dispatch(ctx context.Context, c *domain.ReminderCandidate) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("reply reminder dispatch panic",
				"conversation_id", c.ConversationID, "recover", r)
		}
	}()

	minutesWaiting := int(time.Since(c.WaitingSince).Minutes())
	convURL := w.appBaseURL + "/conversations/" + c.ConversationID

	// 1) Dashboard WS event so an open tab can flash a badge.
	if w.broadcaster != nil {
		payload, mErr := json.Marshal(map[string]any{
			"event":           "reply_reminder",
			"conversation_id": c.ConversationID,
			"minutes_waiting": minutesWaiting,
			"reminder_number": c.SentCount + 1,
			"contact_name":    c.ContactName,
		})
		if mErr == nil {
			w.broadcaster.BroadcastToAccount(c.AccountID, payload)
		}
	}

	// 2) Push notification — distinct payload so the SW renders the
	// reminder copy and the OS coalesces with prior conv push under
	// the same tag.
	if w.pushUC != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("reply reminder push panic",
						"conversation_id", c.ConversationID, "recover", r)
				}
			}()
			pctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			body := "⏰ Cliente aguardando há " + minutesToHuman(minutesWaiting)
			payload := domain.PushPayload{
				Title:          c.ContactName,
				Body:           body,
				Tag:            "conv:" + c.ConversationID,
				ConversationID: c.ConversationID,
				Event:          "reply_reminder",
				ClickURL:       "/home?conversation=" + c.ConversationID,
			}
			// Account-wide push: every team device with a registered
			// subscription gets the reminder; the escalation ladder
			// limits noise to 3 reminders max per conversation.
			w.pushUC.NotifyAccount(pctx, c.AccountID, "", payload)
		}()
	}

	// 3) Email — find the attendant (or fall back to the admin) and
	// send. Skip silently if the user lookup fails or there's no email
	// sender wired.
	if w.emailSender != nil && w.userFinder != nil {
		var recipient *domain.User
		if c.AssignedTo != nil {
			if u, uErr := w.userFinder.GetByID(ctx, *c.AssignedTo); uErr == nil {
				recipient = u
			}
		}
		if recipient == nil {
			if admin, aErr := w.userFinder.GetAdminByAccountID(ctx, c.AccountID); aErr == nil {
				recipient = admin
			}
		}
		if recipient != nil && recipient.Email != "" {
			go func(to, name string) {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("reply reminder email panic", "recover", r)
					}
				}()
				if err := w.emailSender.SendReplyReminderEmail(to, name, c.ContactName, convURL, minutesWaiting); err != nil {
					slog.Warn("reply reminder email failed",
						"to", to, "conversation_id", c.ConversationID, "error", err)
				}
			}(recipient.Email, recipient.Name)
		}
	}

	// 4) Bump the counter so the next tick advances the escalation
	// ladder. Done last on purpose: a crash before this leaves the row
	// re-tryable rather than silently lost.
	if err := w.convRepo.MarkReminderSent(ctx, c.ConversationID); err != nil {
		slog.Warn("reply reminder: mark sent failed",
			"conversation_id", c.ConversationID, "error", err)
	}
}

func minutesToHuman(m int) string {
	if m < 60 {
		return itoa(m) + " min"
	}
	h := m / 60
	if h < 24 {
		return itoa(h) + "h"
	}
	d := h / 24
	return itoa(d) + "d"
}

// itoa avoids pulling strconv just for this tiny helper.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

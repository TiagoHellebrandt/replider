package usecases

import (
	"context"
	"log/slog"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// digestWindow is the trailing period the digest summarizes (the "last 7
// days" headline).
const digestWindow = 7 * 24 * time.Hour

// digestThrottle is the minimum gap between two digests for the same
// account. Slightly under a week so a Monday cron that drifts by a few
// hours week-to-week never skips a send.
const digestThrottle = 6 * 24 * time.Hour

// WeeklyDigestStore is the narrow port the digest use case needs over the
// accounts/activity tables (satisfied by postgres.WeeklyDigestRepository).
// Kept small so the test fake stays tiny and the use case never drags the
// full sqlc AccountRepository.
type WeeklyDigestStore interface {
	ListAccountsDueForDigest(ctx context.Context, before time.Time) ([]domain.WeeklyDigestAccount, error)
	ComputeWeeklyStats(ctx context.Context, accountID string, since time.Time) (domain.WeeklyDigestStats, error)
	MarkDigestSent(ctx context.Context, accountID string, at time.Time) error
}

// DigestAdminResolver resolves the admin recipient for an account.
// Satisfied by domain.UserRepository.
type DigestAdminResolver interface {
	GetAdminByAccountID(ctx context.Context, accountID string) (*domain.User, error)
}

// WeeklyDigestNotifier sends the rendered digest. Satisfied by
// *email.SMTPAdapter (the method is deliberately NOT on the broad
// EmailService interface — see SendWeeklyDigestEmail).
type WeeklyDigestNotifier interface {
	SendWeeklyDigestEmail(to, userName string, digest domain.WeeklyDigest, manageURL, unsubscribeURL, locale string) error
}

// SendWeeklyDigestUseCase emails each opted-in account's admin a weekly
// activity recap (US-176): messages handled, time saved, bot resolution
// rate, and the top customer questions. It reinforces value to reduce
// churn. Idempotent per week via the store's last-sent stamp, so the
// driving cron can tick freely.
type SendWeeklyDigestUseCase struct {
	store    WeeklyDigestStore
	admins   DigestAdminResolver
	notifier WeeklyDigestNotifier
	appURL   string
	window   time.Duration
	throttle time.Duration
	// now is injectable so tests pin "today" without sleeping.
	now func() time.Time
}

func NewSendWeeklyDigestUseCase(
	store WeeklyDigestStore,
	admins DigestAdminResolver,
	notifier WeeklyDigestNotifier,
	appURL string,
) *SendWeeklyDigestUseCase {
	return &SendWeeklyDigestUseCase{
		store:    store,
		admins:   admins,
		notifier: notifier,
		appURL:   appURL,
		window:   digestWindow,
		throttle: digestThrottle,
		now:      time.Now,
	}
}

// WithClock overrides the time source (tests only).
func (uc *SendWeeklyDigestUseCase) WithClock(now func() time.Time) *SendWeeklyDigestUseCase {
	uc.now = now
	return uc
}

// Execute sends the digest to every account due for one and returns the
// number sent. A single account's failure (missing admin, send error) is
// logged and skipped — it never aborts the batch, and an unstamped
// account is simply retried next tick.
func (uc *SendWeeklyDigestUseCase) Execute(ctx context.Context) (int, error) {
	now := uc.now()
	due, err := uc.store.ListAccountsDueForDigest(ctx, now.Add(-uc.throttle))
	if err != nil {
		return 0, err
	}

	sent := 0
	for _, acc := range due {
		if ctx.Err() != nil {
			return sent, ctx.Err()
		}
		if uc.sendOne(ctx, now, acc) {
			sent++
		}
	}
	if sent > 0 {
		slog.Info("weekly digest emails sent", "count", sent)
	}
	return sent, nil
}

// sendOne computes the stats, sends the email, and stamps the account on
// success. Returns true only when the email actually went out, so an
// account whose admin we can't resolve is retried (not silently marked).
func (uc *SendWeeklyDigestUseCase) sendOne(ctx context.Context, now time.Time, acc domain.WeeklyDigestAccount) bool {
	admin, err := uc.admins.GetAdminByAccountID(ctx, acc.AccountID)
	if err != nil || admin == nil || admin.Email == "" {
		slog.Warn("weekly digest: could not resolve admin", "account_id", acc.AccountID, "error", err)
		return false
	}

	since := now.Add(-uc.window)
	stats, err := uc.store.ComputeWeeklyStats(ctx, acc.AccountID, since)
	if err != nil {
		slog.Warn("weekly digest: stats failed", "account_id", acc.AccountID, "error", err)
		return false
	}

	digest := domain.WeeklyDigest{Stats: stats, PeriodStart: since, PeriodEnd: now}
	manageURL := uc.appURL + "/home"
	unsubscribeURL := uc.appURL + "/settings"
	if err := uc.notifier.SendWeeklyDigestEmail(admin.Email, admin.Name, digest, manageURL, unsubscribeURL, ""); err != nil {
		slog.Error("weekly digest: send failed", "account_id", acc.AccountID, "email", admin.Email, "error", err)
		return false
	}

	if err := uc.store.MarkDigestSent(ctx, acc.AccountID, now); err != nil {
		// Email already left; a failed stamp means it may resend next
		// tick. Log loudly but count it as sent.
		slog.Error("weekly digest: mark sent failed", "account_id", acc.AccountID, "error", err)
	}
	return true
}

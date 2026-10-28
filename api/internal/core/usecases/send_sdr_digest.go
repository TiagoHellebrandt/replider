package usecases

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Actionable daily digest (US-M010).
//
// One notification a day answering "what needs me today?", with a deep
// link per item. Two delivery paths, in this order:
//
//  1. Web push, because the whole point is one tap to the work.
//  2. Email, only when nobody on the account has a live push subscription
//     — a fallback, not a second copy. Sending both trains people to
//     ignore whichever arrives second.
//
// Idempotency is a DB stamp, not worker state: the process restarting is
// exactly the case worker memory cannot survive.

// SDRDigestPusher delivers the push. Satisfied by PushNotificationUseCase.
type SDRDigestPusher interface {
	NotifyAccount(ctx context.Context, accountID, excludeUserID string, payload domain.PushPayload)
	// HasSubscribers reports whether anyone on the account can receive a
	// push at all — that is what decides whether email is a fallback or a
	// duplicate.
	HasSubscribers(ctx context.Context, accountID string) bool
}

// SDRDigestEmailer sends the fallback email. Narrow port, satisfied by
// *email.SMTPAdapter.
type SDRDigestEmailer interface {
	SendSDRDigestEmail(to, userName string, lines []domain.DigestLine, appBaseURL, locale string) error
}

// SendSDRDigestUseCase builds and delivers the daily digest.
type SendSDRDigestUseCase struct {
	repo       domain.SDRDigestRepository
	users      DigestAdminResolver
	pusher     SDRDigestPusher
	emailer    SDRDigestEmailer
	appBaseURL string
	now        func() time.Time
	// sendHour is the LOCAL hour, in each account's own timezone, from
	// which the digest may go out. It lives here rather than on the worker
	// because the decision is per account and the worker has no account.
	sendHour int
}

// DefaultDigestSendHour is the start of the working day. A "what needs you
// today" that arrives at 5am is read tomorrow, by which point it is wrong.
const DefaultDigestSendHour = 8

func NewSendSDRDigestUseCase(
	repo domain.SDRDigestRepository,
	users DigestAdminResolver,
	appBaseURL string,
) *SendSDRDigestUseCase {
	return &SendSDRDigestUseCase{
		repo:       repo,
		users:      users,
		appBaseURL: strings.TrimRight(appBaseURL, "/"),
		now:        time.Now,
		sendHour:   DefaultDigestSendHour,
	}
}

// SetDelivery wires the two delivery paths. Either may be nil: with no
// pusher the digest is email-only, with no emailer it is push-only, and
// with neither the use case is a no-op that says so.
func (uc *SendSDRDigestUseCase) SetDelivery(pusher SDRDigestPusher, emailer SDRDigestEmailer) {
	uc.pusher = pusher
	uc.emailer = emailer
}

// WithClock pins the clock (tests).
func (uc *SendSDRDigestUseCase) WithClock(now func() time.Time) *SendSDRDigestUseCase {
	uc.now = now
	return uc
}

// WithSendHour overrides the local hour the digest may go out from.
func (uc *SendSDRDigestUseCase) WithSendHour(h int) *SendSDRDigestUseCase {
	uc.sendHour = h
	return uc
}

// Execute sends the digest to every due account. Returns how many were
// actually sent — accounts with nothing to act on are skipped, not sent an
// empty digest, because a daily notification that fires on quiet days is
// how a daily notification gets muted.
func (uc *SendSDRDigestUseCase) Execute(ctx context.Context) (int, error) {
	if uc.repo == nil {
		return 0, nil
	}
	now := uc.now()
	// Due = local clock past sendHour AND not sent since the account's own
	// local midnight. Both live in the query, in the account's timezone —
	// see ListDue. Comparing against a day boundary rather than "24h ago"
	// keeps the digest on a stable daily rhythm even when a tick runs late;
	// "24h ago" would drift the send time forward every day until it fell
	// off the end of the window and skipped a day.
	candidates, err := uc.repo.ListDue(ctx, uc.sendHour, 0)
	if err != nil {
		return 0, err
	}

	sent := 0
	for _, c := range candidates {
		accountID := c.AccountID
		// The digest covers the account's own day, so its boundary is
		// local midnight there, not the server's.
		dayStart := domain.StartOfDayIn(now, c.Timezone)
		digest, err := uc.repo.Build(ctx, accountID, dayStart.Add(-24*time.Hour), now.Add(-domain.StalledLeadAfter))
		if err != nil {
			slog.WarnContext(ctx, "sdr digest: failed to build", "account_id", accountID, "error", err)
			continue
		}
		if !digest.HasWork() {
			// Deliberately NOT stamped: nothing was sent, so the account
			// stays due and gets its digest the moment there is something
			// worth saying.
			continue
		}
		if uc.deliver(ctx, accountID, *digest) {
			if err := uc.repo.MarkSent(ctx, accountID, now); err != nil {
				slog.WarnContext(ctx, "sdr digest: failed to stamp send", "account_id", accountID, "error", err)
			}
			sent++
		}
	}
	return sent, nil
}

// deliver pushes, falling back to email only when nobody can receive a
// push. Reports whether anything actually went out — a digest that failed
// to deliver must not be stamped as sent, or the account silently loses
// that day.
func (uc *SendSDRDigestUseCase) deliver(ctx context.Context, accountID string, digest domain.SDRDigest) bool {
	lines := digest.Lines()
	if len(lines) == 0 {
		return false
	}

	if uc.pusher != nil && uc.pusher.HasSubscribers(ctx, accountID) {
		uc.pusher.NotifyAccount(ctx, accountID, "", domain.PushPayload{
			Title: "Seu dia na Replider",
			Body:  digest.Headline(),
			// Tag collapses a re-sent digest onto the previous one instead
			// of stacking notifications the user never asked twice for.
			Tag:       "sdr_digest",
			Event:     "sdr_digest",
			AccountID: accountID,
			// The most urgent line's destination: one tap to the work.
			ClickURL: uc.appURL(lines[0].ClickURL),
		})
		return true
	}

	if uc.emailer == nil || uc.users == nil {
		return false
	}
	admin, err := uc.users.GetAdminByAccountID(ctx, accountID)
	if err != nil || admin == nil || admin.Email == "" {
		slog.WarnContext(ctx, "sdr digest: no admin to email", "account_id", accountID, "error", err)
		return false
	}
	if err := uc.emailer.SendSDRDigestEmail(admin.Email, admin.Name, lines, uc.appBaseURL, "pt-BR"); err != nil {
		slog.WarnContext(ctx, "sdr digest: email failed", "account_id", accountID, "error", err)
		return false
	}
	return true
}

func (uc *SendSDRDigestUseCase) appURL(path string) string {
	if uc.appBaseURL == "" {
		return path
	}
	return uc.appBaseURL + path
}

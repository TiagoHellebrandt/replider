package usecases

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// ReactivationStore is the narrow port the reactivation use case needs
// over the accounts table (satisfied by postgres.ReactivationRepository).
// Kept small so the test fake stays tiny and the use case never drags the
// full sqlc AccountRepository.
type ReactivationStore interface {
	ListLapsedTrials(ctx context.Context, now time.Time) ([]domain.ReactivationAccount, error)
	MarkReactivationSent(ctx context.Context, accountID string, stage int) error
}

// ReactivationNotifier sends the rendered reactivation email. Satisfied by
// *email.SMTPAdapter (the method is deliberately NOT on the broad
// EmailService interface — mirrors SendWeeklyDigestEmail).
type ReactivationNotifier interface {
	SendReactivationEmail(to, userName string, daysSinceTrial int, reactivateURL, locale string) error
}

// SendReactivationEmailsUseCase emails never-converted accounts whose trial
// has lapsed, at 7/14/30 days after expiry, nudging them to reactivate
// (US-181). It catches warm leads who churned silently. Idempotent per
// milestone via the store's reactivation_stage_sent stamp, so the driving
// cron can tick freely (daily) without re-sending a stage.
type SendReactivationEmailsUseCase struct {
	store    ReactivationStore
	admins   DigestAdminResolver // reused: resolves the account admin
	notifier ReactivationNotifier
	appURL   string
	// now is injectable so tests pin "today" relative to seeded
	// trial-expiry timestamps without sleeping.
	now func() time.Time
}

func NewSendReactivationEmailsUseCase(
	store ReactivationStore,
	admins DigestAdminResolver,
	notifier ReactivationNotifier,
	appURL string,
) *SendReactivationEmailsUseCase {
	return &SendReactivationEmailsUseCase{
		store:    store,
		admins:   admins,
		notifier: notifier,
		appURL:   appURL,
		now:      time.Now,
	}
}

// WithClock overrides the time source (tests only).
func (uc *SendReactivationEmailsUseCase) WithClock(now func() time.Time) *SendReactivationEmailsUseCase {
	uc.now = now
	return uc
}

// Execute emails every lapsed-trial account whose next reactivation
// milestone is now due, and returns the number sent. A single account's
// failure (missing admin, send error) is logged and skipped — it never
// aborts the batch, and an unstamped account is simply retried next tick.
func (uc *SendReactivationEmailsUseCase) Execute(ctx context.Context) (int, error) {
	now := uc.now()
	due, err := uc.store.ListLapsedTrials(ctx, now)
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
		slog.Info("reactivation emails sent", "count", sent)
	}
	return sent, nil
}

// sendOne decides which milestone is due for the account, sends the email,
// and stamps the stage on success. Returns true only when the email
// actually went out, so an account whose admin we can't resolve is retried
// (not silently marked done).
func (uc *SendReactivationEmailsUseCase) sendOne(ctx context.Context, now time.Time, acc domain.ReactivationAccount) bool {
	daysSince := int(now.Sub(acc.TrialExpiresAt).Hours() / 24)
	stage := domain.NextReactivationStage(daysSince, acc.StageSent)
	if stage == 0 {
		// Listed (a nudge is still pending) but not yet elapsed to the
		// next milestone — nothing to do this tick.
		return false
	}

	admin, err := uc.admins.GetAdminByAccountID(ctx, acc.AccountID)
	if err != nil || admin == nil || admin.Email == "" {
		slog.Warn("reactivation: could not resolve admin", "account_id", acc.AccountID, "error", err)
		return false
	}

	if err := uc.notifier.SendReactivationEmail(admin.Email, admin.Name, daysSince, uc.reactivateURL(stage), ""); err != nil {
		slog.Error("reactivation: send failed", "account_id", acc.AccountID, "email", admin.Email, "error", err)
		return false
	}

	if err := uc.store.MarkReactivationSent(ctx, acc.AccountID, stage); err != nil {
		// Email already left; a failed stamp means it may resend next
		// tick. Log loudly but count it as sent.
		slog.Error("reactivation: mark sent failed", "account_id", acc.AccountID, "error", err)
	}
	return true
}

// reactivateURL builds the UTM-tagged login link so the click is tracked
// through the existing UTM-capture pipeline (utm_bootstrap on the app),
// attributing reactivations per milestone via utm_content.
func (uc *SendReactivationEmailsUseCase) reactivateURL(stage int) string {
	return fmt.Sprintf("%s/login?utm_source=lifecycle&utm_medium=email&utm_campaign=reactivation&utm_content=d%d",
		uc.appURL, stage)
}

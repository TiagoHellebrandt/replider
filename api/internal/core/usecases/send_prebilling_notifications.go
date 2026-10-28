package usecases

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/replider/api/internal/adapters/email"
	"github.com/replider/api/internal/core/domain"
)

// prebillingWindow is how far ahead of the charge we email the admin.
// 72h is the US-124 spec — enough lead time to update a card or cancel
// before the renewal lands, without nagging too early.
const prebillingWindow = 72 * time.Hour

// SendPrebillingNotificationsUseCase emails the account admin shortly
// before their subscription renews (US-124), reducing involuntary churn
// and surprise-charge support tickets. It is idempotent per billing
// cycle: the repository's prebilling_notified_period_end guard means an
// account already emailed for the current period_end is excluded from
// the due list, so the driving cron can tick as often as it likes.
type SendPrebillingNotificationsUseCase struct {
	accountRepo domain.AccountRepository
	userRepo    domain.UserRepository
	planRepo    domain.PlanRepository
	emailSvc    email.EmailService
	appBaseURL  string
	window      time.Duration
	// now is injectable so tests can pin "today" relative to seeded
	// period_end timestamps without sleeping.
	now func() time.Time
}

func NewSendPrebillingNotificationsUseCase(
	accountRepo domain.AccountRepository,
	userRepo domain.UserRepository,
	planRepo domain.PlanRepository,
	emailSvc email.EmailService,
	appBaseURL string,
) *SendPrebillingNotificationsUseCase {
	return &SendPrebillingNotificationsUseCase{
		accountRepo: accountRepo,
		userRepo:    userRepo,
		planRepo:    planRepo,
		emailSvc:    emailSvc,
		appBaseURL:  appBaseURL,
		window:      prebillingWindow,
		now:         time.Now,
	}
}

// WithClock overrides the time source (tests only).
func (uc *SendPrebillingNotificationsUseCase) WithClock(now func() time.Time) *SendPrebillingNotificationsUseCase {
	uc.now = now
	return uc
}

// WithWindow overrides the look-ahead window (tests only).
func (uc *SendPrebillingNotificationsUseCase) WithWindow(d time.Duration) *SendPrebillingNotificationsUseCase {
	uc.window = d
	return uc
}

// Execute finds every account renewing within the window that hasn't yet
// been notified for the current cycle, emails its admin, and stamps the
// notification so it isn't resent. Returns the number of reminders sent.
// A single account's failure (missing admin, send error) is logged and
// skipped — it never aborts the batch, and an unstamped account is simply
// retried on the next tick.
func (uc *SendPrebillingNotificationsUseCase) Execute(ctx context.Context) (int, error) {
	now := uc.now()
	cutoff := now.Add(uc.window)

	due, err := uc.accountRepo.ListAccountsDueForPrebilling(ctx, cutoff)
	if err != nil {
		return 0, err
	}

	sent := 0
	for _, acc := range due {
		if uc.notifyOne(ctx, now, acc) {
			sent++
		}
	}
	if sent > 0 {
		slog.Info("pre-billing reminders sent", "count", sent)
	}
	return sent, nil
}

// notifyOne sends a single reminder and stamps it on success. Returns
// true only when the email actually went out — so a renewal whose admin
// we can't resolve is retried (not silently marked done) next tick.
func (uc *SendPrebillingNotificationsUseCase) notifyOne(ctx context.Context, now time.Time, acc *domain.PrebillingAccount) bool {
	admin, err := uc.userRepo.GetAdminByAccountID(ctx, acc.AccountID)
	if err != nil || admin == nil {
		slog.Warn("pre-billing: could not resolve admin", "account_id", acc.AccountID, "error", err)
		return false
	}

	planName := "seu plano"
	if uc.planRepo != nil && acc.PlanID != nil {
		if plan, _ := uc.planRepo.GetByID(ctx, *acc.PlanID); plan != nil {
			planName = plan.Name
		}
	}

	// Round UP so a charge 49h out reads "2 days", never an alarming
	// "1 day". Clamp at 0 against any clock skew past period_end.
	days := int(math.Ceil(acc.PeriodEnd.Sub(now).Hours() / 24))
	if days < 0 {
		days = 0
	}

	manageURL := uc.appBaseURL + "/settings?billing=manage"
	if err := uc.emailSvc.SendPrebillingEmail(admin.Email, admin.Name, planName, days, manageURL, ""); err != nil {
		slog.Error("pre-billing: send failed", "account_id", acc.AccountID, "email", admin.Email, "error", err)
		return false
	}

	if err := uc.accountRepo.MarkPrebillingNotified(ctx, acc.AccountID, acc.PeriodEnd); err != nil {
		// Email already left; a failed stamp means it may resend next
		// cycle tick. Log loudly but count it as sent.
		slog.Error("pre-billing: mark notified failed", "account_id", acc.AccountID, "error", err)
	}
	return true
}

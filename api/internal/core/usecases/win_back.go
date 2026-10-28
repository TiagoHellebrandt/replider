package usecases

import (
	"context"
	"log/slog"
	"net/url"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// WinBackStore is the narrow port the win-back use case needs over the
// accounts table (satisfied by postgres.WinBackRepository). Kept small so
// the test fake stays tiny and the use case never drags the full sqlc
// AccountRepository.
type WinBackStore interface {
	StartWinBack(ctx context.Context, accountID, couponCode string, now time.Time) error
	ListWinBackDue(ctx context.Context) ([]domain.WinBackAccount, error)
	MarkWinBackSent(ctx context.Context, accountID string, stage int) error
}

// WinBackCouponIssuer mints the automatic recovery coupon for a churned
// customer and returns the redeemable promo code. Satisfied by
// *payment.StripeAdapter — kept narrow (not on the broad PaymentAdapter
// interface) so adopting it doesn't churn the PaymentAdapter mock.
type WinBackCouponIssuer interface {
	CreateWinBackCoupon(customerID string) (code string, err error)
}

// WinBackNotifier sends the rendered win-back email. Satisfied by
// *email.SMTPAdapter (the method is deliberately NOT on the broad
// EmailService interface — mirrors SendReactivationEmail).
type WinBackNotifier interface {
	SendWinBackEmail(to, userName string, daysSince, discountPercent, durationMonths int, couponCode, reactivateURL, locale string) error
}

// WinBackUseCase recovers churned customers (US-179). When a paid
// subscription is canceled, Start mints a 50%-off-3-months Stripe coupon,
// persists it + the recovery-window anchor, and sends the immediate offer
// email. A daily worker then drives Execute, which re-sends the still-valid
// coupon as follow-up reminders at 7 and 30 days. Idempotent per milestone
// via the store's winback_stage_sent stamp, so the cron can tick freely.
type WinBackUseCase struct {
	store    WinBackStore
	issuer   WinBackCouponIssuer
	admins   DigestAdminResolver // reused: resolves the account admin
	notifier WinBackNotifier
	appURL   string
	// now is injectable so tests pin "today" relative to a seeded
	// cancellation timestamp without sleeping.
	now func() time.Time
}

func NewWinBackUseCase(
	store WinBackStore,
	issuer WinBackCouponIssuer,
	admins DigestAdminResolver,
	notifier WinBackNotifier,
	appURL string,
) *WinBackUseCase {
	return &WinBackUseCase{
		store:    store,
		issuer:   issuer,
		admins:   admins,
		notifier: notifier,
		appURL:   appURL,
		now:      time.Now,
	}
}

// WithClock overrides the time source (tests only).
func (uc *WinBackUseCase) WithClock(now func() time.Time) *WinBackUseCase {
	uc.now = now
	return uc
}

// Start kicks off the win-back flow for a just-churned account: mints the
// automatic 50%-off-3-months coupon, persists it + the recovery-window
// anchor, and sends the immediate offer email (the "day 0" nudge). It is
// best-effort and self-contained — every failure is logged, never
// propagated — so it can never fail the Stripe webhook that triggers it.
func (uc *WinBackUseCase) Start(ctx context.Context, accountID, customerID string) {
	if accountID == "" || customerID == "" {
		return
	}

	code, err := uc.issuer.CreateWinBackCoupon(customerID)
	if err != nil || code == "" {
		slog.Error("winback: failed to create coupon", "account_id", accountID, "error", err)
		return
	}

	if err := uc.store.StartWinBack(ctx, accountID, code, uc.now()); err != nil {
		slog.Error("winback: failed to persist start", "account_id", accountID, "error", err)
		return
	}

	admin, err := uc.admins.GetAdminByAccountID(ctx, accountID)
	if err != nil || admin == nil || admin.Email == "" {
		slog.Warn("winback: could not resolve admin for offer", "account_id", accountID, "error", err)
		return
	}

	if err := uc.notifier.SendWinBackEmail(admin.Email, admin.Name, 0,
		domain.WinBackDiscountPercent, domain.WinBackDurationMonths, code, uc.reactivateURL(code), ""); err != nil {
		slog.Error("winback: failed to send offer email", "account_id", accountID, "email", admin.Email, "error", err)
		return
	}
	slog.Info("winback offer sent", "account_id", accountID)
}

// Execute re-emails every churned account whose next win-back follow-up
// milestone is now due, and returns the number sent. A single account's
// failure (missing admin, send error) is logged and skipped — it never
// aborts the batch, and an unstamped account is simply retried next tick.
func (uc *WinBackUseCase) Execute(ctx context.Context) (int, error) {
	due, err := uc.store.ListWinBackDue(ctx)
	if err != nil {
		return 0, err
	}

	now := uc.now()
	sent := 0
	for _, acc := range due {
		if ctx.Err() != nil {
			return sent, ctx.Err()
		}
		if uc.sendFollowUp(ctx, now, acc) {
			sent++
		}
	}
	if sent > 0 {
		slog.Info("winback follow-up emails sent", "count", sent)
	}
	return sent, nil
}

// sendFollowUp decides which milestone is due for the account, re-sends the
// coupon reminder, and stamps the stage on success. Returns true only when
// the email actually went out, so an account whose admin we can't resolve
// is retried (not silently marked done).
func (uc *WinBackUseCase) sendFollowUp(ctx context.Context, now time.Time, acc domain.WinBackAccount) bool {
	daysSince := int(now.Sub(acc.StartedAt).Hours() / 24)
	stage := domain.NextWinBackStage(daysSince, acc.StageSent)
	if stage == 0 {
		// In the window but not yet elapsed to the next milestone.
		return false
	}

	admin, err := uc.admins.GetAdminByAccountID(ctx, acc.AccountID)
	if err != nil || admin == nil || admin.Email == "" {
		slog.Warn("winback: could not resolve admin", "account_id", acc.AccountID, "error", err)
		return false
	}

	if err := uc.notifier.SendWinBackEmail(admin.Email, admin.Name, daysSince,
		domain.WinBackDiscountPercent, domain.WinBackDurationMonths, acc.CouponCode, uc.reactivateURL(acc.CouponCode), ""); err != nil {
		slog.Error("winback: follow-up send failed", "account_id", acc.AccountID, "email", admin.Email, "error", err)
		return false
	}

	if err := uc.store.MarkWinBackSent(ctx, acc.AccountID, stage); err != nil {
		// Email already left; a failed stamp means it may resend next
		// tick. Log loudly but count it as sent.
		slog.Error("winback: mark sent failed", "account_id", acc.AccountID, "error", err)
	}
	return true
}

// reactivateURL builds the UTM-tagged billing link that pre-fills the
// recovery coupon, so the existing app UTM-capture pipeline records the
// click and checkout applies the discount automatically.
func (uc *WinBackUseCase) reactivateURL(code string) string {
	return uc.appURL + "/billing?utm_source=email&utm_medium=email&utm_campaign=winback&coupon=" + url.QueryEscape(code)
}

package usecases

import (
	"context"
	"log/slog"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// PIX renewal cycle (US-M014).
//
// A card subscription renews itself; PIX does not. So the platform mints
// the renewal charge, chases it, and eventually stops serving. Each rung
// is delivered at most once per period (DB stamp), and a run that is late
// delivers only the rung that is true today — see NextPixDunningStage.

// PixRenewalNotifier delivers the rungs. Narrow port satisfied by
// *email.SMTPAdapter; a nil notifier still lets the ladder mint charges
// and suspend, it just cannot tell anyone.
type PixRenewalNotifier interface {
	// SendPrebillingEmail is reused for "your PIX charge is ready" and the
	// two reminders — same shape, different day count.
	SendPrebillingEmail(to, userName, planName string, daysUntil int, manageURL, locale string) error
}

// SendPixRenewalCycleUseCase walks every due account one rung.
type SendPixRenewalCycleUseCase struct {
	repo        domain.PixRenewalRepository
	accountRepo domain.AccountRepository
	planRepo    domain.PlanRepository
	users       DigestAdminResolver
	abacate     domain.AbacatePayAdapter
	notifier    PixRenewalNotifier
	appBaseURL  string
	now         func() time.Time
}

func NewSendPixRenewalCycleUseCase(
	repo domain.PixRenewalRepository,
	accountRepo domain.AccountRepository,
	planRepo domain.PlanRepository,
	users DigestAdminResolver,
	appBaseURL string,
) *SendPixRenewalCycleUseCase {
	return &SendPixRenewalCycleUseCase{
		repo:        repo,
		accountRepo: accountRepo,
		planRepo:    planRepo,
		users:       users,
		appBaseURL:  appBaseURL,
		now:         time.Now,
	}
}

// SetDelivery wires charge minting and notification. Both optional: with
// no AbacatePay adapter the ladder still reminds and suspends, which is
// the part that protects revenue.
func (uc *SendPixRenewalCycleUseCase) SetDelivery(abacate domain.AbacatePayAdapter, notifier PixRenewalNotifier) {
	uc.abacate = abacate
	uc.notifier = notifier
}

// WithClock pins the clock (tests).
func (uc *SendPixRenewalCycleUseCase) WithClock(now func() time.Time) *SendPixRenewalCycleUseCase {
	uc.now = now
	return uc
}

// Execute advances every due account by at most one rung. Returns how
// many rungs were delivered.
func (uc *SendPixRenewalCycleUseCase) Execute(ctx context.Context) (int, error) {
	if uc.repo == nil {
		return 0, nil
	}
	now := uc.now()
	accounts, err := uc.repo.ListDue(ctx, now, 0)
	if err != nil {
		return 0, err
	}

	delivered := 0
	for _, acc := range accounts {
		stage := domain.NextPixDunningStage(
			domain.DaysFromRenewal(now, acc.PeriodEnd), acc.StageForPeriod())
		if stage == domain.PixStageNone {
			continue
		}
		if !uc.deliver(ctx, acc, stage) {
			// Not stamped: a rung that failed must be retried on the next
			// run, not silently skipped for the rest of the period.
			continue
		}
		if err := uc.repo.MarkStage(ctx, acc.AccountID, stage, acc.PeriodEnd); err != nil {
			slog.WarnContext(ctx, "pix cycle: failed to stamp stage",
				"account_id", acc.AccountID, "stage", stage, "error", err)
			continue
		}
		delivered++
	}
	return delivered, nil
}

// deliver performs one rung. Reports whether it actually happened.
func (uc *SendPixRenewalCycleUseCase) deliver(ctx context.Context, acc domain.PixRenewalAccount, stage domain.PixDunningStage) bool {
	if stage == domain.PixStageSuspended {
		// The only rung with a side effect on service. Suspension is
		// reversible (a paid webhook reactivates), which is why it can be
		// automated at all.
		if uc.accountRepo == nil {
			return false
		}
		if err := uc.accountRepo.Suspend(ctx, acc.AccountID, "pix_renewal_unpaid"); err != nil {
			slog.ErrorContext(ctx, "pix cycle: suspend failed",
				"account_id", acc.AccountID, "error", err)
			return false
		}
		slog.InfoContext(ctx, "pix cycle: account suspended for non-payment",
			"account_id", acc.AccountID, "period_end", acc.PeriodEnd)
		return true
	}

	plan := uc.planFor(ctx, acc.PlanID)
	if stage == domain.PixStageCharge {
		// Minting is best-effort: if AbacatePay is unreachable we still
		// want the reminder ladder to run, so the customer is not silently
		// suspended without ever being asked.
		uc.mintCharge(ctx, acc, plan)
	}

	if uc.notifier == nil || uc.users == nil {
		// Nothing to send with — but the rung still counts as handled, or
		// a deployment without email would suspend accounts that were
		// never told anything.
		return uc.notifier == nil && uc.users == nil
	}
	admin, err := uc.users.GetAdminByAccountID(ctx, acc.AccountID)
	if err != nil || admin == nil || admin.Email == "" {
		slog.WarnContext(ctx, "pix cycle: no admin to notify",
			"account_id", acc.AccountID, "error", err)
		return false
	}
	planName := "Replider"
	if plan != nil {
		planName = plan.Name
	}
	days := -domain.DaysFromRenewal(uc.now(), acc.PeriodEnd)
	if err := uc.notifier.SendPrebillingEmail(
		admin.Email, admin.Name, planName, days, uc.appBaseURL+"/billing", "pt-BR"); err != nil {
		slog.WarnContext(ctx, "pix cycle: notification failed",
			"account_id", acc.AccountID, "stage", stage, "error", err)
		return false
	}
	return true
}

func (uc *SendPixRenewalCycleUseCase) planFor(ctx context.Context, planID string) *domain.Plan {
	if uc.planRepo == nil || planID == "" {
		return nil
	}
	plan, err := uc.planRepo.GetByID(ctx, planID)
	if err != nil {
		return nil
	}
	return plan
}

// mintCharge creates the renewal PIX and remembers it, so the reminders
// point at the SAME code instead of minting a second one the customer
// might also pay.
func (uc *SendPixRenewalCycleUseCase) mintCharge(ctx context.Context, acc domain.PixRenewalAccount, plan *domain.Plan) {
	if uc.abacate == nil || plan == nil || acc.ChargeID != "" {
		return
	}
	charge, err := uc.abacate.CreatePixCharge(ctx, domain.PixChargeInput{
		AmountCents: int64(plan.Price),
		Description: "Renovação " + plan.Name,
		// Idempotency key: one charge per account per period, so a retried
		// run cannot bill the customer twice for the same month.
		ExternalID: "renewal:" + acc.AccountID + ":" + acc.PeriodEnd.Format("2006-01"),
		Metadata: map[string]string{
			"account_id": acc.AccountID,
			"plan_id":    plan.ID,
			"kind":       "renewal",
		},
	})
	if err != nil || charge == nil {
		slog.WarnContext(ctx, "pix cycle: could not mint the renewal charge",
			"account_id", acc.AccountID, "error", err)
		return
	}
	if err := uc.repo.SetChargeID(ctx, acc.AccountID, charge.ID); err != nil {
		slog.WarnContext(ctx, "pix cycle: could not store the charge id",
			"account_id", acc.AccountID, "error", err)
	}
}

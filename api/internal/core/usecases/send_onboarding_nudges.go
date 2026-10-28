package usecases

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// OnboardingNudgeStore is the narrow port the onboarding-nudge use case
// needs over the accounts table (satisfied by
// postgres.OnboardingNudgeRepository). Kept small so the test fake stays
// tiny and the use case never drags the full sqlc AccountRepository.
type OnboardingNudgeStore interface {
	ListDueForNudge(ctx context.Context, createdAfter time.Time) ([]domain.OnboardingNudgeAccount, error)
	MarkOnboardingNudgeSent(ctx context.Context, accountID string, stage int) error
}

// OnboardingNudgeNotifier sends the rendered onboarding nudge email.
// Satisfied by *email.SMTPAdapter (the method is deliberately NOT on the
// broad EmailService interface — mirrors SendReactivationEmail). The
// milestone selects the copy (one of the OnboardingMilestone* constants).
type OnboardingNudgeNotifier interface {
	SendOnboardingNudgeEmail(to, userName string, milestone string, ctaURL, locale string) error
}

// OnboardingNudgeProgress reads an account's SALES milestones — the same
// six GET /me/onboarding-progress reports, percentage included. Satisfied
// by *GetOnboardingProgressUseCase. Reading it here rather than widening
// the due-list query keeps "what counts as done" in exactly one place.
type OnboardingNudgeProgress interface {
	Execute(ctx context.Context, accountID string) (*domain.OnboardingProgress, error)
}

// The six sales milestones, in the order the product needs them done. The
// string is the copy key suffix in email/strings.go (onboarding_nudge_<m>_*)
// and the utm_content of the nudge's link.
const (
	OnboardingMilestoneChannel      = "channel"
	OnboardingMilestoneProducts     = "products"
	OnboardingMilestoneCustomers    = "customers"
	OnboardingMilestoneProposal     = "proposal"
	OnboardingMilestoneFirstInbound = "first_inbound"
	OnboardingMilestoneTeamInvited  = "team_invited"
)

// onboardingMilestoneOrder is the order a nudge walks the milestones: the
// first one still pending is what the email asks for. Setup before use,
// use before replies, and the seat for the seller last — the owner
// finishes the wizard alone and invites her once the first offer has made
// the case.
var onboardingMilestoneOrder = []string{
	OnboardingMilestoneChannel,
	OnboardingMilestoneProducts,
	OnboardingMilestoneCustomers,
	OnboardingMilestoneProposal,
	OnboardingMilestoneFirstInbound,
	OnboardingMilestoneTeamInvited,
}

// onboardingMilestonePaths is where each milestone gets done. Every path is
// a live route of the reduced product — the old nudges linked /knowledge
// and "share your agent", screens archived with v1.
var onboardingMilestonePaths = map[string]string{
	OnboardingMilestoneChannel:      "/settings/channels",
	OnboardingMilestoneProducts:     "/sales/products/import",
	OnboardingMilestoneCustomers:    "/contacts/import",
	OnboardingMilestoneProposal:     "/prospect?tab=queue",
	OnboardingMilestoneFirstInbound: "/prospect?tab=queue",
	OnboardingMilestoneTeamInvited:  "/settings/team",
}

// NextOnboardingMilestone returns the first milestone still pending, or ""
// when the six are all done.
func NextOnboardingMilestone(p *domain.OnboardingProgress) string {
	if p == nil {
		return OnboardingMilestoneChannel
	}
	done := map[string]bool{
		OnboardingMilestoneChannel:      p.ChannelConnected,
		OnboardingMilestoneProducts:     p.ProductsAdded,
		OnboardingMilestoneCustomers:    p.CustomersAdded,
		OnboardingMilestoneProposal:     p.ProposalMade,
		OnboardingMilestoneFirstInbound: p.FirstInboundReceived,
		OnboardingMilestoneTeamInvited:  p.TeamInvited,
	}
	for _, m := range onboardingMilestoneOrder {
		if !done[m] {
			return m
		}
	}
	return ""
}

// onboardingNudgeMaxAge bounds the due-list to recently-created accounts so
// a first deploy doesn't blast the entire existing user base — only
// accounts that signed up within this window are ever nudged. The last
// stage is day 7, so two weeks leaves headroom for a worker backlog.
const onboardingNudgeMaxAge = 14 * 24 * time.Hour

// SendOnboardingNudgesUseCase drips activation emails to accounts that
// haven't finished onboarding, at D1 / D3 / D7 after signup (US-182). Each
// stage asks for the FIRST of the six sales milestones still pending
// (channel → products → customers → proposal → first inbound → team), so
// the three emails follow the person instead of a fixed script — an
// account that connected its mailbox on day one is asked for products on
// day three, not for the channel it already has. An account at 100% is
// skipped (no email) but its stamp still advances, so the drip moves on.
// Idempotent per stage via the store's onboarding_nudge_stage_sent stamp,
// so the daily cron can tick freely without re-sending a stage.
type SendOnboardingNudgesUseCase struct {
	store    OnboardingNudgeStore
	admins   DigestAdminResolver // reused: resolves the account admin
	notifier OnboardingNudgeNotifier
	progress OnboardingNudgeProgress
	appURL   string
	// now is injectable so tests pin "today" relative to seeded
	// created_at timestamps without sleeping.
	now func() time.Time
}

func NewSendOnboardingNudgesUseCase(
	store OnboardingNudgeStore,
	admins DigestAdminResolver,
	notifier OnboardingNudgeNotifier,
	progress OnboardingNudgeProgress,
	appURL string,
) *SendOnboardingNudgesUseCase {
	return &SendOnboardingNudgesUseCase{
		store:    store,
		admins:   admins,
		notifier: notifier,
		progress: progress,
		appURL:   appURL,
		now:      time.Now,
	}
}

// WithClock overrides the time source (tests only).
func (uc *SendOnboardingNudgesUseCase) WithClock(now func() time.Time) *SendOnboardingNudgesUseCase {
	uc.now = now
	return uc
}

// Execute emails every due account whose next onboarding milestone is now
// pending, and returns the number actually sent. A single account's failure
// (missing admin, send error) is logged and skipped — it never aborts the
// batch, and an unstamped account is simply retried next tick.
func (uc *SendOnboardingNudgesUseCase) Execute(ctx context.Context) (int, error) {
	now := uc.now()
	due, err := uc.store.ListDueForNudge(ctx, now.Add(-onboardingNudgeMaxAge))
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
		slog.Info("onboarding nudge emails sent", "count", sent)
	}
	return sent, nil
}

// sendOne decides which stage is due for the account, skips it if the
// account is fully onboarded (advancing the stamp), otherwise asks for the
// first pending milestone and stamps. Returns true only when an email
// actually went out, so an account whose admin (or progress) we can't
// resolve is retried (not silently marked done).
func (uc *SendOnboardingNudgesUseCase) sendOne(ctx context.Context, now time.Time, acc domain.OnboardingNudgeAccount) bool {
	daysSince := int(now.Sub(acc.CreatedAt).Hours() / 24)
	stage := domain.NextOnboardingNudgeStage(daysSince, acc.StageSent)
	if stage == 0 {
		// Listed (a nudge is still pending) but not yet elapsed to the
		// next milestone — nothing to do this tick.
		return false
	}

	if uc.progress == nil {
		slog.Warn("onboarding nudge: progress reader not wired; nothing sent", "account_id", acc.AccountID)
		return false
	}
	progress, err := uc.progress.Execute(ctx, acc.AccountID)
	if err != nil || progress == nil {
		slog.Warn("onboarding nudge: could not read onboarding progress", "account_id", acc.AccountID, "error", err)
		return false
	}

	// "Skip if already done": nothing left to ask for. Don't email, but
	// advance the stamp so the next tick moves on to the following stage
	// instead of re-evaluating this one forever.
	milestone := NextOnboardingMilestone(progress)
	if progress.CompletionPercentage >= 100 || milestone == "" {
		if err := uc.store.MarkOnboardingNudgeSent(ctx, acc.AccountID, stage); err != nil {
			slog.Error("onboarding nudge: mark (skip) failed", "account_id", acc.AccountID, "error", err)
		}
		return false
	}

	admin, err := uc.admins.GetAdminByAccountID(ctx, acc.AccountID)
	if err != nil || admin == nil || admin.Email == "" {
		slog.Warn("onboarding nudge: could not resolve admin", "account_id", acc.AccountID, "error", err)
		return false
	}

	if err := uc.notifier.SendOnboardingNudgeEmail(admin.Email, admin.Name, milestone, uc.nudgeURL(stage, milestone), ""); err != nil {
		slog.Error("onboarding nudge: send failed", "account_id", acc.AccountID, "email", admin.Email, "error", err)
		return false
	}

	if err := uc.store.MarkOnboardingNudgeSent(ctx, acc.AccountID, stage); err != nil {
		// Email already left; a failed stamp means it may resend next
		// tick. Log loudly but count it as sent.
		slog.Error("onboarding nudge: mark sent failed", "account_id", acc.AccountID, "error", err)
	}
	return true
}

// nudgeURL builds the UTM-tagged deep link for the milestone's call to
// action so the click is tracked through the existing UTM-capture pipeline
// (utm_bootstrap on the app), attributing activations per stage and
// milestone via utm_content. Each milestone lands on the screen where it
// gets done; a path that already carries a query (the offers queue) gets
// the UTMs appended, not a second '?'.
func (uc *SendOnboardingNudgesUseCase) nudgeURL(stage int, milestone string) string {
	path, ok := onboardingMilestonePaths[milestone]
	if !ok {
		path = "/start"
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return fmt.Sprintf("%s%s%sutm_source=lifecycle&utm_medium=email&utm_campaign=onboarding_nudge&utm_content=d%d_%s",
		uc.appURL, path, sep, stage, milestone)
}

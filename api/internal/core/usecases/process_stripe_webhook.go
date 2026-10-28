package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/email"
	"github.com/replider/api/internal/core/domain"
	"github.com/stripe/stripe-go/v76"
)

// OverageSettler bills any outstanding conversation overage for an
// account and closes the cycle. Implemented by services.OverageSettler.
// Invoked on invoice.paid so overage accrued since the worker's last
// tick is billed, and the ledger archived, before ResetUsage zeroes
// the cycle counters.
type OverageSettler interface {
	Settle(ctx context.Context, c domain.OverageCandidate) error
	CloseCycle(ctx context.Context, accountID string) error
}

type ProcessStripeWebhookUseCase struct {
	accountRepo     domain.AccountRepository
	usageRepo       domain.UsageRepository
	stripeEventRepo domain.StripeEventRepository
	userRepo        domain.UserRepository
	planRepo        domain.PlanRepository
	billingEvents   domain.BillingEventRepository
	emailService    email.EmailService
	overageSettler  OverageSettler
	appBaseURL      string
	// Referral reward dependencies (US-174). Optional and nil-safe —
	// when unwired the webhook processes events normally but grants no
	// referral credit.
	referralRepo domain.ReferralRepository
	payment      domain.PaymentAdapter
	// Win-back recovery flow (US-179). Optional and nil-safe — when
	// unwired a cancellation is processed normally but no recovery coupon
	// or email is generated.
	winBack WinBackInitiator
	// settings backs the USD↔BRL conversion of cross-currency referral
	// credits (US-015 review fixes). Optional and nil-safe — unwired
	// falls back to DefaultUSDBRLRateCents.
	settings domain.PlatformSettingRepository
}

// WinBackInitiator kicks off the win-back recovery flow when a paid
// subscription is canceled (US-179). Satisfied by *WinBackUseCase.
type WinBackInitiator interface {
	Start(ctx context.Context, accountID, customerID string)
}

// SetWinBackInitiator injects the win-back recovery flow (US-179).
// Optional — when unset, cancellations are processed without a win-back
// offer.
func (uc *ProcessStripeWebhookUseCase) SetWinBackInitiator(wb WinBackInitiator) {
	uc.winBack = wb
}

// SetPlanActivator injects the dependencies used to map a Stripe price id
// to a plan and activate it on the account after a paid checkout (US-001).
// Without it a completed checkout only flips subscription_status — the
// account would keep its previous (trial) plan limits, so main.go MUST
// wire this; the nil-check below is a last-resort guard that logs loudly.
func (uc *ProcessStripeWebhookUseCase) SetPlanActivator(planRepo domain.PlanRepository, payment domain.PaymentAdapter) {
	uc.planRepo = planRepo
	uc.payment = payment
}

// SetReferralRewarder injects the dependencies that grant the "1 month
// free for both" referral reward when a referred account makes its first
// paid subscription (US-174). Optional — when unset, no reward is granted.
func (uc *ProcessStripeWebhookUseCase) SetReferralRewarder(referralRepo domain.ReferralRepository, payment domain.PaymentAdapter) {
	uc.referralRepo = referralRepo
	uc.payment = payment
}

// SetPlatformSettings wires the platform_settings KV so cross-currency
// referral credits convert at the owner-managed USD↔BRL rate (US-015
// review fixes) instead of copying the raw cents figure across
// currencies. Optional — nil falls back to DefaultUSDBRLRateCents.
func (uc *ProcessStripeWebhookUseCase) SetPlatformSettings(s domain.PlatformSettingRepository) {
	uc.settings = s
}

// SetBillingEventRecorder injects the revenue-ledger repository (US-071).
// Optional and nil-safe — when unset the webhook processes events
// normally but writes no analytics rows. Recording is best-effort: a
// ledger failure never fails the webhook (it would force Stripe to retry
// the primary side effects).
func (uc *ProcessStripeWebhookUseCase) SetBillingEventRecorder(repo domain.BillingEventRepository) {
	uc.billingEvents = repo
}

// SetOverageSettler injects the overage settler used to bill pending
// conversation overage at cycle boundaries. Optional — without it
// invoice.paid still resets usage, just without the final settle.
func (uc *ProcessStripeWebhookUseCase) SetOverageSettler(s OverageSettler) {
	uc.overageSettler = s
}

// SetEmailNotifier injects the dependencies needed to send payment-failure
// emails. Optional — when not set the webhook still processes events but
// does not send email notifications.
func (uc *ProcessStripeWebhookUseCase) SetEmailNotifier(userRepo domain.UserRepository, planRepo domain.PlanRepository, emailSvc email.EmailService, appBaseURL string) {
	uc.userRepo = userRepo
	uc.planRepo = planRepo
	uc.emailService = emailSvc
	uc.appBaseURL = appBaseURL
}

func NewProcessStripeWebhookUseCase(accountRepo domain.AccountRepository, usageRepo domain.UsageRepository, stripeEventRepo domain.StripeEventRepository) *ProcessStripeWebhookUseCase {
	return &ProcessStripeWebhookUseCase{
		accountRepo:     accountRepo,
		usageRepo:       usageRepo,
		stripeEventRepo: stripeEventRepo,
	}
}

func (uc *ProcessStripeWebhookUseCase) Execute(ctx context.Context, event stripe.Event) error {
	// Store the raw event alongside the claim so a crashed handler can be
	// replayed by the reconciler without re-fetching from Stripe (US-011).
	payload, err := json.Marshal(event)
	if err != nil {
		slog.Error("failed to marshal stripe event for claim", "event_id", event.ID, "error", err)
		return err
	}

	// Idempotency + crash recovery: claim the event atomically BEFORE
	// running side effects. Stripe retries non-2xx with at-least-once
	// semantics, so two retries can land in parallel; ClaimEvent ensures
	// only one wins (and re-claims a stuck row from a prior crash).
	claimed, err := uc.stripeEventRepo.ClaimEvent(ctx, event.ID, string(event.Type), payload)
	if err != nil {
		// Transient DB error — return failure so Stripe retries the
		// event. Better to retry than to drop or double-process.
		slog.Error("failed to claim stripe event for processing", "event_id", event.ID, "error", err)
		return err
	}
	if !claimed {
		slog.Info("skipping already-processed or in-flight stripe event", "event_id", event.ID, "type", event.Type)
		return nil
	}

	if err := uc.processEvent(ctx, event); err != nil {
		// Side-effects failed after the claim. We deliberately do NOT
		// mark the event processed: processed_at stays NULL, so the row
		// is now a "stuck claim". Stripe will retry the delivery (we
		// return 500), and if Stripe gives up the reconciler re-runs the
		// stored payload after the 5-minute claim window. Either path
		// re-claims via ClaimEvent, so the event is never silently
		// dropped — the regression this story fixes.
		slog.Error("stripe event processing failed after claim, left for retry",
			"event_id", event.ID, "type", event.Type, "error", err)
		return err
	}

	// Side-effects committed — stamp processed_at so no retry re-runs it.
	if err := uc.stripeEventRepo.MarkProcessed(ctx, event.ID, string(event.Type)); err != nil {
		// The work is done but we couldn't record it. Returning an error
		// would make Stripe retry and re-run the (mostly idempotent)
		// side-effects; instead log and succeed — worst case the
		// reconciler re-runs it once after the claim window.
		slog.Error("stripe event processed but MarkProcessed failed",
			"event_id", event.ID, "type", event.Type, "error", err)
	}

	return nil
}

// ReconcileStuck re-runs Stripe events that were claimed but never
// completed — handlers that crashed mid-flight (US-011). It re-claims
// each via ClaimEvent (resetting the window and serializing against
// concurrent reconcilers / live retries), replays the stored payload,
// and marks it processed on success. Returns the number recovered.
func (uc *ProcessStripeWebhookUseCase) ReconcileStuck(ctx context.Context) (int, error) {
	stuck, err := uc.stripeEventRepo.ListStuckEvents(ctx)
	if err != nil {
		return 0, err
	}

	recovered := 0
	for _, rec := range stuck {
		var event stripe.Event
		if err := json.Unmarshal(rec.Payload, &event); err != nil {
			// A row we can't replay would loop forever; mark it processed
			// to drain it and surface loudly for manual follow-up.
			slog.Error("reconcile: unparseable stored stripe event, draining",
				"event_id", rec.EventID, "error", err)
			_ = uc.stripeEventRepo.MarkProcessed(ctx, rec.EventID, rec.EventType)
			continue
		}

		// Re-claim resets claimed_at and skips rows another worker grabbed.
		claimed, err := uc.stripeEventRepo.ClaimEvent(ctx, event.ID, string(event.Type), rec.Payload)
		if err != nil {
			slog.Error("reconcile: re-claim failed", "event_id", event.ID, "error", err)
			continue
		}
		if !claimed {
			continue
		}

		if err := uc.processEvent(ctx, event); err != nil {
			slog.Error("reconcile: reprocess failed, will retry next cycle",
				"event_id", event.ID, "type", event.Type, "error", err)
			continue
		}
		if err := uc.stripeEventRepo.MarkProcessed(ctx, event.ID, string(event.Type)); err != nil {
			slog.Error("reconcile: MarkProcessed failed", "event_id", event.ID, "error", err)
			continue
		}
		recovered++
		slog.Info("reconcile: recovered stuck stripe event", "event_id", event.ID, "type", event.Type)
	}

	return recovered, nil
}

func (uc *ProcessStripeWebhookUseCase) processEvent(ctx context.Context, event stripe.Event) error {
	switch event.Type {
	case "checkout.session.completed":
		return uc.handleCheckoutCompleted(ctx, event)

	case "invoice.paid":
		return uc.handleInvoicePaid(ctx, event)

	case "invoice.payment_failed":
		return uc.handleInvoicePaymentFailed(ctx, event)

	case "customer.subscription.created", "customer.subscription.updated":
		return uc.handleSubscriptionUpdated(ctx, event)

	case "customer.subscription.deleted":
		return uc.handleSubscriptionDeleted(ctx, event)
	}

	return nil
}

func (uc *ProcessStripeWebhookUseCase) handleCheckoutCompleted(ctx context.Context, event stripe.Event) error {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		return fmt.Errorf("error parsing checkout session: %w", err)
	}

	accountID := session.ClientReferenceID
	if accountID == "" {
		return fmt.Errorf("missing client reference ID on checkout session")
	}

	// Check if this is an addon purchase
	if session.Metadata != nil && session.Metadata["type"] == "addon" {
		packID := session.Metadata["pack_id"]
		var pack *AddonPack
		for i := range AddonPacks {
			if AddonPacks[i].ID == packID {
				pack = &AddonPacks[i]
				break
			}
		}
		if pack == nil {
			return fmt.Errorf("unknown addon pack ID: %s", packID)
		}

		if err := uc.usageRepo.AddAddonTokens(ctx, accountID, pack.Tokens); err != nil {
			return fmt.Errorf("failed to add addon tokens for account %s: %w", accountID, err)
		}
		slog.Info("successfully added addon tokens", "tokens", pack.Tokens, "pack_id", packID, "account_id", accountID)
		return nil
	}

	// Standard subscription checkout
	customerID := session.Customer.ID
	var subscriptionID string
	if session.Subscription != nil {
		subscriptionID = session.Subscription.ID
	}

	err := uc.accountRepo.UpdateAccountSubscription(ctx, accountID, &customerID, &subscriptionID, domain.SubscriptionStatusActive)
	if err != nil {
		return fmt.Errorf("failed to update account subscription: %w", err)
	}
	slog.Info("successfully activated subscription", "subscription_id", subscriptionID, "account_id", accountID)

	// Activate the purchased plan on the account (US-001). Without this
	// the customer pays but keeps trial limits — accounts.plan_id was
	// never written by any webhook. Transient failures return an error so
	// Stripe retries; an unmappable price id logs + ledgers an alert row
	// instead (retrying won't fix bad price wiring).
	if err := uc.activatePurchasedPlan(ctx, accountID, subscriptionID, session.AmountTotal, string(session.Currency), time.Unix(event.Created, 0).UTC()); err != nil {
		return err
	}

	// Ledger the new subscription for revenue analytics (US-071). Skip the
	// extra account lookup entirely when the recorder is unwired.
	if uc.billingEvents != nil {
		acct, _ := uc.accountRepo.GetByID(ctx, accountID)
		uc.recordBillingEvent(ctx, accountID, domain.BillingEventSubscriptionCreated, uc.planNameFor(ctx, acct), session.AmountTotal, string(session.Currency), time.Unix(event.Created, 0).UTC())
	}

	// Referral reward (US-174): this is the account's first paid checkout.
	// If they signed up via someone's referral link, credit both sides a
	// month. Best-effort — a reward failure never fails the webhook.
	uc.grantReferralReward(ctx, accountID, customerID, session.AmountTotal, string(session.Currency))
	return nil
}

// activatePurchasedPlan resolves the plan behind the checkout's
// subscription price id and writes accounts.plan_id + clears the trial
// deadline (US-001). Returns an error only for transient failures
// (Stripe fetch / DB write) so the webhook is retried; an unmapped price
// id is a wiring bug retries can't fix — it logs a structured error and
// ledgers a plan_activation_failed alert row instead of silently leaving
// the paying customer on trial limits.
func (uc *ProcessStripeWebhookUseCase) activatePurchasedPlan(ctx context.Context, accountID, subscriptionID string, amountCents int64, currency string, occurredAt time.Time) error {
	if uc.planRepo == nil || uc.payment == nil {
		slog.Error("checkout completed but plan activator unwired — account keeps previous plan limits",
			"account_id", accountID, "subscription_id", subscriptionID)
		return nil
	}
	if subscriptionID == "" {
		slog.Error("checkout completed without subscription id — cannot resolve purchased plan",
			"account_id", accountID)
		return nil
	}

	priceID, err := uc.payment.GetSubscriptionPriceID(subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to fetch subscription price for plan activation (account %s, sub %s): %w", accountID, subscriptionID, err)
	}

	plan, err := uc.planRepo.GetByStripePriceID(ctx, priceID)
	if err != nil {
		return fmt.Errorf("failed to resolve plan by stripe price %s (account %s): %w", priceID, accountID, err)
	}
	if plan == nil {
		slog.Error("checkout price id maps to no plan — paying account left on previous limits, reconcile manually",
			"account_id", accountID, "subscription_id", subscriptionID, "price_id", priceID)
		uc.recordBillingEvent(ctx, accountID, domain.BillingEventPlanActivationFailed, priceID, amountCents, currency, occurredAt)
		return nil
	}

	// Detect a self-serve paid→pricier-paid move BEFORE overwriting
	// plan_id — it feeds the commission upgrade bonus (US-008), which
	// previously only fired on the owner-panel path. Best-effort; the
	// lookup only runs when there is a recorder to feed.
	if uc.billingEvents != nil {
		if acct, err := uc.accountRepo.GetByID(ctx, accountID); err == nil && acct != nil {
			uc.recordSelfServePlanUpgrade(ctx, acct, plan, occurredAt)
		}
	}

	if err := uc.accountRepo.SetPlan(ctx, accountID, plan.ID); err != nil {
		return fmt.Errorf("failed to set plan %s on account %s: %w", plan.ID, accountID, err)
	}
	if err := uc.accountRepo.ClearTrialExpiresAt(ctx, accountID); err != nil {
		return fmt.Errorf("failed to clear trial deadline on account %s: %w", accountID, err)
	}
	slog.Info("activated purchased plan", "account_id", accountID, "plan_id", plan.ID, "plan_name", plan.Name, "price_id", priceID)
	return nil
}

// recordSelfServePlanUpgrade ledgers a 'plan_upgraded' billing event
// when a Stripe-driven plan change moved the account from one PAID plan
// to a pricier one in the same currency — the signal the commission
// upgrade bonus (US-008) reads. Mirrors the owner-panel rules
// (OwnerAdminUseCase.recordPlanUpgrade): trial→paid is a conversion,
// not an upgrade; cross-currency moves aren't comparable. Best-effort —
// never fails the webhook. account must carry the PRE-change PlanID.
func (uc *ProcessStripeWebhookUseCase) recordSelfServePlanUpgrade(ctx context.Context, account *domain.Account, newPlan *domain.Plan, occurredAt time.Time) {
	if uc.billingEvents == nil || uc.planRepo == nil || account.PlanID == nil || *account.PlanID == newPlan.ID {
		return
	}
	oldPlan, err := uc.planRepo.GetByID(ctx, *account.PlanID)
	if err != nil || oldPlan == nil {
		return
	}
	if oldPlan.Price <= 0 || oldPlan.Currency != newPlan.Currency || newPlan.Price <= oldPlan.Price {
		return
	}
	uc.recordBillingEvent(ctx, account.ID, domain.BillingEventPlanUpgraded, newPlan.Name, int64(newPlan.Price), newPlan.Currency, occurredAt)
	slog.Info("self-serve plan upgrade ledgered",
		"account_id", account.ID, "old_plan", oldPlan.Name, "new_plan", newPlan.Name)
}

// grantReferralReward credits the referrer and the referee one month's
// worth of account balance the first time the referee pays (US-174). The
// referrals status guard + per-side Stripe idempotency keys make a replay
// safe (no double credit).
func (uc *ProcessStripeWebhookUseCase) grantReferralReward(ctx context.Context, refereeAccountID, refereeCustomerID string, amountCents int64, currency string) {
	if uc.referralRepo == nil || uc.payment == nil || amountCents <= 0 {
		return
	}
	ref, err := uc.referralRepo.GetPendingByReferee(ctx, refereeAccountID)
	if err != nil {
		slog.Error("referral: failed to load pending referral", "account_id", refereeAccountID, "error", err)
		return
	}
	if ref == nil {
		return // not a referred account, or already rewarded
	}

	if refereeCustomerID != "" {
		// The referee's credit rides the checkout currency — which IS their
		// account currency (checkout enforces the match, US-003).
		if err := uc.payment.GrantReferralCredit(refereeCustomerID, amountCents, currency, "Recompensa de indicação (indicado)", "referral_referee_"+ref.ID); err != nil {
			slog.Error("referral: failed to credit referee", "referral_id", ref.ID, "error", err)
		}
	}

	if referrerCustomerID, referrerCurrency := uc.resolveReferrerCustomer(ctx, ref.ReferrerAccountID); referrerCustomerID != "" {
		// The referrer may bill in a different currency than the referee
		// paid (cross-currency referral). A Stripe balance credit only
		// offsets invoices in its own currency, so credit the referrer in
		// THEIR account currency — CONVERTED at the owner-managed USD↔BRL
		// rate (US-015 review fixes: crediting the same cents figure in a
		// different currency turned an R$199 referee into a $199 credit,
		// ~5x the reward).
		if referrerCurrency == "" {
			referrerCurrency = currency
		}
		referrerCents, ok := uc.convertReferralCents(ctx, amountCents, currency, referrerCurrency)
		if !ok {
			slog.Error("referral: cannot convert credit between currencies — referrer credit skipped, reconcile manually",
				"referral_id", ref.ID, "from_currency", currency, "to_currency", referrerCurrency, "amount_cents", amountCents)
		} else if err := uc.payment.GrantReferralCredit(referrerCustomerID, referrerCents, referrerCurrency, "Recompensa de indicação (indicador)", "referral_referrer_"+ref.ID); err != nil {
			slog.Error("referral: failed to credit referrer", "referral_id", ref.ID, "error", err)
		}
	}

	if err := uc.referralRepo.MarkRewarded(ctx, ref.ID, amountCents, currency, time.Now()); err != nil {
		slog.Error("referral: failed to mark referral rewarded", "referral_id", ref.ID, "error", err)
		return
	}
	slog.Info("referral reward granted", "referral_id", ref.ID, "referrer", ref.ReferrerAccountID, "referee", refereeAccountID, "amount_cents", amountCents)
}

// convertReferralCents converts a referral credit between the referee's
// checkout currency and the referrer's account currency at the
// owner-managed manual rate (SettingUSDBRLRateCents). Same currency
// passes through; an unpriceable pair returns ok=false so the caller
// skips the credit instead of granting a wrong amount. A settings read
// error degrades to the default rate — the reward is goodwill, a
// slightly stale rate beats dropping the credit.
func (uc *ProcessStripeWebhookUseCase) convertReferralCents(ctx context.Context, cents int64, from, to string) (int64, bool) {
	rate, err := getIntSetting(ctx, uc.settings, domain.SettingUSDBRLRateCents, DefaultUSDBRLRateCents)
	if err != nil {
		slog.Warn("referral: failed to read usd_brl rate, using default", "error", err)
		rate = DefaultUSDBRLRateCents
	}
	return domain.ConvertCents(cents, from, to, rate)
}

// resolveReferrerCustomer returns the Stripe customer id to credit for the
// referrer, plus their account billing currency (empty when the account
// lookup failed — the caller falls back to the checkout currency). If
// they've never transacted, it mints and persists a customer so the credit
// lands on the same customer their future checkout will reuse.
func (uc *ProcessStripeWebhookUseCase) resolveReferrerCustomer(ctx context.Context, referrerAccountID string) (string, string) {
	acct, err := uc.accountRepo.GetByID(ctx, referrerAccountID)
	referrerCurrency := ""
	if err == nil && acct != nil {
		referrerCurrency = acct.Currency
		if acct.StripeCustomerID != nil && *acct.StripeCustomerID != "" {
			return *acct.StripeCustomerID, referrerCurrency
		}
	}
	custID, err := uc.payment.EnsureCustomer(referrerAccountID)
	if err != nil {
		slog.Error("referral: failed to ensure referrer stripe customer", "account_id", referrerAccountID, "error", err)
		return "", referrerCurrency
	}
	effective, err := uc.accountRepo.SetStripeCustomerIfAbsent(ctx, referrerAccountID, custID)
	if err != nil || effective == "" {
		return custID, referrerCurrency
	}
	return effective, referrerCurrency
}

func (uc *ProcessStripeWebhookUseCase) handleInvoicePaid(ctx context.Context, event stripe.Event) error {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		return fmt.Errorf("error parsing invoice: %w", err)
	}

	var subscriptionID string
	if invoice.Subscription != nil {
		subscriptionID = invoice.Subscription.ID
	}

	if subscriptionID == "" {
		return nil // Not a subscription invoice
	}

	account, err := uc.accountRepo.GetAccountByStripeSubscription(ctx, subscriptionID)
	if err != nil || account == nil {
		slog.Warn("received invoice.paid for unknown subscription", "subscription_id", subscriptionID)
		return nil
	}

	// Ensure subscription is active (in case it was past_due)
	if account.SubscriptionStatus != domain.SubscriptionStatusActive {
		_ = uc.accountRepo.UpdateAccountSubscription(ctx, account.ID, account.StripeCustomerID, &subscriptionID, domain.SubscriptionStatusActive)
	}

	// Settle any conversation overage accrued since the worker's last
	// tick BEFORE ResetUsage zeroes conversations_count — otherwise the
	// tail of the cycle would never be billed. Best-effort: the worker
	// has already billed most of the cycle in 6h increments, so a rare
	// Stripe failure here loses only the final-window delta. The
	// idempotency key keeps the settle safe if it partially ran.
	if uc.overageSettler != nil {
		if cand, cErr := uc.usageRepo.GetOverageCandidate(ctx, account.ID); cErr != nil {
			slog.Error("invoice.paid: load overage candidate failed",
				"account_id", account.ID, "error", cErr)
		} else if cand != nil {
			if sErr := uc.overageSettler.Settle(ctx, *cand); sErr != nil {
				slog.Error("invoice.paid: final overage settle failed — tail overage may be uncharged",
					"account_id", account.ID, "error", sErr)
			}
		}
		// Close the overage ledger cycle so this cycle's charged rows
		// stop counting toward the next cycle's total.
		if cErr := uc.overageSettler.CloseCycle(ctx, account.ID); cErr != nil {
			slog.Error("invoice.paid: close overage cycle failed",
				"account_id", account.ID, "error", cErr)
		}
	}

	err = uc.usageRepo.ResetUsage(ctx, account.ID)
	if err != nil {
		return fmt.Errorf("failed to reset usage for account %s: %w", account.ID, err)
	}
	slog.Info("successfully reset AI tokens", "subscription_id", subscriptionID, "account_id", account.ID)

	// Ledger the recurring payment for revenue analytics (US-071).
	uc.recordBillingEvent(ctx, account.ID, domain.BillingEventSubscriptionRenewed, uc.planNameFor(ctx, account), invoice.AmountPaid, string(invoice.Currency), time.Unix(event.Created, 0).UTC())
	return nil
}

func (uc *ProcessStripeWebhookUseCase) handleInvoicePaymentFailed(ctx context.Context, event stripe.Event) error {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		return fmt.Errorf("error parsing invoice: %w", err)
	}

	var subscriptionID string
	if invoice.Subscription != nil {
		subscriptionID = invoice.Subscription.ID
	}

	if subscriptionID == "" {
		return nil
	}

	account, err := uc.accountRepo.GetAccountByStripeSubscription(ctx, subscriptionID)
	if err != nil || account == nil {
		slog.Warn("received invoice.payment_failed for unknown subscription", "subscription_id", subscriptionID)
		return nil
	}

	// Mark account as past_due (SQL auto-stamps past_due_at on first transition)
	err = uc.accountRepo.UpdateAccountSubscription(ctx, account.ID, account.StripeCustomerID, &subscriptionID, domain.SubscriptionStatusPastDue)
	if err != nil {
		return fmt.Errorf("failed to mark account as past_due: %w", err)
	}
	slog.Warn("payment failed, account marked as past_due", "subscription_id", subscriptionID, "account_id", account.ID)

	// Best-effort: send notification email to account admin
	go uc.notifyPaymentFailed(account.ID)

	return nil
}

// notifyPaymentFailed sends a payment-failure email to the account admin.
// Runs in a background goroutine — failures are logged, never propagated.
func (uc *ProcessStripeWebhookUseCase) notifyPaymentFailed(accountID string) {
	if uc.emailService == nil || uc.userRepo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	admin, err := uc.userRepo.GetAdminByAccountID(ctx, accountID)
	if err != nil || admin == nil {
		slog.Error("payment failed email: could not find admin", "account_id", accountID, "error", err)
		return
	}

	account, _ := uc.accountRepo.GetByID(ctx, accountID)
	locale := ""
	if account != nil {
		locale = domain.EmailLocaleForCurrency(account.Currency)
	}
	planName := "seu plano"
	if locale == "en" {
		planName = "your plan"
	}
	if uc.planRepo != nil && account != nil && account.PlanID != nil {
		if plan, _ := uc.planRepo.GetByID(ctx, *account.PlanID); plan != nil {
			planName = plan.Name
		}
	}

	retryURL := uc.appBaseURL + "/settings?billing=retry"
	if err := uc.emailService.SendPaymentFailedEmail(admin.Email, admin.Name, planName, retryURL, locale); err != nil {
		slog.Error("failed to send payment failed email", "account_id", accountID, "email", admin.Email, "error", err)
	}
}

func (uc *ProcessStripeWebhookUseCase) handleSubscriptionUpdated(ctx context.Context, event stripe.Event) error {
	var subscription stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
		return fmt.Errorf("error parsing subscription: %w", err)
	}

	subID := subscription.ID
	account, err := uc.accountRepo.GetAccountByStripeSubscription(ctx, subID)
	if err != nil || account == nil {
		slog.Warn("received subscription.updated for unknown subscription", "subscription_id", subID)
		return nil
	}

	// Cache the next-charge timestamp so the pre-billing cron (US-124) can
	// email the admin ~72h before renewal. Stripe sends current_period_end
	// as a Unix epoch; a renewal moves it forward, re-arming the reminder.
	if subscription.CurrentPeriodEnd > 0 {
		periodEnd := time.Unix(subscription.CurrentPeriodEnd, 0).UTC()
		if err := uc.accountRepo.UpdateSubscriptionPeriodEnd(ctx, account.ID, periodEnd); err != nil {
			// Non-fatal: status sync below still matters more than the reminder.
			slog.Warn("failed to cache subscription period end", "subscription_id", subID, "account_id", account.ID, "error", err)
		}
	}

	// Map Stripe status to our domain status. We collapse failure-mode
	// statuses Stripe sends onto our smaller domain vocabulary: anything
	// that means "billing is broken but still recoverable" → past_due
	// (the usage guard already gates AI on this); anything terminal that
	// will never charge again → canceled. The previous switch returned
	// early on `incomplete` / `incomplete_expired` / `unpaid` which
	// left the row stuck on the prior status — a subscription whose
	// initial payment failed could remain `active` indefinitely.
	var newStatus string
	switch subscription.Status {
	case stripe.SubscriptionStatusActive:
		newStatus = domain.SubscriptionStatusActive
	case stripe.SubscriptionStatusPastDue:
		newStatus = domain.SubscriptionStatusPastDue
	case stripe.SubscriptionStatusPaused:
		newStatus = domain.SubscriptionStatusPaused
	case stripe.SubscriptionStatusCanceled:
		newStatus = domain.SubscriptionStatusCanceled
	case stripe.SubscriptionStatusTrialing:
		newStatus = domain.SubscriptionStatusTrialing
	case stripe.SubscriptionStatusIncomplete:
		// Initial payment failed but still inside Stripe's recovery
		// window — gate AI access via past_due semantics.
		newStatus = domain.SubscriptionStatusPastDue
	case stripe.SubscriptionStatusIncompleteExpired:
		// Stripe gave up on the initial payment; this is terminal.
		newStatus = domain.SubscriptionStatusCanceled
	case stripe.SubscriptionStatusUnpaid:
		// Retries exhausted on a recurring invoice; Stripe stops
		// charging until manual recovery. Treat as canceled so the
		// usage guard cuts access immediately.
		newStatus = domain.SubscriptionStatusCanceled
	default:
		// Genuinely unknown Stripe status — log and skip the write so
		// we don't poison the column with an unmapped value the usage
		// guard won't recognise.
		slog.Warn("subscription.updated: unmapped Stripe status — ignored",
			"subscription_id", subID, "stripe_status", string(subscription.Status))
		return nil
	}

	if account.SubscriptionStatus != newStatus {
		err = uc.accountRepo.UpdateAccountSubscription(ctx, account.ID, account.StripeCustomerID, &subID, newStatus)
		if err != nil {
			return fmt.Errorf("failed to update subscription status: %w", err)
		}
		slog.Info("subscription status updated", "subscription_id", subID, "account_id", account.ID, "old_status", account.SubscriptionStatus, "new_status", newStatus)
	}

	// Reinforcement of US-001: keep accounts.plan_id in sync with the
	// subscription's line-item price. Covers plan changes made in the
	// Stripe portal and backstops checkout.session.completed if that
	// event's plan activation was missed. Best-effort — the checkout
	// path is the authoritative one and returns errors for retry.
	if newStatus == domain.SubscriptionStatusActive || newStatus == domain.SubscriptionStatusTrialing {
		uc.syncPlanFromSubscriptionItems(ctx, account, &subscription, time.Unix(event.Created, 0).UTC())
	}
	return nil
}

// syncPlanFromSubscriptionItems resolves the plan behind the
// subscription's first line-item price (present in the
// customer.subscription.* payload, unlike checkout.session.completed)
// and writes accounts.plan_id when it diverges. A paid→pricier-paid
// move also ledgers 'plan_upgraded' so the self-serve upgrade path
// (Stripe portal / checkout) feeds the commission upgrade bonus, not
// just the owner-panel event. Best-effort: failures are logged, never
// propagated.
func (uc *ProcessStripeWebhookUseCase) syncPlanFromSubscriptionItems(ctx context.Context, account *domain.Account, subscription *stripe.Subscription, occurredAt time.Time) {
	if uc.planRepo == nil {
		return
	}
	priceID := ""
	if subscription.Items != nil {
		for _, item := range subscription.Items.Data {
			if item != nil && item.Price != nil && item.Price.ID != "" {
				priceID = item.Price.ID
				break
			}
		}
	}
	if priceID == "" {
		return
	}

	plan, err := uc.planRepo.GetByStripePriceID(ctx, priceID)
	if err != nil {
		slog.Error("subscription plan sync: failed to resolve plan by price", "account_id", account.ID, "price_id", priceID, "error", err)
		return
	}
	if plan == nil {
		slog.Error("subscription plan sync: price id maps to no plan — account may be on wrong limits",
			"account_id", account.ID, "subscription_id", subscription.ID, "price_id", priceID)
		return
	}
	if account.PlanID != nil && *account.PlanID == plan.ID {
		return
	}

	uc.recordSelfServePlanUpgrade(ctx, account, plan, occurredAt)

	if err := uc.accountRepo.SetPlan(ctx, account.ID, plan.ID); err != nil {
		slog.Error("subscription plan sync: failed to set plan", "account_id", account.ID, "plan_id", plan.ID, "error", err)
		return
	}
	if err := uc.accountRepo.ClearTrialExpiresAt(ctx, account.ID); err != nil {
		slog.Error("subscription plan sync: failed to clear trial deadline", "account_id", account.ID, "error", err)
	}
	slog.Info("subscription plan synced", "account_id", account.ID, "plan_id", plan.ID, "plan_name", plan.Name, "price_id", priceID)
}

func (uc *ProcessStripeWebhookUseCase) handleSubscriptionDeleted(ctx context.Context, event stripe.Event) error {
	var subscription stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
		return fmt.Errorf("error parsing subscription: %w", err)
	}

	subID := subscription.ID
	account, err := uc.accountRepo.GetAccountByStripeSubscription(ctx, subID)
	if err != nil || account == nil {
		return nil
	}

	err = uc.accountRepo.UpdateAccountSubscription(ctx, account.ID, nil, &subID, domain.SubscriptionStatusCanceled)
	if err != nil {
		return fmt.Errorf("failed to downgrade account subscription status: %w", err)
	}
	slog.Info("successfully canceled subscription", "subscription_id", subID, "account_id", account.ID)

	// Ledger the churn for revenue analytics (US-071). Amount 0 so the
	// account drops out of MRR; the latest-event-per-account rollup keys
	// off the churned type, not the amount.
	uc.recordBillingEvent(ctx, account.ID, domain.BillingEventChurned, uc.planNameFor(ctx, account), 0, "", time.Unix(event.Created, 0).UTC())

	// Win-back (US-179): offer a 50%-off-3-months recovery coupon + drip
	// follow-up reminders to the churned customer. Best-effort and
	// nil-safe; runs in its own goroutine with a fresh context (it mints a
	// Stripe coupon and sends an email) so it never delays the webhook ack
	// — mirrors notifyPaymentFailed. Only paid customers (those with a
	// Stripe customer id) qualify; never-converted trials are handled by
	// the reactivation flow instead.
	if uc.winBack != nil && account.StripeCustomerID != nil && *account.StripeCustomerID != "" {
		go uc.startWinBack(ctx, account.ID, *account.StripeCustomerID)
	}
	return nil
}

// startWinBack kicks off the win-back recovery flow in the background. The
// webhook acks Stripe (cancelling the request context) before the coupon
// mint + email finish, so we detach the cancellation with WithoutCancel
// while keeping the context's values, then bound the work to 30s.
// Best-effort — the use case logs its own failures.
func (uc *ProcessStripeWebhookUseCase) startWinBack(ctx context.Context, accountID, customerID string) {
	bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	uc.winBack.Start(bg, accountID, customerID)
}

// recordBillingEvent appends a row to the revenue ledger. Best-effort:
// missing wiring and errors are logged, never propagated, so analytics
// can never break the primary webhook side effects.
func (uc *ProcessStripeWebhookUseCase) recordBillingEvent(ctx context.Context, accountID, eventType, plan string, amountCents int64, currency string, occurredAt time.Time) {
	if uc.billingEvents == nil || accountID == "" {
		return
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	if err := uc.billingEvents.Create(ctx, &domain.BillingEvent{
		ID:          ulid.Make().String(),
		AccountID:   accountID,
		EventType:   eventType,
		Plan:        plan,
		AmountCents: amountCents,
		Currency:    currency,
		OccurredAt:  occurredAt,
	}); err != nil {
		slog.Error("failed to record billing event", "account_id", accountID, "event_type", eventType, "error", err)
	}
}

// planNameFor resolves the account's current plan name for the ledger
// snapshot. Returns "" when the plan repo is unwired or the lookup misses.
func (uc *ProcessStripeWebhookUseCase) planNameFor(ctx context.Context, account *domain.Account) string {
	if uc.planRepo == nil || account == nil || account.PlanID == nil {
		return ""
	}
	plan, err := uc.planRepo.GetByID(ctx, *account.PlanID)
	if err != nil || plan == nil {
		return ""
	}
	return plan.Name
}

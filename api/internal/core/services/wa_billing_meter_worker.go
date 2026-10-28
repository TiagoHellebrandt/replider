package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/usagerecord"

	"github.com/replider/api/internal/core/domain"
)

// WABillingMeterWorker rolls up WhatsApp pricing-category usage and
// pushes it to Stripe as metered subscription_item usage_records.
//
// Pipeline:
//  1. Aggregate: pull every (account, conversation, category) seen
//     in the current billing window into wa_billing_meter
//     (idempotent — ON CONFLICT DO NOTHING).
//  2. ListPending: read un-metered rollups joined with the account's
//     plan to get Stripe subscription_item IDs.
//  3. PushUsage: for each rollup with a non-empty subscription_item,
//     POST usage_records.create with the billable count.
//  4. MarkPushed: stamp metered_at so the next tick doesn't double-bill.
//
// Stripe push is no-op when stripeEnabled=false (dev / test) or when
// the rollup's plan has no item ID for the category — those flow as
// flat-rate billing absorbed by the plan's monthly fee.
type WABillingMeterWorker struct {
	repo          domain.WABillingMeterRepository
	pollInterval  time.Duration
	batchSize     int
	stripeEnabled bool
	running       atomic.Bool
}

func NewWABillingMeterWorker(repo domain.WABillingMeterRepository, stripeKey string) *WABillingMeterWorker {
	if stripeKey != "" {
		stripe.Key = stripeKey
	}
	return &WABillingMeterWorker{
		repo:          repo,
		pollInterval:  6 * time.Hour, // 4 ticks per day; granular enough for monthly billing
		batchSize:     500,
		stripeEnabled: stripeKey != "",
	}
}

func (w *WABillingMeterWorker) WithPollInterval(d time.Duration) *WABillingMeterWorker {
	w.pollInterval = d
	return w
}

func (w *WABillingMeterWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("wa billing meter worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("wa billing meter worker started",
		"poll_interval", w.pollInterval,
		"stripe_enabled", w.stripeEnabled)

	w.tick(ctx)
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("wa billing meter worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *WABillingMeterWorker) tick(ctx context.Context) {
	// 1. Aggregate the current month into wa_billing_meter.
	now := time.Now().UTC()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, 0)
	if err := w.repo.Aggregate(ctx, periodStart, periodEnd); err != nil {
		slog.Warn("wa billing meter: aggregate failed", "error", err)
		return
	}

	// 2. Read pending rollups.
	rollups, err := w.repo.ListPending(ctx, w.batchSize)
	if err != nil {
		slog.Warn("wa billing meter: list pending failed", "error", err)
		return
	}
	if len(rollups) == 0 {
		return
	}
	slog.Info("wa billing meter: rollups ready", "count", len(rollups))

	// 3. Push each rollup that has a Stripe item ID. Empty IDs are
	//    legitimately absorbed by the plan's flat fee.
	for _, r := range rollups {
		itemID := r.SubscriptionItemID()
		if itemID == "" {
			// Plan doesn't meter this category — still mark pushed
			// so we don't keep re-listing. The flat-rate fee covers
			// it.
			if _, err := w.repo.MarkPushed(ctx, r.AccountID, r.PricingCategory, r.BillingPeriod); err != nil {
				slog.Warn("wa billing meter: mark pushed (flat) failed",
					"account_id", r.AccountID, "category", r.PricingCategory, "error", err)
			}
			continue
		}
		if !w.stripeEnabled {
			slog.Info("wa billing meter: would push to Stripe (disabled)",
				"account_id", r.AccountID, "category", r.PricingCategory,
				"quantity", r.BillableCount, "item_id", itemID)
			continue
		}
		if err := w.pushUsageRecord(ctx, itemID, int64(r.BillableCount)); err != nil {
			slog.Warn("wa billing meter: stripe push failed",
				"account_id", r.AccountID, "category", r.PricingCategory,
				"item_id", itemID, "error", err)
			continue
		}
		if _, err := w.repo.MarkPushed(ctx, r.AccountID, r.PricingCategory, r.BillingPeriod); err != nil {
			slog.Warn("wa billing meter: mark pushed failed",
				"account_id", r.AccountID, "category", r.PricingCategory, "error", err)
			continue
		}
		slog.Info("wa billing meter: pushed to Stripe",
			"account_id", r.AccountID, "category", r.PricingCategory,
			"quantity", r.BillableCount, "item_id", itemID)
	}
}

// pushUsageRecord posts a single usage_records.create to Stripe with
// action=set so the value is the cumulative count for the period (not
// an increment). action=set is idempotent across worker re-runs:
// pushing the same quantity twice is a no-op rather than double-bill.
func (w *WABillingMeterWorker) pushUsageRecord(ctx context.Context, subscriptionItemID string, quantity int64) error {
	params := &stripe.UsageRecordParams{
		SubscriptionItem: stripe.String(subscriptionItemID),
		Quantity:         stripe.Int64(quantity),
		Action:           stripe.String(string(stripe.UsageRecordActionSet)),
		Timestamp:        stripe.Int64(time.Now().Unix()),
	}
	params.Context = ctx
	_, err := usagerecord.New(params)
	return err
}

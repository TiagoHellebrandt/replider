package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// ReorderWorker offers a repurchase before the customer runs out (§6.5, F3).
//
// This is where a health distributor makes money. Consumables are recurring
// revenue: a customer whose repurchase works does not churn, and one who has
// to remember to reorder eventually remembers a competitor instead.
//
// It puts a draft in the APPROVAL QUEUE and stops there. Nothing is sent. A
// machine that decides on its own to ask a hospital to buy more gloves is one
// wrong prediction away from a customer who stops trusting anything the
// system says — and the queue already exists precisely for this shape of
// decision.
type ReorderWorker struct {
	portfolio ReorderSource
	drafter   ReorderDrafter
	interval  time.Duration
	// leadTime is how far ahead of the predicted run-out an approach goes
	// out. See domain.ReorderLeadTimeDays.
	leadTime time.Duration
	batch    int
	now      func() time.Time
	// lifecycle emits reorder.predicted (J2). Optional: nil queues the
	// suggestion exactly as before.
	lifecycle LifecycleNotifier
	// runLog stamps completed passes so a boot decides honestly whether the
	// 6h tick was missed (daily deploys). Optional — see WithRunLog.
	runLog domain.WorkerRunRepository
}

// reorderWorkerName keys the worker_runs stamp.
const reorderWorkerName = "reorder"

// WithRunLog wires the worker_runs stamp (boot pass when the last completed
// pass is older than the interval).
func (w *ReorderWorker) WithRunLog(r domain.WorkerRunRepository) *ReorderWorker {
	w.runLog = r
	return w
}

// LifecycleNotifier is the slice of the outbound dispatcher this worker needs.
//
// Declared here rather than imported from the use cases: a service must not
// depend on a use case, and the port is one method.
type LifecycleNotifier interface {
	Emit(accountID, event string, payload any)
}

// WithLifecycleEmitter wires the reorder.predicted webhook (J2).
func (w *ReorderWorker) WithLifecycleEmitter(e LifecycleNotifier) *ReorderWorker {
	w.lifecycle = e
	return w
}

// ReorderSource reads the predictions and remembers what was offered.
type ReorderSource interface {
	DueForReorder(ctx context.Context, within time.Time, limit int) ([]*domain.ReorderCandidate, error)
	RecordNudge(ctx context.Context, id, accountID, contactID, saleID, productKey string, dueAt time.Time) error
}

// ReorderDrafter puts one repurchase suggestion into the approval queue.
//
// A narrow port: the worker knows nothing about funnels, AI providers or
// message templates, and the queue is the only thing it is allowed to touch.
type ReorderDrafter interface {
	QueueReorderApproach(ctx context.Context, accountID, contactID, text string) error
}

func NewReorderWorker(portfolio ReorderSource, drafter ReorderDrafter) *ReorderWorker {
	return &ReorderWorker{
		portfolio: portfolio, drafter: drafter,
		interval: 6 * time.Hour,
		leadTime: domain.ReorderLeadTimeDays * 24 * time.Hour,
		batch:    100,
		now:      time.Now,
	}
}

func (w *ReorderWorker) WithInterval(d time.Duration) *ReorderWorker {
	w.interval = d
	return w
}

func (w *ReorderWorker) WithClock(now func() time.Time) *ReorderWorker {
	w.now = now
	return w
}

// Start runs the loop until the context is cancelled.
func (w *ReorderWorker) Start(ctx context.Context) {
	if w.portfolio == nil || w.drafter == nil {
		return
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// No unconditional immediate pass, for the same reason as the autopilot
	// worker: a deploy loop that restarts three times in a minute must not
	// become three rounds of suggestions. But a missed tick costs six hours
	// on a seven-day window, so the boot pass runs when the stamped last
	// pass is older than the interval (or there is none).
	if bootPassDue(ctx, w.runLog, reorderWorkerName, w.interval, w.now()) {
		w.RunOnce(ctx)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.RunOnce(ctx)
		}
	}
}

// RunOnce does one pass.
func (w *ReorderWorker) RunOnce(ctx context.Context) {
	if w.portfolio == nil || w.drafter == nil {
		return
	}
	// Everything predicted to run out within the lead time, INCLUDING what
	// is already overdue — a customer who ran out last week is the most
	// expensive state in the model, not one to skip because the window
	// passed.
	horizon := w.now().Add(w.leadTime)

	candidates, err := w.portfolio.DueForReorder(ctx, horizon, w.batch)
	if err != nil {
		slog.ErrorContext(ctx, "reorder: could not read the predictions", "error", err)
		return
	}

	var queued int
	for _, c := range candidates {
		text := ReorderMessage(c, w.now())
		if err := w.drafter.QueueReorderApproach(ctx, c.AccountID, c.ContactID, text); err != nil {
			// One customer failing is a fact about that customer — a missing
			// funnel, an opted-out contact — and must not cost the batch.
			slog.WarnContext(ctx, "reorder: could not queue a suggestion",
				"account_id", c.AccountID, "contact_id", c.ContactID, "error", err)
			continue
		}
		// Recorded AFTER the queue accepted it: a nudge marked as sent that
		// never reached the queue is a customer nobody ever asks.
		if err := w.portfolio.RecordNudge(ctx, ulid.Make().String(),
			c.AccountID, c.ContactID, c.SaleID, c.ProductKey, c.DueAt); err != nil {
			slog.WarnContext(ctx, "reorder: queued but could not mark it — the customer may be asked twice",
				"account_id", c.AccountID, "sale_id", c.SaleID, "error", err)
		}
		// J2: emitted AFTER the nudge is recorded, so an integration hears about
		// a prediction the product itself has committed to — not one it might
		// re-queue on the next tick.
		if w.lifecycle != nil {
			w.lifecycle.Emit(c.AccountID, domain.EventReorderPredicted, domain.LifecyclePayload{
				ID:         c.SaleID,
				ContactID:  c.ContactID,
				Confirmed:  false,
				OccurredAt: c.DueAt.UTC().Format(time.RFC3339),
			})
		}
		queued++
	}
	if queued > 0 {
		slog.InfoContext(ctx, "reorder: repurchase suggestions queued for approval",
			"count", queued)
	}
	stampRun(ctx, w.runLog, reorderWorkerName, w.now())
}

// ReorderMessage writes the draft a human will review.
//
// Deliberately not generated by a model. It is the same three facts every
// time — what they bought, when, and that they are about to run out — and a
// model would spend a call to rephrase them into something less specific.
// The person reviewing it edits anything they want.
func ReorderMessage(c *domain.ReorderCandidate, now time.Time) string {
	what := reorderItems(c)
	days := int(c.DueAt.Sub(now).Hours() / 24)

	var b strings.Builder
	b.WriteString("Olá! ")
	switch {
	case days < 0:
		// Overdue: say so plainly rather than pretending to have noticed on
		// time. "Deve ter acabado" is honest and invites a correction.
		b.WriteString(fmt.Sprintf(
			"Pelas nossas contas o estoque de %s deve ter acabado por aí. ", what))
	case days == 0:
		b.WriteString(fmt.Sprintf("Pelas nossas contas %s está acabando aí. ", what))
	default:
		b.WriteString(fmt.Sprintf(
			"Pelas nossas contas %s deve acabar em cerca de %d dias. ", what, days))
	}
	b.WriteString("Quer que eu já separe a mesma quantidade da última compra?")
	return b.String()
}

// reorderItems names what they bought, or falls back to something true.
func reorderItems(c *domain.ReorderCandidate) string {
	if c == nil || len(c.Items) == 0 {
		// No item list: "o material" is vague, and vague is better than
		// inventing a product name a customer never ordered.
		return "o material da última compra"
	}
	first := strings.TrimSpace(c.Items[0].Description)
	if first == "" {
		return "o material da última compra"
	}
	if len(c.Items) == 1 {
		return first
	}
	// One name plus a count. Listing six items turns a short message into a
	// packing slip nobody reads.
	return fmt.Sprintf("%s (e mais %d itens)", first, len(c.Items)-1)
}

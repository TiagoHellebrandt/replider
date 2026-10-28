package services

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// OverageCharger abstracts the Stripe call so the settler is testable
// without a live Stripe key. *payment.StripeAdapter satisfies it.
type OverageCharger interface {
	CreateOverageInvoiceItem(customerID string, amountCents int64, currency, description, idempotencyKey string) error
}

// stripeIdempotencyWindow is how long Stripe retains an idempotency
// key. A journalled-but-unconfirmed charge older than this can no
// longer be retried safely — Stripe would treat the retry as a new
// request and double-bill — so the settler escalates it for review.
const stripeIdempotencyWindow = 24 * time.Hour

// OverageSettler bills conversation overage once a plan franchise is
// exhausted. The overage_charges ledger makes the bill-then-record
// step crash-safe (the C2 fix in docs/OVERAGE_METERING_DESIGN.md):
//
//   - every charge is journalled `pending` before the Stripe call and
//     flipped to `charged` after;
//   - the Stripe idempotency key IS the ledger row id;
//   - the cycle-to-date total is summed from the ledger, not a counter
//     a crash could leave stale — so a crash anywhere in the sequence
//     can never produce a duplicate charge;
//   - a pending row older than the idempotency window is escalated to
//     `needs_review` rather than retried into a possible double bill.
type OverageSettler struct {
	usageRepo     domain.UsageRepository
	chargeRepo    domain.OverageChargeRepository
	charger       OverageCharger
	stripeEnabled bool
	now           func() time.Time
}

func NewOverageSettler(
	usageRepo domain.UsageRepository,
	chargeRepo domain.OverageChargeRepository,
	charger OverageCharger,
	stripeEnabled bool,
) *OverageSettler {
	return &OverageSettler{
		usageRepo:     usageRepo,
		chargeRepo:    chargeRepo,
		charger:       charger,
		stripeEnabled: stripeEnabled,
		now:           time.Now,
	}
}

// WithClock overrides the time source (used by tests).
func (s *OverageSettler) WithClock(now func() time.Time) *OverageSettler {
	if now != nil {
		s.now = now
	}
	return s
}

// CloseCycle archives the account's charged ledger rows so the next
// cycle's charged total starts back at zero. Called on invoice.paid
// after the final Settle.
func (s *OverageSettler) CloseCycle(ctx context.Context, accountID string) error {
	return s.chargeRepo.ArchiveCharged(ctx, accountID)
}

// Settle bills the cycle-to-date overage delta for one account. It
// returns an error only when a step fails in a way the caller should
// retry; an account under its franchise, or with nothing new to bill,
// is a no-op returning nil.
func (s *OverageSettler) Settle(ctx context.Context, c domain.OverageCandidate) error {
	// 1. Crash recovery — resolve any in-flight charge first.
	pending, err := s.chargeRepo.GetPending(ctx, c.AccountID)
	if err != nil {
		return fmt.Errorf("load pending overage charge for %s: %w", c.AccountID, err)
	}
	if pending != nil {
		if s.now().Sub(pending.CreatedAt) > stripeIdempotencyWindow {
			// Too old to retry safely — Stripe has forgotten the
			// idempotency key, so a retry could double-bill. Escalate
			// and stop; a human reconciles against Stripe before
			// overage charging resumes for this account.
			if err := s.chargeRepo.MarkNeedsReview(ctx, pending.ID); err != nil {
				return fmt.Errorf("escalate stale overage charge %s: %w", pending.ID, err)
			}
			slog.Error("overage charge stuck past the idempotency window — manual review needed",
				"account_id", c.AccountID, "charge_id", pending.ID,
				"amount_cents", pending.AmountCents, "created_at", pending.CreatedAt)
			return nil
		}
		// Young enough — complete it. Re-sending to Stripe with the
		// same key (the row id) dedupes within the window.
		if err := s.completeCharge(ctx, c, pending); err != nil {
			return err
		}
		// Fall through: the just-charged row now counts in SumCharged.
	}

	// 2. Compute the fresh delta. overageCents is the running total;
	//    SumCharged (the ledger) is the crash-safe "already billed".
	overageConvs := int(c.ConversationsCount) - c.MaxConversationsPerMonth
	if overageConvs <= 0 {
		return nil
	}
	overageCents := overageConvs * c.OverageConversationCents
	capCents := int(math.Round(float64(c.PlanPriceCents) * c.OverageCapMultiplier))
	capped := false
	if capCents > 0 && overageCents >= capCents {
		overageCents = capCents
		capped = true
	}

	chargedSoFar, err := s.chargeRepo.SumCharged(ctx, c.AccountID)
	if err != nil {
		return fmt.Errorf("sum charged overage for %s: %w", c.AccountID, err)
	}
	delta := overageCents - int(chargedSoFar)
	if delta <= 0 {
		// Nothing new to bill. Stamp the cap marker once if the
		// ceiling was just reached, for the dashboard.
		if capped && !c.CapReached {
			if err := s.usageRepo.RecordOverageCharge(ctx, c.AccountID, int32(overageConvs), chargedSoFar, true); err != nil {
				return fmt.Errorf("record cap marker for %s: %w", c.AccountID, err)
			}
		}
		return nil
	}

	// 3. Journal the charge `pending`, then bill Stripe, then confirm.
	charge := &domain.OverageCharge{
		ID:                        ulid.Make().String(),
		AccountID:                 c.AccountID,
		AmountCents:               int32(delta),
		RunningTotalCents:         int32(overageCents),
		OverageConversationsCount: int32(overageConvs),
		Capped:                    capped,
	}
	if err := s.chargeRepo.Create(ctx, charge); err != nil {
		return fmt.Errorf("journal overage charge for %s: %w", c.AccountID, err)
	}
	return s.completeCharge(ctx, c, charge)
}

// completeCharge sends one journalled charge to Stripe (idempotency key
// = the ledger row id) and, on success, flips the row to `charged` and
// refreshes the account_usages display cache.
func (s *OverageSettler) completeCharge(ctx context.Context, c domain.OverageCandidate, charge *domain.OverageCharge) error {
	if s.stripeEnabled {
		// The invoice item is billed in the account's currency (US-003).
		// Accounts predating the currency column read back 'brl' (column
		// default), so this is always non-empty.
		currency := c.Currency
		if currency == "" {
			currency = domain.CurrencyBRL
		}
		symbol := "R$"
		if currency == domain.CurrencyUSD {
			symbol = "$"
		}
		desc := fmt.Sprintf("Conversas excedentes — %d × %s %.2f",
			charge.OverageConversationsCount, symbol, float64(c.OverageConversationCents)/100)
		if err := s.charger.CreateOverageInvoiceItem(
			c.StripeCustomerID, int64(charge.AmountCents), currency, desc, charge.ID,
		); err != nil {
			// Row stays `pending`; the next tick resumes it and Stripe
			// dedupes the retry on the same key within the window.
			return fmt.Errorf("stripe invoice item for %s: %w", c.AccountID, err)
		}
	} else {
		slog.Info("overage settle: would bill (stripe disabled)",
			"account_id", c.AccountID, "charge_id", charge.ID, "amount_cents", charge.AmountCents)
	}

	if err := s.chargeRepo.MarkCharged(ctx, charge.ID); err != nil {
		return fmt.Errorf("mark overage charge %s charged: %w", charge.ID, err)
	}
	// Display cache only — the ledger is authoritative, so a failure
	// here cannot cause a double charge, just a momentarily stale
	// dashboard. Log and move on.
	if err := s.usageRepo.RecordOverageCharge(ctx, c.AccountID,
		charge.OverageConversationsCount, charge.RunningTotalCents, charge.Capped); err != nil {
		slog.Warn("overage settle: display cache update failed (ledger is authoritative)",
			"account_id", c.AccountID, "error", err)
	}
	slog.Info("overage charge settled",
		"account_id", c.AccountID, "charge_id", charge.ID,
		"amount_cents", charge.AmountCents, "running_total_cents", charge.RunningTotalCents,
		"capped", charge.Capped)
	return nil
}

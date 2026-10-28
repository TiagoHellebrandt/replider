package payment

import (
	"testing"
	"time"

	"github.com/stripe/stripe-go/v76"
)

// The paid-invoice lookback must not bound created-at from below:
// Stripe's list filter only supports created-at, and dunning can
// recover an invoice months after creation, so any lower bound
// silently drops old invoices paid inside the cycle (US-007 review
// fix — the old code cut at created >= cycle-3 months). The [from, to)
// window is enforced client-side on status_transitions.paid_at.
func TestPaidInvoiceListParams_NoCreatedLowerBound(t *testing.T) {
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)

	params := paidInvoiceListParams("cus_123", to)

	if params.CreatedRange == nil {
		t.Fatal("expected a created range with an upper bound")
	}
	if params.CreatedRange.GreaterThanOrEqual != 0 || params.CreatedRange.GreaterThan != 0 {
		t.Fatalf("created must have NO lower bound (an invoice can be paid any time after creation), got >=%d >%d",
			params.CreatedRange.GreaterThanOrEqual, params.CreatedRange.GreaterThan)
	}
	if params.CreatedRange.LesserThan != to.Unix() {
		t.Fatalf("created upper bound must be the window end %d, got %d", to.Unix(), params.CreatedRange.LesserThan)
	}
	if params.Customer == nil || *params.Customer != "cus_123" {
		t.Fatalf("unexpected customer filter: %v", params.Customer)
	}
	if params.Status == nil || *params.Status != string(stripe.InvoiceStatusPaid) {
		t.Fatalf("unexpected status filter: %v", params.Status)
	}
	// The fee comes from the charge's balance_transaction, expanded on
	// the list call — and the charge itself carries amount_refunded.
	found := false
	for _, e := range params.Expand {
		if e != nil && *e == "data.charge.balance_transaction" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected data.charge.balance_transaction expand")
	}
}

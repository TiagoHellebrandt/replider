package domain

import (
	"context"
	"time"
)

// Portfolio and repurchase (§6.5, F2/F3).
//
// This is where a health distributor makes money. Consumables are recurring
// revenue: a customer whose repurchase works does not churn, and a customer
// who has to remember to reorder eventually remembers a competitor instead.
//
// The whole feature is one number — when they will run out — and one
// consequence: somebody hears from us a few days before, not a few weeks
// after.

// PortfolioCustomer is one customer as the portfolio sees them.
type PortfolioCustomer struct {
	ContactID   string `json:"contact_id"`
	ContactName string `json:"contact_name"`

	// Purchases is how many times they have bought. A repeat customer and a
	// one-off look identical on revenue alone.
	Purchases int `json:"purchases"`
	// GrossCents and NetCents are lifetime. Net leads in the UI, because
	// what a customer is worth is what the company kept.
	GrossCents int64 `json:"gross_cents"`
	NetCents   int64 `json:"net_cents"`

	LastPurchaseAt *time.Time `json:"last_purchase_at,omitempty"`
	// NextDueAt is when they are predicted to need it again. Nil when no
	// sale carried a cycle — a guessed date puts a real customer on a
	// follow-up list on a day nobody chose.
	NextDueAt *time.Time `json:"next_due_at,omitempty"`

	// LastContactAt is the last message either way. The relationship's
	// pulse: a customer nobody has spoken to in four months is a customer a
	// competitor is speaking to.
	LastContactAt *time.Time `json:"last_contact_at,omitempty"`
}

// DaysUntilDue is the countdown the card shows. Nil without a prediction.
// Negative means overdue — which is the case that matters most and is
// therefore NOT clamped to zero.
func (c *PortfolioCustomer) DaysUntilDue(now time.Time) *int {
	if c == nil || c.NextDueAt == nil {
		return nil
	}
	d := int(c.NextDueAt.Sub(now).Hours() / 24)
	return &d
}

// DaysSinceContact is the relationship health number. Nil when nobody has
// ever spoken to them, which is a different thing from "spoke long ago".
func (c *PortfolioCustomer) DaysSinceContact(now time.Time) *int {
	if c == nil || c.LastContactAt == nil {
		return nil
	}
	d := int(now.Sub(*c.LastContactAt).Hours() / 24)
	return &d
}

// PortfolioUrgency ranks the list.
//
// A portfolio sorted alphabetically or by revenue is a portfolio nobody works
// through: the person opening it has an hour, and the question is who to call
// FIRST. Higher sorts first.
func (c *PortfolioCustomer) PortfolioUrgency(now time.Time) int {
	if c == nil {
		return 0
	}
	score := 0

	if days := c.DaysUntilDue(now); days != nil {
		switch {
		case *days < 0:
			// Already overdue. The most expensive state in the whole model:
			// they have run out, and somebody is selling to them today.
			score += 100 + min(-*days, 60)
		case *days <= ReorderLeadTimeDays:
			// Inside the window where an approach still arrives before they
			// run out. Sooner is more urgent.
			score += 60 + (ReorderLeadTimeDays - *days)
		default:
			// Known but not yet. Worth being in the list, not worth a call.
			score += 10
		}
	}

	// Silence is its own risk, independent of any reorder date.
	if days := c.DaysSinceContact(now); days != nil && *days >= StaleRelationshipDays {
		score += 20
	}
	return score
}

// ReorderLeadTimeDays is how far ahead an approach goes out.
//
// Seven days: long enough that a purchasing department can raise an order and
// a delivery can arrive, short enough that "vocês vão precisar em breve" is
// still true rather than a guess about next month.
const ReorderLeadTimeDays = 7

// StaleRelationshipDays is when silence becomes a risk worth surfacing.
//
// Sixty days. Shorter would flag every customer on a quarterly cycle as
// neglected, which trains people to ignore the flag.
const StaleRelationshipDays = 60

// PortfolioRepository reads the customer view of sales.
type PortfolioRepository interface {
	// List returns customers who have bought, most urgent first.
	List(ctx context.Context, accountID string, limit, offset int) ([]*PortfolioCustomer, error)
	Get(ctx context.Context, accountID, contactID string) (*PortfolioCustomer, error)
	// DueForReorder lists customers predicted to run out within the window,
	// across accounts, for the worker.
	DueForReorder(ctx context.Context, within time.Time, limit int) ([]*ReorderCandidate, error)
}

// ReorderCandidate is one customer about to run out, with what they bought.
type ReorderCandidate struct {
	AccountID   string `json:"-"`
	ContactID   string `json:"contact_id"`
	ContactName string `json:"contact_name"`
	SaleID      string `json:"sale_id"`
	// ProductKey identifies WHICH product of that sale this prediction is
	// about (product_id, else the normalised description) — a sale with two
	// cycled products yields two candidates and two nudges.
	ProductKey string     `json:"product_key,omitempty"`
	Items      []SaleItem `json:"items,omitempty"`
	LastSoldAt time.Time  `json:"last_sold_at"`
	DueAt      time.Time  `json:"due_at"`
	// GrossCents of the last order, so an approach can offer the same
	// quantity rather than asking what they want.
	GrossCents int64 `json:"gross_cents"`
}

// ConsumptionRate estimates how fast a customer goes through what they
// bought, in units per day.
//
// Version one on purpose: last quantity divided by the cycle. It is wrong for
// a customer whose usage changed, and it is right for the overwhelming
// majority who buy the same thing at the same interval — and it needs no
// history, so it works on the SECOND purchase rather than the fifth.
//
// Returns nil when either input is missing, because the alternative is a rate
// derived from a guess.
func ConsumptionRate(quantity, cycleDays int) *float64 {
	if quantity <= 0 || cycleDays <= 0 {
		return nil
	}
	r := float64(quantity) / float64(cycleDays)
	return &r
}

// PredictRunOut estimates when a quantity bought on a date runs out.
//
// Uses the CYCLE when the sale carries one, falling back to the product's own
// consumption cycle. Nil when neither is known: a prediction with no basis is
// worse than none, because somebody acts on it.
func PredictRunOut(soldAt time.Time, saleCycleDays, productCycleDays int) *time.Time {
	days := saleCycleDays
	if days <= 0 {
		days = productCycleDays
	}
	if days <= 0 {
		return nil
	}
	out := soldAt.AddDate(0, 0, days)
	return &out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

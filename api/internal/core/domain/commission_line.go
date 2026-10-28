package domain

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Commission entries and the monthly statement (§6.6, G2).
//
// One entry is one line somebody can point at: this sale, this rule, this
// base, this rate, this amount. The statement is those lines added up for one
// month and one salesperson.
//
// The hard requirement is not the arithmetic — it is that a PAID line never
// changes. Money that left the company is history; if editing a rule silently
// rewrote last month's payout, every number on the screen would stop being
// evidence. So a recomputation touches pending lines only, and a change that
// lands after payment becomes a visible adjustment on the current month.

// CommissionLineKind separates what produced a line, because they answer
// different questions and are corrected differently.
type CommissionLineKind string

const (
	// LineCommission is a percentage of a sale.
	LineCommission CommissionLineKind = "commission"
	// LineMilestone is the fixed bonus for reaching a target. It belongs to
	// the PERIOD, not to any single sale, so it carries no sale id.
	LineMilestone CommissionLineKind = "milestone"
	// LineAdjustment is a human correction, or the delta produced when a
	// rule changed after a month was paid. Negative allowed: a clawback is a
	// real event, and hiding it would mean the total no longer matches what
	// was actually transferred.
	LineAdjustment CommissionLineKind = "adjustment"
)

// CommissionLineStatus is the lifecycle. Two states on purpose: a line is
// either still arguable or it is settled money.
type CommissionLineStatus string

const (
	CommissionPending CommissionLineStatus = "pending"
	CommissionPaid    CommissionLineStatus = "paid"
)

// CommissionLine is one line of one statement.
type CommissionLine struct {
	ID        string `json:"id"`
	AccountID string `json:"-"`

	// Cycle is the month the line belongs to, "YYYY-MM", resolved in the
	// ACCOUNT's timezone — a sale closed at 21:00 on the 31st belongs to the
	// month the salesperson thinks it does.
	Cycle string `json:"cycle"`

	// SellerUserID is who gets paid. Never empty: a line nobody is owed is a
	// line that should not exist.
	SellerUserID string `json:"seller_user_id"`
	// SellerName is filled on read for display. Not persisted — a name is
	// somebody's current name, not a fact about a payout.
	SellerName string `json:"seller_name,omitempty"`

	Kind CommissionLineKind `json:"kind"`

	// SaleID is empty on a milestone and on a manual adjustment.
	SaleID string `json:"sale_id,omitempty"`
	// RuleID is which rule produced this. Kept even after the rule is
	// deleted, because "why was I paid this?" outlives the rule.
	RuleID string `json:"rule_id,omitempty"`
	// Role is the split role this line pays ("closer", "quem indicou"), empty
	// when the whole commission goes to the sale's own salesperson.
	Role string `json:"role,omitempty"`

	BaseCents   int64   `json:"base_cents"`
	RatePercent float64 `json:"rate_percent"`
	AmountCents int64   `json:"amount_cents"`

	// Explanation is the sentence the engine wrote when it calculated this.
	// Stored rather than recomputed: the rule may have changed since, and the
	// answer to "why this number" has to be the reason it was ACTUALLY paid.
	Explanation string `json:"explanation,omitempty"`
	// Note is what a human typed when adjusting or paying.
	Note string `json:"note,omitempty"`

	Status CommissionLineStatus `json:"status"`
	PaidAt *time.Time           `json:"paid_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CommissionStatement is one salesperson's month.
type CommissionStatement struct {
	Cycle        string `json:"cycle"`
	SellerUserID string `json:"seller_user_id"`
	SellerName   string `json:"seller_name,omitempty"`

	// SalesCents and SalesCount are what the commission was earned ON, so the
	// statement can be checked against the invoices without a second screen.
	SalesCents int64 `json:"sales_cents"`
	SalesCount int   `json:"sales_count"`

	CommissionCents int64 `json:"commission_cents"`
	MilestoneCents  int64 `json:"milestone_cents"`
	AdjustmentCents int64 `json:"adjustment_cents"`
	// TotalCents is everything on the statement, paid and pending.
	TotalCents int64 `json:"total_cents"`
	// PendingCents is what is still owed — the number the admin is about to
	// transfer, and the only one that changes when they mark the month paid.
	PendingCents int64 `json:"pending_cents"`
	PaidCents    int64 `json:"paid_cents"`

	Entries []*CommissionLine `json:"entries"`

	// Warnings are the lines somebody should look at before paying: a sale
	// that lost money, a commission with no rule, a negative total.
	Warnings []string `json:"warnings,omitempty"`
}

// Recompute-time totals used by the engine to reach a tier or a milestone.
func (s *CommissionStatement) recount() {
	s.CommissionCents, s.MilestoneCents, s.AdjustmentCents = 0, 0, 0
	s.TotalCents, s.PendingCents, s.PaidCents = 0, 0, 0
	for _, e := range s.Entries {
		switch e.Kind {
		case LineMilestone:
			s.MilestoneCents += e.AmountCents
		case LineAdjustment:
			s.AdjustmentCents += e.AmountCents
		default:
			s.CommissionCents += e.AmountCents
		}
		s.TotalCents += e.AmountCents
		if e.Status == CommissionPaid {
			s.PaidCents += e.AmountCents
		} else {
			s.PendingCents += e.AmountCents
		}
	}
}

// BuildStatement turns a month's entries into the statement, keeping the
// entries in the order the repository returned them.
func BuildStatement(cycle, sellerUserID string, entries []*CommissionLine) *CommissionStatement {
	st := &CommissionStatement{
		Cycle: cycle, SellerUserID: sellerUserID,
		Entries: entries,
	}
	for _, e := range entries {
		if st.SellerName == "" {
			st.SellerName = e.SellerName
		}
	}
	st.recount()
	return st
}

// CycleOf is the "YYYY-MM" a moment belongs to, in a timezone.
//
// The account's own timezone, not the server's: this is the customer's payroll
// and their month has to end when their calendar says it does. (The platform's
// OWN seller ledger deliberately stays on a fixed zone — that one is our
// payroll, not theirs.)
func CycleOf(t time.Time, tz string) string {
	local := t.In(LoadLocation(tz))
	return fmt.Sprintf("%04d-%02d", local.Year(), int(local.Month()))
}

// CycleRange is the half-open [from, to) interval of a cycle in a timezone.
func CycleRange(cycle, tz string) (time.Time, time.Time, error) {
	loc := LoadLocation(tz)
	start, err := time.ParseInLocation("2006-01", strings.TrimSpace(cycle), loc)
	if err != nil {
		return time.Time{}, time.Time{}, &AppError{
			Code: "COMMISSION_CYCLE_INVALID", Message: "ciclo inválido, use AAAA-MM"}
	}
	return start, start.AddDate(0, 1, 0), nil
}

// IsCycleOpen reports a cycle that has not ended yet.
//
// Paying an open month is legitimate — plenty of companies advance commission
// — but it must be a deliberate act, because sales landing after the transfer
// will produce lines nobody has budgeted for.
func IsCycleOpen(cycle, tz string, now time.Time) bool {
	_, end, err := CycleRange(cycle, tz)
	if err != nil {
		return false
	}
	return now.Before(end)
}

// CommissionLineRepository persists the lines.
type CommissionLineRepository interface {
	// ReplacePendingForCycle writes the recomputed pending lines for one
	// cycle, removing the pending lines it supersedes and NEVER touching a
	// paid one. Atomic: a statement half-recomputed is a statement that pays
	// the wrong amount.
	ReplacePendingForCycle(ctx context.Context, accountID, cycle string, entries []*CommissionLine) error
	// Insert adds a single line (an adjustment).
	Insert(ctx context.Context, e *CommissionLine) error
	// ListByCycle returns every line of a cycle, optionally for one seller.
	ListByCycle(ctx context.Context, accountID, cycle, sellerUserID string) ([]*CommissionLine, error)
	// SumPaidByCycle is what has already been transferred per seller, keyed by
	// seller id — the guard that stops a recomputation rewriting history.
	SumPaidByCycle(ctx context.Context, accountID, cycle string) (map[string]int64, error)
	// MarkPaid settles a seller's pending lines and returns what was settled.
	MarkPaid(ctx context.Context, accountID, cycle, sellerUserID, note string, at time.Time) ([]*CommissionLine, int64, error)
	// PendingTotalForSeller is the tile on the salesperson's own home screen
	// (G3): what they have earned this month so far.
	PendingTotalForSeller(ctx context.Context, accountID, cycle, sellerUserID string) (int64, error)
}

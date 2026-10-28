package domain

import (
	"testing"
	"time"
)

// Cycle boundaries and statement totals (§6.6, G2).
//
// The month a sale belongs to is decided in the ACCOUNT's timezone. Getting it
// wrong moves somebody's deal into the wrong payslip, which is the kind of
// error that is remembered for a year.

func TestCycleOf_UsesTheAccountsOwnMidnight(t *testing.T) {
	// 1 September, 02:00 UTC. In São Paulo (UTC-3) it is still 31 August, and
	// the salesperson who closed it will look for it in August.
	moment := time.Date(2026, 9, 1, 2, 0, 0, 0, time.UTC)

	if got := CycleOf(moment, "America/Sao_Paulo"); got != "2026-08" {
		t.Fatalf("São Paulo cycle = %q, want 2026-08", got)
	}
	if got := CycleOf(moment, "UTC"); got != "2026-09" {
		t.Fatalf("UTC cycle = %q, want 2026-09", got)
	}
	// An unknown timezone must not silently reclassify a month — LoadLocation
	// falls back rather than failing, and the fallback has to be predictable.
	if got := CycleOf(moment, "Mars/Olympus"); got != "2026-09" {
		t.Fatalf("unknown timezone cycle = %q, want the UTC fallback 2026-09", got)
	}
}

func TestCycleRange_IsHalfOpenInTheAccountsZone(t *testing.T) {
	from, to, err := CycleRange("2026-08", "America/Sao_Paulo")
	if err != nil {
		t.Fatalf("CycleRange: %v", err)
	}
	// Local midnight on the 1st, which is 03:00 UTC.
	if from.UTC() != time.Date(2026, 8, 1, 3, 0, 0, 0, time.UTC) {
		t.Fatalf("from = %v, want 2026-08-01T03:00Z", from.UTC())
	}
	if to.UTC() != time.Date(2026, 9, 1, 3, 0, 0, 0, time.UTC) {
		t.Fatalf("to = %v, want 2026-09-01T03:00Z", to.UTC())
	}

	if _, _, err := CycleRange("agosto", "UTC"); err == nil {
		t.Fatal("a malformed cycle must be refused, not answered for 'now'")
	}
}

func TestIsCycleOpen(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	if !IsCycleOpen("2026-08", "UTC", now) {
		t.Fatal("the running month is open")
	}
	if IsCycleOpen("2026-07", "UTC", now) {
		t.Fatal("a month that ended is closed")
	}
}

func TestBuildStatement_SeparatesWhatIsOwedFromWhatIsSettled(t *testing.T) {
	paidAt := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	st := BuildStatement("2026-07", "seller-1", []*CommissionLine{
		{Kind: LineCommission, SaleID: "s1", AmountCents: 150000,
			Status: CommissionPaid, PaidAt: &paidAt, SellerName: "Joana"},
		{Kind: LineCommission, SaleID: "s2", AmountCents: 50000, Status: CommissionPending},
		{Kind: LineMilestone, AmountCents: 30000, Status: CommissionPending},
		// A clawback: negative, visible, and part of the total. A statement
		// that cannot express one stops matching the bank.
		{Kind: LineAdjustment, AmountCents: -20000, Status: CommissionPending},
	})

	if st.SellerName != "Joana" {
		t.Fatalf("seller name = %q, want Joana", st.SellerName)
	}
	if st.CommissionCents != 200000 {
		t.Fatalf("commission = %d, want 200000", st.CommissionCents)
	}
	if st.MilestoneCents != 30000 {
		t.Fatalf("milestone = %d, want 30000", st.MilestoneCents)
	}
	if st.AdjustmentCents != -20000 {
		t.Fatalf("adjustment = %d, want -20000", st.AdjustmentCents)
	}
	if st.TotalCents != 210000 {
		t.Fatalf("total = %d, want 210000", st.TotalCents)
	}
	// The two numbers that matter to different people: what already left the
	// company, and what the admin is about to transfer.
	if st.PaidCents != 150000 {
		t.Fatalf("paid = %d, want 150000", st.PaidCents)
	}
	if st.PendingCents != 60000 {
		t.Fatalf("pending = %d, want 60000", st.PendingCents)
	}
}

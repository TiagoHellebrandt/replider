package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// US-023 (review-fixes follow-up): a clawback — the negative adjustment
// of an already-paid cycle recomputed DOWN — must be charged to the
// seller who actually RECEIVED the money, not the account's current
// seller. Before this fix, an A→B reallocation followed by a downward
// recompute debited B's payout with money A was paid.

// A paid R$500 for the cycle -> account reallocated A→B -> profit
// recomputed down to R$300. The -R$200 adjustment lands under A (who
// was paid), and B's payout never shrinks for it.
func TestComputeCommissions_ClawbackChargesPaidSellerNotCurrent(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "sellerA", AccountID: "a1", LeadSource: strPointer(domain.LeadSourceInbound), NetProfitCents: 250000},
	}
	uc := newCommissionUC(repo, nil)
	ctx := context.Background()

	// Cycle computed under A (inbound 20% of 250000 = 50000) and paid.
	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("first compute: %v", err)
	}
	markProfitSharePaid(t, repo, "sellerA", "a1", "2026-06", domain.CommissionReasonStandard)

	// Account reallocated to B, then the profit is recomputed DOWN:
	// 20% of 150000 = 30000 due vs 50000 paid -> delta -20000.
	repo.bases["2026-06"][0].SellerUserID = "sellerB"
	repo.bases["2026-06"][0].NetProfitCents = 150000
	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("recompute after reallocation: %v", err)
	}

	// The clawback debits A — the seller the 50000 actually went to.
	adj := repo.get(t, "sellerA", "a1", "2026-06", domain.CommissionReasonAdjustment)
	if adj.AmountCents != -20000 || adj.Status != domain.CommissionStatusPending {
		t.Fatalf("expected pending adjustment -20000 under sellerA, got %s/%d", adj.Status, adj.AmountCents)
	}
	if _, ok := repo.entries[entryKey("sellerB", "a1", "2026-06", domain.CommissionReasonAdjustment)]; ok {
		t.Fatal("clawback must not land under sellerB — B never received the paid money")
	}

	// MarkPayoutPaid for B settles nothing of A's debt: B's total is
	// untouched by money A received.
	entries, totalCents, err := repo.MarkPayoutPaid(ctx, "sellerB", "2026-06", time.Now(), nil)
	if err != nil {
		t.Fatalf("payout B: %v", err)
	}
	if entries != 0 || totalCents != 0 {
		t.Fatalf("B's payout must not absorb A's clawback, settled %d entries / %d cents", entries, totalCents)
	}

	// A's next payout absorbs the -20000 saldo.
	entries, totalCents, err = repo.MarkPayoutPaid(ctx, "sellerA", "2026-06", time.Now(), nil)
	if err != nil {
		t.Fatalf("payout A: %v", err)
	}
	if entries != 1 || totalCents != -20000 {
		t.Fatalf("A settles the clawback, got %d entries / %d cents", entries, totalCents)
	}
}

// Positive deltas are NEW work under the current link: after the
// reallocation, an upward recompute credits B (the current seller),
// never resurrects A.
func TestComputeCommissions_PositiveDeltaStaysWithCurrentSeller(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "sellerA", AccountID: "a1", LeadSource: strPointer(domain.LeadSourceInbound), NetProfitCents: 100000},
	}
	uc := newCommissionUC(repo, nil)
	ctx := context.Background()

	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("first compute: %v", err)
	}
	markProfitSharePaid(t, repo, "sellerA", "a1", "2026-06", domain.CommissionReasonStandard) // 20000 paid

	repo.bases["2026-06"][0].SellerUserID = "sellerB"
	repo.bases["2026-06"][0].NetProfitCents = 200000 // 40000 due -> delta +20000
	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("recompute: %v", err)
	}

	adj := repo.get(t, "sellerB", "a1", "2026-06", domain.CommissionReasonAdjustment)
	if adj.AmountCents != 20000 || adj.Status != domain.CommissionStatusPending {
		t.Fatalf("expected pending adjustment +20000 under sellerB, got %s/%d", adj.Status, adj.AmountCents)
	}
	if _, ok := repo.entries[entryKey("sellerA", "a1", "2026-06", domain.CommissionReasonAdjustment)]; ok {
		t.Fatal("positive delta must not land under the unlinked sellerA")
	}
}

// Chained clawback: A's clawback settles, then the cycle drops again.
// The paid seller is now the seller of the most recently settled row —
// still A (the clawback settled under A), so the next negative delta
// keeps debiting A.
func TestComputeCommissions_ClawbackFollowsMostRecentlyPaidSeller(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "sellerA", AccountID: "a1", LeadSource: strPointer(domain.LeadSourceInbound), NetProfitCents: 250000},
	}
	uc := newCommissionUC(repo, nil)
	ctx := context.Background()

	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("first compute: %v", err)
	}
	markProfitSharePaid(t, repo, "sellerA", "a1", "2026-06", domain.CommissionReasonStandard) // 50000 paid

	repo.bases["2026-06"][0].SellerUserID = "sellerB"
	repo.bases["2026-06"][0].NetProfitCents = 150000 // 30000 due -> -20000 under A
	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("second compute: %v", err)
	}
	if _, _, err := repo.MarkPayoutPaid(ctx, "sellerA", "2026-06", time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC), nil); err != nil {
		t.Fatalf("settle A's clawback: %v", err)
	}

	// Paid total is now 30000 (50000 - 20000). Cycle drops again:
	// 20% of 100000 = 20000 due -> delta -10000, still under A.
	repo.bases["2026-06"][0].NetProfitCents = 100000
	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("third compute: %v", err)
	}
	adj := repo.get(t, "sellerA", "a1", "2026-06", domain.CommissionReasonAdjustment)
	if adj.AmountCents != -10000 || adj.Status != domain.CommissionStatusPending {
		t.Fatalf("expected pending adjustment -10000 under sellerA, got %s/%d", adj.Status, adj.AmountCents)
	}
	if _, ok := repo.entries[entryKey("sellerB", "a1", "2026-06", domain.CommissionReasonAdjustment)]; ok {
		t.Fatal("chained clawback must keep debiting the paid seller, not B")
	}
}

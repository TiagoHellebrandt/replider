package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// US-004 (review fixes): paid ledger rows are immutable. These tests
// are adversarial reproductions of the double-pay holes: a reason flip
// or a seller reallocation after the cycle settled used to mint a
// SECOND full pending row next to the paid one (different UNIQUE key),
// paying the same account-cycle profit twice.

// markProfitSharePaid settles the entry directly, like the owner payout
// does.
func markProfitSharePaid(t *testing.T, repo *mockCommissionRepo, seller, account, cycle, reason string) {
	t.Helper()
	e := repo.get(t, seller, account, cycle, reason)
	e.Status = domain.CommissionStatusPaid
	at := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	e.PaidAt = &at
}

// Paid standard -> the SLA ladder flips the cycle to sla_reduced on a
// recompute. The reduced rate must NOT become a new pending row (the
// paid 5000 already over-covers it); the difference lands as a negative
// pending adjustment (saldo).
func TestComputeCommissions_PaidCycleSLAFlipDoesNotDoublePay(t *testing.T) {
	repo := newMockCommissionRepo()
	uc := newSLACommissionUC(repo)
	ctx := context.Background()

	// Cycle computed clean (no breach) and settled: 50% of 10000.
	seedSLABase(repo, "2026-03", "s1", "a1")
	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-03"); err != nil {
		t.Fatalf("first compute: %v", err)
	}
	markProfitSharePaid(t, repo, "s1", "a1", "2026-03", domain.CommissionReasonStandard)

	// The SLA data lands late: previous cycle already breached and March
	// has an unanswered conversation -> 2nd consecutive breach -> the
	// recompute now prices the cycle at sla_reduced 20% (2000).
	repo.slaStates["s1|2026-02"] = &domain.SellerSLAState{
		SellerUserID: "s1", Cycle: "2026-02", Breach: true, ConsecutiveBreaches: 1,
		Action: domain.SLAActionWarning,
	}
	repo.slaConvs = []domain.SLAConversation{
		{ConversationID: "c1", AssignedTo: "s1", CreatedAt: time.Date(2026, 3, 2, 14, 0, 0, 0, time.UTC)},
	}

	entries, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-03")
	if err != nil {
		t.Fatalf("recompute: %v", err)
	}
	if _, ok := repo.entries[entryKey("s1", "a1", "2026-03", domain.CommissionReasonSLAReduced)]; ok {
		t.Fatal("paid cycle must not gain a new pending sla_reduced row (double pay)")
	}
	paid := repo.get(t, "s1", "a1", "2026-03", domain.CommissionReasonStandard)
	if paid.Status != domain.CommissionStatusPaid || paid.AmountCents != 5000 {
		t.Fatalf("paid row must survive untouched, got %s/%d", paid.Status, paid.AmountCents)
	}
	adj := repo.get(t, "s1", "a1", "2026-03", domain.CommissionReasonAdjustment)
	if adj.AmountCents != -3000 || adj.Status != domain.CommissionStatusPending {
		t.Fatalf("expected pending adjustment -3000 (2000 due - 5000 paid), got %s/%d", adj.Status, adj.AmountCents)
	}
	if len(entries) != 2 {
		t.Fatalf("expected paid standard + adjustment only, got %d entries", len(entries))
	}

	// Idempotent: a second recompute keeps the single adjustment row.
	if entries, err = uc.ComputeCycle(ctx, "owner@x.com", "2026-03"); err != nil || len(entries) != 2 {
		t.Fatalf("recompute again: err %v entries %d, want 2", err, len(entries))
	}
	if adj = repo.get(t, "s1", "a1", "2026-03", domain.CommissionReasonAdjustment); adj.AmountCents != -3000 {
		t.Fatalf("adjustment must stay -3000 under recompute, got %d", adj.AmountCents)
	}
}

// Paid standard on the inbound track -> a plan upgrade lands late and
// the recompute flips the cycle to bonus_upgrade at the own rate. The
// upgrade must pay only the POSITIVE DELTA vs what settled, not a fresh
// full row.
func TestComputeCommissions_PaidCycleUpgradeFlipPaysOnlyDelta(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "s1", AccountID: "a1", LeadSource: strPointer(domain.LeadSourceInbound), NetProfitCents: 10000},
	}
	uc := newCommissionUC(repo, nil)
	ctx := context.Background()

	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("first compute: %v", err)
	}
	markProfitSharePaid(t, repo, "s1", "a1", "2026-06", domain.CommissionReasonStandard) // 20% = 2000

	repo.bases["2026-06"][0].HasPlanUpgrade = true // recompute now prices 50% = 5000
	entries, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06")
	if err != nil {
		t.Fatalf("recompute: %v", err)
	}
	if _, ok := repo.entries[entryKey("s1", "a1", "2026-06", domain.CommissionReasonBonusUpgrade)]; ok {
		t.Fatal("paid cycle must not gain a full pending bonus_upgrade row (double pay)")
	}
	adj := repo.get(t, "s1", "a1", "2026-06", domain.CommissionReasonAdjustment)
	if adj.AmountCents != 3000 || adj.Status != domain.CommissionStatusPending {
		t.Fatalf("expected pending adjustment +3000 (5000 due - 2000 paid), got %s/%d", adj.Status, adj.AmountCents)
	}
	if len(entries) != 2 {
		t.Fatalf("expected paid standard + adjustment, got %d entries", len(entries))
	}

	// The adjustment settles too; a further recompute finds the full
	// 5000 already paid -> delta 0 -> no new pending row appears.
	markProfitSharePaid(t, repo, "s1", "a1", "2026-06", domain.CommissionReasonAdjustment)
	if entries, err = uc.ComputeCycle(ctx, "owner@x.com", "2026-06"); err != nil || len(entries) != 2 {
		t.Fatalf("post-settlement recompute: err %v entries %d, want the same 2 paid rows", err, len(entries))
	}
}

// Reallocation A -> B after the cycle settled under A: B must NOT
// receive a fresh full pending row for profit that was already paid
// out. Same profit -> delta 0 -> no adjustment either.
func TestComputeCommissions_ReallocationAfterPaidCycleNoNewFullRow(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "sellerA", AccountID: "a1", LeadSource: strPointer(domain.LeadSourceInbound), NetProfitCents: 100000},
	}
	uc := newCommissionUC(repo, nil)
	ctx := context.Background()

	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("first compute: %v", err)
	}
	markProfitSharePaid(t, repo, "sellerA", "a1", "2026-06", domain.CommissionReasonStandard) // 20000

	// Owner reallocates the account to sellerB, then recomputes June.
	repo.bases["2026-06"][0].SellerUserID = "sellerB"
	entries, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06")
	if err != nil {
		t.Fatalf("recompute: %v", err)
	}
	if _, ok := repo.entries[entryKey("sellerB", "a1", "2026-06", domain.CommissionReasonStandard)]; ok {
		t.Fatal("reallocation must not mint a full pending row for the new seller on a paid cycle")
	}
	if len(entries) != 1 {
		t.Fatalf("expected only sellerA's paid row (delta 0, no adjustment), got %d entries", len(entries))
	}
	paid := repo.get(t, "sellerA", "a1", "2026-06", domain.CommissionReasonStandard)
	if paid.Status != domain.CommissionStatusPaid || paid.AmountCents != 20000 {
		t.Fatalf("sellerA's paid row must survive, got %s/%d", paid.Status, paid.AmountCents)
	}
}

// A retention bonus granted to seller A must keep blocking after the
// account moves to seller B — the once-ever dedup is per (account,
// reason), not per seller, and the current cycle's row counts.
func TestComputeCommissions_RetentionBonusSurvivesReallocation(t *testing.T) {
	repo := newMockCommissionRepo()
	// Bonus already granted to sellerA in this same cycle.
	repo.entries[entryKey("sellerA", "a1", "2026-06", domain.CommissionReasonBonusRetention)] = &domain.CommissionEntry{
		SellerUserID: "sellerA", AccountID: "a1", Cycle: "2026-06",
		Reason: domain.CommissionReasonBonusRetention, Status: domain.CommissionStatusPending, AmountCents: 10000,
	}
	// Account now belongs to sellerB and still qualifies for retention.
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "sellerB", AccountID: "a1", LeadSource: strPointer(domain.LeadSourceInbound), NetProfitCents: 100000,
			AccountCreatedAt: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC), SubscriptionStatus: domain.SubscriptionStatusActive},
	}
	uc := newCommissionUC(repo, nil)

	entries, err := uc.ComputeCycle(context.Background(), "owner@x.com", "2026-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := repo.entries[entryKey("sellerB", "a1", "2026-06", domain.CommissionReasonBonusRetention)]; ok {
		t.Fatal("reallocation must not duplicate the retention bonus under the new seller")
	}
	retention := 0
	for _, e := range entries {
		if e.Reason == domain.CommissionReasonBonusRetention {
			retention++
		}
	}
	if retention != 1 {
		t.Fatalf("expected exactly 1 retention bonus row, got %d", retention)
	}
}

// Same dedup for the quarterly NPS bonus: sellerA earned it earlier in
// the quarter (already paid), the account moves to sellerB inside the
// SAME cycle it would re-trigger — no second grant.
func TestComputeCommissions_NPSBonusSurvivesReallocationSameCycle(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.entries[entryKey("sellerA", "a1", "2026-06", domain.CommissionReasonBonusNPS)] = &domain.CommissionEntry{
		SellerUserID: "sellerA", AccountID: "a1", Cycle: "2026-06",
		Reason: domain.CommissionReasonBonusNPS, Status: domain.CommissionStatusPaid, AmountCents: 10000,
	}
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "sellerB", AccountID: "a1", LeadSource: strPointer(domain.LeadSourceInbound), NetProfitCents: 100000, HasPromoterNPS: true},
	}
	uc := newCommissionUC(repo, nil)

	if _, err := uc.ComputeCycle(context.Background(), "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := repo.entries[entryKey("sellerB", "a1", "2026-06", domain.CommissionReasonBonusNPS)]; ok {
		t.Fatal("same-cycle reallocation must not duplicate the NPS bonus under the new seller")
	}
}

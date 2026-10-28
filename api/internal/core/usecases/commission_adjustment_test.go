package usecases

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/replider/api/internal/core/domain"
)

// US-005 (review fixes): a delta that appears AFTER an early payout is
// paid as an adjustment, never silently lost — including the case the
// US-004 guard left open, where the previous adjustment itself already
// settled and blocked the next delta's insert.

// captureAuditLog records commission audit entries so the tests can
// assert the WHAT (the adjusted delta), not just that an action ran.
type captureAuditLog struct{ logs []*domain.AuditLog }

func (c *captureAuditLog) Create(_ context.Context, l *domain.AuditLog) error {
	c.logs = append(c.logs, l)
	return nil
}
func (c *captureAuditLog) ListByAccount(context.Context, string, int32, int32) ([]*domain.AuditLog, error) {
	return nil, nil
}
func (c *captureAuditLog) CountByAccount(context.Context, string) (int64, error) { return 0, nil }
func (c *captureAuditLog) ListFiltered(context.Context, domain.AuditLogFilter) ([]*domain.AuditLog, error) {
	return nil, nil
}
func (c *captureAuditLog) CountFiltered(context.Context, domain.AuditLogFilter) (int64, error) {
	return 0, nil
}

func (c *captureAuditLog) find(action string) *domain.AuditLog {
	for _, l := range c.logs {
		if l.Action == action {
			return l
		}
	}
	return nil
}

// The PRD's exact scenario: R$300 settled early, the recompute finds
// R$500 due -> pending adjustment of R$200; a further recompute drops
// the due to R$200 -> the SAME pending adjustment becomes -R$100
// (saldo), never a second row.
func TestComputeCommissions_EarlyPayoutDeltaBecomesAdjustment(t *testing.T) {
	repo := newMockCommissionRepo()
	audit := &captureAuditLog{}
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "s1", AccountID: "a1", LeadSource: strPointer(domain.LeadSourceInbound), NetProfitCents: 150000},
	}
	uc := NewComputeCommissionsUseCase(repo, &mockCommissionUsers{users: map[string]*domain.User{}}, audit)
	ctx := context.Background()

	// 20% of 150000 = 30000 ("R$300"), settled early.
	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("first compute: %v", err)
	}
	markProfitSharePaid(t, repo, "s1", "a1", "2026-06", domain.CommissionReasonStandard)

	// Profit grows: 20% of 250000 = 50000 due vs 30000 paid -> +20000.
	repo.bases["2026-06"][0].NetProfitCents = 250000
	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("recompute up: %v", err)
	}
	adj := repo.get(t, "s1", "a1", "2026-06", domain.CommissionReasonAdjustment)
	if adj.Status != domain.CommissionStatusPending || adj.AmountCents != 20000 {
		t.Fatalf("expected pending adjustment +20000, got %s/%d", adj.Status, adj.AmountCents)
	}

	// The audit trail records the delta itself (US-005), not just that a
	// recompute ran.
	entry := audit.find("commission.adjustment")
	if entry == nil {
		t.Fatal("expected a commission.adjustment audit entry with the delta")
	}
	var payload map[string]any
	if err := json.Unmarshal(entry.Payload, &payload); err != nil {
		t.Fatalf("adjustment audit payload must be JSON: %v", err)
	}
	if payload["delta_cents"] != float64(20000) || payload["paid_cents"] != float64(30000) {
		t.Fatalf("audit payload must carry the delta, got %v", payload)
	}

	// Profit shrinks: 20% of 100000 = 20000 due vs 30000 paid -> the
	// pending adjustment is REWRITTEN to -10000 (saldo), no second row.
	repo.bases["2026-06"][0].NetProfitCents = 100000
	entries, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06")
	if err != nil {
		t.Fatalf("recompute down: %v", err)
	}
	adj = repo.get(t, "s1", "a1", "2026-06", domain.CommissionReasonAdjustment)
	if adj.Status != domain.CommissionStatusPending || adj.AmountCents != -10000 {
		t.Fatalf("expected pending adjustment -10000, got %s/%d", adj.Status, adj.AmountCents)
	}
	if len(entries) != 2 {
		t.Fatalf("expected paid standard + single adjustment, got %d entries", len(entries))
	}
}

// The hole US-004 left open: once an adjustment itself SETTLES, the
// old UNIQUE(seller, account, cycle, reason) made the next delta's
// insert collide with the paid adjustment row and vanish (the upsert's
// pending-only guard no-oped). With the 000135 pending-only index the
// new delta lands as a fresh pending adjustment NEXT TO the paid one.
// FAILS on the pre-US-005 code: the second delta was silently dropped.
func TestComputeCommissions_PaidAdjustmentDoesNotBlockNextDelta(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "s1", AccountID: "a1", LeadSource: strPointer(domain.LeadSourceInbound), NetProfitCents: 100000},
	}
	uc := newCommissionUC(repo, nil)
	ctx := context.Background()

	// Standard 20% of 100000 = 20000, settled.
	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("first compute: %v", err)
	}
	markProfitSharePaid(t, repo, "s1", "a1", "2026-06", domain.CommissionReasonStandard)

	// First delta: due 30000 vs 20000 paid -> adjustment +10000, which
	// then settles too (total paid 30000).
	repo.bases["2026-06"][0].NetProfitCents = 150000
	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("recompute 1: %v", err)
	}
	markProfitSharePaid(t, repo, "s1", "a1", "2026-06", domain.CommissionReasonAdjustment)

	// Second delta: due 50000 vs 30000 paid -> a NEW pending adjustment
	// of +20000 must appear next to the settled one.
	repo.bases["2026-06"][0].NetProfitCents = 250000
	entries, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06")
	if err != nil {
		t.Fatalf("recompute 2: %v", err)
	}

	adj := repo.get(t, "s1", "a1", "2026-06", domain.CommissionReasonAdjustment)
	if adj.Status != domain.CommissionStatusPending || adj.AmountCents != 20000 {
		t.Fatalf("expected NEW pending adjustment +20000 after the paid one, got %s/%d", adj.Status, adj.AmountCents)
	}
	var paidAdjustments, pendingAdjustments int
	for _, e := range entries {
		if e.Reason != domain.CommissionReasonAdjustment {
			continue
		}
		if e.Status == domain.CommissionStatusPaid {
			paidAdjustments++
			if e.AmountCents != 10000 {
				t.Fatalf("settled adjustment must stay untouched at +10000, got %d", e.AmountCents)
			}
		} else {
			pendingAdjustments++
		}
	}
	if paidAdjustments != 1 || pendingAdjustments != 1 {
		t.Fatalf("expected 1 paid + 1 pending adjustment, got %d paid / %d pending (%d entries)",
			paidAdjustments, pendingAdjustments, len(entries))
	}

	// Everything settles -> total paid 50000 -> a further recompute at
	// the same profit finds delta 0 and mints nothing.
	markProfitSharePaid(t, repo, "s1", "a1", "2026-06", domain.CommissionReasonAdjustment)
	entries, err = uc.ComputeCycle(ctx, "owner@x.com", "2026-06")
	if err != nil {
		t.Fatalf("recompute 3: %v", err)
	}
	for _, e := range entries {
		if e.Status != domain.CommissionStatusPaid {
			t.Fatalf("no pending row may remain once the full due settled, got %+v", e)
		}
	}
}

// A pending adjustment whose delta returns to 0 disappears on the next
// recompute (DeleteStaleProfitShare) — no dangling saldo row.
func TestComputeCommissions_AdjustmentBackToZeroIsRemoved(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.bases["2026-06"] = []domain.CommissionBase{
		{SellerUserID: "s1", AccountID: "a1", LeadSource: strPointer(domain.LeadSourceInbound), NetProfitCents: 100000},
	}
	uc := newCommissionUC(repo, nil)
	ctx := context.Background()

	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("first compute: %v", err)
	}
	markProfitSharePaid(t, repo, "s1", "a1", "2026-06", domain.CommissionReasonStandard) // 20000

	repo.bases["2026-06"][0].NetProfitCents = 150000 // due 30000 -> +10000 pending
	if _, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06"); err != nil {
		t.Fatalf("recompute up: %v", err)
	}
	repo.get(t, "s1", "a1", "2026-06", domain.CommissionReasonAdjustment)

	repo.bases["2026-06"][0].NetProfitCents = 100000 // due back to 20000 -> delta 0
	entries, err := uc.ComputeCycle(ctx, "owner@x.com", "2026-06")
	if err != nil {
		t.Fatalf("recompute back: %v", err)
	}
	if _, ok := repo.entries[entryKey("s1", "a1", "2026-06", domain.CommissionReasonAdjustment)]; ok {
		t.Fatal("zero-delta pending adjustment must be removed, not left dangling")
	}
	if len(entries) != 1 {
		t.Fatalf("expected only the paid standard row, got %d entries", len(entries))
	}
}

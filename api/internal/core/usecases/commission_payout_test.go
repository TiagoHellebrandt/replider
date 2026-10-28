package usecases

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

func seedPayoutEntry(repo *mockCommissionRepo, seller, account, cycle, reason, status string, amount int64) {
	key := entryKey(seller, account, cycle, reason)
	repo.entries[key] = &domain.CommissionEntry{
		ID:           key,
		SellerUserID: seller,
		AccountID:    account,
		Cycle:        cycle,
		Reason:       reason,
		Status:       status,
		AmountCents:  amount,
		RateApplied:  20,
		SellerEmail:  seller + "@x.com",
		AccountName:  "Conta " + account,
	}
}

func TestListPayouts_GroupsAndTotals(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.settings["s1"] = &domain.SellerSettings{UserID: "s1", PixKey: strPointer("s1@pix.com")}
	// s1: cycle pending + paid + an earlier-cycle pending (adjustment).
	seedPayoutEntry(repo, "s1", "a1", "2026-06", domain.CommissionReasonStandard, domain.CommissionStatusPending, 5000)
	seedPayoutEntry(repo, "s1", "a2", "2026-06", domain.CommissionReasonBonusNPS, domain.CommissionStatusPaid, 10000)
	seedPayoutEntry(repo, "s1", "a1", "2026-05", domain.CommissionReasonStandard, domain.CommissionStatusPending, 3000)
	// s2: only current-cycle pending; earlier PAID row is not an adjustment.
	seedPayoutEntry(repo, "s2", "a3", "2026-06", domain.CommissionReasonStandard, domain.CommissionStatusPending, 2000)
	seedPayoutEntry(repo, "s2", "a3", "2026-05", domain.CommissionReasonStandard, domain.CommissionStatusPaid, 9000)
	uc := newCommissionUC(repo, nil)

	out, err := uc.ListPayouts(context.Background(), "2026-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Sellers) != 2 {
		t.Fatalf("expected 2 sellers, got %d", len(out.Sellers))
	}
	s1 := out.Sellers[0]
	if s1.SellerUserID != "s1" || s1.PendingCents != 5000 || s1.PaidCents != 10000 ||
		s1.AdjustmentCents != 3000 || s1.TotalDueCents != 8000 || s1.AdjustmentEntries != 1 {
		t.Fatalf("unexpected s1 summary: %+v", s1)
	}
	if s1.PixKey == nil || *s1.PixKey != "s1@pix.com" {
		t.Fatalf("expected s1 pix key joined, got %+v", s1.PixKey)
	}
	s2 := out.Sellers[1]
	if s2.PendingCents != 2000 || s2.AdjustmentCents != 0 || s2.TotalDueCents != 2000 {
		t.Fatalf("unexpected s2 summary: %+v", s2)
	}
	if out.TotalDueCents != 10000 || out.TotalPaidCents != 10000 {
		t.Fatalf("unexpected totals: due=%d paid=%d", out.TotalDueCents, out.TotalPaidCents)
	}
}

func TestListPayouts_InvalidCycle(t *testing.T) {
	uc := newCommissionUC(newMockCommissionRepo(), nil)
	if _, err := uc.ListPayouts(context.Background(), "junho"); err == nil {
		t.Fatal("expected invalid-cycle error")
	}
}

func TestMarkPayoutPaid_SettlesCycleAndAdjustments(t *testing.T) {
	repo := newMockCommissionRepo()
	seedPayoutEntry(repo, "s1", "a1", "2026-06", domain.CommissionReasonStandard, domain.CommissionStatusPending, 5000)
	seedPayoutEntry(repo, "s1", "a1", "2026-05", domain.CommissionReasonStandard, domain.CommissionStatusPending, 3000)
	// Another seller and a FUTURE cycle must stay untouched.
	seedPayoutEntry(repo, "s2", "a3", "2026-06", domain.CommissionReasonStandard, domain.CommissionStatusPending, 2000)
	seedPayoutEntry(repo, "s1", "a1", "2026-07", domain.CommissionReasonStandard, domain.CommissionStatusPending, 7000)
	uc := newCommissionUC(repo, nil)
	paidAt := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	uc.now = func() time.Time { return paidAt }

	out, err := uc.MarkPayoutPaid(context.Background(), "owner@x.com", "s1", "2026-06", "PIX tx 123", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cur := repo.get(t, "s1", "a1", "2026-06", domain.CommissionReasonStandard)
	adj := repo.get(t, "s1", "a1", "2026-05", domain.CommissionReasonStandard)
	for _, e := range []*domain.CommissionEntry{cur, adj} {
		if e.Status != domain.CommissionStatusPaid || e.PaidAt == nil || !e.PaidAt.Equal(paidAt) {
			t.Fatalf("expected entry settled at %v, got %+v", paidAt, e)
		}
		if e.Note == nil || *e.Note != "PIX tx 123" {
			t.Fatalf("expected note stamped, got %+v", e.Note)
		}
	}
	if e := repo.get(t, "s2", "a3", "2026-06", domain.CommissionReasonStandard); e.Status != domain.CommissionStatusPending {
		t.Fatalf("other seller must stay pending, got %+v", e)
	}
	if e := repo.get(t, "s1", "a1", "2026-07", domain.CommissionReasonStandard); e.Status != domain.CommissionStatusPending {
		t.Fatalf("future cycle must stay pending, got %+v", e)
	}
	// Refreshed listing: s1 settled, s2 still due.
	for _, s := range out.Sellers {
		if s.SellerUserID == "s1" && s.TotalDueCents != 0 {
			t.Fatalf("expected s1 settled in refreshed listing, got %+v", s)
		}
	}
}

func TestMarkPayoutPaid_IdempotentAndLateRecompute(t *testing.T) {
	repo := newMockCommissionRepo()
	seedPayoutEntry(repo, "s1", "a1", "2026-06", domain.CommissionReasonStandard, domain.CommissionStatusPending, 5000)
	uc := newCommissionUC(repo, nil)
	uc.now = func() time.Time { return time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC) }

	if _, err := uc.MarkPayoutPaid(context.Background(), "owner@x.com", "s1", "2026-06", "", false); err != nil {
		t.Fatalf("first mark: %v", err)
	}
	first := *repo.get(t, "s1", "a1", "2026-06", domain.CommissionReasonStandard)
	// Second click: no-op, paid_at unchanged, no error.
	if _, err := uc.MarkPayoutPaid(context.Background(), "owner@x.com", "s1", "2026-06", "outra nota", false); err != nil {
		t.Fatalf("second mark: %v", err)
	}
	second := repo.get(t, "s1", "a1", "2026-06", domain.CommissionReasonStandard)
	if !second.PaidAt.Equal(*first.PaidAt) || second.Note != nil {
		t.Fatalf("paid entry must be untouched by a second mark, got %+v", second)
	}

	// Late recompute mints a new pending entry AFTER settlement: it must
	// stay pending and surface as adjustment on the NEXT cycle.
	seedPayoutEntry(repo, "s1", "a2", "2026-06", domain.CommissionReasonStandard, domain.CommissionStatusPending, 1500)
	out, err := uc.ListPayouts(context.Background(), "2026-07")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Sellers) != 1 || out.Sellers[0].AdjustmentCents != 1500 || out.Sellers[0].TotalDueCents != 1500 {
		t.Fatalf("expected late entry as next-cycle adjustment, got %+v", out.Sellers)
	}
}

func TestMarkPayoutPaid_Validation(t *testing.T) {
	uc := newCommissionUC(newMockCommissionRepo(), nil)
	uc.now = func() time.Time { return time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC) }
	if _, err := uc.MarkPayoutPaid(context.Background(), "o@x.com", "s1", "2026/06", "", false); err == nil {
		t.Fatal("expected invalid-cycle error")
	}
	if _, err := uc.MarkPayoutPaid(context.Background(), "o@x.com", "", "2026-06", "", false); err == nil {
		t.Fatal("expected missing-seller error")
	}
	if _, err := uc.MarkPayoutPaid(context.Background(), "o@x.com", "s1", "2026-06", strings.Repeat("x", 281), false); err == nil {
		t.Fatal("expected note-too-long error")
	}
}

// US-005 guard: the RUNNING (or a future) cycle only settles with the
// explicit allow_open_cycle override — an early payout freezes a
// partial number, so it must be a conscious act, never the default.
// FAILS on the pre-US-005 code, where any cycle settled silently.
func TestMarkPayoutPaid_RejectsOpenCycleWithoutOverride(t *testing.T) {
	repo := newMockCommissionRepo()
	seedPayoutEntry(repo, "s1", "a1", "2026-07", domain.CommissionReasonStandard, domain.CommissionStatusPending, 5000)
	uc := newCommissionUC(repo, nil)
	uc.now = func() time.Time { return time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC) }
	ctx := context.Background()

	_, err := uc.MarkPayoutPaid(ctx, "o@x.com", "s1", "2026-07", "", false)
	assertAppErrorCode(t, err, "COMMISSION_CYCLE_OPEN")
	if e := repo.get(t, "s1", "a1", "2026-07", domain.CommissionReasonStandard); e.Status != domain.CommissionStatusPending {
		t.Fatalf("open cycle must stay pending without the override, got %+v", e)
	}

	// Future cycle: same rejection.
	_, err = uc.MarkPayoutPaid(ctx, "o@x.com", "s1", "2026-08", "", false)
	assertAppErrorCode(t, err, "COMMISSION_CYCLE_OPEN")

	// Explicit override settles the partial cycle.
	if _, err := uc.MarkPayoutPaid(ctx, "o@x.com", "s1", "2026-07", "", true); err != nil {
		t.Fatalf("override must settle the open cycle: %v", err)
	}
	if e := repo.get(t, "s1", "a1", "2026-07", domain.CommissionReasonStandard); e.Status != domain.CommissionStatusPaid {
		t.Fatalf("expected entry settled under override, got %+v", e)
	}

	// A closed cycle needs no override.
	seedPayoutEntry(repo, "s1", "a2", "2026-06", domain.CommissionReasonStandard, domain.CommissionStatusPending, 1000)
	if _, err := uc.MarkPayoutPaid(ctx, "o@x.com", "s1", "2026-06", "", false); err != nil {
		t.Fatalf("closed cycle must settle without override: %v", err)
	}
}

func assertAppErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	appErr, ok := err.(*domain.AppError)
	if !ok || appErr.Code != code {
		t.Fatalf("expected AppError %s, got %v", code, err)
	}
}

func TestExportPayoutCSV(t *testing.T) {
	repo := newMockCommissionRepo()
	repo.settings["s1"] = &domain.SellerSettings{UserID: "s1", PixKey: strPointer("chave-pix-s1")}
	seedPayoutEntry(repo, "s1", "a1", "2026-06", domain.CommissionReasonStandard, domain.CommissionStatusPending, 5000)
	// Comma in the account name must come out quoted, not split.
	repo.entries[entryKey("s1", "a1", "2026-06", domain.CommissionReasonStandard)].AccountName = "Padaria, Pão & Cia"
	uc := newCommissionUC(repo, nil)

	data, err := uc.ExportPayoutCSV(context.Background(), "2026-06")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	csvText := string(data)
	lines := strings.Split(strings.TrimSpace(csvText), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected header + 1 row, got %d lines:\n%s", len(lines), csvText)
	}
	if !strings.HasPrefix(lines[0], "cycle,seller_email,seller_pix_key,account_name,reason") {
		t.Fatalf("unexpected header: %s", lines[0])
	}
	row := lines[1]
	for _, want := range []string{"2026-06", "s1@x.com", "chave-pix-s1", `"Padaria, Pão & Cia"`, "standard", "5000", "pending"} {
		if !strings.Contains(row, want) {
			t.Fatalf("row missing %q: %s", want, row)
		}
	}

	if _, err := uc.ExportPayoutCSV(context.Background(), "not-a-cycle"); err == nil {
		t.Fatal("expected invalid-cycle error")
	}
}

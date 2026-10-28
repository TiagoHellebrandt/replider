package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// --- shared fakes (used by the settler and worker tests) ---

type recordCall struct {
	accountID string
	convs     int32
	total     int32
	capped    bool
}

type fakeOverageUsageRepo struct {
	domain.UsageRepository
	candidates []domain.OverageCandidate
	recorded   []recordCall
	recordErr  error
}

func (f *fakeOverageUsageRepo) ListOverageCandidates(context.Context) ([]domain.OverageCandidate, error) {
	return f.candidates, nil
}

func (f *fakeOverageUsageRepo) RecordOverageCharge(_ context.Context, accountID string, convs, total int32, capped bool) error {
	if f.recordErr != nil {
		return f.recordErr
	}
	f.recorded = append(f.recorded, recordCall{accountID, convs, total, capped})
	return nil
}

type chargeCall struct {
	customerID string
	amount     int64
	currency   string
	idemKey    string
}

type fakeOverageCharger struct {
	calls []chargeCall
	err   error
}

func (f *fakeOverageCharger) CreateOverageInvoiceItem(customerID string, amount int64, currency, _, idemKey string) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, chargeCall{customerID, amount, currency, idemKey})
	return nil
}

// fakeLedger is an in-memory domain.OverageChargeRepository.
type fakeLedger struct {
	charges   map[string]*domain.OverageCharge
	createErr error
}

func newFakeLedger() *fakeLedger {
	return &fakeLedger{charges: map[string]*domain.OverageCharge{}}
}

func (l *fakeLedger) Create(_ context.Context, c *domain.OverageCharge) error {
	if l.createErr != nil {
		return l.createErr
	}
	cp := *c
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now()
	}
	cp.Status = domain.OverageChargePending
	l.charges[cp.ID] = &cp
	return nil
}

func (l *fakeLedger) GetPending(_ context.Context, accountID string) (*domain.OverageCharge, error) {
	for _, c := range l.charges {
		if c.AccountID == accountID && c.Status == domain.OverageChargePending {
			cp := *c
			return &cp, nil
		}
	}
	return nil, nil
}

func (l *fakeLedger) MarkCharged(_ context.Context, id string) error {
	if c, ok := l.charges[id]; ok {
		c.Status = domain.OverageChargeCharged
	}
	return nil
}

func (l *fakeLedger) MarkNeedsReview(_ context.Context, id string) error {
	if c, ok := l.charges[id]; ok {
		c.Status = domain.OverageChargeNeedsReview
	}
	return nil
}

func (l *fakeLedger) SumCharged(_ context.Context, accountID string) (int32, error) {
	var total int32
	for _, c := range l.charges {
		if c.AccountID == accountID && c.Status == domain.OverageChargeCharged {
			total += c.AmountCents
		}
	}
	return total, nil
}

func (l *fakeLedger) ArchiveCharged(_ context.Context, accountID string) error {
	for _, c := range l.charges {
		if c.AccountID == accountID && c.Status == domain.OverageChargeCharged {
			c.Status = domain.OverageChargeArchived
		}
	}
	return nil
}

// seedPending inserts a pre-existing pending charge (simulates a crash
// that left an in-flight charge behind).
func (l *fakeLedger) seedPending(id, accountID string, amount int32, createdAt time.Time) {
	l.charges[id] = &domain.OverageCharge{
		ID:                        id,
		AccountID:                 accountID,
		AmountCents:               amount,
		RunningTotalCents:         amount,
		OverageConversationsCount: 50,
		Status:                    domain.OverageChargePending,
		CreatedAt:                 createdAt,
	}
}

func (l *fakeLedger) statusOf(id string) string {
	if c, ok := l.charges[id]; ok {
		return c.Status
	}
	return ""
}

func (l *fakeLedger) pendingCount() int {
	n := 0
	for _, c := range l.charges {
		if c.Status == domain.OverageChargePending {
			n++
		}
	}
	return n
}

// Starter-shaped candidate: R$399 plan, 700-conversation franchise,
// 70¢/conversation overage, 2× cap (→ a 79800¢ ceiling).
func starterCandidate(convs int32) domain.OverageCandidate {
	return domain.OverageCandidate{
		AccountID:                "acc_1",
		StripeCustomerID:         "cus_1",
		Currency:                 domain.CurrencyBRL,
		PlanPriceCents:           39900,
		MaxConversationsPerMonth: 700,
		OverageConversationCents: 70,
		OverageCapMultiplier:     2.0,
		ConversationsCount:       convs,
	}
}

// --- OverageSettler tests ---

func TestOverageSettler_NoOverageUnderFranchise(t *testing.T) {
	usage := &fakeOverageUsageRepo{}
	ledger := newFakeLedger()
	charger := &fakeOverageCharger{}
	s := NewOverageSettler(usage, ledger, charger, true)

	if err := s.Settle(context.Background(), starterCandidate(650)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(charger.calls) != 0 || len(ledger.charges) != 0 {
		t.Fatalf("expected no charge/ledger row, got %d/%d", len(charger.calls), len(ledger.charges))
	}
}

func TestOverageSettler_BillsDeltaUnderCap(t *testing.T) {
	// 750 conversations → 50 over → 50 × 70¢ = 3500¢.
	usage := &fakeOverageUsageRepo{}
	ledger := newFakeLedger()
	charger := &fakeOverageCharger{}
	s := NewOverageSettler(usage, ledger, charger, true)

	if err := s.Settle(context.Background(), starterCandidate(750)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(charger.calls) != 1 || charger.calls[0].amount != 3500 {
		t.Fatalf("expected one 3500¢ charge, got %+v", charger.calls)
	}
	// Regression (US-003): a BRL account is billed in brl.
	if charger.calls[0].currency != "brl" {
		t.Fatalf("expected brl invoice item, got %q", charger.calls[0].currency)
	}
	// The ledger row must exist and be marked charged.
	if len(ledger.charges) != 1 {
		t.Fatalf("expected one ledger row, got %d", len(ledger.charges))
	}
	for id, c := range ledger.charges {
		if c.Status != domain.OverageChargeCharged {
			t.Fatalf("expected charged status, got %q", c.Status)
		}
		// The idempotency key must be the ledger row id.
		if charger.calls[0].idemKey != id {
			t.Fatalf("idempotency key %q != ledger id %q", charger.calls[0].idemKey, id)
		}
	}
}

// TestOverageSettler_BillsInAccountCurrency locks in US-003: the invoice
// item is created in the candidate's account currency, never hardcoded.
func TestOverageSettler_BillsInAccountCurrency(t *testing.T) {
	usage := &fakeOverageUsageRepo{}
	ledger := newFakeLedger()
	charger := &fakeOverageCharger{}
	s := NewOverageSettler(usage, ledger, charger, true)

	c := starterCandidate(750)
	c.Currency = domain.CurrencyUSD
	if err := s.Settle(context.Background(), c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(charger.calls) != 1 || charger.calls[0].currency != "usd" {
		t.Fatalf("expected one usd charge, got %+v", charger.calls)
	}
}

func TestOverageSettler_SecondSettleDoesNotDoubleCharge(t *testing.T) {
	// Settling twice with an unchanged candidate must not bill twice —
	// the second pass reads the charged ledger row and finds delta 0.
	// This is the crash-after-record case: even if the display cache
	// never updated, the ledger drives the delta.
	usage := &fakeOverageUsageRepo{}
	ledger := newFakeLedger()
	charger := &fakeOverageCharger{}
	s := NewOverageSettler(usage, ledger, charger, true)
	ctx := context.Background()

	if err := s.Settle(ctx, starterCandidate(750)); err != nil {
		t.Fatalf("first settle: %v", err)
	}
	if err := s.Settle(ctx, starterCandidate(750)); err != nil {
		t.Fatalf("second settle: %v", err)
	}
	if len(charger.calls) != 1 {
		t.Fatalf("expected exactly one charge across two settles, got %d", len(charger.calls))
	}
}

func TestOverageSettler_BillsIncrementalDelta(t *testing.T) {
	// First settle bills 50 over (3500¢); the candidate then grows to
	// 100 over (7000¢) and the second settle bills only the 3500¢ delta.
	usage := &fakeOverageUsageRepo{}
	ledger := newFakeLedger()
	charger := &fakeOverageCharger{}
	s := NewOverageSettler(usage, ledger, charger, true)
	ctx := context.Background()

	if err := s.Settle(ctx, starterCandidate(750)); err != nil {
		t.Fatalf("first settle: %v", err)
	}
	if err := s.Settle(ctx, starterCandidate(800)); err != nil {
		t.Fatalf("second settle: %v", err)
	}
	if len(charger.calls) != 2 {
		t.Fatalf("expected two charges, got %d", len(charger.calls))
	}
	if charger.calls[0].amount != 3500 || charger.calls[1].amount != 3500 {
		t.Fatalf("expected two 3500¢ deltas, got %d and %d", charger.calls[0].amount, charger.calls[1].amount)
	}
}

func TestOverageSettler_ClampsToCap(t *testing.T) {
	// 2000 conversations → 1300 over → 91000¢ raw, clamped to 79800¢.
	usage := &fakeOverageUsageRepo{}
	ledger := newFakeLedger()
	charger := &fakeOverageCharger{}
	s := NewOverageSettler(usage, ledger, charger, true)

	if err := s.Settle(context.Background(), starterCandidate(2000)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(charger.calls) != 1 || charger.calls[0].amount != 79800 {
		t.Fatalf("expected single charge clamped to 79800¢, got %+v", charger.calls)
	}
}

func TestOverageSettler_StripeFailureLeavesRowPending(t *testing.T) {
	// A Stripe failure must leave the journalled row `pending` so the
	// next tick resumes it — never lost, never advanced.
	usage := &fakeOverageUsageRepo{}
	ledger := newFakeLedger()
	charger := &fakeOverageCharger{err: errors.New("stripe down")}
	s := NewOverageSettler(usage, ledger, charger, true)

	if err := s.Settle(context.Background(), starterCandidate(750)); err == nil {
		t.Fatal("expected an error when Stripe fails")
	}
	if ledger.pendingCount() != 1 {
		t.Fatalf("expected one pending row left behind, got %d", ledger.pendingCount())
	}
}

func TestOverageSettler_ResumesYoungPendingCharge(t *testing.T) {
	// A pending charge from a crash 1h ago is completed (re-sent to
	// Stripe with the row id as the idempotency key, then marked
	// charged) — within Stripe's 24h dedupe window.
	usage := &fakeOverageUsageRepo{}
	ledger := newFakeLedger()
	charger := &fakeOverageCharger{}
	now := time.Now()
	ledger.seedPending("chg_stuck", "acc_1", 3500, now.Add(-1*time.Hour))
	s := NewOverageSettler(usage, ledger, charger, true).
		WithClock(func() time.Time { return now })

	// candidate convs 750 → overage 3500¢, fully covered once the
	// resumed row is charged, so no extra charge is created.
	if err := s.Settle(context.Background(), starterCandidate(750)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(charger.calls) != 1 || charger.calls[0].idemKey != "chg_stuck" {
		t.Fatalf("expected the pending row resumed with its id as the key, got %+v", charger.calls)
	}
	if ledger.statusOf("chg_stuck") != domain.OverageChargeCharged {
		t.Fatalf("expected resumed row charged, got %q", ledger.statusOf("chg_stuck"))
	}
}

func TestOverageSettler_EscalatesStalePendingCharge(t *testing.T) {
	// A pending charge older than the 24h idempotency window must NOT
	// be retried (Stripe would double-bill) — it is escalated to
	// needs_review and charging stops until a human reconciles.
	usage := &fakeOverageUsageRepo{}
	ledger := newFakeLedger()
	charger := &fakeOverageCharger{}
	now := time.Now()
	ledger.seedPending("chg_stale", "acc_1", 3500, now.Add(-25*time.Hour))
	s := NewOverageSettler(usage, ledger, charger, true).
		WithClock(func() time.Time { return now })

	if err := s.Settle(context.Background(), starterCandidate(2000)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(charger.calls) != 0 {
		t.Fatalf("expected NO charge for a stale pending row, got %d", len(charger.calls))
	}
	if ledger.statusOf("chg_stale") != domain.OverageChargeNeedsReview {
		t.Fatalf("expected needs_review, got %q", ledger.statusOf("chg_stale"))
	}
}

func TestOverageSettler_StripeDisabledRecordsLedgerWithoutCharging(t *testing.T) {
	usage := &fakeOverageUsageRepo{}
	ledger := newFakeLedger()
	charger := &fakeOverageCharger{}
	s := NewOverageSettler(usage, ledger, charger, false)

	if err := s.Settle(context.Background(), starterCandidate(750)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(charger.calls) != 0 {
		t.Fatalf("stripe disabled — expected no charge, got %d", len(charger.calls))
	}
	for _, c := range ledger.charges {
		if c.Status != domain.OverageChargeCharged {
			t.Fatalf("expected the ledger row charged, got %q", c.Status)
		}
	}
}

func TestOverageSettler_CloseCycleArchivesChargedRows(t *testing.T) {
	usage := &fakeOverageUsageRepo{}
	ledger := newFakeLedger()
	charger := &fakeOverageCharger{}
	s := NewOverageSettler(usage, ledger, charger, true)
	ctx := context.Background()

	if err := s.Settle(ctx, starterCandidate(750)); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if err := s.CloseCycle(ctx, "acc_1"); err != nil {
		t.Fatalf("close cycle: %v", err)
	}
	// After the cycle closes the charged total resets, so the same
	// usage in the next cycle is billed again.
	if err := s.Settle(ctx, starterCandidate(750)); err != nil {
		t.Fatalf("next-cycle settle: %v", err)
	}
	if len(charger.calls) != 2 {
		t.Fatalf("expected a fresh charge after the cycle closed, got %d", len(charger.calls))
	}
}

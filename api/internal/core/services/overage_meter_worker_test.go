package services

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
)

// fakes (fakeOverageUsageRepo / fakeOverageCharger / fakeLedger /
// starterCandidate) are defined in overage_settler_test.go — same package.

func TestOverageMeterWorker_SettlesEveryCandidate(t *testing.T) {
	c1 := starterCandidate(750)
	c2 := starterCandidate(800)
	c2.AccountID = "acc_2"
	c2.StripeCustomerID = "cus_2"
	usage := &fakeOverageUsageRepo{candidates: []domain.OverageCandidate{c1, c2}}
	ledger := newFakeLedger()
	charger := &fakeOverageCharger{}
	w := NewOverageMeterWorker(usage, NewOverageSettler(usage, ledger, charger, true))

	w.tick(context.Background())

	if len(charger.calls) != 2 {
		t.Fatalf("expected both candidates billed, got %d charges", len(charger.calls))
	}
}

func TestOverageMeterWorker_NoCandidatesIsNoOp(t *testing.T) {
	usage := &fakeOverageUsageRepo{candidates: nil}
	ledger := newFakeLedger()
	charger := &fakeOverageCharger{}
	w := NewOverageMeterWorker(usage, NewOverageSettler(usage, ledger, charger, true))

	w.tick(context.Background())

	if len(charger.calls) != 0 {
		t.Fatalf("expected no-op, got %d charges", len(charger.calls))
	}
}

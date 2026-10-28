package domain

import (
	"strings"
	"testing"
	"time"
)

// This arithmetic decides what a real person is paid at the end of the month.
// Every rounding decision and every clamp below is tested because the first
// time a payout is wrong, the customer stops trusting every number on the
// screen.

func saleOf(gross, cost, tax int64) *Sale {
	return &Sale{GrossCents: gross, CostCents: cost, TaxCents: tax}
}

func flatRule(base CommissionBaseKind, rate float64) *CommissionRule {
	return &CommissionRule{
		Name: "Padrão", Scope: ScopeGlobal, Base: base,
		RatePercent: rate, Active: true,
	}
}

// R$10.000 gross, R$7.200 cost, R$1.400 tax. The distribution shape the whole
// module exists for.
func TestCommissionBase_GrossNetMargin(t *testing.T) {
	sale := saleOf(1000000, 720000, 140000)

	gross := CommissionBaseCents(flatRule(BaseGross, 5), sale)
	if gross != 1000000 {
		t.Fatalf("gross base = %d", gross)
	}

	margin := CommissionBaseCents(flatRule(BaseMargin, 5), sale)
	if margin != 280000 {
		t.Fatalf("margin base = %d, want gross - cost", margin)
	}

	rule := flatRule(BaseNet, 5)
	rule.Deductions = CommissionDeductions{Cost: true, Tax: true}
	if net := CommissionBaseCents(rule, sale); net != 140000 {
		t.Fatalf("net base = %d, want gross - cost - tax", net)
	}
}

// The whole reason the product nudges towards margin: 5% of gross is R$500 on
// a deal that only made R$1.400 before tax.
func TestCommission_GrossCanEatTheMargin(t *testing.T) {
	sale := saleOf(1000000, 720000, 140000)

	onGross := CalculateCommission(flatRule(BaseGross, 5), CommissionInput{Sale: sale})
	onMargin := CalculateCommission(flatRule(BaseMargin, 5), CommissionInput{Sale: sale})

	if onGross.AmountCents != 50000 {
		t.Fatalf("5%% of gross = %d", onGross.AmountCents)
	}
	if onMargin.AmountCents != 14000 {
		t.Fatalf("5%% of margin = %d", onMargin.AmountCents)
	}
}

// Truncation always favours the company, on every single sale.
func TestCommission_RoundsToTheNearestCent(t *testing.T) {
	// 3.33% of R$100,01 = 333,3333 cents → 333, not 333 truncated from 333.33
	// (same here); the sharper case is 1.5% of 3333 = 49.995 → 50.
	if got := percentOfCents(3333, 1.5); got != 50 {
		t.Fatalf("percentOfCents(3333, 1.5) = %d, want 50 (rounded, not 49)", got)
	}
}

// Docking somebody's pay for a discount their manager approved is not a
// commission rule, it is a bug.
func TestCommission_NegativeBasePaysNothingAndWarns(t *testing.T) {
	sale := saleOf(100000, 120000, 0)

	calc := CalculateCommission(flatRule(BaseMargin, 5), CommissionInput{Sale: sale})

	if calc.AmountCents != 0 {
		t.Fatalf("a loss-making sale paid %d", calc.AmountCents)
	}
	if calc.Warning == "" {
		t.Fatal("a sale that lost money must be flagged, not silently zeroed")
	}
}

func TestCommission_TiersPayTheReachedTier(t *testing.T) {
	rule := flatRule(BaseGross, 0)
	rule.Tiers = []CommissionTier{
		{FromCents: 0, RatePercent: 2},
		{FromCents: 5000000, RatePercent: 3},
		{FromCents: 10000000, RatePercent: 5},
	}

	// Below the first step.
	if r := rule.RateFor(1000000, false); r != 2 {
		t.Fatalf("rate below the ladder = %v", r)
	}
	// Exactly on a step counts as reached — "a partir de R$50k" includes
	// R$50k to everybody who says it out loud.
	if r := rule.RateFor(5000000, false); r != 3 {
		t.Fatalf("rate at the step = %v", r)
	}
	if r := rule.RateFor(12000000, false); r != 5 {
		t.Fatalf("top tier = %v", r)
	}
}

// Unsorted tiers are what a UI produces when somebody adds a step out of
// order.
func TestCommission_TiersDoNotDependOnInputOrder(t *testing.T) {
	rule := flatRule(BaseGross, 0)
	rule.Tiers = []CommissionTier{
		{FromCents: 10000000, RatePercent: 5},
		{FromCents: 0, RatePercent: 2},
		{FromCents: 5000000, RatePercent: 3},
	}

	if r := rule.RateFor(6000000, false); r != 3 {
		t.Fatalf("unsorted tiers gave %v", r)
	}
}

// "Menos para recompra" means a different rate, not a different rung of the
// same ladder.
func TestCommission_RepurchaseRateReplacesTheLadder(t *testing.T) {
	half := 1.0
	rule := flatRule(BaseGross, 0)
	rule.Tiers = []CommissionTier{{FromCents: 0, RatePercent: 5}}
	rule.RepurchaseRatePercent = &half

	if r := rule.RateFor(99999999, true); r != 1 {
		t.Fatalf("repurchase rate = %v, want the override", r)
	}
	if r := rule.RateFor(99999999, false); r != 5 {
		t.Fatalf("a new sale must still climb the ladder, got %v", r)
	}
}

// Nil is "same as a new sale"; zero is "we pay nothing on repurchase". A
// company that means the second has to say so.
func TestCommission_NilRepurchaseRateIsNotZero(t *testing.T) {
	rule := flatRule(BaseGross, 5)

	if r := rule.RateFor(0, true); r != 5 {
		t.Fatalf("nil repurchase rate = %v, want the normal rate", r)
	}

	zero := 0.0
	rule.RepurchaseRatePercent = &zero
	if r := rule.RateFor(0, true); r != 0 {
		t.Fatalf("an explicit zero = %v", r)
	}
}

// Hitting R$100k after R$50k is one achievement with two names.
func TestCommission_OnlyTheLargestMilestonePays(t *testing.T) {
	rule := flatRule(BaseGross, 1)
	rule.Milestones = []CommissionMilestone{
		{TargetCents: 5000000, BonusCents: 50000},
		{TargetCents: 10000000, BonusCents: 150000},
	}

	calc := CalculateCommission(rule, CommissionInput{
		Sale: saleOf(1000000, 0, 0), PeriodTotalCents: 12000000,
	})
	if calc.MilestoneCents != 150000 {
		t.Fatalf("milestone = %d, want only the largest reached", calc.MilestoneCents)
	}
}

func TestCommission_MilestoneDoesNotPayTwiceInAPeriod(t *testing.T) {
	rule := flatRule(BaseGross, 1)
	rule.Milestones = []CommissionMilestone{{TargetCents: 100, BonusCents: 50000}}

	calc := CalculateCommission(rule, CommissionInput{
		Sale: saleOf(1000000, 0, 0), PeriodTotalCents: 12000000,
		MilestoneAlreadyPaid: true,
	})
	if calc.MilestoneCents != 0 {
		t.Fatalf("a milestone already paid must not pay again, got %d", calc.MilestoneCents)
	}
}

// The parts must always add up to the whole, even at 33.33 × 3.
func TestCommission_SplitsAlwaysSumToTheTotal(t *testing.T) {
	rule := flatRule(BaseGross, 10)
	rule.Splits = []CommissionSplit{
		{Role: "closer", SharePercent: 33.33},
		{Role: "quem indicou", SharePercent: 33.33},
		{Role: "gerente", SharePercent: 33.34},
	}

	calc := CalculateCommission(rule, CommissionInput{Sale: saleOf(100001, 0, 0)})

	var sum int64
	for _, s := range calc.Shares {
		sum += s.AmountCents
	}
	if sum != calc.TotalCents {
		t.Fatalf("shares sum to %d but the commission is %d", sum, calc.TotalCents)
	}
}

func TestCommissionRule_SplitsMustBeWhole(t *testing.T) {
	rule := flatRule(BaseGross, 10)
	rule.Splits = []CommissionSplit{
		{Role: "closer", SharePercent: 60},
		{Role: "quem indicou", SharePercent: 30},
	}

	// 90% quietly loses money somebody earned.
	if err := rule.Validate(); err == nil {
		t.Fatal("splits that do not total 100% must be refused")
	}

	rule.Splits[1].SharePercent = 40
	if err := rule.Validate(); err != nil {
		t.Fatalf("100%% of splits must be accepted: %v", err)
	}
}

// A decimal point in the wrong place, caught here rather than in a payout.
func TestCommissionRule_RateIsBounded(t *testing.T) {
	rule := flatRule(BaseGross, 500)
	if err := rule.Validate(); err == nil {
		t.Fatal("500% must be refused")
	}
}

func TestCommissionRule_ScopeNeedsATarget(t *testing.T) {
	rule := flatRule(BaseGross, 5)
	rule.Scope = ScopeSeller
	if err := rule.Validate(); err == nil {
		t.Fatal("a seller-scoped rule with no seller must be refused")
	}

	rule.ScopeID = "user1"
	if err := rule.Validate(); err != nil {
		t.Fatalf("a targeted rule must be accepted: %v", err)
	}
}

func TestCommissionRule_GlobalRuleHasNoTarget(t *testing.T) {
	rule := flatRule(BaseGross, 5)
	rule.ScopeID = "user1"
	if err := rule.Validate(); err == nil {
		t.Fatal("a global rule pointing at somebody must be refused")
	}
}

// A payout that depends on which row the database returned first is a payout
// nobody can defend.
func TestPickCommissionRule_MostSpecificWins(t *testing.T) {
	global := &CommissionRule{ID: "g", Scope: ScopeGlobal, Active: true}
	byProduct := &CommissionRule{ID: "p", Scope: ScopeProduct, ScopeID: "prod1", Active: true}
	bySeller := &CommissionRule{ID: "s", Scope: ScopeSeller, ScopeID: "user1", Active: true}

	got := PickCommissionRule([]*CommissionRule{global, byProduct, bySeller},
		"user1", "", []string{"prod1"})
	if got == nil || got.ID != "s" {
		t.Fatalf("expected the seller rule, got %+v", got)
	}

	got = PickCommissionRule([]*CommissionRule{global, byProduct},
		"user2", "", []string{"prod1"})
	if got == nil || got.ID != "p" {
		t.Fatalf("expected the product rule, got %+v", got)
	}
}

func TestPickCommissionRule_TieGoesToTheNewest(t *testing.T) {
	older := &CommissionRule{ID: "old", Scope: ScopeSeller, ScopeID: "u1",
		Active: true, CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	newer := &CommissionRule{ID: "new", Scope: ScopeSeller, ScopeID: "u1",
		Active: true, CreatedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)}

	got := PickCommissionRule([]*CommissionRule{newer, older}, "u1", "", nil)
	if got.ID != "new" {
		t.Fatalf("expected the newest, got %s", got.ID)
	}
	// And the same answer whichever order they arrive in.
	got = PickCommissionRule([]*CommissionRule{older, newer}, "u1", "", nil)
	if got.ID != "new" {
		t.Fatalf("order changed the answer: got %s", got.ID)
	}
}

func TestPickCommissionRule_InactiveRulesAreIgnored(t *testing.T) {
	off := &CommissionRule{ID: "s", Scope: ScopeSeller, ScopeID: "u1", Active: false}
	global := &CommissionRule{ID: "g", Scope: ScopeGlobal, Active: true}

	got := PickCommissionRule([]*CommissionRule{off, global}, "u1", "", nil)
	if got.ID != "g" {
		t.Fatalf("an inactive rule was applied: %s", got.ID)
	}
}

func TestPickCommissionRule_NoMatchIsNil(t *testing.T) {
	byTeam := &CommissionRule{ID: "t", Scope: ScopeTeam, ScopeID: "team1", Active: true}

	if got := PickCommissionRule([]*CommissionRule{byTeam}, "u1", "team2", nil); got != nil {
		t.Fatalf("expected no rule, got %s", got.ID)
	}
}

// A simulator with its own arithmetic lies exactly when the rule is
// complicated enough for somebody to need it.
func TestSimulateCommission_UsesTheSameEngine(t *testing.T) {
	rule := flatRule(BaseMargin, 10)

	sim := SimulateCommission(rule, 1000000, 30, 4)

	// 30% margin on R$10.000 = R$3.000; 10% of that = R$300 per sale.
	if sim.PerSaleCents != 30000 {
		t.Fatalf("per sale = %d", sim.PerSaleCents)
	}
	if sim.MonthlyCents != 120000 {
		t.Fatalf("monthly = %d, want four sales", sim.MonthlyCents)
	}
	// The number an owner actually needs: 3% of the month's gross.
	if sim.EffectiveRatePercent != 3 {
		t.Fatalf("effective rate = %v", sim.EffectiveRatePercent)
	}
}

// The bonus is paid once a month, not once a sale.
func TestSimulateCommission_MilestonePaysOncePerMonth(t *testing.T) {
	rule := flatRule(BaseGross, 1)
	rule.Milestones = []CommissionMilestone{{TargetCents: 1000000, BonusCents: 50000}}

	sim := SimulateCommission(rule, 1000000, 30, 4)

	// 1% × R$10.000 × 4 sales = R$400, plus one R$500 bonus.
	if sim.MonthlyCents != 40000+50000 {
		t.Fatalf("monthly = %d", sim.MonthlyCents)
	}
}

// The explanation is what settles an argument about a payout.
func TestCommission_ExplainsItself(t *testing.T) {
	rule := flatRule(BaseMargin, 5)
	calc := CalculateCommission(rule, CommissionInput{Sale: saleOf(1000000, 720000, 140000)})

	if !strings.Contains(calc.Explanation, "margem") {
		t.Fatalf("the explanation must name the base: %q", calc.Explanation)
	}
	if !strings.Contains(calc.Explanation, "R$ 2.800,00") {
		t.Fatalf("the explanation must show the base amount: %q", calc.Explanation)
	}
	if !strings.Contains(calc.Explanation, "5%") {
		t.Fatalf("the explanation must show the rate: %q", calc.Explanation)
	}
}

func TestFormatBRL(t *testing.T) {
	cases := map[int64]string{
		0:         "R$ 0,00",
		5:         "R$ 0,05",
		100:       "R$ 1,00",
		123456:    "R$ 1.234,56",
		100000000: "R$ 1.000.000,00",
		-2550:     "-R$ 25,50",
	}
	for cents, want := range cases {
		if got := formatBRL(cents); got != want {
			t.Errorf("formatBRL(%d) = %q, want %q", cents, got, want)
		}
	}
}

// No rule means no commission, and saying so beats a silent zero.
func TestCommission_NoRuleExplainsItself(t *testing.T) {
	calc := CalculateCommission(nil, CommissionInput{Sale: saleOf(1000, 0, 0)})

	if calc.TotalCents != 0 || calc.Explanation == "" {
		t.Fatalf("expected an explained zero, got %+v", calc)
	}
}

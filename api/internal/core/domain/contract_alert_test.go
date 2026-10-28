package domain

import (
	"strings"
	"testing"
	"time"
)

// The red-flag derivation (§6.11, G5).
//
// Deterministic from confirmed fields, so these tests are the whole guarantee
// that the list cannot invent a clause. The two properties that matter: nothing
// is derived from an unconfirmed reading, and a number only appears when the data
// behind it is whole.

func confirmedContract(mutate func(*Contract)) *Contract {
	at := time.Date(2026, time.January, 10, 0, 0, 0, 0, time.UTC)
	c := &Contract{
		ID: "c1", Title: "Fornecimento de luvas", ValueCents: 10000000,
		ConfirmedAt: &at, ConfirmedByUserID: "u1",
	}
	if mutate != nil {
		mutate(c)
	}
	return c
}

func flagOf(flags []ContractRedFlag, kind ContractAlertKind) *ContractRedFlag {
	for i := range flags {
		if flags[i].Kind == kind {
			return &flags[i]
		}
	}
	return nil
}

// The gate: a red flag is a claim about somebody's agreement.
func TestDeriveRedFlags_NothingFromAnUnconfirmedReading(t *testing.T) {
	now := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	unconfirmed := &Contract{
		Title: "Fornecimento", AutoRenewal: true, NoticeDays: 60,
		EndsOn: date(2026, time.September, 30), Exclusivity: true,
	}
	if flags := DeriveRedFlags(unconfirmed, nil, now); len(flags) != 0 {
		t.Fatalf("an unconfirmed reading must produce no flags, got %d — telling a "+
			"customer their contract renews itself from a reading nobody checked is "+
			"the failure this gate exists for", len(flags))
	}
}

func TestDeriveRedFlags_TheRenewalPairIsTheExpensiveOne(t *testing.T) {
	now := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)

	// Ends 30/09/2026 with a 60-day window: the deadline is 01/08/2026 — today.
	c := confirmedContract(func(c *Contract) {
		c.AutoRenewal = true
		c.NoticeDays = 60
		c.EndsOn = date(2026, time.September, 30)
	})
	flag := flagOf(DeriveRedFlags(c, nil, now), AlertRenewalNotice)
	if flag == nil {
		t.Fatal("automatic renewal with a window must be flagged")
	}
	if flag.Severity != FlagWarning {
		t.Fatalf("severity = %s, want warning inside the horizon", flag.Severity)
	}
	if !strings.Contains(flag.Title, "01/08/2026") {
		t.Fatalf("the title must carry the deadline: %q", flag.Title)
	}
	// What acting on it is worth: the contract's whole value.
	if flag.MoneyAtStakeCents != 10000000 {
		t.Fatalf("stake = %d, want the contract value", flag.MoneyAtStakeCents)
	}
	if !flag.Schedulable() {
		t.Fatal("a dated flag must be schedulable")
	}

	// Past the window the FACT changes: it is not a deadline any more, the
	// contract already renewed. Saying "avise até" about a date that passed is
	// worse than saying nothing.
	late := DeriveRedFlags(c, nil, time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC))
	lateFlag := flagOf(late, AlertRenewalNotice)
	if lateFlag == nil || lateFlag.Severity != FlagCritical {
		t.Fatalf("a lapsed window is critical: %+v", lateFlag)
	}
	if !strings.Contains(lateFlag.Title, "já renovou") {
		t.Fatalf("the title must say it already renewed: %q", lateFlag.Title)
	}

	// Renews itself with NO stated window: the absence is the flag, and it cannot
	// be scheduled because there is no date to schedule.
	noWindow := confirmedContract(func(c *Contract) {
		c.AutoRenewal = true
		c.EndsOn = date(2027, time.March, 31)
	})
	absent := flagOf(DeriveRedFlags(noWindow, nil, now), AlertRenewalNotice)
	if absent == nil || absent.Schedulable() {
		t.Fatalf("a missing window is a flag with no date: %+v", absent)
	}
}

func TestDeriveRedFlags_AdjustmentOnlyClaimsAWholeSeries(t *testing.T) {
	now := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	march := 3
	c := confirmedContract(func(c *Contract) {
		c.AdjustmentIndex = "IPCA"
		c.AdjustmentMonth = &march
		c.StartsOn = date(2025, time.March, 1)
	})

	gain := int64(617000)
	suggested := int64(10617000)
	complete := &ContractAdjustment{
		BaseDate: *date(2025, time.March, 1), DueOn: *date(2026, time.March, 1),
		Overdue: true, CurrentValueCents: 10000000,
		SuggestedValueCents: &suggested, GainCents: &gain,
		Accumulation: &IndexAccumulation{
			Code: IndexIPCA, Months: 12, Percent: 6.17, Complete: true,
		},
	}
	flag := flagOf(DeriveRedFlags(c, complete, now), AlertAdjustment)
	if flag == nil || flag.Severity != FlagCritical {
		t.Fatalf("an overdue adjustment is critical: %+v", flag)
	}
	// The sentence has to carry the percentage AND the money, because that is
	// what makes somebody open it.
	if !strings.Contains(flag.Title, "6,17") && !strings.Contains(flag.Title, "6.17") {
		t.Fatalf("the title must carry the accumulated percentage: %q", flag.Title)
	}
	if !strings.Contains(flag.Title, "6.170") {
		t.Fatalf("the title must carry the money on the table: %q", flag.Title)
	}
	if flag.MoneyAtStakeCents != gain {
		t.Fatalf("stake = %d, want the gain %d", flag.MoneyAtStakeCents, gain)
	}

	// Incomplete series: the flag stands (the date is a fact) and carries NO
	// percentage, with the detail saying why.
	partial := &ContractAdjustment{
		BaseDate: *date(2025, time.March, 1), DueOn: *date(2026, time.March, 1),
		Overdue: true, CurrentValueCents: 10000000,
		Accumulation: &IndexAccumulation{
			Code: IndexIPCA, Months: 10, Percent: 5.11, Complete: false,
			MissingMonths: []string{"2025-12", "2026-01"},
		},
	}
	partialFlag := flagOf(DeriveRedFlags(c, partial, now), AlertAdjustment)
	if partialFlag == nil {
		t.Fatal("the date still fires without a percentage")
	}
	if strings.Contains(partialFlag.Title, "5,11") || strings.Contains(partialFlag.Title, "5.11") {
		t.Fatalf("a partial accumulation must not be quoted: %q", partialFlag.Title)
	}
	if !strings.Contains(partialFlag.Detail, "incompleta") {
		t.Fatalf("the detail must say why there is no percentage: %q", partialFlag.Detail)
	}
	if partialFlag.MoneyAtStakeCents != 0 {
		t.Fatalf("no gain to claim: %d", partialFlag.MoneyAtStakeCents)
	}
}

func TestDeriveRedFlags_RegistryBalanceAndExpiry(t *testing.T) {
	now := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)

	c := confirmedContract(func(c *Contract) {
		c.EndsOn = date(2026, time.September, 15)
		c.PriceRegistry = true
		c.RegistryBalanceCents = 8000000
		c.RegistryBalanceOn = date(2026, time.July, 20)
	})
	flags := DeriveRedFlags(c, nil, now)

	expiry := flagOf(flags, AlertExpiry)
	if expiry == nil || !strings.Contains(expiry.Title, "45 dias") {
		t.Fatalf("the term ending must be flagged with its countdown: %+v", expiry)
	}

	registry := flagOf(flags, AlertRegistryBalance)
	if registry == nil {
		t.Fatal("an ata with balance left is pre-approved revenue on the table")
	}
	if registry.MoneyAtStakeCents != 8000000 {
		t.Fatalf("stake = %d, want the balance", registry.MoneyAtStakeCents)
	}
	// The date the balance was known is part of the fact: an old balance is a
	// number to re-check, not one to plan with.
	if !strings.Contains(registry.Title, "20/07/2026") {
		t.Fatalf("the title must date the balance: %q", registry.Title)
	}

	// A contract that ends far away produces no expiry flag: a list where
	// everything is urgent is a list nobody reads.
	far := confirmedContract(func(c *Contract) { c.EndsOn = date(2028, time.January, 1) })
	if flagOf(DeriveRedFlags(far, nil, now), AlertExpiry) != nil {
		t.Fatal("a term ending in two years is not an attention item")
	}
}

func TestDeriveRedFlags_OnlyPenaltiesWorthReading(t *testing.T) {
	now := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)

	c := confirmedContract(func(c *Contract) {
		c.Penalties = []ContractPenalty{
			// Standard late fee: not flagged. Flagging every 2% clause teaches
			// people to skip the list.
			{Description: "juros de mora", Percent: 2},
			{Description: "atraso na entrega", Percent: 10, Trigger: "atraso acima de 5 dias"},
			{Description: "rescisão antecipada", AmountCents: 2000000},
		}
		c.Exclusivity = true
		c.ExclusivityText = "exclusividade na região sul"
	})
	flags := DeriveRedFlags(c, nil, now)

	var penalties int
	for _, f := range flags {
		if f.Kind == AlertPenalty {
			penalties++
			// A penalty has no date and must not be schedulable: a task with no
			// deadline is a task nobody closes.
			if f.Schedulable() {
				t.Fatalf("a penalty carries no deadline: %+v", f)
			}
		}
	}
	if penalties != 2 {
		t.Fatalf("flagged %d penalties, want 2 (the 2%% late fee is standard)", penalties)
	}

	// The percentage penalty's stake is what it would cost on this contract.
	for _, f := range flags {
		if f.Kind == AlertPenalty && strings.Contains(f.Title, "10") {
			if f.MoneyAtStakeCents != 1000000 {
				t.Fatalf("10%% of R$100.000 = R$10.000, got %d", f.MoneyAtStakeCents)
			}
		}
	}

	exclusivity := flagOf(flags, AlertExclusivity)
	if exclusivity == nil || exclusivity.Schedulable() {
		t.Fatalf("exclusivity belongs on the list with no date: %+v", exclusivity)
	}
}

func TestPlainSummary_ComposedFromTheConfirmedFields(t *testing.T) {
	march := 3
	c := confirmedContract(func(c *Contract) {
		c.Parties = []ContractParty{{Name: "Hospital São Lucas"}, {Name: "MedDistribui"}}
		c.Subject = "Fornecimento mensal de EPI."
		c.PaymentTerms = "30 dias"
		c.StartsOn = date(2026, time.April, 1)
		c.EndsOn = date(2027, time.March, 31)
		c.AutoRenewal = true
		c.NoticeDays = 60
		c.AdjustmentIndex = "IPCA"
		c.AdjustmentMonth = &march
		c.SLAText = "entrega em 48h"
	})

	joined := strings.Join(PlainSummary(c), " ")
	for _, want := range []string{
		"Hospital São Lucas", "MedDistribui", "Fornecimento mensal",
		"R$ 100.000,00", "30 dias", "01/04/2026", "31/03/2027",
		"60 dias", "IPCA", "entrega em 48h",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("summary missing %q: %s", want, joined)
		}
	}
	// Nothing is asked of a model a second time, so the summary cannot disagree
	// with the fields — an empty contract summarises to nothing rather than to
	// invented prose.
	if len(PlainSummary(&Contract{})) != 0 {
		t.Fatal("an empty contract has nothing to say")
	}
}

func TestContractAlert_CountdownGoesNegative(t *testing.T) {
	now := time.Date(2026, time.August, 10, 15, 0, 0, 0, time.UTC)
	a := &ContractAlert{Status: AlertPending, DueOn: *date(2026, time.August, 1)}

	if got := a.DaysUntilDue(now); got != -9 {
		t.Fatalf("days = %d, want -9 — a lapsed deadline must not read as zero", got)
	}
	if !a.Overdue(now) {
		t.Fatal("a pending alert past its date is overdue")
	}
	// A closed alert is not overdue, whatever its date: it was dealt with.
	done := &ContractAlert{Status: AlertDone, DueOn: *date(2026, time.August, 1)}
	if done.Overdue(now) {
		t.Fatal("a completed alert is not overdue")
	}
}

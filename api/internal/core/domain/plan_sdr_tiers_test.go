package domain

import "testing"

// US-M013. The tier caps are what a customer buys, so the rules that
// resolve them have to hold in the two cases that have bitten before: a
// contract plan storing 0 (which must NOT read as "not allowed"), and a
// legacy plan with no SDR columns (which must not silently block a feature
// the customer already had).

func sdrPlan(leads, enrich, mailboxes, funnels int) *Plan {
	return &Plan{
		ID:                     "01PLAN_SDR_EQUIPE_00000",
		Name:                   "Equipe",
		StripePriceID:          strPtr("price_equipe"),
		Currency:               "brl",
		Price:                  149700,
		MaxLeadsPerMonth:       leads,
		MaxEnrichmentsPerMonth: enrich,
		MaxEmailMailboxes:      mailboxes,
		MaxFunnels:             funnels,
	}
}

func TestSDRTierCaps_ResolveFromThePlan(t *testing.T) {
	acc := &Account{ID: "acc1"}
	plan := sdrPlan(500, 500, 4, 999)

	if got := acc.EffectiveMaxLeadsPerMonth(plan); got != 500 {
		t.Errorf("leads: got %d, want 500", got)
	}
	if got := acc.EffectiveMaxEnrichmentsPerMonth(plan); got != 500 {
		t.Errorf("enrichments: got %d, want 500", got)
	}
	if got := acc.EffectiveMaxEmailMailboxes(plan); got != 4 {
		t.Errorf("mailboxes: got %d, want 4", got)
	}
	if got := acc.EffectiveMaxFunnels(plan); got != 999 {
		t.Errorf("funnels: got %d, want 999", got)
	}
}

// A contract plan stores 0 and negotiates per deal. Reading that as "zero
// allowed" is the footgun that once capped enterprise customers below the
// trial floor — aqui cai no teto de plataforma, que é finito de
// propósito: nada é ilimitado, e a tela do cliente não pode dizer que é.
func TestSDRTierCaps_ContractPlanFallsBackToCeiling(t *testing.T) {
	acc := &Account{ID: "acc1"}
	contract := &Plan{ID: "01PLAN_ENTERPRISE_00000", Name: "Enterprise", Currency: "brl"}
	if !acc.IsContractPlan(contract) {
		t.Fatal("a plan with no Stripe price is a contract plan")
	}

	for name, c := range map[string]struct{ got, want int }{
		"leads":       {acc.EffectiveMaxLeadsPerMonth(contract), CeilingLeadsPerMonth},
		"enrichments": {acc.EffectiveMaxEnrichmentsPerMonth(contract), CeilingEnrichmentsPerMonth},
		"mailboxes":   {acc.EffectiveMaxEmailMailboxes(contract), CeilingEmailMailboxes},
		"funnels":     {acc.EffectiveMaxFunnels(contract), CeilingFunnels},
	} {
		if c.got != c.want {
			t.Errorf("%s: got %d, want %d (teto de plataforma)", name, c.got, c.want)
		}
	}
}

// A self-serve plan predating the SDR columns reads 0. Callers treat that
// as "no cap configured" and skip enforcement — the alternative would
// silently disable prospecting for every legacy account.
func TestSDRTierCaps_LegacySelfServePlanHasNoCap(t *testing.T) {
	acc := &Account{ID: "acc1"}
	legacy := &Plan{
		ID:            "01PLAN_BRONZE_000000000",
		Name:          "Starter",
		StripePriceID: strPtr("price_starter"),
		Currency:      "brl",
	}
	if got := acc.EffectiveMaxLeadsPerMonth(legacy); got != 0 {
		t.Errorf("legacy leads cap: got %d, want 0 (meaning 'no cap configured')", got)
	}
	if got := acc.EffectiveMaxEmailMailboxes(legacy); got != 0 {
		t.Errorf("legacy mailbox cap: got %d, want 0", got)
	}
}

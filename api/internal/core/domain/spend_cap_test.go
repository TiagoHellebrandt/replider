package domain

import (
	"testing"
	"time"
)

// Per-plan spend caps (K4).
//
// What matters here is the accounting, not the arithmetic: which provider calls
// count against a customer's allowance, and whose month the cycle belongs to.

func TestSpendOperations_OurRetriesAreNotTheCustomersProblem(t *testing.T) {
	ops := SpendOperations(SpendAIDraft)

	for _, op := range ops {
		if op == "approach_draft_retry" {
			t.Fatal("a retry is OUR repair of a malformed model response — charging it " +
				"against the customer's allowance bills them for our bug")
		}
	}
	if len(ops) != 1 || ops[0] != "approach_draft" {
		t.Fatalf("draft operations = %v, want just the draft itself", ops)
	}
}

func TestSpendOperations_TextAndVisionAreOneDocument(t *testing.T) {
	ops := SpendOperations(SpendDocumentExtraction)

	// The pipeline takes one route or the other, never both, so counting both
	// contract operations is one unit per document — not two. The brochure
	// read (product PDF import) is the same unit and shares the allowance.
	want := map[string]bool{
		"contract_extract":        true,
		"contract_extract_vision": true,
		"product_pdf_extract":     true,
	}
	if len(ops) != len(want) {
		t.Fatalf("operations = %v, want the three document-read routes", ops)
	}
	for _, op := range ops {
		if !want[op] {
			t.Fatalf("unexpected operation %q", op)
		}
	}
}

func TestSpendOperations_AnUnknownKindCountsNothing(t *testing.T) {
	// An unknown kind must not silently map onto some other kind's operations,
	// which would charge one feature's calls to another's allowance.
	if ops := SpendOperations(SpendKind("whatever")); len(ops) != 0 {
		t.Fatalf("operations = %v, want none", ops)
	}
}

func TestSpendCapFor_GoesThroughTheEffectiveAccessors(t *testing.T) {
	// A self-serve plan (it has a Stripe price) means the column IS the answer,
	// and 0 means "not included".
	priceID := "price_123"
	plan := &Plan{Name: "Equipe", StripePriceID: &priceID, MaxAIDraftsPerMonth: 1000}
	account := &Account{}

	if got := SpendCapFor(SpendAIDraft, account, plan); got != 1000 {
		t.Fatalf("cap = %d, want 1000", got)
	}
	if got := SpendCapFor(SpendDocumentExtraction, account, plan); got != 0 {
		t.Fatalf("cap = %d, want 0 (column unset on a self-serve plan)", got)
	}
	// Nil inputs resolve to zero rather than panicking on a worker path.
	if got := SpendCapFor(SpendAIDraft, nil, nil); got != 0 {
		t.Fatalf("cap = %d, want 0", got)
	}
}

func TestSpendCapFor_AZeroOnAContractPlanIsTheCeiling(t *testing.T) {
	// `IsContractPlan` é "não tem preço na Stripe", o que os planos de SDR
	// satisfazem (vêm com price id NULL). Uma coluna em 0 ali quer dizer
	// "combina-se por negócio", não "bloqueado" — por isso a migração 000184
	// semeia um número real em todo plano. O que mudou é o valor do caso não
	// negociado: era infinito, e infinito virava "ilimitado" na tela do
	// cliente. Agora é o teto de plataforma: alto, e finito.
	plan := &Plan{Name: "Enterprise"} // no Stripe price → contract plan
	account := &Account{}

	if got := SpendCapFor(SpendAIDraft, account, plan); got != CeilingAIDraftsPerMonth {
		t.Fatalf("cap = %d, want %d — uma coluna 0 em plano sob contrato é "+
			"espaço para o override, não recusa", got, CeilingAIDraftsPerMonth)
	}
}

func TestSpendErrors_NotIncludedAndUsedUpAreDifferentConversations(t *testing.T) {
	notAllowed := ErrSpendNotAllowed(SpendAIDraft)
	exceeded := ErrSpendExceeded(SpendAIDraft, 1000, 1000,
		time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))

	// One is an upgrade, the other is "wait for the cycle". A single code would
	// make the client guess which.
	if notAllowed.Code == exceeded.Code {
		t.Fatalf("both refusals share the code %q", notAllowed.Code)
	}
	// The numbers travel in the message: "limite excedido" makes somebody open
	// a support thread to find out by how much.
	for _, want := range []string{"1000", "01/08/2026"} {
		if !contains(exceeded.Message, want) {
			t.Fatalf("message %q missing %q", exceeded.Message, want)
		}
	}
}

func TestSpendErrors_EachKindHasItsOwnCode(t *testing.T) {
	kinds := []SpendKind{SpendAIDraft, SpendDocumentExtraction}
	seen := map[string]bool{}
	for _, kind := range kinds {
		for _, code := range []string{SpendNotAllowedCode(kind), SpendExceededCode(kind)} {
			if seen[code] {
				t.Fatalf("code %q reused across kinds", code)
			}
			seen[code] = true
		}
	}
}

func TestSpendWindow_IsTheAccountsMonth(t *testing.T) {
	// 02:30 UTC on the 1st is still 22:30 on the LAST day of the previous month
	// in São Paulo. An account told "renova dia 1" has to see it renew on THEIR
	// first.
	now := time.Date(2026, 8, 1, 2, 30, 0, 0, time.UTC)

	from, resetsAt := SpendWindow(now, "America/Sao_Paulo")

	if from.Month() != time.July {
		t.Fatalf("window starts in %v, want July (the account is still in July)", from.Month())
	}
	if from.Day() != 1 || from.Hour() != 0 {
		t.Fatalf("window starts %v, want local midnight of the 1st", from)
	}
	if resetsAt.Month() != time.August || resetsAt.Day() != 1 {
		t.Fatalf("resets %v, want the 1st of the next month", resetsAt)
	}
}

func TestSpendWindow_UnknownTimezoneDegradesRatherThanGuessing(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)

	from, _ := SpendWindow(now, "Mars/Olympus_Mons")

	// A typo in one account's settings must not break the cap for everybody;
	// UTC is the documented fallback, and guessing a country would be the same
	// bug as hardcoding one.
	if from.Location() != time.UTC {
		t.Fatalf("location = %v, want UTC", from.Location())
	}
	if from.Day() != 1 || from.Month() != time.August {
		t.Fatalf("window starts %v, want 1 August", from)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

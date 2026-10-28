package domain

import (
	"testing"
	"time"
)

// Contract dates and role redaction (§6.11, G4).
//
// Two things carry money here: the notice deadline (miss it and the contract
// renews itself) and what a given role is allowed to read.

func date(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

func TestNoticeDeadline_OnlyWhenThePaperSaysSo(t *testing.T) {
	renewing := &Contract{
		EndsOn: date(2027, time.March, 31), AutoRenewal: true, NoticeDays: 60,
	}
	deadline := renewing.NoticeDeadline()
	if deadline == nil {
		t.Fatal("a renewing contract with a notice window has a deadline")
	}
	if got := deadline.Format("2006-01-02"); got != "2027-01-30" {
		t.Fatalf("deadline = %s, want 2027-01-30", got)
	}

	// No automatic renewal, no window, or no end date: nil. Inventing a date
	// would put a task on somebody's list on a day the paper never mentions.
	for name, c := range map[string]*Contract{
		"no auto renewal": {EndsOn: date(2027, time.March, 31), NoticeDays: 60},
		"no window":       {EndsOn: date(2027, time.March, 31), AutoRenewal: true},
		"no end date":     {AutoRenewal: true, NoticeDays: 60},
	} {
		if c.NoticeDeadline() != nil {
			t.Fatalf("%s: expected no deadline", name)
		}
	}
}

func TestDaysUntilEnd_GoesNegativeAfterExpiry(t *testing.T) {
	now := time.Date(2026, time.August, 1, 15, 0, 0, 0, time.UTC)

	future := (&Contract{EndsOn: date(2026, time.August, 31)}).DaysUntilEnd(now)
	if future == nil || *future != 30 {
		t.Fatalf("days until end = %v, want 30", future)
	}
	// Already expired is the state that costs money; it must not read as zero.
	past := (&Contract{EndsOn: date(2026, time.July, 1)}).DaysUntilEnd(now)
	if past == nil || *past != -31 {
		t.Fatalf("expired days = %v, want -31", past)
	}
	// No end date is unknown, not "today".
	if (&Contract{}).DaysUntilEnd(now) != nil {
		t.Fatal("a contract with no end date has no countdown")
	}
}

func TestContractValidate(t *testing.T) {
	valid := &Contract{Title: "Fornecimento de luvas"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid contract rejected: %v", err)
	}

	month := 13
	cases := map[string]*Contract{
		"no title":           {},
		"negative value":     {Title: "x", ValueCents: -1},
		"absurd notice":      {Title: "x", NoticeDays: MaxContractNoticeDays + 1},
		"bad adjust month":   {Title: "x", AdjustmentMonth: &month},
		"ends before starts": {Title: "x", StartsOn: date(2027, time.March, 1), EndsOn: date(2026, time.March, 1)},
		"penalty over 100":   {Title: "x", Penalties: []ContractPenalty{{Description: "multa", Percent: 150}}},
		"negative penalty":   {Title: "x", Penalties: []ContractPenalty{{Description: "multa", AmountCents: -100}}},
	}
	for name, c := range cases {
		if err := c.Validate(); err == nil {
			t.Fatalf("%s: expected refusal", name)
		}
	}
}

// The redaction is the authorization. A client deciding what to hide is one
// devtools tab away from showing everything.
func TestRedactForRole_RemovesTheCommercialTermsForTheFloor(t *testing.T) {
	full := &Contract{
		Title: "Fornecimento", ValueCents: 50000000, PaymentTerms: "30 dias",
		SLAText: "atendimento em 24h", EndsOn: date(2027, time.March, 31),
		Penalties:       []ContractPenalty{{Description: "atraso", Percent: 10}},
		ExclusivityText: "exclusividade na região sul", Exclusivity: true,
		Guarantees: "fiança bancária", Termination: "aviso de 90 dias",
		Notes:      "renegociar em janeiro",
		AIProposal: map[string]any{"value_cents": 50000000},
		Amendments: []*Contract{{Title: "Aditivo 1", ValueCents: 1000000}},
	}

	for _, role := range []string{"owner", "admin", "gestor"} {
		if got := full.RedactForRole(role); got.ValueCents != 50000000 {
			t.Fatalf("%s must read the value", role)
		}
	}

	for _, role := range []string{"vendedor", "attendant", "", "nonsense"} {
		redacted := full.RedactForRole(role)
		if !redacted.Redacted {
			t.Fatalf("%s: the response must admit it hid something — a value of 0 "+
				"would read as 'this contract is free'", role)
		}
		if redacted.ValueCents != 0 || redacted.PaymentTerms != "" ||
			len(redacted.Penalties) != 0 || redacted.ExclusivityText != "" ||
			redacted.Guarantees != "" || redacted.Termination != "" ||
			redacted.Notes != "" {
			t.Fatalf("%s: commercial terms leaked: %+v", role, redacted)
		}
		// The proposal carries every field that was just removed — handing it
		// over would undo the redaction in one line.
		if redacted.AIProposal != nil {
			t.Fatalf("%s: the AI proposal leaked the redacted fields", role)
		}
		// What serving the customer needs stays: the term and the SLA.
		if redacted.SLAText == "" || redacted.EndsOn == nil {
			t.Fatalf("%s: term and SLA must remain readable", role)
		}
		// Exclusivity as a FACT stays (a salesperson must not promise a
		// competitor's region); the clause text does not.
		if !redacted.Exclusivity {
			t.Fatalf("%s: the exclusivity flag must remain", role)
		}
		if len(redacted.Amendments) != 1 || redacted.Amendments[0].ValueCents != 0 {
			t.Fatalf("%s: amendments must be redacted too, got %+v", role, redacted.Amendments)
		}
	}

	// The original is untouched: redaction returns a copy, so the same entity
	// can be served to two roles in one request.
	if full.ValueCents != 50000000 || full.AIProposal == nil {
		t.Fatal("redaction must not mutate the source contract")
	}
}

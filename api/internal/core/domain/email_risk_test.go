package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The dor these checks exist for: B2B healthcare is a constant target for
// invoice fraud. Someone impersonates a supplier, sends a boleto with new
// bank details, and the money leaves. The tests are written as the cases a
// salesperson would recognise.

var testNow = time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)

func daysAgo(n int) *time.Time {
	t := testNow.Add(-time.Duration(n) * 24 * time.Hour)
	return &t
}

func kinds(a EmailRiskAssessment) map[EmailRiskKind]string {
	out := map[EmailRiskKind]string{}
	for _, s := range a.Signals {
		out[s.Kind] = s.Detail
	}
	return out
}

// The message this whole layer exists to catch.
func TestAssessRisk_ImpersonatedSupplierIsQuarantined(t *testing.T) {
	got := AssessEmailRisk(EmailRiskInput{
		FromAddress:        "financeiro@medl1ne.com.br",
		Auth:               InboundAuthVerdict{Suspicious: true},
		KnownDomains:       []string{"medline.com.br"},
		DomainRegisteredAt: daysAgo(3),
		Now:                testNow,
	})

	k := kinds(got)
	assert.Contains(t, k, RiskAuthFailed)
	assert.Contains(t, k, RiskLookalikeDomain)
	assert.Contains(t, k, RiskNewDomain)
	assert.True(t, got.Quarantined(), "score was %d", got.Score)
	// The evidence is what makes it actionable in ten seconds.
	assert.Contains(t, k[RiskLookalikeDomain], "medline.com.br")
	assert.Contains(t, k[RiskNewDomain], "3 dias")
}

// The opposite failure, and the one that kills a warning system: a normal
// cold reply must stay quiet. Prospecting means strangers writing back.
func TestAssessRisk_OrdinaryColdReplyIsNotQuarantined(t *testing.T) {
	got := AssessEmailRisk(EmailRiskInput{
		FromAddress:  "compras@clinicavidanova.com.br",
		Auth:         InboundAuthVerdict{Trustworthy: true},
		KnownDomains: []string{"medline.com.br"},
		Now:          testNow,
	})

	assert.False(t, got.Quarantined())
	assert.Equal(t, weightFirstContact, got.Score)
}

// An unverified sender from an older provider is most legitimate mail. It
// must never reach the threshold on its own.
func TestAssessRisk_UnverifiedFirstContactStaysQuiet(t *testing.T) {
	got := AssessEmailRisk(EmailRiskInput{
		FromAddress: "dr@hospital.com.br",
		Auth:        InboundAuthVerdict{}, // nobody checked
		Now:         testNow,
	})

	assert.False(t, got.Quarantined(), "score was %d", got.Score)
	assert.Contains(t, kinds(got), RiskAuthUnknown)
}

// No single weak signal may quarantine anything. If one ever can, the whole
// system becomes noise and stops being read.
func TestAssessRisk_NoWeakSignalAloneReachesTheThreshold(t *testing.T) {
	cases := map[string]EmailRiskInput{
		"unverified":    {Auth: InboundAuthVerdict{}},
		"first_contact": {Auth: InboundAuthVerdict{Trustworthy: true}},
		"reply_to": {
			FromAddress: "a@fornecedor.com.br",
			ReplyTo:     "b@outro.com.br",
			Auth:        InboundAuthVerdict{Trustworthy: true},
			SeenBefore:  true,
		},
		"new_domain": {
			FromAddress:        "a@novo.com.br",
			Auth:               InboundAuthVerdict{Trustworthy: true},
			DomainRegisteredAt: daysAgo(2),
			SeenBefore:         true,
		},
	}
	for name, in := range cases {
		in.Now = testNow
		got := AssessEmailRisk(in)
		assert.False(t, got.Quarantined(), "%s alone must not quarantine (score %d)", name, got.Score)
	}
}

// A failed authentication plus anything else does.
func TestAssessRisk_AuthFailurePlusOneMoreQuarantines(t *testing.T) {
	got := AssessEmailRisk(EmailRiskInput{
		FromAddress:        "a@fornecedor.com.br",
		Auth:               InboundAuthVerdict{Suspicious: true},
		DomainRegisteredAt: daysAgo(5),
		SeenBefore:         true,
		Now:                testNow,
	})

	assert.True(t, got.Quarantined(), "score was %d", got.Score)
}

func TestAssessRisk_ReplyToMismatchIsReported(t *testing.T) {
	got := AssessEmailRisk(EmailRiskInput{
		FromAddress: "compras@clinica.com.br",
		ReplyTo:     "financeiro@gmail.com",
		Auth:        InboundAuthVerdict{Trustworthy: true},
		SeenBefore:  true,
		Now:         testNow,
	})

	k := kinds(got)
	require.Contains(t, k, RiskReplyToMismatch)
	assert.Contains(t, k[RiskReplyToMismatch], "gmail.com")
}

// Same domain in Reply-To is the normal case (mailing lists, aliases).
func TestAssessRisk_SameDomainReplyToIsNotAMismatch(t *testing.T) {
	got := AssessEmailRisk(EmailRiskInput{
		FromAddress: "compras@clinica.com.br",
		ReplyTo:     "vendas@clinica.com.br",
		Auth:        InboundAuthVerdict{Trustworthy: true},
		SeenBefore:  true,
		Now:         testNow,
	})

	assert.NotContains(t, kinds(got), RiskReplyToMismatch)
	assert.True(t, got.Clean())
}

// A lookup that did not happen must not read as "old and trusted".
func TestAssessRisk_UnknownDomainAgeIsNotAFlagAndNotAPass(t *testing.T) {
	got := AssessEmailRisk(EmailRiskInput{
		FromAddress:        "a@qualquer.com.br",
		Auth:               InboundAuthVerdict{Trustworthy: true},
		DomainRegisteredAt: nil,
		SeenBefore:         true,
		Now:                testNow,
	})

	assert.NotContains(t, kinds(got), RiskNewDomain)
	assert.True(t, got.Clean())
}

func TestAssessRisk_OldDomainIsNotFlagged(t *testing.T) {
	got := AssessEmailRisk(EmailRiskInput{
		FromAddress:        "a@antiga.com.br",
		Auth:               InboundAuthVerdict{Trustworthy: true},
		DomainRegisteredAt: daysAgo(4000),
		SeenBefore:         true,
		Now:                testNow,
	})

	assert.NotContains(t, kinds(got), RiskNewDomain)
}

// ── Lookalike detection ───────────────────────────────────────────────────

func TestFindLookalike_CatchesTheClassicSwaps(t *testing.T) {
	known := []string{"medline.com.br", "philips.com", "clinicavidanova.com.br"}
	cases := []string{
		"medl1ne.com.br", // 1 → l
		"rnedline.com",   // rn → m
		"medlline.com",   // doubled letter
		"medlne.com",     // dropped letter
		"philps.com",     // dropped letter
	}
	for _, c := range cases {
		_, ok := FindLookalikeDomain(c, known)
		assert.True(t, ok, "must be caught: %q", c)
	}
}

// The real domain is not an imitation of itself, and neither is its own
// subdomain — flagging those would fire on every legitimate message.
func TestFindLookalike_IgnoresTheRealThing(t *testing.T) {
	known := []string{"medline.com.br"}

	_, exact := FindLookalikeDomain("medline.com.br", known)
	assert.False(t, exact)

	_, sub := FindLookalikeDomain("mail.medline.com.br", known)
	assert.False(t, sub, "a subdomain is not an impersonation of its parent")
}

// A genuinely different company must not be accused. This is the expensive
// false positive: telling someone their real supplier is a scam.
func TestFindLookalike_DoesNotAccuseUnrelatedDomains(t *testing.T) {
	known := []string{"medline.com.br", "philips.com"}
	for _, c := range []string{
		"clinicavidanova.com.br",
		"hospitalsaolucas.com.br",
		"gmail.com",
		"siemens-healthineers.com",
	} {
		_, ok := FindLookalikeDomain(c, known)
		assert.False(t, ok, "must not be flagged: %q", c)
	}
}

// Short labels are one edit from everything; flagging them is pure noise.
func TestFindLookalike_SkipsShortLabels(t *testing.T) {
	_, ok := FindLookalikeDomain("abc.com", []string{"abd.com"})
	assert.False(t, ok)
}

func TestFindLookalike_EmptyKnownListFindsNothing(t *testing.T) {
	_, ok := FindLookalikeDomain("medl1ne.com.br", nil)
	assert.False(t, ok)
}

func TestDomainOf(t *testing.T) {
	assert.Equal(t, "clinica.com.br", DomainOf("Compras@Clinica.com.br"))
	assert.Equal(t, "", DomainOf("sem-arroba"))
	assert.Equal(t, "", DomainOf("termina@"))
	assert.Equal(t, "", DomainOf(""))
}

// The golpe do boleto, end to end through the rules: a supplier you have paid
// before, writing from their real verified address, asking for money at a new
// account. Every other check passes. This one has to be enough on its own,
// because it is the last moment anyone can stop the transfer.
func TestAssessRisk_ChangedPaymentDetailsAloneQuarantines(t *testing.T) {
	got := AssessEmailRisk(EmailRiskInput{
		FromAddress: "financeiro@medline.com.br",
		Auth:        InboundAuthVerdict{Trustworthy: true},
		SeenBefore:  true,
		ChangedPayments: []PaymentDetail{
			{Kind: PaymentBankAccount, Value: "9999/1112223", Raw: "Ag 9999 C/C 111222-3"},
		},
		Now: testNow,
	})

	assert.True(t, got.Quarantined(), "score was %d", got.Score)
	k := kinds(got)
	require.Contains(t, k, RiskChangedPaymentDetails)
	// The evidence has to be the account itself: that is what the reader
	// checks against the invoice on their desk.
	assert.Contains(t, k[RiskChangedPaymentDetails], "Ag 9999 C/C 111222-3")
}

// No history means no alarm. A first invoice from a new supplier must not be
// flagged, or the alert fires on every new relationship.
func TestAssessRisk_NoPaymentChangeIsNoSignal(t *testing.T) {
	got := AssessEmailRisk(EmailRiskInput{
		FromAddress:     "financeiro@medline.com.br",
		Auth:            InboundAuthVerdict{Trustworthy: true},
		SeenBefore:      true,
		ChangedPayments: nil,
		Now:             testNow,
	})

	assert.True(t, got.Clean())
}

// The signal no single account could produce. Two independent businesses
// reporting the same domain is not a coincidence.
func TestAssessRisk_NetworkReportsAloneQuarantine(t *testing.T) {
	got := AssessEmailRisk(EmailRiskInput{
		FromAddress:    "contato@golpista.com.br",
		Auth:           InboundAuthVerdict{Trustworthy: true},
		SeenBefore:     true,
		NetworkReports: NetworkScamThreshold,
		Now:            testNow,
	})

	assert.True(t, got.Quarantined(), "score was %d", got.Score)
	assert.Contains(t, kinds(got), RiskNetworkReported)
}

// One report is a single company's judgement, and can be a mistake, a grudge
// or a misunderstanding. It must not quarantine anyone else's mail.
func TestAssessRisk_ASingleReportIsNotEnoughForTheNetwork(t *testing.T) {
	got := AssessEmailRisk(EmailRiskInput{
		FromAddress:    "contato@disputado.com.br",
		Auth:           InboundAuthVerdict{Trustworthy: true},
		SeenBefore:     true,
		NetworkReports: 1,
		Now:            testNow,
	})

	assert.NotContains(t, kinds(got), RiskNetworkReported)
	assert.True(t, got.Clean())
}

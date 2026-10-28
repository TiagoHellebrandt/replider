package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Writing into someone else's DNS zone is the most destructive thing this
// product does. These tests are about not breaking mail that already works.

// THE test. A domain may have only ONE SPF record, and a customer already
// sending through Google Workspace, an ERP and an NF-e provider has all of
// them in it. Replacing that record stops every one of those systems from
// delivering — silently.
func TestMergeSPF_KeepsEverythingThatWasAlreadyThere(t *testing.T) {
	existing := "v=spf1 include:_spf.google.com include:spf.nfe.com.br ~all"

	merged, changed := MergeSPF(existing, "_spf.replider.com")

	assert.True(t, changed)
	assert.Contains(t, merged, "include:_spf.google.com")
	assert.Contains(t, merged, "include:spf.nfe.com.br")
	assert.Contains(t, merged, "include:_spf.replider.com")
}

// SPF evaluation stops at the first match, so an include placed AFTER the
// terminal mechanism is never reached — a record that looks right in the
// panel and authorises nothing.
func TestMergeSPF_PutsTheIncludeBeforeAll(t *testing.T) {
	merged, _ := MergeSPF("v=spf1 include:_spf.google.com ~all", "_spf.replider.com")

	assert.Equal(t,
		"v=spf1 include:_spf.google.com include:_spf.replider.com ~all", merged)
}

func TestMergeSPF_HandlesEveryAllQualifier(t *testing.T) {
	for _, all := range []string{"~all", "-all", "?all", "+all", "all"} {
		merged, _ := MergeSPF("v=spf1 a "+all, "x.com")
		assert.Equal(t, "v=spf1 a include:x.com "+all, merged, "qualifier %q", all)
	}
}

// Running twice must not produce two of our includes.
func TestMergeSPF_IsIdempotent(t *testing.T) {
	once, _ := MergeSPF("v=spf1 ~all", "x.com")
	twice, changed := MergeSPF(once, "x.com")

	assert.False(t, changed)
	assert.Equal(t, once, twice)
}

func TestMergeSPF_MatchesTheIncludeCaseInsensitively(t *testing.T) {
	_, changed := MergeSPF("v=spf1 include:_SPF.Replider.com ~all", "_spf.replider.com")

	assert.False(t, changed, "DNS is case-insensitive; a second copy would be a bug")
}

func TestMergeSPF_WritesAFreshRecordWhenThereIsNone(t *testing.T) {
	merged, changed := MergeSPF("", "x.com")

	assert.True(t, changed)
	assert.Equal(t, "v=spf1 include:x.com ~all", merged)
}

// A TXT record at the root that is not SPF (a domain-verification token, say)
// must be left completely alone — appending SPF syntax to it would corrupt
// whatever it proves.
func TestMergeSPF_RefusesToTouchANonSPFRecord(t *testing.T) {
	existing := "google-site-verification=abc123"

	merged, changed := MergeSPF(existing, "x.com")

	assert.False(t, changed)
	assert.Equal(t, existing, merged)
}

// ── The lookup ceiling ────────────────────────────────────────────────────

// Over ten DNS lookups an SPF record is a PERMERROR, which most receivers
// treat as no SPF at all — so one include too many can break a domain that
// was working. Counted so we can WARN; never used to refuse, because the
// count cannot see inside a nested include.
func TestCountSPFLookups_CountsTheMechanismsThatCost(t *testing.T) {
	rec := "v=spf1 a mx include:a.com include:b.com exists:%{i}.x.com ip4:1.2.3.4 ~all"

	// a, mx, two includes, exists — ip4 costs nothing.
	assert.Equal(t, 5, CountSPFLookups(rec))
}

func TestCountSPFLookups_IPMechanismsAreFree(t *testing.T) {
	assert.Equal(t, 0, CountSPFLookups("v=spf1 ip4:1.2.3.4 ip6:::1 -all"))
}

// ── Zone resolution ───────────────────────────────────────────────────────

// A sending domain is often a subdomain while the zone is the registrable
// domain. Looking up only the exact name finds nothing and tells the
// customer their domain is not in the account — which is wrong, and sends
// them hunting for a zone that is right there.
func TestZoneCandidates_WalksUpToTheRegistrableDomain(t *testing.T) {
	assert.Equal(t,
		[]string{"mail.clinica.com.br", "clinica.com.br", "com.br"},
		ZoneCandidates("mail.clinica.com.br"))
}

func TestZoneCandidates_StopsAtTwoLabels(t *testing.T) {
	assert.Equal(t, []string{"clinica.com"}, ZoneCandidates("clinica.com"))
	// A single label is not a zone.
	assert.Empty(t, ZoneCandidates("localhost"))
}

func TestZoneCandidates_NormalisesTheInput(t *testing.T) {
	assert.Equal(t, []string{"clinica.com.br", "com.br"},
		ZoneCandidates("  Clinica.COM.BR.  "))
}

// ── The report ────────────────────────────────────────────────────────────

// A partial run is still progress: three of four published is worth showing
// record by record, not hiding behind one error that sends the user back to
// doing all four by hand.
func TestDNSProvisionReport_FailedIsPerRecord(t *testing.T) {
	r := DNSProvisionReport{Results: []DNSProvisionResult{
		{Kind: DNSKindSPF, Action: DNSActionMerged},
		{Kind: DNSKindDKIM, Action: DNSActionCreated},
		{Kind: DNSKindDMARC, Action: DNSActionFailed},
	}}

	assert.True(t, r.Failed())
}

func TestDNSProvisionReport_KeptIsNotAFailure(t *testing.T) {
	r := DNSProvisionReport{Results: []DNSProvisionResult{
		{Kind: DNSKindDMARC, Action: DNSActionKept},
	}}

	assert.False(t, r.Failed())
}

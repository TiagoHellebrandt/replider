package services

import (
	"context"
	"errors"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-M006. Gmail and Outlook reject unauthenticated bulk mail, and the
// failure is invisible from our side — the SMTP handoff succeeds and the
// message simply never arrives. These tests pin the two things that decide
// whether the customer ever gets past setup: that a CORRECT record is
// recognized even when it does not look like ours, and that a propagating
// record is never called broken.

type fakeResolver struct {
	txt   map[string][]string
	cname map[string]string
	ns    []string
	err   error
}

func (f *fakeResolver) LookupTXT(_ context.Context, name string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	v, ok := f.txt[name]
	if !ok {
		return nil, errors.New("NXDOMAIN")
	}
	return v, nil
}

func (f *fakeResolver) LookupCNAME(_ context.Context, name string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	v, ok := f.cname[name]
	if !ok {
		return "", errors.New("NXDOMAIN")
	}
	return v, nil
}

func (f *fakeResolver) LookupNS(_ context.Context, _ string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.ns, nil
}

func testCfg() domain.EmailSendingConfig {
	return domain.EmailSendingConfig{
		SPFInclude:     "_spf.replider.com",
		DKIMSelector:   "rplr",
		DKIMTarget:     "rplr.dkim.replider.com",
		TrackingTarget: "track.replider.com",
		DMARCReportTo:  "dmarc@replider.com",
	}
}

func recordOf(setup domain.EmailDNSSetup, kind domain.DNSRecordKind) domain.EmailDNSRecord {
	for _, r := range setup.Records {
		if r.Kind == kind {
			return r
		}
	}
	return domain.EmailDNSRecord{}
}

func TestEmailDNS_BuildProducesTheFourRecords(t *testing.T) {
	setup := domain.BuildEmailDNSSetup("Clinica.com.BR ", testCfg())

	assert.Equal(t, "clinica.com.br", setup.Domain)
	require.Len(t, setup.Records, 4)

	spf := recordOf(setup, domain.DNSKindSPF)
	assert.Equal(t, "TXT", spf.Type)
	assert.Equal(t, "@", spf.HostShort, "registrars ask for the short form; the FQDN is the #1 setup mistake")
	// Softfail, not -all: a customer who also sends from their own server
	// would have every one of those messages rejected by a hard fail.
	assert.Equal(t, "v=spf1 include:_spf.replider.com ~all", spf.Value)
	assert.True(t, spf.Required)

	dkim := recordOf(setup, domain.DNSKindDKIM)
	assert.Equal(t, "CNAME", dkim.Type, "CNAME delegation lets us rotate keys without the customer touching DNS again")
	assert.Equal(t, "rplr._domainkey.clinica.com.br", dkim.Host)

	dmarc := recordOf(setup, domain.DNSKindDMARC)
	assert.Equal(t, "_dmarc.clinica.com.br", dmarc.Host)
	// p=none on day one: quarantine/reject would break the customer's OTHER
	// mail flows (ERP, NF-e, forms) that nobody has aligned yet.
	assert.Contains(t, dmarc.Value, "p=none")
	assert.Contains(t, dmarc.Value, "rua=mailto:dmarc@replider.com")

	tracking := recordOf(setup, domain.DNSKindTracking)
	assert.False(t, tracking.Required, "click tracking is a nice-to-have; mail must not be gated on it")
}

func TestEmailDNS_VerifiesPublishedRecords(t *testing.T) {
	res := &fakeResolver{
		txt: map[string][]string{
			"clinica.com.br":        {"v=spf1 include:_spf.replider.com ~all"},
			"_dmarc.clinica.com.br": {"v=DMARC1; p=none; rua=mailto:dmarc@replider.com; fo=1"},
		},
		cname: map[string]string{
			"rplr._domainkey.clinica.com.br": "rplr.dkim.replider.com.", // trailing dot
		},
		ns: []string{"eoin.ns.cloudflare.com.", "eve.ns.cloudflare.com."},
	}
	v := NewEmailDNSVerifier(res, testCfg())

	setup := v.Verify(context.Background(), "clinica.com.br")

	assert.True(t, setup.Verified, "all REQUIRED records are published")
	assert.Equal(t, domain.DNSStatusOK, recordOf(setup, domain.DNSKindSPF).Status)
	assert.Equal(t, domain.DNSStatusOK, recordOf(setup, domain.DNSKindDKIM).Status,
		"the resolver's trailing dot must not read as a mismatch")
	assert.Equal(t, domain.DNSStatusOK, recordOf(setup, domain.DNSKindDMARC).Status)
	// Optional record missing does not block.
	assert.Equal(t, domain.DNSStatusPending, recordOf(setup, domain.DNSKindTracking).Status)
	assert.Equal(t, "cloudflare", setup.NameserverHint)
}

// The audience this product sells to keeps its professional domains on the
// Brazilian hosts, not on Cloudflare — the detection must name them, or the
// guide silently assumes an infrastructure nobody there uses.
func TestEmailDNS_DetectsBrazilianHosts(t *testing.T) {
	cases := []struct {
		ns   string
		slug string
	}{
		{"ns1.locaweb.com.br.", "locaweb"},
		{"a.sec.dns.br.", "registrobr"},
		{"ns1.kinghost.net.", "kinghost"},
		{"ns1.umbler.com.", "umbler"},
		{"ns1.secureserver.net.", "godaddy"},
	}
	for _, c := range cases {
		v := NewEmailDNSVerifier(&fakeResolver{ns: []string{c.ns}}, testCfg())
		setup := v.Verify(context.Background(), "clinica.com.br")
		assert.Equal(t, c.slug, setup.NameserverHint, c.ns)
	}
}

// A customer already sending through another provider has a longer SPF
// record. Reporting it as wrong would tell them to break their existing mail.
func TestEmailDNS_AcceptsAnExistingSPFThatIncludesUs(t *testing.T) {
	res := &fakeResolver{txt: map[string][]string{
		"clinica.com.br": {"v=spf1 include:_spf.google.com include:_spf.replider.com include:sendgrid.net -all"},
	}}
	v := NewEmailDNSVerifier(res, testCfg())

	setup := v.Verify(context.Background(), "clinica.com.br")
	assert.Equal(t, domain.DNSStatusOK, recordOf(setup, domain.DNSKindSPF).Status)
}

// Likewise for DMARC: a customer on p=quarantine is BETTER protected than
// our suggested p=none, and telling them to weaken it would be malpractice.
func TestEmailDNS_AcceptsAStricterDMARCPolicy(t *testing.T) {
	res := &fakeResolver{txt: map[string][]string{
		"_dmarc.clinica.com.br": {"v=DMARC1; p=quarantine; pct=100; rua=mailto:seguranca@clinica.com.br"},
	}}
	v := NewEmailDNSVerifier(res, testCfg())

	setup := v.Verify(context.Background(), "clinica.com.br")
	assert.Equal(t, domain.DNSStatusOK, recordOf(setup, domain.DNSKindDMARC).Status)
}

// Propagation looks exactly like a missing record for the first minutes.
// Calling a correct record broken during that window is how support tickets
// get created.
func TestEmailDNS_MissingRecordIsPendingNotBroken(t *testing.T) {
	v := NewEmailDNSVerifier(&fakeResolver{}, testCfg())
	setup := v.Verify(context.Background(), "clinica.com.br")

	assert.False(t, setup.Verified)
	for _, r := range setup.Records {
		assert.Equal(t, domain.DNSStatusPending, r.Status, string(r.Kind))
	}
	assert.Len(t, setup.PendingRequired(), 3, "the UI names exactly what is missing")
}

// Something published but wrong IS a different situation from nothing
// published — the user needs to know to EDIT, not to create.
func TestEmailDNS_WrongValueIsAMismatch(t *testing.T) {
	res := &fakeResolver{
		txt:   map[string][]string{"clinica.com.br": {"v=spf1 include:outrofornecedor.com ~all"}},
		cname: map[string]string{"rplr._domainkey.clinica.com.br": "algo.errado.com"},
	}
	v := NewEmailDNSVerifier(res, testCfg())
	setup := v.Verify(context.Background(), "clinica.com.br")

	spf := recordOf(setup, domain.DNSKindSPF)
	assert.Equal(t, domain.DNSStatusMismatch, spf.Status)
	assert.Contains(t, spf.Found, "outrofornecedor.com", "the actual value is shown so it is debuggable")
	assert.Equal(t, domain.DNSStatusMismatch, recordOf(setup, domain.DNSKindDKIM).Status)
}

// A total resolver outage must not present as "your DNS is wrong".
func TestEmailDNS_ResolverFailureDegradesToPending(t *testing.T) {
	v := NewEmailDNSVerifier(&fakeResolver{err: errors.New("network unreachable")}, testCfg())
	setup := v.Verify(context.Background(), "clinica.com.br")

	assert.False(t, setup.Verified)
	assert.Equal(t, domain.DNSStatusPending, recordOf(setup, domain.DNSKindSPF).Status)
	assert.Empty(t, setup.NameserverHint)
}

// Setup() must not touch the network: the screen renders instantly and
// verification runs behind it.
func TestEmailDNS_SetupDoesNotResolve(t *testing.T) {
	res := &fakeResolver{err: errors.New("must not be called")}
	setup := NewEmailDNSVerifier(res, testCfg()).Setup("clinica.com.br")

	require.Len(t, setup.Records, 4)
	assert.False(t, setup.Verified)
}

// A domain cannot be "verified" for sending when the platform never told it
// what to publish: with no SPF include and no DKIM target configured, the
// setup holds a lone DMARC record, and treating that as verified would open
// the send caps on a domain that authenticates nothing.
func TestEmailDNS_UnconfiguredPlatformNeverVerifies(t *testing.T) {
	v := NewEmailDNSVerifier(&fakeResolver{
		txt: map[string][]string{
			"_dmarc.clinica.com.br": {"v=DMARC1; p=none; fo=1"},
		},
	}, domain.EmailSendingConfig{})

	setup := v.Verify(context.Background(), "clinica.com.br")
	if setup.Verified {
		t.Fatal("a domain with no SPF/DKIM records must never verify")
	}
}

// ── Per-channel (BYO) verification ──────────────────────────────────────────
//
// The env model assumed a platform relay and its empty-env production state
// held every customer in an unliftable warning. The per-channel build derives
// the records from what the channel actually sends through.

func TestEmailDNSForChannel_SMTPAcceptsAnySPF(t *testing.T) {
	v := NewEmailDNSVerifier(&fakeResolver{
		txt: map[string][]string{
			"clinica.com.br":        {"v=spf1 include:_spf.kinghost.net ~all"},
			"_dmarc.clinica.com.br": {"v=DMARC1; p=quarantine"},
		},
	}, domain.EmailSendingConfig{})

	setup := v.VerifyForChannel(context.Background(), "clinica.com.br", false)
	assert.Equal(t, domain.DNSStatusOK, recordOf(setup, domain.DNSKindSPF).Status,
		"we cannot know the customer's provider include — presence is the check")
	assert.True(t, setup.Verified)
}

func TestEmailDNSForChannel_SMTPWithoutDMARCStaysUnverified(t *testing.T) {
	v := NewEmailDNSVerifier(&fakeResolver{
		txt: map[string][]string{
			"clinica.com.br": {"v=spf1 include:_spf.kinghost.net ~all"},
		},
	}, domain.EmailSendingConfig{})

	setup := v.VerifyForChannel(context.Background(), "clinica.com.br", false)
	assert.False(t, setup.Verified)
	assert.Equal(t, domain.DNSStatusPending, recordOf(setup, domain.DNSKindDMARC).Status)
}

func TestEmailDNSForChannel_GoogleWorkspaceDemandsGoogleInclude(t *testing.T) {
	v := NewEmailDNSVerifier(&fakeResolver{
		txt: map[string][]string{
			"clinica.com.br":        {"v=spf1 include:_spf.kinghost.net ~all"},
			"_dmarc.clinica.com.br": {"v=DMARC1; p=none"},
		},
	}, domain.EmailSendingConfig{})

	setup := v.VerifyForChannel(context.Background(), "clinica.com.br", true)
	assert.Equal(t, domain.DNSStatusMismatch, recordOf(setup, domain.DNSKindSPF).Status,
		"an SPF that authorizes another provider does not authorize Google")
	assert.False(t, setup.Verified)
}

func TestEmailDNSForChannel_GoogleDKIMIsPresenceCheckedAndOptional(t *testing.T) {
	v := NewEmailDNSVerifier(&fakeResolver{
		txt: map[string][]string{
			"clinica.com.br":                   {"v=spf1 include:_spf.google.com ~all"},
			"_dmarc.clinica.com.br":            {"v=DMARC1; p=none"},
			"google._domainkey.clinica.com.br": {"v=DKIM1; k=rsa; p=MIGf..."},
		},
	}, domain.EmailSendingConfig{})

	setup := v.VerifyForChannel(context.Background(), "clinica.com.br", true)
	dkim := recordOf(setup, domain.DNSKindDKIM)
	assert.Equal(t, domain.DNSStatusOK, dkim.Status)
	assert.False(t, dkim.Required, "only the customer's admin console knows the key — recommending, not blocking")
	assert.True(t, setup.Verified)
}

func TestEmailDNSForChannel_PersonalGmailNeedsNothing(t *testing.T) {
	// A resolver that answers NOTHING: gmail.com's own records are Google's
	// business, and no lookup should decide this outcome.
	v := NewEmailDNSVerifier(&fakeResolver{}, domain.EmailSendingConfig{})

	setup := v.VerifyForChannel(context.Background(), "gmail.com", true)
	assert.True(t, setup.ProviderManaged)
	assert.True(t, setup.Verified)
	assert.Empty(t, setup.Records)
}

package dnsprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A fake Cloudflare that remembers what was written, so the tests can assert
// on the ZONE's final state — which is the thing that either works or breaks
// a company's mail.

type fakeCF struct {
	*httptest.Server
	zones map[string]string // name -> id
	// records is keyed "type|name".
	records map[string][]cfDNSRecord
	writes  []string
	token   string
	reject  bool
}

func newFakeCF(t *testing.T) *fakeCF {
	t.Helper()
	f := &fakeCF{
		zones:   map[string]string{"clinica.com.br": "zone1"},
		records: map[string][]cfDNSRecord{},
	}
	mux := http.NewServeMux()

	mux.HandleFunc("/zones", func(w http.ResponseWriter, r *http.Request) {
		f.token = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if f.reject {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		name := r.URL.Query().Get("name")
		out := []map[string]string{}
		if id, ok := f.zones[name]; ok {
			out = append(out, map[string]string{"id": id, "name": name})
		}
		writeCF(w, out)
	})

	mux.HandleFunc("/zones/zone1/dns_records", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			key := r.URL.Query().Get("type") + "|" + r.URL.Query().Get("name")
			writeCF(w, f.records[key])
		case http.MethodPost:
			var body cfDNSRecord
			_ = json.NewDecoder(r.Body).Decode(&body)
			body.ID = "rec-new"
			key := body.Type + "|" + body.Name
			f.records[key] = append(f.records[key], body)
			f.writes = append(f.writes, "create "+key+" = "+body.Content)
			writeCF(w, body)
		}
	})

	mux.HandleFunc("/zones/zone1/dns_records/", func(w http.ResponseWriter, r *http.Request) {
		var body cfDNSRecord
		_ = json.NewDecoder(r.Body).Decode(&body)
		key := body.Type + "|" + body.Name
		f.records[key] = []cfDNSRecord{{ID: "rec1", Type: body.Type, Name: body.Name, Content: body.Content}}
		f.writes = append(f.writes, "update "+key+" = "+body.Content)
		writeCF(w, body)
	})

	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

func writeCF(w http.ResponseWriter, result any) {
	raw, _ := json.Marshal(result)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true, "errors": []any{}, "result": json.RawMessage(raw),
	})
}

func (f *fakeCF) seed(recType, name, content string) {
	key := recType + "|" + name
	f.records[key] = append(f.records[key], cfDNSRecord{
		ID: "rec1", Type: recType, Name: name, Content: content,
	})
}

func (f *fakeCF) content(recType, name string) string {
	recs := f.records[recType+"|"+name]
	if len(recs) == 0 {
		return ""
	}
	return recs[0].Content
}

func testSetup(sendingDomain string) domain.EmailDNSSetup {
	return domain.BuildEmailDNSSetup(sendingDomain, domain.EmailSendingConfig{
		SPFInclude:   "_spf.replider.com",
		DKIMSelector: "rep1",
		DKIMTarget:   "rep1.dkim.replider.com",
	})
}

func apply(t *testing.T, f *fakeCF, sendingDomain string) domain.DNSProvisionReport {
	t.Helper()
	p := NewCloudflareProvisioner().WithBaseURL(f.URL)
	report, err := p.Apply(context.Background(), "cf-token", testSetup(sendingDomain))
	require.NoError(t, err)
	return report
}

func actionFor(r domain.DNSProvisionReport, kind domain.DNSRecordKind) domain.DNSProvisionAction {
	for _, res := range r.Results {
		if res.Kind == kind {
			return res.Action
		}
	}
	return ""
}

func TestCloudflare_PublishesEveryRecordOnAnEmptyZone(t *testing.T) {
	f := newFakeCF(t)

	report := apply(t, f, "clinica.com.br")

	assert.Equal(t, "clinica.com.br", report.Zone)
	assert.False(t, report.Failed())
	assert.Equal(t, domain.DNSActionCreated, actionFor(report, domain.DNSKindSPF))
	assert.Equal(t, domain.DNSActionCreated, actionFor(report, domain.DNSKindDKIM))
	assert.Equal(t, domain.DNSActionCreated, actionFor(report, domain.DNSKindDMARC))
	assert.Contains(t, f.content("TXT", "clinica.com.br"), "include:_spf.replider.com")
	assert.Equal(t, "rep1.dkim.replider.com",
		f.content("CNAME", "rep1._domainkey.clinica.com.br"))
}

// THE test for this file. Replacing an existing SPF record stops every other
// system the customer sends through — Workspace, ERP, NF-e — from
// delivering, silently, with no error anywhere they would look.
func TestCloudflare_MergesIntoAnExistingSPFInsteadOfReplacingIt(t *testing.T) {
	f := newFakeCF(t)
	f.seed("TXT", "clinica.com.br", "v=spf1 include:_spf.google.com include:spf.nfe.com.br ~all")

	report := apply(t, f, "clinica.com.br")

	assert.Equal(t, domain.DNSActionMerged, actionFor(report, domain.DNSKindSPF))
	final := f.content("TXT", "clinica.com.br")
	assert.Contains(t, final, "include:_spf.google.com")
	assert.Contains(t, final, "include:spf.nfe.com.br")
	assert.Contains(t, final, "include:_spf.replider.com")
}

// An existing DMARC policy is left alone. A customer running p=reject is
// strictly better protected than our suggested p=none, and replacing theirs
// with ours would quietly weaken their domain to make our checklist go green.
func TestCloudflare_NeverWeakensAnExistingDMARCPolicy(t *testing.T) {
	f := newFakeCF(t)
	f.seed("TXT", "_dmarc.clinica.com.br", "v=DMARC1; p=reject; rua=mailto:seg@clinica.com.br")

	report := apply(t, f, "clinica.com.br")

	assert.Equal(t, domain.DNSActionKept, actionFor(report, domain.DNSKindDMARC))
	assert.Contains(t, f.content("TXT", "_dmarc.clinica.com.br"), "p=reject")
	for _, w := range f.writes {
		assert.NotContains(t, w, "_dmarc", "the existing policy must not be touched")
	}
}

// Running it twice must not double anything.
func TestCloudflare_IsIdempotent(t *testing.T) {
	f := newFakeCF(t)

	apply(t, f, "clinica.com.br")
	writesAfterFirst := len(f.writes)
	report := apply(t, f, "clinica.com.br")

	assert.Equal(t, writesAfterFirst, len(f.writes), "a second run must write nothing")
	assert.Equal(t, domain.DNSActionKept, actionFor(report, domain.DNSKindSPF))
	assert.Equal(t, domain.DNSActionKept, actionFor(report, domain.DNSKindDKIM))
}

// A stale DKIM CNAME pointing at an old key IS ours to replace — unlike SPF
// and DMARC, nothing else in the customer's world lives under that name.
func TestCloudflare_ReplacesAStaleDKIMTarget(t *testing.T) {
	f := newFakeCF(t)
	f.seed("CNAME", "rep1._domainkey.clinica.com.br", "old.dkim.replider.com")

	report := apply(t, f, "clinica.com.br")

	assert.Equal(t, domain.DNSActionUpdated, actionFor(report, domain.DNSKindDKIM))
	assert.Equal(t, "rep1.dkim.replider.com",
		f.content("CNAME", "rep1._domainkey.clinica.com.br"))
}

// The sending domain is often a subdomain while the zone is the registrable
// domain. Looking up only the exact name would tell the customer their
// domain is not in the account, which is wrong.
func TestCloudflare_FindsTheZoneForASubdomain(t *testing.T) {
	f := newFakeCF(t)

	report := apply(t, f, "mail.clinica.com.br")

	assert.Equal(t, "clinica.com.br", report.Zone)
	assert.Contains(t, f.content("TXT", "mail.clinica.com.br"), "include:_spf.replider.com")
}

func TestCloudflare_RejectedTokenIsOneClearError(t *testing.T) {
	f := newFakeCF(t)
	f.reject = true

	p := NewCloudflareProvisioner().WithBaseURL(f.URL)
	_, err := p.Apply(context.Background(), "bad-token", testSetup("clinica.com.br"))

	assert.ErrorIs(t, err, domain.ErrDNSCredentialInvalid)
}

// A valid token that does not cover this domain is a DIFFERENT mistake from
// an invalid one: the customer picked the wrong account.
func TestCloudflare_UnknownZoneSaysSo(t *testing.T) {
	f := newFakeCF(t)

	p := NewCloudflareProvisioner().WithBaseURL(f.URL)
	_, err := p.Apply(context.Background(), "cf-token", testSetup("outra.com.br"))

	assert.ErrorIs(t, err, domain.ErrDNSZoneNotFound)
}

func TestCloudflare_EmptyTokenNeverReachesTheNetwork(t *testing.T) {
	f := newFakeCF(t)

	p := NewCloudflareProvisioner().WithBaseURL(f.URL)
	_, err := p.Apply(context.Background(), "   ", testSetup("clinica.com.br"))

	assert.ErrorIs(t, err, domain.ErrDNSCredentialInvalid)
	assert.Empty(t, f.writes)
}

package domain

import (
	"context"
	"strings"
)

// One-click DNS setup (§6.9 / C6).
//
// Publishing four records by hand in a registrar's panel is where email
// onboarding dies. The records are correct, the customer pastes them into
// the wrong field (FQDN vs subdomain is the classic), nothing verifies, and
// they give up — with the send caps still closed, so the product does
// nothing for them.
//
// So when we CAN write the records ourselves, we should. What follows is
// deliberately conservative about that power: see MergeSPF and the DMARC
// rule below.

// DNSProvisionAction says what we did to one record, because "done" is not
// a useful report when the interesting outcomes are "left yours alone" and
// "merged ours into yours".
type DNSProvisionAction string

const (
	DNSActionCreated DNSProvisionAction = "created"
	DNSActionUpdated DNSProvisionAction = "updated"
	// DNSActionMerged means an existing record was kept and our part added
	// to it — only ever SPF.
	DNSActionMerged DNSProvisionAction = "merged"
	// DNSActionKept means we found something valid and did NOT touch it.
	DNSActionKept   DNSProvisionAction = "kept"
	DNSActionFailed DNSProvisionAction = "failed"
)

// DNSProvisionResult is one record's outcome.
type DNSProvisionResult struct {
	Kind   DNSRecordKind      `json:"kind"`
	Host   string             `json:"host"`
	Action DNSProvisionAction `json:"action"`
	// Detail explains a kept or failed record in the user's terms.
	Detail string `json:"detail,omitempty"`
}

// DNSProvisionReport is what the screen shows after a one-click run.
type DNSProvisionReport struct {
	Provider string               `json:"provider"`
	Zone     string               `json:"zone"`
	Results  []DNSProvisionResult `json:"results"`
}

// Failed reports whether anything could not be written. A partial run is
// still worth reporting record by record — three of four published is real
// progress, and hiding it behind one error would send the user back to
// doing all four by hand.
func (r DNSProvisionReport) Failed() bool {
	for _, res := range r.Results {
		if res.Action == DNSActionFailed {
			return true
		}
	}
	return false
}

// DNSProvisioner writes records into a customer's zone.
type DNSProvisioner interface {
	// Name is the provider slug ("cloudflare"), matching NameserverHint.
	Name() string
	// Apply publishes the setup's records. The credential is passed per
	// call and never persisted — see the handler.
	Apply(ctx context.Context, credential string, setup EmailDNSSetup) (DNSProvisionReport, error)
}

// ErrDNSProviderUnsupported is returned for a zone we cannot write to.
var ErrDNSProviderUnsupported = &AppError{
	Code:    "DNS_PROVIDER_UNSUPPORTED",
	Message: "configuração automática ainda não disponível para este provedor de DNS",
}

// ErrDNSCredentialInvalid covers a rejected token. Deliberately one error
// for "wrong token" and "token without permission": both are fixed the same
// way, by minting a new one with the documented scope.
var ErrDNSCredentialInvalid = &AppError{
	Code:    "DNS_CREDENTIAL_INVALID",
	Message: "o token não foi aceito — confira se ele tem permissão de editar DNS da zona",
}

// ErrDNSZoneNotFound means the token is valid but does not cover this
// domain, which is a different mistake from an invalid token: the customer
// picked the wrong account or the wrong zone.
var ErrDNSZoneNotFound = &AppError{
	Code:    "DNS_ZONE_NOT_FOUND",
	Message: "este domínio não foi encontrado na conta do token informado",
}

// MergeSPF adds our include to an EXISTING SPF record instead of replacing it.
//
// This is the single most destructive thing an automatic DNS setup can get
// wrong. A customer already sending through Google Workspace, an ERP and a
// NF-e provider has all of that in one SPF record — a domain may only have
// ONE — and overwriting it stops every one of those systems from delivering,
// silently, with no error anywhere they would look.
//
// Returns the merged value and whether anything changed.
func MergeSPF(existing, include string) (string, bool) {
	inc := strings.TrimSpace(include)
	if inc == "" {
		return existing, false
	}
	cur := strings.TrimSpace(existing)
	if cur == "" {
		return "v=spf1 include:" + inc + " ~all", true
	}
	if !strings.HasPrefix(strings.ToLower(cur), "v=spf1") {
		return existing, false
	}
	token := "include:" + inc
	if strings.Contains(strings.ToLower(cur), strings.ToLower(token)) {
		return existing, false
	}

	// The include goes BEFORE the terminal `all` mechanism. SPF evaluation
	// stops at the first match, so an include placed after `~all` or `-all`
	// is never reached — a record that looks right and authorises nothing.
	fields := strings.Fields(cur)
	out := make([]string, 0, len(fields)+1)
	inserted := false
	for _, f := range fields {
		if !inserted && isSPFAllMechanism(f) {
			out = append(out, token)
			inserted = true
		}
		out = append(out, f)
	}
	if !inserted {
		out = append(out, token)
	}
	return strings.Join(out, " "), true
}

// isSPFAllMechanism matches the terminal mechanism in any qualifier form.
func isSPFAllMechanism(field string) bool {
	f := strings.ToLower(strings.TrimSpace(field))
	return f == "all" || f == "~all" || f == "-all" || f == "+all" || f == "?all"
}

// SPFLookupLimit is the DNS-lookup ceiling in RFC 7208 §4.6.4. A record that
// exceeds it is a PERMERROR — which most receivers treat as no SPF at all,
// so adding one include too many can break a domain that was working.
const SPFLookupLimit = 10

// CountSPFLookups counts the mechanisms that cost a DNS lookup.
//
// Not exact — a nested include's own lookups are invisible from here — so it
// is used to WARN, never to refuse. Refusing on an estimate would block a
// setup that is actually fine.
func CountSPFLookups(record string) int {
	n := 0
	for _, f := range strings.Fields(record) {
		switch {
		case strings.HasPrefix(strings.ToLower(f), "include:"),
			strings.HasPrefix(strings.ToLower(f), "a:"),
			strings.HasPrefix(strings.ToLower(f), "mx:"),
			strings.HasPrefix(strings.ToLower(f), "ptr:"),
			strings.HasPrefix(strings.ToLower(f), "exists:"),
			strings.HasPrefix(strings.ToLower(f), "redirect="):
			n++
		case strings.EqualFold(f, "a"), strings.EqualFold(f, "mx"):
			n++
		}
	}
	return n
}

// ZoneCandidates lists the zone names to try for a domain, longest first.
//
// A sending domain is often a subdomain (mail.clinica.com.br) while the zone
// is the registrable domain — so an exact-name lookup alone finds nothing
// and the customer is told, wrongly, that their domain is not in the account.
func ZoneCandidates(sendingDomain string) []string {
	d := strings.ToLower(strings.Trim(strings.TrimSpace(sendingDomain), "."))
	if d == "" {
		return nil
	}
	labels := strings.Split(d, ".")
	var out []string
	// Stop at two labels: a single label is not a zone, and going that far
	// would query for a public suffix ("com.br") on the way.
	for i := 0; i+2 <= len(labels); i++ {
		out = append(out, strings.Join(labels[i:], "."))
	}
	return out
}

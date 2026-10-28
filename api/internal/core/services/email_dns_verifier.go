package services

import (
	"context"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// EmailDNSVerifier checks whether a customer published the records from
// US-M006.
//
// Written to be POLLED: the setup screen calls it every few seconds while
// the user is in their registrar's dashboard, so every lookup is bounded and
// a failure to resolve is reported as "pending", never as an error. DNS
// propagation looks exactly like a missing record for the first few minutes,
// and telling the user their correct record is "wrong" during that window is
// how a support ticket gets created.
type EmailDNSVerifier struct {
	resolver domain.DNSResolver
	cfg      domain.EmailSendingConfig
	timeout  time.Duration
}

func NewEmailDNSVerifier(resolver domain.DNSResolver, cfg domain.EmailSendingConfig) *EmailDNSVerifier {
	return &EmailDNSVerifier{resolver: resolver, cfg: cfg, timeout: 5 * time.Second}
}

// Setup returns the records to publish, WITHOUT touching DNS. Instant, so
// the screen renders while verification runs behind it.
func (v *EmailDNSVerifier) Setup(domainName string) domain.EmailDNSSetup {
	return domain.BuildEmailDNSSetup(domainName, v.cfg)
}

// SetupForChannel derives the records from what the channel actually sends
// through (BYO posture) — see domain.BuildEmailDNSSetupForChannel.
func (v *EmailDNSVerifier) SetupForChannel(domainName string, googleMailbox bool) domain.EmailDNSSetup {
	return domain.BuildEmailDNSSetupForChannel(domainName, googleMailbox, v.cfg)
}

// Verify resolves each record and fills in its status.
func (v *EmailDNSVerifier) Verify(ctx context.Context, domainName string) domain.EmailDNSSetup {
	setup := v.Setup(domainName)
	v.verifySetup(ctx, &setup)
	setup.Verified = setup.RequiredRecordsVerified()
	return setup
}

// VerifyForChannel verifies the per-channel (BYO) record set. Verified comes
// from AllRequiredOK: the list is correct by construction here, so the
// env-model's hasSPF/hasDKIM fail-closed guard — which held this screen in
// an eternal, unliftable warning in production — does not apply.
func (v *EmailDNSVerifier) VerifyForChannel(ctx context.Context, domainName string, googleMailbox bool) domain.EmailDNSSetup {
	setup := v.SetupForChannel(domainName, googleMailbox)
	if setup.ProviderManaged {
		return setup
	}
	v.verifySetup(ctx, &setup)
	setup.Verified = setup.AllRequiredOK()
	return setup
}

func (v *EmailDNSVerifier) verifySetup(ctx context.Context, setup *domain.EmailDNSSetup) {
	if v.resolver == nil || len(setup.Records) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	for i := range setup.Records {
		rec := &setup.Records[i]
		switch rec.Type {
		case "TXT":
			v.verifyTXT(ctx, rec)
		case "CNAME":
			v.verifyCNAME(ctx, rec)
		}
	}
	setup.NameserverHint = v.nameserverHint(ctx, setup.Domain)
}

func (v *EmailDNSVerifier) verifyTXT(ctx context.Context, rec *domain.EmailDNSRecord) {
	values, err := v.resolver.LookupTXT(ctx, rec.Host)
	if err != nil || len(values) == 0 {
		// NXDOMAIN and "still propagating" are indistinguishable here, and
		// calling a propagating record broken generates support tickets.
		rec.Status = domain.DNSStatusPending
		return
	}
	rec.Found = strings.Join(values, " | ")

	for _, got := range values {
		switch rec.Kind {
		case domain.DNSKindSPF:
			// Compare by MECHANISM, not by string: a customer who already
			// sends through another provider has a longer, perfectly valid
			// record that must not be reported as wrong. What matters is
			// that the REQUIRED include is in there — and on a MatchAny row
			// (own-SMTP path, where only the customer's provider knows its
			// include) any valid SPF counts.
			if isSPFRecord(got) &&
				(rec.MatchAny || strings.Contains(strings.ToLower(got), strings.ToLower("include:"+spfIncludeOf(rec, v.cfg)))) {
				rec.Status = domain.DNSStatusOK
				return
			}
		case domain.DNSKindDKIM:
			// TXT-form DKIM (Google Workspace's google._domainkey) is
			// presence-checked: the key only exists in the customer's admin
			// console, so a published v=DKIM1 is the whole answer.
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(got)), "v=dkim1") {
				rec.Status = domain.DNSStatusOK
				return
			}
		case domain.DNSKindDMARC:
			// Likewise: any valid DMARC policy counts. A customer who
			// already runs p=quarantine is BETTER off than our suggested
			// p=none, and overwriting it would weaken their security.
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(got)), "v=dmarc1") {
				rec.Status = domain.DNSStatusOK
				return
			}
		default:
			if strings.EqualFold(strings.TrimSpace(got), strings.TrimSpace(rec.Value)) {
				rec.Status = domain.DNSStatusOK
				return
			}
		}
	}

	// Something IS published under this name but it is not what we need —
	// a genuinely different situation from "nothing there yet".
	rec.Status = domain.DNSStatusMismatch
}

func (v *EmailDNSVerifier) verifyCNAME(ctx context.Context, rec *domain.EmailDNSRecord) {
	got, err := v.resolver.LookupCNAME(ctx, rec.Host)
	if err != nil || strings.TrimSpace(got) == "" {
		rec.Status = domain.DNSStatusPending
		return
	}
	rec.Found = got
	if equalHost(got, rec.Value) {
		rec.Status = domain.DNSStatusOK
		return
	}
	rec.Status = domain.DNSStatusMismatch
}

// equalHost compares hostnames ignoring case and the trailing dot resolvers
// add — the single most common false "mismatch".
func equalHost(a, b string) bool {
	norm := func(s string) string {
		return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(s), "."))
	}
	return norm(a) == norm(b)
}

func isSPFRecord(txt string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(txt)), "v=spf1")
}

// spfIncludeOf resolves which include an exact-match SPF row demands: the one
// embedded in the row's own value (per-channel build) or the legacy env one.
func spfIncludeOf(rec *domain.EmailDNSRecord, cfg domain.EmailSendingConfig) string {
	for _, tok := range strings.Fields(rec.Value) {
		if inc, ok := strings.CutPrefix(strings.ToLower(tok), "include:"); ok {
			return inc
		}
	}
	return cfg.SPFInclude
}

// knownNameservers maps a nameserver substring to the provider slug the UI
// uses to pick the right screenshots. The 1-click Cloudflare flow (F2) keys
// off the same detection.
var knownNameservers = []struct {
	match string
	slug  string
}{
	{"cloudflare", "cloudflare"},
	{"godaddy", "godaddy"},
	{"domaincontrol", "godaddy"},
	{"registro.br", "registrobr"},
	{"dns.br", "registrobr"},
	{"hostinger", "hostinger"},
	{"hostgator", "hostgator"},
	{"locaweb", "locaweb"},
	{"uolhost", "uolhost"},
	// GoDaddy's actual production nameservers are nsXX.domaincontrol.com,
	// but resold domains often sit on secureserver.net.
	{"secureserver", "godaddy"},
	// The Brazilian hosts a professional domain actually lives on — the
	// audience this product sells to is not on Cloudflare.
	{"kinghost", "kinghost"},
	{"umbler", "umbler"},
	{"weblink", "weblink"},
	{"redehost", "redehost"},
	{"hostnet", "hostnet"},
	{"wixdns", "wix"},
	{"squarespacedns", "googledomains"},
	{"awsdns", "route53"},
	{"googledomains", "googledomains"},
	{"azure-dns", "azure"},
}

// nameserverHint detects the DNS host so the guide can show the right
// screenshots. Best-effort: an unknown provider just yields generic
// instructions, never an error.
func (v *EmailDNSVerifier) nameserverHint(ctx context.Context, domainName string) string {
	ns, err := v.resolver.LookupNS(ctx, domainName)
	if err != nil || len(ns) == 0 {
		return ""
	}
	joined := strings.ToLower(strings.Join(ns, " "))
	for _, k := range knownNameservers {
		if strings.Contains(joined, k.match) {
			return k.slug
		}
	}
	return ""
}

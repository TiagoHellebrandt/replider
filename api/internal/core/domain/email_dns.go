package domain

import (
	"context"
	"fmt"
	"strings"
)

// Email authentication setup (US-M006).
//
// A domain that sends without SPF, DKIM and DMARC does not get "slightly
// worse" delivery — Gmail and Outlook have required authentication for bulk
// senders since 2024, so unauthenticated mail is rejected or spam-foldered
// outright. Worse, the failure is invisible from our side: the SMTP handoff
// succeeds and the message simply never appears.
//
// So this is not a checklist to nag the user with. It is a gate: the caps in
// US-M007 refuse to send cold mail from a domain that has not passed it.

// DNSRecordKind identifies what a record is for, so the UI can explain it
// without hardcoding strings.
type DNSRecordKind string

const (
	DNSKindSPF      DNSRecordKind = "spf"
	DNSKindDKIM     DNSRecordKind = "dkim"
	DNSKindDMARC    DNSRecordKind = "dmarc"
	DNSKindTracking DNSRecordKind = "tracking"
)

// DNSRecordStatus is the verification outcome for one record.
type DNSRecordStatus string

const (
	DNSStatusPending  DNSRecordStatus = "pending"  // not published (or still propagating)
	DNSStatusOK       DNSRecordStatus = "ok"       // published and correct
	DNSStatusMismatch DNSRecordStatus = "mismatch" // published but wrong value
)

// EmailDNSRecord is one record the customer must publish.
type EmailDNSRecord struct {
	Kind DNSRecordKind `json:"kind"`
	Type string        `json:"type"` // TXT | CNAME
	// Host is the fully-qualified name to create. Registrars differ on
	// whether they want the FQDN or just the subdomain, which is the single
	// most common setup mistake — the UI shows both (see HostShort).
	Host      string `json:"host"`
	HostShort string `json:"host_short"`
	Value     string `json:"value"`
	// Required distinguishes "mail will not be delivered" from "you lose a
	// nice-to-have". Tracking is optional; the other three are not.
	Required bool `json:"required"`
	// Purpose is user-facing copy explaining WHY, in one line.
	Purpose string `json:"purpose"`

	Status DNSRecordStatus `json:"status"`
	// Found is what DNS actually returned, so a mismatch is debuggable
	// without leaving the screen.
	Found string `json:"found,omitempty"`

	// MatchAny verifies PRESENCE of a valid record of this kind rather than
	// an exact value — the BYO posture: a customer sending through their own
	// SMTP provider needs AN SPF that authorizes it, and we cannot know which
	// include that is. Value then carries a suggestion for display only;
	// auto-provisioning must skip MatchAny records (publishing a suggestion
	// literally would be wrong).
	MatchAny bool `json:"match_any,omitempty"`
}

// EmailDNSSetup is the whole picture for one sending domain.
type EmailDNSSetup struct {
	Domain  string           `json:"domain"`
	Records []EmailDNSRecord `json:"records"`
	// Verified is true when every REQUIRED record checks out. This is the
	// flag the send caps read.
	Verified bool `json:"verified"`
	// NameserverHint is the detected DNS host ("cloudflare", "godaddy", …)
	// so the UI can show the right screenshots. Empty when unknown.
	NameserverHint string `json:"nameserver_hint,omitempty"`
	// ProviderManaged: the mailbox provider authenticates the domain itself
	// (a personal @gmail.com — Google publishes gmail.com's SPF/DKIM/DMARC).
	// There is nothing for the customer to do, and asking them to edit a
	// zone they do not own would be the incoherent screen this replaced.
	ProviderManaged bool `json:"provider_managed,omitempty"`
}

// DNSResolver is the narrow port used to verify published records.
// Injectable so tests never touch the network.
type DNSResolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
	LookupCNAME(ctx context.Context, name string) (string, error)
	LookupNS(ctx context.Context, name string) ([]string, error)
}

// EmailSendingConfig describes the platform side of the setup: which
// provider the customer sends through, and the hostnames they must point at.
type EmailSendingConfig struct {
	// SPFInclude is the provider's SPF include token (e.g.
	// "amazonses.com", "_spf.resend.com"). Empty disables the SPF record.
	SPFInclude string
	// DKIMSelector + DKIMTarget build the CNAME the provider publishes the
	// key behind. Delegating via CNAME (rather than pasting a TXT key) means
	// the provider can rotate keys without the customer touching DNS again.
	DKIMSelector string
	DKIMTarget   string
	// TrackingTarget is the host that link-wrapped clicks resolve to. A
	// tracking domain under the CUSTOMER's name is what keeps click tracking
	// from looking like a redirect through a stranger's domain.
	TrackingTarget string
	// DMARCReportTo receives aggregate reports. Optional.
	DMARCReportTo string
}

// BuildEmailDNSSetup produces the records for a sending domain. Pure — no
// network, no clock — so the UI can render the instructions instantly and
// verification is a separate, slower call.
func BuildEmailDNSSetup(domain string, cfg EmailSendingConfig) EmailDNSSetup {
	d := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(domain, ".")))
	setup := EmailDNSSetup{Domain: d}
	if d == "" {
		return setup
	}

	if cfg.SPFInclude != "" {
		setup.Records = append(setup.Records, EmailDNSRecord{
			Kind:      DNSKindSPF,
			Type:      "TXT",
			Host:      d,
			HostShort: "@",
			// ~all (softfail), not -all: a customer who also sends from
			// their own mail server would have every one of those messages
			// rejected the moment they publish a hard fail here.
			Value:    fmt.Sprintf("v=spf1 include:%s ~all", cfg.SPFInclude),
			Required: true,
			Purpose:  "Autoriza nosso servidor a enviar em nome do seu domínio. Sem isso, provedores tratam seus e-mails como falsificação.",
			Status:   DNSStatusPending,
		})
	}

	if cfg.DKIMSelector != "" && cfg.DKIMTarget != "" {
		host := fmt.Sprintf("%s._domainkey.%s", cfg.DKIMSelector, d)
		setup.Records = append(setup.Records, EmailDNSRecord{
			Kind:      DNSKindDKIM,
			Type:      "CNAME",
			Host:      host,
			HostShort: fmt.Sprintf("%s._domainkey", cfg.DKIMSelector),
			Value:     cfg.DKIMTarget,
			Required:  true,
			Purpose:   "Assina cada e-mail criptograficamente. É o que prova que a mensagem não foi alterada no caminho.",
			Status:    DNSStatusPending,
		})
	}

	setup.Records = append(setup.Records, EmailDNSRecord{
		Kind:      DNSKindDMARC,
		Type:      "TXT",
		Host:      "_dmarc." + d,
		HostShort: "_dmarc",
		// p=none deliberately: a policy of quarantine/reject on day one
		// breaks the customer's OTHER mail flows (ERP, NF-e, forms) that
		// nobody has aligned yet. Tighten after the reports come back clean.
		Value:    buildDMARCValue(cfg.DMARCReportTo),
		Required: true,
		Purpose:  "Diz aos provedores o que fazer quando alguém falsifica seu domínio, e te dá relatórios de quem está tentando.",
		Status:   DNSStatusPending,
	})

	if cfg.TrackingTarget != "" {
		setup.Records = append(setup.Records, EmailDNSRecord{
			Kind:      DNSKindTracking,
			Type:      "CNAME",
			Host:      "links." + d,
			HostShort: "links",
			Value:     cfg.TrackingTarget,
			Required:  false,
			Purpose:   "Opcional: links de rastreio saem no SEU domínio em vez de um domínio estranho — melhora entrega e confiança.",
			Status:    DNSStatusPending,
		})
	}

	return setup
}

// GoogleSPFInclude is the include Google asks Workspace domains to publish.
const GoogleSPFInclude = "_spf.google.com"

// providerOwnedDomains are mailbox providers' OWN domains: the provider
// publishes and maintains the authentication records, and the customer could
// not edit that zone if they wanted to.
var providerOwnedDomains = map[string]bool{
	"gmail.com":      true,
	"googlemail.com": true,
}

// BuildEmailDNSSetupForChannel derives the records from what the channel
// ACTUALLY sends through, replacing the env-driven build for the BYO path.
//
// The env model assumed a platform relay: EMAIL_SPF_INCLUDE named OUR
// server, EMAIL_DKIM_* delegated to OUR key. But the product sends through
// the CUSTOMER's mailbox (Gmail OAuth or their own SMTP), so those envs were
// empty in production — the build emitted only a DMARC row, verification
// failed closed forever, and the screen showed an eternal warning above a
// single green check. The records a BYO sender needs are a function of the
// channel, not of the platform:
//
//   - Gmail OAuth on a personal @gmail.com: Google authenticates gmail.com
//     itself; there is NOTHING to publish (ProviderManaged, Verified).
//   - Gmail OAuth on a Workspace domain: SPF must include _spf.google.com;
//     DMARC required; Google's DKIM (google._domainkey TXT) recommended —
//     presence-checked, since only the customer's admin console knows the key.
//   - Own SMTP: AN SPF must exist (we cannot know their provider's include,
//     so presence is the check and the value is a suggestion); DMARC
//     required. DKIM is unknowable (any selector) — the SMTP test-send proof
//     (MarkSMTPProven) is that path's real gate.
func BuildEmailDNSSetupForChannel(domainName string, googleMailbox bool, cfg EmailSendingConfig) EmailDNSSetup {
	d := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(domainName, ".")))
	setup := EmailDNSSetup{Domain: d}
	if d == "" {
		return setup
	}
	if googleMailbox && providerOwnedDomains[d] {
		setup.ProviderManaged = true
		setup.Verified = true
		return setup
	}

	if googleMailbox {
		setup.Records = append(setup.Records, EmailDNSRecord{
			Kind:      DNSKindSPF,
			Type:      "TXT",
			Host:      d,
			HostShort: "@",
			Value:     fmt.Sprintf("v=spf1 include:%s ~all", GoogleSPFInclude),
			Required:  true,
			Purpose:   "Autoriza o Google (sua caixa) a enviar em nome do seu domínio. Sem isso, provedores tratam seus e-mails como falsificação.",
			Status:    DNSStatusPending,
		})
		setup.Records = append(setup.Records, EmailDNSRecord{
			Kind:      DNSKindDKIM,
			Type:      "TXT",
			Host:      "google._domainkey." + d,
			HostShort: "google._domainkey",
			// The key only exists in the customer's Google Admin console
			// (Apps → Google Workspace → Gmail → Autenticar e-mail), so the
			// value here is guidance and the check is presence.
			Value:    "Gere a chave no Google Admin (Autenticar e-mail) e publique o TXT que ele mostrar",
			Required: false,
			MatchAny: true,
			Purpose:  "Recomendado: a assinatura DKIM do Google no seu domínio. Ative no Google Admin e publique o registro gerado lá.",
			Status:   DNSStatusPending,
		})
	} else {
		setup.Records = append(setup.Records, EmailDNSRecord{
			Kind:      DNSKindSPF,
			Type:      "TXT",
			Host:      d,
			HostShort: "@",
			// A suggestion, not a value to copy blindly: only the customer's
			// email provider knows its include token.
			Value:    "v=spf1 include:<servidor do seu provedor de e-mail> ~all",
			Required: true,
			MatchAny: true,
			Purpose:  "Seu domínio precisa de um registro SPF autorizando o servidor que envia seu e-mail — o valor exato está na documentação do seu provedor de e-mail.",
			Status:   DNSStatusPending,
		})
	}

	setup.Records = append(setup.Records, EmailDNSRecord{
		Kind:      DNSKindDMARC,
		Type:      "TXT",
		Host:      "_dmarc." + d,
		HostShort: "_dmarc",
		Value:     buildDMARCValue(cfg.DMARCReportTo),
		Required:  true,
		Purpose:   "Diz aos provedores o que fazer quando alguém falsifica seu domínio, e te dá relatórios de quem está tentando.",
		Status:    DNSStatusPending,
	})
	return setup
}

// AllRequiredOK reports whether every required record checks out — the
// verified flag for the per-channel (BYO) build, where the record LIST is
// already correct by construction. The env-driven RequiredRecordsVerified
// keeps its hasSPF/hasDKIM fail-closed guard because there the list can be
// silently incomplete (empty env = missing rows).
func (s EmailDNSSetup) AllRequiredOK() bool {
	if s.ProviderManaged {
		return true
	}
	if len(s.Records) == 0 {
		return false
	}
	for _, r := range s.Records {
		if r.Required && r.Status != DNSStatusOK {
			return false
		}
	}
	return true
}

func buildDMARCValue(reportTo string) string {
	v := "v=DMARC1; p=none;"
	if r := strings.TrimSpace(reportTo); r != "" {
		v += fmt.Sprintf(" rua=mailto:%s;", r)
	}
	// fo=1 asks for a report whenever ANY mechanism fails, which is what
	// makes the reports useful while the setup is still being fixed.
	return v + " fo=1"
}

// RequiredRecordsVerified reports whether every required record passed.
//
// SPF and DKIM must be PRESENT, not merely passing. An empty platform
// sending config emits neither record, and a setup holding only a DMARC
// TXT would then verify — opening the US-M007 send caps on a domain that
// authenticates nothing. Missing platform config is our gap, and it must
// fail closed rather than certify a domain we cannot sign for.
func (s EmailDNSSetup) RequiredRecordsVerified() bool {
	var hasSPF, hasDKIM bool
	for _, r := range s.Records {
		if r.Required && r.Status != DNSStatusOK {
			return false
		}
		switch r.Kind {
		case DNSKindSPF:
			hasSPF = true
		case DNSKindDKIM:
			hasDKIM = true
		}
	}
	return hasSPF && hasDKIM
}

// PendingRequired lists the records still blocking the domain, so the UI can
// say exactly what is missing instead of "not verified".
func (s EmailDNSSetup) PendingRequired() []EmailDNSRecord {
	var out []EmailDNSRecord
	for _, r := range s.Records {
		if r.Required && r.Status != DNSStatusOK {
			out = append(out, r)
		}
	}
	return out
}

var ErrEmailDomainNotVerified = &AppError{
	Code:    "EMAIL_DOMAIN_NOT_VERIFIED",
	Message: "publique os registros DNS (SPF, DKIM, DMARC) antes de enviar por este domínio",
}

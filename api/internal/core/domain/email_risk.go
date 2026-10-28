package domain

import (
	"context"
	"strings"
	"time"
	"unicode"
)

// Risk signals on inbound email (§6.10, camada 1).
//
// Cheap checks, no model call, run on everything that arrives. The dor is
// specific and validated: B2B healthcare is a constant target for invoice
// fraud — someone impersonates a supplier, sends a boleto with different bank
// details, and the money leaves. Nobody in SDR covers this.
//
// Everything here answers ONE question: is this sender who they say they are?
// The judgement of what to DO about it belongs upstairs — these are the
// facts, each with the evidence that produced it.

// EmailRiskKind is a stable, machine-readable signal name.
type EmailRiskKind string

const (
	// RiskAuthFailed means the domain in the From line did not authorise the
	// send. The strongest single technical signal of impersonation.
	RiskAuthFailed EmailRiskKind = "auth_failed"
	// RiskAuthUnknown means nobody checked. Weak on its own — plenty of
	// legitimate mail arrives unverified — but it stops being weak when it
	// travels with a lookalike domain or a brand-new one.
	RiskAuthUnknown EmailRiskKind = "auth_unknown"
	// RiskLookalikeDomain is a domain one or two edits away from one this
	// account already corresponds with: medl1ne.com vs medline.com.
	RiskLookalikeDomain EmailRiskKind = "lookalike_domain"
	// RiskNewDomain is a domain registered days ago. Legitimate businesses
	// rarely email you from a domain created last Tuesday.
	RiskNewDomain EmailRiskKind = "new_domain"
	// RiskReplyToMismatch means the address that would receive your answer
	// is not the one that sent the message — the mechanic of a redirected
	// reply.
	RiskReplyToMismatch EmailRiskKind = "reply_to_mismatch"
	// RiskFirstContact is a sender never seen before on this account. Not a
	// problem by itself; the point of prospecting is strangers writing back.
	// It is here because it multiplies everything else.
	RiskFirstContact EmailRiskKind = "first_contact"
	// RiskChangedPaymentDetails is the golpe do boleto: a contact you have
	// paid before, sending different bank details. The heaviest signal in the
	// system, and the only one that alone is enough — because it is the
	// moment the money would leave.
	RiskChangedPaymentDetails EmailRiskKind = "changed_payment_details"
	// RiskNetworkReported means other companies on the platform reported this
	// sender as a scam. The signal no single account could produce on its
	// own, and the reason the network is worth being part of.
	RiskNetworkReported EmailRiskKind = "network_reported"
)

// EmailRiskSignal is one finding, with the evidence that produced it.
//
// Evidence rather than a score alone, because the person who has to act on
// this knows their own business: "o domínio difere do que o Fornecedor X
// sempre usou" is something they can confirm in ten seconds, and "risco 68"
// is not.
type EmailRiskSignal struct {
	Kind EmailRiskKind `json:"kind"`
	// Detail is human-readable and specific. Shown verbatim.
	Detail string `json:"detail"`
	// Weight contributes to the score. Kept on the signal so a change of
	// policy is a change of numbers, not of structure.
	Weight int `json:"weight"`
}

// Signal weights.
//
// Calibrated so that no SINGLE weak signal can quarantine a message, and so
// that the combinations that actually describe fraud do. A brand-new domain
// that also fails authentication and looks like a supplier you know is 95;
// a first contact from an unverified older provider is 15, which is most
// legitimate cold replies and must stay quiet.
const (
	weightAuthFailed      = 40
	weightAuthUnknown     = 10
	weightLookalikeDomain = 35
	weightNewDomain       = 25
	weightReplyToMismatch = 20
	weightFirstContact    = 5
	// Alone above the threshold, on purpose. Every other signal describes
	// something that MIGHT be wrong; this one describes a known counterparty
	// asking for money somewhere new, which is the exact shape of the fraud
	// and the last moment anyone can stop it.
	weightChangedPayment = 60
	// Also alone above the threshold. Independent businesses do not
	// coincidentally report the same domain; by the time two have, the
	// question is settled.
	weightNetworkReported = 70

	// RiskQuarantineThreshold is where a message stops being "worth a look"
	// and becomes "do not act on this until you have confirmed by phone".
	RiskQuarantineThreshold = 50
)

// EmailRiskAssessment is the verdict for one message.
type EmailRiskAssessment struct {
	Score   int               `json:"score"`
	Signals []EmailRiskSignal `json:"signals"`
}

// Quarantined reports whether this message should be held out of the normal
// flow and shown with a warning.
func (a EmailRiskAssessment) Quarantined() bool { return a.Score >= RiskQuarantineThreshold }

// Clean reports that nothing at all was found. Distinct from a low score:
// "nothing found" and "a little suspicious" are different things to show.
func (a EmailRiskAssessment) Clean() bool { return len(a.Signals) == 0 }

// EmailRiskInput is everything the technical layer needs. No repositories, no
// clock, no network — a pure function of its inputs, so every rule can be
// tested by stating the case.
type EmailRiskInput struct {
	FromAddress string
	ReplyTo     string
	Auth        InboundAuthVerdict

	// KnownDomains are the domains this account has corresponded with
	// before. The lookalike check is only meaningful against them: a domain
	// that resembles nothing you know is just a domain.
	KnownDomains []string
	// SeenBefore is whether this exact address has written before.
	SeenBefore bool
	// DomainRegisteredAt is nil when the lookup failed or was not run —
	// absence must not read as "old and trusted".
	DomainRegisteredAt *time.Time

	// ChangedPayments are payment identifiers in this message that this
	// contact has never used. Empty when there is no history to compare
	// against: a first invoice from a new supplier is a first invoice, not a
	// fraud.
	ChangedPayments []PaymentDetail

	// NetworkReports is how many OTHER accounts reported this sender as a
	// scam. Zero for almost everything; the check exists for the case where
	// it is not.
	NetworkReports int

	Now time.Time
}

// InboundAuthVerdict is the three-way SPF/DKIM/DMARC conclusion, restated
// here so the domain layer does not depend on a channel adapter.
type InboundAuthVerdict struct {
	Trustworthy bool
	Suspicious  bool
}

// NewDomainMaxAge is how recently a domain must have been registered to be
// worth flagging. Thirty days: fraud domains are typically days old, and a
// legitimate company emailing you from a domain registered last month is
// unusual enough to mention and common enough not to block on alone.
const NewDomainMaxAge = 30 * 24 * time.Hour

// NetworkScamThreshold is how many independent accounts must report a sender
// before the platform quarantines it for everyone.
//
// Two, not one. One report is a single company's judgement and can be a
// mistake, a grudge, or a misunderstanding; two independent businesses
// reporting the same domain is not a coincidence. Higher than two would mean
// the third victim is the price of certainty, and the whole point is that
// they should not have to be.
const NetworkScamThreshold = 2

// AssessEmailRisk runs every technical check and returns what it found.
func AssessEmailRisk(in EmailRiskInput) EmailRiskAssessment {
	var out EmailRiskAssessment
	add := func(kind EmailRiskKind, weight int, detail string) {
		out.Signals = append(out.Signals, EmailRiskSignal{Kind: kind, Weight: weight, Detail: detail})
		out.Score += weight
	}

	fromDomain := DomainOf(in.FromAddress)

	switch {
	case in.Auth.Suspicious:
		add(RiskAuthFailed, weightAuthFailed,
			"o domínio "+fromDomain+" não autorizou este envio")
	case !in.Auth.Trustworthy:
		add(RiskAuthUnknown, weightAuthUnknown,
			"could not confirmar que o envio partiu de "+fromDomain)
	}

	if match, ok := FindLookalikeDomain(fromDomain, in.KnownDomains); ok {
		add(RiskLookalikeDomain, weightLookalikeDomain,
			"parecido com "+match+", que você já usa — confira letra por letra")
	}

	if in.DomainRegisteredAt != nil {
		if age := in.Now.Sub(*in.DomainRegisteredAt); age < NewDomainMaxAge && age >= 0 {
			add(RiskNewDomain, weightNewDomain,
				"domínio registrado há "+humanDays(age))
		}
	}

	if replyDomain := DomainOf(in.ReplyTo); replyDomain != "" && replyDomain != fromDomain {
		add(RiskReplyToMismatch, weightReplyToMismatch,
			"sua resposta iria para "+replyDomain+", não para "+fromDomain)
	}

	if !in.SeenBefore {
		add(RiskFirstContact, weightFirstContact, "primeiro contato deste endereço")
	}

	if in.NetworkReports >= NetworkScamThreshold {
		add(RiskNetworkReported, weightNetworkReported,
			"golpe reportado por "+itoa(in.NetworkReports)+" outra(s) empresa(s) da rede")
	}

	for _, p := range in.ChangedPayments {
		add(RiskChangedPaymentDetails, weightChangedPayment,
			"dados de pagamento diferentes dos que este contato sempre usou: "+p.Raw)
	}
	return out
}

// DomainOf extracts the domain from an address, lowercased. Empty for
// anything that is not one.
func DomainOf(address string) string {
	at := strings.LastIndex(address, "@")
	if at < 0 || at == len(address)-1 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(address[at+1:]))
}

// lookalikeMaxDistance is how many edits still count as "trying to look like
// this". Two catches medl1ne/medline and rnedline/medline; three starts
// matching genuinely different companies in the same industry.
const lookalikeMaxDistance = 2

// FindLookalikeDomain reports a known domain that this one is imitating.
//
// Compares the registrable part only ("medline" out of "mail.medline.com.br")
// so a legitimate subdomain does not read as an impersonation of its own
// parent. An exact match is not a lookalike — it is the real thing.
func FindLookalikeDomain(candidate string, known []string) (string, bool) {
	c := registrableLabel(candidate)
	if len(c) < 4 {
		// Short labels are one edit from everything. Flagging them produces
		// noise, and noise is how a warning stops being read.
		return "", false
	}
	for _, k := range known {
		kl := registrableLabel(k)
		if kl == "" || kl == c {
			continue
		}
		if levenshtein(c, kl) <= lookalikeMaxDistance {
			return k, true
		}
		// Homoglyph swaps are one substitution each, so they are already
		// caught above — except when several stack. Normalising catches
		// "rnedl1ne" (rn→m, 1→i), which is three raw edits.
		if normalizeHomoglyphs(c) == normalizeHomoglyphs(kl) {
			return k, true
		}
	}
	return "", false
}

// registrableLabel is the name people read: "medline" from
// "mail.medline.com.br". Approximate by design — a real public-suffix list is
// a dependency and a data file, and being wrong here costs a missed signal,
// not a false accusation.
func registrableLabel(domain string) string {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(domain)), ".")
	// Drop the public suffix: com.br, co.uk and friends are two labels.
	for len(parts) > 1 && isSuffixLabel(parts[len(parts)-1]) {
		parts = parts[:len(parts)-1]
	}
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func isSuffixLabel(s string) bool {
	switch s {
	case "com", "net", "org", "gov", "edu", "br", "co", "uk", "io", "app", "med", "ind":
		return true
	}
	return len(s) == 2 // country codes
}

// homoglyphs maps characters that read alike to one representative.
var homoglyphs = strings.NewReplacer(
	"0", "o", "1", "l", "3", "e", "4", "a", "5", "s", "7", "t",
	"rn", "m", "vv", "w", "ii", "u",
)

func normalizeHomoglyphs(s string) string {
	return homoglyphs.Replace(strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, s))
}

// levenshtein is the edit distance between two short strings.
func levenshtein(a, b string) int {
	ar, br := []rune(a), []rune(b)
	prev := make([]int, len(br)+1)
	cur := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		cur[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			cur[j] = min3(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(br)]
}

func humanDays(d time.Duration) string {
	days := int(d.Hours() / 24)
	switch {
	case days <= 0:
		return "menos de um dia"
	case days == 1:
		return "1 dia"
	default:
		return itoa(days) + " dias"
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// DomainAgeLookup answers when a domain was registered.
//
// A port because it is a network call to a registry that can be slow, rate
// limited or simply absent for a TLD. A nil implementation means the check
// does not run — and a check that did not run must never be recorded as one
// that passed.
type DomainAgeLookup interface {
	// RegisteredAt returns nil, nil when the registry has no answer. An
	// error means the lookup failed, which is also not an answer.
	RegisteredAt(ctx context.Context, domain string) (*time.Time, error)
}

// EmailRiskRecord is one stored assessment.
type EmailRiskRecord struct {
	MessageID    string              `json:"message_id"`
	AccountID    string              `json:"-"`
	Assessment   EmailRiskAssessment `json:"assessment"`
	SenderDomain string              `json:"sender_domain"`
	CreatedAt    time.Time           `json:"created_at"`
}

// EmailRiskRepository persists assessments and answers the two questions the
// technical checks need about history.
type EmailRiskRepository interface {
	Save(ctx context.Context, rec *EmailRiskRecord) error
	// Clear removes an assessment. Used when a human says the message is
	// legitimate: the warning disappears because it was answered, not
	// because it was hidden.
	Clear(ctx context.Context, messageID, accountID string) error
	GetByMessage(ctx context.Context, messageID string) (*EmailRiskRecord, error)
	ListByMessages(ctx context.Context, messageIDs []string) (map[string]*EmailRiskRecord, error)
	// KnownDomains is what makes a lookalike detectable: a domain resembling
	// nothing you know is just a domain.
	KnownDomains(ctx context.Context, accountID string, limit int32) ([]string, error)
	SeenBefore(ctx context.Context, accountID, address, exceptMessageID string) (bool, error)
}

// SenderAllowlistRepository remembers senders a human has vouched for.
//
// One clean click, per §6.10 camada 3. It exists because a false positive
// that cannot be dismissed is worse than no warning at all: the reader learns
// to scroll past the banner, and then the real one is invisible too.
type SenderAllowlistRepository interface {
	// Allow vouches for one sender on one account.
	Allow(ctx context.Context, accountID, address, byUserID string) error
	// IsAllowed reports whether a human already vouched for this sender.
	IsAllowed(ctx context.Context, accountID, address string) (bool, error)
}

// ScamReport is one company saying "this was a scam".
type ScamReport struct {
	ID        string `json:"id"`
	AccountID string `json:"-"`
	MessageID string `json:"message_id"`
	// SenderDomain is the indicator that crosses accounts. A scammer's domain
	// is not sensitive customer data, so it travels without k-anonymity —
	// unlike the anonymised strategy tier, where the data IS about a customer.
	SenderDomain string `json:"sender_domain"`
	// PaymentValues are the bank details the scam used, so the same account
	// reappearing under a new domain is still caught.
	PaymentValues []string `json:"payment_values"`
	// Reason is the reporter's own words, and the highest-value field here:
	// "esse fornecedor nunca manda boleto por email" is knowledge no
	// technical check has.
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// ScamReportRepository stores reports and answers the network question.
type ScamReportRepository interface {
	// Report records one. Idempotent per (account, domain): a single customer
	// cannot push a domain over the network threshold alone.
	Report(ctx context.Context, r *ScamReport) error
	// CountReportingAccounts is how many DISTINCT accounts reported this
	// domain, excluding the one asking — a company's own report is not
	// corroboration of itself.
	CountReportingAccounts(ctx context.Context, senderDomain, exceptAccountID string) (int, error)
	// CountByPaymentValue is the same question for a bank account, so a
	// scammer who changes domain but keeps the account is still caught.
	CountByPaymentValue(ctx context.Context, value, exceptAccountID string) (int, error)
}

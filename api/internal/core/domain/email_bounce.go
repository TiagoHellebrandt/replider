package domain

import (
	"context"
	"regexp"
	"strings"
	"time"
)

// Bounce handling (§6.8.4).
//
// A bounce is the only feedback loop email has that says "stop". Ignoring it
// is not a missing metric — it is the fastest way to lose a customer's
// domain: mailbox providers read repeated sends to dead addresses as a
// signal the sender does not maintain their list, and the throttling that
// follows takes weeks to undo. That is why a hard bounce SUPPRESSES the
// address here rather than only incrementing a counter.
//
// The distinction that matters:
//
//   - HARD — the address does not exist, or the domain does not. Permanent.
//     Never write to it again.
//   - SOFT — full mailbox, temporary server failure, greylisting. The
//     address is fine; today is not. Counted, never suppressed.
//   - COMPLAINT — the recipient pressed "spam". Worse than a hard bounce:
//     the address works, and the person told their provider we are abusive.
//     Suppressed like a hard bounce, and never silently.
type BounceKind string

const (
	BounceHard      BounceKind = "hard"
	BounceSoft      BounceKind = "soft"
	BounceComplaint BounceKind = "complaint"
)

// Permanent reports a bounce that must stop future sends to the address.
func (k BounceKind) Permanent() bool {
	return k == BounceHard || k == BounceComplaint
}

// EmailBounce is one delivery failure, normalized across providers.
type EmailBounce struct {
	Kind      BounceKind
	Recipient string
	// Diagnostic is the remote server's own words ("550 5.1.1 User unknown").
	// Kept verbatim: a support conversation about a blocked domain is won or
	// lost on this string, and paraphrasing it loses the code.
	Diagnostic string
	// OriginalMessageID is the Message-ID of the message that failed, when
	// the report carries it. Often absent — see EmailBounceRepository.
	OriginalMessageID string
}

// enhancedStatusRe matches an RFC 3463 status code (5.1.1, 4.2.2). The FIRST
// digit is the whole verdict: 5 permanent, 4 transient.
var enhancedStatusRe = regexp.MustCompile(`\b([245])\.(\d{1,3})\.(\d{1,3})\b`)

// smtpReplyRe matches a bare SMTP reply code at the start of a diagnostic.
var smtpReplyRe = regexp.MustCompile(`\b(5\d{2}|4\d{2})\b`)

// hardPhrases are what a permanent failure says when no machine-readable
// code survived the trip. Deliberately narrow: guessing HARD from vague
// wording would suppress a working address, and a wrongly suppressed
// customer address is silent — nobody notices the email that never went.
var hardPhrases = []string{
	"user unknown", "no such user", "recipient not found", "unknown recipient",
	"mailbox unavailable", "does not exist", "no mailbox", "address rejected",
	"invalid recipient", "usuário desconhecido", "destinatário inexistente",
	"domain not found", "unrouteable address", "recipient address rejected",
}

// softPhrases are the transient ones. Checked FIRST, because several overlap
// with hard wording ("mailbox full" contains neither, but "over quota" and
// "temporarily deferred" must never be read as permanent).
var softPhrases = []string{
	"mailbox full", "over quota", "quota exceeded", "temporarily", "try again",
	"greylist", "grey-list", "deferred", "timed out", "too many connections",
	"rate limited", "caixa de entrada cheia", "cota excedida",
}

// ClassifyBounce decides permanence from what the report actually says.
//
// Order matters. A machine-readable status code beats prose every time — the
// prose is written by whichever server rejected the message and is not a
// contract. Prose is the fallback for reports that carry no code, and when
// even that is silent the answer is SOFT: a soft bounce costs one retry, a
// wrong hard bounce costs the address forever.
func ClassifyBounce(kindHint, diagnostic string) BounceKind {
	switch strings.ToLower(strings.TrimSpace(kindHint)) {
	case "complaint", "spam", "spamreport", "abuse":
		return BounceComplaint
	case "hard", "permanent", "hardbounce", "bounce_hard", "blocked", "invalid":
		return BounceHard
	case "soft", "transient", "temporary", "softbounce", "deferred", "delayed":
		return BounceSoft
	}

	d := strings.ToLower(diagnostic)
	if m := enhancedStatusRe.FindStringSubmatch(d); m != nil {
		if m[1] == "5" {
			return BounceHard
		}
		return BounceSoft
	}
	if m := smtpReplyRe.FindStringSubmatch(d); m != nil {
		if strings.HasPrefix(m[1], "5") {
			return BounceHard
		}
		return BounceSoft
	}
	for _, p := range softPhrases {
		if strings.Contains(d, p) {
			return BounceSoft
		}
	}
	for _, p := range hardPhrases {
		if strings.Contains(d, p) {
			return BounceHard
		}
	}
	// Unknown. A retry is cheap; losing a real customer address is not.
	return BounceSoft
}

// SuppressionReason says why an address stopped receiving mail.
type SuppressionReason string

const (
	SuppressionHardBounce SuppressionReason = "hard_bounce"
	SuppressionComplaint  SuppressionReason = "complaint"
	// SuppressionManual is a human decision — "never write to this person
	// again" — which must be as durable as a bounce.
	SuppressionManual SuppressionReason = "manual"
)

// EmailSuppression is one address this account must not write to.
type EmailSuppression struct {
	ID        string            `json:"id"`
	AccountID string            `json:"-"`
	Address   string            `json:"address"`
	Reason    SuppressionReason `json:"reason"`
	// Diagnostic is the remote server's verbatim refusal, so a person asking
	// "why can't I email them?" gets an answer instead of a policy.
	Diagnostic string    `json:"diagnostic,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// ErrEmailAddressSuppressed refuses a send to a dead or complaining address.
var ErrEmailAddressSuppressed = &AppError{
	Code:    "EMAIL_ADDRESS_SUPPRESSED",
	Message: "este endereço não recebe mais e-mails: houve rejeição permanente ou marcação de spam",
}

// EmailSuppressionRepository persists the do-not-send list.
type EmailSuppressionRepository interface {
	// Suppress is idempotent per (account, address). A second hard bounce
	// must not fail, and must not overwrite the FIRST diagnostic — the first
	// one is the reason; later ones are consequences of ignoring it.
	Suppress(ctx context.Context, s *EmailSuppression) error
	IsSuppressed(ctx context.Context, accountID, address string) (bool, error)
	List(ctx context.Context, accountID string, limit, offset int) ([]*EmailSuppression, error)
	// Release exists because a suppression can be wrong: an address is fixed,
	// a mailbox is emptied, a person changes their mind. Nothing about a
	// bounce should be permanent to the point of being unappealable.
	Release(ctx context.Context, accountID, address string) error
}

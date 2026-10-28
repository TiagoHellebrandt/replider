package domain

import (
	"regexp"
	"strings"
)

// Inbox triage (§6.10, camada 3).
//
// A shared commercial mailbox is not a chat stream. In one morning it holds a
// lead answering a cold email, a customer with a problem, a supplier's
// invoice, three newsletters and one phishing attempt — and a purely
// chronological list buries the first under the last three.
//
// So each conversation gets a category and an importance, and both are
// derived from things we ALREADY know (the risk score, the CRM, the lead
// pipeline) before any model is asked. A classifier that costs money and
// latency should only see what the facts could not answer.

// InboxCategory is the bucket a conversation lands in.
type InboxCategory string

const (
	// CategoryQuarantine outranks everything. A message we believe is a scam
	// must never be filed as an ordinary supplier invoice, which is exactly
	// how the invoice fraud works.
	CategoryQuarantine InboxCategory = "quarantine"
	// CategoryHotLead is a prospect who ANSWERED. It is the whole point of
	// the product, and it is what a chronological inbox buries.
	CategoryHotLead  InboxCategory = "hot_lead"
	CategoryCustomer InboxCategory = "customer"
	CategorySupplier InboxCategory = "supplier"
	// CategoryFinancial is money moving: an invoice, a boleto, a PIX.
	// Separate from supplier because it is what a fraudster imitates.
	CategoryFinancial  InboxCategory = "financial"
	CategoryNewsletter InboxCategory = "newsletter"
	CategoryOther      InboxCategory = "other"
)

// ValidInboxCategory guards the filter parameter — the category reaches SQL,
// and "whatever the client sent" must never.
func ValidInboxCategory(c string) bool {
	switch InboxCategory(strings.ToLower(strings.TrimSpace(c))) {
	case CategoryQuarantine, CategoryHotLead, CategoryCustomer,
		CategorySupplier, CategoryFinancial, CategoryNewsletter, CategoryOther:
		return true
	}
	return false
}

// InboxSignals is everything the classifier gets to see. Deliberately a
// value type of plain facts: the rules below must be testable without a
// database, a model, or a network.
type InboxSignals struct {
	// Quarantined comes from the risk engine (camada 1/2).
	Quarantined bool
	// LeadStatus is the contact's pipeline status, empty when not a lead.
	LeadStatus LeadStatus
	// HasSale is true when this contact has ever bought — the one fact that
	// separates a customer from a prospect.
	HasSale bool
	// HasPaymentInstruction is true when the message carries bank details, a
	// boleto line or a PIX key.
	HasPaymentInstruction bool
	// ListUnsubscribe is the header every legitimate bulk sender must set.
	ListUnsubscribe bool
	Subject         string
	Body            string
	// Direction of the last message. An inbound is what makes a lead HOT;
	// our own outbound does not.
	LastWasInbound bool
}

var (
	// newsletterRe matches the words bulk senders use about themselves. Only
	// consulted as a fallback: List-Unsubscribe is the real signal, and
	// matching prose alone would file a customer writing "cancelar" as a
	// newsletter.
	newsletterRe = regexp.MustCompile(`(?i)(newsletter|descadastr|unsubscribe|cancelar inscri|você está recebendo|no-?reply)`)

	// financialRe covers the vocabulary of money moving. Paired with an
	// actual payment instruction wherever possible — the words alone appear
	// in every "vou te mandar o orçamento" in the language.
	financialRe = regexp.MustCompile(`(?i)(boleto|nota fiscal|nfe\b|nf-e|fatura|cobran[çc]a|pagamento|vencimento|pix\b|invoice)`)
)

// ClassifyInbox picks a category from facts alone.
//
// Returns ok=false when nothing here is decisive, which is the caller's cue
// to ask a cheap model. That split is the point: a rule that is sure costs
// nothing, and the model only sees the genuinely ambiguous minority.
func ClassifyInbox(s InboxSignals) (InboxCategory, bool) {
	// Order is the design. Quarantine first because a scam dressed as an
	// invoice must not be filed as an invoice — that mistake is the fraud
	// working exactly as intended.
	if s.Quarantined {
		return CategoryQuarantine, true
	}

	// A newsletter can carry payment words ("sua fatura chegou") and can come
	// from a company we buy from, so it is checked before both. The header is
	// the honest signal: a bulk sender that omits it is already misbehaving.
	if s.ListUnsubscribe {
		return CategoryNewsletter, true
	}

	if s.HasPaymentInstruction {
		return CategoryFinancial, true
	}

	// A prospect who answered. Checked before "customer" because a lead who
	// replies is the event the product exists to surface, and before any
	// prose matching because pipeline state is a fact and prose is a guess.
	if s.LastWasInbound && isRespondingLead(s.LeadStatus) {
		return CategoryHotLead, true
	}

	if s.HasSale {
		return CategoryCustomer, true
	}

	text := s.Subject + " " + s.Body
	if newsletterRe.MatchString(text) && !s.LastWasInbound {
		return CategoryNewsletter, true
	}
	if financialRe.MatchString(text) {
		// Words without an actual instruction: likely financial, but not
		// certain — a supplier saying "vou emitir a nota" belongs here while
		// a colleague saying "o pagamento saiu" does not.
		return CategoryFinancial, true
	}

	// Not decidable from facts. The caller may ask a model; until it
	// answers, "other" is honest.
	return CategoryOther, false
}

// machineLocalParts are mailbox names only a machine writes from. Matched on
// the local part alone, because the domain is worthless here: Stripe's
// receipts and a hospital's purchasing manager can both be @stripe.com in
// principle, and "notifications@" never is a person either way.
var machineLocalParts = map[string]bool{
	"no-reply": true, "noreply": true, "donotreply": true, "do-not-reply": true,
	"nao-responda": true, "naoresponda": true, "no-responder": true,
	"notification": true, "notifications": true, "notificacao": true, "notificacoes": true,
	"mailer-daemon": true, "postmaster": true, "bounce": true, "bounces": true,
	"billing": true, "faturamento": true, "cobranca": true, "invoice": true,
	"invoices": true, "receipt": true, "receipts": true, "boleto": true,
	"newsletter": true, "news": true, "marketing": true, "automated": true,
	"alert": true, "alerts": true, "noreply-": true,
}

// IsMachineSender reports an address that belongs to a sending system rather
// than a person. Local part only, folded and lowercased; a `+tag` suffix is
// ignored so `no-reply+abc@` still matches.
func IsMachineSender(address string) bool {
	at := strings.IndexByte(address, '@')
	if at <= 0 {
		return false
	}
	local := strings.ToLower(strings.TrimSpace(address[:at]))
	if plus := strings.IndexByte(local, '+'); plus > 0 {
		local = local[:plus]
	}
	return machineLocalParts[local]
}

// IsProspectInbound reports whether an unsolicited inbound is plausibly a
// person writing to the business, and therefore worth minting a lead for.
//
// This exists because "somebody emailed us" was being read as "a prospect
// replied". Every message reaching a connected mailbox — Stripe receipts,
// domain renewals, newsletters — became a contact AND a lead stamped
// `responded`, so the one number the owner checks to decide whether the
// product works ("quem respondeu") was mostly the company's own paperwork.
//
// Deliberately conservative in ONE direction: it only refuses on evidence.
// A message with nothing machine-like about it mints a lead, because
// missing a real prospect is the expensive mistake and an extra lead in a
// funnel is a cheap one. Advancing an EXISTING lead is not gated by this at
// all — we wrote to them first, so their reply is a reply whatever it says.
func IsProspectInbound(sender, subject, body string, listUnsubscribe bool) bool {
	if IsMachineSender(sender) {
		return false
	}
	// A bulk sender that sets the header is telling the truth about being
	// bulk. Nobody negotiating a tender sets List-Unsubscribe.
	if listUnsubscribe {
		return false
	}
	text := subject + " " + body
	// Newsletter vocabulary only — NOT the financial words. "Segue a nota
	// fiscal" and "me manda o boleto" are exactly what a hospital that wants
	// to buy writes, and refusing those would throw away the lead this
	// product exists to catch.
	return !newsletterRe.MatchString(text)
}

// isRespondingLead reports a pipeline stage where the prospect is engaged.
func isRespondingLead(s LeadStatus) bool {
	switch s {
	case LeadStatusResponded, LeadStatusQualified:
		return true
	}
	return false
}

// InboxImportance orders the list.
//
// The signals are taken as a parameter even though only the category is read
// today: recency and deal value belong here next, and threading them in later
// would change every call site.
//
// Chronological order is wrong for a commercial mailbox: the newest thing in
// it is usually a newsletter, and the thing that pays for the product is a
// lead who answered two hours ago. Higher sorts first.
func InboxImportance(c InboxCategory, s InboxSignals) int {
	switch c {
	case CategoryQuarantine:
		// Top, and not because it is urgent business — because it is the one
		// thing that costs money if it is not seen before it is trusted.
		return 100
	case CategoryHotLead:
		return 90
	case CategoryFinancial:
		return 70
	case CategoryCustomer:
		return 60
	case CategorySupplier:
		return 40
	case CategoryOther:
		return 30
	case CategoryNewsletter:
		// Bottom, never hidden. Filing something as junk and then not showing
		// it is how a real message disappears.
		return 10
	}
	return 20
}

// InboxCategoryLabelKey is the i18n key for a category, so the server never
// ships user-facing copy and the app never re-implements the enum.
func InboxCategoryLabelKey(c InboxCategory) string {
	return "inbox_category_" + string(c)
}

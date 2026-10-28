package domain

import (
	"regexp"
	"strings"
)

// Extracting the payment instructions out of a message (§6.10, camada 2).
//
// This is the half of the anti-fraud layer that nobody else can build: the
// golpe do boleto works because a supplier's real thread is hijacked and one
// message carries different bank details. Generic spam filters cannot see it
// — nothing about that message is spammy. What catches it is knowing what
// THAT contact's bank details have always been, which only a system holding
// the whole relationship knows.
//
// So the job here is narrow and mechanical: pull every payment identifier out
// of a message, normalised enough that "1234-5" and "12345" are the same
// account. Deciding what it means belongs to the comparison against history.

// PaymentKind names one kind of payment instruction.
type PaymentKind string

const (
	// PaymentPixKey is a PIX key of any of the five accepted forms.
	PaymentPixKey PaymentKind = "pix_key"
	// PaymentBankAccount is an agência/conta pair, which is what actually
	// changes in the fraud.
	PaymentBankAccount PaymentKind = "bank_account"
	// PaymentBoleto is a linha digitável — the 47-48 digits a person types
	// into their bank.
	PaymentBoleto PaymentKind = "boleto"
	// PaymentCNPJ is the company the money is going to. A different CNPJ on
	// an invoice from a supplier you know is the same signal as a different
	// account.
	PaymentCNPJ PaymentKind = "cnpj"
)

// PaymentDetail is one identifier found in a message.
type PaymentDetail struct {
	Kind PaymentKind `json:"kind"`
	// Value is normalised for comparison: digits only where the format is
	// numeric, lowercased where it is not. Two messages carrying the same
	// account in different punctuation produce the same Value.
	Value string `json:"value"`
	// Raw is what actually appeared, for showing the reader. "Ag 1234 / CC
	// 56789-0" is recognisable; "1234567890" is not.
	Raw string `json:"raw"`
}

var (
	// Linha digitável: 47 (bank) or 48 (utility) digits, usually written in
	// blocks with dots and spaces. Matched on the punctuated form first so a
	// long CPF sequence cannot masquerade as one.
	boletoRe = regexp.MustCompile(`\b(\d{5}[.\s]?\d{5}[\s.]?\d{5}[.\s]?\d{6}[\s.]?\d{5}[.\s]?\d{6}[\s.]?\d[\s.]?\d{14})\b`)

	// CNPJ, punctuated or bare.
	cnpjRe = regexp.MustCompile(`\b(\d{2}\.?\d{3}\.?\d{3}/?\d{4}-?\d{2})\b`)

	// Agência + conta, in the shapes Brazilian invoices actually use:
	// "Ag. 1234 C/C 56789-0", "agência 1234 conta 56789-0".
	bankAccountRe = regexp.MustCompile(`(?i)ag(?:[êe]ncia|\.)?\s*:?\s*(\d{3,5})[-\s/]*\d?\s*[,;]?\s*(?:c/?c|conta(?:\s+corrente)?|cc)\s*:?\s*(\d{4,12})-?(\d)?`)

	// PIX by explicit label, which is how it appears in an invoice email.
	// The value is whatever follows on the same line.
	pixLabelRe = regexp.MustCompile(`(?i)\b(?:chave\s+)?pix\s*(?:key)?\s*[:\-]\s*([^\n\r,;]{6,80})`)

	// A bare UUID is a random PIX key.
	pixRandomRe = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
)

// ExtractPaymentDetails pulls every payment identifier out of a message.
//
// Deliberately generous about what it recognises and strict about what it
// normalises. A missed identifier is a fraud that goes unflagged; a
// mis-normalised one is a false alarm about a supplier who did nothing wrong,
// and false alarms are what make people stop reading alerts.
func ExtractPaymentDetails(text string) []PaymentDetail {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	var out []PaymentDetail
	seen := map[string]struct{}{}
	add := func(kind PaymentKind, value, raw string) {
		if value == "" {
			return
		}
		key := string(kind) + "|" + value
		if _, dup := seen[key]; dup {
			return
		}
		seen[key] = struct{}{}
		out = append(out, PaymentDetail{Kind: kind, Value: value, Raw: strings.TrimSpace(raw)})
	}

	// Boletos first, and their text is then removed from what the other
	// patterns see: the tail of a linha digitável is fourteen digits, which
	// is exactly a CNPJ. Reading it as one would invent a company the message
	// never mentioned — and then report it as "new" on every invoice.
	rest := text
	for _, m := range boletoRe.FindAllString(text, -1) {
		add(PaymentBoleto, nonDigits.ReplaceAllString(m, ""), m)
		rest = strings.Replace(rest, m, " ", 1)
	}

	for _, m := range bankAccountRe.FindAllStringSubmatch(rest, -1) {
		// Agência and conta together: an agência alone is a branch, and
		// branches are shared by thousands of legitimate accounts.
		account := m[2]
		if len(m) > 3 && m[3] != "" {
			account += m[3]
		}
		add(PaymentBankAccount, m[1]+"/"+account, m[0])
	}

	for _, m := range pixLabelRe.FindAllStringSubmatch(text, -1) {
		add(PaymentPixKey, normalizePixKey(m[1]), m[1])
	}
	for _, m := range pixRandomRe.FindAllString(text, -1) {
		add(PaymentPixKey, strings.ToLower(m), m)
	}

	for _, m := range cnpjRe.FindAllString(rest, -1) {
		digits := nonDigits.ReplaceAllString(m, "")
		if len(digits) == 14 {
			add(PaymentCNPJ, digits, m)
		}
	}
	return out
}

// normalizePixKey makes the five key forms comparable.
//
// A CPF written "123.456.789-00" and "12345678900" are one key; an email in
// mixed case is one key. Without this, the same supplier re-sending their own
// details in different punctuation would look like a change.
func normalizePixKey(raw string) string {
	v := strings.TrimSpace(raw)
	v = strings.Trim(v, ".,;:")
	if v == "" {
		return ""
	}
	if strings.Contains(v, "@") {
		return strings.ToLower(v)
	}
	digits := nonDigits.ReplaceAllString(v, "")
	switch len(digits) {
	case 11, 14: // CPF, CNPJ
		return digits
	case 12, 13: // phone with country code, with or without the 9
		return digits
	}
	if len(digits) == 0 {
		return strings.ToLower(v)
	}
	// Something numeric but not a recognised length. Keep it: an unrecognised
	// key that stays stable across messages still detects a change, and a
	// change is the whole signal.
	return digits
}

// HasPaymentInstruction reports whether a message is asking for money at all.
//
// Used to decide whether the comparison against history is even relevant:
// most mail carries no payment details, and running the fraud check on it
// would be noise.
func HasPaymentInstruction(details []PaymentDetail) bool {
	for _, d := range details {
		switch d.Kind {
		case PaymentBoleto, PaymentBankAccount, PaymentPixKey:
			return true
		}
	}
	return false
}

// NewPaymentDetails returns the details that are NOT in the known set.
//
// This is the comparison the fraud alert rests on. Note what it does NOT do:
// it says nothing when there is no history. A first invoice from a new
// supplier is not suspicious — it is a first invoice, and flagging it would
// fire on every legitimate new relationship.
func NewPaymentDetails(found []PaymentDetail, known map[string]struct{}) []PaymentDetail {
	if len(known) == 0 {
		return nil
	}
	var out []PaymentDetail
	for _, d := range found {
		if _, ok := known[PaymentKey(d.Kind, d.Value)]; !ok {
			out = append(out, d)
		}
	}
	return out
}

// PaymentKey is the stable identity of one detail, for the known-set.
func PaymentKey(kind PaymentKind, value string) string {
	return string(kind) + "|" + value
}

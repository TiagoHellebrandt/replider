package whatsapp

import (
	"encoding/base64"
	"strings"
)

// waBizDescription is the non-standard property a WhatsApp Business account
// uses to describe itself inside a shared contact card. It is frequently the
// only text that says what the shared number actually IS ("Loja
// pertencente a Rede X"), which the parsed name/phone pair cannot convey.
const waBizDescription = "X-WA-BIZ-DESCRIPTION"

// waBizName is the name the BUSINESS declares for itself. It is not the same
// fact as the card's FN, which is whatever the person sharing had saved in
// their own phonebook ("Gerente Paulo celular") — so when the two differ, this is
// the one that identifies the entity and the FN is a nickname. They are equal
// on most cards, which is why it only ever gets shown when it differs.
const waBizName = "X-WA-BIZ-NAME"

// sameDisplayName reports whether two names would read as the same label.
// Compared case-folded with collapsed whitespace and NOT accent-folded:
// "Sao Jose" and "São José" are different spellings a business may care
// about, and showing both when they differ is cheap while wrongly collapsing
// them loses the official name entirely.
func sameDisplayName(a, b string) bool {
	return strings.EqualFold(collapseSpaces(a), collapseSpaces(b))
}

func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// decodeVCard returns the vCard text of a shared contact.
//
// Every payload we have captured carries it base64-encoded, but the field is
// not documented as such, so plain text is accepted too. Anything that is
// neither — a truncated card, a future encoding — returns empty rather than
// garbage: a contact card still renders from its name and phone, so failing to
// read the card must never cost the message.
func decodeVCard(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if looksLikeVCard(raw) {
		return raw
	}
	// Both paddings: Meta pads, but a re-encoder in between might not.
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding} {
		if out, err := enc.DecodeString(raw); err == nil && looksLikeVCard(string(out)) {
			return string(out)
		}
	}
	return ""
}

func looksLikeVCard(s string) bool {
	return strings.Contains(strings.ToUpper(s), "BEGIN:VCARD")
}

// vcardProperty reads one property value out of a vCard body.
//
// A `strings.Contains` would be wrong on all three of the shapes real cards
// use: a group prefix (`item1.TEL`), parameters after a semicolon
// (`TEL;waid=5565...`), and line folding — a line starting with a space or tab
// is a continuation of the one before it, so a long description arrives split
// across lines and must be rejoined before it is read.
func vcardProperty(card, name string) string {
	if card == "" {
		return ""
	}
	want := strings.ToUpper(name)

	unfolded := make([]string, 0, 16)
	for _, raw := range strings.Split(strings.ReplaceAll(card, "\r\n", "\n"), "\n") {
		if raw == "" {
			continue
		}
		if (raw[0] == ' ' || raw[0] == '\t') && len(unfolded) > 0 {
			// Exactly ONE character, per RFC 6350: the fold inserts CRLF plus a
			// single space, so removing more would eat a space that belongs to
			// the text ("...pertencente a" + fold + " Rede" reads as two
			// spaces on the wire, one of which is content).
			unfolded[len(unfolded)-1] += raw[1:]
			continue
		}
		unfolded = append(unfolded, raw)
	}

	for _, line := range unfolded {
		colon := strings.Index(line, ":")
		if colon < 0 {
			continue
		}
		key := line[:colon]
		if i := strings.Index(key, ";"); i >= 0 {
			key = key[:i] // drop parameters
		}
		if i := strings.LastIndex(key, "."); i >= 0 {
			key = key[i+1:] // drop the group prefix
		}
		if strings.ToUpper(strings.TrimSpace(key)) != want {
			continue
		}
		return unescapeVCardValue(strings.TrimSpace(line[colon+1:]))
	}
	return ""
}

// unescapeVCardValue turns RFC 6350's escapes back into text. `\\` is listed
// before `\n` so that a literal backslash followed by an n does not become a
// newline.
func unescapeVCardValue(v string) string {
	return strings.NewReplacer(
		`\\`, `\`,
		`\n`, "\n",
		`\N`, "\n",
		`\,`, ",",
		`\;`, ";",
	).Replace(v)
}

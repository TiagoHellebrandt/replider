package whatsapp

import (
	"encoding/base64"
	"testing"
)

// The card Meta actually sent on 2026-08-17, verbatim from wa_webhook_events.
// The business description on it is the only text saying what the shared
// number is; name and phone alone leave the attendant guessing.
const realVCard = "BEGIN:VCARD\n" +
	"VERSION:3.0\n" +
	"N:Centro;Mercearia Boa;José;;\n" +
	"FN:Mercearia Boa Vista Centro\n" +
	"item1.TEL;waid=551190000001:+55 11 9000-0001\n" +
	"item1.X-ABLabel:Celular\n" +
	"X-WA-BIZ-DESCRIPTION:Loja pertencente a Rede Exemplo Fictício do Bairro - SP\n" +
	"X-WA-BIZ-NAME:Mercearia Boa Vista Centro\n" +
	"END:VCARD"

func TestDecodeVCard_ReadsTheShapeMetaActuallySends(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte(realVCard))
	if got := decodeVCard(encoded); got != realVCard {
		t.Fatalf("base64 card did not round-trip, got %q", got)
	}
	// The field is not documented as base64, so plain text must work too.
	if got := decodeVCard(realVCard); got != realVCard {
		t.Errorf("plain card should pass through, got %q", got)
	}
}

func TestDecodeVCard_RefusesAnythingItCannotRead(t *testing.T) {
	// Reading the card must never be able to invent content. A card we cannot
	// read has to come back empty so the message still renders from name+phone.
	for _, in := range []string{
		"",
		"   ",
		"not base64 and not a card",
		base64.StdEncoding.EncodeToString([]byte("BEGIN:SOMETHINGELSE")),
	} {
		if got := decodeVCard(in); got != "" {
			t.Errorf("decodeVCard(%q) = %q, want empty", in, got)
		}
	}
}

func TestVCardProperty_ReadsThroughGroupsParamsAndFolding(t *testing.T) {
	if got := vcardProperty(realVCard, waBizDescription); got !=
		"Loja pertencente a Rede Exemplo Fictício do Bairro - SP" {
		t.Fatalf("business description not read, got %q", got)
	}

	// A group prefix (`item1.`) and parameters (`;waid=`) must not hide a
	// property — a substring match would return the wrong line here.
	if got := vcardProperty(realVCard, "TEL"); got != "+55 11 9000-0001" {
		t.Errorf("TEL should be found past its group and params, got %q", got)
	}

	// Folding: a value longer than 75 octets is split, and each continuation
	// line is prefixed with ONE whitespace that is the fold marker, not text.
	// So a space that belongs to the sentence is written twice on the wire —
	// stripping all leading whitespace silently glues words together.
	folded := "BEGIN:VCARD\n" + waBizDescription + ":Loja pertencente a\n" +
		"  Rede Exemplo Fictício\n\t do Bairro\nEND:VCARD"
	if got := vcardProperty(folded, waBizDescription); got !=
		"Loja pertencente a Rede Exemplo Fictício do Bairro" {
		t.Errorf("folded value not rejoined, got %q", got)
	}

	// An absent property is empty, not the next line's value.
	if got := vcardProperty(realVCard, "X-NOT-THERE"); got != "" {
		t.Errorf("missing property should be empty, got %q", got)
	}
}

func TestUnescapeVCardValue_BackslashBeforeN(t *testing.T) {
	// `\\n` is a literal backslash then an n — NOT a newline. Replacing `\n`
	// first would eat the escape and corrupt the text.
	if got := unescapeVCardValue(`a\\nb`); got != `a\nb` {
		t.Errorf("got %q, want %q", got, `a\nb`)
	}
	if got := unescapeVCardValue(`linha1\nlinha2`); got != "linha1\nlinha2" {
		t.Errorf("real newline escape not honoured, got %q", got)
	}
	if got := unescapeVCardValue(`Silva\, Jose\; Maria`); got != "Silva, Jose; Maria" {
		t.Errorf("comma/semicolon escapes not honoured, got %q", got)
	}
}

func TestVCardProperty_ReadsTheBusinessName(t *testing.T) {
	if got := vcardProperty(realVCard, waBizName); got != "Mercearia Boa Vista Centro" {
		t.Fatalf("business name not read, got %q", got)
	}
}

func TestSameDisplayName(t *testing.T) {
	// Equal enough to not print twice.
	for _, p := range [][2]string{
		{"Mercearia Boa Vista Centro", "Mercearia Boa Vista Centro"},
		{"padaria do joão", "Padaria do João"},
		{"  Padaria   do  João ", "Padaria do João"},
	} {
		if !sameDisplayName(p[0], p[1]) {
			t.Errorf("sameDisplayName(%q,%q) = false, want true", p[0], p[1])
		}
	}
	// Different enough that hiding one would lose a fact. Accents are NOT
	// folded: a business that spells itself "São" is not the same label as
	// "Sao", and showing both costs a few characters while collapsing them
	// silently drops the official name.
	for _, p := range [][2]string{
		{"Gerente Paulo celular", "Rede Exemplo Fictício do Bairro"},
		{"Sao Jose", "São José"},
		{"", "Rede X"},
	} {
		if sameDisplayName(p[0], p[1]) {
			t.Errorf("sameDisplayName(%q,%q) = true, want false", p[0], p[1])
		}
	}
}

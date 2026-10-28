package domain

import "testing"

// The identity index exists so that a person who wrote by email and later on
// WhatsApp is one contact rather than two strangers. It was empty for WhatsApp.
//
// Meta writes a sender as bare digits. That value failed the E.164 check in
// Validate, the write was rejected, and the caller — which treats identity
// registration as best-effort — logged a warning and moved on. Nothing was
// broken loudly enough to notice.
func TestIdentityValueFromChannelSender_QualifiesABareSender(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"whatsapp sender, as Meta writes it", "5511999998888", "+5511999998888"},
		{"already qualified, untouched", "+5511999998888", "+5511999998888"},
		{"spaces from a pasted number", "+55 11 99999 8888", "+5511999998888"},
		// A channel's sender is already country-coded — the platform routed the
		// message to us. Prepending '+' keeps a foreign sender in their own
		// country; promoting them to +55 would put the reply on a stranger's
		// phone in São Paulo.
		{"foreign sender keeps its own country", "14155551234", "+14155551234"},
		{"empty stays empty", "", ""},
		// Not a phone at all: a webchat session key resolves to the phone kind
		// by default. Left unqualified so Validate still refuses it, which is
		// the behaviour that was already correct.
		{"non-numeric is left for Validate to refuse", "sess_abc123", "sess_abc123"},
		{"a formatted id is not silently turned into a number", "sess-99-11", "sess-99-11"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IdentityValueFromChannelSender(IdentityKindPhone, tc.in)
			if got != tc.want {
				t.Fatalf("IdentityValueFromChannelSender(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// The write and the lookup must agree, or the lookup misses a row that is
// really there. This pins that a WhatsApp sender survives validation once it
// has crossed the channel boundary.
func TestContactIdentity_Validate_AcceptsAQualifiedWhatsAppSender(t *testing.T) {
	id := &ContactIdentity{
		AccountID: "acc",
		ContactID: "contact",
		Kind:      IdentityKindPhone,
		Value:     IdentityValueFromChannelSender(IdentityKindPhone, "5511999998888"),
	}
	if err := id.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil — this is what every WhatsApp inbound writes", err)
	}
	if id.Value != "+5511999998888" {
		t.Fatalf("Value = %q, want the E.164 form so an emailed lookup finds it", id.Value)
	}
}

// The rule lives at the boundary that knows where the number came from, and
// NOT in the shared normalizer — which sees values from every origin and
// cannot tell them apart.
//
// "11999990000" typed into a form is a Brazilian national number; the same
// digits with a '+' in front address Canada. A normalizer that guessed would
// send a proposal to a stranger in another country.
func TestNormalizeIdentityValue_DoesNotPromoteABareNumber(t *testing.T) {
	if got := NormalizeIdentityValue(IdentityKindPhone, "11999990000"); got != "11999990000" {
		t.Fatalf("got %q — a bare national number must stay unqualified and be refused", got)
	}
	id := &ContactIdentity{
		AccountID: "acc", ContactID: "c",
		Kind: IdentityKindPhone, Value: "11999990000",
	}
	if err := id.Validate(); err == nil {
		t.Fatal("Validate() = nil, want a refusal: this number names no country")
	}
}

// A human typing their own country's format is the other origin, and it has its
// own rule.
func TestNormalizeTypedPhone_AcceptsWhatABrazilianTypes(t *testing.T) {
	for _, in := range []string{"51 98888-7766", "(51) 98888-7766", "5551988887766"} {
		got, ok := NormalizeTypedPhone(in)
		if !ok || got != "+5551988887766" {
			t.Fatalf("NormalizeTypedPhone(%q) = %q, %v", in, got, ok)
		}
	}
}

// Email normalization is what stops one mailbox becoming two identities.
func TestNormalizeIdentityValue_EmailIsCaseFolded(t *testing.T) {
	if got := NormalizeIdentityValue(IdentityKindEmail, " Joe@ACME.com "); got != "joe@acme.com" {
		t.Fatalf("got %q", got)
	}
	// And the channel boundary must not undo it.
	if got := IdentityValueFromChannelSender(IdentityKindEmail, " Joe@ACME.com "); got != "joe@acme.com" {
		t.Fatalf("got %q", got)
	}
}

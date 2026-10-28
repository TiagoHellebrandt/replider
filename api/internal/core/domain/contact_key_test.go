package domain

import "testing"

// One customer, one key: every door must derive the same external_id for
// the same person, whatever spelling the address arrived in.
func TestContactKey(t *testing.T) {
	cases := []struct {
		name        string
		phone, mail string
		want        string
		ok          bool
	}{
		{"E.164 phone becomes digits without the plus", "+556536188000", "", "556536188000", true},
		{"a human-formatted E.164 is stripped the same way", "+55 65 3618-8000", "", "556536188000", true},
		{"the phone wins over the e-mail", "+556536188000", "compras@hospital.br", "556536188000", true},
		{"an e-mail alone is lower-cased", "", "Compras@Hospital.BR", "compras@hospital.br", true},
		{"a bare national number is refused — its country is unknown here", "6536188000", "", "", false},
		{"an unparseable phone falls through to the e-mail", "abc", "x@y.com", "x@y.com", true},
		{"garbage in both is no key", "", "not-an-email", "", false},
		{"nothing is no key", "", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ContactKey(tc.phone, tc.mail)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("ContactKey(%q, %q) = (%q, %v), want (%q, %v)", tc.phone, tc.mail, got, ok, tc.want, tc.ok)
			}
		})
	}
}

// The CSV row key is the same rule, so an imported customer and the WhatsApp
// message they send later meet on one row.
func TestCustomerImportRow_ChannelKeyUsesTheOneRule(t *testing.T) {
	key, ch := CustomerImportRow{Phone: "+5551988887766", Email: "x@y.com"}.ChannelKey()
	if key != "5551988887766" || ch != "whatsapp" {
		t.Fatalf("phone row: got (%q, %q)", key, ch)
	}
	key, ch = CustomerImportRow{Email: "Compras@Hospital.BR"}.ChannelKey()
	if key != "compras@hospital.br" || ch != "email" {
		t.Fatalf("email row: got (%q, %q)", key, ch)
	}
}

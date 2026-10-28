package domain

import "testing"

// Every one of these is how a Brazilian actually writes a phone number. A
// salesperson standing at a hospital door who is rejected for typing their
// own country's format has met a wall, not a validation.
func TestNormalizeTypedPhone_AcceptsWhatPeopleType(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"51 98888-7766", "+5551988887766"},
		{"(51) 98888-7766", "+5551988887766"},
		{"51988887766", "+5551988887766"},
		{"5133334444", "+555133334444"}, // fixo, 10 dígitos
		{"55 51 98888-7766", "+5551988887766"},
		{"+55 51 98888-7766", "+5551988887766"},
		{"+5551988887766", "+5551988887766"},
	} {
		got, ok := NormalizeTypedPhone(tc.in)
		if !ok || got != tc.want {
			t.Errorf("NormalizeTypedPhone(%q) = %q,%v; quero %q", tc.in, got, ok, tc.want)
		}
	}
}

// Guessing a country for an ambiguous string would put a cold outbound
// message on a stranger's phone.
func TestNormalizeTypedPhone_RefusesTheUnguessable(t *testing.T) {
	for _, in := range []string{"", "   ", "não é telefone", "123", "9999999999999999999"} {
		if _, ok := NormalizeTypedPhone(in); ok {
			t.Errorf("NormalizeTypedPhone(%q) aceitou, deveria recusar", in)
		}
	}
}

// An explicit + means the caller already said which country — never
// re-country it as Brazilian.
func TestNormalizeTypedPhone_KeepsForeignNumbers(t *testing.T) {
	got, ok := NormalizeTypedPhone("+1 415 555 0100")
	if !ok || got != "+14155550100" {
		t.Errorf("número dos EUA virou %q,%v", got, ok)
	}
}

package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The golpe do boleto works because a supplier's real thread is hijacked and
// one message carries different bank details. Nothing about that message is
// spammy — what catches it is knowing what THAT contact's details have always
// been. These pin the extraction that makes the comparison possible.

func paymentValues(ds []PaymentDetail, kind PaymentKind) []string {
	var out []string
	for _, d := range ds {
		if d.Kind == kind {
			out = append(out, d.Value)
		}
	}
	return out
}

func TestExtract_RealInvoiceEmail(t *testing.T) {
	body := `Prezados, segue a fatura de julho.

Banco Itaú
Ag. 1234 C/C 56789-0
CNPJ: 12.345.678/0001-90
Chave PIX: financeiro@medline.com.br

Linha digitável: 34191.79001 01043.510047 91020.150008 1 98770000026000

Atenciosamente,
Financeiro`

	got := ExtractPaymentDetails(body)

	assert.Equal(t, []string{"1234/567890"}, paymentValues(got, PaymentBankAccount))
	assert.Equal(t, []string{"12345678000190"}, paymentValues(got, PaymentCNPJ))
	assert.Equal(t, []string{"financeiro@medline.com.br"}, paymentValues(got, PaymentPixKey))
	assert.Len(t, paymentValues(got, PaymentBoleto), 1)
	assert.True(t, HasPaymentInstruction(got))
}

// Punctuation must not make the same account look like a new one — that is a
// false alarm about a supplier who did nothing wrong, and false alarms are
// what make people stop reading alerts.
func TestExtract_SameAccountDifferentPunctuationIsOneValue(t *testing.T) {
	a := ExtractPaymentDetails("Ag 1234 C/C 56789-0")
	b := ExtractPaymentDetails("agência: 1234, conta corrente: 567890")

	require.NotEmpty(t, a)
	require.NotEmpty(t, b)
	assert.Equal(t, paymentValues(a, PaymentBankAccount), paymentValues(b, PaymentBankAccount))
}

func TestExtract_PixKeyForms(t *testing.T) {
	cases := map[string]string{
		"PIX: 123.456.789-00":                      "12345678900",
		"chave pix: 12.345.678/0001-90":            "12345678000190",
		"Chave PIX: Financeiro@Medline.com.BR":     "financeiro@medline.com.br",
		"pix: +55 15 99999-0000":                   "5515999990000",
		"PIX 550e8400-e29b-41d4-a716-446655440000": "550e8400-e29b-41d4-a716-446655440000",
	}
	for body, want := range cases {
		got := paymentValues(ExtractPaymentDetails(body), PaymentPixKey)
		assert.Contains(t, got, want, "input %q", body)
	}
}

// Most mail carries no payment instruction, and running the fraud comparison
// on it would be noise.
func TestExtract_OrdinaryMessageHasNothing(t *testing.T) {
	got := ExtractPaymentDetails(
		"Bom dia, podemos marcar uma reunião na quinta às 14h? Abraço, Dra. Ana")

	assert.Empty(t, got)
	assert.False(t, HasPaymentInstruction(got))
}

// A CNPJ in a signature is not a request for money. It is recorded, because a
// changed CNPJ on an invoice matters, but on its own it must not make an
// ordinary message look like a payment instruction.
func TestExtract_CNPJAloneIsNotAPaymentInstruction(t *testing.T) {
	got := ExtractPaymentDetails("Medline Brasil — CNPJ 12.345.678/0001-90")

	assert.Equal(t, []string{"12345678000190"}, paymentValues(got, PaymentCNPJ))
	assert.False(t, HasPaymentInstruction(got))
}

func TestExtract_DeduplicatesRepeatedValues(t *testing.T) {
	got := ExtractPaymentDetails("PIX: a@b.com\nrepetindo, PIX: a@b.com")

	assert.Len(t, paymentValues(got, PaymentPixKey), 1)
}

// An agência alone is a branch, shared by thousands of legitimate accounts.
func TestExtract_AgencyWithoutAccountIsNotEnough(t *testing.T) {
	got := ExtractPaymentDetails("Nossa agência é a 1234, fica no centro")

	assert.Empty(t, paymentValues(got, PaymentBankAccount))
}

// ── The comparison the alert rests on ─────────────────────────────────────

func knownPayments(pairs ...PaymentDetail) map[string]struct{} {
	out := map[string]struct{}{}
	for _, d := range pairs {
		out[PaymentKey(d.Kind, d.Value)] = struct{}{}
	}
	return out
}

// The message this whole layer exists to catch.
func TestNewPaymentDetails_ChangedAccountIsDetected(t *testing.T) {
	history := knownPayments(PaymentDetail{Kind: PaymentBankAccount, Value: "1234/567890"})
	found := ExtractPaymentDetails("Mudamos de banco. Ag 9999 C/C 111222-3")

	changed := NewPaymentDetails(found, history)

	require.Len(t, changed, 1)
	assert.Equal(t, "9999/1112223", changed[0].Value)
}

func TestNewPaymentDetails_SameAccountIsSilent(t *testing.T) {
	history := knownPayments(PaymentDetail{Kind: PaymentBankAccount, Value: "1234/567890"})
	found := ExtractPaymentDetails("Segue a fatura. Ag. 1234 C/C 56789-0")

	assert.Empty(t, NewPaymentDetails(found, history))
}

// The most important negative: a first invoice from a new supplier is not
// suspicious, it is a first invoice. Flagging it fires on every legitimate
// new relationship, and then the alert stops being read.
func TestNewPaymentDetails_NoHistoryMeansNoAlarm(t *testing.T) {
	found := ExtractPaymentDetails("Ag 9999 C/C 111222-3")

	assert.Empty(t, NewPaymentDetails(found, nil))
	assert.Empty(t, NewPaymentDetails(found, map[string]struct{}{}))
}

// A supplier adding a PIX key alongside the account they always used is still
// a change worth showing — the reader decides.
func TestNewPaymentDetails_AddedKeyAlongsideAKnownOneIsReported(t *testing.T) {
	history := knownPayments(PaymentDetail{Kind: PaymentBankAccount, Value: "1234/567890"})
	found := ExtractPaymentDetails("Ag 1234 C/C 56789-0 ou PIX: novo@fornecedor.com")

	changed := NewPaymentDetails(found, history)

	require.Len(t, changed, 1)
	assert.Equal(t, PaymentPixKey, changed[0].Kind)
}

// The tail of a linha digitável is fourteen digits, which is exactly a CNPJ.
// Reading it as one invents a company the message never mentioned, and then
// reports it as "new" on every single invoice.
func TestExtract_BoletoDigitsAreNotMistakenForACNPJ(t *testing.T) {
	got := ExtractPaymentDetails(
		"Linha digitável: 34191.79001 01043.510047 91020.150008 1 98770000026000")

	assert.Len(t, paymentValues(got, PaymentBoleto), 1)
	assert.Empty(t, paymentValues(got, PaymentCNPJ),
		"digits inside a boleto are not a company")
}

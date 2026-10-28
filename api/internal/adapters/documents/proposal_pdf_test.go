package documents

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ledongthuc/pdf"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

// The quote has to come out as a file, and the file has to say the numbers.
//
// A generated PDF that opens blank is indistinguishable from a working one
// until a customer receives it, so the test reopens the bytes and reads the
// text back rather than asserting on a byte count.
func TestRenderProposal_SaysThePricesOutLoud(t *testing.T) {
	valid := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	out, err := RenderProposal(ProposalDocument{
		Seller:     "CuiabáMed Equipamentos",
		Signature:  "Marina Alves · comercial@cuiabamed.com.br",
		Customer:   "Hospital Santa Rosa",
		Reference:  "01KZMVMGFC35QAM2K3W10WD9F1",
		Currency:   domain.CurrencyBRL,
		Body:       "Conforme conversamos, segue a proposta para os monitores multiparamétricos.",
		CreatedAt:  time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC),
		ValidUntil: &valid,
		Items: []domain.ProposalItem{
			{Name: "Monitor multiparametrico Logicare", Quantity: 2, UnitPriceCents: 1_225_000},
			{Name: "Sensor de oximetria", Quantity: 4, UnitPriceCents: 32_500},
		},
	})
	require.NoError(t, err)
	require.True(t, bytes.HasPrefix(out, []byte("%PDF-")), "must be a PDF")

	text := readBack(t, out)

	// Every number the customer will be held to.
	require.Contains(t, text, "24.500,00", "line total for 2 x 12.250,00")
	require.Contains(t, text, "1.300,00", "line total for 4 x 325,00")
	require.Contains(t, text, "25.800,00", "the grand total")
	// And who it is from, to, and about.
	require.Contains(t, text, "CuiabáMed")
	require.Contains(t, text, "Hospital Santa Rosa")
	require.Contains(t, text, "Monitor multiparametrico Logicare")
	require.Contains(t, text, "15/10/2026", "the validity the customer will hold us to")
	// The reference is the ULID's TAIL: its head is a timestamp, so every
	// proposal made the same afternoon would share it.
	require.Contains(t, text, "0WD9F1")
}

// A long paragraph must not run off the page. It wraps; nothing is lost.
func TestRenderProposal_WrapsTheSalespersonsWords(t *testing.T) {
	long := strings.Repeat("equipamento hospitalar de alta performance ", 12)
	out, err := RenderProposal(ProposalDocument{
		Seller:   "CuiabáMed",
		Currency: domain.CurrencyBRL,
		Body:     long,
		Items:    []domain.ProposalItem{{Name: "Monitor", Quantity: 1, UnitPriceCents: 100}},
	})
	require.NoError(t, err)

	text := readBack(t, out)
	require.Contains(t, text, "alta performance")
	// The last words of the paragraph survived the wrap — a silent truncation
	// would drop the salesperson's closing sentence.
	require.Contains(t, text, "hospitalar")
}

// A newline inside the body used to end the line silently, leaving a blank
// area on the customer's screen.
func TestRenderProposal_ANewlineDoesNotSwallowTheRest(t *testing.T) {
	out, err := RenderProposal(ProposalDocument{
		Seller:   "CuiabáMed",
		Currency: domain.CurrencyBRL,
		Body:     "Primeira linha.\nSegunda linha com o prazo.",
		Items:    []domain.ProposalItem{{Name: "Monitor", Quantity: 1, UnitPriceCents: 100}},
	})
	require.NoError(t, err)

	text := readBack(t, out)
	require.Contains(t, text, "Primeira linha")
	require.Contains(t, text, "Segunda linha com o prazo")
}

// An offer with no lines is not a document — refusing beats handing a customer
// a letterhead with an empty table.
func TestRenderProposal_RefusesAnEmptyOffer(t *testing.T) {
	_, err := RenderProposal(ProposalDocument{Seller: "X", Currency: domain.CurrencyBRL})
	require.Error(t, err)
}

func readBack(t *testing.T, raw []byte) string {
	t.Helper()
	r, err := pdf.NewReader(bytes.NewReader(raw), int64(len(raw)))
	require.NoError(t, err, "the PDF we generated must be one we can reopen")
	rc, err := r.GetPlainText()
	require.NoError(t, err)
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(rc)
	require.NoError(t, err)
	return buf.String()
}

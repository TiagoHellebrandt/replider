package usecases

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Importing a sales history. The preview is where a person catches what the
// parser could not read or resolve, so these pin that it reports rather than
// guesses — and that the import records exactly what the preview showed.

// salesContactStore is the customer-import fake plus a name lookup, which the
// sales import needs and the customer import does not.
type salesContactStore struct {
	*fakeContactStore
}

func (s *salesContactStore) ListByAccountID(_ context.Context, _, query string, _, _ int) ([]*domain.Contact, error) {
	var out []*domain.Contact
	for _, c := range s.byID {
		if c.Name != nil && strings.Contains(strings.ToLower(*c.Name), strings.ToLower(query)) {
			out = append(out, c)
		}
	}
	return out, nil
}

type fakeSaleIndex struct {
	recorded map[string]bool
}

func (f *fakeSaleIndex) HasSaleAt(_ context.Context, _, contactID string, soldAt time.Time, gross int64) (bool, error) {
	return f.recorded[saleImportKey(contactID, soldAt, gross)], nil
}

type saleImportFixture struct {
	uc       *ImportSalesUseCase
	sales    *fakeSaleRepo
	contacts *salesContactStore
	index    *fakeSaleIndex
	products *importProductRepo
}

func newSaleImportFixture() *saleImportFixture {
	contacts := &salesContactStore{fakeContactStore: newFakeContactStore()}
	aurora := "Clínica Aurora"
	contacts.byID["c-aurora"] = &domain.Contact{ID: "c-aurora", ExternalID: "5551988887766", Name: &aurora}
	contacts.byExternal["5551988887766"] = contacts.byID["c-aurora"]
	vale := "Hospital Vale"
	contacts.byID["c-vale"] = &domain.Contact{ID: "c-vale", ExternalID: "compras@vale.com.br", Name: &vale}
	contacts.byExternal["compras@vale.com.br"] = contacts.byID["c-vale"]

	identities := newFakeIdentityStore()
	identities.byValue[identityKey(domain.IdentityKindPhone, "+5551988887766")] = "c-aurora"

	products := &importProductRepo{existing: []*domain.Product{
		{ID: "p-luvas", AccountID: importAccount, Name: "Luvas de procedimento"},
	}}
	sales := &fakeSaleRepo{}
	index := &fakeSaleIndex{recorded: map[string]bool{}}

	uc := NewImportSalesUseCase(sales, index, contacts, products, nil)
	uc.SetIdentityRepo(identities)
	return &saleImportFixture{uc: uc, sales: sales, contacts: contacts, index: index, products: products}
}

func (f *saleImportFixture) preview(t *testing.T, csv string) *SaleImportPreview {
	t.Helper()
	out, err := f.uc.Preview(context.Background(), importAccount, strings.NewReader(csv))
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	return out
}

func TestImportSales_PreviewResolvesCustomersAndProducts(t *testing.T) {
	f := newSaleImportFixture()

	out := f.preview(t, "Cliente;Produto;Data;Valor;Qtd\n"+
		"(51) 98888-7766;luvas de procedimento;10/03/2026;1.500,00;200\n"+
		"compras@vale.com.br;Gaze estéril;2026-03-11;320,50;\n"+
		"Hospital Vale;Seringa;11/03/2026;R$ 99,90;10\n")

	if out.Importable != 3 || out.Problems != 0 {
		t.Fatalf("counts wrong: %+v", out)
	}
	rows := out.Rows
	// Phone typed the Brazilian way → identity index → the contact.
	if rows[0].ContactID != "c-aurora" || rows[0].ContactName != "Clínica Aurora" {
		t.Fatalf("phone resolution wrong: %+v", rows[0])
	}
	// Catalogue match is case-insensitive.
	if rows[0].ProductID != "p-luvas" {
		t.Fatalf("product match wrong: %+v", rows[0])
	}
	if rows[0].GrossCents != 150000 || rows[0].Quantity != 200 {
		t.Fatalf("money/quantity wrong: %+v", rows[0])
	}
	if !rows[0].SoldAt.Equal(time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("date wrong: %v", rows[0].SoldAt)
	}
	// E-mail → channel key; ISO date; no catalogue match keeps the text.
	if rows[1].ContactID != "c-vale" || rows[1].ProductID != "" || rows[1].GrossCents != 32050 {
		t.Fatalf("email row wrong: %+v", rows[1])
	}
	// A name resolves when exactly one contact carries it.
	if rows[2].ContactID != "c-vale" || rows[2].GrossCents != 9990 {
		t.Fatalf("name row wrong: %+v", rows[2])
	}
}

// Every refusal is attached to the line the person is looking at, and a
// customer the base does not have is reported — never created.
func TestImportSales_PreviewReportsWhatItCannotUse(t *testing.T) {
	f := newSaleImportFixture()

	out := f.preview(t, "Cliente,Produto,Data,Valor\n"+
		",Luvas,10/03/2026,100\n"+
		"Hospital Vale,Luvas,,100\n"+
		"Hospital Vale,Luvas,ontem,100\n"+
		"Hospital Vale,Luvas,10/03/2026,\n"+
		"Hospital Vale,Luvas,10/03/2026,\"1,234\"\n"+
		"ninguem@nada.com,Luvas,10/03/2026,100\n"+
		"Hospital Vale,Luvas,10/03/2026,100\n")

	want := []string{
		ProblemSaleNoCustomer, ProblemSaleNoDate, ProblemSaleBadDate,
		ProblemSaleNoAmount, domain.ProblemAmbiguousPrice,
		ProblemSaleCustomerUnknown, "",
	}
	for i, problem := range want {
		if out.Rows[i].Problem != problem {
			t.Errorf("row %d: problem = %q, want %q", i, out.Rows[i].Problem, problem)
		}
	}
	if out.Importable != 1 || out.Problems != 6 || out.UnknownCustomers != 1 {
		t.Fatalf("counts wrong: %+v", out)
	}
	if len(f.contacts.upserts) != 0 {
		t.Fatalf("the import must not create customers: %+v", f.contacts.upserts)
	}
}

func TestImportSales_TheWrongFileNamesTheMissingColumns(t *testing.T) {
	f := newSaleImportFixture()
	_, err := f.uc.Preview(context.Background(), importAccount, strings.NewReader("Nome,Preço\nGaze,49.90\n"))
	if err != ErrSaleImportNoHeader {
		t.Fatalf("err = %v", err)
	}
	_, err = f.uc.Preview(context.Background(), importAccount, strings.NewReader(""))
	if err != ErrSaleImportEmpty {
		t.Fatalf("empty: err = %v", err)
	}
}

// One customer, one day, one total: recorded once, whether the twin is in the
// base already or further down the same file.
func TestImportSales_AlreadyRecordedIsSkippedNotDoubled(t *testing.T) {
	f := newSaleImportFixture()
	f.index.recorded[saleImportKey("c-vale", time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), 10000)] = true

	out := f.preview(t, "Cliente,Data,Valor\n"+
		"Hospital Vale,10/03/2026,100.00\n"+ // already in the base
		"Hospital Vale,11/03/2026,100.00\n"+ // new
		"Hospital Vale,11/03/2026,100.00\n") // twin of the line above

	if out.AlreadyRecorded != 2 || out.Importable != 1 {
		t.Fatalf("counts wrong: %+v", out)
	}
	if !out.Rows[0].Duplicate || out.Rows[0].Problem != ProblemSaleAlreadyRecorded {
		t.Fatalf("existing sale not flagged: %+v", out.Rows[0])
	}
	if !out.Rows[2].Duplicate {
		t.Fatalf("in-file twin not flagged: %+v", out.Rows[2])
	}

	res, err := f.uc.Import(context.Background(), importAccount, ImportSalesInput{Rows: out.Rows})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if res.Created != 1 || res.Skipped != 2 || len(f.sales.created) != 1 {
		t.Fatalf("result wrong: %+v (created %d)", res, len(f.sales.created))
	}
}

// The import records what the preview resolved: source=imported, the sale
// dated by the file, the item carrying the catalogue product and a unit price.
func TestImportSales_RecordsImportedSalesWithTheProductOnTheItem(t *testing.T) {
	f := newSaleImportFixture()
	out := f.preview(t, "Cliente;Produto;Data;Valor;Quantidade\n"+
		"(51) 98888-7766;Luvas de procedimento;10/03/2026;1.500,00;200\n")

	res, err := f.uc.Import(context.Background(), importAccount, ImportSalesInput{Rows: out.Rows})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if res.Created != 1 || len(f.sales.created) != 1 {
		t.Fatalf("result wrong: %+v", res)
	}
	s := f.sales.created[0]
	if s.Source != domain.SaleImported || s.AccountID != importAccount || s.ContactID != "c-aurora" {
		t.Fatalf("sale wrong: %+v", s)
	}
	if s.Currency != "brl" || s.GrossCents != 150000 || !s.SoldAt.Equal(time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("sale money/date wrong: %+v", s)
	}
	if len(s.Items) != 1 || s.Items[0].ProductID != "p-luvas" || s.Items[0].Quantity != 200 ||
		s.Items[0].UnitCents != 750 || s.Items[0].Description != "Luvas de procedimento" {
		t.Fatalf("item wrong: %+v", s.Items)
	}
}

// A confirmed row is re-derived, never trusted: a contact id the preview did
// not resolve, or a problem the client cleared, cannot file a sale.
func TestImportSales_RechecksInsteadOfTrustingTheClient(t *testing.T) {
	f := newSaleImportFixture()

	res, err := f.uc.Import(context.Background(), importAccount, ImportSalesInput{Rows: []SaleImportRow{
		// Unknown customer arriving with a contact id attached.
		{Line: 2, Customer: "ninguem@nada.com", ContactID: "c-vale", Date: "10/03/2026", GrossCents: 100},
		// A date the preview refused, arriving with its problem cleared.
		{Line: 3, Customer: "Hospital Vale", Date: "ontem", GrossCents: 100},
		// Fine.
		{Line: 4, Customer: "Hospital Vale", Date: "10/03/2026", GrossCents: 100},
	}})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if res.Created != 1 || res.Skipped != 2 {
		t.Fatalf("result wrong: %+v", res)
	}
	if f.sales.created[0].ContactID != "c-vale" ||
		!f.sales.created[0].SoldAt.Equal(time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("wrong sale: %+v", f.sales.created[0])
	}
}

func TestParseSaleDate_ReadsWhatASpreadsheetWrites(t *testing.T) {
	cases := map[string]time.Time{
		"10/03/2026":           time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC),
		"2026-03-10":           time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC),
		"10-03-2026":           time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC),
		"1/3/2026":             time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		"10/03/2026 14:32":     time.Date(2026, 3, 10, 14, 32, 0, 0, time.UTC),
		"2026-03-10T14:32:00Z": time.Date(2026, 3, 10, 14, 32, 0, 0, time.UTC),
	}
	for raw, want := range cases {
		got, ok := ParseSaleDate(raw)
		if !ok || !got.Equal(want) {
			t.Errorf("%q: got %v ok=%v, want %v", raw, got, ok, want)
		}
	}
	for _, raw := range []string{"", "ontem", "31/02/2026", "março"} {
		if _, ok := ParseSaleDate(raw); ok {
			t.Errorf("%q should not parse", raw)
		}
	}
}
